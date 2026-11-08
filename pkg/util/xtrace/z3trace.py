#!/usr/bin/env python3
"""Convert a z3 TRACE build's .z3-trace file to Go xtrace lines.

The Go port (built with -tags xtrace) prints "XTRACE: [tag] text" lines to
stdout at the points that mirror z3 TRACE(tag, ...) calls, with the trace text
collapsed to one line. This script prints the z3 trace blocks in the same
form, so the first divergence can be found with diff:

    cd /tmp/z && z3-trace-build -tr:assign_core -tr:decide -tr:seq f.smt2
    python3 z3trace.py /tmp/z/.z3-trace > z.txt
    g3-xtrace-build f.smt2 | grep '^XTRACE: ' > g.txt
    diff z.txt g.txt | head

Only the blocks the Go port traces are kept; use --tags to restrict further
(for example --tags assign_core,decide). Go traces z3's "seq" tag only for
add_axiom, final_check_eh (the level line) and get_automaton.
"""

import argparse
import re
import sys

HEADER = re.compile(r'^-------- \[([^\]]+)\] (\S+) \S+ ---------$')
FOOTER = '------------------------------------------------'

SEQ_FUNCTIONS = {'add_axiom', 'final_check_eh', 'get_automaton'}


def blocks(lines):
    tag = func = None
    body = []
    for line in lines:
        line = line.rstrip('\n')
        m = HEADER.match(line)
        if m:
            tag, func, body = m.group(1), m.group(2), []
        elif line == FOOTER and tag is not None:
            yield tag, func, ' '.join(' '.join(body).split())
            tag = None
        elif tag is not None:
            body.append(line)


def convert(tag, func, text):
    if tag == 'assign_core':
        # Drop the justification display that follows the level.
        m = re.match(r'(.* level: \d+)', text)
        return m.group(1) if m else text
    if tag == 'seq':
        if func not in SEQ_FUNCTIONS:
            return None
        if func == 'final_check_eh':
            m = re.match(r'(level: \d+)', text)
            if not m:
                return None
            text = m.group(1)
        return func + ' ' + text
    if tag in ('decide', 'add_eq_detail', 'mk_bool_var'):
        return text
    return None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('trace', nargs='?', default='.z3-trace')
    ap.add_argument('--tags', default='')
    args = ap.parse_args()
    keep = set(t for t in args.tags.split(',') if t)
    with open(args.trace) as f:
        for tag, func, text in blocks(f):
            if keep and tag not in keep:
                continue
            out = convert(tag, func, text)
            if out is not None:
                sys.stdout.write('XTRACE: [%s] %s\n' % (tag, out))


if __name__ == '__main__':
    main()
