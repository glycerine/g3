"""Run the C++ control for BenchmarkRaftOrderKernel with Ivy's Z3 bindings."""

import gzip
from pathlib import Path
from time import perf_counter

import ivy.z3 as z3


def replay(query):
    ctx = z3.Context()
    solver = z3.SimpleSolver(ctx=ctx)
    solver.set("smt.macro_finder", True)
    start = perf_counter()
    solver.from_string(query)
    loaded = perf_counter()
    answer = solver.check()
    checked = perf_counter()
    if answer != z3.unsat:
        raise ValueError(f"expected UNSAT, got {answer}")
    return loaded - start, checked - loaded


def main():
    root = Path(__file__).resolve().parents[2]
    corpus = root / "api/test_vectors/raft_order/kernel.smt2.gz"
    with gzip.open(corpus, "rt") as source:
        text = source.read()
    boundary = "(check-sat)\n(reset)\n"
    queries = text.removesuffix(boundary).split(boundary)
    if not text.endswith(boundary) or len(queries) != 419:
        raise ValueError("incomplete Raft corpus")
    load = check = 0.0
    start = perf_counter()
    for number, query in enumerate(queries, 1):
        try:
            load_time, check_time = replay(query)
        except Exception as exc:
            raise RuntimeError(f"obligation {number}") from exc
        load += load_time
        check += check_time
    elapsed = perf_counter() - start
    print("obligations,total_s,load_s,check_s")
    print(f"{len(queries)},{elapsed:.6f},{load:.6f},{check:.6f}")


if __name__ == "__main__":
    main()
