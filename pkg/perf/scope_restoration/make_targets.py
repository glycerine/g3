"""Run the repository's Go and C++ scope restoration benchmark targets in alternating order."""
import argparse
import csv
import os
from pathlib import Path
import re
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--pairs", type=int, default=21)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.pairs < 1:
        parser.error("supply a positive pair count")
    args.output.mkdir(parents=True)
    package = Path(__file__).resolve().parents[2]
    row = re.compile(r"^BenchmarkSMTScopeRestoration/(SurvivingAtoms|ScopedAtoms)(?:-\d+)?\s+(\d+)\s+([\d.]+) ns/op\s+128(?:\.0+)? atoms/op\s+64(?:\.0+)? clauses/op(?:\s+([\d.]+) B/op\s+([\d.]+) allocs/op)?\s*$", re.M)
    with (args.output / "runs.csv").open("w") as out:
        writer = csv.writer(out, lineterminator="\n")
        writer.writerow(["pair", "target", "cpu", "benchmark", "iterations", "ns_op", "bytes_op", "allocs_op"])
        for i in range(args.pairs):
            targets = ["bench", "cppbench"]
            for target in targets if i % 2 == 0 else reversed(targets):
                result = subprocess.run(["taskset", "-c", str(args.cpu), "make", target], cwd=package,
                                        env=dict(os.environ, GOMAXPROCS="1", GOGC="100"), text=True,
                                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
                log = args.output / f"{i+1}-{target}.txt"
                log.write_text(result.stdout)
                samples = row.findall(result.stdout)
                if result.returncode or {r[0] for r in samples} != {"SurvivingAtoms", "ScopedAtoms"} or len(samples) != 2:
                    raise ValueError(f"incomplete benchmark in {log}")
                for name, iterations, ns, allocated, allocs in samples:
                    writer.writerow([i+1, target, args.cpu, name, iterations, ns, allocated, allocs])
                out.flush()


if __name__ == "__main__":
    main()
