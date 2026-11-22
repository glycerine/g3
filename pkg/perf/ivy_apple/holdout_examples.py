"""Run complete, paired checks on independent Ivy examples with known outcomes."""

import argparse
import csv
from pathlib import Path
import subprocess
import sys

from compare import EVENTS

# Expected PASS and FAIL counts come from a complete linked-Z3 run.
EXAMPLES = {
    "toy-consensus": ("doc/examples/toy_consensus.ivy", 72, 0),
    "ticket": ("doc/examples/liveness/ticket_l2s_auto.ivy", 297, 0),
    "progress": ("doc/examples/liveness/progress2.ivy", 103, 0),
    "ping-pong": ("doc/examples/pingpongclock.ivy", 11, 0),
    "leader-ring": ("doc/examples/leader_election_ring.ivy", 7, 1),
    "election-btw": ("doc/examples/leader_election_ring_btw.ivy", 13, 1),
    "ring-bug": ("examples/pldi16/leader_election_ring_bug.ivy", 2, 1),
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--examples", type=Path, required=True, help="ivy-lang-examples checkout")
    parser.add_argument("--native", type=Path, required=True)
    parser.add_argument("--g3", type=Path, required=True)
    parser.add_argument("--pairs", type=int, default=6)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    root = args.output.resolve()
    root.mkdir(parents=True)
    driver = Path(__file__).resolve().with_name("compare.py")
    with (root / "examples.csv").open("w") as f:
        fields = ["example", "path", "failures", "pair", "checker", "isolate", "cpu", "exit",
                  "wall_s", "user_s", "sys_s", "rss_KiB", "passes", "same_transcript", *EVENTS]
        writer = csv.DictWriter(f, fieldnames=fields, lineterminator="\n")
        writer.writeheader()
        for name, (relative, passes, failures) in EXAMPLES.items():
            cmd = [sys.executable, "-B", driver, "--whole-spec", "--spec", args.examples / relative,
                   "--labels", "native", "g3", "--pairs", str(args.pairs), "--cpu", str(args.cpu),
                   "--perf", args.perf, "--expected-failures", str(failures), "--output", root / name,
                   args.native, args.g3]
            for row in csv.DictReader(subprocess.check_output(cmd, text=True).splitlines()):
                if int(row["passes"]) != passes:
                    raise ValueError(f"{name}: expected {passes} passing obligations")
                writer.writerow(dict(example=name, path=relative, failures=failures, **row))
            f.flush()


if __name__ == "__main__":
    main()
