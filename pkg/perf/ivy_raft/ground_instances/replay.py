"""Replay precomputed ground instances in the ordinary incremental GoIvy Raft search."""

import argparse
import csv
import gzip
import json
import os
from pathlib import Path
import re
import sys

from bench import parts

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "conditional"))
from search import KEYS, METHOD, measure, observed_checker, observed_source, replace_once

MODES = ["stock", "ground", "all-ground", "parent", "seed", "seed-first",
         "seed-multi", "seed-multi-first", "parent-seed"]


def replacement(case, mode):
    if mode == "stock":
        return case["problem"]
    declarations, assertions = parts(case)
    last = len(assertions) - 1
    instances = [(i, "(assert " + term + ")") for i, term in case["instances"] if i != last]
    ground = [a for _, a in instances]
    parents = {i for i, _ in instances}
    if mode == "ground":
        prefix = ground
    elif mode == "all-ground":
        prefix = list(dict.fromkeys([a for a in assertions[:-1] if not a.startswith("(assert (forall ")] + ground))
    else:
        prefix = [a for i, a in enumerate(assertions[:-1]) if not mode.startswith("parent") or i in parents]
        if "seed" in mode:
            seeds = []
            for i, instance in instances:
                parent = assertions[i]
                if not parent.startswith("(assert (forall "):
                    continue
                binders = parent.split("))", 1)[0] + "))"
                if "multi" not in mode or len(re.findall(r"\(v\d+ s\d+\)", binders)) >= 4:
                    seeds.append(instance)
            prefix = seeds + prefix if mode.endswith("-first") else prefix + seeds
    return declarations + "\n".join(prefix + [assertions[-1]]) + "\n"


def replay_source():
    code = replace_once(observed_source(), "import (", 'import (\n"strings"')
    code = replace_once(code, METHOD, """
var raftStudyObligation int
var replayInputs map[int][2]string
func init() {
    if err := json.Unmarshal([]byte(os.Getenv("RAFT_REPLAY")), &replayInputs); err != nil { panic(err) }
}
""" + METHOD + """
    raftStudyObligation++
    pair, selected := replayInputs[raftStudyObligation]
    replaced := false
    defer func() {
        if selected && (!replaced || !proved || err != nil) { panic("ground replay did not prove the selected query") }
    }()
""")
    check = "return p.check(parent, proof.Order, proof.SearchSteps, reuse)"
    code = replace_once(code, check, """
    if selected {
        if proof.Order.Text != pair[0] { panic("ground replay source query changed") }
        proof.Order.Text = pair[1]
        proof.Order.LastAssertionStart = strings.LastIndex(pair[1], "(assert ")
        replaced = true
    }
""" + check)
    return code


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", type=Path, required=True)
    p.add_argument("--spec", type=Path, required=True)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--mode", choices=MODES, action="append", help="replay just these modes")
    args = p.parse_args()
    ivy, spec = args.goivy.resolve(strict=True), args.spec.resolve(strict=True)
    with gzip.open(HERE / "cases.json.gz", "rt") as f:
        cases = json.load(f)
    code = replay_source()
    writer = csv.writer(sys.stdout, lineterminator="\n")
    short = ["dec", "qi", "conf", "resources"]
    writer.writerow(["variant"] + [f"{n}_{k}" for n in ["all", 251, 321] for k in short])
    with observed_checker(ivy, code) as exe:
        for mode in args.mode or MODES:
            replay = {s["obligation"]: [s["problem"], replacement(s, mode)] for s in cases}
            rows = measure(exe, spec, args.cpu, env=dict(os.environ, RAFT_REPLAY=json.dumps(replay)))
            totals = [sum(call[i] for group in rows.values() for call in group) for i in range(len(KEYS))]
            writer.writerow([mode, *totals, *rows[251][-1], *rows[321][-1]])
            sys.stdout.flush()


if __name__ == "__main__":
    main()
