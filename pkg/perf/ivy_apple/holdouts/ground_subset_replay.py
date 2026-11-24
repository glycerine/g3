"""Check sibling goals against ground instances of their common premises."""
import argparse
from collections import defaultdict
import csv
from functools import lru_cache
from itertools import islice, product
from pathlib import Path
import subprocess
import sys

from batch_prefix_replay import conjuncts, parse, prepare, smt


@lru_cache(24000)
def ground(term):
    if isinstance(term, str):
        return True
    return bool(term) and term[0] not in ("forall", "exists", "lambda", "match", "let") and all(
        ground(arg) for arg in term[1:])


def body(term):
    while isinstance(term, tuple) and term and term[0] == "!":
        term = term[1]
    return term


def instantiate(term, bindings):
    term = body(term)
    if isinstance(term, str):
        return bindings.get(term, term)
    return (term[0], *(instantiate(arg, bindings) for arg in term[1:]))


def premises(terms):
    facts, rules = [], []
    for row in dict.fromkeys(x for term in terms for x in conjuncts(body(term))):
        row = body(row)
        if ground(row):
            facts.append(row)
            continue
        if row[0] == "not" and isinstance(row[1], tuple) and row[1][0] == "exists":
            row = ("forall", row[1][1], ("not", body(row[1][2])))
        if row[0] == "forall" and len(row[1]) <= 8 and ground(body(row[2])):
            rules.append((row[1], body(row[2])))
    return facts, rules


def pool(goal, sorts, limit):
    selected = defaultdict(list)
    todo, seen = [goal], set()
    while todo:
        term = todo.pop()
        if term in seen:
            continue
        seen.add(term)
        if isinstance(term, tuple):
            todo.extend(reversed(term[1:]))
        elif term in sorts:
            terms = selected[sorts[term]]
            if len(terms) < limit:
                terms.append(term)
    return tuple(sorted((s, tuple(sorted(ts))) for s, ts in selected.items()))


def check_group(args, declarations, facts, rules, constants, goals):
    by_sort = dict(constants)
    assertions = dict.fromkeys(facts)
    for variables, term in rules:
        choices = (by_sort.get(sort, ()) for _, sort in variables)
        for values in islice(product(*choices), args.tuples):
            bindings = dict(zip((name for name, _ in variables), values))
            assertions[instantiate(term, bindings)] = None
    commands = [f"(set-option :timeout {args.timeout_ms})", *declarations]
    commands.extend(f"(assert {smt(term)})" for term in assertions)
    for _, _, term in goals:
        commands.extend(("(push 1)", f"(assert {smt(term)})", "(check-sat)", "(pop 1)"))
    process = subprocess.run([args.solver, "-in", "-smt2"], input="\n".join(commands) + "\n",
                             capture_output=True, text=True,
                             timeout=120 + len(goals) * args.timeout_ms / 1000)
    answers = process.stdout.splitlines()
    if process.returncode or process.stderr or len(answers) != len(goals):
        raise ValueError(f"solver failed on ground subset: {process.stdout[:250]} {process.stderr[:250]}")
    return answers, len(assertions)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--events", type=Path, required=True, help="compressed incremental assertion stream")
    parser.add_argument("--solver", type=Path, required=True, help="SMT-LIB executable")
    parser.add_argument("--selection", type=Path, help="optional CSV with batch and term IDs")
    parser.add_argument("--batch", type=int, action="append", help="limit execution to these batches")
    parser.add_argument("--constants", type=int, default=3, help="observed constants per sort")
    parser.add_argument("--tuples", type=int, default=32, help="maximum instances per universal")
    parser.add_argument("--timeout-ms", type=int, default=1000, help="solver timeout per check")
    args = parser.parse_args()
    if min(args.constants, args.tuples, args.timeout_ms) <= 0:
        parser.error("limits must be positive")
    selected = None
    if args.selection:
        with args.selection.open(newline="") as stream:
            selected = {(int(r["batch"]), int(r["term"])) for r in csv.DictReader(stream)}
    prelude, _, batches = prepare(args.events)
    commands = [parse(row)[0] for row in prelude]
    definitions = [row[-1] for row in commands if row[0] == "define-fun"]
    declarations = [smt(row) for row in commands if row[0] != "define-fun"]
    uninterpreted = {row[1] for row in commands if row[0] == "declare-sort" and row[2] == "0"}
    sorts = {row[1]: row[3] for row in commands
             if row[0] == "declare-fun" and not row[2] and row[3] in uninterpreted}
    output = csv.writer(sys.stdout, lineterminator="\n")
    output.writerow(["batch", "term", "group", "group_goals", "facts", "asserted", "expected", "answer"])
    for batch, base, checks in batches:
        if args.batch and batch not in args.batch:
            continue
        facts, rules = premises(definitions[i] for i in base)
        groups = defaultdict(list)
        for i, expected in checks:
            if selected is not None and (batch, i) not in selected:
                continue
            term = definitions[i]
            if ground(term):
                groups[pool(term, sorts, args.constants)].append((i, expected, term))
        for group, (constants, goals) in enumerate(groups.items()):
            answers, count = check_group(args, declarations, facts, rules, constants, goals)
            for (i, expected, _), answer in zip(goals, answers):
                if answer not in ("sat", "unsat", "unknown") or answer == "unsat" and expected != "unsat":
                    raise ValueError(f"invalid subset result at batch {batch}, term {i}: {answer} / {expected}")
                output.writerow([batch, i, group, len(goals), len(facts), count, expected, answer])


if __name__ == "__main__":
    main()
