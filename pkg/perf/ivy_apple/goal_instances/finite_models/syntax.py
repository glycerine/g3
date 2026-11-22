"""Read the Boolean, enumerated and uninterpreted terms in the Apple captures."""

from collections import defaultdict
from itertools import count
import re


def parse(text):
    tokens = re.compile(r';[^\n]*|\(|\)|\|(?:\\.|[^|\\])*\||"(?:[^"]|"")*"|[^\s();]+')
    stack = [[]]
    for token in tokens.findall(text):
        if token.startswith(";"):
            continue
        if token == "(":
            stack.append([])
        elif token == ")":
            if len(stack) == 1:
                raise ValueError("unexpected closing parenthesis")
            term = tuple(stack.pop())
            stack[-1].append(term)
        else:
            stack[-1].append(token)
    if len(stack) != 1:
        raise ValueError("incomplete input")
    return stack[0]


def symbols(term):
    if isinstance(term, str):
        yield term
    else:
        for child in term:
            yield from symbols(child)


def smt(term):
    if isinstance(term, str):
        return term
    return "(" + " ".join(map(smt, term)) + ")"


def fresh_names(forms, prefix):
    reserved = {name for form in forms for name in symbols(form)}
    for number in count():
        candidate = prefix + str(number)
        if candidate not in reserved:
            yield candidate


def separate(forms):
    """Give overloaded functions and bound variables unambiguous names."""
    signatures = defaultdict(list)
    for op, *args in forms:
        if op == "declare-fun":
            name, inputs, result = args
            signatures[name].append((inputs, result))
        elif op == "declare-const":
            name, result = args
            signatures[name].append(((), result))
        elif op == "declare-datatypes":
            for (sort, arity), constructors in zip(*args, strict=True):
                if arity != "0" or any(len(c) != 1 for c in constructors):
                    raise ValueError(f"expected a finite enumeration for {sort}")
                for (name,) in constructors:
                    signatures[name].append(((), sort))

    names = fresh_names(forms, "g3!vc!")
    renamed = {}
    for name, choices in signatures.items():
        if len(choices) > 1:
            if len({inputs for inputs, _ in choices}) != len(choices):
                raise ValueError(f"result-only overloading in {name}")
            for inputs, _ in choices:
                renamed[name, inputs] = next(names)

    def application(name, children):
        inputs = tuple(sort for _, sort in children)
        result, = (sort for signature, sort in signatures[name] if signature == inputs)
        name = renamed.get((name, inputs), name)
        return ((name, *(value for value, _ in children)) if children else name), result

    def expr(t, env):
        if isinstance(t, str):
            if t in ("true", "false"):
                return t, "Bool"
            return env[t] if t in env else application(t, ())
        op, *args = t
        if op in ("forall", "exists", "let"):
            bindings, body = args
            local = env.copy()
            result = []
            for name, rhs in bindings:
                new = next(names)
                if op == "let":
                    value, sort = expr(rhs, env)
                else:
                    value = sort = rhs
                local[name] = (new, sort)
                result.append((new, value))
            body, sort = expr(body, local)
            if op != "let" and sort != "Bool":
                raise ValueError(f"ill-sorted quantifier {op}")
            return (op, tuple(result), body), sort
        if op == "!":
            return expr(args[0], env)
        children = [expr(arg, env) for arg in args]
        sorts = [sort for _, sort in children]
        if op in ("or", "and", "=>", "not", "xor"):
            if any(sort != "Bool" for sort in sorts):
                raise ValueError(f"ill-sorted connective {op}")
            sort = "Bool"
        elif op in ("=", "distinct"):
            if len(set(sorts)) != 1:
                raise ValueError(f"ill-sorted relation {op}")
            sort = "Bool"
        elif op == "ite":
            if len(sorts) != 3 or sorts[0] != "Bool" or sorts[1] != sorts[2]:
                raise ValueError("ill-sorted conditional")
            sort = sorts[1]
        else:
            return application(op, children)
        return (op, *(value for value, _ in children)), sort

    out = []
    for form in forms:
        op, *args = form
        if op in ("declare-fun", "declare-const"):
            signature = args[1] if op == "declare-fun" else ()
            form = (op, renamed.get((args[0], signature), args[0]), *args[1:])
        elif op == "declare-datatypes":
            sorts, constructors = args
            form = (op, sorts, tuple(tuple((renamed.get((c, ()), c),) for (c,) in group)
                                     for group in constructors))
        elif op == "assert":
            body, sort = expr(args[0], {})
            if sort != "Bool":
                raise ValueError("expected a Boolean assertion")
            form = (op, body)
        out.append(form)
    return out


def unlet(term, env):
    if isinstance(term, str):
        return env.get(term, term)
    op, *args = term
    if op == "let":
        bindings, body = args
        local = {name: unlet(value, env) for name, value in bindings}
        return unlet(body, {**env, **local})
    if op in ("forall", "exists"):
        # separate() assigned every binder a unique name.
        return op, args[0], unlet(args[1], env)
    return (op, *(unlet(arg, env) for arg in args))


def conjuncts(term, env):
    if isinstance(term, tuple) and term[0] == "let":
        _, definitions, body = term
        local = {name: unlet(value, env) for name, value in definitions}
        yield from conjuncts(body, {**env, **local})
    elif isinstance(term, tuple) and term[0] == "and":
        for child in term[1:]:
            yield from conjuncts(child, env)
    else:
        yield unlet(term, env)
