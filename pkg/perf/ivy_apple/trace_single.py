"""Drain the Ivy golden-test stream independently, recording progress, time and check outcomes."""
import argparse
import csv
import os
from pathlib import Path
import subprocess
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("checker", type=Path)
    parser.add_argument("spec", type=Path)
    parser.add_argument("--isolate", required=True)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--cpu", required=True, type=int)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=False)
    checker = args.checker.absolute()
    spec = args.spec.absolute()
    env = os.environ.copy()
    env.pop("XTRACE_OFF", None)
    command = ["taskset", "-c", str(args.cpu), "/usr/bin/time", "-f", "%e %U %S %M %x",
               "-o", str((args.output / "resources").absolute()), str(checker), "isolate=" + args.isolate, str(spec)]
    last = b""
    start = time.monotonic()
    with (args.output / "checker.out").open("wb") as ordinary, (args.output / "progress.csv").open("w") as progress:
        rows = csv.writer(progress)
        rows.writerow(["lines", "trace_records", "bytes", "passed", "wall_s"])
        lines = traces = size = passed = 0
        with subprocess.Popen(command, env=env, cwd=args.output, stdout=subprocess.PIPE, stderr=subprocess.STDOUT) as child:
            for line in child.stdout:
                lines += 1
                size += len(line)
                if line.startswith(b"XTRACE:"):
                    traces += 1
                else:
                    ordinary.write(line)
                    if line.strip():
                        last = line.strip()
                    passed += line == b"PASS\n"
                if lines % 500000 == 0:
                    rows.writerow([lines, traces, size, passed, round(time.monotonic() - start, 3)])
                    progress.flush()
            status = child.wait()
            rows.writerow([lines, traces, size, passed, round(time.monotonic() - start, 3)])
        if status:
            raise SystemExit(status)
        if last != b"OK":
            raise SystemExit(f"Incomplete checker output in {args.output}")
        if not traces:
            raise SystemExit("The checker emitted no trace records.")


if __name__ == "__main__":
    main()
