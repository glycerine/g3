"""Generate polynomial component benchmarks and cross-check exact reconstructed models."""

import argparse
from pathlib import Path

from integer_model_cases import ROUTES, numeral, verify_model


SHAPES = (
    "square", "negative", "multi", "unique", "cubic", "quartic", "unsat",
    "mixed-var", "mixed-component", "coupled-bad", "guard-unsat", "signed-order",
    "rational-subset", "opposite-signs", "duplicates", "shared-parameter",
)
UNSAT = {"unsat", "coupled-bad", "guard-unsat"}


def workload(shape, count):
    names = [f"x{i}" for i in range(count)]
    lines = ["(set-option :produce-models true)", "(set-logic QF_NRA)"]
    lines.extend(f"(declare-const {x} Real)" for x in names)
    if shape == "shared-parameter":
        lines.extend(("(declare-const parameter Real)", "(assert (> parameter 1))",
                      "(assert (< parameter 3))"))
    for i, x in enumerate(names):
        sq = f"(* {x} {x})"
        expression = sq
        if shape == "cubic":
            expression = f"(* {x} {x} {x})"
        elif shape == "quartic":
            expression = f"(- (* {x} {x} {x} {x}) {sq})"
        elif shape == "mixed-var" and i:
            difference = f"(- {x} {names[i - 1]})"
            expression = f"(* {difference} {difference})"

        rhs = 2
        if shape == "multi":
            rhs += i % 7
        elif shape == "unique":
            rhs += 3 * i
        elif shape == "rational-subset" and not i:
            rhs = 4
        elif shape == "unsat":
            rhs = -1
        bound = "parameter" if shape == "shared-parameter" else numeral(rhs)
        equation = f"(assert (= {expression} {bound}))"
        negative = shape in ("negative", "signed-order") or shape == "opposite-signs" and i % 2
        sign = f"(assert ({'<' if negative else '>'} {x} 0))"
        if shape == "duplicates" and i % 2:
            lines.extend((sign, equation, sign))
        else:
            lines.append(equation)
            if shape != "unsat":
                lines.append(sign)
    if shape == "mixed-component":
        lines.extend(("(assert (> (+ x0 x1) 2))", "(assert (= x2 x3))"))
    elif shape == "coupled-bad":
        lines.append("(assert (< x0 x1))")
    elif shape == "guard-unsat":
        lines.append(f"(assert (> {names[-1]} 9))")
    elif shape == "signed-order":
        lines.append("(assert (>= x1 x0))")
    elif shape == "rational-subset":
        lines.append("(assert (< x0 3))")
    if shape == "shared-parameter":
        names.append("parameter")
    return "\n".join(lines) + "\n", names


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("variables", type=int)
    parser.add_argument("--shape", choices=SHAPES, default="square")
    parser.add_argument("--route", choices=ROUTES, default="ordinary")
    parser.add_argument("--values", action="store_true")
    parser.add_argument("--verify", nargs=2, type=Path, metavar=("G3", "NATIVE_Z3"),
                        help="verify all shapes and rebind every model value in the original input")
    parser.add_argument("--timeout", default=90, type=int)
    args = parser.parse_args()
    if args.variables < 4 or args.timeout < 1:
        parser.error("at least four variables and a positive timeout are required")
    for shape in SHAPES if args.verify else (args.shape,):
        preamble, names = workload(shape, args.variables)
        if args.verify:
            expected = "unsat" if shape in UNSAT else "sat"
            matches = verify_model(*[p.resolve() for p in args.verify], preamble, names,
                                   ROUTES[args.route], expected, args.timeout)
            print(f"{shape},{args.route},{args.variables},{expected},{matches},pass", flush=True)
        else:
            print(preamble + ROUTES[args.route])
            if args.values and shape not in UNSAT:
                print("(get-value (" + " ".join(names) + "))")


if __name__ == "__main__":
    main()
