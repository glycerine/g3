"""Capture model-table decisions and replay the original auxiliary questions in native Z3."""
import argparse
import base64
from collections import Counter
import csv
import gzip
from pathlib import Path
import os
import re
import subprocess
import sys

from complete_search import same_output

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "goal_instances"))
from capture import standard_declarations


def canonical_query(source):
    source = standard_declarations(source)
    token = r"\|[^|]*\||[^()\s]+"
    sorts = {s.strip("|"): s for s in re.findall(r"^\(declare-sort\s+(" + token + r")\s+0\)", source, re.M)}
    declared = set(re.findall(r"^\(declare-fun\s+(" + token + r")\s+", source, re.M))
    values = {}
    for name in re.findall(token, source):
        match = re.fullmatch(r"(.+)!val!\d+", name.strip("|"))
        if match and name not in declared:
            sort = sorts[match[1]]
            values.setdefault(sort, set()).add(name)
    # The model printer emits builtin model values as atoms. Give each a
    # constant of its actual sort and preserve pairwise distinctness per sort.
    extra = []
    for sort, names in sorted(values.items()):
        names = sorted(names)
        extra.extend(f"(declare-const {v} {sort})" for v in names)
        if len(names) > 1:
            extra.append("(assert (distinct " + " ".join(names) + "))")
    if extra:
        pos = source.index("(assert ") if "(assert " in source else source.index("(check-sat)")
        source = source[:pos] + "\n".join(extra) + "\n" + source[pos:]
    keys = re.findall(r"^\(declare-fun (k!\d+) \(\) ", source, re.M)
    prefix = "mbqiReplay!"
    while prefix in source:
        prefix += "_"
    renamed = {old: prefix + str(i) for i, old in enumerate(keys)}
    return re.sub(r"\|[^|]*\||[^()\s]+", lambda m: renamed.get(m[0], m[0]), source)


def capture(args):
    flags = [f"isolate={args.isolate}"] if args.isolate else []
    command = ["taskset", "-c", str(args.cpu), args.checker.resolve(), *flags, args.spec.absolute()]
    directory = args.output.resolve()
    directory.mkdir(parents=True)
    with (directory / "stdout").open("wb") as out, gzip.open(directory / "audit.gz", "wb") as archive:
        with subprocess.Popen(command, cwd=directory, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"),
                              stdout=out, stderr=subprocess.PIPE) as checker:
            for line in checker.stderr:
                archive.write(line)
            status = checker.wait()
    if status != args.expected_exit or not same_output((directory / "stdout").read_bytes(), args.reference.read_bytes(), True):
        raise ValueError(f"changed proof transcript or exit {status} in {directory}")
    certificates = {}
    events = Counter()
    with gzip.open(directory / "audit.gz", "rt") as audit:
        for line in audit:
            if line.startswith("TABLE-CERT "):
                _, status, data = line.split()
                question = canonical_query(base64.b64decode(data, validate=True).decode())
                certificates.setdefault(question, Counter())[int(status)] += 1
            elif line.startswith("TABLE-WITNESS "):
                events["witnesses"] += 1
            elif line.strip() == "TABLE-TRUE":
                events["evaluated_true"] += 1
    with gzip.open(directory / "questions.csv.gz", "wt") as out:
        writer = csv.writer(out, lineterminator="\n")
        writer.writerow(["question", "aux_unsat", "aux_unknown", "aux_sat", "smt2"])
        for i, (source, counts) in enumerate(certificates.items()):
            writer.writerow([i, counts[-1], counts[0], counts[1], source])
    with (directory / "events.csv").open("w") as out:
        writer = csv.writer(out, lineterminator="\n")
        writer.writerow(["witnesses", "evaluated_true", "certificates", "unique_questions"])
        writer.writerow([events["witnesses"], events["evaluated_true"], sum(c.total() for c in certificates.values()), len(certificates)])
    return directory / "questions.csv.gz"


def replay(native, cpu, questions, result):
    csv.field_size_limit(sys.maxsize)
    with gzip.open(questions, "rt") as f:
        rows = list(csv.DictReader(f))
    native = native.resolve(strict=True)
    with result.open("w") as f:
        writer = csv.writer(f, lineterminator="\n")
        writer.writerow(["question", "aux_unsat", "aux_unknown", "aux_sat", "native"])
        for i in range(0, len(rows), 100):
            group = rows[i:i + 100]
            source = "\n(reset)\n".join(r["smt2"] for r in group)
            p = subprocess.run(["taskset", "-c", str(cpu), native, "-in", "-smt2"], input=source,
                               text=True, capture_output=True, timeout=120, check=True)
            answers = p.stdout.splitlines()
            if len(answers) != len(group) or p.stderr:
                raise ValueError(f"unexpected native solver output at question {i}: {p.stderr[:300]}")
            for row, answer in zip(group, answers):
                writer.writerow([row[k] for k in ("question", "aux_unsat", "aux_unknown", "aux_sat")] + [answer])
                if answer != "unsat":
                    raise ValueError(f"native Z3 returned {answer} for question {row['question']}")
    return len(rows)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--native", type=Path, required=True)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--output", type=Path, required=True)
    p.add_argument("--questions", type=Path, help="replay an existing, typed collection")
    p.add_argument("--checker", type=Path, help="GoIvy built with model_tables_audit.patch")
    p.add_argument("--spec", type=Path)
    p.add_argument("--isolate")
    p.add_argument("--reference", type=Path)
    p.add_argument("--expected-exit", type=int, default=0)
    args = p.parse_args()
    if args.questions:
        args.output.mkdir(parents=True)
        questions = args.questions
    else:
        if not all((args.checker, args.spec, args.reference)):
            p.error("capture requires --checker, --spec and --reference")
        questions = capture(args)
    replay(args.native, args.cpu, questions, args.output / "native.csv")


if __name__ == "__main__":
    main()
