"""Generate nonlinear SMT-LIB workloads or replay a solver's complete model in Z3."""

import argparse
import random
import re
import subprocess
from pathlib import Path


SHAPES = (
    "independent", "ordered", "scrambled", "norm", "outside", "shell", "mixed",
    "boolean", "coupled", "intervals", "ite", "fixed", "conflict", "latezero",
    "cycle", "linear", "fractional", "irrational", "large", "integers", "mixed-sorts",
    "power", "odd-power", "coupled-powers", "power-conflict", "power-irrational",
)
UNSAT = {"conflict", "latezero", "cycle", "power-conflict"}
ROUTES = {"ordinary": "(check-sat)", "qfnra": "(check-sat-using qfnra)",
          "nlsat-portfolio": "(check-sat-using qfnra-nlsat)"}


def numeral(n):
    return str(n) if n >= 0 else f"(- {-n})"


def workload(shape, count, seed):
    """Return the original assertions and all free names, in declaration order."""
    rng = random.Random(seed)
    names = [f"x{i}" for i in range(count)]
    logic = {"integers": "QF_NIA", "mixed-sorts": "QF_NIRA"}.get(shape, "QF_NRA")
    if "power" in shape:
        # This fork exposes the ^ extension in ALL.
        logic = "ALL"
    lines = ["(set-option :produce-models true)", f"(set-logic {logic})"]
    integer = [shape == "integers" or shape == "mixed-sorts" and i % 2 == 0
               for i in range(count)]
    lines.extend(f"(declare-const {x} {'Int' if integer[i] else 'Real'})"
                 for i, x in enumerate(names))
    order = list(names)
    if shape == "scrambled":
        rng.shuffle(order)
    forms = []
    for i, x in enumerate(order):
        y = order[(i + 1) % count]
        square = f"(* {x} {x})"
        product = f"(* {x} {y})"
        if shape in ("norm", "outside"):
            forms.append(f"(> (+ {square} (* {y} {y})) {5 if shape == 'norm' else 37})")
        elif shape == "shell":
            forms.append(f"(and (> {square} 5) (< {square} 20))")
        elif shape == "mixed":
            forms.append(f"(and (> {square} 2) ({'<' if i % 2 else '>'} {x} 0))")
        elif shape == "boolean":
            forms.append(f"(or (= {square} 17) (and (> {x} 0) (> {product} 3)))")
        elif shape == "coupled":
            forms.extend((f"(> {square} 2)", f"(> {product} 2)"))
        elif shape == "intervals":
            a = rng.choice((-1, 1)) * rng.randint(3, 90)
            forms.extend((f"(< {numeral(a - 2)} {x})", f"(>= {numeral(a + 1)} {x})",
                          f"(>= {square} {min(abs(a - 1), abs(a + 1)) ** 2})"))
        elif shape == "ite":
            forms.append(f"(> (ite (<= {x} {y}) {square} (- 0 {product})) 2)")
        elif shape == "fixed":
            # Generate a planted integer model with pairwise interactions.
            a, b = (i % 11) - 5, ((i + 1) % count % 11) - 5
            forms.extend((f"(= {numeral(a)} {x})",
                          f"(= (+ {product} {square}) {numeral(a * b + a * a)})"))
        elif shape in ("power", "power-conflict", "power-irrational"):
            forms.append(f"(> (^ {x} 2) 5)")
        elif shape == "odd-power":
            forms.append(f"(> (^ {x} 3) 19)" if i % 2
                         else f"(and (< {x} 0) (< (^ {x} 3) (- 5)))")
        elif shape == "coupled-powers":
            forms.append(f"(> (+ (^ {x} 2) (^ {y} 4)) 15)")
        elif shape == "linear":
            forms.append(f"(> {x} {i % 5})")
        elif shape == "mixed-sorts":
            xr = f"(to_real {x})" if integer[i] else x
            yr = f"(to_real {y})" if integer[(i + 1) % count] else y
            forms.append(f"(> (* {xr} {yr}) 2.0)")
        else:
            forms.append(f"(> {square} 2)")
    if shape in ("ordered", "scrambled"):
        forms.extend(f"(< {x} {y})" for x, y in zip(order, order[1:]))
    if shape == "cycle":
        forms.extend(f"(<= {x} {y})" for x, y in zip(order, order[1:]))
        forms.append(f"(< {order[-1]} {order[0]})")
    if shape == "conflict":
        forms.append("(<= (* x0 x0) 2)")
    if shape == "latezero":
        forms.append("(= x0 0)")
    if shape == "fractional":
        forms.append("(= x0 (/ 7.0 3.0))")
    if shape == "irrational":
        forms.extend(("(= (* x0 x0) 3)", "(> x0 0)"))
    if shape == "power-conflict":
        forms.append("(<= (^ x0 2) 5)")
    if shape == "power-irrational":
        forms.extend(("(= (^ x0 2) 7)", "(< x0 0)"))
    if shape == "large":
        forms.append("(= x0 9223372036854775808)")
    if shape in ("fixed", "intervals"):
        rng.shuffle(forms)
    lines.extend(f"(assert {form})" for form in forms)
    return "\n".join(lines) + "\n", names


def expressions(text):
    """Parse atoms and lists in get-value results, including exact root-obj numerals."""
    top = []
    stack = [top]
    for token in re.findall(r"\(|\)|[^()\s]+", text):
        if token == "(":
            child = []
            stack[-1].append(child)
            stack.append(child)
        elif token == ")":
            if len(stack) == 1:
                raise ValueError("unmatched closing parenthesis in model")
            stack.pop()
        else:
            stack[-1].append(token)
    if len(stack) != 1:
        raise ValueError("unterminated expression in model")
    return top


def render(expr):
    return "(" + " ".join(render(arg) for arg in expr) + ")" if isinstance(expr, list) else expr


def query(binary, smt, timeout):
    run = subprocess.run([str(binary), "-in", "-smt2"], input=smt, text=True,
                         capture_output=True, timeout=timeout)
    if run.returncode or run.stderr or "(error " in run.stdout:
        raise ValueError(f"{binary}: exit {run.returncode}: {run.stdout[:500]} {run.stderr[:500]}")
    return run.stdout.strip()


def verify(solver, native, shape, route, count, seed, timeout):
    preamble, names = workload(shape, count, seed)
    expected = "unsat" if shape in UNSAT else "sat"
    request = preamble + ROUTES[route] + "\n"
    # Both historical default solvers can return unknown on mixed Int/Real
    # products. The explicitly selected polynomial portfolio supports this case.
    if shape == "mixed-sorts" and route == "ordinary":
        if query(solver, request, timeout) == "unknown":
            if query(native, request, timeout) != "unknown":
                raise ValueError("mixed-sort default solver verdict mismatch")
            return None
    return verify_model(solver, native, preamble, names, ROUTES[route], expected, timeout)


def verify_model(solver, native, preamble, names, command, expected, timeout):
    """Ask for every free value, then fix each one in the original problem in Z3."""
    request = preamble + command + "\n"
    if expected == "sat":
        request += "(get-value (" + " ".join(names) + "))\n"
    values = expressions(query(solver, request, timeout))
    if not values or values[0] != expected:
        raise ValueError(f"expected {expected}, got {str(values)[:180]}")
    if expected == "unsat":
        if values != ["unsat"] or query(native, preamble + "(check-sat)\n", timeout) != "unsat":
            raise ValueError("independent native verdict mismatch")
        return 0
    if len(values) != 2 or not isinstance(values[1], list):
        raise ValueError("missing get-value result")
    assignments = {}
    for pair in values[1]:
        if not isinstance(pair, list) or len(pair) != 2 or pair[0] in assignments:
            raise ValueError(f"malformed assignment {pair}")
        assignments[pair[0]] = render(pair[1])
    if set(assignments) != set(names):
        raise ValueError("get-value did not cover all variables")
    fixed = "".join(f"(assert (= {x} {assignments[x]}))\n" for x in names)
    if query(native, preamble + fixed + "(check-sat)\n", timeout) != "sat":
        raise ValueError("native rejected the returned assignments in the original problem")
    return len(assignments)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("variables", type=int)
    parser.add_argument("--shape", choices=SHAPES, default="independent")
    parser.add_argument("--route", choices=ROUTES, default="ordinary")
    parser.add_argument("--seed", type=int, default=2941)
    parser.add_argument("--values", action="store_true", help="print a get-value command")
    parser.add_argument("--verify", nargs=2, type=Path, metavar=("G3", "NATIVE_Z3"),
                        help="run all shapes, then check each returned assignment in native Z3")
    parser.add_argument("--timeout", type=int, default=60)
    args = parser.parse_args()
    if args.variables < 2 or args.timeout < 1:
        parser.error("at least two variables and a positive timeout are required")
    if args.verify:
        for shape in SHAPES:
            count = verify(*[p.resolve() for p in args.verify], shape, args.route,
                           args.variables, args.seed, args.timeout)
            if count is None:
                print(f"{shape},{args.route},{args.variables},{args.seed},0,matched-unknown", flush=True)
            else:
                print(f"{shape},{args.route},{args.variables},{args.seed},{count},pass", flush=True)
    else:
        preamble, names = workload(args.shape, args.variables, args.seed)
        print(preamble + ROUTES[args.route])
        if args.values and args.shape not in UNSAT:
            print("(get-value (" + " ".join(names) + "))")


if __name__ == "__main__":
    main()
