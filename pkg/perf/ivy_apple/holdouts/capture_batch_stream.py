"""Collect typed SMT premises and ordered solver answers from the instrumented GoIvy checker."""
import argparse
import base64
import csv
import gzip
import os
from pathlib import Path
import subprocess


def capture(args):
    root = args.output.resolve()
    root.mkdir(parents=True)
    cmd = ["taskset", "-c", str(args.cpu), args.checker.resolve(strict=True)]
    if args.isolate:
        cmd.append("isolate=" + args.isolate)
    cmd.append(args.spec.absolute())
    known = set()
    current = None
    count = 0
    with gzip.open(root / "events.csv.gz", "wt") as events, (root / "stdout").open("wb") as out, (root / "stderr").open("wb") as err:
        writer = csv.writer(events, lineterminator="\n")
        writer.writerow(["event", "batch_or_term", "id", "answer", "smt2"])
        with subprocess.Popen(cmd, cwd=root, stdout=out, stderr=subprocess.PIPE,
                              env=dict(os.environ, GOMAXPROCS="1", GOGC="100", XTRACE_OFF="1")) as checker:
            for line in checker.stderr:
                tag = line.split(maxsplit=1)[:1]
                if tag in ([b"W-BASE"], [b"W-TERM"]):
                    kind, number, encoded = line.split()
                    index = int(number)
                    if kind == b"W-BASE":
                        current = index
                    elif index in known:
                        raise ValueError(f"duplicate term id {index}")
                    else:
                        known.add(index)
                    writer.writerow([kind.decode()[2:].lower(), index, "", "",
                                     base64.b64decode(encoded, validate=True).decode()])
                elif tag in ([b"W-ASSUME"], [b"W-GOAL"]):
                    kind, batch, term, *answer = line.split()
                    if int(batch) != current or int(term) not in known or len(answer) != int(kind == b"W-GOAL"):
                        raise ValueError("misordered solver event")
                    if answer and int(answer[0]) not in (-1, 0, 1):
                        raise ValueError("invalid solver answer")
                    writer.writerow([kind.decode()[2:].lower(), int(batch), int(term), int(answer[0]) if answer else "", ""])
                    count += len(answer)
                else:
                    err.write(line)
            status = checker.wait()
    if status != args.expected_exit:
        raise RuntimeError(f"checker exited {status}, expected {args.expected_exit}")
    if args.reference:
        for name in ("stdout", "stderr"):
            if (root / name).read_bytes() != (args.reference / name).read_bytes():
                raise ValueError(f"reference {name} differs")
    print(f"{count} physical checks, {len(known)} distinct goal or assumption terms")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("checker", "output", "spec"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--reference", type=Path, help="directory containing expected stdout and stderr")
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--expected-exit", type=int, default=0)
    parser.add_argument("--isolate")
    capture(parser.parse_args())


if __name__ == "__main__":
    main()
