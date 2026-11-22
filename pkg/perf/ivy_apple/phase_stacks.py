"""Sample user-mode instructions during an incremental solver phase with Linux perf."""

import argparse
import gzip
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

from phase_counters import build


def record(args, binary, selection, directory, reference):
    directory.mkdir()
    ctl, ack = directory / "control", directory / "ack"
    os.mkfifo(ctl, 0o600)
    os.mkfifo(ack, 0o600)
    env = dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_PERF_SELECTION=selection,
               IVY_PERF_CONTROL=str(ctl), IVY_PERF_ACK=str(ack))
    profile = directory / "perf.data"
    command = ["taskset", "-c", str(args.cpu), args.perf, "record", "--no-buildid",
               "--call-graph", args.call_graph, "-e", "instructions:u", "-c", str(args.period),
               "-m", "512", "--control", f"fifo:{ctl},{ack}", "-o", str(profile)]
    if selection != "outside":
        command.extend(["-D", "-1"])
    command.extend(["--", str(binary), f"isolate={args.isolate}", str(args.spec)])
    try:
        with (directory / "stderr").open("wb") as err:
            result = subprocess.run(command, cwd=directory, env=env, stdout=subprocess.PIPE,
                                    stderr=err, check=True)
    finally:
        ctl.unlink()
        ack.unlink()
    if result.stdout != reference:
        raise ValueError(f"{binary.name}: complete stdout differs from reference")
    checks = [int(s.split()[1]) for s in (directory / "stderr").read_text().splitlines()
              if s.startswith("IVY-PERF-CHECK ")]
    if checks != list(range(1, 1 + reference.splitlines().count(b"PASS"))):
        raise ValueError("incomplete solver check sequence")
    if selection.isdecimal() and int(selection) not in checks:
        raise ValueError("the selected check was never reached")
    inline = "--inline" if args.inline else "--no-inline"
    with gzip.open(directory / "stacks.gz", "wb", compresslevel=1) as dest:
        with subprocess.Popen([args.perf, "script", inline, "-i", str(profile),
                               "-F", "event,ip,sym,dso"], stdout=subprocess.PIPE) as proc:
            shutil.copyfileobj(proc.stdout, dest, 1 << 20)
            if proc.wait():
                raise RuntimeError("perf script failed")
    if not args.keep_raw:
        profile.unlink()


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", type=Path, required=True)
    p.add_argument("--spec", type=Path, required=True)
    p.add_argument("--reference", type=Path, required=True)
    p.add_argument("--output", type=Path, required=True)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--isolate", default="rfn2")
    p.add_argument("--select", nargs="+", default=["inside"], help="inside, outside or a one-based check number")
    p.add_argument("--engines", nargs="+", choices=["g3", "native"], default=["g3", "native"])
    p.add_argument("--keep-binaries", action="store_true",
                   help="save the exact executables in OUTPUT/bin for later source-line attribution")
    p.add_argument("--period", type=int, default=4_800_019, help="user instructions per sample")
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--perf", default="perf")
    p.add_argument("--modfile", type=Path)
    p.add_argument("--keep-raw", action="store_true", help="keep perf.data after symbolizing stacks")
    p.add_argument("--inline", action="store_true", help="include inline frames when symbolizing")
    p.add_argument("--call-graph", choices=["dwarf,8192", "fp"], default="dwarf,8192",
                   help="use frame pointers for binaries built with them, or DWARF stacks")
    p.add_argument("--go-overlay", type=Path, help="optional Go source overlay for the g3 build")
    args = p.parse_args()
    if len(set(args.engines)) != len(args.engines) or len(set(args.select)) != len(args.select):
        p.error("each engine and check selection must appear only once")
    if args.period < 1 or args.pairs < 1 or any(
            s not in ("inside", "outside") and (not s.isdecimal() or int(s) < 1 or str(int(s)) != s)
            for s in args.select):
        p.error("supply positive counts and valid check selections")
    args.goivy = args.goivy.resolve(strict=True)
    args.spec = args.spec.absolute()
    if args.modfile:
        args.modfile = args.modfile.resolve(strict=True)
    if not args.spec.is_file():
        p.error("the specification must be a file")
    reference = args.reference.read_bytes()
    if reference.splitlines()[-1:] != [b"OK"] or not reference.splitlines().count(b"PASS"):
        p.error("the reference must contain successful obligations and finish with OK")
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    with tempfile.TemporaryDirectory() as scratch:
        binaries = [build(engine, args, Path(scratch), go_overlay=args.go_overlay if engine == "g3" else None)
                    for engine in args.engines]
        if args.keep_binaries:
            bindir = output / "bin"
            bindir.mkdir()
            for binary in binaries:
                shutil.copy2(binary, bindir / binary.name)
        for i in range(args.pairs):
            for select in args.select:
                for binary in binaries[::1 if i % 2 == 0 else -1]:
                    record(args, binary, select, output / f"{select}-{i+1}-{binary.name}", reference)


if __name__ == "__main__":
    main()
