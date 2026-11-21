"""Time whole-spec builds in alternating pairs and record competing CPU activity."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys
import time

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "ivy_raft"))
from measure import transcript


def cpu_ticks():
    result = {}
    for line in Path("/proc/stat").read_text().splitlines():
        name, *fields = line.split()
        if name.startswith("cpu") and name[3:].isdigit():
            # Linux includes guest time in user and nice already.
            user, nice, system, idle, wait, irq, softirq, steal = map(int, fields[:8])
            active = user + nice + system + irq + softirq + steal
            result[int(name[3:])] = active, active + idle + wait
    return result


def siblings(cpu):
    text = Path(f"/sys/devices/system/cpu/cpu{cpu}/topology/thread_siblings_list").read_text()
    result = set()
    for bounds in text.strip().split(","):
        first, _, last = bounds.partition("-")
        result.update(range(int(first), int(last or first)+1))
    result.discard(cpu)
    return result


def busy_fraction(start, end, cpu):
    active = end[cpu][0] - start[cpu][0]
    ticks = end[cpu][1] - start[cpu][1]
    return active / max(1, ticks)


def choose_cpu(allowed):
    start = cpu_ticks()
    time.sleep(1)
    end = cpu_ticks()
    load = {cpu: busy_fraction(start, end, cpu) for cpu in start.keys() & end.keys()}
    return min(allowed, key=lambda cpu: (load[cpu] + sum(load[c] for c in siblings(cpu)), load[cpu], cpu))


def measure(output, label, binary, workload, spec, reference, cpu):
    output.mkdir()
    args = [binary, spec] if workload == "raft" else [binary, "isolate=" + workload, spec]
    command = ["taskset", "-c", str(cpu), "/usr/bin/time", "-f", "%e %U %S %M",
               "-o", output / "time", *args]
    env = dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1")
    start = cpu_ticks()
    with (output / "out").open("wb") as out, (output / "err").open("wb") as err:
        subprocess.run(command, cwd=output, env=env, stdout=out, stderr=err, check=True)
    end = cpu_ticks()
    observed = (output / "out").read_bytes()
    if workload == "raft":
        matches = transcript(observed.decode().splitlines()) == transcript(reference.decode().splitlines())
    else:
        matches = observed == reference
    if not matches:
        raise ValueError(f"{label} transcript differs on {workload}")
    wall, user, system, rss = (output / "time").read_text().split()
    sibling_load = max((busy_fraction(start, end, n) for n in siblings(cpu)), default=0)
    total_busy = (end[cpu][0]-start[cpu][0]) / os.sysconf("SC_CLK_TCK")
    other = max(0, total_busy - float(user) - float(system)) / max(float(wall), 0.01)
    return [wall, user, system, rss, round(other, 4), round(sibling_load, 4)]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--binary", nargs=2, action="append", required=True, metavar=("LABEL", "GOIVY"))
    p.add_argument("--workload", nargs=3, action="append", required=True, metavar=("ISOLATE_OR_raft", "IVY", "REFERENCE"))
    p.add_argument("--pairs", type=int, default=5)
    p.add_argument("--cpu", type=int, help="pin all pairs here. By default choose the least busy core before each pair")
    p.add_argument("--dir", type=Path, required=True, help="new output directory")
    a = p.parse_args()
    binary = {name: Path(path).resolve(strict=True) for name, path in a.binary}
    if len(binary) != 2 or len(a.binary) != 2 or a.pairs < 1:
        p.error("supply two differently named builds and a positive number of pairs")
    workloads = [(name, Path(spec).absolute(), Path(raw).read_bytes()) for name, spec, raw in a.workload]
    if len({name for name, _, _ in workloads}) != len(workloads):
        p.error("workload names must be unique")
    for name in [*binary, *(name for name, _, _ in workloads)]:
        if not name or not all(c.isalnum() or c in "-_" for c in name):
            p.error("use names containing only letters, digits, underscores and hyphens")
    allowed = os.sched_getaffinity(0)
    if a.cpu is not None and a.cpu not in allowed:
        p.error("selected CPU is outside process affinity")
    dest = a.dir.resolve()
    dest.mkdir(parents=True)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["workload", "pair", "cpu", "build", "wall_s", "user_s", "system_s", "rss_kib",
                     "other_active_fraction", "busiest_sibling_active_fraction"])
    for i in range(a.pairs):
        order = list(binary) if i % 2 == 0 else list(reversed(binary))
        for name, spec, reference in workloads:
            cpu = a.cpu if a.cpu is not None else choose_cpu(allowed)
            for label in order:
                path = dest / f"{name}-{i+1}-{label}"
                values = measure(path, label, binary[label], name, spec, reference, cpu)
                writer.writerow([name, i+1, cpu, label, *values])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
