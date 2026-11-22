"""Measure GoIvy fragment naming across two revisions on a complete specification."""

import argparse
import csv
from pathlib import Path
import sys
from types import SimpleNamespace

from spec_survey import measure

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from frontend_memory import compile_checker, METRICS


def checker(goivy, output, engine):
    args = SimpleNamespace(output=output, goivy=goivy, frontend_overlay=None, go_overlay=None,
                           modfile=None, ballast_mib=[], count_calls=False)
    return compile_checker(args, engine)


def memory_and_errors(path):
    metrics = {}
    diagnostic = bytearray()
    with path.open("rb") as source:
        for line in source:
            if line.startswith(b"IVY-MEMORY "):
                _, key, value = line.decode().split()
                metrics[key] = value
            else:
                diagnostic.extend(line)
    if set(metrics) != set(METRICS):
        raise ValueError(f"incomplete runtime metrics in {path}")
    return metrics, diagnostic


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("before", "after", "spec", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--seconds", type=int, default=300)
    parser.add_argument("--pairs", type=int, default=4)
    parser.add_argument("--gogc", type=int, default=100)
    parser.add_argument("--perf", default="perf")
    args = parser.parse_args()
    args.spec = args.spec.resolve(strict=True)
    args.output = args.output.resolve()
    if min(args.seconds, args.pairs, args.gogc) < 1:
        parser.error("supply positive time, pair count, and GC setting")
    args.output.mkdir(parents=True)
    paths = {}
    for variant in ("before", "after"):
        source = getattr(args, variant).resolve(strict=True)
        target = args.output / variant
        target.mkdir()
        for engine in ("g3", "native"):
            paths[variant, engine] = checker(source, target, engine)
    csvout = csv.writer(sys.stdout, lineterminator="\n")
    csvout.writerow(["case", "cpu", "round", "build", "engine", "exit", "same_stdout", "same_stderr",
                     "wall_s", "user_s", "sys_s", "rss_kib", "instructions", *METRICS])
    reference = None
    for pair in range(1, args.pairs + 1):
        variants = ("before", "after") if pair % 2 else ("after", "before")
        engines = ("g3", "native") if pair % 2 else ("native", "g3")
        for variant in variants:
            for engine in engines:
                directory = args.output / f"{pair}-{variant}-{engine}"
                observed = measure(args, args.spec, paths[variant, engine], directory, args.cpu)
                if observed[1]:
                    raise ValueError(f"checker exceeded the time limit in {directory}")
                metrics, diagnostics = memory_and_errors(directory / "stderr")
                result = observed[0], (directory / "stdout").read_bytes(), diagnostics
                if reference is None:
                    reference = result
                if result != reference:
                    raise ValueError(f"exit or output differs from the first run in {directory}")
                csvout.writerow([args.spec.name, args.cpu, pair, variant, engine, observed[0], 1, 1,
                                 *observed[2:7], *(metrics[key] for key in METRICS)])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
