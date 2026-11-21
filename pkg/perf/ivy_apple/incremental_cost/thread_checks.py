"""Count instructions or cycles on the solver thread for every incremental Apple proof."""

import argparse
import csv
import os
from pathlib import Path
import re
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from phase_counters import build

EVENTS = {"instructions": 1, "cycles": 0, "branches": 4, "branch-misses": 5}
PACKAGES = ("fmt", "os", "runtime", "strconv", "syscall", "time", "unsafe")
PREFIX = "IVY-THREAD-CHECK "


def compile_builds(args, output):
    observer = (HERE / "thread_counter.go.txt").read_text()
    configs = [("g3", "g3", None), ("native", "native", None)]
    configs.extend((name, "g3", Path(overlay).resolve(strict=True)) for name, overlay in args.go_overlay)
    names = [name for name, _, _ in configs]
    if len(set(names)) != len(names) or any(not re.fullmatch(r"[A-Za-z0-9_-]+", n) for n in names):
        raise ValueError("choose unique build names using letters, digits, underscores or hyphens")
    binaries = {}
    for name, engine, overlay in configs:
        directory = output / name
        directory.mkdir()
        binaries[name] = build(engine, args, directory, observer, PACKAGES, go_overlay=overlay)
    return binaries


def measure(path, binary, isolate, spec, cpu, event, reference):
    path.mkdir()
    command = ["taskset", "-c", str(cpu), binary, "isolate=" + isolate, spec]
    env = dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_THREAD_EVENT=str(EVENTS[event]))
    with (path / "out").open("wb") as out, (path / "err").open("wb") as err:
        subprocess.run(command, cwd=path, env=env, stdout=out, stderr=err, check=True)
    if (path / "out").read_bytes() != reference:
        raise ValueError(f"verification transcript differs for {path}")
    rows = []
    for line in (path / "err").read_text().splitlines():
        if line.startswith(PREFIX):
            check, count, empty, enabled, running, wall = map(int, line.removeprefix(PREFIX).split())
            if check != len(rows) + 1 or not 0 < running <= enabled or count <= 0:
                raise ValueError(f"unexpected counter or check sequence in {path}")
            rows.append((check, count, empty, enabled, running, wall))
    expected = sum(line.strip() == b"PASS" for line in reference.splitlines())
    if not rows or len(rows) != expected:
        raise ValueError(f"{path} recorded {len(rows)} of {expected} verification conditions")
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", type=Path, required=True)
    parser.add_argument("--modfile", type=Path, help="optional alternate GoIvy module file")
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--isolate", nargs=2, action="append", required=True, metavar=("NAME", "REFERENCE_STDOUT"))
    parser.add_argument("--go-overlay", action="append", nargs=2, default=[], metavar=("NAME", "OVERLAY_JSON"))
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--pairs", type=int, default=3)
    parser.add_argument("--event", choices=EVENTS, default="instructions")
    parser.add_argument("--output", type=Path, required=True, help="new directory")
    args = parser.parse_args()
    args.goivy = args.goivy.resolve(strict=True)
    spec = args.spec.absolute()  # The input path appears verbatim in GoIvy output.
    if args.pairs < 1 or not spec.is_file():
        parser.error("supply an existing specification and a positive pair count")
    references = {name: Path(path).read_bytes() for name, path in args.isolate}
    if len(references) != len(args.isolate) or any(not re.fullmatch(r"[A-Za-z0-9_-]+", n) for n in references):
        parser.error("choose distinct alphanumeric isolate names")
    if any(not ref.rstrip().endswith(b"\nOK") for ref in references.values()):
        parser.error("all reference transcripts must end in OK")
    output = args.output.resolve()
    output.mkdir(parents=True)
    binaries = compile_builds(args, output)
    names = list(binaries)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "cpu", "round", "engine", "event", "check", "count", "empty",
                     "enabled_ns", "running_ns", "wall_ns"])
    for i in range(args.pairs):
        offset = i % len(names)
        order = names[offset:] + names[:offset]
        if i % 2:
            order.reverse()
        for isolate, reference in references.items():
            for name in order:
                directory = output / f"{isolate}-{i+1}-{name}"
                rows = measure(directory, binaries[name], isolate, spec, args.cpu, args.event, reference)
                writer.writerows((isolate, args.cpu, i+1, name, args.event, *row) for row in rows)
                sys.stdout.flush()


if __name__ == "__main__":
    main()
