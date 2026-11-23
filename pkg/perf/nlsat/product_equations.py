"""Generate real product equations and independently check the reconstructed witnesses."""

import argparse
import random
from pathlib import Path

from integer_model_cases import ROUTES, numeral, verify_model


SHAPES = (
    "pairs", "signed", "chain", "triangle", "cycle", "cycle-bad", "sign-bad",
    "anchor-bad", "triangle-signs", "permuted", "bounds", "rational", "zero",
    "branch", "equalities", "large", "separate-conflict", "opposite",
    "negative-target", "mixed-core",
)
UNSAT = {"cycle-bad", "anchor-bad", "sign-bad", "separate-conflict"}


def equation(x, y, rhs):
    return f"(= (* {x} {y}) {numeral(rhs)})"


def sign(var, coefficient):
    return f"({'>' if coefficient > 0 else '<'} {var} 0)"


def workload(shape, groups, seed):
    rng = random.Random(seed)
    names, forms = [], []
    for i in range(groups):
        a, b, c, d = (rng.choice((-1, 1)) * rng.randint(1, 6) for _ in range(4))
        if shape in ("pairs", "bounds", "rational", "zero", "branch", "large",
                     "opposite", "negative-target"):
            a = b = c = d = 1
        if shape in ("opposite", "negative-target"):
            b = -1
        radicand = i + 2
        if shape == "rational":
            radicand **= 2
        elif shape == "large":
            radicand = 2_147_483_647 - 2 * i
        x, y, z, t = (f"{prefix}{i}" for prefix in "xyzt")
        names.extend((x, y))
        if shape in ("triangle", "triangle-signs"):
            names.append(z)
            if shape == "triangle":
                # x²=5a²/3, y²=3b²/5, z²=15c². No equation gives a square directly.
                rows = [equation(x, y, a*b), equation(y, z, 3*b*c),
                        equation(z, x, 5*c*a), sign(x, a)]
            else:
                rows = [equation(x, y, a*b), equation(y, z, b*c), equation(z, x, c*a),
                        sign(x, a), sign(y, b), sign(z, c)]
        elif shape in ("chain", "cycle", "cycle-bad", "mixed-core"):
            names.extend((z, t))
            rows = [equation(x, y, a*b), equation(y, z, b*c),
                    equation(z, t, c*d), sign(x, a)]
            if shape != "chain":
                rows.append(equation(t, x, d*a + int(shape == "cycle-bad" and i == 0)))
            if shape in ("chain", "mixed-core"):
                rows.append(equation(x, x, a*a*radicand))
            if shape == "mixed-core":
                rows.append(f"(or {sign(y, b)} (= {t} 0))")
        else:
            rows = [equation(x, x, a*a*radicand), equation(x, y, a*b),
                    sign(x, a), sign(y, b)]
            if shape == "zero":
                rows = [equation(x, x, radicand), equation(x, y, 0), sign(x, 1), f"(= {y} 0)"]
            elif shape == "bounds":
                rows.extend((f"(< {y} 1)", f"(<= {y} {x})"))
            elif shape == "branch":
                rows.append(f"(or (< {x} 1) (<= {y} {x}))")
            elif shape == "equalities" and i % 3 == 0:
                rows.append(f"(= {x} {x})")
            elif shape == "sign-bad" and i == 0:
                rows.append(sign(y, -b))
            elif shape == "anchor-bad" and i == 0:
                rows.append(equation(x, x, a*a*radicand + 1))
            elif shape == "separate-conflict" and i == groups - 1:
                rows.append(equation(x, y, a*b + 1))
            elif shape == "negative-target":
                rows.append(f"(> 0 {y})")
        if shape == "permuted":
            rng.shuffle(rows)
        forms.extend(rows)
    if shape == "permuted":
        rng.shuffle(forms)
    script = ["(set-option :produce-models true)", "(set-logic QF_NRA)"]
    script.extend(f"(declare-const {name} Real)" for name in names)
    script.extend(f"(assert {form})" for form in forms)
    return "\n".join(script) + "\n", names


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("groups", type=int)
    parser.add_argument("--shape", choices=SHAPES, default="pairs")
    parser.add_argument("--seed", type=int, default=21)
    parser.add_argument("--route", choices=ROUTES, default="ordinary")
    parser.add_argument("--values", action="store_true")
    parser.add_argument("--verify", nargs=2, type=Path, metavar=("G3", "Z3"))
    parser.add_argument("--timeout", type=int, default=120)
    args = parser.parse_args()
    if args.groups < 1 or args.timeout < 1:
        parser.error("groups and timeout must be positive")
    for shape in SHAPES if args.verify else (args.shape,):
        script, names = workload(shape, args.groups, args.seed)
        expected = "unsat" if shape in UNSAT else "sat"
        if args.verify:
            count = verify_model(*(p.resolve() for p in args.verify), script, names,
                                 ROUTES[args.route], expected, args.timeout)
            print(f"{shape},{args.seed},{args.groups},{args.route},{expected},{count},pass", flush=True)
        else:
            print(script + ROUTES[args.route])
            if args.values and expected == "sat":
                print("(get-value (" + " ".join(names) + "))")


if __name__ == "__main__":
    main()
