"""Replay the disequality between each original and rewritten Ivy or synthetic goal."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys
import tarfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--g3", required=True, type=Path)
    parser.add_argument("--z3", required=True, type=Path)
    parser.add_argument("--timeout", type=int, default=60)
    args = parser.parse_args()
    archive = Path(__file__).with_name("nov23-existential-definitions.tar.gz")
    env = dict(os.environ, GOMAXPROCS="1")
    rows = csv.writer(sys.stdout)
    rows.writerow(("formula", "engine", "verdict"))
    with tarfile.open(archive) as bundle:
        inputs = sorted(name for name in bundle.getnames()
                        if name.startswith("equivalence/") and name.endswith(".smt2"))
        if not inputs:
            raise ValueError("measurement archive contains no equivalence queries")
        for name in inputs:
            query = bundle.extractfile(name).read()
            for engine, binary in (("g3", args.g3), ("z3", args.z3)):
                result = subprocess.run([str(binary.resolve()), "-in", "-smt2"],
                                        input=query, capture_output=True, env=env,
                                        timeout=args.timeout, check=True)
                if result.stdout.strip() != b"unsat" or result.stderr:
                    raise ValueError(f"{engine} {name}: {result.stdout!r} {result.stderr!r}")
                rows.writerow((Path(name).stem, engine, "unsat"))


if __name__ == "__main__":
    main()
