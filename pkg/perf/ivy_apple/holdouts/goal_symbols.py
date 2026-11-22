"""Solve goal-directed premise subsets without a reference core, with an optional solver budget."""
import argparse
import csv
from collections import Counter
from pathlib import Path
import re
import sys
import time

from premise_slices import HERE, Problem, parse, run, symbols


def select(problem):
    declarations = {f[1] for f in parse(problem.original) if f[0] in ("declare-fun","declare-const")}
    terms = [set(symbols(t)) & declarations for _,t,_ in problem.pieces]
    ground = {i for i,(_,_,n) in enumerate(problem.pieces) if n==0}
    targets = next(ids for ids,cmd in reversed(problem.commands) if ids is not None)
    initial = set().union(*(terms[i] for i in targets))
    if not initial:
        return {}
    freq = Counter(s for row in terms for s in row)
    result = {}
    for tolerance in (1,3):
        active = initial.copy()
        selected = set(targets)
        for depth in range(1,5):
            chosen=set()
            for i,row in enumerate(terms):
                shared = active & row
                if i not in selected and shared and min(freq[s] for s in shared)<=tolerance*min(freq[s] for s in row):
                    chosen.add(i)
            if not chosen:
                break
            selected.update(chosen)
            active.update(s for i in chosen for s in terms[i])
            result[f"rare{tolerance}-depth{depth}"]=sorted(selected)
            result[f"rare{tolerance}-depth{depth}-ground"]=sorted(selected|ground)
    # Distribute relevance over the bipartite graph of declarations and premises.
    # The goal supplies the seed. Frequent declarations spread their mass thinly.
    weights={s:1/len(initial) for s in initial}
    relevance=[0.0]*len(terms)
    for _ in range(6):
        incoming=[sum(weights.get(s,0.)/freq[s] for s in row) for row in terms]
        nxt=Counter({s:0.3/len(initial) for s in initial})
        for i,(score,row) in enumerate(zip(incoming,terms)):
            relevance[i]+=score
            if score:
                for s in row:
                    nxt[s]+=0.7*score/len(row)
        weights=nxt
    order=sorted(range(len(terms)),key=lambda i:(-relevance[i],i))
    for fraction in (0.1,0.25,0.5,0.75):
        ids=set(order[:max(1,round(fraction*len(terms)))])|set(targets)
        result[f"rank-{int(fraction*100)}"]=sorted(ids)
        result[f"rank-{int(fraction*100)}-ground"]=sorted(ids|ground)
    return result


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for arg in ("g3","output"):
        p.add_argument("--"+arg,type=Path,required=True)
    p.add_argument("--cases",nargs="+",required=True)
    p.add_argument("--routes",nargs="+")
    p.add_argument("--budget",type=int,default=35000)
    p.add_argument("--rounds",type=int,default=1)
    p.add_argument("--timeout",type=float,default=3.)
    p.add_argument("--cpu",type=int,required=True)
    p.add_argument("--perf",default="perf")
    a=p.parse_args()
    a.output.mkdir(parents=True)
    binary=a.g3.resolve(strict=True)
    out=csv.writer(sys.stdout,lineterminator="\n")
    out.writerow(["case","round","route","total","retained","budget","cpu","selection_wall_ms","answer","instructions","instances","steps"])
    for case in a.cases:
        directory=a.output/case
        directory.mkdir()
        problem=Problem(HERE.parent/"test_vectors"/(case+".smt2.gz"))
        before=time.perf_counter()
        routes=select(problem)
        construction=(time.perf_counter()-before)*1000
        for i in range(a.rounds):
            seen={}
            for name,selection in routes.items():
                if a.routes and name not in a.routes:
                    continue
                key=tuple(selection)
                if key not in seen:
                    original = problem.query(selection)
                    if a.budget:
                        original=f"(set-option :rlimit {a.budget})\n"+original
                    result,cost,stdout=run(a,directory,f"{i+1}-{name}",binary,original,a.timeout)
                    stats=dict(re.findall(r":([\w-]+)\s+(\d+)\b",stdout))
                    seen[key]=[result,cost,stats.get("quant-instantiations",0),stats.get("rlimit-count",0)]
                out.writerow([case,i+1,name,len(problem.all),len(selection),a.budget,a.cpu,round(construction,3),*seen[key]])
                sys.stdout.flush()


if __name__=="__main__":
    main()
