"""Measure memory retained by independent solvers after popping a warmed scope."""

import argparse
import csv
import os
from pathlib import Path
import re
import subprocess
import sys


BENCH = "BenchmarkSMTScopeRestorationPeakIdle"
POPULATIONS = {(48, 2048), (1024, 128)}


def measure(binary, cpu):
    command = ["taskset", "-c", str(cpu), binary, "-test.run", "^$", "-test.bench",
               f"^{BENCH}$", "-test.benchtime=1x"]
    output = subprocess.check_output(command, env=dict(os.environ, GOMAXPROCS="1"), text=True)
    results = {}
    for line in output.splitlines():
        if not line.startswith("Benchmark"):
            continue
        name, count, *metrics = line.split()
        match = re.fullmatch(BENCH + r"/Workers(\d+)Atoms(\d+)(?:-\d+)?", name)
        if not match or count != "1":
            raise ValueError(f"unexpected benchmark row {line}")
        size = int(match[1]), int(match[2])
        if size in results:
            raise ValueError(f"duplicate worker population {size}")
        results[size] = [float(metrics[metrics.index(key)-1])
                         for key in ("fixture-B/solver", "idle-B/solver", "pop-ns/op")]
    if results.keys() != POPULATIONS:
        raise ValueError(f"incomplete worker populations from {binary}")
    return results


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", action="append", nargs=2, required=True, metavar=("LABEL", "TEST_BINARY"))
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=8)
    args = p.parse_args()
    binaries = {name: Path(path).resolve(strict=True) for name, path in args.go}
    if args.rounds < 1 or len(binaries) != len(args.go):
        p.error("choose unique build labels and positive rounds")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "cpu", "variant", "workers", "atoms", "fixture_bytes_per_solver",
                     "idle_bytes_per_solver", "pop_ns"])
    names = list(binaries)
    for i in range(args.rounds):
        offset = i % len(names)
        order = names[offset:] + names[:offset]
        if len(names) > 2 and i % 2:
            order.reverse()
        for name in order:
            for size, values in measure(binaries[name], args.cpu).items():
                writer.writerow([i+1, args.cpu, name, *size, *values])
            sys.stdout.flush()


if __name__ == "__main__":
    main()
