"""Build one archived ground-proof experiment with a Go overlay in an empty directory."""
import argparse
from pathlib import Path
import sys
import tarfile

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "incremental_cost"))
from first_push_rewrite_rows import build_checker, stage_patch


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--archive", type=Path, default=HERE / "nov23-ground-proof-prototypes.tar.gz")
    p.add_argument("--variant", required=True, help="patch name in the archive, e.g. congruence/eu")
    p.add_argument("--goivy", required=True, type=Path)
    p.add_argument("--output", required=True, type=Path)
    p.add_argument("--abc", type=Path)
    p.add_argument("--native", action="store_true", help="build the same frontend against linked Z3")
    args = p.parse_args()
    output = args.output.resolve()
    goivy = args.goivy.resolve(strict=True)
    with tarfile.open(args.archive, "r:gz") as archive:
        patch = archive.extractfile(args.variant + ".patch").read().decode()
    output.mkdir(parents=True)
    print(build_checker(goivy, output, stage_patch(patch, goivy, output), args.abc, args.native))


if __name__ == "__main__":
    main()
