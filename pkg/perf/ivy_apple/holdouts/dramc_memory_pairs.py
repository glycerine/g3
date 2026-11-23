"""Compare default and large Go heaps in alternating full dramc_nb2 verification pairs."""
import argparse
import csv
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from compare import EVENTS, measure


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("g3", "native", "spec", "reference", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--pairs", default=5, type=int)
    parser.add_argument("--large-g3-gogc", default=415, type=int)
    parser.add_argument("--large-native-gogc", default=565, type=int)
    parser.add_argument("--perf", default="perf")
    args = parser.parse_args()
    if min(args.pairs, args.large_g3_gogc, args.large_native_gogc) <= 0:
        parser.error("pairs and heap settings must be positive")
    args.spec = args.spec.absolute()  # GoIvy prints the caller's spelling of the path.
    args.isolate = "dramc_nb2"
    expected = args.reference.read_bytes()
    if sum(line.strip() == b"PASS" for line in expected.splitlines()) != 2479:
        parser.error("supply stdout from a successful 2,479-check dramc_nb2 verification")
    binaries = {name: getattr(args, name).resolve(strict=True) for name in ("g3", "native")}
    settings = {
        "default": {"g3": 100, "native": 100},
        "large": {"g3": args.large_g3_gogc, "native": args.large_native_gogc},
    }
    output = args.output.resolve()
    output.mkdir(parents=True)
    with (output / "runs.csv").open("w") as file:
        writer = csv.writer(file, lineterminator="\n")
        writer.writerow(["case", "pair", "regime", "engine", "gogc", "cpu", "match",
                         "wall", "user", "sys", "rss", *EVENTS])
        for pair in range(1, args.pairs + 1):
            for regime in settings if pair % 2 else reversed(settings):
                for engine in binaries if pair % 2 else reversed(binaries):
                    directory = output / f"{pair}-{regime}-{engine}"
                    gc = settings[regime][engine]
                    status, timing, counts = measure(binaries[engine], args, directory, 0, gc)
                    if status or (directory / "stdout").read_bytes() != expected or (directory / "stderr").stat().st_size:
                        raise ValueError(f"verification changed in {directory}")
                    writer.writerow([args.isolate, pair, regime, engine, gc, args.cpu, 1, *timing, *counts])
                    file.flush()


if __name__ == "__main__":
    main()
