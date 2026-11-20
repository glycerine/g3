"""Run the C++ control for BenchmarkRaftOrderKernel with Ivy's Z3 bindings."""

import argparse
import csv
import gzip
import json
import os
import re
import subprocess
import tempfile
from pathlib import Path
from time import perf_counter



def replay(query, expected=None):
    import ivy.z3 as z3

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
    if expected is not None:
        verify("native Z3", expected, str(solver.statistics()))
    return loaded - start, checked - loaded


def verify(engine, expected, text):
    values = dict(re.findall(r":([\w-]+)\s+([\d.e+-]+)", text))
    for key, want in expected.items():
        if key == "obligation" or key.endswith("_ms"):
            continue
        actual = float(values.get(key, 0))
        if actual != float(want):
            raise ValueError(f"{engine}, obligation {expected['obligation']}, {key}: {actual} != {want}")


def verify_go(root, expected):
    with tempfile.TemporaryDirectory() as tmp:
        work = Path(tmp)
        output = work / "counters.jsonl"
        overlay = work / "overlay.json"
        test = Path(__file__).resolve().with_name("kernel_statistics_test.go.txt")
        overlay.write_text(json.dumps({"Replace": {str(root / "api/raft_study_counters_test.go"): str(test)}}))
        subprocess.run(["go", "test", f"-overlay={overlay}", "./api", "-run", "^TestStudyRaftKernelCounters$",
                        "-count=1"], cwd=root, check=True, env=dict(os.environ, RAFT_COUNTER_OUTPUT=str(output)))
        rows = [json.loads(line) for line in output.read_text().splitlines()]
    if len(rows) != len(expected):
        raise ValueError(f"g3 returned {len(rows)} checks, expected {len(expected)}")
    for row, want in zip(rows, expected):
        if row["Obligation"] != int(want["obligation"]):
            raise ValueError(f"g3 obligation out of order: {row['Obligation']}")
        verify("g3", want, row["Statistics"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--verify-search", action="store_true",
                        help="also replay g3 and verify both kernels against the recorded 18 counters per check")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    expected = None
    if args.verify_search:
        with Path(__file__).with_name("order-matched-search.csv").open() as source:
            expected = list(csv.DictReader(source))
        if [int(r["obligation"]) for r in expected] != list(range(1, 420)):
            raise ValueError("incomplete search counter reference")
        verify_go(root, expected)
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
            load_time, check_time = replay(query, expected[number-1] if expected else None)
        except Exception as exc:
            raise RuntimeError(f"obligation {number}") from exc
        load += load_time
        check += check_time
    elapsed = perf_counter() - start
    if expected:
        print("All 18 search counters matched on every obligation in g3 and native Z3.")
    print("obligations,total_s,load_s,check_s")
    print(f"{len(queries)},{elapsed:.6f},{load:.6f},{check:.6f}")


if __name__ == "__main__":
    main()
