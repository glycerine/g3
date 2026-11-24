"""Measure each preprocessing pass inside the first physical push of a fresh solver."""
import argparse
from bisect import bisect_left
import csv
import gzip
import os
from pathlib import Path
import subprocess

from first_push_rewrite_rows import G3, HERE, build_checker, native_library, stage_patch


def prepare(args):
    stage = args.output / "build"
    stage.mkdir()
    subprocess.run(["go", "work", "init", G3 / "pkg", args.goivy], cwd=stage,
                   env=dict(os.environ, GOWORK="off"), check=True)
    os.environ["GOWORK"] = str(stage / "go.work")
    name = "detail" if args.detail else "observers"
    patch = gzip.decompress((HERE / f"nov24-first-pass-{name}.patch.gz").read_bytes()).decode()
    sources = stage_patch(patch, args.goivy, stage)
    bins = {}
    for engine in ("g3", "native"):
        build = stage / ("bin-" + engine)
        build.mkdir()
        files = dict(sources)
        flags = native_library(args, build, files, counter=None) if engine == "native" else []
        bins[engine] = build_checker(args.goivy, build, files, args.abc, engine == "native", flags)
    return bins


def measure(args, binary, directory):
    directory.mkdir()
    command = ["taskset", "-c", str(args.cpu), binary]
    if args.isolate:
        command.append("isolate=" + args.isolate)
    command.append(args.spec)
    proc = subprocess.run(command, cwd=directory, capture_output=True,
                          env=dict(os.environ, GOMAXPROCS="1", GOGC="100", XTRACE_OFF="1"))
    windows, passes, ordinary = [], [], bytearray()
    for line in proc.stderr.splitlines(keepends=True):
        if line.startswith(b"W-INTERVAL "):
            _, group, phase, _, tid, start, end = line.split()
            if phase == b"first":
                windows.append(tuple(map(int, (group, tid, start, end))))
        elif line.startswith(b"ASSERTED-PASS "):
            _, name, *values = line.split()
            passes.append((name.decode(), *map(int, values)))
        else:
            ordinary.extend(line)
    if (proc.returncode != args.expect_exit or proc.stdout != (args.reference / "stdout").read_bytes()
            or ordinary != (args.reference / "stderr").read_bytes()):
        (directory / "stdout").write_bytes(proc.stdout)
        (directory / "stderr").write_bytes(proc.stderr)
        raise RuntimeError(f"verification output differed in {directory}")
    ends = [w[3] for w in windows]
    if ends != sorted(ends):
        raise ValueError("overlapping or out-of-order first pushes")
    recorded = 0
    with (directory / "passes.csv").open("w", newline="") as file:
        writer = csv.writer(file)
        writer.writerow(["push", "stage", "instructions", "empty"])
        for name, tid, start, end, ins, empty in passes:
            i = bisect_left(ends, start)
            if i < len(windows):
                group, thread, opened, closed = windows[i]
                if tid == thread and opened <= start <= end <= closed:
                    writer.writerow([group, name, ins, empty])
                    recorded += 1
    if windows and not recorded:
        raise RuntimeError(f"no preprocessing pass recorded in {len(windows)} first pushes")
    return len(windows)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", type=Path, required=True)
    parser.add_argument("--cpp-build", type=Path, default=G3 / "build")
    parser.add_argument("--abc", type=Path)
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--reference", type=Path, required=True, help="Directory with expected stdout and stderr")
    parser.add_argument("--expect-exit", type=int, default=0)
    parser.add_argument("--isolate", default="")
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--detail", action="store_true",
                        help="Include nested intervals for formula loops and propagation directions")
    parser.add_argument("--rounds", type=int, default=5)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.rounds < 1:
        parser.error("--rounds must be positive")
    # GoIvy includes the input spelling in source locations on stdout.
    args.spec = args.spec.absolute()
    for name in ("goivy", "cpp_build", "reference", "abc", "output"):
        if (path := getattr(args, name)) is not None:
            setattr(args, name, path.resolve())
    for path in (args.spec, args.reference / "stdout", args.reference / "stderr"):
        if not path.is_file():
            parser.error(f"missing input {path}")
    args.output.mkdir(parents=True)
    binaries = prepare(args)
    with (args.output / "runs.csv").open("w", newline="") as file:
        writer = csv.writer(file)
        writer.writerow(["round", "engine", "first_pushes"])
        for i in range(args.rounds):
            for engine in list(binaries)[::1 if i % 2 == 0 else -1]:
                count = measure(args, binaries[engine], args.output / f"{i+1}-{engine}")
                writer.writerow([i+1, engine, count])
                file.flush()


if __name__ == "__main__":
    main()
