"""Attribute Linux perf instruction samples from phase_stacks.py to solver operations."""

import argparse
from collections import Counter
import csv
from functools import lru_cache
import gzip
from pathlib import Path
import re
import sys


# Match demangled frames from this fork. A stack can cross several boundaries.
OPERATIONS = {
    "quantifier queue": (
        ("(*SMTQIQueueSchedule).instantiate", "(*SMTQuantifierManager).instantiate"),
        ("smt::qi_queue::instantiate(",)),
    "candidate checker": (("(*SMTChecker).",), ("smt::checker::",)),
    "solver rewriter": (("ast/rewriter.(*rewriterEngine).",), ("rewriter_tpl<",)),
    "cached substitution": (("(*SMTCachedVarSubstitution).",),
                            ("cached_var_subst::",)),
    "relevance assignment": (("(*SMTRelevancyPropagator).assignmentEvent",),
                             ("smt::relevancy_propagator_imp::assign_eh(",)),
    "disjunction relevance": (("(*SMTRelevancyPropagator).propagateOr",
                                "(*SMTRelevancyPropagator).propagateAssertedOr"),
                               ("smt::or_relevancy_eh::operator()", "smt::relevancy_propagator_imp::propagate_or(")),
    "Boolean propagation": ((), ("smt::context::bcp(",)),
    "pattern matching": (("(*smtMAMImpl).", "(*smtMAMInterpreter)."),
                         ("smt::mam_impl::", "smt::interpreter::")),
    "congruence table": (("(*SMTCongruenceTable).", "(*smtCg"),
                         ("smt::cg_table::", "smt::cg_table_")),
    "model checker": (("(*SMTModelChecker).",), ("smt::model_checker::",)),
    "conflict resolution": (("(*SMTInternalizerState).ResolveConflict", "(*SMTConflictResolution)."),
                            ("smt::context::resolve_conflict(", "smt::conflict_resolution::")),
    "scope pop": (("(*SMTKernel).PopScope", "(*SMTInternalizerState).PopScope",
                   "(*SMTInternalizerState).popScope"),
                  ("smt::context::pop_scope",)),
    "application construction": (("ast.(*ASTManager).mkApp",), ("ast_manager::mk_app_core(",)),
    "Go garbage collection": (("runtime.gcDrain", "runtime.gcBgMark", "runtime.gcAssistAlloc",
                               "runtime.(*gcControllerState).", "runtime.scanObject"), ()),
}
FRAME = re.compile(r"^\s*[0-9a-fA-F]+\s+(.+)\s+\(.+\)\s*$")
DIRECTORY = re.compile(r"(inside|[1-9]\d*)-([1-9]\d*)-(g3|native)")


@lru_cache(maxsize=None)
def categories(engine, function):
    i = 0 if engine == "g3" else 1
    # PropagateCore coordinates many subsystems. Only Propagate is native bcp.
    if function.endswith("(*SMTInternalizerState).Propagate"):
        return ("Boolean propagation",)
    return tuple(name for name, matches in OPERATIONS.items()
                 if any(pattern in function for pattern in matches[i]))


def samples(path):
    current = None
    with gzip.open(path, "rt", errors="replace") as source:
        for line in source:
            if line.strip() == "instructions:u:":
                if current is not None:
                    yield current
                current = []
            elif line.strip():
                match = FRAME.fullmatch(line)
                if current is None or not match:
                    raise ValueError(f"{path}: unrecognized perf script output: {line!r}")
                current.append(match[1])
    if current is not None:
        yield current


def summarize(path, engine):
    inclusive, nearest, leaf = Counter(), Counter(), Counter()
    total = 0
    for stack in samples(path):
        total += 1
        operations = [op for symbol in stack for op in categories(engine, symbol)]
        inclusive.update(set(operations))
        nearest[operations[0] if operations else "other"] += 1
        leaf[stack[0] if stack else "[no stack]"] += 1
    if not total:
        raise ValueError(f"{path}: no samples")
    return total, inclusive, nearest, leaf


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("runs", type=Path, nargs="+", help="completed directories named PHASE-PAIR-ENGINE")
    p.add_argument("--period", type=int, required=True, help="user-mode instructions per sample")
    p.add_argument("--leaf", action="store_true", help="emit physical leaf symbols instead of operation groups")
    args = p.parse_args()
    if args.period < 1:
        p.error("a positive sampling period is required")
    out = csv.writer(sys.stdout, lineterminator="\n")
    out.writerow(["selection", "pair", "engine", "period", "samples", "attribution", "operation", "votes"])
    for run in args.runs:
        match = DIRECTORY.fullmatch(run.name)
        if not match:
            p.error(f"unrecognized run directory {run}")
        selection, pair, engine = match.groups()
        total, inclusive, nearest, leaf = summarize(run / "stacks.gz", engine)
        counts = [("leaf", leaf)] if args.leaf else [("inclusive", inclusive), ("nearest", nearest)]
        for kind, votes in counts:
            for name, count in sorted(votes.items()):
                out.writerow([selection, pair, engine, args.period, total, kind, name, count])


if __name__ == "__main__":
    main()
