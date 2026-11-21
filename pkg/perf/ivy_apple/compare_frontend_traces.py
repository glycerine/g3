"""Compare complete GoIvy XTrace streams without storing a copy of each trace."""
import argparse
from contextlib import ExitStack
import csv
import os
from pathlib import Path
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--checker", nargs=3, action="append", required=True,
                        metavar=("LABEL", "CPU", "BINARY"))
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--isolate", required=True)
    parser.add_argument("--output", type=Path, required=True, help="new directory for checker stderr")
    parser.add_argument("--passes", type=int, required=True)
    args = parser.parse_args()
    names = [name for name, _, _ in args.checker]
    if len(set(names)) != len(names) or len(names) < 2 or args.passes < 1:
        parser.error("use at least two distinct labels and a positive expected pass count")
    args.output.mkdir(parents=True)
    spec = args.spec.absolute()
    env = dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="0")
    lines = size = records = passes = 0
    last = b""
    with ExitStack() as stack:
        children = []
        try:
            for index, (_, cpu, binary) in enumerate(args.checker):
                directory = args.output / str(index + 1)
                directory.mkdir()
                err = stack.enter_context((directory / "stderr").open("wb"))
                command = ["taskset", "-c", str(int(cpu)), Path(binary).resolve(strict=True),
                           "isolate=" + args.isolate, spec]
                children.append(subprocess.Popen(command, stdout=subprocess.PIPE, stderr=err,
                                                 cwd=directory, env=env))
            while True:
                line = children[0].stdout.readline()
                for name, child in zip(names[1:], children[1:]):
                    other = child.stdout.readline()
                    if line != other:
                        raise ValueError(f"{names[0]} and {name} differ at line {lines + 1}: "
                                         f"{line[:180]!r}, {other[:180]!r}")
                if not line:
                    break
                lines += 1
                size += len(line)
                records += line.startswith(b"XTRACE:")
                passes += line == b"PASS\n"
                last = line
            statuses = [p.wait() for p in children]
            if any(statuses) or passes != args.passes or not records or last.strip() != b"OK":
                raise ValueError(f"incomplete traces: exits {statuses}, outcomes {passes}, events {records}")
        finally:
            for child in children:
                if child.poll() is None:
                    child.kill()
            for child in children:
                child.wait()
                child.stdout.close()
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "checkers", "lines", "bytes", "xtrace_records", "passes"])
    writer.writerow([args.isolate, "|".join(names), lines, size, records, passes])


if __name__ == "__main__":
    main()
