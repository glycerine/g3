"""Prove each extra ground equality using the counterexample and point definitions."""

import argparse
import csv
import gzip
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "order_models"))
from finite_model import parse, smt
sys.path.insert(0, str(HERE.parent / "induction"))
from verify import archived_queries


def substitute(term, bindings):
    if isinstance(term, str):
        return bindings.get(term, term)
    if term[0] in ("forall", "exists"):
        raise ValueError("expected a ground instance of a quantifier-free definition")
    return [substitute(child, bindings) for child in term]


def solve(native, declarations, assertions):
    script = "\n".join([*declarations, *(f"(assert {smt(p)})" for p in assertions), "(check-sat)"])
    return subprocess.run([native, "-in", "-smt2"], input=script, text=True, capture_output=True,
                          check=True, timeout=10).stdout.strip()


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--native", type=Path, required=True)
    native = p.parse_args().native.resolve()
    with (HERE / "final-assertions.csv").open() as f:
        new = {int(r["obligation"]): parse(r["final_assertion"])[1] for r in csv.DictReader(f)}
    with gzip.open(HERE.parent / "transition/final-assertions.csv.gz", "rt") as f:
        previous = {int(r["obligation"]): parse(r["final_assertion"])[1] for r in csv.DictReader(f)}
    for number, original, _ in archived_queries():
        if number not in new:
            continue
        row = new.pop(number)
        earlier = previous[number]
        if row[:len(earlier)] != earlier:
            raise ValueError(f"obligation {number} changed the specialized goal")
        additions = row[len(earlier):]
        declarations = [l for l in original.splitlines() if l.startswith("(declare-")]
        roots = [parse(l)[1] for l in original.splitlines()[:-1] if l.startswith("(assert ")]
        assumptions = [earlier[1]]
        for fact in additions:
            if fact[0] != "=":
                continue
            site = fact[1]
            for q in roots:
                if q[0] != "forall" or q[2][0] != "=" or not isinstance(q[2][1], list):
                    continue
                lhs = q[2][1]
                if (lhs[0] == site[0] and len(lhs) == len(site) == len(q[1]) + 1
                        and set(lhs[1:]) == dict(q[1]).keys()):
                    assumptions.append(substitute(q[2], dict(zip(lhs[1:], site[1:]))))
                    break
            else:
                raise ValueError(f"missing point-update definition for {site}")
        if not additions or solve(native, declarations, assumptions) != "sat":
            raise ValueError(f"obligation {number} lacks a satisfiable positive control")
        for fact in additions:
            if solve(native, declarations, [*assumptions, ["not", fact]]) != "unsat":
                raise ValueError(f"obligation {number} failed to establish {smt(fact)}")
        print(f"{number} verified {len(additions)} ground consequences")
    if new:
        raise ValueError(f"unknown obligation numbers {list(new)}")


if __name__ == "__main__":
    main()
