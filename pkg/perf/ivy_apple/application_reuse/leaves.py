"""Sample instruction addresses while the native and Go application fixtures run."""

import argparse
import csv
from collections import Counter
from concurrent.futures import ThreadPoolExecutor
import os
from pathlib import Path
import re
import subprocess
import sys

from compare import ROOT, build_native
from phase_counters import replace_once

CASES = ((2, 1), (4, 1), (2, 0), (2, 256))


def native_binary(directory, build, iterations):
    source = (ROOT / "src/test/perf/ast_application_reuse.cpp").read_text()
    source = replace_once(source, "constexpr unsigned iterations = 200000;",
                          f"constexpr unsigned iterations = {iterations};")
    start = source.index("    for (unsigned arity :", source.index("int main()"))
    end = source.index("    return 0;", start)
    select = """    unsigned arity = std::stoul(std::getenv("APP_ARITY"));
    unsigned retained = std::stoul(std::getenv("APP_RETAINED"));
    run(arity, retained, retained == 1);
"""
    path = directory / "fixture.cpp"
    path.write_text(source[:start] + select + source[end:])
    binary = directory / "native"
    build_native(build, binary, path)
    return binary


def profile(perf, directory, binary, native, cpu, arity, retained, iterations, period):
    directory.mkdir()
    cmd = [binary]
    if not native:
        bench = (f"^BenchmarkASTCanonicalApplication$/^{arity}$" if retained == 1 else
                 f"^BenchmarkASTApplicationReuse$/^{arity}$/^Resident{retained}$")
        cmd += ["-test.run", "^$", "-test.bench", bench, f"-test.benchtime={iterations}x"]
    env = dict(os.environ, GOMAXPROCS="1", APP_ARITY=str(arity), APP_RETAINED=str(retained))
    data = directory / "perf.data"
    with (directory / "stderr").open("w") as err:
        output = subprocess.check_output(["taskset", "-c", str(cpu), perf, "record", "--no-buildid",
                                          "-e", "instructions:u", "-c", str(period), "-o", data, "--", *cmd],
                                         env=env, stderr=err, text=True)
    if native:
        workload = "single" if retained == 1 else "rotating"
        if f"{workload},{arity},{retained},{iterations}," not in output:
            raise ValueError(output)
    elif not re.search(r"^BenchmarkAST\S+\s+" + str(iterations) + r"\s+", output, re.M):
        raise ValueError(output)
    symbols = Counter()
    script = subprocess.check_output([perf, "script", "--no-inline", "-F", "period,ip,sym,dso", "-i", data], text=True)
    for line in script.splitlines():
        if line.strip():
            weight, _, symbol_and_dso = line.split(maxsplit=2)
            if int(weight) != period:
                raise ValueError(f"{data}: unexpected sample period {weight}")
            symbols[symbol_and_dso.rsplit(" (", 1)[0].strip()] += 1
    if not symbols:
        raise ValueError(f"{data}: no samples")
    return symbols


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", required=True, nargs=2, action="append", metavar=("NAME", "TEST_BINARY"))
    p.add_argument("--cpu", required=True, type=int, nargs=4,
                   help="one CPU per case, in order: repeated binary, repeated arity four, fresh binary, retained binary")
    p.add_argument("--dir", required=True, type=Path, help="fresh output directory for raw perf recordings")
    p.add_argument("--build", type=Path, default=ROOT / "build")
    p.add_argument("--perf", default="perf")
    p.add_argument("--iterations", type=int, default=4_000_000)
    p.add_argument("--period", type=int, default=700_001)
    p.add_argument("--rounds", type=int, default=3)
    args = p.parse_args()
    binaries = {k: Path(path).resolve(strict=True) for k, path in args.go}
    if (len(binaries) != len(args.go) or "native" in binaries or
            any(not re.fullmatch(r"[\w-]+", k) for k in binaries) or
            args.iterations <= 1 or args.rounds < 1 or args.period < 1):
        p.error("supply unique build labels, iterations greater than one, and positive rounds and period")
    directory = args.dir.resolve()
    directory.mkdir(parents=True)
    binaries["native"] = native_binary(directory, args.build.resolve(strict=True), args.iterations)
    names = list(binaries)

    def lane(case):
        (arity, retained), cpu = case
        for i in range(args.rounds):
            order = names[i % len(names):] + names[:i % len(names)]
            for name in order[::-1] if i % 2 else order:
                dest = directory / f"{arity}-{retained}-{i+1}-{name}"
                symbols = profile(args.perf, dest, binaries[name], name == "native", cpu, arity, retained,
                                  args.iterations, args.period)
                total = sum(symbols.values())
                for symbol, votes in symbols.most_common():
                    yield [i+1, cpu, name, arity, retained, args.iterations, args.period, total, symbol, votes]

    out = csv.writer(sys.stdout, lineterminator="\n")
    out.writerow(["round", "cpu", "engine", "arity", "resident", "iterations", "period", "samples", "symbol", "votes"])
    with ThreadPoolExecutor(len(CASES)) as pool:
        jobs = [pool.submit(list, lane(case)) for case in zip(CASES, args.cpu)]
        for job in jobs:
            out.writerows(job.result())
            sys.stdout.flush()


if __name__ == "__main__":
    main()
