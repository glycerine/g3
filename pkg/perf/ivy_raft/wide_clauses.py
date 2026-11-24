"""Measure complete DIMACS loads and SAT checks as live clause storage grows."""

import argparse
import csv
import os
from pathlib import Path
import random
import subprocess
import sys
import tempfile
import time

from watch_propagation import check_output


def clauses(variables, width, count):
    rng = random.Random(11007)
    for i in range(count):
        row = [v if rng.randrange(2) else -v for v in rng.sample(range(1, variables + 1), width)]
        row[i % width] = abs(row[i % width])
        yield row


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--solver", nargs=2, metavar=("NAME", "PATH"), action="append", required=True)
    parser.add_argument("--case", nargs=2, metavar=("WIDTH", "CLAUSES"), type=int, action="append",
                        help="a clause width and population, repeatable")
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--cpu", type=int)
    parser.add_argument("--perf", type=Path, help="Linux perf executable for user instructions and cycles")
    args = parser.parse_args()
    variables = 1024
    cases = args.case or [(16, 16384), (64, 4096), (64, 16384), (64, 65536)]
    if args.rounds < 1 or any(not 1 <= width <= variables or count < 1 for width, count in cases):
        parser.error("width must be 1 to 1024 and counts and rounds must be positive")
    solvers = [(name, Path(path).resolve()) for name, path in args.solver]
    if len({name for name, _ in solvers}) != len(solvers):
        parser.error("each solver needs a distinct name")
    out = csv.writer(sys.stdout, lineterminator="\n")
    out.writerow(["width", "clauses", "round", "solver", "answer", "instructions", "cycles",
                  "wall_seconds", "decisions", "conflicts", "mk_clause"])
    with tempfile.TemporaryDirectory() as tmp:
        source = Path(tmp) / "clauses.cnf"
        counts = Path(tmp) / "hardware.csv"
        for width, count in cases:
            with source.open("w") as f:
                print("p cnf", variables, count, file=f)
                for row in clauses(variables, width, count):
                    print(*row, 0, file=f)
            checked_models = set()
            reference = {}
            for i in range(args.rounds):
                offset = i % len(solvers)
                for name, solver in solvers[offset:] + solvers[:offset]:
                    cmd = ["taskset", "-c", str(args.cpu)] if args.cpu is not None else []
                    if args.perf:
                        cmd.extend([args.perf, "stat", "-x,", "-o", counts, "-e", "instructions:u,cycles:u", "--"])
                    cmd.extend([solver, "-st", "-dimacs", source])
                    start = time.monotonic()
                    result = subprocess.run(cmd, capture_output=True, text=True, env=dict(os.environ, GOMAXPROCS="1"))
                    wall = time.monotonic() - start
                    lines = result.stdout.splitlines()
                    if result.returncode not in (0, 10) or len(lines) < 2 or lines[0] != "sat":
                        raise ValueError((name, width, count, result.returncode, lines[:1], result.stderr))
                    model = lines[1]
                    rows = () if model in checked_models else clauses(variables, width, count)
                    stats = check_output(result.stdout, "sat", variables, rows)
                    checked_models.add(model)
                    if reference.setdefault(name, stats) != stats:
                        raise ValueError(f"{name} had different search counters on repeat {i + 1}")
                    hardware = {}
                    if args.perf:
                        with counts.open() as f:
                            hardware = {r[2]: int(r[0]) for r in csv.reader(f) if len(r) >= 5
                                        and r[2] in ("instructions:u", "cycles:u") and float(r[4]) == 100}
                        if len(hardware) != 2:
                            raise ValueError(f"missing or multiplexed hardware counters for {name}")
                    out.writerow([width, count, i + 1, name, "sat", hardware.get("instructions:u", ""),
                                  hardware.get("cycles:u", ""), f"{wall:.6f}", stats.get("decisions", 0),
                                  stats.get("conflicts", 0), stats.get("mk-clause", 0)])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
