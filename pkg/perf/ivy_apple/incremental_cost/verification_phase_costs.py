"""Compare solver-thread instruction counts by operation on a complete Ivy input."""

import argparse
import csv
import gzip
import os
from pathlib import Path
import subprocess

from first_push_rewrite_rows import build_checker, stage_patch


def run_case(binary, args, directory):
    cmd = ["taskset", "-c", str(args.cpu), str(binary)]
    if args.isolate:
        cmd.append("isolate=" + args.isolate)
    cmd.append(str(args.spec.resolve(strict=True)))
    result = subprocess.run(cmd, cwd=directory, capture_output=True,
                            env=dict(os.environ, GOMAXPROCS="1", GOGC="100", XTRACE_OFF="1"))
    phases = []
    diagnostics = bytearray()
    for line in result.stderr.splitlines(keepends=True):
        if line.startswith(b"VC-PHASE "):
            _, batch, condition, stage, verdict, count = line.split()
            phases.append((int(batch), int(condition), stage.decode(), int(verdict), int(count)))
        else:
            diagnostics.extend(line)
    if result.returncode != args.expected_exit or not phases:
        raise ValueError(f"{directory}: exit {result.returncode}, {len(phases)} rows, {diagnostics[-300:]!r}")
    (directory / "stdout").write_bytes(result.stdout)
    (directory / "stderr").write_bytes(diagnostics)
    with gzip.open(directory / "phases.csv.gz", "wt", newline="") as stream:
        writer = csv.writer(stream, lineterminator="\n")
        writer.writerow(("batch", "condition", "stage", "result", "instructions"))
        writer.writerows(phases)
    return result.stdout, bytes(diagnostics)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "output"):
        p.add_argument("--" + name, type=Path, required=True)
    p.add_argument("--isolate")
    p.add_argument("--abc", type=Path)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--expected-exit", type=int, default=0)
    p.add_argument("--pairs", type=int, default=3)
    args = p.parse_args()
    if args.pairs < 1:
        p.error("--pairs must be positive")
    root = args.output.resolve()
    root.mkdir(parents=True)
    patch = Path(__file__).with_name("nov23-verification-phase-counter.patch.gz")
    with gzip.open(patch, "rt") as stream:
        src = stage_patch(stream.read(), args.goivy.resolve(), root)
    bins = {}
    for engine in ("g3", "native"):
        stage = root / engine
        stage.mkdir()
        bins[engine] = build_checker(args.goivy.resolve(), stage, src, args.abc, engine == "native")
    reference = None
    for i in range(1, args.pairs + 1):
        for engine in (bins if i % 2 else reversed(bins)):
            directory = root / f"{i}-{engine}"
            directory.mkdir()
            transcript = run_case(bins[engine], args, directory)
            if reference is None:
                reference = transcript
            elif transcript != reference:
                raise ValueError(f"different verification transcript in {directory}")


if __name__ == "__main__":
    main()
