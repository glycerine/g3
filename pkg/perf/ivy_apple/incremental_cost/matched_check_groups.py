"""Compare matching verification conditions and the work of the two general solvers."""
import argparse
from collections import Counter, defaultdict
import csv
import gzip
from pathlib import Path
import sys


SEARCH = ("quant-instantiations", "decisions", "conflicts", "mk-bool-var", "propagations")


def rows(path):
    opener = gzip.open if path.suffix == ".gz" else open
    with opener(path, "rt", newline="") as stream:
        yield from csv.DictReader(stream)


def indexed(records, ordinal):
    result = {}
    for row in records:
        key = int(row["batch"]), int(row[ordinal])
        if key in result:
            raise ValueError(f"duplicate condition {key}")
        result[key] = row
    return result


def groups(go_phases, native_phases, go_inner, go_stats, native_stats):
    phases = list(rows(go_phases))
    go = indexed((r for r in phases if r["stage"] == "check"), "condition")
    early = indexed((r for r in phases if r["stage"] == "early"), "condition")
    native = indexed((r for r in rows(native_phases) if r["stage"] == "check"), "condition")
    inner, gs, ns = (indexed(rows(path), "goal") for path in (go_inner, go_stats, native_stats))
    completed_early = {key for key, r in early.items() if r["result"] == "-1"}
    if (go.keys() != inner.keys() or go.keys() != gs.keys() or native.keys() != ns.keys()
            or go.keys() & completed_early or go.keys() | completed_early != native.keys()):
        raise ValueError("condition sets from the independent runs disagree")
    totals = defaultdict(Counter)
    for key, nr in native.items():
        if nr["result"] != ns[key]["result"]:
            raise ValueError(f"native verdict differs between observations at {key}")
        if key in completed_early:
            kind, gr = "before_first_push", early[key]
        else:
            gr = go[key]
            observed = inner[key]
            if gr["result"] != observed["result"] or gr["result"] != gs[key]["result"]:
                raise ValueError(f"g3 verdict differs between observations at {key}")
            if int(observed["solver"]):
                kind = "general_search"
            elif int(observed["proof"]):
                kind = "after_push_proof"
            else:
                kind = "direct_check"
        if gr["result"] != nr["result"]:
            raise ValueError(f"solvers disagree at {key}")
        total = totals[kind]
        total["conditions"] += 1
        total["g3_instructions"] += int(gr["instructions"])
        total["z3_instructions"] += int(nr["instructions"])
        if kind == "general_search":
            if int(gs[key]["observed_checks"]) != 1 or int(ns[key]["observed_checks"]) != 1:
                raise ValueError(f"missing solver search counters at {key}")
            for field in SEARCH:
                total["g3_" + field] += int(gs[key][field])
                total["z3_" + field] += int(ns[key][field])
    return totals


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("go-phases", "native-phases", "go-inner", "go-stats", "native-stats"):
        parser.add_argument("--" + name, type=Path, required=True)
    totals = groups(**vars(parser.parse_args()))
    fields = ["group", "conditions", "g3_instructions", "z3_instructions"]
    fields += [engine + "_" + field for field in SEARCH for engine in ("g3", "z3")]
    out = csv.DictWriter(sys.stdout, fieldnames=fields, lineterminator="\n")
    out.writeheader()
    for group, values in sorted(totals.items()):
        out.writerow(dict(values, group=group))


if __name__ == "__main__":
    main()
