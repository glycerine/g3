"""Repeat a saved incremental query on one solver and compare every search counter."""
import argparse
import csv
import gzip
import os
from pathlib import Path
import re
import subprocess

from query_replay import CASES, HERE, NUMERIC_ANSWERS, query_inputs


def script(name, iterations, statistics):
    auxiliary = name in NUMERIC_ANSWERS
    directory = HERE.parent / "test_vectors"
    if auxiliary:
        directory /= "numeric_auxiliary"
    source = query_inputs(directory / (name + ".smt2.gz"), auxiliary)["incremental"]
    if source.count("(push 1)") != 1:
        raise ValueError(f"{name} has multiple incremental scopes")
    prefix, suffix = source.split("(push 1)")
    tail = "\n(get-info :all-statistics)\n" if statistics else "\n"
    return prefix + ("(push 1)" + suffix + tail + "(pop 1)\n") * iterations


def run(args, binary, name, stem, iterations, statistics):
    data = stem.with_suffix(".perf")
    command = ["taskset", "-c", str(args.cpu), args.perf, "stat", "-x", ",", "-e", "instructions:u",
               "-o", str(data), "--", binary, "-in", "-smt2"]
    result = subprocess.run(command, input=script(name, iterations, statistics), text=True,
                            capture_output=True, check=True, timeout=args.timeout,
                            env=dict(os.environ, GOMAXPROCS="1", GOGC=args.gogc))
    with gzip.open(stem.with_suffix(".out.gz"), "wt") as dest:
        dest.write(result.stdout)
    if "(error " in result.stdout or result.stderr.strip():
        raise ValueError(f"solver diagnostic in {stem}: {result.stderr[:256]}")
    chunks = re.split(r"(?m)^(sat|unsat|unknown)$", result.stdout)
    answers = chunks[1::2]
    if answers != [NUMERIC_ANSWERS.get(name, "unsat")] * iterations:
        raise ValueError(f"incorrect answers in {stem}")
    columns = ["decisions", "added-eqs" if name in NUMERIC_ANSWERS else "quant-instantiations",
               "conflicts", "rlimit-count"]
    observed = []
    if statistics:
        for block in chunks[2::2]:
            values = dict(re.findall(r":([\w-]+)\s+(\d+)\b", block))
            if columns[-1] not in values:
                raise ValueError(f"missing stats in {stem}")
            observed.append(tuple(int(values.get(key, 0)) for key in columns))
    with data.open() as file:
        counters = [row for row in csv.reader(file) if len(row) > 4 and row[2] == "instructions:u"]
    if len(counters) != 1 or float(counters[0][4]) != 100.0:
        raise ValueError(f"incomplete counter in {stem}")
    return int(counters[0][0]), observed


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("g3", "native", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    parser.add_argument("--queries", nargs="+", choices=(*CASES, *NUMERIC_ANSWERS), default=(*CASES, *NUMERIC_ANSWERS))
    parser.add_argument("--iterations", type=int, nargs="+", default=[1, 8, 64])
    parser.add_argument("--pairs", type=int, default=5)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--gogc", default="100")
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--timeout", type=int, default=240)
    args = parser.parse_args()
    if min(args.pairs, args.timeout, *args.iterations) < 1 or len(set(args.iterations)) != len(args.iterations):
        parser.error("supply positive, distinct iteration counts and positive pairs and timeout")
    binaries = {k: getattr(args, k).resolve(strict=True) for k in ("g3", "native")}
    args.output.mkdir(parents=True)
    with (args.output / "runs.csv").open("w") as full, (args.output / "search.csv").open("w") as checks:
        runs, searches = csv.writer(full), csv.writer(checks)
        runs.writerow(["query", "iterations", "pair", "engine", "cpu", "gogc", "instructions"])
        searches.writerow(["query", "engine", "iteration", "decisions", "added_eqs_or_instantiations",
                           "conflicts", "resource_steps", "matched", "gogc"])
        for query in args.queries:
            n = max(args.iterations)
            observed = {}
            for engine, binary in binaries.items():
                _, observed[engine] = run(args, binary, query, args.output / f"{query}-search-{engine}", n, True)
            for i in range(n):
                matched = int(observed["g3"][i] == observed["native"][i])
                for engine, outcomes in observed.items():
                    searches.writerow([query, engine, i+1, *outcomes[i], matched, args.gogc])
            checks.flush()
            for i in range(args.pairs):
                for count in args.iterations if i % 2 == 0 else reversed(args.iterations):
                    for engine in binaries if i % 2 == 0 else reversed(binaries):
                        stem = args.output / f"{query}-{count}-{i+1}-{engine}"
                        instructions, _ = run(args, binaries[engine], query, stem, count, False)
                        runs.writerow([query, count, i+1, engine, args.cpu, args.gogc, instructions])
                full.flush()


if __name__ == "__main__":
    main()
