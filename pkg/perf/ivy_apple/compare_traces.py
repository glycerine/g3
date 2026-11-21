#!/usr/bin/env python3
"""Compare Z3 proof trace events and the E-matches referenced by each instance."""

import argparse
from pathlib import Path


def compare(go, native, limits):
    offsets = [0, 0]
    matches = instances = events = 0
    go_to_native, native_to_go = {}, {}
    while offsets[0] < limits[0] and offsets[1] < limits[1]:
        x, y = go.readline(), native.readline()
        events += 1
        if not x.endswith(b"\n") or not y.endswith(b"\n"):
            raise ValueError(f"incomplete event {events}")
        offsets[0] += len(x)
        offsets[1] += len(y)
        if x == y and not x.startswith((b"[instance] ", b"[new-match] ")):
            continue
        a, b = x.split(), y.split()
        if (len(a) > 1 and len(b) > 1 and a[0] == b[0] and a[0] in (b"[new-match]", b"[instance]")
                and a[1].startswith(b"0x") and b[1].startswith(b"0x")):
            g, z = a[1], b[1]
            if a[0] == b"[new-match]":
                # Either allocator can reuse addresses once a scope is popped.
                go_to_native[g], native_to_go[z] = z, g
                matches += 1
            else:
                if go_to_native.get(g) != z or native_to_go.get(z) != g:
                    raise ValueError(f"event {events} references different E-matches")
                instances += 1
            a[1] = b[1] = b"fingerprint"
        if a != b:
            raise ValueError(f"event {events} differs\ng3 {x[:250]!r}\nZ3 {y[:250]!r}")
    if offsets != limits:
        raise ValueError(f"trace lengths differ after {events} events, offsets {offsets}, limits {limits}")
    return events, matches, instances


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("go", type=Path)
    p.add_argument("native", type=Path)
    p.add_argument("--through", type=int, nargs=2, metavar=("G3_BYTES", "Z3_BYTES"),
                   help="byte offsets flushed through the last check in each file")
    args = p.parse_args()
    sizes = [f.stat().st_size for f in (args.go, args.native)]
    limits = args.through or sizes
    if any(n <= 0 or n > size for n, size in zip(limits, sizes)):
        p.error("each byte limit must be positive and within its file")
    with args.go.open("rb") as go, args.native.open("rb") as native:
        events, matches, instances = compare(go, native, limits)
    print(f"Matched {events:,} events, including {matches:,} E-matches and {instances:,} instance references.")


if __name__ == "__main__":
    main()
