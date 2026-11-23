"""Count shell instructions between loading assertions and checking one SMT query on Linux."""

import argparse
import csv
import gzip
import os
from pathlib import Path
import select
import signal
import subprocess
import sys
import tempfile


def measure(binary, source, cpu, perf, phase):
    with tempfile.TemporaryDirectory() as temp:
        directory = Path(temp)
        control, ack = directory / "control", directory / "ack"
        os.mkfifo(control)
        os.mkfifo(ack)
        # Open both ends so perf can open its peers in either order.
        with control.open("r+b", buffering=0) as commands, ack.open("r+b", buffering=0) as replies:
            def send_control(command):
                commands.write(command + b"\n")
                if not select.select([replies], [], [], 30)[0] or not replies.read(64).startswith(b"ack"):
                    raise RuntimeError("perf did not acknowledge its counter command")

            counts = directory / "instructions.csv"
            command = ["taskset", "-c", str(cpu), perf, "stat", "-D", "-1", "--control",
                       f"fifo:{control},{ack}", "-x", ",", "-e", "instructions:u", "-o", counts,
                       "--", binary, "-in", "-smt2"]
            with subprocess.Popen(command, cwd=directory, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  stderr=subprocess.PIPE, start_new_session=True,
                                  env=dict(os.environ, GOMAXPROCS="1", GOGC="100")) as proc:
                def send(data):
                    proc.stdin.write(data)
                    proc.stdin.flush()

                try:
                    send(b'(echo "ready")\n')
                    if proc.stdout.readline().strip() != b"ready":
                        raise RuntimeError("solver startup failed")
                    if phase == "load":
                        send_control(b"enable")
                    send(source + b'\n(echo "loaded")\n')
                    if proc.stdout.readline().strip() != b"loaded":
                        raise RuntimeError("expected declarations and assertions without output")
                    send_control(b"disable" if phase == "load" else b"enable")
                    send(b"(check-sat)\n")
                    result = proc.stdout.readline().strip().decode()
                    if phase == "check":
                        send_control(b"disable")
                    remaining, error = proc.communicate(b"(exit)\n")
                    if proc.returncode or remaining or result not in ("sat", "unsat", "unknown"):
                        raise RuntimeError(f"{binary} exited {proc.returncode}: {result} {remaining!r} {error!r}")
                except BaseException:
                    if proc.poll() is None:
                        os.killpg(proc.pid, signal.SIGKILL)
                    raise
            with counts.open() as file:
                instructions, = [int(row[0]) for row in csv.reader(file)
                                 if len(row) > 2 and row[2] == "instructions:u"]
            return result, instructions


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--g3", type=Path, required=True)
    parser.add_argument("--native", type=Path, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--rounds", type=int, default=1)
    parser.add_argument("inputs", nargs="+", type=Path,
                        help="SMT2 or gzip input with declarations, assertions, and optional final (check-sat)")
    args = parser.parse_args()
    if args.rounds < 1:
        parser.error("rounds must be positive")
    engines = [("g3", args.g3.resolve(strict=True)), ("native", args.native.resolve(strict=True))]
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["file", "round", "engine", "phase", "result", "instructions"])
    for path in args.inputs:
        source = gzip.decompress(path.read_bytes()) if path.suffix == ".gz" else path.read_bytes()
        source = source.rstrip().removesuffix(b"(check-sat)")
        expected = None
        for index in range(args.rounds):
            for engine, binary in engines[::1 if index % 2 == 0 else -1]:
                for phase in ("load", "check"):
                    result, instructions = measure(binary, source, args.cpu, args.perf, phase)
                    if expected is not None and result != expected:
                        raise RuntimeError(f"{path} returned {result} following {expected}")
                    expected = result
                    writer.writerow([path, index + 1, engine, phase, result, instructions])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
