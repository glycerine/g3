"""Measure which operations account for additional matched checks on a warm SMT solver."""
import argparse
from collections import Counter
import csv
from functools import lru_cache
import os
from pathlib import Path
import subprocess
import sys

from matched_stack_counts import frame_classifier
from warm_queries import CASES, HERE, NUMERIC_ANSWERS, run, script

sys.path.insert(0, str(HERE.parent))
from stack_costs import FRAME


@lru_cache(None)
def command_frame(engine, frame):
    names = (("assert", "Assert", "assert_expr"), ("push", "Push", "push"),
             ("check", "CheckSat", "check_sat"), ("pop", "Pop", "pop"))
    for label, go, cpp in names:
        if engine == "g3" and frame.endswith(tuple("cmd_context.(*Context)." + n for n in (go, go + "Display", go + "WithText"))):
            return label
        if engine == "native" and "cmd_context::" + cpp + "(" in frame:
            return label
    return None


def observe(args, engine, binary, query, iterations, directory, classify):
    directory.mkdir()
    data = directory / "perf.data"
    command = ["taskset", "-c", str(args.cpu), args.perf, "record", "--no-buildid", "-m", "1024",
               "-e", "instructions:u", "-c", str(args.period), "--call-graph",
               "fp" if engine == "g3" else "dwarf,8192", "-o", data, "--", binary, "-in", "-smt2"]
    traced = subprocess.run(command, input=script(query, iterations, False), capture_output=True,
                            text=True, check=True, timeout=args.timeout,
                            env=dict(os.environ, GOMAXPROCS="1", GOGC=args.gogc))
    if traced.stdout.splitlines() != [NUMERIC_ANSWERS.get(query, "unsat")] * iterations or "lost" in traced.stderr.lower():
        raise ValueError(f"unexpected output or lost samples in {directory}")
    histogram = Counter()
    frames = None

    def record():
        if frames is not None:
            operation = next((v for f in frames if (v := classify(engine, f))), "unattributed")
            phase = next((v for f in frames if (v := command_frame(engine, f))), "outside or unresolved")
            owner = ""
            if operation == "rewriter":
                owner = next((v for f in frames if (v := classify(engine, f)) and v != "rewriter"), "unattributed")
            histogram[phase, operation, owner] += 1

    with (directory / "decode.log").open("w") as log, subprocess.Popen(
            [args.perf, "script", "--no-inline", "-i", data, "-F", "event,ip,sym,dso"],
            stdout=subprocess.PIPE, stderr=log, text=True) as proc:
        for line in proc.stdout:
            if line.strip() == "instructions:u:":
                record()
                frames = []
            elif line.strip():
                frame = FRAME.fullmatch(line)
                if frames is None or not frame:
                    raise ValueError(f"unrecognized perf frame in {directory}: {line!r}")
                frames.append(frame[1])
        record()
        if proc.wait() or not histogram:
            raise ValueError(f"no complete trace in {directory}")
    if "lost" in (directory / "decode.log").read_text().lower():
        raise ValueError(f"lost samples in {directory}")
    data.unlink()
    return histogram


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for arg in ("g3", "native", "output"):
        parser.add_argument("--" + arg, type=Path, required=True)
    parser.add_argument("--queries", nargs="+", required=True, choices=(*CASES, *NUMERIC_ANSWERS))
    parser.add_argument("--iterations", nargs=2, type=int, default=[4, 32])
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--pairs", type=int, default=5)
    parser.add_argument("--period", type=int, default=1000003)
    parser.add_argument("--timeout", type=int, default=240)
    parser.add_argument("--gogc", default="100")
    parser.add_argument("--perf", default="perf")
    args = parser.parse_args()
    if min(args.period, args.pairs, args.timeout, *args.iterations) < 1 or args.iterations[0] >= args.iterations[1]:
        parser.error("use positive limits and two increasing iteration counts")
    args.output.mkdir(parents=True)
    binaries = {k: getattr(args, k).resolve(strict=True) for k in ("g3", "native")}
    classify = frame_classifier()
    with (args.output / "samples.csv").open("w") as out, (args.output / "search.csv").open("w") as checks:
        wr = csv.writer(out, lineterminator="\n")
        wr.writerow(["query", "pair", "engine", "cpu", "gogc", "iterations", "period", "command", "category", "rewriter_caller", "samples"])
        audit = csv.writer(checks, lineterminator="\n")
        audit.writerow(["query", "iteration", "decisions", "equalities_or_instantiations", "conflicts", "resource_steps"])
        for query in args.queries:
            search = [run(args, binary, query, args.output / f"{query}-{engine}-search", args.iterations[1], True)[1]
                      for engine, binary in binaries.items()]
            if search[0] != search[1]:
                raise ValueError(f"{query} changed search within the measured iterations")
            audit.writerows([query, i, *values] for i, values in enumerate(search[0], 1))
            checks.flush()
            for i in range(args.pairs):
                for n in args.iterations if i % 2 == 0 else reversed(args.iterations):
                    for engine in binaries if i % 2 == 0 else reversed(binaries):
                        directory = args.output / f"{query}-{i+1}-{n}-{engine}"
                        counts = observe(args, engine, binaries[engine], query, n, directory, classify)
                        wr.writerows([query, i+1, engine, args.cpu, args.gogc, n, args.period, *k, count]
                                     for k, count in sorted(counts.items()))
                        out.flush()


if __name__ == "__main__":
    main()
