"""Test that concrete finite models lift to models of g3's emitted order problems."""

import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

from finite_model import Problem


def build_driver(work):
    here = Path(__file__).resolve().parent
    root = here.parents[2]
    package = root / "api/internal/order"
    original = package / "relax.go"
    source = original.read_text()
    result = "return t.render(), true, nil"
    if source.count(result) != 1:
        raise ValueError("cannot attach diagnostic names to the order renderer")
    instrumented = work / "relax.go"
    instrumented.write_text(source.replace(result, """
        names, err := t.modelStudyMetadata()
        if err != nil { return "", false, err }
        return names + t.render(), true, nil"""))
    overlay = work / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {
        str(original): str(instrumented),
        str(package / "study_metadata.go"): str(here / "metadata.go.txt"),
        str(root / "api/study_order_models_test.go"): str(here / "stream_test.go.txt"),
    }}))
    driver = work / "order-models.test"
    subprocess.run(["go", "test", "-c", f"-overlay={overlay}", "-o", driver, "./api"],
                   cwd=root, check=True)
    return driver


def check_native(exe, script):
    query = "(set-option :timeout 1500)\n" + script + "\n(check-sat)\n"
    result = subprocess.run([exe, "-in", "-smt2"], input=query, text=True,
                            capture_output=True, timeout=10, check=True)
    answer = result.stdout.strip()
    if answer not in ("sat", "unknown"):
        raise ValueError(f"native solver returned {answer} for a witnessed query")
    return answer


def samples(args, proc):
    started = time.monotonic()
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["first_seed", "models", "proof_attempts", "assertions", "domains",
                     "native_sat", "native_unknown", "elapsed_s"])
    attempts = assertions = domains = native_sat = native_unknown = 0
    cache = set()
    for i, seed in enumerate(range(args.start, args.start + args.count), 1):
        if seed % 500 == 0:
            cache.clear()
        model = Problem(seed)
        script = model.script()
        try:
            if args.native and seed % args.native_every == 0:
                answer = check_native(args.native, script)
                native_sat += answer == "sat"
                native_unknown += answer == "unknown"
            check = bool(args.prove_every and seed % args.prove_every == 0)
            proc.stdin.write(json.dumps({"Script": script, "Check": check}) + "\n")
            proc.stdin.flush()
            result = json.loads(proc.stdout.readline())
            if result["Err"] or not result["Ready"] or result["Proved"]:
                raise ValueError(f"unexpected proof response {result}")
            n, d = model.validate(result["Script"], cache)
            assertions += n
            domains += d
            attempts += check
        except Exception as exc:
            if args.failure:
                args.failure.write_text(script + "\n")
            raise RuntimeError(f"finite model witness failed at seed {seed}") from exc
        if i % 250 == 0 or i == args.count:
            writer.writerow([args.start, i, attempts, assertions, domains, native_sat,
                             native_unknown, f"{time.monotonic() - started:.2f}"])
            sys.stdout.flush()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--start", type=int, default=0)
    parser.add_argument("--count", type=int, default=250)
    parser.add_argument("--prove-every", type=int, default=25, help="0 skips actual proof attempts")
    parser.add_argument("--native", type=Path, help="optional Z3 executable to check source SAT")
    parser.add_argument("--native-every", type=int, default=500)
    parser.add_argument("--failure", type=Path, help="save the first failing query to this path")
    args = parser.parse_args()
    if args.count < 1 or args.start < 0 or args.native_every < 1 or args.prove_every < 0:
        parser.error("invalid seed range or sampling interval")
    with tempfile.TemporaryDirectory() as tmp:
        driver = build_driver(Path(tmp))
        command = [driver, "-test.run=^TestStudyOrderModelStream$", "-test.timeout=0"]
        with subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True,
                              env=dict(os.environ, GOMAXPROCS="1")) as proc:
            try:
                samples(args, proc)
            finally:
                proc.stdin.close()
                proc.stdout.read()
            if proc.wait():
                raise RuntimeError(f"Go driver exited with status {proc.returncode}")


if __name__ == "__main__":
    main()
