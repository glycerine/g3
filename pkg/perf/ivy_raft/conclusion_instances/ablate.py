"""Measure the two conclusion instances and wrapper omission separately."""

import argparse
import csv
import gzip
import json
import os
from pathlib import Path
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "ground_instances"))
from replay import measure, observed_checker, replay_source


def controls(case, projected):
    original = case["original"].splitlines()[:-1] + case["specialized"].splitlines()
    active = projected.splitlines()
    additions = [line for line in active if line not in original]
    omitted = [line for line in original if line not in active]
    if len(additions) != 2 or len(omitted) != 2 or original[-1] != active[-1]:
        raise ValueError("expected a pair of specialized premises and two omitted definitions")
    rows = {}
    for keep in (True, False):
        prefix = [line for line in original[:-1] if keep or line not in omitted]
        for label, copies in [("neither", []), ("first", additions[:1]),
                              ("second", additions[1:]), ("both", additions)]:
            name = ("keep" if keep else "omit") + "-" + label
            rows[name] = "\n".join([*prefix, *copies, original[-1]]) + "\n"
    if rows["omit-both"] != projected:
        raise ValueError("control does not reproduce the solver query")
    return rows


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", type=Path, required=True)
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    args = parser.parse_args()
    with gzip.open(HERE / "primary.json.gz", "rt") as f:
        projected = json.load(f)
    with gzip.open(HERE.parent / "conditional/cases.json.gz", "rt") as f:
        experiments = {c["obligation"]: controls(c, projected[str(c["obligation"])])
                       for c in json.load(f) if c["established"]}
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["variant", *[f"{n}_{k}" for n in experiments for k in ("decisions", "instances", "conflicts", "resources")]])
    with observed_checker(args.goivy.resolve(strict=True), replay_source()) as exe:
        for mode in next(iter(experiments.values())):
            data = {n: [projected[str(n)], modes[mode]] for n, modes in experiments.items()}
            rows = measure(exe, args.spec.resolve(strict=True), args.cpu,
                           env=dict(os.environ, RAFT_REPLAY=json.dumps(data)))
            writer.writerow([mode, *[value for n in experiments for value in rows[n][-1]]])
            sys.stdout.flush()
