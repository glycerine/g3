"""Join perf instruction samples to solver-thread check intervals on Linux."""
import bisect
from collections import Counter
import csv
from decimal import Decimal
import re
import subprocess

from phase_counters import OBSERVER, replace_once
from stack_costs import categories


PACKAGES = ("fmt", "io", "os", "runtime", "strconv", "syscall", "time", "unsafe")
HEADER = re.compile(r"^\s*(\d+)\s+(\d+\.\d+):\s+instructions:u:\s*$")
FRAME = re.compile(r"^\s*[0-9a-fA-F]+\s+(.+)\s+\(.+\)\s*$")


def observer():
    begin = """runtime.LockOSThread()
    thread := syscall.Gettid()
    begin := checkMonotonicTime()
    start := time.Now()"""
    end = """finish := checkMonotonicTime()
        runtime.UnlockOSThread()
        elapsed := time.Since(start)
        fmt.Fprintln(os.Stderr, "IVY-THREAD-CHECK", n, thread, begin, finish)"""
    source = replace_once(OBSERVER, "start := time.Now()", begin)
    return replace_once(source, "elapsed := time.Since(start)", end) + """
func checkMonotonicTime() int64 {
    var ts syscall.Timespec
    _, _, err := syscall.RawSyscall(syscall.SYS_CLOCK_GETTIME, 1, uintptr(unsafe.Pointer(&ts)), 0)
    if err != 0 { panic(err) }
    return ts.Nano()
}
"""


def write_counts(path, header, rows):
    with path.open("w") as output:
        writer = csv.writer(output, lineterminator="\n")
        writer.writerow(header)
        writer.writerows(rows)


def summarize(perf, profile, directory, engine, expected, period):
    windows = [tuple(map(int, line.split()[1:])) for line in (directory / "stderr").read_text().splitlines()
               if line.startswith("IVY-THREAD-CHECK ")]
    if ([n for n, _, _, _ in windows] != list(range(1, expected + 1))
            or any(start > end for _, _, start, end in windows)
            or any(a[3] > b[2] for a, b in zip(windows, windows[1:]))):
        raise ValueError("incomplete or overlapping solver-thread windows")
    ends = [w[3] for w in windows]
    nearest, leaves = Counter(), Counter()
    total = matched = other_threads = 0
    command = [perf, "script", "--ns", "--no-inline", "-i", str(profile), "-F", "tid,time,event,ip,sym,dso"]
    selected, stack = None, []

    def count():
        if selected is not None:
            op = next((op for frame in stack for op in categories(engine, frame)), "other")
            nearest[selected, op] += 1
            leaves[selected, stack[0] if stack else "[no stack]"] += 1

    with subprocess.Popen(command, stdout=subprocess.PIPE, text=True) as proc:
        for line in proc.stdout:
            header = HEADER.match(line)
            if header:
                count()
                selected, stack = None, []
                total += 1
                tid, ns = int(header[1]), int(Decimal(header[2]) * 1_000_000_000)
                i = bisect.bisect_left(ends, ns)
                if i < expected and ns >= windows[i][2]:
                    check, owner = windows[i][:2]
                    if tid == owner:
                        selected = check
                        matched += 1
                    else:
                        other_threads += 1
            elif selected is not None and line.strip():
                frame = FRAME.match(line)
                if frame:
                    stack.append(frame[1])
        count()
        if proc.wait() != 0 or not matched or sum(nearest.values()) != matched:
            raise ValueError("perf script failed or no solver-thread samples matched")
    write_counts(directory / "coverage.csv", ["checks", "samples", "matched", "other_threads", "period"],
                 [(expected, total, matched, other_threads, period)])
    for name, rows in (("nearest", nearest), ("leaf", leaves)):
        write_counts(directory / (name + ".csv"), ["check", "category", "samples"],
                     ((check, symbol, n) for (check, symbol), n in sorted(rows.items())))
