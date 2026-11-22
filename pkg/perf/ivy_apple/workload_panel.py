"""Run paired complete GoIvy comparisons on ordinary Raft and the nine Apple isolates."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import csv
from pathlib import Path
import subprocess
import sys

from compare import EVENTS


# Schedule the longest isolates first. A workload always uses one CPU for both
# engines, regardless of the number of lanes requested.
WORKLOADS = {
    "cf_pio_live": 2215, "dramc_nb2": 2479, "e2e": 2378, "sys_live": 1738, "cmpl_live": 437,
    "raft": 419, "rfn2": 26, "rfn": 33, "cf_live": 336, "tprops": 700,
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--raft", required=True, type=Path, help="path to raft_no_assume_test.ivy")
    parser.add_argument("--apple", required=True, type=Path, help="path to ord_live.ivy")
    parser.add_argument("--cpus", nargs="+", type=int, required=True, help="one CPU per lane, on distinct cores")
    parser.add_argument("--pairs", type=int, default=6)
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--references", type=Path, help="directory of complete known-good WORKLOAD.out files")
    parser.add_argument("--output", type=Path, required=True, help="new directory for all raw transcripts and panel.csv")
    parser.add_argument("--workloads", nargs="+", choices=WORKLOADS, default=list(WORKLOADS))
    parser.add_argument("--g3", type=Path, required=True, help="GoIvy built with xtrace_off")
    parser.add_argument("--native", type=Path, required=True, help="same GoIvy built with classic_z3,xtrace_off")
    args = parser.parse_args()
    if args.pairs < 1 or len(set(args.workloads)) != len(args.workloads) or len(set(args.cpus)) != len(args.cpus):
        parser.error("use a positive number of pairs and distinct workloads and CPUs")
    root = args.output.resolve()
    root.mkdir(parents=True, exist_ok=False)
    checkers = [args.g3.resolve(strict=True), args.native.resolve(strict=True)]
    driver = Path(__file__).resolve().with_name("compare.py")

    def lane(lane_index):
        rows = []
        for i in range(lane_index, len(args.workloads), len(args.cpus)):
            name = args.workloads[i]
            call = [sys.executable, "-B", driver, "--labels", "g3", "native", "--cpu", str(args.cpus[lane_index]),
                    "--perf", args.perf, "--pairs", str(args.pairs), "--output", root / name]
            if name == "raft":
                call += ["--spec", args.raft.absolute(), "--whole-spec"]
            else:
                call += ["--spec", args.apple.absolute(), "--isolate", name]
            if i % 2:
                call.append("--reverse")
            if args.references:
                call += ["--reference", (args.references / (name + ".out")).resolve(strict=True)]
            path = root / (name + ".csv")
            with path.open("w") as out:
                subprocess.run([*call, *checkers], stdout=out, check=True)
            with path.open() as stream:
                runs = list(csv.DictReader(stream))
            if len(runs) != 2 * args.pairs or any(int(r["passes"]) != WORKLOADS[name] for r in runs):
                raise ValueError(f"{name}: incomplete comparison")
            rows.extend(dict(workload=name, **r) for r in runs)
        return rows

    with ThreadPoolExecutor(len(args.cpus)) as pool:
        results = [r for group in pool.map(lane, range(len(args.cpus))) for r in group]
    fields = ["workload", "pair", "checker", "isolate", "cpu", "exit", "wall_s", "user_s", "sys_s",
              "rss_KiB", "passes", "same_transcript", *EVENTS]
    with (root / "panel.csv").open("w") as stream:
        writer = csv.DictWriter(stream, fieldnames=fields, lineterminator="\n")
        writer.writeheader()
        writer.writerows(sorted(results, key=lambda r: (r["workload"], int(r["pair"]))))


if __name__ == "__main__":
    main()
