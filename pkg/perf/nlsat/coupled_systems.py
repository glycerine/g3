"""Generate independent nonlinear systems and verify all reconstructed model values."""

import argparse
from pathlib import Path

from integer_model_cases import ROUTES, verify_model


SHAPES = (
    "pair", "parameter", "triple", "four", "permute", "negatives", "index-mismatch",
    "unique", "partial", "contradiction", "parameter-cross", "parameter-conflict",
    "nested", "mixed-contradiction", "triple-bad", "interleaved",
)
UNSAT = {"contradiction", "parameter-conflict", "triple-bad", "mixed-contradiction"}


def workload(groups, shape):
    """Return the original assertions and the names needed for independent model replay."""
    lines = ["(set-option :produce-models true)", "(set-logic QF_NRA)"]
    parameters = ["t"] if shape.startswith("parameter") else []
    if parameters:
        lines.extend(("(declare-const t Real)", "(assert (= (* t t) 2))", "(assert (> t 0))"))
    names, systems = [], []
    for i in range(groups):
        variables = [f"{letter}{i}" for letter in ("xyz" if shape.startswith("triple") else "xy")]
        names.extend(variables)
        lines.extend(f"(declare-const {v} Real)" for v in variables)
        x, y = variables[:2]
        rhs = "t" if parameters else "2"
        if shape == "unique":
            rhs = str(2 + i)
        elif shape == "four":
            rhs = str(2 + i % 4)
        sign = "<" if shape == "negatives" and i % 3 == 1 else ">"
        rows = [f"(= (* {x} {x}) {rhs})", f"({sign} {x} 0)",
                f"(= (* {x} {y}) 1)", f"({sign} {y} 0)"]
        if len(variables) == 3:
            z = variables[2]
            rows.extend((f"(= (+ (* {z} {z}) (* {x} {y})) 4)", f"(> {z} 0)"))
            if shape == "triple-bad" and i == 0:
                rows.append(f"(< {z} 0)")
        if shape == "permute" and i % 2:
            rows = rows[2:] + rows[:2]
        elif shape == "index-mismatch" and i == 1:
            rows[2] = f"(= (* {y} {y}) 1)"
        systems.append(rows)
    if shape == "interleaved":
        systems = zip(*systems)
    lines.extend(f"(assert {row})" for rows in systems for row in rows)
    if shape in ("partial", "parameter-cross"):
        lines.append("(assert (<= x0 x1))")
    elif shape in ("contradiction", "parameter-conflict"):
        lines.append("(assert (< x0 x1))")
    elif shape == "nested":
        lines.append("(assert (or (<= x0 x1) (not (= x2 x3))))")
    elif shape == "mixed-contradiction":
        lines.extend(("(assert (= y0 x1))", "(assert (> (* y0 y0) 3))"))
    return "\n".join(lines) + "\n", names + parameters


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("groups", type=int)
    parser.add_argument("--shape", choices=SHAPES, default="pair")
    parser.add_argument("--route", choices=ROUTES, default="ordinary")
    parser.add_argument("--values", action="store_true")
    parser.add_argument("--verify", nargs=2, type=Path, metavar=("G3", "NATIVE_Z3"))
    parser.add_argument("--timeout", type=int, default=180)
    args = parser.parse_args()
    if args.groups < 4 or args.timeout < 1:
        parser.error("at least four groups and a positive timeout are required")
    for shape in SHAPES if args.verify else (args.shape,):
        src, names = workload(args.groups, shape)
        expected = "unsat" if shape in UNSAT else "sat"
        if args.verify:
            n = verify_model(*[p.resolve() for p in args.verify], src, names,
                             ROUTES[args.route], expected, args.timeout)
            print(f"{args.groups},{shape},{args.route},{expected},{n},pass", flush=True)
        else:
            print(src + ROUTES[args.route])
            if args.values and expected == "sat":
                print("(get-value (" + " ".join(names) + "))")


if __name__ == "__main__":
    main()
