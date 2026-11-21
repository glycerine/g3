"""Check point-update specializations with an independent native SMT solver."""

import argparse
import csv
import gzip
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "induction"))
from verify import archived_queries
from finite_model import parse, smt


def symbols(expr):
    if isinstance(expr, str):
        yield expr
    else:
        for child in expr:
            yield from symbols(child)


def substitute(expr, names):
    if isinstance(expr, str):
        return names.get(expr, expr)
    return [substitute(child, names) for child in expr]


def differences(left, right):
    if left == right:
        return
    if isinstance(left, list) and isinstance(right, list) and left[0] == right[0] and len(left) == len(right):
        for a, b in zip(left[1:], right[1:]):
            yield from differences(a, b)
    else:
        yield left, right


def unsat(native, decls, premises):
    query = "\n".join([*decls, *(f"(assert {smt(expr)})" for expr in premises), "(check-sat)"])
    run = subprocess.run([native, "-in", "-smt2", "-T:5"], input=query, text=True,
                         capture_output=True, check=True, timeout=10)
    return run.stdout.strip() == "unsat"


def verify(native, previous, current):
    before, after = previous.splitlines(), current.splitlines()
    if before[:-1] != after[:-1]:
        raise ValueError("changed a declaration or premise")
    original, special = parse(before[-1])[1], parse(after[-1])[1]
    if original == special:
        return None
    substitutions = list(differences(original, special))
    if any(not isinstance(x, str) or not isinstance(y, str) for x, y in substitutions):
        raise ValueError("expected a replacement of constants")
    (witness, pivot), = set(substitutions)
    if substitute(original, {witness: pivot}) != special:
        raise ValueError("partial substitution")

    decls = [line for line in before if line.startswith("(declare-")]
    sorts = {d[1]: d[3] for line in decls if (d := parse(line))[0] == "declare-fun" and not d[2]}
    if witness not in sorts or sorts.get(pivot) != sorts[witness]:
        raise ValueError("witness and pivot must be constants of the same sort")
    premises = [parse(line)[1] for line in before[:-1] if line.startswith("(assert ")]
    _, goal, lemma = original
    if original[0] != "and" or goal[0] != "not":
        raise ValueError("expected a ground counterexample and induction lemma")

    # Use only the ground instances of the defining equations at the changed
    # sites. Including all premises would make the benchmark trivially UNSAT.
    grounded = {}
    for _, site in differences(lemma, goal[1]):
        for premise in premises:
            if premise[0] != "forall" or premise[2][0] != "=":
                continue
            lhs = premise[2][1]
            if (isinstance(lhs, list) and isinstance(site, list) and lhs[0] == site[0]
                    and len(lhs) == len(site) == len(premise[1]) + 1 and set(lhs[1:]) == dict(premise[1]).keys()):
                equation = substitute(premise[2], dict(zip(lhs[1:], site[1:])))
                grounded[smt(equation)] = equation
    outside = [*grounded.values(), original, ["distinct", witness, pivot]]
    if unsat(native, decls, outside):
        return witness, pivot, "", len(grounded)
    used = set(symbols(premises))
    if witness in used:
        raise ValueError("a shared witness cannot exchange interpretations")
    for other in dict.fromkeys(symbols(original)):
        if other == witness or other in used or sorts.get(other) != sorts[witness]:
            continue
        swapped = substitute(original, {other: witness, witness: other})
        if (unsat(native, decls, [["distinct", original, swapped]])
                and unsat(native, decls, [*outside, ["distinct", other, pivot]])):
            return witness, pivot, other, len(grounded)
    raise ValueError("failed native frame or symmetry proof")


def main():
    args = argparse.ArgumentParser(description=__doc__)
    args.add_argument("--native", type=Path, required=True)
    native = args.parse_args().native.resolve()
    with gzip.open(HERE / "final-assertions.csv.gz", "rt") as source:
        specialized = {int(row["obligation"]): row["final_assertion"] for row in csv.DictReader(source)}
    out = csv.writer(sys.stdout, lineterminator="\n")
    out.writerow(["obligation", "witness", "pivot", "symmetric_partner", "ground_equations"])
    for number, _, previous in archived_queries():
        if number not in specialized:
            continue
        prefix, _, _ = previous.rpartition("(assert ")
        after = prefix + specialized.pop(number) + "\n"
        try:
            row = verify(native, previous, after)
            if row is None:
                raise ValueError("archived specialization has no change")
        except (ValueError, subprocess.SubprocessError) as err:
            raise RuntimeError(f"checking obligation {number}") from err
        out.writerow([number, *row])
    if specialized:
        raise ValueError("unmatched obligation numbers")


if __name__ == "__main__":
    main()
