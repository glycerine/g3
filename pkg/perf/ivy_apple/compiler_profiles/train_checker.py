"""Train an optional Go compiler profile for each GoIvy backend using Raft and Apple cmpl_live."""

import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile


BACKENDS = {"g3": "xtrace_off", "native": "classic_z3,xtrace_off"}


def build(checkout, output, engine, profile, overlay=None, modfile=None):
    cmd = ["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=" + str(profile),
           "-tags=" + BACKENDS[engine], "-o", output]
    if overlay:
        cmd.append("-overlay=" + str(overlay))
    if modfile:
        cmd.append("-modfile=" + str(modfile))
    subprocess.run([*cmd, "./cmd/goivy_check"], cwd=checkout, check=True)


def train(args, output, scratch):
    source = args.goivy / "cmd/goivy_check/main.go"
    patch = Path(__file__).resolve().parents[2] / "ivy_raft/goivy-cpu.patch"
    modified = scratch / "main.go"
    with modified.open("wb") as f:
        subprocess.run(["patch", "--silent", "--output", "-", source, patch], stdout=f, check=True)
    overlay = scratch / "profile.json"
    overlay.write_text(json.dumps({"Replace": {str(source): str(modified)}}))
    references = {name: (args.references / (name + ".out")).read_bytes() for name in ("raft", "cmpl_live")}
    workloads = {"raft": [args.raft], "cmpl_live": ["isolate=cmpl_live", args.apple]}
    for engine in args.engines:
        checker = scratch / (engine + "-training")
        build(args.goivy, checker, engine, "off", overlay=overlay, modfile=args.modfile)
        profiles = []
        for pair in range(1, args.pairs + 1):
            for name, invocation in workloads.items():
                run = scratch / f"{engine}-{name}-{pair}"
                run.mkdir()
                profile = run / "cpu.pprof"
                with (run / "stderr").open("wb") as err:
                    result = subprocess.run(["taskset", "-c", str(args.cpu), checker, *invocation], cwd=run,
                                            stdout=subprocess.PIPE, stderr=err,
                                            env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1",
                                                     IVY_STUDY_CPU=str(profile)))
                if result.returncode or result.stdout != references[name]:
                    message = (run / "stderr").read_text(errors="replace")
                    raise ValueError(f"{engine} training on {name} failed or changed the proof transcript\n{message}")
                profiles.append(profile)
        merged = output / (engine + ".pprof")
        subprocess.run(["go", "tool", "pprof", "-proto", "-output", merged, *profiles], check=True)
        build(args.goivy, output / ("goivy-" + engine), engine, merged, modfile=args.modfile)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for arg in ("goivy", "raft", "apple", "references", "output"):
        parser.add_argument("--" + arg, type=Path, required=True)
    parser.add_argument("--modfile", type=Path, help="optional GoIvy module file with a local g3 replace")
    parser.add_argument("--engines", nargs="+", choices=BACKENDS, default=list(BACKENDS))
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--pairs", type=int, default=3, help="number of complete runs of each training workload")
    args = parser.parse_args()
    if args.pairs < 1 or len(set(args.engines)) != len(args.engines):
        parser.error("supply a positive run count and distinct backends")
    args.goivy = args.goivy.resolve(strict=True)
    args.references = args.references.resolve(strict=True)
    if args.modfile:
        args.modfile = args.modfile.resolve(strict=True)
    # GoIvy writes the spelling of each source path to the proof transcript.
    args.raft, args.apple = args.raft.absolute(), args.apple.absolute()
    if not args.raft.is_file() or not args.apple.is_file():
        parser.error("both Ivy specifications must exist")
    output = args.output.resolve()
    output.mkdir(parents=True)
    with tempfile.TemporaryDirectory() as scratch:
        train(args, output, Path(scratch))


if __name__ == "__main__":
    main()
