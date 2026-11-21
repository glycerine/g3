"""Check the shared update guards independently with native Z3."""

import argparse
import gzip
import json
from pathlib import Path
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "conditional"))
from verify import parse, solve, substitute, walk


def verify(case, native):
    original, active = [case[key].splitlines() for key in ("original", "active")]
    if len(original) != len(active):
        raise ValueError("expected corresponding declarations and assertions")
    decls = [line for line in original if line.startswith("(declare-")]
    old_goal, new_goal = [parse(lines[-1])[1] for lines in (original, active)]
    guard = new_goal[-1]
    if new_goal[:-1] != old_goal or new_goal[0] != "and":
        raise ValueError("goal changed beyond the required update guard")

    def check(premises, expected):
        actual = solve(native, decls, premises)
        if expected != actual:
            raise ValueError(f"obligation {case['obligation']} expected {expected}, got {actual}")

    old_defs = {}
    for before, after in zip(original[:-1], active[:-1]):
        if before == after:
            continue
        old, new = parse(before)[1], parse(after)[1]
        if old[0] != "forall" or new[:2] != old[:2] or old[2][:2] != new[2][:2] or old[2][0] != "=":
            raise ValueError("expected the same universally defined application")
        lhs = old[2][1]
        old_defs[lhs[0]] = (lhs[1:], old[2])
        # Check equivalence for all arguments while the guard holds.
        check([guard, old, ["not", new]], "unsat")
        check([guard, new, ["not", old]], "unsat")
        check([guard, old, new], "sat")
    ground = []
    for app in walk(old_goal):
        definition = old_defs.get(app[0])
        if definition:
            formals, equality = definition
            instance = substitute(equality, dict(zip(formals, app[1:])))
            if instance not in ground:
                ground.append(instance)
    if not old_defs or not ground:
        raise ValueError("missing changed definitions or ground applications")
    # The original ground goal can hold. Whenever it does, the original
    # update equations alone imply that the transition took an active arm.
    check([old_goal, *ground], "sat")
    check([old_goal, *ground, ["not", guard]], "unsat")
    check([["not", guard]], "sat")
    print(f"Raft {case['obligation']} verified {len(old_defs)} total definitions and {len(ground)} ground applications")


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--native", required=True, type=Path)
    native = p.parse_args().native.resolve(strict=True)
    with gzip.open(HERE / "cases.json.gz", "rt") as f:
        for case in json.load(f):
            verify(case, native)
