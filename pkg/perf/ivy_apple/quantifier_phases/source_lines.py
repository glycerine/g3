"""Assign each Go instruction sample to its physical leaf and one calling stage.

Use phase_stacks.py with --engines g3 --keep-binaries and a numeric --select.
Pass the matching output/bin/g3 executable and one or more stacks.gz files.
"""
import argparse
from collections import Counter
import csv
import gzip
from pathlib import Path
import re
import subprocess
import sys

FRAME = re.compile(r"^\s*([\da-fA-F]+)\s+(.+)\s+\((.+)\)\s*$")
MODULE = Path(__file__).resolve().parents[3]


def stage(frames):
    names = [frame[1] for frame in frames]

    def has(fragment):
        return any(fragment in name for name in names)

    if has("(*SMTCachedVarSubstitution).Instantiate"):
        return "substitution"
    if has("(*SMTQIInstanceBuilder).materialize") and has("(*TheoryRewriter).Rewrite"):
        return "rewrite instance"
    if has("(*SMTQIInstanceBuilder).MaterializeAndInternalize") and not has("(*SMTQIInstanceBuilder).materialize"):
        if has("internalize") or has("InternalizeQ") or has("InternalizeInstance"):
            return "internalize lemma"
    if has("(*SMTChecker)."):
        return "check candidate"
    if has("(*SMTQIInstanceBuilder).Finish"):
        return "construct lemma"
    if has("(*SMTQIQueueSchedule).instantiateNew"):
        return "other candidate work"
    return "outside candidates"


def samples(path):
    frames = None
    with gzip.open(path, "rt", errors="replace") as source:
        for line in source:
            if line.strip() == "instructions:u:":
                if frames is not None:
                    yield frames
                frames = []
            elif line.strip():
                match = FRAME.fullmatch(line.rstrip())
                if frames is None or match is None:
                    raise ValueError(f"{path}: unexpected perf line {line!r}")
                frames.append(match.groups())
    if frames is not None:
        yield frames


def source_name(location):
    filename, sep, line = location.rpartition(":")
    if not sep or not filename:
        return location
    path = Path(filename)
    if path.is_absolute():
        path = path.resolve()
        if path.is_relative_to(MODULE):
            return f"{path.relative_to(MODULE)}:{line}"
        if "/src/runtime/" in filename:
            return "runtime/" + filename.split("/src/runtime/", 1)[1] + ":" + line
    return path.name + ":" + line


def summarize(binary, path):
    counts = Counter()
    for frames in samples(path):
        leaf = frames[0] if frames else ("", "[no stack]", "")
        counts[(stage(frames), *leaf)] += 1
    if not counts:
        raise ValueError(f"no instruction samples in {path}")
    addresses = sorted({pc for _, pc, _, obj in counts if Path(obj).name == binary.name})
    rows = subprocess.check_output(["go", "tool", "addr2line", binary],
                                   input="".join("0x" + pc + "\n" for pc in addresses), text=True).splitlines()
    if len(rows) != 2 * len(addresses):
        raise ValueError("Go symbolizer returned an incomplete address list")
    locations = dict(zip(addresses, map(source_name, rows[1::2])))
    result = Counter()
    for (operation, pc, symbol, obj), hits in counts.items():
        location = locations[pc] if Path(obj).name == binary.name else ""
        result[operation, symbol, location] += hits
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--period", required=True, type=int)
    parser.add_argument("profiles", type=Path, nargs="+")
    args = parser.parse_args()
    if args.period < 1:
        parser.error("the sampling period must be positive")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["profile", "period", "total_samples", "stage", "leaf", "source", "samples"])
    for profile in args.profiles:
        counts = summarize(args.binary, profile)
        total = sum(counts.values())
        for (group, symbol, line), n in counts.most_common():
            writer.writerow([profile.parent.name, args.period, total, group, symbol, line, n])


if __name__ == "__main__":
    main()
