"""Classify each saved instruction sample by its nearest solver operation."""
import argparse
from collections import Counter
import csv
from functools import lru_cache
from pathlib import Path
import sys

from query_replay import HERE

sys.path.insert(0, str(HERE.parent))
from stack_costs import samples


def frame_classifier():
    rules = {"g3": [], "native": []}
    with (HERE / "query_stack_groups.csv").open() as file:
        for row in csv.DictReader(file):
            rules[row["engine"]].append((row["category"], row["match"], row["symbol"]))

    @lru_cache(None)
    def classify(engine, frame):
        for category, method, symbol in rules[engine]:
            matches = frame.endswith(symbol) if method == "suffix" else symbol in frame
            if matches:
                return category
        return None

    return classify


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path, help="query_stacks.py output directory")
    parser.add_argument("--period", required=True, type=int, help="sampling period used to record the stacks")
    args = parser.parse_args()
    if args.period < 1:
        parser.error("the sampling period must be positive")
    classify = frame_classifier()

    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["query", "engine", "round", "category", "samples", "period", "total"])
    traces = sorted(args.directory.glob("*-*-*/stacks.gz"))
    if not traces:
        parser.error("no recorded query stacks")
    for trace in traces:
        query, index, engine = trace.parent.name.rsplit("-", 2)
        histogram = Counter()
        for stack in samples(trace):
            category = next((c for frame in stack if (c := classify(engine, frame))), "unattributed")
            histogram[category] += 1
        total = sum(histogram.values())
        if not total:
            raise ValueError(f"no decodable samples in {trace}")
        for name, count in sorted(histogram.items()):
            writer.writerow([query, engine, int(index), name, count, args.period, total])


if __name__ == "__main__":
    main()
