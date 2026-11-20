"""Compare SAT propagation on counting contradictions and formulas with a planted model."""

import argparse
import csv
import itertools
from pathlib import Path
import random
import re
import subprocess
import sys
import tempfile
from time import perf_counter


def pigeonhole(holes):
    pigeons = holes + 1

    def variable(bird, hole):
        return bird * holes + hole + 1

    clauses = [[variable(bird, hole) for hole in range(holes)] for bird in range(pigeons)]
    for hole in range(holes):
        for a, b in itertools.combinations(range(pigeons), 2):
            clauses.append([-variable(a, hole), -variable(b, hole)])
    return dimacs(holes * pigeons, clauses)


def planted(variables):
    rng = random.Random(1929 + variables)
    model = [rng.randrange(2) for _ in range(variables)]
    clauses = []
    for _ in range(int(2.7 * variables)):
        row = rng.sample(range(variables), 3)
        signs = [rng.randrange(2) for _ in row]
        if all(sign != model[var] for sign, var in zip(signs, row)):
            i = rng.randrange(3)
            signs[i] = model[row[i]]
        clauses.append([v + 1 if sign else -v - 1 for sign, v in zip(signs, row)])
    return dimacs(variables, clauses)


def dimacs(variables, clauses):
    lines = [f"p cnf {variables} {len(clauses)}"]
    lines.extend(" ".join(map(str, row)) + " 0" for row in clauses)
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("solvers", type=Path, nargs="+", help="g3 or z3 executables with the DIMACS frontend")
    parser.add_argument("--holes", type=int, nargs="+", default=[7, 8])
    parser.add_argument("--planted", type=int, nargs="+", default=[180, 420, 1500])
    parser.add_argument("--rounds", type=int, default=5)
    parser.add_argument("--timeout", type=float, default=180)
    args = parser.parse_args()
    counters = ["decisions", "conflicts", "binary-propagations", "ternary-propagations", "propagations"]
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "solver", "case", "answer", "wall_s", *counters])
    with tempfile.TemporaryDirectory() as directory:
        cases = {}
        for holes in args.holes:
            path = Path(directory) / f"pigeon-{holes}.cnf"
            path.write_text(pigeonhole(holes))
            cases[path] = "unsat"
        for variables in args.planted:
            path = Path(directory) / f"planted-{variables}.cnf"
            path.write_text(planted(variables))
            cases[path] = "sat"
        searches = {}
        for i in range(args.rounds):
            offset = i % len(args.solvers)
            for solver in args.solvers[offset:] + args.solvers[:offset]:
                for path, expected in cases.items():
                    start = perf_counter()
                    run = subprocess.run([solver.resolve(), "-dimacs", "-st", path], capture_output=True,
                                         text=True, timeout=args.timeout)
                    elapsed = perf_counter() - start
                    if run.returncode not in {0, 10, 20} or run.stdout.splitlines()[:1] != [expected]:
                        raise RuntimeError((solver, path.stem, run.stderr))
                    stats = dict(re.findall(r":([\w-]+)\s+([\d.e+-]+)", run.stdout))
                    if "decisions" not in stats:
                        raise ValueError(f"missing search statistics from {solver}")
                    values = [int(stats.get(k, "0")) for k in counters]
                    if expected == "unsat" and searches.setdefault(path, values) != values:
                        raise ValueError(f"search differs on {path.stem} for {solver}")
                    writer.writerow([i + 1, str(solver), path.stem, expected, f"{elapsed:.6f}", *values])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
