"""Select original defining equations reachable from an incremental verification goal."""

import argparse
from collections import defaultdict
import sys

from premise_slices import conjuncts, parse, smt, symbols


def expand_lets(term, bindings):
    if isinstance(term, str):
        return bindings.get(term, term)
    if term[0] == "let":
        local = {key: expand_lets(value, bindings) for key, value in term[1]}
        return expand_lets(term[2], {**bindings, **local})
    if term[0] in ("forall", "exists"):
        bound = dict(term[1])
        inner = {key: value for key, value in bindings.items() if key not in bound}
        return term[0], term[1], expand_lets(term[2], inner)
    return tuple(expand_lets(child, bindings) for child in term)


def definition_head(term, declarations):
    bound = set()
    while isinstance(term, tuple) and term[0] == "forall":
        bound.update(name for name, _ in term[1])
        term = term[2]
    if isinstance(term, tuple) and term[0] == "=" and len(term) == 3:
        lhs = term[1]
        if isinstance(lhs, str):
            if not bound and lhs in declarations:
                return lhs
        elif lhs[0] in declarations and len(lhs) - 1 == len(bound) and set(lhs[1:]) == bound:
            return lhs[0]
    return None


def select_definitions(text, depth):
    forms = parse(text)
    operations = [form[0] for form in forms if not form[0].startswith("declare-")
                  and form[0] not in ("set-logic", "set-option", "set-info")]
    if operations != ["assert", "push", "assert", "check-sat"]:
        raise ValueError("expected a base assertion followed by push, a goal assertion and check-sat")
    declarations = {form[1] for form in forms if form[0] in ("declare-fun", "declare-const")}
    base, goal = (list(conjuncts(form[1])) for form in forms if form[0] == "assert")
    definitions = defaultdict(list)
    uses = []
    for i, row in enumerate(base):
        term = expand_lets(row, {})
        uses.append(declarations.intersection(symbols(term)))
        head = definition_head(term, declarations)
        if head is not None:
            definitions[head].append(i)
    frontier = {name for row in goal for name in symbols(expand_lets(row, {})) if name in declarations}
    selected = set()
    for _ in range(depth):
        new = {i for name in frontier for i in definitions[name]} - selected
        if not new:
            break
        selected.update(new)
        frontier = set().union(*(uses[i] for i in new))

    # Only the index uses expanded terms. Emit the original conjuncts with
    # their original let scopes, declarations and order of assertion.
    groups = iter(([row for i, row in enumerate(base) if i in selected], goal))
    for form in forms:
        if form[0] == "assert":
            for row in next(groups):
                yield smt(("assert", row))
        else:
            yield smt(form)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--depth", type=int, required=True, help="number of definition layers")
    args = parser.parse_args()
    if args.depth < 0:
        parser.error("depth must be nonnegative")
    for line in select_definitions(sys.stdin.read(), args.depth):
        print(line)


if __name__ == "__main__":
    main()
