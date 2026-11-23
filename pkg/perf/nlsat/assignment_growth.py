"""Generate a satisfiable nonlinear arithmetic input for the g3 and Z3 shells."""

import argparse


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("variables", type=int)
    parser.add_argument("--shape", choices=("independent", "ordered", "coupled"),
                        default="independent")
    parser.add_argument("--pipeline", choices=("default", "cnf"), default="default")
    args = parser.parse_args()
    if args.variables < 1:
        parser.error("variables must be positive")

    print("(set-logic QF_NRA)")
    for i in range(args.variables):
        print(f"(declare-const x{i} Real)")
    for i in range(args.variables):
        print(f"(assert (> (* x{i} x{i}) 2))")
    for i in range(args.variables - 1):
        if args.shape == "ordered":
            print(f"(assert (< x{i} x{i+1}))")
        elif args.shape == "coupled":
            print(f"(assert (> (* x{i} x{i+1}) 2))")

    tactic = "qfnra-nlsat" if args.pipeline == "default" else "(then simplify purify-arith tseitin-cnf nlsat)"
    print(f"(check-sat-using {tactic})")


if __name__ == "__main__":
    main()
