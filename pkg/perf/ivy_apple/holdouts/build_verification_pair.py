"""Build both GoIvy verification backends from one checkout and record their provenance."""

import argparse
import json
from pathlib import Path
import subprocess


def output(command, directory):
    return subprocess.check_output(command, cwd=directory, text=True).strip()


def checkout(directory):
    return {
        "directory": str(directory),
        "revision": output(["git", "rev-parse", "HEAD"], directory),
        "tracked_changes": output(["git", "diff", "HEAD", "--", "."], directory),
        "untracked_go_files": output(["git", "ls-files", "--others", "--exclude-standard",
                                      "--", "*.go"], directory).splitlines(),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="new build directory")
    args = parser.parse_args()
    goivy = args.goivy.resolve(strict=True)
    dest = args.output.resolve()
    module = json.loads(output(["go", "list", "-mod=readonly", "-m", "-json",
                                "github.com/glycerine/g3/pkg"], goivy))
    frontend = checkout(goivy)
    metadata = {"goivy": frontend, "go": output(["go", "version"], goivy), "g3_module": module}
    local = module.get("Replace", {})
    if "Dir" in local and not local.get("Version"):
        metadata["g3_checkout"] = checkout(Path(local["Dir"]).resolve(strict=True))
    dest.mkdir(parents=True, exist_ok=False)
    for engine, tags in (("g3", "xtrace_off"), ("native", "xtrace_off,classic_z3")):
        binary = dest / engine / "goivy_check"
        binary.parent.mkdir()
        command = ["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=true",
                   "-tags=" + tags, "-o", str(binary), "./cmd/goivy_check"]
        subprocess.run(command, cwd=goivy, check=True)
        metadata[engine] = {
            "command": command,
            "build_id": output(["go", "tool", "buildid", str(binary)], goivy),
            "build_info": output(["go", "version", "-m", str(binary)], goivy),
        }
        settings = [line.split() for line in metadata[engine]["build_info"].splitlines()]
        if ["build", "vcs.revision=" + frontend["revision"]] not in settings:
            raise RuntimeError(f"{engine} did not record the expected GoIvy revision")
    if checkout(goivy) != frontend:
        raise RuntimeError("GoIvy changed during the build")
    if "g3_checkout" in metadata and checkout(Path(local["Dir"]).resolve()) != metadata["g3_checkout"]:
        raise RuntimeError("g3 changed during the build")
    (dest / "build.json").write_text(json.dumps(metadata, indent=2) + "\n")


if __name__ == "__main__":
    main()
