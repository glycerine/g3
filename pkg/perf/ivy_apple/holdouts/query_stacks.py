"""Sample the incremental route of the short-obligation replay benchmark on Linux."""
import argparse
from collections import Counter
import csv
import gzip
import os
from pathlib import Path
import subprocess
import sys

from query_replay import CASES, HERE, NUMERIC_ANSWERS, query_inputs

sys.path.insert(0, str(HERE.parent))
from stack_costs import categories, samples


def profile(args, directory, source, engine, binary, answer):
    directory.mkdir()
    raw = directory / "perf.data"
    result = subprocess.run(
        ["taskset", "-c", str(args.cpu), args.perf, "record", "--no-buildid", "-m", "1024",
         "-e", "instructions:u", "-c", str(args.period), "--call-graph",
         "fp" if engine == "g3" else "dwarf,8192", "-o", raw, "--", binary, "-in", "-smt2"],
        input=source, text=True, capture_output=True, env=dict(os.environ, GOMAXPROCS="1"),
        timeout=120, check=True)
    if result.stdout.splitlines()[:1] != [answer] or "(error " in result.stdout or "lost" in result.stderr.lower():
        raise ValueError(f"{directory}: {result.stdout} {result.stderr}")
    decoded = subprocess.run([args.perf, "script", "--no-inline", "-i", raw, "-F", "event,ip,sym,dso"],
                             check=True, capture_output=True)
    if b"lost" in decoded.stderr.lower():
        raise ValueError(f"{directory}: {decoded.stderr.decode()}")
    trace = directory / "stacks.gz"
    with gzip.open(trace, "wb", compresslevel=1) as file:
        file.write(decoded.stdout)
    raw.unlink()
    counts = Counter()
    total = 0
    for stack in samples(trace):
        total += 1
        counts["leaf", stack[0] if stack else "[no stack]"] += 1
        for frame in set(stack):
            counts["inclusive", frame] += 1
        operation = next((op for frame in stack for op in categories(engine, frame)), "other")
        counts["nearest", operation] += 1
    if total == 0:
        raise ValueError(f"{directory}: no instruction samples")
    return total, counts


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("g3", "native", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--pairs", type=int, default=6)
    parser.add_argument("--period", type=int, default=710009)
    parser.add_argument("--queries", nargs="+", choices=(*CASES, *NUMERIC_ANSWERS, "empty"), default=CASES)
    parser.add_argument("--vectors", type=Path, default=HERE.parent / "test_vectors")
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--statistics", action="store_true", help="also query the final search statistics")
    args = parser.parse_args()
    if min(args.period, args.pairs) < 1:
        parser.error("supply a positive sampling period and pair count")
    args.output.mkdir(parents=True)
    binaries = {name: getattr(args, name).resolve(strict=True) for name in ("g3", "native")}
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["query", "round", "engine", "cpu", "period", "samples", "attribution", "symbol", "count"])
    for name in args.queries:
        auxiliary = name in NUMERIC_ANSWERS
        answer = NUMERIC_ANSWERS.get(name, "unsat")
        if name == "empty":
            source = "(set-logic ALL)\n(push 1)\n(check-sat)\n"
            answer = "sat"
        else:
            directory = args.vectors / "numeric_auxiliary" if auxiliary else args.vectors
            source = query_inputs(directory / (name + ".smt2.gz"), auxiliary)["incremental"]
            if args.statistics:
                source += "\n(get-info :all-statistics)\n"
        for index in range(args.pairs):
            for engine in (tuple(binaries) if index % 2 == 0 else tuple(reversed(binaries))):
                run = args.output / f"{name}-{index+1}-{engine}"
                total, counts = profile(args, run, source, engine, binaries[engine], answer)
                for (classification, symbol), count in sorted(counts.items()):
                    writer.writerow([name, index+1, engine, args.cpu, args.period, total, classification, symbol, count])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
