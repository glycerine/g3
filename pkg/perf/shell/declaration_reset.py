"""Generate a complete SMT-LIB declaration, scope and reset benchmark."""

import argparse


def constants(start, stop, sort):
    for i in range(start, stop):
        print(f"(declare-const x{i} {sort})")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("names", type=int)
    parser.add_argument("--shape", choices=["single", "overload", "scope", "sorts"],
                        default="single")
    args = parser.parse_args()
    if args.names < 1:
        parser.error("names must be positive")

    print("(set-option :produce-models true)")
    if args.shape == "overload":
        for i in range(args.names):
            print(f"(declare-fun f{i} (Int) Bool)")
            print(f"(declare-fun f{i} (Bool) Bool)")
    elif args.shape == "scope":
        split = args.names // 2
        constants(0, split, "Bool")
        print("(push 1)")
        constants(split, args.names, "Real")
        print("(pop 1)")
        constants(split, args.names, "Bool")
    elif args.shape == "sorts":
        for i in range(args.names):
            print(f"(declare-sort S{i} 0)")
            print(f"(declare-const x{i} S{i})")
    else:
        constants(0, args.names, "Bool")
    print("(check-sat)")
    print("(reset)")
    print("(check-sat)")


if __name__ == "__main__":
    main()
