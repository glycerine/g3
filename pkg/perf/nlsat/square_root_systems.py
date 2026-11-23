"""Benchmark and independently verify systems with two possible real roots per variable."""

import argparse
from pathlib import Path

from integer_model_cases import ROUTES, verify_model


SHAPES = (
    "unique", "squares", "mixed", "repeated", "ordered", "order-bad", "sign-bad",
    "less", "compound", "backward", "backward-bad", "zero", "opposite",
    "mismatched", "negative-radicand", "boundary", "disjunction",
)
UNSAT = {"order-bad", "sign-bad", "backward-bad", "mismatched", "negative-radicand"}


def workload(shape, count):
    names = [f"x{i}" for i in range(count)]
    lines = ["(set-option :produce-models true)", "(set-logic QF_NRA)"]
    lines.extend(f"(declare-const {x} Real)" for x in names)
    for i, x in enumerate(names):
        c = 3 * i + 2
        if shape == "squares":
            c = (i + 1) ** 2
        elif shape == "repeated":
            c = 2
        elif shape == "zero" and i == 0:
            c = 0
        elif shape == "boundary":
            c = (2147483647, 2147395600, 2147395601, 1, 0)[i % 5]
        sign = "<" if shape in ("mixed", "less", "sign-bad", "opposite") and i % 2 else ">"
        lines.append(f"(assert (= (* {x} {x}) {c}))")
        if shape not in ("backward", "backward-bad"):
            if c == 0:
                lines.append(f"(assert (= {x} 0))")
            else:
                lines.append(f"(assert ({sign} {x} 0))")
    if shape in ("ordered", "order-bad", "backward", "backward-bad"):
        pairs = list(zip(names, names[1:]))
        if shape.startswith("backward"):
            pairs.reverse()
            lines.append("(assert (> x0 0))")
        lines.extend(f"(assert (< {x} {y}))" for x, y in pairs)
    if shape == "order-bad":
        lines.append(f"(assert (<= {names[-1]} x0))")
    elif shape == "backward-bad":
        lines.append(f"(assert (<= {names[-1]} 0))")
    elif shape == "sign-bad":
        lines.append("(assert (> x1 0))")
    elif shape == "less":
        lines.append("(assert (< x1 x0))")
    elif shape == "compound":
        lines.append("(assert (> (* x1 x0) 1))")
    elif shape == "opposite":
        lines.extend(f"(assert (< x{i} x{i-1}))" for i in range(1, count, 2))
    elif shape == "mismatched":
        lines.append("(assert (= 3 (* x0 x0)))")
    elif shape == "negative-radicand":
        lines.append(f"(assert (= (* {names[-1]} {names[-1]}) (- 1)))")
    elif shape == "disjunction":
        lines.append("(assert (or (< x0 1) (> x1 1)))")
    return "\n".join(lines) + "\n", names


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("variables", type=int)
    p.add_argument("--shape", choices=SHAPES, default="unique")
    p.add_argument("--route", choices=ROUTES, default="ordinary")
    p.add_argument("--values", action="store_true")
    p.add_argument("--verify", nargs=2, type=Path, metavar=("G3", "Z3"))
    p.add_argument("--timeout", type=int, default=120)
    a = p.parse_args()
    if a.variables < 2 or a.timeout < 1:
        p.error("at least two variables and a positive timeout are required")
    for shape in SHAPES if a.verify else (a.shape,):
        preamble, names = workload(shape, a.variables)
        expected = "unsat" if shape in UNSAT else "sat"
        if a.verify:
            n = verify_model(*[f.resolve() for f in a.verify], preamble, names,
                             ROUTES[a.route], expected, a.timeout)
            print(f"{shape},{a.variables},{a.route},{expected},{n},pass", flush=True)
        else:
            print(preamble + ROUTES[a.route])
            if a.values and expected == "sat":
                print("(get-value (" + " ".join(names) + "))")


if __name__ == "__main__":
    main()
