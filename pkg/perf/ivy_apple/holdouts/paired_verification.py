"""Compare complete GoIvy runs at explicit GC settings, including negative and inconclusive controls."""
import argparse
import csv
from pathlib import Path
import sys

from complete_search import same_output

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from compare import EVENTS, measure
from holdout_examples import EXAMPLES


APPLE = "doc/examples/apple/ord_live.ivy"
# Each signature records successful obligations, failed obligations, inconclusive results.
CASES = {
    "raft_no_assume_test": ("examples/raft/raft_no_assume_test.ivy", None, 419, 0, 0),
    "raft_no_assume": ("examples/raft/raft_no_assume.ivy", None, 419, 0, 0),
    "raftok": ("examples/raft/raftok.ivy", None, 420, 0, 0),
    "tlb": ("examples/liveness/tlb.ivy", None, 18951, 2, 0),
    "flash2": ("examples/ivy/flash2.ivy", None, 2147, 65, 0),
    "hybrid_reliable_broadcast_cisa": ("examples/liveness/hybrid_reliable_broadcast_cisa.ivy", None, 839, 0, 0),
    "rfn2": (APPLE, "rfn2", 26, 0, 0),
    "cf_live": (APPLE, "cf_live", 336, 0, 0),
    "dramc_nb2": (APPLE, "dramc_nb2", 2479, 0, 0),
    "examp1_numeric": ("doc/examples/cav2024/examp1_numeric.ivy", None, 14, 0, 1),
}
CASES.update({k: (s, None, p, f, 0) for k, (s, p, f) in EXAMPLES.items()})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for argument in ("examples", "g3", "native", "output"):
        parser.add_argument("--" + argument, required=True, type=Path)
    parser.add_argument("--cases", required=True, nargs="+", choices=CASES)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--pairs", type=int, default=5)
    parser.add_argument("--g3-gogc", type=int, default=100)
    parser.add_argument("--native-gogc", type=int, default=100)
    parser.add_argument("--perf", default="perf")
    args = parser.parse_args()
    if min(args.pairs, args.g3_gogc, args.native_gogc) <= 0:
        parser.error("use positive pair counts and GC settings")
    binaries = {k: getattr(args, k).resolve(strict=True) for k in ("g3", "native")}
    root = args.output.resolve()
    root.mkdir(parents=True)
    with (root / "runs.csv").open("w") as file:
        writer = csv.writer(file, lineterminator="\n")
        writer.writerow(["case", "pair", "engine", "gogc", "cpu", "exit", "passes", "failures", "unknown",
                         "same_stdout_bytes", "same_stderr_bytes", "wall_s", "user_s", "sys_s", "rss_kib", *EVENTS])
        for i in range(args.pairs):
            for name in args.cases:
                relative, isolate, passes, failures, unknown = CASES[name]
                config = argparse.Namespace(cpu=args.cpu, perf=args.perf, spec=args.examples.absolute() / relative, isolate=isolate)
                status = int(bool(failures or unknown))
                observations = []
                for engine in binaries if i % 2 == 0 else reversed(binaries):
                    gc = args.g3_gogc if engine == "g3" else args.native_gogc
                    directory = root / name / str(i + 1) / engine
                    code, timing, counters = measure(binaries[engine], config, directory, status, gc)
                    stdout, stderr = ((directory / stream).read_bytes() for stream in ("stdout", "stderr"))
                    verdicts = [line.strip() for line in stdout.splitlines()]
                    outcomes = verdicts.count(b"PASS"), verdicts.count(b"FAIL"), stderr.splitlines().count(b"Solver produced inconclusive result")
                    if code != status or outcomes != (passes, failures, unknown):
                        raise ValueError(f"incomplete or changed verification in {directory}: exit {code}, outcomes {outcomes}")
                    observations.append((engine, gc, stdout, stderr, timing, counters))
                a, b = observations
                if a[3] != b[3] or not same_output(a[2], b[2], unknown):
                    raise ValueError(f"transcript mismatch in round {i+1} for {name}")
                for engine, gc, _, _, timing, counters in observations:
                    writer.writerow([name, i+1, engine, gc, args.cpu, status, passes, failures, unknown,
                                     int(a[2] == b[2]), 1, *timing, *counters])
                file.flush()


if __name__ == "__main__":
    main()
