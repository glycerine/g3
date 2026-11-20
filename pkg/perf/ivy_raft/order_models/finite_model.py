"""Generate a quantified bit-vector problem with an explicit satisfying witness."""

import itertools
import random
import re


def smt(term):
    if isinstance(term, str):
        return term
    return "(" + " ".join(map(smt, term)) + ")"


def parse(text):
    stack = [[]]
    for token in re.findall(r"\(|\)|[^()\s]+", text):
        if token == "(":
            stack.append([])
        elif token == ")":
            term = stack.pop()
            stack[-1].append(term)
        else:
            stack[-1].append(token)
    if len(stack) != 1 or len(stack[0]) != 1:
        raise ValueError("expected one complete expression")
    return stack[0][0]


def values(sort, widths):
    if sort == "Bool":
        return (False, True)
    return range(1 << widths[sort])


def evaluate(term, environment, widths, tables):
    """Interpret a source or target term. Enumerate each quantifier's finite domain."""
    if isinstance(term, str):
        if term.startswith("#b"):
            return int(term[2:], 2)
        if term in ("true", "false"):
            return term == "true"
        if term in environment:
            return environment[term]
        if term in tables:
            return tables[term][()]
        numeral = re.fullmatch(r"n[0-9]+_([0-9]+)", term)
        if numeral:
            return int(numeral[1])
        raise ValueError(f"uninterpreted symbol {term}")

    op, *args = term
    if op in ("forall", "exists"):
        declarations, body = args
        names = [name for name, _ in declarations]
        domains = [values(sort, widths) for _, sort in declarations]
        outcomes = (
            evaluate(body, {**environment, **dict(zip(names, row))}, widths, tables)
            for row in itertools.product(*domains)
        )
        return all(outcomes) if op == "forall" else any(outcomes)

    def visit(arg):
        return evaluate(arg, environment, widths, tables)

    if op == "and":
        return all(map(visit, args))
    if op == "or":
        return any(map(visit, args))
    if op == "not":
        return not visit(args[0])
    if op == "=>":
        return not visit(args[0]) or visit(args[1])
    if op == "ite":
        return visit(args[1] if visit(args[0]) else args[2])

    row = tuple(map(visit, args))
    if op == "=":
        return all(value == row[0] for value in row[1:])
    if op == "distinct":
        return len(set(row)) == len(row)
    if op == "xor":
        return row[0] != row[1]
    if op == "bvult" or re.fullmatch("lt[0-9]+", op):
        return row[0] < row[1]
    if op == "bvule":
        return row[0] <= row[1]
    if op == "bvugt":
        return row[0] > row[1]
    if op == "bvuge":
        return row[0] >= row[1]
    return tables[op][row]


class Problem:
    """A seed fixes the interpretation first, then the terms and their polarity."""

    signatures = {
        "a": ([], "A"), "b": ([], "B"), "g": (["A"], "B"), "h": (["B"], "A"),
        "u": (["A", "A"], "A"), "p": (["A"], "Bool"), "q": (["B", "A"], "Bool"),
        "z": ([], "Bool"), "pick": (["Bool", "B"], "A"), "truth": (["Bool"], "Bool"),
    }

    def __init__(self, seed):
        self.random = rng = random.Random(seed)
        self.widths = {"A": rng.randint(1, 3), "B": rng.randint(1, 3)}
        if seed % 4 == 0:
            self.widths["B"] = self.widths["A"]
        if seed % 31 == 0:
            self.widths["A"] = 4
        self.tables = {}
        for name, (inputs, result) in self.signatures.items():
            domain = itertools.product(*(values(sort, self.widths) for sort in inputs))
            self.tables[name] = {row: rng.choice(values(result, self.widths)) for row in domain}

        # This tautology makes every query eligible for the transformation.
        self.terms = [["forall", [["anchor", "A"]], ["or", ["p", "anchor"], ["not", ["p", "anchor"]]]]]
        for _ in range(3):
            body = self.boolean(3, [], 0)
            self.terms.append(body if evaluate(body, {}, self.widths, self.tables) else ["not", body])
        self.terms.append(["forall", [["zero_bound", "B"]], ["bvule", self.numeral(0, "B"), "zero_bound"]])

    def numeral(self, value, sort):
        return "#b" + format(value, f"0{self.widths[sort]}b")

    def value(self, sort, depth, scope):
        rng = self.random
        bound = [name for name, typ in scope if sort == typ]
        if depth == 0 or rng.random() < 0.37:
            if bound and rng.random() < 0.55:
                return rng.choice(bound)
            constant = {"A": "a", "B": "b"}[sort]
            numeral = self.numeral(rng.randrange(1 << self.widths[sort]), sort)
            return rng.choice([constant, numeral])
        if rng.random() < 0.28:
            return ["ite", self.boolean(depth - 1, scope, 2),
                    self.value(sort, depth - 1, scope), self.value(sort, depth - 1, scope)]
        op = rng.choice(["h", "u", "pick"]) if sort == "A" else "g"
        return self.application(op, depth - 1, scope, 2)

    def application(self, op, depth, scope, quantifier_depth):
        inputs, _ = self.signatures[op]
        return [op, *(self.boolean(depth, scope, quantifier_depth) if sort == "Bool"
                      else self.value(sort, depth, scope) for sort in inputs)]

    def boolean(self, depth, scope, quantifier_depth):
        rng = self.random
        sort = rng.choice(["A", "B"])
        if depth == 0:
            leaves = ["true", "false", "z", *(n for n, s in scope if s == "Bool"),
                      ["p", self.value("A", 0, scope)]]
            return rng.choice(leaves)
        kind = rng.randrange(11)
        if kind < 3 and quantifier_depth < 2:
            declarations = []
            for i in range(rng.choice([1, 1, 2])):
                name = rng.choice(["x", "y", "zv", "t", "v"])
                if any(name == old for old, _ in declarations):
                    name += "_" + str(i)
                declarations.append([name, rng.choice(["A", "A", "B", "Bool"])])
            inner = [(n, s) for n, s in scope if all(n != v for v, _ in declarations)] + declarations
            return [rng.choice(["exists", "forall"]), declarations,
                    self.boolean(depth - 1, inner, quantifier_depth + 1)]
        if kind in (3, 4):
            op = rng.choice(["and", "or", "xor", "=>", "="])
            return [op, self.boolean(depth - 1, scope, quantifier_depth),
                    self.boolean(depth - 1, scope, quantifier_depth)]
        if kind == 5:
            return ["ite", *(self.boolean(depth - 1, scope, quantifier_depth) for _ in range(3))]
        if kind == 6:
            return self.application(rng.choice(["p", "q", "truth"]), depth - 1, scope, quantifier_depth)
        if kind == 7:
            return ["not", self.boolean(depth - 1, scope, quantifier_depth)]
        op = rng.choice(["=", "distinct", "bvule", "bvuge", "bvult", "bvugt"])
        arity = 3 if op == "distinct" and rng.randrange(2) == 0 else 2
        return [op, *(self.value(sort, depth - 1, scope) for _ in range(arity))]

    def script(self):
        lines = [f"(define-sort {sort} () (_ BitVec {width}))" for sort, width in self.widths.items()]
        for name, (inputs, result) in self.signatures.items():
            lines.append(f"(declare-fun {name} ({' '.join(inputs)}) {result})")
        return "\n".join(lines + [smt(["assert", term]) for term in self.terms])

    def validate(self, script, axiom_cache):
        """Transport the known witness across inferred domains and evaluate each assertion."""
        widths, tables, assertions = {}, {}, []
        for line in script.splitlines():
            if line.startswith(";domain "):
                _, sort, width = line.split()
                widths[sort] = int(width)
            elif line.startswith(";function "):
                _, opaque, name = line.split()
                tables[opaque] = self.tables[name]
            elif line.startswith("(assert "):
                assertions.append(line)
        if not assertions or not widths or not tables:
            raise ValueError("missing model map or assertions")
        domain_sizes = tuple(sorted(widths.items()))
        for line in assertions:
            # Axioms without original functions depend only on domains and
            # numeral values. The same quantified order axioms recur often.
            key = None if re.search(r"\bf\d+\b", line) else (line, domain_sizes)
            if key is not None and key in axiom_cache:
                continue
            _, formula = parse(line)
            if not evaluate(formula, {}, widths, tables):
                raise ValueError(f"assertion false in lifted model: {line}")
            if key is not None:
                axiom_cache.add(key)
        return len(assertions), len(widths)
