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
DIRECTORY = re.compile(r"(inside|outside|[1-9]\d*)-([1-9]\d*)-(g3|native)")


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


def rewriter_callees(path, engine):
    callees = Counter()
    total = 0
    for stack in samples(path):
        total += 1
        for i, symbol in enumerate(stack):
            if "solver rewriter" in categories(engine, symbol):
                callees[stack[i-1] if i else "(rewriter itself)"] += 1
                break
    if not total:
        raise ValueError(f"{path}: no samples")
    return total, callees


def operation_leaves(path, engine):
    counts = Counter()
    total = 0
    for stack in samples(path):
        total += 1
        for symbol in stack:
            matches = categories(engine, symbol)
            if matches:
                counts[matches[0], stack[0]] += 1
                break
    if not total:
        raise ValueError(f"{path}: no samples")
    return total, counts


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("runs", type=Path, nargs="+", help="completed directories named PHASE-PAIR-ENGINE")
    p.add_argument("--period", type=int, required=True, help="user-mode instructions per sample")
    view = p.add_mutually_exclusive_group()
    view.add_argument("--leaf", action="store_true", help="emit physical leaf symbols instead of operation groups")
    view.add_argument("--rewriter-callees", action="store_true",
                      help="attribute each sampled rewriter stack to its nearest physical callee")
    view.add_argument("--operation-leaves", action="store_true",
                      help="report physical leaves grouped by the nearest named solver operation")
    args = p.parse_args()
    if args.period < 1:
        p.error("a positive sampling period is required")
    out = csv.writer(sys.stdout, lineterminator="\n")
    columns = ["selection", "pair", "engine", "period", "samples", "attribution", "operation"]
    if args.operation_leaves:
        columns.append("leaf")
    out.writerow([*columns, "votes"])
    for run in args.runs:
        match = DIRECTORY.fullmatch(run.name)
        if not match:
            p.error(f"unrecognized run directory {run}")
        selection, pair, engine = match.groups()
        if args.operation_leaves:
            total, counts = operation_leaves(run / "stacks.gz", engine)
            for (owner, leaf), count in sorted(counts.items()):
                out.writerow([selection, pair, engine, args.period, total, "nearest", owner, leaf, count])
            continue
        if args.rewriter_callees:
            total, callees = rewriter_callees(run / "stacks.gz", engine)
            counts = [("rewriter callee", callees)]
        else:
            total, inclusive, nearest, leaf = summarize(run / "stacks.gz", engine)
            counts = [("leaf", leaf)] if args.leaf else [("inclusive", inclusive), ("nearest", nearest)]
        for kind, votes in counts:
            for name, count in sorted(votes.items()):
                out.writerow([selection, pair, engine, args.period, total, kind, name, count])


if __name__ == "__main__":
    main()
