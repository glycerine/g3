"""Check arithmetic and floating-point strategy outcomes against small known models or contradictions."""

import argparse
import csv
from pathlib import Path
import subprocess


TACTICS = {"QF_NRA": "qfnra", "QF_NIA": "qfnia", "QF_FP": "qffp", "QF_FPLRA": "qffplra"}


def cases():
    fp = "(_ FloatingPoint 5 11)"
    zero = "(_ +zero 5 11)"
    for k in range(1, 5):
        formulas = [
            ("QF_NRA", "Real", f"(= (* x x) {k}.0)", "sat", "real-square"),
            ("QF_NRA", "Real", f"(= (+ (* x x) {k}.0) 0.0)", "unsat", "real-negative"),
            ("QF_NIA", "Int", f"(= (* x x) {k*k})", "sat", "int-square"),
            ("QF_NIA", "Int", f"(and (>= x 0) (<= x {k}) (= (* x x) {k*k+1}))", "unsat", "int-bounded"),
            ("QF_FPLRA", fp, f"(and (= x {zero}) (> (fp.to_real x) {k}.0))", "unsat", "fp-real-positive"),
        ]
        if k == 1:
            formulas += [
                ("QF_FP", fp, f"(and (= x (_ -zero 5 11)) (fp.eq x {zero}))", "sat", "fp-zero"),
                ("QF_FP", fp, "(and (= x (_ NaN 5 11)) (fp.eq x x))", "unsat", "fp-nan"),
                ("QF_FPLRA", fp, f"(and (= x {zero}) (= (fp.to_real x) 0.0))", "sat", "fp-real-zero"),
            ]
        for logic, sort, goal, answer, label in formulas:
            for mode in range(4):
                option = "(set-option :smt.macro_finder true)\n" if mode % 2 else ""
                query = "(check-sat)" if mode < 2 else f"(check-sat-using {TACTICS[logic]})"
                source = f"{option}(set-logic {logic})\n(declare-const x {sort})\n(assert {goal})\n{query}\n"
                yield f"{label}-{k}-{mode}", logic, query, source, answer


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--g3", type=Path, required=True)
    p.add_argument("--native", type=Path, required=True)
    p.add_argument("--output", type=Path, required=True)
    args = p.parse_args()
    engines = [(name, getattr(args, name).resolve(strict=True)) for name in ("g3", "native")]
    root = args.output.resolve()
    root.mkdir(parents=True)
    with (root / "outcomes.csv").open("w") as stream:
        writer = csv.writer(stream, lineterminator="\n")
        writer.writerow(["case", "logic", "command", "expected", "engine", "exit", "result"])
        for name, logic, query, source, expected in cases():
            path = root / (name + ".smt2")
            path.write_text(source)
            for engine, shell in engines:
                r = subprocess.run([shell, "-smt2", path], cwd=root, text=True, capture_output=True, timeout=30)
                result = r.stdout.strip()
                if r.returncode or result != expected:
                    raise ValueError(f"{name} {engine}: expected {expected}; got {r.returncode}, {result!r}, {r.stderr!r}")
                writer.writerow([name, logic, query, expected, engine, r.returncode, result])


if __name__ == "__main__":
    main()
