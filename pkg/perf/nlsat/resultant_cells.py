"""Check nonlinear conflicts on finite sets of roots with an exhaustive arithmetic oracle."""

import argparse
import itertools
import random
from pathlib import Path

from integer_model_cases import ROUTES, numeral, verify_model


def product(terms):
    result, *tail = terms
    for term in tail:
        result = f"(* {result} {term})"
    return result


def workload(seed):
    """Every variable equals a known integer times one common positive square root."""
    rng = random.Random(seed)
    width = 2 + seed % 2
    names = list("xyz"[:width])
    radicand = (1, 2, 3, 5)[seed // 2 % 4]
    domains, restrictions, polynomials = [], [], []
    for i, name in enumerate(names):
        weights = rng.sample((1, 2, 3), 2) if seed % 5 == 0 else [1 + (i if seed % 3 == 0 else 0)]
        factors = [f"(- (* {name} {name}) {w * w * radicand})" for w in weights]
        if seed % 17 == 0:
            factors.append(factors[0])
        polynomials.append(f"(= {product(factors)} 0)")
        domains.append([sign * w for w in weights for sign in (-1, 1)])
    # Homogeneous linear forms have the sign of their integer coefficient
    # after dividing by sqrt(radicand). Products take exact integer values.
    for j in range(1 + seed % 4):
        i, k = rng.sample(range(width), 2)
        a, b = rng.choice((-2, -1, 1, 2)), rng.choice((-2, -1, 1, 2))
        op = rng.choice(("<", "<=", "=", ">", ">=", "distinct"))
        if j % 2 == 0:
            body = f"(+ (* {numeral(a)} {names[i]}) (* {numeral(b)} {names[k]}))"
            value = lambda vs, a=a, b=b, i=i, k=k: a * vs[i] + b * vs[k]
            bound = 0
        else:
            body = f"(* {names[i]} {names[k]})"
            value = lambda vs, i=i, k=k: radicand * vs[i] * vs[k]
            bound = rng.choice((-2, -1, 0, 1, 2)) * radicand
        restrictions.append((f"({op} {body} {numeral(bound)})", value, op, bound))
    def accepts(values):
        for _, value, op, bound in restrictions:
            v = value(values)
            if not {"<": v < bound, "<=": v <= bound, "=": v == bound,
                    ">": v > bound, ">=": v >= bound, "distinct": v != bound}[op]:
                return False
        return True
    expected = "sat" if any(accepts(v) for v in itertools.product(*domains)) else "unsat"
    rows = polynomials + [r[0] for r in restrictions]
    rng.shuffle(rows)
    script = ["(set-logic QF_NRA)", *(f"(declare-const {x} Real)" for x in names),
              *(f"(assert {r})" for r in rows)]
    return "\n".join(script) + "\n", names, expected


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("seed", type=int)
    parser.add_argument("--route", choices=ROUTES, default="ordinary")
    parser.add_argument("--verify", nargs=2, type=Path, metavar=("G3", "NATIVE_Z3"))
    parser.add_argument("--count", type=int, default=1)
    args = parser.parse_args()
    if args.count < 1 or args.count != 1 and not args.verify:
        parser.error("generate one input or supply a positive count for verification")
    for seed in range(args.seed, args.seed + args.count):
        src, names, answer = workload(seed)
        if args.verify:
            n = verify_model(*[p.resolve() for p in args.verify], src, names,
                             ROUTES[args.route], answer, 90)
            print(f"{seed},{args.route},{answer},{n},pass", flush=True)
        else:
            print(src + ROUTES[args.route])


if __name__ == "__main__":
    main()
