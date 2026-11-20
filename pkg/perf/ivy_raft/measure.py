"""Time one full Ivy checker invocation and verify its ordered Raft proof transcript."""

import argparse
import csv
import os
from pathlib import Path
import re
import subprocess
import sys
import time


def transcript(lines):
    normalized = []
    reported_ok = False
    for line in lines:
        line = line.strip()
        if line.startswith("The following properties are treated as axioms"):
            normalized.clear()
            normalized.append(line)
        elif normalized and line == "OK":
            reported_ok = True
            break
        elif normalized and line and not re.fullmatch(r"#+", line):
            # The trace-capable GoIvy build can normalize paths itself.
            normalized.append(re.sub(r"(?:<IVY_EXAMPLES>|/[^\s:]+)/examples/raft/", "raft/", line))
    if not reported_ok or normalized.count("PASS") != 419:
        raise ValueError("incomplete Raft proof transcript")
    return normalized


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reference", type=Path, required=True)
    parser.add_argument("--cpu", type=int)
    parser.add_argument("--label", required=True)
    parser.add_argument("--sample", type=int, required=True)
    parser.add_argument("--output", type=Path, required=True, help="raw checker stdout")
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command
    if command[:1] == ["--"]:
        command = command[1:]
    if not command:
        parser.error("supply the checker command after --")
    paths = [args.output, *(args.output.with_suffix(s) for s in (".resources", ".stderr", ".passes"))]
    resolved = {p.resolve() for p in paths}
    if args.reference.resolve() in resolved or len(resolved) != len(paths):
        parser.error("use distinct paths for the reference and output files")
    want = transcript(args.reference.read_text().splitlines())
    env = dict(os.environ, XTRACE_OFF="1", PYTHONDONTWRITEBYTECODE="1")
    if args.cpu is not None:
        command = ["taskset", "-c", str(args.cpu), *command]
        env["GOMAXPROCS"] = "1"
    timing = args.output.with_suffix(".resources")
    command = ["/usr/bin/time", "-f", "%e %U %S %M", "-o", str(timing), *command]
    passes = []
    start = time.monotonic()
    with args.output.open("w") as output, args.output.with_suffix(".stderr").open("w") as err:
        with subprocess.Popen(command, env=env, stdout=subprocess.PIPE, stderr=err, text=True) as proc:
            for line in proc.stdout:
                output.write(line)
                if line.strip() == "PASS":
                    passes.append(time.monotonic() - start)
            if proc.wait():
                raise SystemExit("checker failed, see " + str(err.name))
    actual = args.output.read_text().splitlines()
    if len(passes) != 419 or next((line.strip() for line in reversed(actual) if line.strip()), "") != "OK":
        raise ValueError("checker output must end in OK after all 419 passes")
    if transcript(actual) != want:
        raise ValueError("Raft proof transcript diverged from " + str(args.reference))
    wall, user, system, rss = timing.read_text().split()
    with args.output.with_suffix(".passes").open("w") as f:
        for number, elapsed in enumerate(passes, 1):
            f.write(f"{number},{elapsed:.6f}\n")
    csv.writer(sys.stdout, lineterminator="\n").writerow(
        [args.sample, args.label, args.cpu if args.cpu is not None else "default",
         wall, user, system, rss, len(passes)])


if __name__ == "__main__":
    main()
