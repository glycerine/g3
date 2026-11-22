"""Attribute instruction samples in Z3_solver_assert and the corresponding g3 API operation."""

import argparse
from collections import Counter
import csv
import gzip
import os
from pathlib import Path
import shutil
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from holdout_examples import EXAMPLES
from stack_costs import samples


def attribute(path, engine):
    measured = Counter()
    total = assertions = 0
    for stack in samples(path):
        total += 1
        if engine == "native":
            inside = any(s == "Z3_solver_assert" or s.startswith("Z3_solver_assert+") for s in stack)
        else:
            inside = any("g3/pkg/api.(*APISimpleSolver).Assert" in s for s in stack)
        if inside:
            assertions += 1
            measured["leaf", stack[0]] += 1
            for frame in set(stack):
                measured["inclusive", frame] += 1
    if not assertions:
        raise ValueError(f"no assertion stack in {path}")
    return total, assertions, measured


def record(args, out, engine, binary, name):
    relative, _, failures = EXAMPLES[name]
    raw = out / "perf.data"
    command = ["taskset", "-c", str(args.cpu), args.perf, "record", "--no-buildid", "-g", "--call-graph",
               "fp" if engine == "g3" else "dwarf,8192", "-e", "instructions:u", "-c", str(args.period),
               "-m", "512", "-o", raw, "--", binary, args.examples / relative]
    with (out / "stderr").open("wb") as err:
        result = subprocess.run(command, stdout=subprocess.PIPE, stderr=err, cwd=out,
                                env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"))
    if result.returncode != int(failures > 0) or result.stdout != (args.references / (name + ".out")).read_bytes():
        raise ValueError(f"{name}: proof transcript changed")
    stacks = out / "stacks.gz"
    with gzip.open(stacks, "wb", compresslevel=1) as dest:
        with subprocess.Popen([args.perf, "script", "--no-inline", "-i", raw, "-F", "event,ip,sym,dso"],
                              stdout=subprocess.PIPE) as proc:
            shutil.copyfileobj(proc.stdout, dest, 1 << 20)
            if proc.wait():
                raise ValueError(f"{out}: perf script failed")
    raw.unlink()
    return stacks


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("examples", "references", "output", "g3", "native"):
        p.add_argument("--" + name, required=True, type=Path)
    p.add_argument("--example", nargs="+", choices=EXAMPLES, default=["ping-pong", "ticket", "toy-consensus"])
    p.add_argument("--cpu", required=True, type=int)
    p.add_argument("--period", default=800_011, type=int)
    p.add_argument("--pairs", default=3, type=int)
    p.add_argument("--perf", default="perf")
    args = p.parse_args()
    if args.period < 1 or args.pairs < 1 or len(set(args.example)) != len(args.example):
        p.error("choose distinct examples and positive pair and period counts")
    args.examples = args.examples.absolute()
    args.references = args.references.resolve(strict=True)
    root = args.output.resolve()
    root.mkdir(parents=True)
    binaries = {key: getattr(args, key).resolve(strict=True) for key in ("g3", "native")}
    with (root / "samples.csv").open("w") as f:
        writer = csv.writer(f, lineterminator="\n")
        writer.writerow(["example", "pair", "engine", "cpu", "period", "total_samples", "assert_samples",
                         "attribution", "symbol", "hits"])
        for name in args.example:
            for i in range(args.pairs):
                for engine in list(binaries)[::1 if i % 2 == 0 else -1]:
                    out = root / f"{name}-{i+1}-{engine}"
                    out.mkdir()
                    stacks = record(args, out, engine, binaries[engine], name)
                    total, assertions, counts = attribute(stacks, engine)
                    writer.writerows((name, i+1, engine, args.cpu, args.period, total, assertions, *key, value)
                                     for key, value in sorted(counts.items()))
                    f.flush()


if __name__ == "__main__":
    main()
