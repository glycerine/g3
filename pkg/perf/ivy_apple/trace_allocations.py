"""Attribute Go heap allocations to arguments of disabled GoIvy trace calls."""
import argparse
import collections
import csv
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys


def capture(args):
    original = args.goivy / "cmd/goivy_check/main.go"
    code = original.read_text()
    call = "os.Exit(goivy.MainWithConfig(args, cfg))"
    if code.count(call) != 1:
        raise ValueError(f"cannot locate the single checker entry point in {original}")
    code = code.replace("import (", 'import (\n"runtime"\n"runtime/pprof"', 1)
    code = code.replace(call, """result := goivy.MainWithConfig(args, cfg)
    runtime.GC()
    file, err := os.Create("after.heap")
    if err != nil { panic(err) }
    if err = pprof.WriteHeapProfile(file); err != nil { panic(err) }
    if err = file.Close(); err != nil { panic(err) }
    os.Exit(result)""")
    instrumented = args.output / "main.go"
    instrumented.write_text(code)
    overlay = args.output / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {str(original): str(instrumented)}}))
    checker = args.output / "checker"
    tags = "classic_z3,xtrace_off" if args.engine == "native" else "xtrace_off"
    flags = ["-modfile=" + str(args.modfile.resolve(strict=True))] if args.modfile else []
    subprocess.run(["go", "build", "-pgo=off", "-buildvcs=false", "-mod=readonly", *flags, "-tags=" + tags,
                    "-overlay=" + str(overlay), "-o", checker, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    command = ["taskset", "-c", str(args.cpu), checker]
    if args.isolate:
        command.append("isolate=" + args.isolate)
    command.append(args.spec.absolute())
    with (args.output / "stdout").open("wb") as out:
        run = subprocess.run(command, cwd=args.output, env=dict(os.environ, XTRACE_OFF="1", GOMAXPROCS="1",
                             GOGC=str(args.gogc)), stdout=out)
    if run.returncode != args.exit_status or (args.output / "stdout").read_bytes() != args.reference.read_bytes():
        raise ValueError("reference transcript or exit status differs")
    return checker, args.output / "after.heap"


def attribute(raw, directory, sites):
    samples, locations = raw.split("\nLocations\n", 1)
    source_lines = {}
    for s in sites:
        for line in range(s["First"], s["Last"] + 1):
            source_lines[s["File"], line] = (s["File"], s["Function"], s["First"])
    frames = collections.defaultdict(list)
    index = None
    for line in locations.splitlines():
        if line.startswith("Mappings"):
            break
        start = re.match(r"\s*(\d+): 0x", line)
        if start:
            index = int(start[1])
        filename = re.search(r"/([\w.-]+\.go):(\d+):\d+ s=", line)
        if index is not None and filename and str(directory) + "/" in line:
            site = source_lines.get((filename[1], int(filename[2])))
            if site:
                frames[index].append(site)
    totals = collections.defaultdict(lambda: [0, 0])
    all_bytes = 0
    for line in samples.splitlines():
        sample = re.fullmatch(r"\s*(\d+)\s+(\d+)\s+\d+\s+\d+: ([\d ]+)", line)
        if not sample:
            continue
        objects, size = int(sample[1]), int(sample[2])
        all_bytes += size
        # A sample can contain nested trace arguments. The outermost site is
        # sufficient to avoid all of its allocation when rendering is disabled.
        for loc in reversed(sample[3].split()):
            matches = frames.get(int(loc))
            if matches:
                value = totals[matches[-1]]
                value[0] += objects
                value[1] += size
                break
    if not all_bytes:
        raise ValueError("no allocation samples found in go tool pprof -raw output")
    return all_bytes, totals


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ["goivy", "spec", "output", "reference"]:
        parser.add_argument("--" + name, required=True, type=Path)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--isolate")
    parser.add_argument("--engine", choices=["g3", "native"], default="g3")
    parser.add_argument("--gogc", type=int, default=100)
    parser.add_argument("--modfile", type=Path)
    parser.add_argument("--exit-status", type=int, default=0)
    args = parser.parse_args()
    args.goivy = args.goivy.resolve(strict=True)
    args.output = args.output.resolve()
    args.output.mkdir(parents=True)
    checker, heap = capture(args)
    helper = args.output / "trace_sites.go"
    shutil.copyfile(Path(__file__).with_name("trace_sites.go.txt"), helper)
    sites = json.loads(subprocess.check_output(["go", "run", helper, args.goivy]))
    raw = subprocess.check_output(["go", "tool", "pprof", "-raw", checker, heap], text=True)
    all_bytes, totals = attribute(raw, args.goivy, sites)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["file", "function", "line", "objects", "bytes", "all_bytes"])
    for site, values in sorted(totals.items(), key=lambda item: -item[1][1]):
        writer.writerow([*site, *values, all_bytes])


if __name__ == "__main__":
    main()
