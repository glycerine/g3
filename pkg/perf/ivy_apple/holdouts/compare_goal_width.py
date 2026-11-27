"""Compare complete verifiers on one Ivy input and validate each run against linked Z3."""

import argparse
import csv
import os
from pathlib import Path
import shutil
import statistics
import subprocess
import tempfile
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("before", "after", "native", "examples", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--rounds", type=int, default=7)
    parser.add_argument("--perf", default=shutil.which("perf"))
    parser.add_argument("--isolate")
    parser.add_argument("spec", help="path relative to the Ivy examples root")
    args = parser.parse_args()
    if not args.perf or args.rounds < 1:
        parser.error("a perf executable and a positive round count are required")
    env = dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1",
               GOIVY_INCLUDE=str(args.examples.absolute() / "ivy/include"))
    env.pop("GOGC", None)
    ivy_args = (["isolate=" + args.isolate] if args.isolate else []) + [
        str(args.examples.absolute() / args.spec)]
    prefix = ["taskset", "-c", str(args.cpu)]
    ratios = []
    with tempfile.TemporaryDirectory() as tmp, args.output.open("x", newline="") as out:
        def run(cmd):
            result = subprocess.run(prefix + cmd, env=env, cwd=tmp,
                                    capture_output=True, timeout=240)
            return result.returncode, result.stdout, result.stderr

        expected = run([args.native.resolve(), *ivy_args])
        reported_check = b"PASS" in expected[1] or b"FAIL" in expected[1].splitlines()
        if (expected[0] not in (0, 1) or not reported_check or
                b"failed to run model checker" in expected[2]):
            raise RuntimeError("linked Z3 failed to produce a verification transcript")
        writer = csv.writer(out)
        writer.writerow(["case", "round", "engine", "instructions", "cycles", "milliseconds"])
        engines = [("before", args.before.resolve()), ("after", args.after.resolve())]
        counters = Path(tmp) / "counts"
        for i in range(args.rounds):
            measured = {}
            for label, exe in engines:
                start = time.monotonic()
                actual = run([args.perf, "stat", "-x,", "-o", counters, "-e",
                              "instructions:u,cycles:u", "--", exe, *ivy_args])
                ms = (time.monotonic() - start) * 1000
                if actual != expected:
                    raise RuntimeError(f"{label} differs from linked Z3 in round {i + 1}")
                with counters.open() as src:
                    counts = {r[2]: int(r[0]) for r in csv.reader(src)
                              if len(r) > 4 and float(r[4]) == 100}
                measured[label] = counts["instructions:u"]
                writer.writerow([args.spec, i + 1, label, measured[label], counts["cycles:u"], ms])
                out.flush()
            ratios.append(measured["after"] / measured["before"])
            engines.reverse()
    print(f"Median paired instruction change {100 * (statistics.median(ratios) - 1):+.2f}%")


if __name__ == "__main__":
    main()
