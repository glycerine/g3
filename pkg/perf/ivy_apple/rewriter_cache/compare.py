"""Measure cache lifetime costs or live heap per independent AST manager."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys


def measure(binary, cpu, memory):
    if memory:
        pattern, iterations = "BenchmarkASTActivityCacheWorkerMemory", 1
        cases = {pattern}
        metrics = ["workers", "entries/worker", "active-B/worker", "reset-B/worker"]
    else:
        pattern, iterations = "Benchmark(ASTActivityCacheLifetime|ActivityCacheCycle)", 100_000
        cases = {"BenchmarkASTActivityCacheLifetime/" + name for name in ("Empty", "OneEntry")}
        cases.update("BenchmarkActivityCacheCycle/" + name for name in ("Find", "InsertReset"))
        metrics = ["ns/op", "B/op", "allocs/op"]
    cmd = ["taskset", "-c", str(cpu), binary, "-test.run=^$", "-test.bench=^" + pattern + "$",
           f"-test.benchtime={iterations}x"]
    out = subprocess.check_output(cmd, env=dict(os.environ, GOMAXPROCS="1"), text=True)
    found = set()
    for line in out.splitlines():
        if not line.startswith("Benchmark"):
            continue
        case, n, *fields = line.split()
        if case in found or int(n) != iterations:
            raise ValueError(line)
        found.add(case)
        yield [case, n, *(fields[fields.index(unit) - 1] for unit in metrics)]
    if found != cases:
        raise ValueError(f"unexpected benchmark cases {found}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", action="append", nargs=2, required=True, metavar=("LABEL", "TEST_BINARY"))
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--rounds", type=int, default=8)
    parser.add_argument("--memory", action="store_true")
    args = parser.parse_args()
    binaries = {label: Path(name).resolve(strict=True) for label, name in args.go}
    if args.rounds < 1 or len(binaries) != len(args.go):
        parser.error("give each executable a unique label and request positive rounds")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    metrics = (["workers", "entries_per_worker", "active_bytes_per_worker", "reset_bytes_per_worker"]
               if args.memory else ["ns_per_op", "bytes_per_op", "allocs_per_op"])
    writer.writerow(["round", "cpu", "variant", "case", "iterations", *metrics])
    names = list(binaries)
    for i in range(args.rounds):
        offset = i % len(names)
        order = names[offset:] + names[:offset]
        if len(names) > 2 and i % 2:
            order.reverse()
        for label in order:
            for row in measure(binaries[label], args.cpu, args.memory):
                writer.writerow([i + 1, args.cpu, label, *row])
            sys.stdout.flush()


if __name__ == "__main__":
    main()
