"""Collect rotated before/after samples from the Boolean record study."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys

CASES = [
    ("scope", "SMTScopeRestorationWithQueue", 10000, 7),
    ("bcp", "SMT(BooleanPropagation|BooleanPropagationLargeIDs|ConflictResolution)", 30000, 7),
    ("record", "SMTBooleanRecord", 1000000, 3),
    ("bindings", "SMTBooleanBindingStorage", 10, 5),
    ("heap", "SMTScopeRestorationPeakIdle", 1, 3),
]
METRICS = ["ns/op", "pop-ns/op", "B/op", "allocs/op", "idle-B/solver", "fixture-B/solver",
           "B/record", "B/reason", "live-B/binding"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("before", type=Path)
    parser.add_argument("after", type=Path)
    parser.add_argument("native", type=Path, help="test-z3 executable")
    parser.add_argument("--maps", type=Path, help="optional sparse-map trial test binary")
    parser.add_argument("--cpu", default=6, type=int)
    parser.add_argument("--workers", default=48, type=int, help="Go processors for retained-heap runs")
    args = parser.parse_args()
    binaries = {"before": args.before, "after": args.after, "z3": args.native, "maps": args.maps}
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["series", "sample", "variant", "benchmark", "iterations", *METRICS])
    for series, bench, iterations, repeats in CASES:
        variants = ["before", "after"]
        if series == "scope":
            variants.append("z3")
        if series == "bindings" and args.maps:
            variants.append("maps")
        for sample in range(repeats):
            start = sample % len(variants)
            for variant in variants[start:] + variants[:start]:
                program = str(binaries[variant].resolve())
                if variant == "z3":
                    cmd = [program, "smt_scope_restoration", str(iterations)]
                else:
                    cmd = [program, "-test.run=^$", f"-test.bench=^Benchmark{bench}$",
                           f"-test.benchtime={iterations}x"]
                if series != "heap":
                    cmd = ["taskset", "-c", str(args.cpu), *cmd]
                env = dict(os.environ, GOGC="100", GOMAXPROCS=str(args.workers if series == "heap" else 1))
                output = subprocess.check_output(cmd, text=True, env=env)
                for line in output.splitlines():
                    if line.startswith("Benchmark"):
                        name, count, *fields = line.split()
                        metrics = dict(zip(fields[1::2], fields[::2]))
                        if variant == "z3":
                            metrics = {"pop-ns/op": fields[0]}
                        writer.writerow([series, sample + 1, variant, name, count,
                                         *(metrics.get(key, "") for key in METRICS)])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
