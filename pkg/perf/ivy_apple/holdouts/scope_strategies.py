"""Compare equivalent single-check query encodings and quantifier strategies on both shells."""
import argparse
import csv
import os
from pathlib import Path
import re
import subprocess
import sys

from query_replay import CASES, HERE, query_inputs

STATS = ("decisions", "quant-instantiations", "conflicts", "rlimit-count")
ROUTES = ("ordinary", "empty-scope", "assumption", "base-only", "no-macros", "late-instances", "model-first")


def encode(vector, route):
    queries = query_inputs(vector)
    base = queries["fresh"]
    cut = base.rindex("\n(assert ")
    end = base.index("(check-sat)", cut)
    head = base[:cut]
    final = base[cut:end].strip()
    if not final.startswith("(assert ") or not final.endswith(")"):
        raise ValueError("unrecognized final assertion")
    formula = final[len("(assert "):-1]
    if route == "base-only":
        # A refutation here is conclusive. SAT or UNKNOWN cannot settle the original query.
        return head+"\n(push 1)\n(check-sat)\n"
    if route == "empty-scope":
        # Both original assertions live in a single scope through the check.
        # This replay ends after the check. A real incremental client must preserve its base across Pop.
        at = base.index("\n(assert ")
        return base[:at]+"\n(push 1)"+base[at:]
    if route == "assumption":
        name = "replay_scope_active"
        if name in base:
            raise ValueError("activation literal occurs in input")
        return head+f"\n(declare-const {name} Bool)\n(assert (=> {name} {formula}))\n(check-sat-assuming ({name}))\n"
    query = queries["incremental"]
    if route == "no-macros":
        return query.replace(":smt.macro_finder true", ":smt.macro_finder false", 1)
    if route == "late-instances":
        return "(set-option :smt.qi.eager_threshold 0)\n"+query
    if route == "model-first":
        return "(set-option :smt.ematching false)\n"+query
    if route != "ordinary":
        raise ValueError(route)
    return query


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("g3","native","output"):
        p.add_argument("--"+name,type=Path,required=True)
    p.add_argument("--cpu",type=int,required=True)
    p.add_argument("--rounds",type=int,default=3)
    p.add_argument("--timeout",type=float,default=5)
    p.add_argument("--routes",nargs="+",choices=ROUTES,default=ROUTES)
    p.add_argument("--cases",nargs="+",choices=CASES,default=CASES)
    p.add_argument("--perf",default="perf")
    args=p.parse_args()
    if args.rounds<1 or args.timeout<=0:
        p.error("use positive round and timeout values")
    out=args.output.resolve()
    out.mkdir(parents=True)
    shells={"g3":args.g3.resolve(strict=True),"native":args.native.resolve(strict=True)}
    wr=csv.writer(sys.stdout,lineterminator="\n")
    wr.writerow(["query","round","engine","route","cpu","result","instructions",*STATS])
    for case in args.cases:
        for i in range(args.rounds):
            order=args.routes[i%len(args.routes):]+args.routes[:i%len(args.routes)]
            for route in order:
                source=encode(HERE.parent/"test_vectors"/(case+".smt2.gz"),route)+"\n(get-info :all-statistics)\n"
                for engine in list(shells)[::1 if i%2==0 else -1]:
                    stem=out/f"{case}-{i+1}-{route}-{engine}"
                    counter=stem.with_suffix(".perf")
                    # timeout applies to the solver, including native subprocesses, inside perf.
                    result=subprocess.run(["taskset","-c",str(args.cpu),args.perf,"stat","-x",",","-e","instructions:u",
                                           "-o",counter,"--","timeout","-k","1",str(args.timeout),shells[engine],"-in","-smt2"],
                                          input=source,text=True,capture_output=True,
                                          env=dict(os.environ,GOMAXPROCS="1",XTRACE_OFF="1"))
                    stem.with_suffix(".out").write_text(result.stdout+result.stderr)
                    if result.returncode in (124,137):
                        verdict="timeout"
                    elif result.returncode or "(error" in result.stdout:
                        raise ValueError(f"{stem} failed: {result.stdout[:160]} {result.stderr[:160]}")
                    else:
                        verdict=result.stdout.splitlines()[0]
                        if verdict not in ("unsat","sat","unknown") or route!="base-only" and verdict=="sat":
                            raise ValueError(f"{stem} returned {verdict}")
                    stats=dict(re.findall(r":([\w-]+)\s+(\d+)\b",result.stdout))
                    with counter.open() as f:
                        counts=[r for r in csv.reader(f) if len(r)>4 and r[2]=="instructions:u" and float(r[4])==100]
                    if len(counts)!=1:
                        raise ValueError(f"{counter} lacks full instruction coverage")
                    wr.writerow([case,i+1,engine,route,args.cpu,verdict,counts[0][0],*[stats.get(x,0)for x in STATS]])
                    sys.stdout.flush()


if __name__=="__main__":
    main()
