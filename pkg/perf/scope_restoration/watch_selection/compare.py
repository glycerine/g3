"""Time the Go and native scope-restoration fixtures on the same CPU."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys


def samples(command, cpu, expected):
    result = subprocess.run(["taskset", "-c", str(cpu), *command],
                            env=dict(os.environ, GOMAXPROCS="1"), check=True,
                            capture_output=True, text=True)
    rows = {}
    for line in result.stdout.splitlines():
        if line.startswith("BenchmarkSMTScopeRestoration"):
            name, iterations, *tail = line.split()
            case = name.split("/")[1].split("-")[0]
            if case in rows or int(iterations) != expected:
                raise ValueError(f"unexpected benchmark row: {line}")
            metrics = dict(zip(tail[1::2], tail[::2]))
            rows[case] = (metrics.get("pop-ns/op", metrics["ns/op"]),
                          metrics["ns/op"] if "pop-ns/op" in metrics else "")
    if set(rows) != {"ScopedAtoms", "SurvivingAtoms"}:
        raise ValueError(f"incomplete benchmark output from {command[0]}")
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", action="append", nargs=2, metavar=("LABEL", "TEST_BINARY"), required=True)
    parser.add_argument("--native", type=Path, help="in-tree test-z3 executable")
    parser.add_argument("--cpus", nargs="+", type=int, required=True)
    parser.add_argument("--rounds", type=int, default=12)
    parser.add_argument("--iterations", type=int, default=5000)
    args = parser.parse_args()
    if args.rounds < 1 or args.iterations < 1:
        parser.error("rounds and iterations must be positive")
    variants = []
    for label, binary in args.go:
        for suite in ["WithQueue", "WithoutQueue"]:
            suffix = "WithQueue" if suite == "WithQueue" else ""
            command = [Path(binary).resolve(strict=True), "-test.run", "^$",
                       "-test.bench", "^BenchmarkSMTScopeRestoration" + suffix + "$",
                       f"-test.benchtime={args.iterations}x"]
            variants.append((label, suite, command))
    if args.native:
        variants.append(("z3", "Native", [args.native.resolve(strict=True), "smt_scope_restoration",
                                         str(args.iterations), "Both"]))
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "cpu", "variant", "fixture", "case", "pop_ns", "whole_ns"])
    for i in range(args.rounds):
        shift = i % len(variants)
        order = variants[shift:] + variants[:shift]
        if i % 2:
            order.reverse()
        for cpu in args.cpus:
            for label, fixture, command in order:
                for case, timings in samples(command, cpu, args.iterations).items():
                    writer.writerow([i + 1, cpu, label, fixture, case, *timings])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
