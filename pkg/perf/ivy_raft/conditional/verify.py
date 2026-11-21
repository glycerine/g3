"""Check the directional side conditions and the resulting point substitutions with native Z3."""

import argparse
import gzip
import json
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "order_models"))
from finite_model import parse, smt


def solve(native, declarations, assertions):
    commands = ["(set-option :smt.macro_finder true)", *declarations,
                *(f"(assert {smt(a)})" for a in assertions), "(check-sat)"]
    result = subprocess.run([native, "-in", "-smt2"], input="\n".join(commands), text=True,
                            capture_output=True, check=True, timeout=35)
    return result.stdout.strip()


def walk(term):
    if isinstance(term, list):
        yield term
        for child in term[1:]:
            yield from walk(child)


def substitute(term, bindings):
    if isinstance(term, str):
        return bindings.get(term, term)
    return [substitute(child, bindings) for child in term]


def verify(case, native):
    original = case["original"].splitlines()
    condition = case["condition"].splitlines()
    number = case["obligation"]
    source = set(original[:-1])
    if any(line not in source for line in condition[:-1]):
        raise ValueError(f"{number} added a premise to the side condition")
    declarations = [line for line in original if line.startswith("(declare-")]
    selected = [parse(line)[1] for line in condition[:-1] if line.startswith("(assert ")]
    exception = parse(condition[-1])[1]
    if exception[:1] != ["not"]:
        raise ValueError(f"{number} lacks a negated side condition")
    expected = "unsat" if case["established"] else "sat"
    if solve(native, declarations, selected) != "sat":
        raise ValueError(f"{number} lacks a satisfiable control for the selected premises")
    if solve(native, declarations, [*selected, exception]) != expected:
        raise ValueError(f"{number} expected {expected} for the exception")
    if not case["established"]:
        print(f"{number} verified fallback with a satisfiable exception")
        return

    # Recheck the specialization using only the emitted implications and
    # defining equations at the original ground accesses. The induction
    # hypothesis and other protocol premises cannot make this check vacuous.
    before = parse(original[-1])[1]
    after = parse(case["specialized"])[1]
    assumptions = [before, exception[1]]
    for line in original[:-1]:
        if not line.startswith("(assert (forall "):
            continue
        quantifier = parse(line)[1]
        equality = quantifier[2]
        if equality[0] != "=":
            continue
        lhs, rhs = equality[1:]
        if not isinstance(lhs, list) or not isinstance(rhs, list) or rhs[0] != "ite":
            continue
        variables = [v for v, _ in quantifier[1]]
        if len(variables) != len(lhs) - 1 or set(variables) != set(lhs[1:]):
            continue
        if any(t[0] in ("exists", "forall") for t in walk(rhs)):
            continue
        for site in walk(before):
            if site[0] == lhs[0]:
                assumptions.append(substitute(equality, dict(zip(lhs[1:], site[1:]))))
    if solve(native, declarations, assumptions) != "sat":
        raise ValueError(f"{number} lacks a satisfiable point-update control")
    if solve(native, declarations, [*assumptions, ["not", after]]) != "unsat":
        raise ValueError(f"{number} specialization did not follow")
    print(f"{number} verified both the side condition and the specialized goal")


def verify_inclusion(case, lifted, native):
    number = case["obligation"]
    ground = case["condition"].splitlines()
    universal = lifted.splitlines()
    if ground == universal:
        if case["established"]:
            raise ValueError(f"{number} unexpectedly retained its ground condition")
        return
    if not set(universal[:-1]) <= set(ground[:-1]):
        raise ValueError(f"{number} added a premise to the inclusion check")
    definitions = [parse(line)[1] for line in ground[:-1] if line not in universal[:-1]]
    declarations = [line for line in universal if line.startswith("(declare-")]
    premises = [parse(line)[1] for line in universal[:-1] if line.startswith("(assert ")]
    negated = parse(universal[-1])[1]
    exception = parse(ground[-1])[1]
    if negated[0] != "not" or negated[1][0] != "forall" or len(definitions) != 2:
        raise ValueError(f"{number} has an unexpected inclusion or definition")
    # Use only the two wrapper definitions to check the implication itself.
    # The protocol axioms could otherwise make an incorrect inference vacuous.
    inclusion = negated[1]
    for inputs, answer in [(premises, "sat"), ([*premises, negated], "unsat"),
                           ([*definitions, exception], "sat"), ([*definitions, inclusion], "sat"),
                           ([*definitions, inclusion, exception], "unsat")]:
        if solve(native, declarations, inputs) != answer:
            raise ValueError(f"{number} failed the inclusion control expecting {answer}")
    print(f"{number} verified pointwise inclusion and both ground consequences")


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--native", type=Path, required=True)
    native = p.parse_args().native.resolve()
    with gzip.open(HERE / "inclusion-conditions.json.gz", "rt") as f:
        inclusions = json.load(f)
    with gzip.open(HERE / "cases.json.gz", "rt") as f:
        for case in json.load(f):
            verify(case, native)
            verify_inclusion(case, inclusions[str(case["obligation"])], native)


if __name__ == "__main__":
    main()
