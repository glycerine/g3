"""Rank solver checks by the paired g3-minus-native instruction estimate."""

import argparse
from collections import defaultdict
import csv
import gzip
from pathlib import Path
import re
from statistics import median
import sys


def transcript_labels(path):
    labels = {}
    headings = []
    for line in path.read_text().splitlines():
        text = line.strip()
        if text == "PASS":
            if not headings:
                raise ValueError(f"missing check label in {path}")
            desc = headings[-1][1]
            location = re.search(r"([^/ ]+\.ivy): line (\d+): (.*)", desc)
            if location:
                desc = f"{location[1]}:{location[2]} {location[3]}"
            parent = headings[-2][1] if len(headings) >= 2 else ""
            labels[len(labels)+1] = (parent, desc.removesuffix(" ..."))
        elif text:
            indent = len(line) - len(line.lstrip())
            while headings and headings[-1][0] >= indent:
                headings.pop()
            headings.append((indent, text))
    return labels


def estimate(path):
    costs = defaultdict(dict)
    with (gzip.open(path, "rt") if path.suffix == ".gz" else path.open()) as inp:
        for row in csv.DictReader(inp):
            check = int(row["check"])
            if check == 0:  # Samples outside calls have no obligation number.
                continue
            period, votes = int(row["period"]), int(row["samples"])
            value = int(row["estimated_instructions"])
            key = row["isolate"], check, int(row["round"])
            engine = row["engine"]
            if engine not in ("g3", "native") or engine in costs[key] or value != period * votes:
                raise ValueError(f"invalid or repeated measurement for {key}, {engine}")
            costs[key][engine] = value
    paired = defaultdict(list)
    for (isolate, number, _), pair in costs.items():
        paired[isolate, number].append((pair["g3"], pair["native"]))
    return paired


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("samples", type=Path, help="CSV from check_samples.py, optionally gzip-compressed")
    parser.add_argument("--transcript", action="append", nargs=2, default=[], metavar=("ISOLATE", "STDOUT"))
    parser.add_argument("--top", type=int, default=5, help="number of checks per isolate")
    args = parser.parse_args()
    if args.top <= 0:
        parser.error("--top must be positive")
    labels = {name: transcript_labels(Path(path)) for name, path in args.transcript}
    ranked = defaultdict(list)
    for (isolate, number), pairs in estimate(args.samples).items():
        delta = median(g-n for g, n in pairs)
        label = labels[isolate][number] if isolate in labels else ("", "")
        ranked[isolate].append((delta, number, len(pairs), median(g for g, _ in pairs),
                                median(n for _, n in pairs), label))
    out = csv.writer(sys.stdout, lineterminator="\n")
    out.writerow(["isolate", "rank", "check", "pairs", "g3_instructions", "native_instructions",
                  "paired_excess_instructions", "action", "obligation"])
    for isolate in sorted(ranked):
        if isolate in labels and len(labels[isolate]) != len(ranked[isolate]):
            raise ValueError(f"incomplete transcript or samples for {isolate}")
        for i, (diff, n, pairs, go, native, label) in enumerate(sorted(ranked[isolate], reverse=True)[:args.top]):
            out.writerow([isolate, i+1, n, pairs, go, native, diff, *label])


if __name__ == "__main__":
    main()
