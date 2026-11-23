"""Measure complete incremental refutations on weaker subsets of the original conjuncts."""
import argparse
import csv
import gzip
import os
from pathlib import Path
import re
import subprocess
import sys

from query_replay import HERE
sys.path.insert(0, str(HERE.parent / "goal_instances"))
from capture import standard_declarations
sys.path.insert(0, str(HERE.parent / "goal_instances/finite_models"))
from syntax import fresh_names, parse, smt, symbols


def conjuncts(term, bindings=()):
    # Keep nested let expressions in lexical order. Copy only definitions used
    # by this conjunct, respecting simultaneous binding in each definition list.
    if isinstance(term, tuple) and term[0] == "let":
        bindings = list(bindings)
        while isinstance(term, tuple) and term[0] == "let":
            bindings.append(term[1])
            term = term[2]
    if isinstance(term, tuple) and term[0] == "and":
        for child in term[1:]:
            yield from conjuncts(child, bindings)
        return
    needed = set(symbols(term))
    for frame in reversed(bindings):
        used = tuple((name, rhs) for name, rhs in frame if name in needed)
        if used:
            needed.difference_update(name for name, _ in used)
            needed.update(sym for _, rhs in used for sym in symbols(rhs))
            term = ("let", used, term)
    yield term


def binder_arity(term):
    if not isinstance(term, tuple):
        return 0
    local = len(term[1]) if term and term[0] in ("forall", "exists") else 0
    return max((local, *(binder_arity(x) for x in term)))


class Problem:
    def __init__(self, vector):
        with gzip.open(vector, "rt") as source:
            original = standard_declarations(source.read())
        points = list(re.finditer(r"(?m)^[ \t]*\(assert(?=[\s(])",original))
        if len(points)<2 or original.count("(check-sat)")!=1:
            raise ValueError(f"expected at least two assertions and one check in {vector}")
        at = points[-1].start()
        self.original = "(set-option :smt.macro_finder true)\n"+original[:at]+"(push 1)\n"+original[at:]
        forms = parse(self.original)
        self.names = fresh_names(forms, "g3!premise!")
        self.pieces = []
        self.commands = []
        for form in forms:
            if form[0] != "assert":
                self.commands.append((None, smt(form)))
                continue
            ids = []
            for term in conjuncts(form[1]):
                idx = len(self.pieces)
                ids.append(idx)
                self.pieces.append((next(self.names), term, binder_arity(term)))
            self.commands.append((ids, None))
        self.all = list(range(len(self.pieces)))

    def query(self, subset, name=False):
        if subset is None:
            return self.original
        keep = set(subset)
        ans = ["(set-option :produce-unsat-cores true)"] if name else []
        for ids, command in self.commands:
            if ids is None:
                ans.append(command)
                continue
            for i in ids:
                if i in keep:
                    label, term, _ = self.pieces[i]
                    source = smt(term)
                    if name:
                        source = f"(! {source} :named {label})"
                    ans.append("(assert "+source+")")
        if name:
            ans.append("(get-unsat-core)")
        return "\n".join(ans)+"\n"


def run(args, directory, tag, binary, query, timeout):
    stem = directory / tag
    result = subprocess.run(["taskset","-c",str(args.cpu),args.perf,"stat","-x",",","-e","instructions:u",
                             "-o",stem.with_suffix(".perf"),"--","timeout","-k","1",str(timeout),binary,"-in","-smt2"],
                            input=query+"\n(get-info :all-statistics)\n",text=True,capture_output=True,
                            env=dict(os.environ,GOMAXPROCS="1",XTRACE_OFF="1"))
    stem.with_suffix(".out").write_text(result.stdout+result.stderr)
    errors = [ln for ln in result.stdout.splitlines() if ln.startswith("(error")]
    if result.returncode in (124,137):
        verdict = "timeout"
    elif query.startswith("(set-option :rlimit ") and errors and all("resource limit exceeded" in e for e in errors):
        # The shell may exhaust its diagnostic budget during Push and then print
        # a result for a different scope. Discard every verdict from that run.
        verdict = "budget"
    elif result.returncode or errors:
        raise ValueError(f"solver rejected {tag}: {errors[:1]} {result.stderr[:200]}")
    else:
        verdict = result.stdout.splitlines()[0]
        if verdict not in ("sat","unsat","unknown"):
            raise ValueError(f"unrecognized verdict {verdict}")
    rows = [row for row in csv.reader(stem.with_suffix(".perf").read_text().splitlines())
            if len(row)>4 and row[2]=="instructions:u" and float(row[4])==100.]
    if len(rows)!=1:
        raise ValueError(f"incomplete hardware count {stem}")
    return verdict, int(rows[0][0]), result.stdout


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for arg in ("g3","native","output"):
        p.add_argument("--"+arg,type=Path,required=True)
    p.add_argument("--cases",nargs="+",required=True)
    p.add_argument("--timeout",type=float,default=6)
    p.add_argument("--rounds",type=int,default=3)
    p.add_argument("--cpu",type=int,required=True)
    p.add_argument("--perf",default="perf")
    args=p.parse_args()
    args.output.mkdir(parents=True)
    bins={"g3":args.g3.resolve(strict=True),"native":args.native.resolve(strict=True)}
    wr=csv.writer(sys.stdout,lineterminator="\n")
    wr.writerow(["case","round","engine","route","cpu","total","retained","answer","instructions","instances","conflicts"])
    for case in args.cases:
        directory=args.output/case
        directory.mkdir()
        problem=Problem(HERE.parent/"test_vectors"/(case+".smt2.gz"))
        # The named core is an offline reference. Its discovery
        # cost is measured separately and this route cannot select a proof in production.
        verdict, cost, output=run(args,directory,"discover",bins["native"],problem.query(problem.all,True),args.timeout)
        wr.writerow([case,0,"native","discover",args.cpu,len(problem.all),len(problem.all),verdict,cost,"",""])
        routes={"ordinary":None,"split":problem.all}
        for limit in (0,1,2):
            routes["arity-"+str(limit)]=[i for i,(_,_,arity) in enumerate(problem.pieces) if arity<=limit]
        if verdict=="unsat":
            core=parse(output)[1]
            if not isinstance(core,tuple):
                raise ValueError("missing core")
            ix={n:i for i,(n,_,_) in enumerate(problem.pieces)}
            routes["known-core"]=[ix[n] for n in core]
        for i in range(args.rounds):
            for route, subset in routes.items():
                source=problem.query(subset)
                for backend in list(bins)[::1 if i%2==0 else -1]:
                    verdict,cost,out=run(args,directory,f"{i+1}-{backend}-{route}",bins[backend],source,args.timeout)
                    if verdict=="sat" and route in ("ordinary","split","known-core"):
                        raise ValueError(f"unexpected SAT on {case} {route} {backend}")
                    stats=dict(re.findall(r":([\w-]+)\s+(\d+)\b",out))
                    wr.writerow([case,i+1,backend,route,args.cpu,len(problem.all),len(subset if subset is not None else problem.all),
                                 verdict,cost,stats.get("quant-instantiations",0),stats.get("conflicts",0)])
                    sys.stdout.flush()


if __name__=="__main__":
    main()
