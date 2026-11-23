"""Benchmark proving a disjunction of sibling counterexample conditions in one solver scope."""
import argparse
from collections import Counter
import csv
import gzip
import json
from pathlib import Path

from batch_core_replay import Shell
from batch_prefix_replay import prepare


def blocks(goals, width, check, bisect, counts):
    for start in range(0, len(goals), width):
        yield from resolve(goals[start:start + width], check, bisect, counts)


def resolve(goals, check, bisect, counts):
    result = check([goal for _, goal, _ in goals])
    if result not in ("unsat", "sat", "unknown"):
        raise RuntimeError(f"unexpected solver result {result!r}")
    if len(goals) == 1:
        counts["single_" + result] += 1
        yield goals[0], result, "single"
    elif result == "unsat":
        counts["proven_blocks"] += 1
        counts["proven_together"] += len(goals)
        for goal in goals:
            yield goal, result, "group"
    else:
        counts["fallback_" + result] += 1
        width = (len(goals) + 1) // 2 if bisect else 1
        yield from blocks(goals, width, check, bisect, counts)


def replay(args):
    decls, names, batches = prepare(args.events)
    selection = None
    if args.selection:
        with gzip.open(args.selection, "rt") as file:
            selection = {tuple(map(int, row)) for row in csv.reader(file)}
    args.output.mkdir(parents=True)
    shell = Shell(args, args.output, decls, mine=False)
    counters = Counter()
    try:
        with gzip.open(args.output / "checks.csv.gz", "wt") as file:
            writer = csv.writer(file, lineterminator="\n")
            writer.writerow(["segment", "base", "ordinal", "expected", "actual", "path"])
            for segment, (group, base, captured) in enumerate(batches):
                goals = [(i, g, ans) for i, (g, ans) in enumerate(captured)
                         if selection is None or (segment, i) in selection]
                if not goals:
                    continue
                shell.write("(push 1)")
                if base:
                    shell.write("(assert (and " + " ".join(names[f] for f in base) + "))")

                def check(ids):
                    term = names[ids[0]] if len(ids) == 1 else "(or " + " ".join(names[g] for g in ids) + ")"
                    answer = shell.ask("(push 1)\n(assert " + term + ")\n(check-sat)")
                    shell.write("(pop 1)")
                    return answer

                for (i, _, expected), answer, path in blocks(goals, args.width, check, args.bisect, counters):
                    writer.writerow([segment, group, i, expected, answer, path])
                    if answer != expected:
                        raise ValueError(f"segment {segment}, goal {i} returned {answer}, expected {expected}")
                    counters["checked"] += 1
                shell.write("(pop 1)")
        if selection is not None and len(selection) != counters["checked"]:
            raise ValueError("selection contains positions outside the input")
    finally:
        shell.close()
    (args.output / "summary.json").write_text(json.dumps(counters, indent=2) + "\n")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for argument in ("events", "solver", "output"):
        parser.add_argument("--" + argument, type=Path, required=True)
    parser.add_argument("--selection", type=Path, help="optional gzip CSV of zero-based (segment, goal) pairs")
    parser.add_argument("--width", type=int, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--bisect", action="store_true")
    args = parser.parse_args()
    if args.width < 1:
        parser.error("--width must be positive")
    replay(args)
