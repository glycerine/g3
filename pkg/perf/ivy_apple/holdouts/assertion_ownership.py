"""Compare the cost of publishing justified assertion batches in g3 and the in-tree Z3."""
import argparse
import csv
import io
from pathlib import Path
import re
import subprocess
import sys

HERE = Path(__file__).resolve().parent
G3 = HERE.parents[3]
sys.path.insert(0, str(HERE.parent))
from relevance_counts import make_value


def build_fixtures(build, output):
    go = output / "smtcore.test"
    subprocess.run(["go", "test", "-c", "-pgo=off", "-o", go, "./smtcore"],
                   cwd=G3 / "pkg", check=True)
    source = G3 / "src/test/perf/asserted_batch_replacement.cpp"
    config = (build / "config.mk").read_text()
    makefile = (build / "Makefile").read_text()
    rule = re.search(r"\$\((includes_\d+)\).*\.\./src/smt/asserted_formulas\.cpp$", makefile, re.M)
    if rule is None:
        raise ValueError("asserted_formulas.cpp compile rule missing from C++ build")
    cxx = make_value(config, "CXX")
    obj, native = output / "fixture.o", output / "native"
    subprocess.run([*cxx, *make_value(config, "CXXFLAGS"), *make_value(makefile, rule[1]),
                    "-fno-access-control", "-o", obj, source], cwd=build, check=True)
    subprocess.run([*cxx, obj, build / "libz3.a", "-fopenmp", "-lpthread", "-ldl", "-lrt",
                    "-o", native], check=True)
    return {"g3": go, "native": native}


def observations(engine, binary, cpu):
    command = ["taskset", "-c", str(cpu), binary]
    if engine != "native":
        command.extend(["-test.run=^$", "-test.bench=^BenchmarkSMTAssertedBatchReplacement$",
                        "-test.benchtime=3000x", "-test.benchmem", "-test.cpu=1"])
    result = subprocess.check_output(command, cwd=G3 / "pkg/smtcore", text=True)
    if engine == "native":
        records = [(r["position"], r["width"], r["iterations"], r["ns"])
                   for r in csv.DictReader(io.StringIO(result))]
    else:
        records = re.findall(r"BenchmarkSMTAssertedBatchReplacement/(\w+)/(\d+)\s+(\d+)\s+([\d.]+) ns/op",
                             result)
    expected = {(p, w, "3000") for p in ("First", "Identical", "Last") for w in ("8", "128", "1024")}
    if len(records) != len(expected) or {r[:3] for r in records} != expected:
        raise RuntimeError(f"unexpected benchmark cells from {binary}")
    return records


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cpp-build", type=Path, default=G3 / "build")
    parser.add_argument("--previous", type=Path, help="Optional prior smtcore.test with the same fixture")
    parser.add_argument("--rounds", type=int, default=21)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.rounds < 1:
        parser.error("--rounds must be positive")
    output = args.output.resolve()
    output.mkdir(parents=True)
    bins = build_fixtures(args.cpp_build.resolve(), output)
    if args.previous:
        bins["previous"] = args.previous.resolve(strict=True)
    engines = list(bins)
    with (output / "runs.csv").open("w", newline="") as file:
        writer = csv.writer(file)
        writer.writerow(["round", "engine", "position", "width", "iterations", "ns"])
        for i in range(args.rounds):
            start = i % len(engines)
            for engine in engines[start:] + engines[:start]:
                for row in observations(engine, bins[engine], args.cpu):
                    writer.writerow([i + 1, engine, *row])
                file.flush()


if __name__ == "__main__":
    main()
