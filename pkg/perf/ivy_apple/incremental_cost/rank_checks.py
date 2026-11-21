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


def measure_pairs(path, engines):
    costs = defaultdict(dict)
    with (gzip.open(path, "rt") if path.suffix == ".gz" else path.open()) as inp:
        rows = csv.DictReader(inp)
        gated = "count" in (rows.fieldnames or ())
        for row in rows:
            engine = row["engine"]
            if engine not in engines:
                continue
            check = int(row["check"])
            if check == 0:  # Samples outside calls have no obligation number.
                continue
            if gated:
                if row["event"] != "instructions" or int(row["enabled_ns"]) != int(row["running_ns"]):
                    raise ValueError("ranking requires unmultiplexed instruction counts")
                value = int(row["count"]) - int(row["empty"])
            else:
                value = int(row["estimated_instructions"])
                if value != int(row["period"]) * int(row["samples"]):
                    raise ValueError(f"unexpected estimate for check {check}")
            key = row["isolate"], row.get("cpu"), check, int(row["round"])
            if engine in costs[key]:
                raise ValueError(f"invalid or repeated measurement for {key}, {engine}")
            costs[key][engine] = value
    paired = defaultdict(list)
    for (isolate, _, number, _), pair in costs.items():
        paired[isolate, number].append(tuple(pair[e] for e in engines))
    return paired


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("samples", type=Path, help="CSV from check_samples.py or thread_checks.py, optionally gzipped")
    parser.add_argument("--engines", nargs=2, default=["g3", "native"], metavar=("BASE", "COMPARISON"))
    parser.add_argument("--transcript", action="append", nargs=2, default=[], metavar=("ISOLATE", "STDOUT"))
    parser.add_argument("--top", type=int, default=5, help="number of checks per isolate")
    args = parser.parse_args()
    if args.top <= 0 or len(set(args.engines)) != 2:
        parser.error("supply a positive --top and two distinct engines")
    labels = {name: transcript_labels(Path(path)) for name, path in args.transcript}
    ranked = defaultdict(list)
    for (isolate, number), pairs in measure_pairs(args.samples, args.engines).items():
        delta = median(g-n for g, n in pairs)
        label = labels[isolate][number] if isolate in labels else ("", "")
        ranked[isolate].append((delta, number, len(pairs), median(g for g, _ in pairs),
                                median(n for _, n in pairs), label))
    out = csv.writer(sys.stdout, lineterminator="\n")
    out.writerow(["isolate", "rank", "check", "pairs", *(e + "_instructions" for e in args.engines),
                  "paired_excess_instructions", "action", "obligation"])
    for isolate in sorted(ranked):
        if isolate in labels and len(labels[isolate]) != len(ranked[isolate]):
            raise ValueError(f"incomplete transcript or samples for {isolate}")
        for i, (diff, n, pairs, go, native, label) in enumerate(sorted(ranked[isolate], reverse=True)[:args.top]):
            out.writerow([isolate, i+1, n, pairs, go, native, diff, *label])


if __name__ == "__main__":
    main()
