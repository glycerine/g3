"""Estimate instructions for every incremental Apple obligation in one run per backend."""

import argparse
from bisect import bisect_right
import csv
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

from phase_counters import build


WINDOWS = r'''
var perfCheckNumber int

func perfMonotonicTime() int64 {
    var stamp unix.Timespec
    if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &stamp); err != nil { panic(err) }
    return stamp.Nano()
}

func beginMeasuredCheck() func() {
    perfCheckNumber++
    sequence := perfCheckNumber
    start := perfMonotonicTime()
    return func() {
        end := perfMonotonicTime()
        fmt.Fprintln(os.Stderr, "IVY-CHECK-WINDOW", sequence, start, end)
    }
}
'''
FIELDS = ["isolate", "cpu", "round", "engine", "check", "period", "samples",
          "estimated_instructions", "check_ns"]
SAMPLE = re.compile(r"\s*(\d+)\.(\d{9}):\s+(\d+) instructions:u:\s*")


def windows(stderr, reference):
    found = []
    for line in stderr.splitlines():
        if line.startswith(b"IVY-CHECK-WINDOW "):
            seq, start, end = map(int, line.split()[1:])
            if seq != len(found) + 1 or end < start or (found and found[-1][1] > start):
                raise ValueError("overlapping or incomplete check windows")
            found.append((start, end))
    if len(found) != reference.splitlines().count(b"PASS") or not found:
        raise ValueError("check window count differs from the complete transcript")
    return found


def count(args, binary, run, reference):
    run.mkdir()
    data, errors = run / "perf.data", run / "stderr"
    cmd = ["taskset", "-c", str(args.cpu), args.perf, "record", "--no-buildid", "--no-buildid-cache",
           "--clockid", "mono", "-T", "-m", "512", "-e", "instructions:u", "-c", str(args.period),
           "-o", str(data), "--", str(binary), "isolate=" + args.isolate, str(args.spec)]
    with errors.open("wb") as err:
        r = subprocess.run(cmd, cwd=run, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"),
                           stdout=subprocess.PIPE, stderr=err, check=True)
    if r.stdout != reference:
        raise ValueError(f"{binary.name} stdout differs from the reference")
    intervals = windows(errors.read_bytes(), reference)
    starts = [start for start, end in intervals]
    # Row zero is frontend and solver work outside CheckVerificationCondition.
    votes, instructions = [0] * (1 + len(starts)), [0] * (1 + len(starts))
    with subprocess.Popen([args.perf, "script", "--ns", "--show-lost-events", "-F", "time,event,period",
                           "-i", str(data)], stdout=subprocess.PIPE, text=True) as reader:
        for line in reader.stdout:
            if not line.strip():
                continue
            match = SAMPLE.fullmatch(line)
            if not match:
                raise ValueError(f"{data}: malformed sample or lost event: {line!r}")
            sec, ns, period = map(int, match.groups())
            tick = sec * 1_000_000_000 + ns
            n = bisect_right(starts, tick)
            if n and tick >= intervals[n-1][1]:
                n = 0
            votes[n] += 1
            instructions[n] += period
        if reader.wait() != 0 or sum(votes) == 0:
            raise ValueError(f"could not read samples from {data}")
    if not args.keep_raw:
        data.unlink()
    return [(i, votes[i], instructions[i], intervals[i-1][1] - intervals[i-1][0] if i else "")
            for i in range(len(votes))]


def build_observer(engine, args, scratch):
    return build(engine, args, scratch, WINDOWS, ("fmt", "os", "golang.org/x/sys/unix"))


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", required=True, type=Path)
    p.add_argument("--spec", required=True, type=Path)
    p.add_argument("--reference", required=True, type=Path)
    p.add_argument("--output", required=True, type=Path, help="new directory for run logs")
    p.add_argument("--isolate", default="rfn2")
    p.add_argument("--cpu", required=True, type=int)
    p.add_argument("--period", type=int, default=800_009, help="user instructions per sample")
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--perf", default="perf")
    p.add_argument("--modfile", type=Path)
    p.add_argument("--keep-raw", action="store_true", help="retain perf.data for each run")
    args = p.parse_args()
    if min(args.period, args.pairs) < 1:
        p.error("supply positive sampling and pair counts")
    args.goivy = args.goivy.resolve(strict=True)
    args.spec = args.spec.absolute()
    if not args.spec.is_file():
        p.error("missing specification")
    reference = args.reference.read_bytes()
    if reference.splitlines()[-1:] != [b"OK"]:
        p.error("the reference must end in OK")
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(FIELDS)
    with tempfile.TemporaryDirectory() as scratch:
        engines = [build_observer(s, args, Path(scratch)) for s in ("g3", "native")]
        for i in range(args.pairs):
            for engine in engines[::1 if i % 2 == 0 else -1]:
                rows = count(args, engine, output / f"{i+1}-{engine.name}", reference)
                for n, samples, instr, duration in rows:
                    writer.writerow([args.isolate, args.cpu, i+1, engine.name, n, args.period,
                                     samples, instr, duration])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
