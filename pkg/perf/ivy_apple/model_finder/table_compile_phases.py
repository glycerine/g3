"""Price function compilation within the whole finite-table compiler on hybrid."""
import argparse
from collections import Counter
import csv
import os
from pathlib import Path
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--perf", default="perf")
    parser.add_argument("profiles", nargs="+", type=Path)
    args = parser.parse_args()
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(("profile", "term_only", "inside_function", "within_fallback_compile"))
    for path in sorted(args.profiles):
        counts = Counter()
        stacks = subprocess.check_output([args.perf, "script", "--no-inline", "-i", path],
                                         env=dict(os.environ, DEBUGINFOD_URLS=""), text=True)
        for sample in stacks.split("\n\n"):
            if "(*SMTQuantifierManager).CheckModel" not in sample:
                continue
            term = sample.find("(*tableModel).compile")
            if term < 0:
                continue
            function = sample.find("(*tableModel).function")
            if function < 0:
                counts["term"] += 1
            elif term < function:
                counts["fallback"] += 1
            else:
                counts["function"] += 1
        writer.writerow((path.stem, counts["term"], counts["function"], counts["fallback"]))


if __name__ == "__main__":
    main()
