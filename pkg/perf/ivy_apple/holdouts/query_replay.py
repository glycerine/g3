"""Replay captured Ivy obligations in fresh g3 and Z3 shell processes."""
import argparse
import csv
import gzip
from pathlib import Path
import re
import subprocess
import sys

HERE = Path(__file__).resolve().parent
CASES = ("toy-consensus-18", "ticket-127", "ticket-290")
NUMERIC_ANSWERS = {
    "aux-003060": "sat", "aux-004781": "sat", "aux-010541": "sat", "aux-011440": "unsat",
    "aux-012725": "sat", "aux-014966": "sat", "aux-017026": "unsat", "aux-019930": "unsat",
    "aux-020302": "unsat", "aux-020936": "sat",
}


def query_inputs(vector, auxiliary=False):
    with gzip.open(vector, "rt") as file:
        source = file.read()
    if auxiliary:
        # The model checker pushes its empty context before asserting the
        # instantiated body, universe restrictions and blocking clauses.
        first = re.search(r"(?m)^\(assert\b", source)
        if first is None or source.count("(check-sat)") != 1:
            raise ValueError(f"expected assertions and one check in {vector}")
        return {"fresh": source, "incremental": source[:first.start()] + "(push 1)\n" + source[first.start():]}
    # The native API printer in the linked fork omits zero arity on sorts.
    source = re.sub(r"^\(declare-sort ([^()\n]+)\)$", r"(declare-sort \1 0)", source, flags=re.M)
    pos = source.rfind("\n(assert ")
    if pos < 0 or source.count("(check-sat)") != 1:
        raise ValueError(f"{vector} has no final assertion or unique check")
    start = "(set-option :smt.macro_finder true)\n"
    return {"fresh": start + source, "incremental": start + source[:pos] + "\n(push 1)" + source[pos:]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("g3", "native", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--pairs", type=int, default=7)
    parser.add_argument("--suite", choices=("ivy", "numeric-auxiliary"), default="ivy")
    parser.add_argument("--queries", nargs="+", metavar="NAME")
    parser.add_argument("--routes", nargs="+", choices=("fresh", "incremental"), default=("fresh", "incremental"))
    parser.add_argument("--go-build", action="append", default=[], metavar="NAME=PATH",
                        help="include another g3 binary in the same rotated comparisons")
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--vectors", type=Path)
    a = parser.parse_args()
    if a.pairs < 1:
        parser.error("supply a positive pair count")
    auxiliary = a.suite == "numeric-auxiliary"
    answers = NUMERIC_ANSWERS if auxiliary else dict.fromkeys(CASES, "unsat")
    queries = a.queries or answers
    if any(case not in answers for case in queries):
        parser.error(f"unknown query for {a.suite}. Choose from {', '.join(answers)}")
    vectors = a.vectors or HERE.parent / "test_vectors"
    if auxiliary and a.vectors is None:
        vectors = vectors / "numeric_auxiliary"
    binaries = {"g3": a.g3.resolve(strict=True)}
    for item in a.go_build:
        label, equals, binary = item.partition("=")
        if not equals or not binary or not re.fullmatch(r"[\w-]+", label) or label in (*binaries, "native"):
            parser.error(f"invalid or repeated build name in {item!r}")
        binaries[label] = Path(binary).resolve(strict=True)
    binaries["native"] = a.native.resolve(strict=True)
    a.output.mkdir(parents=True)
    stats = "\n(get-info :all-statistics)\n"
    columns = ("decisions", "added-eqs" if auxiliary else "quant-instantiations", "conflicts", "rlimit-count")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["query", "route", "pair", "engine", "cpu", "answer", "instructions", *columns])
    for case in queries:
        inputs = query_inputs(vectors / (case + ".smt2.gz"), auxiliary)
        for route in a.routes:
            text = inputs[route]
            reference = None
            for i in range(a.pairs):
                order = list(binaries)
                if a.go_build:
                    offset = i % len(order)
                    order = order[offset:] + order[:offset]
                if i % 2:
                    order.reverse()
                for engine in order:
                    stem = a.output / f"{case}-{route}-{i+1}-{engine}"
                    counter = stem.with_suffix(".perf")
                    result = subprocess.run(["taskset", "-c", str(a.cpu), a.perf, "stat", "-x", ",",
                                             "-e", "instructions:u", "-o", counter, "--", binaries[engine],
                                             "-in", "-smt2"], input=text + stats, capture_output=True, text=True,
                                            timeout=120)
                    stem.with_suffix(".log").write_text(result.stdout + result.stderr)
                    result.check_returncode()
                    if result.stdout.splitlines()[:1] != [answers[case]] or "(error" in result.stdout:
                        raise ValueError(f"{stem} did not return the expected {answers[case]}")
                    entries = dict(re.findall(r":([\w-]+)\s+(\d+)\b", result.stdout))
                    if auxiliary:
                        # The shells omit zero-valued added-eqs on several arithmetic queries.
                        entries.setdefault("added-eqs", "0")
                    if any(col not in entries for col in columns):
                        raise ValueError(f"{stem} is missing search statistics")
                    observed = tuple(int(entries[col]) for col in columns)
                    if reference is None:
                        reference = observed
                    if observed != reference:
                        raise ValueError(f"{stem} changed solver counters from {reference} to {observed}")
                    with counter.open() as file:
                        counts = [row for row in csv.reader(file) if len(row) > 4 and row[2] == "instructions:u"]
                    if len(counts) != 1 or float(counts[0][4]) != 100.0:
                        raise ValueError(f"{counter} missing full instruction coverage")
                    writer.writerow([case, route, i+1, engine, a.cpu, answers[case], int(counts[0][0]), *observed])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
