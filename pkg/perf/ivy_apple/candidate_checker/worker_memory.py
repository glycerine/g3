"""Compare scoped quantifier memory in fresh processes, each with 1,024 independent solvers."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys


def measure(binary, cpu):
    command = ["taskset", "-c", str(cpu), binary, "-test.run", "^$", "-test.bench",
               "^BenchmarkSMTQuantifierCandidateWorkerMemory$", "-test.benchtime=1x"]
    output = subprocess.check_output(command, text=True, env=dict(os.environ, GOMAXPROCS="1"))
    rows = [line.split() for line in output.splitlines() if line.startswith("Benchmark")]
    if len(rows) != 1 or rows[0][1] != "1":
        raise ValueError(f"expected one worker population from {binary}")
    row = rows[0]
    workers = int(row[row.index("workers")-1])
    if workers != 1024:
        raise ValueError(f"unexpected worker count {workers}")
    return [workers, float(row[row.index("active-B/worker")-1]), float(row[row.index("popped-B/worker")-1])]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", action="append", nargs=2, required=True, metavar=("LABEL", "TEST_BINARY"))
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=8)
    args = p.parse_args()
    binaries = {k: Path(v).resolve(strict=True) for k, v in args.go}
    if len(binaries) != len(args.go) or args.rounds < 1:
        p.error("choose unique build labels and a positive round count")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "cpu", "variant", "workers", "active_bytes_per_worker", "popped_bytes_per_worker"])
    names = list(binaries)
    for i in range(args.rounds):
        offset = i % len(names)
        order = names[offset:] + names[:offset]
        if i % 2:
            order.reverse()
        for name in order:
            writer.writerow([i+1, args.cpu, name, *measure(binaries[name], args.cpu)])
            sys.stdout.flush()


if __name__ == "__main__":
    main()
