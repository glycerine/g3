"""Join per-operation thread counters to the first base Push of each solver."""
import argparse
from collections import defaultdict
import csv
import gzip
from pathlib import Path
from statistics import median
import sys


def read_rows(path):
    open_file = gzip.open if path.suffix == ".gz" else open
    with open_file(path, "rt", newline="") as source:
        yield from csv.DictReader(source)


def by_run(path):
    runs = defaultdict(list)
    for row in read_rows(path):
        runs[row["isolate"], int(row["pair"]), row["engine"]].append(row)
    return runs


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mutations", type=Path)
    parser.add_argument("overlap", type=Path)
    args = parser.parse_args()
    bases = {}
    for row in read_rows(args.overlap):
        key = row["isolate"], int(row["push"])
        if key in bases:
            raise ValueError(f"duplicate base sample {key}")
        bases[key] = row
    runs = by_run(args.mutations)
    costs = defaultdict(list)
    expected = {}
    for name, pair in sorted({(name, pair) for name, pair, _ in runs}):
        engines = [runs[name, pair, e] for e in ("g3", "native")]
        seq = [(r["sequence"], r["operation"]) for r in engines[0]]
        if not seq or seq != [(r["sequence"], r["operation"]) for r in engines[1]]:
            raise ValueError(f"unmatched operation sequence for {name}, pair {pair}")
        if expected.setdefault(name, seq) != seq:
            raise ValueError(f"operation order changed between runs of {name}")
        ordinal = 0
        for go, native in zip(*engines):
            if go["operation"] != "Push":
                continue
            ordinal += 1
            if any(r["enabled_ns"] != r["running_ns"] for r in (go, native)):
                raise ValueError(f"multiplexed counter for {name}, push {ordinal}")
            costs[name, ordinal].append(tuple(int(r["count"]) - int(r["empty"]) for r in (go, native)))
    if any(key not in costs for key in bases if key[0] in expected):
        raise ValueError("a base push falls outside the measured operation sequence")
    out = csv.writer(sys.stdout, lineterminator="\n")
    out.writerow(["isolate", "push", "pairs", "g3_instructions", "native_instructions", "median_paired_excess",
                  "first_base", "distinct_nodes", "seen_in_solver", "seen_in_context"])
    for (name, push), pairs in sorted(costs.items()):
        base = bases.get((name, push), {})
        go, native = zip(*pairs)
        out.writerow([name, push, len(pairs), median(go), median(native), median(g - n for g, n in pairs),
                      int(bool(base)), *(base.get(k, "") for k in ("distinct_nodes", "seen_in_solver", "seen_in_context"))])


if __name__ == "__main__":
    main()
