"""Build GoIvy against a specified g3 checkout, optionally instrumenting the Raft run."""

import argparse
import json
from pathlib import Path
import shutil
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("goivy", type=Path, help="GoIvy module directory")
    parser.add_argument("g3", type=Path, help="g3/pkg module directory")
    parser.add_argument("output", type=Path)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--classic", action="store_true", help="GoIvy's vendored C++ Z3")
    mode.add_argument("--profile", choices=["cpu", "heap"])
    parser.add_argument("--pgo", type=Path, help="Go CPU profile for profile-guided compilation")
    args = parser.parse_args()
    goivy = args.goivy.resolve()
    tags = "xtrace_off classic_z3" if args.classic else "xtrace_off"
    with tempfile.TemporaryDirectory() as tmp:
        tmp = Path(tmp)
        mod = tmp / "goivy.mod"
        shutil.copyfile(goivy / "go.mod", mod)
        shutil.copyfile(goivy / "go.sum", tmp / "goivy.sum")
        subprocess.run(["go", "mod", "edit", "-replace=github.com/glycerine/g3/pkg=" +
                        str(args.g3.resolve()), mod], cwd=goivy, check=True)
        flags = ["-buildvcs=false", "-tags", tags, "-modfile=" + str(mod)]
        if args.pgo:
            flags.append("-pgo=" + str(args.pgo.resolve()))
        pkg = ["./cmd/goivy_check"]
        if args.profile:
            patches = Path(__file__).resolve().parent
            if args.profile == "cpu":
                main_src = goivy / "cmd/goivy_check"
                src = main_src / "main.go"
                for f in main_src.glob("*.go"):
                    if f != src and not f.name.endswith("_test.go"):
                        shutil.copyfile(f, tmp / f.name)
                pkg = [str(tmp / "main.go"), *(str(f) for f in sorted(tmp.glob("*.go")))]
            else:
                src = goivy / "smt/g3_solver.go"
                overlay = tmp / "overlay.json"
                overlay.write_text(json.dumps({"Replace": {str(src): str(tmp / src.name)}}))
                flags.append("-overlay=" + str(overlay))
            subprocess.run(["patch", "--silent", "--output", tmp / src.name, src,
                            patches / ("goivy-" + args.profile + ".patch")], check=True)
        subprocess.run(["go", "build", *flags, "-o", args.output.resolve(), *pkg],
                       cwd=goivy, check=True)


if __name__ == "__main__":
    main()
