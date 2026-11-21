"""Verify the two partially instantiated consequences in each projected Raft query."""

import argparse
from collections import Counter
import gzip
import json
from pathlib import Path
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "conditional"))
from verify import parse, solve


def verify(original, projected, native):
    number = original["obligation"]
    if not original["established"]:
        if projected != original["original"]:
            raise ValueError("the satisfiable side condition must use the original query")
        print(f"Raft {number} retained its fallback query")
        return
    before = original["original"].splitlines()[:-1] + original["specialized"].splitlines()
    after = projected.splitlines()
    declarations = [l for l in before if l.startswith("(declare-")]
    if declarations != [l for l in after if l.startswith("(declare-")] or before[-1] != after[-1]:
        raise ValueError("changed declaration or goal")
    removed = list((Counter(before) - Counter(after)).elements())
    added = list((Counter(after) - Counter(before)).elements())
    if len(removed) != 2 or len(added) != 2:
        raise ValueError("expected two omitted definitions and two partial instances")
    for line in removed:
        q = parse(line)[1]
        if q[0] != "forall" or q[2][0] != "=" or q[2][2][0] not in ("forall", "exists"):
            raise ValueError("omitted premise was not a quantified definition")
    parents = [parse(l)[1] for l in before if l.startswith("(assert (forall ")]
    for line in added:
        q = parse(line)[1]
        matches = [p for p in parents if p[1] == q[1]]
        if len(matches) != 1 or q[0] != "forall":
            raise ValueError("missing unique source quantifier")
        parent, = matches
        for formulas, expected in [([parent], "sat"), ([["not", q]], "sat"),
                                   ([parent, ["not", q]], "unsat")]:
            if solve(native, declarations, formulas) != expected:
                raise ValueError(f"Raft {number} failed its {expected} implication control")
    print(f"Raft {number} verified both partial instances from their source premise")


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--native", required=True, type=Path)
    native = p.parse_args().native.resolve(strict=True)
    with gzip.open(HERE / "primary.json.gz", "rt") as f:
        primaries = json.load(f)
    with gzip.open(HERE.parent / "conditional/cases.json.gz", "rt") as f:
        for case in json.load(f):
            verify(case, primaries[str(case["obligation"])], native)
