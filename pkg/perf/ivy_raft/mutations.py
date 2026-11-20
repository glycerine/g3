"""Generate independently checkable negative Raft controls in a new directory."""

import argparse
from pathlib import Path
import re


# Original text, replacement, expected number of replacements.
MUTATIONS = {
    "init-leader": (
        'leader(N,T) := false;',
        'leader(N,T) := true;', 1),
    "init-vote-message": (
        'vote_msg(C, T, V) := false;',
        'vote_msg(C, T, V) := true;', 1),
    "init-commit": (
        'committed(I, V) := false;',
        'committed(I, V) := true;', 1),
    "init-current-term": (
        'current_term(N,T) := T = 0;',
        'current_term(N,T) := true;', 1),
    "init-valid-log": (
        'valid(L) := L = log0;',
        'valid(L) := false;', 1),
    "election-start": (
        'started(t) := true;',
        'started(t) := false;', 1),
    "set-term-inclusive": (
        'term_bigger(n, T) := T < t;',
        'term_bigger(n, T) := T <= t;', 1),
    "vote-unrecorded": (
        'voted(n, t2) := true;',
        'voted(n, t2) := false;', 4),
    "double-vote": (
        't2 >= t & ~voted(n, t2) {',
        't2 >= t {', 1),
    "propose-exclude-last": (
        'index_used(l2, I) := I <= i;',
        'index_used(l2, I) := I < i;', 1),
    "propose-forget-valid": (
        'term_log(t) := l2;',
        'term_log(t) := log0;', 1),
    "commit-invalid-log": (
        'valid(commit_log) := true;',
        'valid(commit_log) := false;', 1),
    "commit-exclude-last": (
        'index_used(commit_log, I) := I <= i;',
        'index_used(commit_log, I) := I < i;', 1),
    "commit-wrong-value": (
        'commit_value := value_at(l, i);',
        'commit_value := value_at(log0, i);', 1),
}


def generate(wrapper, output):
    spec = wrapper.with_name("raft_no_assume.ivy")
    source = spec.read_text()
    original = wrapper.read_text()
    narrow, count = re.subn(r"( -> bv)\[\d+\]", r"\1[1]", original)
    if count != 8:
        raise ValueError(f"expected eight concrete domain widths, found {count}")
    for name, (before, after, expected) in MUTATIONS.items():
        if source.count(before) != expected:
            raise ValueError(f"{name} expected {expected} matching source fragments")
    output.mkdir(parents=True)
    for name, (before, after, _) in MUTATIONS.items():
        for widths, interpretations in [("original", original), ("one", narrow)]:
            folder = output / name / widths / "examples" / "raft"
            folder.mkdir(parents=True)
            (folder / spec.name).write_text(source.replace(before, after))
            (folder / wrapper.name).write_text(interpretations)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("wrapper", type=Path, help="original raft_no_assume_test.ivy")
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    generate(args.wrapper, args.output)
