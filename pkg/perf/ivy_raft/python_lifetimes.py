"""Run Python Ivy with selected native references held until process exit."""

import argparse
import csv
import sys
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--hold", choices=["neither", "asts", "solvers", "both"],
                        default="neither")
    parser.add_argument("--local-macro-finder", action="store_true")
    parser.add_argument("--stats", required=True)
    parser.add_argument("ivy_args", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    ivy_args = args.ivy_args
    if ivy_args[:1] == ["--"]:
        ivy_args = ivy_args[1:]
    if not ivy_args:
        parser.error("supply the arguments for ivy_check after --")
    sys.argv = ["ivy_check", *ivy_args]

    import ivy.z3 as z3
    from ivy.ivy_check import main as ivy_main

    # Omit only the matching native dec_ref. Python may still reclaim the
    # wrapper. These changes apply to this measurement process alone.
    if args.hold in ("asts", "both"):
        z3.AstRef.__del__ = lambda self: None
    if args.hold in ("solvers", "both"):
        z3.Solver.__del__ = lambda self: None
    if args.local_macro_finder:
        original_init = z3.Solver.__init__

        def initialize(solver, *args, **kwargs):
            original_init(solver, *args, **kwargs)
            solver.set("smt.macro_finder", True)

        z3.Solver.__init__ = initialize

    original_check = z3.Solver.check
    fields = ("num checks", "quant instantiations", "decisions", "conflicts",
              "mk bool var", "rlimit count")
    with open(args.stats, "w", newline="") as output:
        writer = csv.writer(output, lineterminator="\n")
        writer.writerow(("check", "seconds", "result", *fields))
        number = 0

        def check(solver, *assumptions):
            nonlocal number
            number += 1
            start = time.monotonic()
            result = original_check(solver, *assumptions)
            elapsed = time.monotonic() - start
            stats = solver.statistics()
            counters = dict(stats[i] for i in range(len(stats)))
            writer.writerow((number, f"{elapsed:.6f}", str(result),
                             *(counters.get(k, 0) for k in fields)))
            return result

        z3.Solver.check = check
        return ivy_main()


if __name__ == "__main__":
    sys.exit(main())
