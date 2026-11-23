"""Replay a chronological stream of verification goals with one solver and scoped common premises."""
import argparse
from collections import Counter, defaultdict
import csv
import gzip
from pathlib import Path
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "goal_instances"))
from capture import standard_declarations
sys.path.insert(0, str(HERE.parent / "goal_instances" / "finite_models"))
from syntax import parse, smt


def expression(term, env, constants):
    if isinstance(term, tuple) and term and term[0] == "let":
        env = env.copy()
        while isinstance(term, tuple) and term and term[0] == "let":
            # All right-hand sides in an SMT let use the enclosing scope.
            env.update({k: expression(v, env, constants) for k, v in term[1]})
            term = term[2]
    if isinstance(term, str):
        return env.get(term, constants.get(term, term))
    if not term:
        return term
    op, *args = term
    if op in ("forall", "exists", "lambda"):
        local = dict(env, **{name: name for name, _ in args[0]})
        return op, args[0], expression(args[1], local, constants)
    if op == "as" and args[0] in constants:
        return constants[args[0]]
    if op == "!":
        out = [op, expression(args[0], env, constants)]
        for key, value in zip(args[1::2], args[2::2]):
            out.extend((key, expression(value, env, constants) if key in (":pattern", ":no-pattern") else value))
        return tuple(out)
    # Preserve an application head when the same name also denotes a nullary
    # declaration. Its argument sorts select the correct overload.
    head = expression(op, env, constants) if isinstance(op, tuple) else op
    return (head, *(expression(t, env, constants) for t in args))


def conjuncts(term):
    if isinstance(term, tuple) and term and term[0] == "and":
        for arg in term[1:]:
            yield from conjuncts(arg)
    else:
        yield term


def events(path):
    with gzip.open(path, "rt") as file:
        yield from csv.DictReader(file)


def prepare(path):
    csv.field_size_limit(sys.maxsize)
    signatures = defaultdict(set)
    prefix = "g3Batch!"
    for row in events(path):
        text = row["smt2"]
        if not text:
            continue
        while prefix in text:
            prefix += "x"
        for cmd in parse(standard_declarations(text)):
            if cmd[0] == "declare-fun":
                signatures[cmd[1]].add(cmd[2:])
    # An original question may use a spelling for a single constant, while a
    # different question uses it at another sort or arity. Give each constant
    # its own declaration when bringing both questions into the same scope.
    renamed = {}
    for name, sigs in sorted(signatures.items()):
        if len(sigs) > 1:
            for domain, sort in sorted(sigs, key=str):
                if not domain:
                    renamed[name, sort] = f"|{prefix}c{len(renamed)}|"

    decls, identifiers, terms = {}, {}, {}
    definitions, batches = [], []
    base = []
    goals = None
    group = None

    def intern(term):
        if term not in identifiers:
            identifiers[term] = len(definitions)
            definitions.append(smt(term))
        return identifiers[term]

    for row in events(path):
        tag = row["event"]
        number = int(row["batch_or_term"])
        if tag in ("term", "base"):
            commands = parse(standard_declarations(row["smt2"]))
            replacements = {}
            for cmd in commands:
                if cmd[0] == "declare-fun" and not cmd[2] and (cmd[1], cmd[3]) in renamed:
                    value = renamed[cmd[1], cmd[3]]
                    if cmd[1] in replacements and replacements[cmd[1]] != value:
                        raise ValueError(f"ambiguous nullary declaration within a single capture: {cmd[1]}")
                    replacements[cmd[1]] = value
                    cmd = (cmd[0], value, *cmd[2:])
                if cmd[0].startswith(("declare", "define")):
                    decls[cmd] = None
            assertions = [expression(c[1], {}, replacements) for c in commands if c[0] == "assert"]
            if tag == "term":
                if len(assertions) != 1 or number in terms:
                    raise ValueError(f"invalid term {number}")
                term = assertions[0]
                terms[number] = intern(term), tuple(intern(t) for t in conjuncts(term))
            else:
                group, goals = number, None
                base = [intern(t) for a in assertions for t in conjuncts(a)]
        else:
            if number != group:
                raise ValueError(f"event outside its batch {number}")
            term, facts = terms[int(row["id"])]
            if tag == "assume":
                base.extend(facts)
                goals = None
            elif tag == "goal":
                if goals is None:
                    goals = []
                    batches.append((group, tuple(base), goals))
                goals.append((term, {-1: "unsat", 0: "unknown", 1: "sat"}[int(row["answer"])]))
            else:
                raise ValueError(f"unknown event {tag}")
    types = {k: None for k in decls if k[0] in ("declare-sort", "define-sort", "declare-datatypes")}
    prelude = [smt(t) for t in types] + [smt(t) for t in decls if t not in types]
    names = [f"|{prefix}p{i}|" for i in range(len(definitions))]
    prelude.extend(f"(define-fun {n} () Bool {body})" for n, body in zip(names, definitions))
    return prelude, names, batches


def pick(ordered, available):
    """Select a multiset in the order in which its elements originally occurred."""
    used = available.copy()
    result = []
    for key in ordered:
        if used[key] > 0:
            result.append(key)
            used[key] -= 1
    return result


def write_replay(data, policy, output, answers, only_push=False):
    prelude, names, batches = data
    output.write("\n".join(prelude) + "\n")
    keep = []
    history = []
    common_rows = 0
    retained_rows = 0

    def push():
        output.write("(push 1)\n")

    def pop(count):
        if count:
            output.write(f"(pop {count})\n")

    def assertion(rows):
        if rows:
            body = names[rows[0]] if len(rows) == 1 else "(and " + " ".join(names[i] for i in rows) + ")"
            output.write("(assert " + body + ")\n")

    push()
    for i, (_, base, goals) in enumerate(batches):
        available = Counter(base)
        # Keep exactly those outer scopes whose premises all occur in this base.
        levels = 0
        for layer in keep:
            counts = Counter(layer)
            if counts - available:
                break
            available -= counts
            levels += 1
        pop(len(keep) - levels)
        del keep[levels:]
        if policy.startswith("lookahead"):
            window = int(policy.removeprefix("lookahead"))
            common = Counter(base)
            if i % window == 0:
                pop(len(keep))
                keep.clear()
                available = Counter(base)
                for _, future, _ in batches[i+1:i+window]:
                    common &= Counter(future)
            else:
                common.clear()
            chunk = max(1, len(base))
        elif policy == "separate":
            common, chunk = Counter(), 1
        else:
            window, chunk = map(int, policy.split("x"))
            common = Counter(base) if len(history) >= window else Counter()
            for previous in history[-window:]:
                common &= previous
        retained_rows += sum(map(len, keep))
        promote = pick(base, common & available)
        for j in range(0, len(promote), chunk):
            layer = promote[j:j+chunk]
            push()
            assertion(layer)
            keep.append(layer)
        available -= Counter(promote)
        common_rows += len(base) - available.total()
        push()
        assertion(pick(base, available))
        if only_push:
            push()
            pop(1)
        else:
            for goal, answer in goals:
                push()
                assertion([goal])
                output.write("(check-sat)\n")
                answers.write(answer + "\n")
                pop(1)
        pop(1)
        history.append(Counter(base))
        if len(history) > 8:
            history.pop(0)
    pop(1 + len(keep))
    output.write('(echo "done")\n')
    answers.write("done\n")
    return len(batches), sum(len(b) for _, b, _ in batches), common_rows, retained_rows


POLICIES = ("separate", "1x32", "2x16", "2x64", "3x32", "4x64", "8x64", "lookahead2", "lookahead4")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--events", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--policies", nargs="+", choices=POLICIES, default=POLICIES)
    parser.add_argument("--only-push", action="store_true")
    args = parser.parse_args()
    data = prepare(args.events)
    args.output.mkdir(parents=True)
    with (args.output / "groups.csv").open("w") as file:
        writer = csv.writer(file, lineterminator="\n")
        writer.writerow(["policy", "batches", "conjuncts", "in_shared_scopes", "retained_from_previous"])
        for mode in args.policies:
            with gzip.open(args.output / (mode + ".smt2.gz"), "wt") as script, (args.output / (mode + ".answers")).open("w") as expected:
                writer.writerow([mode, *write_replay(data, mode, script, expected, args.only_push)])


if __name__ == "__main__":
    main()
