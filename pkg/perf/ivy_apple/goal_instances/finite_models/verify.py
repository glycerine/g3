"""Build satisfiable Apple controls and verify their explicit model with both solvers."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import csv
import gzip
import itertools
from pathlib import Path
import random
import shutil
import subprocess

from syntax import conjuncts, fresh_names, parse, separate, smt, unlet

HERE = Path(__file__).resolve().parent
PKG = HERE.parents[3]
FIXTURES = HERE.parents[1] / "test_vectors"
# These captured workloads use uninterpreted sorts and nullary enumerations.
DEFAULT_CASES = ("cf_live-130", "cf_live-56", "dramc_nb2-369", "dramc_nb2-63", "dramc_nb2-71",
                 "rfn2-13", "rfn2-22", "rfn2-4")


def generate(forms, seed, width, with_witnesses=False, ground_booleans=False, ground_relations=False,
             constant_goals=False):
    rng = random.Random(seed)
    names = fresh_names(forms, "g3!model!")
    domains = {"Bool": ("false", "true")}
    functions = {}
    certificate, source, roots = [], [], []
    for form in forms:
        op = form[0]
        if op == "assert":
            roots.append(form[1])
            continue
        if op in ("check-sat", "set-info", "get-info"):
            continue
        source.append(form)
        if op == "declare-sort":
            sort = form[1]
            values = domains[sort] = tuple(next(names) for _ in range(width))
            certificate.append(("declare-datatypes", ((sort, "0"),),
                                (tuple((v,) for v in values),)))
        elif op == "declare-datatypes":
            certificate.append(form)
            for (sort, _), constructors in zip(form[1], form[2], strict=True):
                domains[sort] = tuple(c for (c,) in constructors)
                for c in domains[sort]:
                    functions[c] = c, {}
        elif op in ("declare-fun", "declare-const"):
            symbol, result = form[1], form[-1]
            inputs = form[2] if op == "declare-fun" else ()
            variables = tuple((next(names), s) for s in inputs)
            default = rng.choice(domains[result])
            table = {}
            for _ in range(min(8, 2 * len(inputs))):
                row = tuple(rng.choice(domains[s]) for s in inputs)
                answer = rng.choice(domains[result])
                if answer != default:
                    table[row] = answer
            value = default
            for key, answer in reversed(table.items()):
                tests = [("=", name, a) for (name, _), a in zip(variables, key, strict=True)]
                condition = tests[0] if len(tests) == 1 else ("and", *tests)
                value = ("ite", condition, answer, value)
            functions[symbol] = default, table
            certificate.append(("define-fun", symbol, variables, result, value))
        else:
            raise ValueError(f"unexpected command {op}")

    def evaluate(t, env):
        if isinstance(t, str):
            if t in env:
                return env[t]
            if t in ("true", "false"):
                return t
            return functions[t][0]
        op, *args = t
        if op in ("forall", "exists"):
            bindings, body = args
            keys = [name for name, _ in bindings]
            inputs = itertools.product(*(domains[s] for _, s in bindings))
            test = all if op == "forall" else any
            result = test(evaluate(body, {**env, **dict(zip(keys, row))}) == "true" for row in inputs)
        elif op == "ite":
            return evaluate(args[1] if evaluate(args[0], env) == "true" else args[2], env)
        elif op in ("and", "or"):
            test = all if op == "and" else any
            result = test(evaluate(child, env) == "true" for child in args)
        elif op == "=>":
            result = any(evaluate(a, env) == "false" for a in args[:-1]) or evaluate(args[-1], env) == "true"
        elif op == "not":
            child, = args
            result = evaluate(child, env) == "false"
        elif op == "xor":
            result = sum(evaluate(a, env) == "true" for a in args) % 2 == 1
        elif op in ("=", "distinct"):
            count = len({evaluate(a, env) for a in args})
            result = count == (1 if op == "=" else len(args))
        else:
            default, table = functions[op]
            return table.get(tuple(evaluate(a, env) for a in args), default)
        return "true" if result else "false"

    premises = [p for root in roots[:-1] for p in conjuncts(root, {})]
    kept = [p for p in premises if evaluate(p, {}) == "true"]
    retained = len(kept)
    goal = unlet(roots[-1], {})
    sign = evaluate(goal, {})
    goal = goal if sign == "true" else ("not", goal)
    if with_witnesses:
        # Every generated model extends to these fresh predicates. Put the
        # quantifiers in the query so the private matcher visits each domain.
        guards = []
        for sort in [form[1] for form in forms if form[0] == "declare-sort"]:
            always, never, x = next(names), next(names), next(names)
            binder = ((x, sort),)
            for symbol, value in ((always, "true"), (never, "false")):
                source.append(("declare-fun", symbol, (sort,), "Bool"))
                certificate.append(("define-fun", symbol, binder, "Bool", value))
            guards.extend((("forall", binder, (always, x)),
                           ("not", ("forall", binder, (never, x))),
                           ("not", ("exists", binder, (never, x)))))
        goal = ("and", goal, *guards)
    if ground_booleans:
        # Retain the certified original goal as a premise. The new query spans
        # existing Boolean state variables whose values are fixed by the model.
        kept.append(goal)
        variables = [f[1] for f in forms if f[0] in ("declare-fun", "declare-const")
                     and f[-1] == "Bool" and (f[0] == "declare-const" or not f[2])]
        while len(variables) < 4:
            name, value = next(names), rng.choice(("true", "false"))
            variables.append(name)
            functions[name] = value, {}
            source.append(("declare-const", name, "Bool"))
            certificate.append(("define-fun", name, (), "Bool", value))
        a, b, c, d = rng.sample(variables, 4)
        queries = [("ite", a, ("=>", b, c), ("not", d)),
                   ("xor", a, b, c, d),
                   ("=", ("or", a, b), ("and", c, d)),
                   ("=>", ("distinct", a, b), ("ite", c, d, a))]
        goal = queries[(seed + width) % len(queries)]
        if evaluate(goal, {}) == "false":
            goal = ("not", goal)
    if ground_relations:
        kept.append(goal)
        sort = rng.choice([form[1] for form in forms if form[0] == "declare-sort"])
        objects = domains[sort]
        relation, x, y = next(names), next(names), next(names)
        args = ((x, sort), (y, sort))
        source.append(("declare-fun", relation, (sort, sort), "Bool"))
        # Vary the full interpretation, including empty, universal, equality,
        # strict directed cycles and arbitrary relations. In the source the
        # constants may alias and the relation has no built-in properties.
        mode = seed % 5
        table = {}
        rows = []
        for i, a in enumerate(objects):
            for j, b in enumerate(objects):
                holds = (False, True, i == j, j == (i + 1) % width, rng.choice((False, True)))[mode]
                if holds:
                    table[a, b] = "true"
                    rows.append(("and", ("=", x, a), ("=", y, b)))
        definition = ("or", *rows) if rows else "false"
        certificate.append(("define-fun", relation, args, "Bool", definition))
        functions[relation] = "false", table
        constants = [next(names) for _ in range(3)]
        for i, name in enumerate(constants):
            value = objects[i % width]
            source.append(("declare-const", name, sort))
            certificate.append(("define-fun", name, (), sort, value))
            functions[name] = value, {}
        signed = []
        for a, b in zip(constants, constants[1:] + constants[:1], strict=True):
            atom = (relation, a, b)
            positive = evaluate(atom, {}) == "true"
            signed.append(atom if positive else ("not", atom))
            guard = ("and", ("=", x, a), ("=", y, b))
            consequent = (relation, x, y) if positive else ("not", (relation, x, y))
            kept.append(("forall", args, ("=>", guard, consequent)))
        # The antecedent holds and the first postcondition fails in this
        # interpretation. Both source names and domain elements can alias.
        flag = next(names)
        source.append(("declare-const", flag, "Bool"))
        certificate.append(("define-fun", flag, (), "Bool", "true"))
        functions[flag] = "true", {}
        goal = ("not", ("=>", ("and", flag, signed[0]), ("and", ("not", signed[1]), signed[2])))
    if constant_goals:
        kept.append(goal)
        sort = rng.choice([form[1] for form in forms if form[0] == "declare-sort"])
        name = next(names)
        source.append(("declare-const", name, sort))
        certificate.append(("define-fun", name, (), sort, rng.choice(domains[sort])))
        for _ in range(8):
            pred, var = next(names), next(names)
            value = rng.choice(("true", "false"))
            source.append(("declare-fun", pred, (sort,), "Bool"))
            binder = ((var, sort),)
            certificate.append(("define-fun", pred, binder, "Bool", value))
            atom, instance = (pred, name), (pred, var)
            if value == "false":
                atom, instance = ("not", atom), ("not", instance)
            kept.extend((atom, ("forall", binder, instance)))
            # The outer OR holds even though one disjunct is false.
            kept.append(("or", ("not", atom), ("exists", binder, instance)))
        goal = ("and", ("not", "false"))
    assertions = [("assert", ("and", *kept)), ("assert", goal), ("check-sat",)]
    return source + assertions, certificate + assertions, len(premises), retained, sign


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--g3", required=True, type=Path, help="g3 SMT-LIB shell")
    parser.add_argument("--native", required=True, type=Path, help="Z3 shell from this fork")
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--jobs", default=2, type=int)
    parser.add_argument("--seeds", default=12, type=int)
    parser.add_argument("--case", nargs="+", help="fixture names without .smt2.gz")
    goals = parser.add_mutually_exclusive_group()
    goals.add_argument("--constant-goals", action="store_true",
                       help="test bounded subsets of universal premises and ground facts under a true goal")
    goals.add_argument("--ground-relations", action="store_true",
                       help="derive a ground binary relation query with a certified finite interpretation")
    goals.add_argument("--ground-booleans", action="store_true",
                       help="derive a ground Boolean query satisfied by the explicit model")
    goals.add_argument("--with-quantified-witnesses", action="store_true",
                        help="extend each model with satisfiable quantified goals over every uninterpreted sort")
    args = parser.parse_args()
    if min(args.seeds, args.jobs) <= 0:
        parser.error("--seeds and --jobs must be positive")
    root = args.output.resolve()
    root.mkdir(parents=True)
    main_go = root / "check.go"
    shutil.copyfile(HERE / "check.go.txt", main_go)
    proof = root / "check"
    subprocess.run(["go", "build", "-pgo=off", "-o", proof, main_go], cwd=PKG, check=True)
    cases = args.case or DEFAULT_CASES
    engines = [p.resolve(strict=True) for p in (args.g3, args.native)]

    def run(case):
        forms = separate(parse(gzip.decompress((FIXTURES / (case + ".smt2.gz")).read_bytes()).decode()))
        rows = []
        for seed in range(1, args.seeds + 1):
            for width in ((1, 2, 3) if args.ground_relations or args.constant_goals else (1, 2)):
                loc = root / f"{case}-{seed}-{width}"
                loc.mkdir()
                source, cert, total, retained, sign = generate(
                    forms, seed, width, args.with_quantified_witnesses, args.ground_booleans,
                    args.ground_relations, args.constant_goals)
                for fname, problem in (("source.smt2", source), ("certificate.smt2", cert)):
                    (loc / fname).write_text("\n".join(map(smt, problem)) + "\n")
                for engine in engines:
                    cmd = [engine, "-smt2", "rlimit=4000000", loc / "certificate.smt2"]
                    verdict = subprocess.check_output(cmd, text=True, timeout=90).strip()
                    if verdict != "sat":
                        raise ValueError(f"invalid certificate for {loc} with {engine}: {verdict}")
                subprocess.run([proof, loc / "source.smt2"], check=True, timeout=90)
                rows.append((case, seed, width, total, retained, sign, "sat", "sat", 0))
        return rows

    with ThreadPoolExecutor(args.jobs) as pool, (root / "results.csv").open("w") as stream:
        out = csv.writer(stream, lineterminator="\n")
        out.writerow(["case", "seed", "sort_size", "premises", "retained", "original_goal",
                      "g3_certificate", "z3_certificate", "private_refutation"])
        for results in pool.map(run, cases):
            out.writerows(results)
            stream.flush()


if __name__ == "__main__":
    main()
