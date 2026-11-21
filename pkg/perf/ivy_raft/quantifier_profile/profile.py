"""Profile complete Raft checks and label samples inside each private proof attempt."""

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


def build(ivy, tmp):
    mod = tmp / "study.mod"
    shutil.copyfile(ivy / "go.mod", mod)
    shutil.copyfile(ivy / "go.sum", tmp / "study.sum")
    subprocess.run(["go", "mod", "edit", "-replace=" + MODULE + "=" + str(PKG), mod],
                   cwd=ivy, check=True)
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
    main = ivy / "cmd/goivy_check/main.go"
    subprocess.run(["patch", "--silent", "--output", tmp / main.name, main,
                    RAFT / "goivy-cpu.patch"], check=True)
    overlay = tmp / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {str(src): str(patched), str(main): str(tmp / main.name)}}))
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
    parser.add_argument("--output", type=Path, required=True, help="merged profile for go tool pprof")
    parser.add_argument("--cpus", type=int, nargs="+", required=True)
    parser.add_argument("--runs", type=int, default=24)
    args = parser.parse_args()
    if args.runs < 1:
        parser.error("--runs must be positive")
    if args.output.exists():
        parser.error("choose a new output profile path")
    spec = args.spec.resolve(strict=True)
    reference = transcript(args.reference.read_text().splitlines())
    with tempfile.TemporaryDirectory() as name:
        tmp = Path(name)
        exe = build(args.goivy.resolve(strict=True), tmp)

        def lane(index):
            for i in range(index, args.runs, len(args.cpus)):
                env = dict(os.environ, IVY_STUDY_CPU=str(tmp / f"{i}.cpu"), GOMAXPROCS="1", XTRACE_OFF="1")
                result = subprocess.run(["taskset", "-c", str(args.cpus[index]), exe, spec],
                                        cwd=tmp, env=env, capture_output=True, text=True, check=True)
                if transcript(result.stdout.splitlines()) != reference:
                    raise ValueError(f"Raft transcript differs on run {i + 1}")

        with ThreadPoolExecutor(len(args.cpus)) as executor:
            list(executor.map(lane, range(len(args.cpus))))
        merged = tmp / "merged.pprof"
        subprocess.run(["go", "tool", "pprof", "-proto", "-output", merged, exe,
                        *(tmp / f"{i}.cpu" for i in range(args.runs))], check=True)
        shutil.copyfile(merged, args.output)


if __name__ == "__main__":
    main()
