"""Collect CPU or allocation profiles from complete, reference-checked GoIvy runs."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
RAFT = HERE.parent
PKG = RAFT.parents[1]
sys.path.insert(0, str(RAFT))
from measure import transcript

METHOD = "func (p *orderProofState) prove(maxSteps uint32, reuse bool) (proved bool, err error) {"
MODULE = "github.com/glycerine/g3/pkg"


def cpu_overlay(tmp, main, paths):
    src = PKG / "api/api_order_abstraction.go"
    code = src.read_text()
    if code.count(METHOD) != 1 or code.count("import (") != 1:
        raise ValueError(f"unrecognized proof entry point in {src}")
    code = code.replace("import (", 'import (\n"context"\n"runtime/pprof"\n"strconv"')
    # Each measured process owns one sequential Raft run from start to finish.
    label = """
    studyObligation++
    ctx := pprof.WithLabels(context.Background(), pprof.Labels("vc", strconv.Itoa(studyObligation)))
    pprof.SetGoroutineLabels(ctx)
    defer pprof.SetGoroutineLabels(context.Background())
"""
    patched = tmp / src.name
    patched.write_text(code.replace(METHOD, "var studyObligation int\n\n" + METHOD + label))
    subprocess.run(["patch", "--silent", "--output", tmp / main.name, main,
                    RAFT / "goivy-cpu.patch"], check=True)
    paths[str(src)] = str(patched)


def build(ivy, tmp, allocation_rate):
    mod = tmp / "study.mod"
    shutil.copyfile(ivy / "go.mod", mod)
    shutil.copyfile(ivy / "go.sum", tmp / "study.sum")
    subprocess.run(["go", "mod", "edit", "-replace=" + MODULE + "=" + str(PKG), mod],
                   cwd=ivy, check=True)
    main = ivy / "cmd/goivy_check/main.go"
    overlay = tmp / "overlay.json"
    paths = {str(main): str(tmp / main.name)}
    if allocation_rate:
        # Allocations from installing CPU labels would become part of the measurement.
        entry = main.read_text()
        exit_call = "os.Exit(goivy.MainWithConfig(args, cfg))"
        start = "func main() {"
        if entry.count(start) != 1 or entry.count(exit_call) != 1 or entry.count("import (") != 1:
            raise ValueError(f"unrecognized GoIvy entry point in {main}")
        entry = entry.replace("import (", 'import (\n"runtime"\n"runtime/pprof"')
        entry = entry.replace(start, start + f"\n\truntime.MemProfileRate = {allocation_rate}")
        entry = entry.replace(exit_call, """code := goivy.MainWithConfig(args, cfg)
        runtime.GC()
        f, err := os.Create(os.Getenv("IVY_STUDY_PROFILE"))
        if err != nil { panic(err) }
        if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil { panic(err) }
        if err := f.Close(); err != nil { panic(err) }
        os.Exit(code)""")
        (tmp / main.name).write_text(entry)
    else:
        cpu_overlay(tmp, main, paths)
    overlay.write_text(json.dumps({"Replace": paths}))
    exe = tmp / "goivy_check"
    subprocess.run(["go", "build", f"-modfile={mod}", "-mod=readonly", "-buildvcs=false",
                    "-pgo=off", "-tags=xtrace_off", f"-overlay={overlay}", "-o", exe,
                    "./cmd/goivy_check"], cwd=ivy, check=True)
    return exe


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", type=Path, required=True)
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--reference", type=Path, required=True, help="complete native or Python checker stdout")
    parser.add_argument("--isolate", help="profile a named isolate and compare its full stdout byte for byte")
    parser.add_argument("--allocation-rate", type=int, help="sample one allocation per this many bytes on average")
    parser.add_argument("--output", type=Path, required=True, help="merged profile for go tool pprof")
    parser.add_argument("--cpus", type=int, nargs="+", required=True)
    parser.add_argument("--runs", type=int, default=24)
    args = parser.parse_args()
    if args.runs < 1:
        parser.error("--runs must be positive")
    if args.allocation_rate is not None and args.allocation_rate < 1:
        parser.error("--allocation-rate must be positive")
    if args.output.exists():
        parser.error("choose a new output profile path")
    spec = args.spec.absolute()
    if not spec.is_file():
        parser.error(f"specification does not exist: {spec}")

    def checked_output(raw):
        if args.isolate:
            lines = raw.splitlines()
            if lines[-1:] != [b"OK"] or not any(line.strip() == b"PASS" for line in lines):
                raise ValueError("incomplete isolate transcript")
            return raw
        return transcript(raw.decode().splitlines())

    reference = checked_output(args.reference.read_bytes())
    checker_args = ([f"isolate={args.isolate}"] if args.isolate else []) + [spec]
    with tempfile.TemporaryDirectory() as name:
        tmp = Path(name)
        exe = build(args.goivy.resolve(strict=True), tmp, args.allocation_rate)

        def lane(index):
            for i in range(index, args.runs, len(args.cpus)):
                env = dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1")
                key = "IVY_STUDY_PROFILE" if args.allocation_rate else "IVY_STUDY_CPU"
                env[key] = str(tmp / f"{i}.pb")
                result = subprocess.run(["taskset", "-c", str(args.cpus[index]), exe, *checker_args],
                                        cwd=tmp, env=env, capture_output=True, check=True)
                if checked_output(result.stdout) != reference:
                    raise ValueError(f"GoIvy transcript differs on run {i + 1}")

        with ThreadPoolExecutor(len(args.cpus)) as executor:
            list(executor.map(lane, range(len(args.cpus))))
        merged = tmp / "merged.pprof"
        subprocess.run(["go", "tool", "pprof", "-proto", "-output", merged, exe,
                        *(tmp / f"{i}.pb" for i in range(args.runs))], check=True)
        shutil.copyfile(merged, args.output)


if __name__ == "__main__":
    main()
