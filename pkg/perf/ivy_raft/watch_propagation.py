"""Measure standalone SAT watch propagation and check each reported SAT model."""

import argparse
import csv
import os
from pathlib import Path
import random
import re
import subprocess
import sys
import tempfile
import time

from propagation_dimacs import dimacs


def formulas(permutations=0):
    for width in (2, 3, 6):
        variables = 12_000
        clauses = [
            [-i, i + 1, *(-j for j in range(max(1, i - width + 2), i))]
            for i in range(1, variables)
        ]
        # The original run used this generator to draw candidate antecedents
        # before shuffling. Replay that draw to preserve the exact input order.
        rng = random.Random(25519 + width)
        for _ in range(1, variables):
            rng.sample(range(1, variables + 1), width - 2)
        rng.shuffle(clauses)
        clauses.append([1])
        yield f"propagate{width}", variables, clauses, "sat"

    for width, variables, count in ((3, 768, 2400), (5, 6000, 24_000), (8, 6000, 36_000)):
        rng = random.Random(920 + width)
        clauses = []
        for _ in range(count):
            clause = [v if rng.randrange(2) else -v for v in rng.sample(range(1, variables + 1), width)]
            if all(lit < 0 for lit in clause):
                clause[rng.randrange(width)] *= -1
            clauses.append(clause)
        yield f"planted{width}", variables, clauses, "sat"

    for holes in (7, 8):
        def literal(bird, hole):
            return 1 + bird * holes + hole

        clauses = [[literal(bird, hole) for hole in range(holes)] for bird in range(holes + 1)]
        for hole in range(holes):
            for bird in range(holes + 1):
                clauses.extend([[-literal(bird, hole), -literal(earlier, hole)] for earlier in range(bird)])
        yield f"pigeon{holes}", holes * (holes + 1), clauses, "unsat"
        variables = holes * (holes + 1)
        for seed in range(permutations):
            rng = random.Random(28000 + seed)
            names = sorted(range(1, variables + 1), key=lambda _: rng.random())
            renamed = [[names[abs(lit) - 1] if lit > 0 else -names[-lit - 1] for lit in clause]
                       for clause in clauses]
            for clause in renamed:
                rng.shuffle(clause)
            rng.shuffle(renamed)
            yield f"pigeon{holes}-s{seed}", variables, renamed, "unsat"


def check_output(output, expected, variables, clauses):
    lines = output.splitlines()
    if not lines or lines[0] != expected:
        raise ValueError(f"expected {expected}, got {lines[:1]}")
    if expected == "sat":
        model = set(map(int, lines[1].split()))
        if any(not 1 <= abs(lit) <= variables or -lit in model for lit in model):
            raise ValueError("invalid model assignment")
        if any(not any(lit in model for lit in clause) for clause in clauses):
            raise ValueError("reported model leaves a clause unsatisfied")
    return {key: int(value) for key, value in re.findall(r":([\w-]+)\s+(\d+)(?=[\s)])", output)
            if "memory" not in key and not key.endswith("time")}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("solvers", nargs="+", type=Path)
    parser.add_argument("--cpu", type=int)
    parser.add_argument("--perf", type=Path, help="Linux perf executable for user instructions and cycles")
    parser.add_argument("--rounds", type=int, default=5)
    parser.add_argument("--permutations", type=int, default=0,
                        help="additional deterministic variable, literal and clause orderings per pigeonhole input")
    parser.add_argument("--compare-heuristics", action="store_true",
                        help="allow different decision and conflict counts across solvers")
    parser.add_argument("--case", action="append", choices=[f"propagate{i}" for i in (2, 3, 6)]
                        + [f"planted{i}" for i in (3, 5, 8)] + ["pigeon7", "pigeon8"])
    args = parser.parse_args()
    if args.rounds < 1:
        parser.error("--rounds must be positive")
    if args.permutations < 0:
        parser.error("--permutations must be nonnegative")
    solvers = [solver.resolve() for solver in args.solvers]
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["case", "round", "solver", "answer", "instructions", "cycles", "seconds", "decisions", "conflicts"])
    with tempfile.TemporaryDirectory() as directory:
        for name, variables, clauses, expected in formulas(args.permutations):
            if args.case and name.split("-s")[0] not in args.case:
                continue
            path = Path(directory) / f"{name}.cnf"
            path.write_text(dimacs(variables, clauses))
            counts = Path(directory) / "counts.csv"
            reference = None
            for i in range(args.rounds):
                offset = i % len(solvers)
                for solver in solvers[offset:] + solvers[:offset]:
                    command = ["taskset", "-c", str(args.cpu)] if args.cpu is not None else []
                    if args.perf:
                        command.extend([args.perf, "stat", "-x,", "-o", counts, "-e", "instructions:u,cycles:u", "--"])
                    command.extend([solver, "-st", "-dimacs", path])
                    start = time.monotonic()
                    result = subprocess.run(command, capture_output=True, text=True, timeout=180,
                                            env=dict(os.environ, GOMAXPROCS="1"))
                    wall = time.monotonic() - start
                    if result.returncode not in {0, 10 if expected == "sat" else 20}:
                        raise RuntimeError((solver, name, result.returncode, result.stderr))
                    stats = check_output(result.stdout, expected, variables, clauses)
                    if expected == "unsat":
                        if not args.compare_heuristics and ("decisions" not in stats or "conflicts" not in stats):
                            raise ValueError(f"{solver} omitted search counters on {name}")
                        if not args.compare_heuristics and reference is not None and stats != reference:
                            raise ValueError(f"{solver} search counters on {name} differ from the reference")
                        reference = stats
                    hardware = {}
                    if args.perf:
                        with counts.open() as stream:
                            for row in csv.reader(stream):
                                if len(row) >= 5 and row[2] in ("instructions:u", "cycles:u") and float(row[4]) == 100:
                                    hardware[row[2]] = int(row[0])
                        if len(hardware) != 2:
                            raise ValueError(f"missing or multiplexed hardware counters on {name}")
                    writer.writerow([name, i + 1, solver, expected, hardware.get("instructions:u", ""),
                                     hardware.get("cycles:u", ""), f"{wall:.6f}",
                                     stats.get("decisions", 0), stats.get("conflicts", 0)])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
