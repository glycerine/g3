"""Count instruction samples in the outermost named reduction pass during an external Push."""
import argparse
from collections import Counter
import csv
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from stack_costs import samples


PASSES = {
    "macro finding": ("(*SMTKernel).findPendingMacros", "asserted_formulas::find_macros_core("),
    "value propagation": ("(*SMTKernel).propagatePendingAssertions", "asserted_formulas::propagate_values("),
    "normal form": ("(*SMTKernel).normalizePendingAssertions", "asserted_formulas::nnf_cnf("),
    "pattern inference": ("(*SMTKernel).inferPendingPatterns", "asserted_formulas::pattern_inference_fn::simplify("),
    "bit-vector sharing": ("(*SMTKernel).maximizePendingBVSharing", "asserted_formulas::max_bv_sharing_fn::simplify("),
}


def classify(stack, engine):
    i = 0 if engine == "g3" else 1
    root = ("(*SMTKernel).reduceAssertedPending", "asserted_formulas::reduce()")[i]
    reducing = False
    for frame in reversed(stack):
        if root in frame:
            reducing = True
        if reducing:
            for name, patterns in PASSES.items():
                if patterns[i] in frame:
                    return name
    return "other reduction" if reducing else "outside assertion reduction"


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--isolate", required=True)
    p.add_argument("--engine", choices=("g3", "native"), required=True)
    p.add_argument("--period", type=int, required=True)
    p.add_argument("runs", nargs="+", type=Path, help="round directories from push_profile.py")
    args = p.parse_args()
    out = csv.writer(sys.stdout, lineterminator="\n")
    out.writerow(["isolate", "engine", "round", "period", "all_samples", "operation", "samples"])
    for run in args.runs:
        counts = Counter(classify(stack, args.engine) for stack in samples(run / "stacks.gz"))
        if not counts or args.period < 1:
            raise ValueError("positive sampling period and complete stack data required")
        for name, n in sorted(counts.items()):
            out.writerow([args.isolate, args.engine, int(run.name), args.period, sum(counts.values()), name, n])


if __name__ == "__main__":
    main()
