"""Generate a complete SMT-LIB declaration, scope and reset benchmark."""

import argparse


def constants(start, stop, sort):
    for i in range(start, stop):
        print(f"(declare-const x{i} {sort})")


def sort_aliases(start, stop):
    for i in range(start, stop):
        print(f"(define-sort S{i} () Bool)")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("names", type=int)
    parser.add_argument("--shape", choices=["single", "overload", "scope", "sorts",
                                 "alias", "alias-scope", "datatype", "mixed-sorts", "parametric"],
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
    elif args.shape == "alias":
        sort_aliases(0, args.names)
    elif args.shape == "alias-scope":
        split = args.names // 2
        sort_aliases(0, split)
        print("(push 1)")
        sort_aliases(split, args.names)
        print("(pop 1)")
        sort_aliases(split, args.names)
    elif args.shape == "datatype":
        for i in range(args.names):
            print(f"(declare-datatypes () ((D{i} (K{i}))))")
    elif args.shape == "mixed-sorts":
        for i in range(args.names):
            print(f"(declare-sort U{i} 0)")
            print(f"(define-sort S{i} () U{i})")
            print(f"(declare-const a{i} S{i})")
    elif args.shape == "parametric":
        for i in range(args.names):
            print(f"(define-sort S{i} (T) (Array T T))")
    else:
        constants(0, args.names, "Bool")
    print("(check-sat)")
    print("(reset)")
    print("(check-sat)")


if __name__ == "__main__":
    main()
