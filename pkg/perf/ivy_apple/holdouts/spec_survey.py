"""Compare complete GoIvy runs on a list of specs, retaining timeouts and differing outputs."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import csv
import filecmp
import os
from pathlib import Path
import subprocess
import time


FIELDS = ("spec", "pair", "cpu", "gogc", "engine", "exit", "timed_out", "wall_s", "user_s", "sys_s",
          "rss_kib", "instructions", "pass", "fail", "ok", "same_stdout", "same_stderr")


def measure(args, spec, binary, directory, cpu):
    directory.mkdir(parents=True)
    stats = directory / "instructions.csv"
    usage = directory / "usage"
    command = ["taskset", "-c", str(cpu), args.perf, "stat", "-x", ",", "-e", "instructions:u",
               "-o", str(stats), "--", "/usr/bin/time", "-f", "%U %S %M", "-o", str(usage),
               "timeout", "-k", "2s", str(args.seconds), str(binary), str(spec)]
    start = time.monotonic()
    with (directory / "stdout").open("wb") as out, (directory / "stderr").open("wb") as err:
        result = subprocess.run(command, cwd=directory, stdout=out, stderr=err,
                                env=dict(os.environ, GOMAXPROCS="1", GOGC=str(args.gogc), XTRACE_OFF="1"))
    elapsed = time.monotonic() - start
    with stats.open() as f:
        counts = [int(r[0]) for r in csv.reader(f) if len(r) > 4 and r[2] == "instructions:u"
                  and float(r[4]) == 100]
    if len(counts) != 1:
        raise ValueError(f"missing instruction count in {stats}")
    user, system, rss = usage.read_text().splitlines()[-1].split()
    with (directory / "stdout").open("rb") as out:
        passes = failures = 0
        last = b""
        for line in out:
            line = line.strip()
            passes += line == b"PASS"
            failures += line == b"FAIL"
            if line:
                last = line
    return [result.returncode, int(result.returncode in (124, 137)), round(elapsed, 4),
            user, system, rss, counts[0], passes, failures, int(last == b"OK")]


def lane(args, cpu, cases):
    with (args.output / f"cpu-{cpu}.csv").open("w") as f:
        writer = csv.writer(f, lineterminator="\n")
        writer.writerow(FIELDS)
        for index, relative in cases:
            spec = args.examples / relative
            for pair in range(1, args.pairs + 1):
                sample = args.output / str(index) / str(pair)
                engines = [("g3", args.g3), ("native", args.native)]
                if (index + pair) % 2:
                    engines.reverse()
                rows = []
                for name, binary in engines:
                    result = measure(args, spec, binary, sample / name, cpu)
                    rows.append([relative, pair, cpu, args.gogc, name, *result])
                same = [int(filecmp.cmp(sample / "g3" / stream, sample / "native" / stream, shallow=False))
                        for stream in ("stdout", "stderr")]
                for row in rows:
                    writer.writerow([*row, *same])
                f.flush()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("examples", "cases", "g3", "native", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--cpu", type=int, nargs="+", required=True)
    parser.add_argument("--seconds", type=int, default=60)
    parser.add_argument("--pairs", type=int, default=3)
    parser.add_argument("--gogc", type=int, default=100)
    parser.add_argument("--perf", default="perf")
    args = parser.parse_args()
    args.examples = args.examples.absolute()
    args.g3, args.native = args.g3.resolve(strict=True), args.native.resolve(strict=True)
    cases = args.cases.read_text().splitlines()
    if not cases or len(set(cases)) != len(cases) or any(
            Path(case).is_absolute() or ".." in Path(case).parts or not (args.examples / case).is_file()
            for case in cases):
        parser.error("supply distinct existing spec paths relative to the examples checkout")
    if min(args.pairs, args.gogc, args.seconds) < 1 or len(set(args.cpu)) != len(args.cpu):
        parser.error("use distinct pinned CPUs and positive limits")
    args.output = args.output.resolve()
    args.output.mkdir(parents=True)
    indexed = list(enumerate(cases, 1))
    with ThreadPoolExecutor(len(args.cpu)) as pool:
        jobs = [pool.submit(lane, args, cpu, indexed[i::len(args.cpu)]) for i, cpu in enumerate(args.cpu)]
        for job in jobs:
            job.result()


if __name__ == "__main__":
    main()
