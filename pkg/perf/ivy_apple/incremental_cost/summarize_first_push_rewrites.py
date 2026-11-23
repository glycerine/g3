"""Summarize one g3 first_push_rewrite_rows.py --mode rows observation."""
import argparse
import csv
import gzip
import sys
from collections import Counter, defaultdict
from pathlib import Path


def summarize(stream):
    local = {}
    previous_push = None
    counts = defaultdict(Counter)
    for line in stream:
        if not line.startswith("CENSUS-ROW "):
            continue
        (_, push, stage, kind, _, options, substitution, entries,
         owner, slot, generation, result_slot, result_gen, steps) = line.split()
        if previous_push != push:
            local.clear()
            previous_push = push
        steps = int(steps)
        term = owner, slot, generation
        result = owner, result_slot, result_gen
        key = kind, options, substitution, term
        row = counts[stage, kind]
        row["calls"] += 1
        row["steps"] += steps
        row["unchanged"] += term == result
        if key in local:
            row["repeated_calls"] += 1
            row["repeated_steps"] += steps
            row["changed_answer"] += local[key] != result
            if local[key] == term:
                row["previously_unchanged_steps"] += steps
            if entries == "0":
                row["empty_substitution_steps"] += steps
        local[key] = result
    return counts


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("observations", type=Path)
    args = p.parse_args()
    with gzip.open(args.observations, "rt") as stream:
        rows = summarize(stream)
    if not rows:
        p.error("no g3 row observations in the supplied file")
    fields = ["calls", "steps", "unchanged", "repeated_calls", "repeated_steps",
              "changed_answer", "previously_unchanged_steps", "empty_substitution_steps"]
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["stage", "kind", *fields])
    for (stage, kind), counts in sorted(rows.items()):
        writer.writerow([stage, kind, *(counts[k] for k in fields)])


if __name__ == "__main__":
    main()
