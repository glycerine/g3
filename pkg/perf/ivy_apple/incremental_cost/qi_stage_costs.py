"""Measure quantified-instance stages on both solvers during an ordinary GoIvy check."""
import argparse
import csv
import gzip
import os
from pathlib import Path
import subprocess

from first_push_rewrite_rows import G3, HERE, build_checker, native_library, stage_patch

STAGES = ("entry", "candidate", "substitution", "rewrite", "lemma", "internalization")


def checker(args, directory, engine):
    directory.mkdir()
    with gzip.open(HERE / f"nov23-qi-phase-{engine}.patch.gz", "rt") as f:
        sources = stage_patch(f.read(), args.goivy, directory)
    native = engine == "native"
    flags = native_library(args, directory, sources, "qi_queue", "ivy_qi_stages.cpp", "Z3_solver_check") if native else []
    return build_checker(args.goivy, directory, sources, args.abc, native, flags)


def measure(binary, args, directory):
    directory.mkdir()
    cmd = ["taskset", "-c", str(args.cpu), binary]
    if args.isolate:
        cmd.append("isolate=" + args.isolate)
    result = subprocess.run([*cmd, args.spec.resolve(strict=True)], cwd=directory, capture_output=True,
                            env=dict(os.environ, GOMAXPROCS="1", GOGC="100", XTRACE_OFF="1", SCOPE_THREAD_EVENT="1"))
    current = None
    diagnostics = bytearray()
    with gzip.open(directory / "counts.csv.gz", "wt", newline="") as out:
        writer = csv.writer(out, lineterminator="\n")
        writer.writerow(("batch", "condition", "stage", "calls", "instructions"))
        for line in result.stderr.splitlines(keepends=True):
            if not line.startswith((b"Q-BEGIN ", b"Q-END ", b"Q-STAGE ")):
                diagnostics.extend(line)
                continue
            key, *fields = line.split()
            if key == b"Q-BEGIN":
                if current is not None:
                    raise ValueError("overlapping check boundaries")
                current = tuple(map(int, fields))
            elif key == b"Q-END":
                if current != tuple(map(int, fields)):
                    raise ValueError("unmatched check boundary")
                current = None
            elif key == b"Q-STAGE":
                if current is not None:
                    stage, calls, instructions = map(int, fields)
                    writer.writerow((*current, STAGES[stage], calls, instructions))
    (directory / "stdout").write_bytes(result.stdout)
    (directory / "stderr").write_bytes(diagnostics)
    if current is not None or result.returncode != args.expected_exit:
        raise ValueError(f"{directory}: exit {result.returncode}, {diagnostics[-300:]!r}")
    return result.stdout, bytes(diagnostics)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    parser.add_argument("--cpp-build", type=Path, default=G3 / "build")
    parser.add_argument("--abc", type=Path)
    parser.add_argument("--isolate")
    parser.add_argument("--expected-exit", type=int, default=0)
    parser.add_argument("--pairs", type=int, default=3)
    parser.add_argument("--cpu", required=True, type=int)
    args = parser.parse_args()
    if args.pairs < 1:
        parser.error("--pairs must be positive")
    args.goivy = args.goivy.resolve(strict=True)
    directory = args.output.resolve()
    directory.mkdir(parents=True)
    engines = ("g3", "native")
    binaries = {e: checker(args, directory / e, e) for e in engines}
    reference = None
    for pair in range(1, args.pairs + 1):
        for engine in engines if pair % 2 else reversed(engines):
            transcript = measure(binaries[engine], args, directory / f"{pair}-{engine}")
            if reference is None:
                reference = transcript
            elif reference != transcript:
                raise ValueError(f"different verification output in pair {pair}, {engine}")


if __name__ == "__main__":
    main()
