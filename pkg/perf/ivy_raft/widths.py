"""Generate the Raft width controls in a separate output directory."""

import argparse
from pathlib import Path
import re
from shutil import copyfile


MIXED = dict(node=3, quorum=2, term=4, index=2, log=3, value=4,
             option_index=2, option_term=3)
UNIFORM = {"narrow1": 1, "narrow2": 2, "uniform3": 3, "uniform4": 4,
           "wide8": 8, "wide16": 16}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("wrapper", type=Path, help="original raft_no_assume_test.ivy")
    parser.add_argument("output", type=Path, help="new directory for the seven controls")
    args = parser.parse_args()
    source = args.wrapper.read_text()
    spec = args.wrapper.with_name("raft_no_assume.ivy")
    pattern = re.compile(r"^(interpret (\w+) -> bv)\[\d+\]$", re.MULTILINE)
    names = [m[2] for m in pattern.finditer(source)]
    if len(names) != len(MIXED) or set(names) != MIXED.keys():
        parser.error("expected the eight interpreted sorts in the original Raft wrapper")
    args.output.mkdir(parents=True)
    for label in (*UNIFORM, "mixed"):
        widths = MIXED if label == "mixed" else dict.fromkeys(names, UNIFORM[label])
        text = pattern.sub(lambda m: f"{m[1]}[{widths[m[2]]}]", source)
        folder = args.output / label / "examples/raft"
        folder.mkdir(parents=True)
        copyfile(spec, folder / spec.name)
        (folder / args.wrapper.name).write_text(text)


if __name__ == "__main__":
    main()
