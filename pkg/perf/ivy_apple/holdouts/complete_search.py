"""Compare per-check search counters on complete GoIvy inputs under ordinary solver scopes."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from search_counters import build, COUNTERS, measurements

sys.path.insert(0, str(HERE.parent / "goal_instances/finite_models"))
from syntax import parse

BOUND = object()


def expand_lets(term, env, depth):
    if isinstance(term, str):
        return env.get(term, term)
    if term and term[0] == "let":
        _, bindings, body = term
        # SMT-LIB defines all right-hand sides in the enclosing environment.
        assigned = {name: expand_lets(value, env, depth) for name, value in bindings}
        return expand_lets(body, {**env, **assigned}, depth)
    if term and term[0] in ("forall", "exists", "lambda"):
        op, bindings, body = term
        assigned = {name: (BOUND, depth + i) for i, (name, _) in enumerate(bindings)}
        return op, bindings, expand_lets(body, {**env, **assigned}, depth + len(bindings))
    return tuple(expand_lets(child, env, depth) for child in term)


def same_output(first, second, unknown):
    if first == second:
        return True
    if not unknown:
        return False
    marker = b"; benchmark generated from python API\n"
    a, mark, left = first.partition(marker)
    b, other, right = second.partition(marker)
    if not mark or other != mark or a != b:
        return False
    # Go and C++ can choose different let sharing when displaying the same
    # inconclusive query. Check the complete ordered command sequence.
    forms = [parse(data.decode()) for data in (left, right)]
    return [expand_lets(t, {}, 0) for t in forms[0]] == [expand_lets(t, {}, 0) for t in forms[1]]


def observe(binary, spec, cpu, directory):
    with (directory / "out").open("wb") as out, (directory / "err").open("wb") as err:
        run = subprocess.run(["taskset", "-c", str(cpu), binary, spec], cwd=directory,
                             env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"), stdout=out, stderr=err)
    stdout = (directory / "out").read_bytes()
    stderr = (directory / "err").read_text().splitlines()
    rows = measurements(stderr)
    messages = [s for s in stderr if not s.startswith(("NATIVE-STATS ", "IVY-STATS "))]
    unknown = messages.count("Solver produced inconclusive result")
    expected = sum(line.strip() in (b"PASS", b"FAIL") for line in stdout.splitlines()) + unknown
    if run.returncode not in (0, 1) or not rows or len(rows) != expected:
        raise ValueError(f"incomplete check sequence in {directory} ({len(rows)} of {expected})")
    return run.returncode, messages, stdout, rows, unknown


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", type=Path, required=True)
    parser.add_argument("--go-overlay", type=Path, help="g3 source overlay for this comparison")
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("spec", type=Path, nargs="+")
    args = parser.parse_args()
    ivy = args.goivy.resolve(strict=True)
    overlay = args.go_overlay.resolve(strict=True) if args.go_overlay else None
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["case", "engine", "check", *COUNTERS])
    with tempfile.TemporaryDirectory() as scratch:
        root = Path(scratch)
        engines = {name: build(ivy, root, name, name == "native", go_overlay=overlay, concrete=name == "concrete")
                   for name in ("native", "concrete")}
        for i, spec in enumerate(args.spec):
            spec = spec.resolve(strict=True)
            previous = None
            for name, binary in engines.items():
                work = root / f"{i}-{name}"
                work.mkdir()
                status, diagnostic, stdout, counters, unknown = observe(binary, spec, args.cpu, work)
                if previous is not None:
                    old_status, old_diagnostic, old_stdout, old_counters = previous
                    if (status != old_status or diagnostic != old_diagnostic or len(old_counters) != len(counters)
                            or not same_output(old_stdout, stdout, unknown)):
                        raise ValueError(f"checker outcomes differ on {spec}")
                    for check, (old, new) in enumerate(zip(old_counters, counters), 1):
                        if old != new:
                            changed = [COUNTERS[i] for i in range(len(old)) if old[i] != new[i]]
                            raise ValueError(f"{spec}, check {check} differs in {changed}")
                previous = status, diagnostic, stdout, counters
                writer.writerows([spec.stem, name, n, *row] for n, row in enumerate(counters, 1))
                sys.stdout.flush()


if __name__ == "__main__":
    main()
