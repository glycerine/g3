"""Count solver-thread instructions during wide scope pops at different Go GC settings."""

import argparse
import csv
import os
from pathlib import Path
from types import SimpleNamespace

from measure import ROOT, WARMUP, measure
from widths import build_width_fixtures


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True, help="new directory for binaries and measurements")
    parser.add_argument("--build", type=Path, default=ROOT / "build")
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--rounds", type=int, default=7)
    parser.add_argument("--widths", type=int, nargs="+", default=[32, 128, 512, 2048, 4096])
    args = parser.parse_args()
    if args.rounds < 1 or any(width < 4 or width % 2 for width in args.widths):
        parser.error("use positive rounds and even widths of at least four atoms")
    output = args.output.resolve()
    output.mkdir(parents=True)
    binaries = build_width_fixtures(output, args.build)
    modes = ["25", "100", "400", "off", "native"]
    with (output / "runs.csv").open("w") as dest:
        writer = csv.writer(dest)
        writer.writerow("round width mode phase intervals instructions enabled running".split())
        for rr in range(args.rounds):
            width_offset = rr % len(args.widths)
            mode_offset = rr % len(modes)
            for width in args.widths[width_offset:] + args.widths[:width_offset]:
                os.environ["SCOPE_ATOMS"] = str(width)
                iterations = WARMUP + max(24, min(600, 50000 // width))
                settings = SimpleNamespace(cpu=args.cpu, timeout=120, iterations=iterations, thread_event="instructions:u")
                for mode in modes[mode_offset:] + modes[:mode_offset]:
                    engine = "native" if mode == "native" else "g3"
                    os.environ["GOGC"] = "100" if engine == "native" else mode
                    for phase in ("empty", "pop", "restore", "relevance"):
                        run = output / f"{rr+1}-{width}-{mode}-{phase}"
                        counts = measure(run, binaries[engine], engine, "ScopedAtoms", phase, settings)
                        fields = (run / "out").read_text().split()
                        if float(fields[fields.index("atoms/op") - 1]) != width or counts[-1] != counts[-2]:
                            raise ValueError(f"wrong atom count or multiplexed counter in {run}")
                        writer.writerow([rr + 1, width, mode, phase, *counts])
                        dest.flush()


if __name__ == "__main__":
    main()
