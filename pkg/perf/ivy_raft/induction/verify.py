"""Check each new Raft lemma against one original universal premise in isolation."""

import argparse
import csv
import gzip
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "order_models"))
from finite_model import parse, smt

HERE = Path(__file__).resolve().parent


def is_instance(premise, lemma):
    if premise[0] != "forall":
        return False
    variables = dict(premise[1])
    bindings = {}

    def match(pattern, term):
        if isinstance(pattern, str):
            if pattern in variables:
                return bindings.setdefault(pattern, term) == term
            return pattern == term
        return (isinstance(term, list) and pattern[0] not in ("forall", "exists")
                and pattern[0] == term[0] and len(pattern) == len(term)
                and all(match(p, t) for p, t in zip(pattern[1:], term[1:])))

    return match(premise[2], lemma) and bindings.keys() == variables.keys()


def verify(before, after, native):
    lines = before.splitlines()
    updated = after.splitlines()
    if lines[:-1] != updated[:-1]:
        raise ValueError("changed declarations or premises")
    if lines[-1] == updated[-1]:
        return []
    goal = parse(lines[-1])[1]
    assertion, conjunction = parse(updated[-1])
    if assertion != "assert" or conjunction[:2] != ["and", goal] or len(conjunction) < 3:
        raise ValueError("expected the original goal followed by additional lemmas")
    declarations = [line for line in lines if line.startswith("(declare-")]
    premises = [parse(line)[1] for line in lines[:-1] if line.startswith("(assert ")]
    witnesses = []
    for lemma in conjunction[2:]:
        for number, premise in enumerate(premises, 1):
            if is_instance(premise, lemma):
                break
        else:
            raise ValueError("lemma is not a substitution instance of any original premise")
        query = "\n".join([*declarations, f"(assert {smt(premise)})",
                           f"(assert (not {smt(lemma)}))", "(check-sat)"])
        result = subprocess.run([native, "-in", "-smt2"], input=query, text=True,
                                capture_output=True, check=True, timeout=20)
        if result.stdout.strip() != "unsat":
            raise ValueError(f"native check of premise {number} returned {result.stdout!r}")
        witnesses.append(number)
    return witnesses


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--captures", type=Path, nargs=2, metavar=("BEFORE", "AFTER"),
                        help="verify another pair of directories containing rendered order problems")
    parser.add_argument("--native", type=Path, required=True, help="native z3 executable")
    args = parser.parse_args()
    if args.captures:
        before, after = args.captures
        files = sorted(path.name for path in before.glob("*.smt2"))
        if not files or files != sorted(path.name for path in after.glob("*.smt2")):
            parser.error("captured input sets differ or are empty")
        cases = ((int(Path(name).stem), (before / name).read_text(), (after / name).read_text())
                 for name in files)
    else:
        cases = archived_queries()
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["obligation", "verified_instances", "premise_numbers"])
    for number, before, after in cases:
        try:
            witnesses = verify(before, after, args.native.resolve())
        except (ValueError, subprocess.SubprocessError) as error:
            raise RuntimeError(f"checking obligation {number}") from error
        writer.writerow([number, len(witnesses), " ".join(map(str, witnesses))])


def archived_queries():
    corpus = HERE.parents[2] / "api/test_vectors/raft_order/goivy-kernel.smt2.gz"
    separator = "(check-sat)\n(reset)\n"
    with gzip.open(corpus, "rt") as source:
        queries = source.read().removesuffix(separator).split(separator)
    with gzip.open(HERE / "final-assertions.csv.gz", "rt") as source:
        for row in csv.DictReader(source):
            number = int(row["obligation"])
            before = queries[number - 1]
            after = before
            if replacement := row["replacement"]:
                prefix, goal = before.rsplit("(assert ", 1)
                if not goal.endswith(")\n"):
                    raise ValueError(f"missing final assertion at {number}")
                after = prefix + replacement + "\n"
            yield number, before, after


if __name__ == "__main__":
    main()
