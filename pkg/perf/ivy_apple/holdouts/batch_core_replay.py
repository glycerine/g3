"""Measure online reuse of an UNSAT core when its exact goal and all premises recur."""
import argparse
from collections import Counter, defaultdict
import csv
import gzip
import os
from pathlib import Path
import subprocess

from batch_prefix_replay import prepare


POLICIES = ("ordinary", "always", "repeat", "atmost32", "first", "two", "repeat-first", "shared1", "shared2")


class Shell:
    def __init__(self, args, directory, prelude, *, mine):
        label = "mine" if mine else "ordinary"
        self.error = (directory / (label + ".stderr")).open("w")
        command = ["taskset", "-c", str(args.cpu), args.perf, "stat", "-x", ",", "-e", "instructions:u",
                   "-o", directory / (label + ".perf"), "--", "/usr/bin/time", "-f", "%e %U %S %M",
                   "-o", directory / (label + ".time"), args.solver, "-in", "-smt2"]
        self.proc = subprocess.Popen(command, text=True, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                     stderr=self.error, env=dict(os.environ, GOMAXPROCS="1", GOGC="100"))
        if mine:
            self.write("(set-option :produce-unsat-cores true)")
            if args.timeout_ms:
                self.write(f"(set-option :timeout {args.timeout_ms})")
        self.write("\n".join(prelude))

    def write(self, text):
        self.proc.stdin.write(text + "\n")

    def ask(self, text):
        self.write(text + '\n(echo "ready")')
        self.proc.stdin.flush()
        lines = []
        for raw in self.proc.stdout:
            line = raw.strip()
            if line == "ready":
                return " ".join(lines)
            if line.startswith("(error"):
                raise RuntimeError(line)
            if line:
                lines.append(line)
        raise RuntimeError("solver exited before its reply")

    def close(self):
        try:
            self.proc.stdin.close()
            if self.proc.wait(timeout=30):
                raise RuntimeError("solver exited with an error")
        finally:
            self.error.close()


def should_mine(policy, seen, tried, attempts, earlier):
    if policy == "ordinary":
        return False
    if policy == "always":
        return True
    if policy == "repeat":
        return seen
    if policy == "atmost32":
        return attempts < 32
    if policy in ("first", "two"):
        return tried < (1 if policy == "first" else 2)
    if policy == "repeat-first":
        return seen and tried == 0
    return earlier and tried < 4


def audit(args, prelude, names, learned):
    code = list(prelude)
    for goal, facts in learned:
        code.append("(push 1)")
        code.extend("(assert " + names[fact] + ")" for fact in facts)
        code.extend(["(assert " + names[goal] + ")", "(check-sat)", "(pop 1)"])
    if learned:
        run = subprocess.run(["taskset", "-c", str(args.cpu), args.native, "-smt2", "-in"],
                             input="\n".join(code) + "\n", text=True, capture_output=True, timeout=300)
        if run.returncode or run.stderr or run.stdout.splitlines() != ["unsat"] * len(learned):
            raise ValueError("independent native verification of the learned cores failed")
    (args.output / "verified").write_text(str(len(learned)) + "\n")


def replay(args):
    prelude, names, batches = prepare(args.events)
    prefix = "g3Track!"
    while any(prefix in stmt for stmt in prelude):
        prefix += "x"
    args.output.mkdir(parents=True)
    shells = {}
    previous = []
    seen = set()
    tried = Counter()
    cores = defaultdict(list)
    learned = {}
    with gzip.open(args.output / "checks.csv.gz", "wt") as checks:
        writer = csv.writer(checks, lineterminator="\n")
        writer.writerow(["batch", "goal", "expected", "observed", "path", "core_size"])
        try:
            for group, base, goals in batches:
                current = frozenset(base)
                started = set()
                attempts = 0
                shared = args.policy.startswith("shared")
                candidate = current
                if shared:
                    candidate = current.intersection(*previous[-int(args.policy[-1]):])

                def query(goal, mine):
                    if mine not in shells:
                        shells[mine] = Shell(args, args.output, prelude, mine=mine)
                    shell = shells[mine]
                    if mine not in started:
                        started.add(mine)
                        shell.write("(push 1)")
                        if mine:
                            for fact in dict.fromkeys(base):
                                if fact in candidate:
                                    shell.write(f"(assert (! {names[fact]} :named {prefix}{fact}))")
                        elif base:
                            shell.write("(assert (and " + " ".join(names[f] for f in base) + "))")
                    result = shell.ask("(push 1)\n(assert " + names[goal] + ")\n(check-sat)")
                    core = None
                    if result not in ("sat", "unsat", "unknown"):
                        raise ValueError(f"unexpected check result {result}")
                    if mine and result == "unsat":
                        reply = shell.ask("(get-unsat-core)")
                        if not (reply.startswith("(") and reply.endswith(")")):
                            raise ValueError("invalid UNSAT core")
                        tags = reply[1:-1].split()
                        if any(not tag.startswith(prefix) for tag in tags):
                            raise ValueError("unrecognized core label")
                        core = frozenset(int(tag[len(prefix):]) for tag in tags)
                        if not core <= candidate:
                            raise ValueError("core refers to a premise outside this batch")
                    shell.write("(pop 1)")
                    return result, core

                for goal, expected in goals:
                    core = next((facts for facts in cores[goal] if facts <= current), None)
                    result = None
                    path = "ordinary"
                    if core is not None:
                        result, path = "unsat", "reuse"
                    elif should_mine(args.policy, goal in seen, tried[goal], attempts, bool(previous)):
                        attempts += 1
                        tried[goal] += 1
                        result, core = query(goal, True)
                        path = "mine"
                        if core is not None:
                            learned[goal, tuple(sorted(core))] = None
                            cores[goal] = [facts for facts in cores[goal] if not core <= facts] + [core]
                        elif shared or result == "unknown":
                            result, path = None, "mine-ordinary"
                    if result is None:
                        result, _ = query(goal, False)
                    if result != expected:
                        raise ValueError(f"batch {group} goal {goal} returned {result}, expected {expected}")
                    writer.writerow([group, goal, expected, result, path, len(core) if core is not None else ""])
                    seen.add(goal)
                for mine in started:
                    shells[mine].write("(pop 1)")
                previous.append(current)
                previous = previous[-2:]
        finally:
            for shell in shells.values():
                shell.close()
    with gzip.open(args.output / "cores.csv.gz", "wt") as file:
        writer = csv.writer(file, lineterminator="\n")
        writer.writerow(["index", "goal", "premises"])
        for index, (goal, facts) in enumerate(learned):
            writer.writerow([index, goal, " ".join(map(str, facts))])
    audit(args, prelude, names, learned)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("events", "solver", "native", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--policy", choices=POLICIES, required=True)
    parser.add_argument("--timeout-ms", type=int, default=180)
    parser.add_argument("--perf", default="perf")
    args = parser.parse_args()
    if args.timeout_ms < 0:
        parser.error("timeout must be nonnegative")
    args.solver = args.solver.resolve(strict=True)
    args.native = args.native.resolve(strict=True)
    replay(args)


if __name__ == "__main__":
    main()
