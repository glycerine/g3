"""Measure and verify exact products and powers of real algebraic numbers."""

import argparse
from pathlib import Path

from integer_model_cases import expressions, query, render


OPERATIONS = ("product", "power", "inverse", "different", "nonbinomial")


def integer(n):
    return f"(- {-n})" if n < 0 else str(n)


def workload(count, degree, denominator, operation):
    names = [f"x{i}" for i in range(count)]
    lines = ["(set-option :produce-models true)", "(set-logic ALL)"]
    lines.extend(f"(declare-const {x} Real)" for x in names)
    terms = []
    for i, x in enumerate(names):
        value = 3 * i + 2
        if i % 2 and degree % 2:
            value = -value
        product = "(* " + " ".join([x] * degree) + ")"
        lhs = product if denominator == 1 else f"(* {denominator} {product})"
        if operation == "nonbinomial":
            # A positive root exists because the right side is positive.
            lhs = f"(+ {lhs} {x})"
            value = abs(value)
        bound = "<" if i % 2 and operation != "nonbinomial" else ">"
        lines.extend((f"(assert (= {lhs} {integer(value)}))", f"(assert ({bound} {x} 0))"))
        if operation in ("product", "nonbinomial"):
            terms.append(product)
        elif operation == "different":
            factors = [x] * (degree - 1) + [names[(i + 1) % count]]
            terms.append("(* " + " ".join(factors) + ")")
        else:
            exponent = degree if operation == "power" else -degree
            terms.append(f"(^ {x} {integer(exponent)})")
    return "\n".join(lines) + "\n", names, terms


def request(prefix, terms, repeat):
    return prefix + "(check-sat-using qfnra-nlsat)\n" + (
        "(get-value (" + " ".join(terms) + "))\n"
    ) * repeat


def verify(g3, z3, prefix, names, terms, timeout):
    requested = names + terms
    answer = expressions(query(g3, request(prefix, requested, 1), timeout))
    if len(answer) != 2 or answer[0] != "sat" or len(answer[1]) != len(requested):
        raise ValueError("incomplete model response")
    equations = []
    for expected, (actual, value) in zip(requested, answer[1]):
        if render(actual) != expected:
            raise ValueError(f"expected {expected}, received {render(actual)}")
        equations.append(f"(assert (= {expected} {render(value)}))")
    rebound = prefix + "\n".join(equations) + "\n(check-sat)\n"
    if query(z3, rebound, timeout) != "sat":
        raise ValueError("native solver rejected values in the original problem")
    return len(equations)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("variables", type=int)
    p.add_argument("--degree", type=int, default=2)
    p.add_argument("--denominator", type=int, default=1)
    p.add_argument("--operation", choices=OPERATIONS, default="product")
    p.add_argument("--repeat", type=int, default=1)
    p.add_argument("--verify", nargs=2, type=Path, metavar=("G3", "Z3"))
    p.add_argument("--timeout", type=int, default=90)
    a = p.parse_args()
    if a.variables < 2 or a.degree < 2 or min(a.denominator, a.repeat, a.timeout) < 1:
        p.error("use at least two variables and degree two, and positive remaining arguments")
    prefix, names, terms = workload(a.variables, a.degree, a.denominator, a.operation)
    if a.verify:
        n = verify(*[b.resolve() for b in a.verify], prefix, names, terms, a.timeout)
        print(f"native solver accepted {n} values")
    else:
        print(request(prefix, terms, a.repeat), end="")


if __name__ == "__main__":
    main()
