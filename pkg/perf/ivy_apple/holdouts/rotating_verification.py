#!/usr/bin/env python3
"""Rotate current g3, previous g3 and linked Z3 on a single Linux CPU."""
import argparse
import csv
from pathlib import Path
import sys
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from compare import EVENTS, measure


def thread_siblings(cpu):
    path = Path(f"/sys/devices/system/cpu/cpu{cpu}/topology/thread_siblings_list")
    ids = set()
    for item in path.read_text().strip().split(","):
        first, _, last = item.partition("-")
        ids.update(range(int(first), int(last or first) + 1))
    return sorted(ids - {cpu})


def ticks(cpus):
    labels = {f"cpu{c}": c for c in cpus}
    result = {}
    for line in Path("/proc/stat").read_text().splitlines():
        name, *fields = line.split()
        if name in labels:
            values = list(map(int, fields))
            result[labels[name]] = (values[3] + values[4], sum(values[:8]))
    return result


def idle(before, after, cpu):
    elapsed = after[cpu][1] - before[cpu][1]
    return (after[cpu][0] - before[cpu][0]) / max(1, elapsed)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for label in ("g3", "previous", "native", "spec", "reference", "output"):
        parser.add_argument("--" + label, required=True, type=Path)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--isolate", default="")
    parser.add_argument("--perf", required=True, help="Linux perf executable")
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--expected-exit", type=int, default=0)
    parser.add_argument("--min-idle", type=float, default=.95)
    parser.add_argument("--max-attempts", type=int, default=6)
    parser.add_argument("--wait-seconds", type=float, default=3600)
    args = parser.parse_args()
    if min(args.rounds, args.max_attempts, args.wait_seconds) <= 0 or not 0 <= args.min_idle <= 1:
        parser.error("rounds, attempts and wait must be positive, idle must be in [0,1]")
    args.spec = args.spec.absolute()
    checkers = [(label, getattr(args, label).resolve(strict=True)) for label in ("g3", "previous", "native")]
    expected = [(args.reference / stream).read_bytes() for stream in ("stdout", "stderr")]
    siblings = thread_siblings(args.cpu)
    cpus = [args.cpu, *siblings]
    args.output.mkdir(parents=True, exist_ok=True)
    with (args.output / "runs.csv").open("x") as dest:
        writer = csv.writer(dest)
        writer.writerow(["round", "engine", "cpu", "siblings", "attempt", "min_idle_before",
                         "sibling_idle_during", "cpu_fraction", "exit", "match", "wall", "user", "sys", "rss", *EVENTS])
        for i in range(args.rounds):
            offset = i % len(checkers)
            for label, checker in checkers[offset:] + checkers[:offset]:
                for attempt in range(1, args.max_attempts + 1):
                    deadline = time.monotonic() + args.wait_seconds
                    while True:
                        first = ticks(cpus)
                        time.sleep(3)
                        start = ticks(cpus)
                        before = min(idle(first, start, cpu) for cpu in cpus)
                        if before >= args.min_idle:
                            break
                        if time.monotonic() >= deadline:
                            raise SystemExit(f"CPU {args.cpu} busy before {label}, round {i+1}")
                        time.sleep(10)
                    path = args.output / f"{i+1}-{label}-{attempt}"
                    rc, timing, counts = measure(checker, args, path, args.expected_exit, 100)
                    end = ticks(cpus)
                    sibling_idle = min((idle(start, end, c) for c in siblings), default=1.0)
                    fraction = (float(timing[1]) + float(timing[2])) / float(timing[0])
                    ok = rc == args.expected_exit and expected == [(path / k).read_bytes() for k in ("stdout", "stderr")]
                    writer.writerow([i+1, label, args.cpu, ",".join(map(str, siblings)), attempt, round(before, 4),
                                     round(sibling_idle, 4), round(fraction, 4), rc, int(ok), *timing, *counts])
                    dest.flush()
                    if not ok:
                        raise SystemExit(f"changed result in {path}")
                    if min(sibling_idle, fraction) >= args.min_idle:
                        break
                else:
                    raise SystemExit(f"no qualifying observation for {label} in round {i+1}")


if __name__ == "__main__":
    main()
