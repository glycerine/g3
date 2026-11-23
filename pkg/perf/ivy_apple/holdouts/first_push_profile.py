"""Classify perf-script stacks inside the observed first Push intervals of one checker."""
import argparse
from bisect import bisect_left
from collections import Counter
import csv
from decimal import Decimal
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from check_stacks import FRAME, HEADER

STAGES = [
    ('macro traversal', ('ast/macros.', 'macro_manager::', 'macro_util::', 'macro_finder::', 'macro_finder_tpl')),
    ('pattern inference', ('inferPendingPatterns', 'PatternInference', 'pattern_inference::', 'pattern_inference_cfg')),
    ('BV sharing', ('MaximizeBVSharing', 'maximize_bv_sharing::', 'max_bv_sharing')),
    ('nnf and CNF', ('NNF', 'nnf::', 'nnf::imp', 'CNFRewriter', 'cnf::', 'cnf::imp')),
    ('term ITE', ('TermITE', 'TermIte', 'elim_term_ite', 'elim_term_ite::', 'cofactor_term_ite')),
    ('quantifier lifting', ('PullQuant', 'PushQuant', 'pull_quant', 'push_quant', 'DistributeForall', 'distribute_forall', 'pull_nested_quant')),
    ('value propagation', ('PropagateValues', 'propagate_values', 'PropagateBVBounds', 'propagate_bv_bounds')),
    ('context simplification', ('CtxSimpl', 'ctx_simpl', 'ContextSimpl', 'SimplifyAsserted', 'simplify_asserted', 'SMTAssertedReducer', 'simplify_fmls')),
    ('preprocess formula', ('preprocessAssertionWithMode', 'SMTAssertedFormula', 'asserted_formulas::reduce', 'asserted_formulas::invoke', 'simplify_formula')),
    ('internalization', ('SMTInternalizer', 'SMTKernel).internalize', 'context::internalize', 'context::mk_bool_var')),
    ('construction', ('newSMTKernel', 'NewSMTKernel', 'newSMTSolver', 'NewSMTSolver', 'context::context(', 'Strategic', 'strategic_solver', 'tactic2solver')),
]


def classify(frames):
    for frame in frames:
        for stage, patterns in STAGES:
            if any(pattern in frame for pattern in patterns):
                return stage, frame
    return "other", "?"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--intervals", type=Path, required=True, help="instrumented checker stderr")
    parser.add_argument("--period", type=int, required=True)
    args = parser.parse_args()
    spans = []
    for line in args.intervals.read_text().splitlines():
        if line.startswith("W-INTERVAL "):
            _, group, phase, _, thread, start, end = line.split()
            if phase == "first":
                spans.append(tuple(map(int, (group, thread, start, end))))
    if not spans or args.period <= 0:
        parser.error("supply a positive period and a nonempty observation")
    ends = [s[3] for s in spans]
    if ends != sorted(ends):
        raise ValueError("unordered verification intervals")
    counts = Counter()
    group = None
    frames = []

    def finish():
        if group is None:
            return
        stage, function = classify(frames)
        for level, name in (("stage", stage), ("caller", function), ("leaf", frames[0] if frames else "?")):
            counts[group, level, stage, name] += 1

    for line in sys.stdin:
        header = HEADER.match(line)
        if header:
            finish()
            group, frames = None, []
            timestamp = int(Decimal(header[2]) * 1000000000)
            i = bisect_left(ends, timestamp)
            if i < len(spans):
                number, tid, begin, _ = spans[i]
                if begin <= timestamp and int(header[1]) == tid:
                    group = number
        elif group is not None:
            frame = FRAME.match(line)
            if frame:
                frames.append(frame[1])
    finish()
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["group", "level", "stage", "function", "samples", "period"])
    writer.writerows([*key, value, args.period] for key, value in sorted(counts.items()))


if __name__ == "__main__":
    main()
