#!/usr/bin/env python3
"""Verify saved Boolean substitutions against the native solver from this fork."""

import argparse
import csv
import gzip
import json
from pathlib import Path
import re
import subprocess
import sys


HERE = Path(__file__).resolve().parent
SYMBOL = re.compile(r"\bf\d+\b")
DEFINITION = re.compile(r"\(assert \(= (f\d+) (.*)\)\)")


def verify(z3, old, new):
    original, updated = old.splitlines(), new.splitlines()
    if len(original) != len(updated):
        raise ValueError("different assertion count")
    first = next(i for i, line in enumerate(original) if line.startswith("(assert "))
    if original[:first] != updated[:first]:
        raise ValueError("different declarations")
    changes = [(a, b) for a, b in zip(original[first:], updated[first:]) if a != b]
    aliases = {}
    for left, right in changes:
        for a, b in zip(SYMBOL.findall(left), SYMBOL.findall(right), strict=True):
            if a != b and aliases.setdefault(a, b) != b:
                raise ValueError("inconsistent substitution")
    for left, right in zip(original[first:], updated[first:]):
        if SYMBOL.sub(lambda m: aliases.get(m[0], m[0]), left) != right:
            raise ValueError("change beyond nullary function substitution")

    script = original[:first]
    expected = []

    def check(assertions, answer):
        script.extend(["(push)", *assertions, "(check-sat)", "(pop)"])
        expected.append(answer)

    definitions = {}
    for line in original[first:-1]:
        if match := DEFINITION.fullmatch(line):
            definitions.setdefault(match[1], {})[match[2]] = line
    for a, b in aliases.items():
        for name in (a, b):
            if f"(declare-fun {name} () Bool)" not in original[:first]:
                raise ValueError(f"{name} is not a nullary Boolean")
        body = next(r for r in definitions[a] if r in definitions[b])
        premises = [definitions[a][body], definitions[b][body]]
        different = f"(assert (distinct {a} {b}))"
        check([*premises, different], "unsat")
        for one in premises:
            check([one, different], "sat")
    script.extend(f"(assert (= {a} {b}))" for a, b in aliases.items())
    for a, b in changes:
        # Each line is exactly one assertion in the private script.
        check([f"(assert (distinct {a[len('(assert '):-1]} {b[len('(assert '):-1]}))"], "unsat")

    result = subprocess.run([z3, "-in", "-smt2"], input="\n".join(script) + "\n",
                            text=True, capture_output=True, check=True, timeout=60)
    if result.stdout.splitlines() != expected:
        raise ValueError(f"native checks failed: {result.stdout} {result.stderr}")
    return len(aliases), len(changes), expected.count("unsat"), expected.count("sat")


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--z3", type=Path, default=HERE.parents[3] / "build/z3")
    p.add_argument("--cases", type=Path, default=HERE / "queries.json.gz")
    args = p.parse_args()
    with gzip.open(args.cases, "rt") as stream:
        cases = json.load(stream)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["search", "aliases", "changed_assertions", "unsat_controls", "sat_controls"])
    for i, old, new in cases:
        writer.writerow([i, *verify(args.z3, old, new)])


if __name__ == "__main__":
    main()
