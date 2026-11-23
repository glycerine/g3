"""Check Boolean projections of real GoIvy goals after unit propagation missed a refutation."""
import argparse
from collections import defaultdict
import csv
import gzip
from pathlib import Path
import subprocess


def groups(path):
    bases, queries = {}, defaultdict(list)
    batch = ordinal = 0
    pending = None
    with gzip.open(path, "rt") as source:
        for line in source:
            fields = line.split()
            if not fields:
                continue
            event = fields[0]
            if event == "PATH-BATCH":
                batch += 1
                ordinal = 0
            elif event == "PATH-QUERY":
                ordinal += 1
                if pending is not None:
                    raise ValueError("missing concrete result for prior goal")
            elif event == "BC-BASE":
                key, variables, size = map(int, fields[1:])
                if key in bases:
                    raise ValueError(f"duplicate base {key}")
                bases[key] = (variables, size, [])
            elif event == "BC":
                bases[key][2].append(tuple(map(int, fields[1:])))
            elif event == "BC-Q":
                _, base, unknown, expression = line.rstrip().split(" ", 3)
                pending = int(base), int(unknown), expression
            elif event == "PATH-END" and pending:
                base, unknown, expr = pending
                queries[base].append((batch, ordinal, unknown, expr, fields[1]))
                pending = None
    if pending is not None or not queries.keys() <= bases.keys():
        raise ValueError("incomplete capture")
    for key, (variables, size, clauses) in bases.items():
        if size != len(clauses) or any(not 0 < abs(lit) <= variables for clause in clauses for lit in clause):
            raise ValueError(f"incorrect clause count or variable outside base {key}")
        if key in queries:
            yield key, variables, clauses, queries[key]


def replay(capture, solver, output, timeout_ms):
    with output.open("w") as results:
        writer = csv.writer(results, lineterminator="\n")
        writer.writerow(["base", "batch", "ordinal", "unknown_atoms", "original", "projected"])
        for key, variables, clauses, queries in groups(capture):
            program = [f"(set-option :timeout {timeout_ms})", "(set-logic QF_UF)"]
            for prefix, count in (("v", variables), ("u", max(q[2] for q in queries))):
                program.extend(f"(declare-const {prefix}{i} Bool)" for i in range(1, count + 1))
            for clause in clauses:
                literals = [f"v{x}" if x > 0 else f"(not v{-x})" for x in clause]
                body = "(or " + " ".join(literals) + ")" if len(literals) > 1 else next(iter(literals), "false")
                program.append(f"(assert {body})")
            for _, _, _, expr, _ in queries:
                program.extend(["(push 1)", f"(assert {expr})", "(check-sat)", "(pop 1)"])
            run = subprocess.run([solver, "-in", "-smt2"], input="\n".join(program) + "\n", text=True,
                                 capture_output=True, check=True)
            answers = run.stdout.splitlines()
            if run.stderr or len(answers) != len(queries) or any(x not in ("sat", "unsat", "unknown") for x in answers):
                raise ValueError(f"invalid results for base {key}")
            for (batch, ordinal, unknown, _, original), answer in zip(queries, answers):
                if answer == "unsat" and original == "sat":
                    raise ValueError(f"projection contradicts a counterexample in batch {batch}, goal {ordinal}")
                writer.writerow([key, batch, ordinal, unknown, original, answer])


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--capture", required=True, type=Path)
    parser.add_argument("--solver", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--timeout-ms", type=int, default=3000)
    args = parser.parse_args()
    replay(args.capture, args.solver, args.output, args.timeout_ms)
