"""Measure verification and mutation instructions separately in complete short GoIvy runs."""
import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from holdout_examples import EXAMPLES


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--examples", required=True, type=Path)
    parser.add_argument("--reference-panel", required=True, type=Path,
                        help="output directory of holdout_examples.py")
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--workloads", nargs="+", choices=EXAMPLES, default=list(EXAMPLES))
    for mode in ("checks", "mutations"):
        for engine in ("g3", "native"):
            parser.add_argument(f"--{mode}-{engine}", type=Path, required=True,
                                help=f"{engine} checker built by the {mode} interval driver")
    a = parser.parse_args()
    if a.rounds < 1:
        parser.error("supply at least one round")
    root = a.output.resolve()
    root.mkdir(parents=True)
    with (root / "intervals.csv").open("w") as file:
        rows = csv.writer(file, lineterminator="\n")
        rows.writerow(["example", "round", "cpu", "engine", "mode", "sequence", "operation", "instructions",
                       "empty", "enabled_ns", "running_ns", "wall_ns"])
        for name in a.workloads:
            relative, _, failures = EXAMPLES[name]
            spec = (a.examples / relative).resolve(strict=True)
            reference = (a.reference_panel / name / "1-0/stdout").read_bytes()
            observed = {}
            for index in range(a.rounds):
                engines = ("g3", "native") if index % 2 == 0 else ("native", "g3")
                for engine in engines:
                    for mode in ("checks", "mutations"):
                        binary = getattr(a, f"{mode}_{engine}").resolve(strict=True)
                        run = root / f"{name}-{index+1}-{engine}-{mode}"
                        run.mkdir()
                        result = subprocess.run(["taskset", "-c", str(a.cpu), binary, spec], cwd=run, capture_output=True,
                                                env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_THREAD_EVENT="1"))
                        if result.returncode != int(failures > 0) or result.stdout != reference:
                            raise ValueError(f"{run} differs from its reference transcript")
                        prefix = "IVY-THREAD-CHECK " if mode == "checks" else "IVY-MUTATION "
                        series = []
                        for line in result.stderr.decode().splitlines():
                            if line.startswith(prefix):
                                parts = line[len(prefix):].split()
                                op, sequence, *data = (["Check", *parts] if mode == "checks" else parts)
                                if len(data) != 5 or int(data[2]) != int(data[3]) or int(data[2]) <= 0:
                                    raise ValueError(f"{run} has incomplete counter coverage")
                                series.append((sequence, op))
                                rows.writerow([name, index+1, a.cpu, engine, mode, sequence, op, *data])
                        if not series or series != observed.setdefault(mode, series):
                            raise ValueError(f"{run} changed the measured sequence")
                        file.flush()


if __name__ == "__main__":
    main()
