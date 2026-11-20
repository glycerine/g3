"""Replay GoIvy's 419 concrete proof attempts through Ivy's native simple solver."""

import argparse
import csv
import gzip
from pathlib import Path
import re
from time import perf_counter

import ivy.z3 as z3


class Kernel:
    def __init__(self):
        self.ctx = z3.Context()
        self.solver = z3.SimpleSolver(ctx=self.ctx)
        self.solver.set("smt.macro_finder", True)
        self.sorts = {}
        self.functions = {}
        self.declarations = {}
        self.premises = ""

    def declare(self, text):
        # The order renderer emits exactly these three declaration commands.
        for line in text.splitlines():
            op, name, *types = re.findall(r"[^\s()]+", line)
            if name in self.declarations:
                if self.declarations[name] != line:
                    raise ValueError(f"incompatible declaration {name}")
                continue
            if op == "declare-sort" and types == ["0"]:
                self.sorts[name] = z3.DeclareSort(name, ctx=self.ctx)
            elif op in {"declare-fun", "declare-const"}:
                sorts = [z3.BoolSort(ctx=self.ctx) if t == "Bool" else self.sorts[t] for t in types]
                self.functions[name] = z3.Function(name, *sorts)
            else:
                raise ValueError(line)
            self.declarations[name] = line

    def load(self, query, expected):
        start = query.index("(assert ")
        self.declare(query[:start])
        assertions = query[start:]
        final = assertions.rindex("(assert ")
        premises = assertions[:final]
        reuse = bool(self.premises) and premises.startswith(self.premises)
        suffix = assertions[len(self.premises):] if reuse else assertions
        # Ivy's Z3 4.7.1 binding returns the conjunction of a parser's
        # assertions. Only split the conjunction when the input had many.
        formula = z3.parse_smt2_string(suffix, sorts=self.sorts, decls=self.functions, ctx=self.ctx)
        count = suffix.count("\n")
        formulas = [formula] if count == 1 else formula.children()
        if len(formulas) != count:
            raise ValueError("wrong number of parsed assertions")
        if not reuse:
            if self.premises:
                self.solver.pop()
            self.premises = ""
            if len(formulas) > 1:
                self.solver.push()
                self.solver.add(*formulas[:-1])
                formulas = formulas[-1:]
                self.premises = premises
        if len(self.premises) != expected:
            raise ValueError("retained premise differs from the recorded scope")
        self.solver.push()
        self.solver.add(*formulas)


def main():
    args = argparse.ArgumentParser()
    args.add_argument("--fresh", action="store_true", help="use an independent kernel for each check")
    fresh = args.parse_args().fresh
    root = Path(__file__).resolve().parents[2] / "api/test_vectors/raft_order"
    with gzip.open(root / "goivy-kernel.smt2.gz", "rt") as src:
        text = src.read()
    boundary = "(check-sat)\n(reset)\n"
    queries = text.removesuffix(boundary).split(boundary)
    with (root / "goivy-scopes.csv").open() as src:
        rows = list(csv.DictReader(src))
    if len(queries) != len(rows) or len(rows) != 419 or not text.endswith(boundary):
        raise ValueError("incomplete Raft corpus")
    load = check = 0.0
    start = perf_counter()
    for i, (query, row) in enumerate(zip(queries, rows), 1):
        if int(row["obligation"]) != i:
            raise ValueError(f"out-of-order obligation {i}")
        if fresh or row["new_kernel"] == "true":
            kernel = Kernel()
        reuse = not fresh and row["batch"] == "true"
        tick = perf_counter()
        if reuse:
            kernel.load(query[query.index("(declare-"):], int(row["retained_premise_bytes"]))
        else:
            kernel.solver.from_string(query)
        parsed = perf_counter()
        if kernel.solver.check() != z3.unsat:
            raise ValueError(f"missing proof for obligation {i}")
        checked = perf_counter()
        load += parsed - tick
        check += checked - parsed
        if reuse:
            kernel.solver.pop()
    del kernel
    print("obligations,total_s,load_s,check_s")
    print(f"{len(rows)},{perf_counter()-start:.6f},{load:.6f},{check:.6f}")


if __name__ == "__main__":
    main()
