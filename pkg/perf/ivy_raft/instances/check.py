"""Compare direct universal instances against finite models and scoped contradictions."""

import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(HERE.parent / "order_models"))
from finite_model import Problem, evaluate


def instance(seed):
    width = 1 + seed % 5
    other = 1 + (seed // 5) % 4
    size = 1 << width
    a = (seed // 11) % size
    b = (a + 1 + (seed // 31) % (size - 1)) % size
    c = (seed // 7) % (1 << other)
    prefix = f"""(define-sort A () (_ BitVec {width}))(define-sort B () (_ BitVec {other}))
(declare-const a A)(declare-const b A)(declare-const c B)
(declare-fun h (A) A)(declare-fun r (A A B) Bool)(declare-fun q (A) Bool)
(assert (= a (_ bv{a} {width})))(assert (= b (_ bv{b} {width})))(assert (= c (_ bv{c} {other})))
(assert (distinct a b))(assert (= (h a) a))(assert (= (h b) b))
"""
    term = ["(h a)", "(h (h a))", "a"][seed % 3]
    diagonal = "(r x x z)"
    ground = f"(r {term} {term} c)"

    def all_x_z(body):
        return f"(forall ((x A)(z B)) {body})"

    # Interpret h as the identity. For each row the predicate r can depend
    # only on equality of its first two arguments. q can be constant or
    # distinguish the two named values. The two last columns respectively
    # preserve this model and contradict a required universal instance.
    cases = [
        (all_x_z(diagonal), "(not (r a b c))", f"(not {ground})"),
        (all_x_z(f"(not {diagonal})"), "(r a b c)", ground),
        (all_x_z(f"(=> (q x) {diagonal})"), "(not (=> (q a) (r a b c)))",
         f"(not (=> (q {term}) {ground}))"),
        (f"(not (exists ((x A)(z B)) {diagonal}))", "(r a b c)", ground),
        (all_x_z(f"(or {diagonal} (q x))"), "(and (not (r a a c)) (q a))",
         f"(not (or {ground} (q {term})))"),
        (all_x_z(f"(not (and {diagonal} (q x)))"), "(and (r a a c) (not (q a)))",
         f"(not (=> (and {ground} (q {term})) false))"),
        (f"(not (exists ((x A)(z B)) (not (= (q x) {diagonal}))))",
         "(not (= (q a) (r a b c)))", f"(not (= (q {term}) {ground}))"),
        (f"(and (exists ((x A)) (q x)) {all_x_z(diagonal)})", "(not (q a))", f"(not {ground})"),
        (f"(and (not (forall ((x A)) (q x))) {all_x_z(diagonal)})", "(q a)", f"(not {ground})"),
        (f"(and (forall ((x A)) (exists ((x A)) (q x))) {all_x_z(diagonal)})",
         "(not (q a))", f"(not {ground})"),
        (all_x_z(f"(= {diagonal} (q (h x)))"), "(not (= (r a b c) (q (h a))))",
         f"(not (= {ground} (q (h {term}))))"),
    ]
    premise, consistent, contrary = cases[seed % len(cases)]
    return f"{prefix}(assert {premise})(assert {consistent})", f"(assert {contrary})"


def check(args, work):
    overlay = work / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {
        str(ROOT / "api/study_instance_witness_test.go"): str(HERE / "witness_test.go.txt")
    }}))
    driver = work / "instance.test"
    subprocess.run(["go", "test", "-c", "-p=2", f"-overlay={overlay}", "-o", driver, "./api"],
                   cwd=ROOT, check=True)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["first_seed", "models", "direct_refutations", "native_models", "concrete_models"])
    for start in range(args.start, args.start + args.count, 250):
        rows = []
        direct = native = concrete = 0
        for seed in range(start, min(start + 250, args.start + args.count)):
            if seed % 3:
                script, clash = instance(seed)
                direct += 1
            else:
                model = Problem(seed, one_bit=seed % 2 == 0)
                if not all(evaluate(t, {}, model.widths, model.tables) for t in model.terms):
                    raise ValueError(f"model failed at seed {seed}")
                script, clash = model.script(), ""
            use_solver = seed % args.concrete_every == 0
            concrete += int(use_solver)
            rows.append(json.dumps({"Seed": seed, "Input": script, "GroundConflict": clash,
                                    "Concrete": use_solver}))
            if args.native and seed % args.native_every == 0:
                query = script + "\n(check-sat)\n"
                answers = ["sat"]
                if clash:
                    query += f"(push)\n{clash}\n(check-sat)\n(pop)\n(check-sat)\n"
                    answers += ["unsat", "sat"]
                result = subprocess.run([args.native, "-in", "-smt2"], input=query,
                                        text=True, capture_output=True, timeout=30, check=True)
                if result.stdout.splitlines() != answers:
                    raise ValueError((seed, result.stdout, query))
                native += 1
        result = subprocess.run([driver, "-test.run=^TestUniversalInstanceModels$", "-test.v"],
                                input="\n".join(rows), env=dict(os.environ, GOMAXPROCS="1"),
                                text=True, capture_output=True)
        if result.returncode or f"verified {len(rows)} models and {direct} direct refutations" not in result.stdout:
            raise RuntimeError(result.stdout + result.stderr)
        writer.writerow([start, len(rows), direct, native, concrete])
        sys.stdout.flush()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--start", type=int, default=0)
    parser.add_argument("--count", type=int, default=1000)
    parser.add_argument("--native", type=Path)
    parser.add_argument("--native-every", type=int, default=251)
    parser.add_argument("--concrete-every", type=int, default=2047)
    args = parser.parse_args()
    if args.start < 0 or min(args.count, args.native_every, args.concrete_every) < 1:
        parser.error("invalid range or sampling interval")
    with tempfile.TemporaryDirectory() as tmp:
        check(args, Path(tmp))
