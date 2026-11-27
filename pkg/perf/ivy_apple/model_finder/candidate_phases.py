"""Count disjoint phases of candidate model construction in saved hybrid profiles."""
import argparse
from collections import Counter
import csv
import os
from pathlib import Path
import re
import subprocess
import sys

PHASES = {
    "adjust": ("(*SMTModelFinder).FixModel", "adjust_model(", "model_finder::fix_model("),
    "generate": ("(*SMTModelGenerator).BuildStateValues", "model_generator::mk_model("),
    "complete": ("(*ProtoModel).CompletePartialFunctions", "complete_partial_funcs("),
    "clean": ("(*ProtoModel).Cleanup", "proto_model::cleanup("),
    "compress": ("(*ProtoModel).Compress", "model_core::compress(", "model::compress("),
}


def summarize(text, anchor):
    counts = Counter()
    total = 0
    for sample in text.split("\n\n"):
        frames = re.findall(r"^\s+[\da-fA-F]+ (.*?)\+0x", sample, re.M)
        if not frames:
            continue
        total += 1
        for index, frame in enumerate(frames):
            if anchor in frame:
                children = "\n".join(frames[:index])
                phase = next((name for name, terms in PHASES.items()
                              if any(term in children for term in terms)), "other")
                counts[phase] += 1
                break
    return total, counts


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--perf", default="perf")
    parser.add_argument("profiles", type=Path, nargs="+")
    args = parser.parse_args()
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(("case", "round", "engine", "process_samples", *PHASES, "other"))
    for path in sorted(args.profiles):
        name, turn, engine = path.stem.rsplit("-", 2)
        anchor = {"g3": "(*SMTKernel).buildCandidateModelGenerator",
                  "z3": "smt::context::mk_proto_model("}[engine]
        stacks = subprocess.check_output([args.perf, "script", "--no-inline", "-i", path],
                                         env=dict(os.environ, DEBUGINFOD_URLS=""), text=True)
        total, counts = summarize(stacks, anchor)
        writer.writerow((name, turn, engine, total, *(counts[key] for key in PHASES), counts["other"]))


if __name__ == "__main__":
    main()
