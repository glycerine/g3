"""Reproduce alternative incremental assertion strategies with all original premises enabled."""
import argparse
import csv
from pathlib import Path
import re
import sys

from premise_slices import HERE, Problem, run, smt


def encode(p, route):
    if route=="ordinary":
        return p.original,0
    if route=="split":
        return p.query(p.all),0
    if route=="named":
        return p.query(p.all,True),len(p.all)
    mode, count=route.split("-")
    count=int(count)
    names=[name for name,_,_ in p.pieces]
    guards=[]
    guard_by_row={}
    for ids,command in p.commands:
        if ids is None:
            continue
        selected=[i for i in ids if mode=="all" or p.pieces[i][2]>0]
        width=len(selected) if count==0 else count
        if width:
            for start in range(0,len(selected),width):
                group=selected[start:start+width]
                guard=names[group[0]]
                guards.append(guard)
                guard_by_row.update((i,guard) for i in group)
    result=[f"(declare-const {n} Bool)" for n in guards]
    for ids,command in p.commands:
        if ids is None:
            if command=="(check-sat)" and guards:
                command="(check-sat-assuming ("+" ".join(guards)+"))"
            result.append(command)
        else:
            for i in ids:
                _,term,_=p.pieces[i]
                if i in guard_by_row:
                    term=("=>",guard_by_row[i],term)
                result.append("(assert "+smt(term)+")")
    return "\n".join(result)+"\n",len(guards)


def main():
    cli=argparse.ArgumentParser(description=__doc__)
    for name in ("output","g3","native"):
        cli.add_argument("--"+name,type=Path,required=True)
    cli.add_argument("--cpu",type=int,required=True)
    cli.add_argument("--cases",nargs="+",required=True)
    cli.add_argument("--rounds",type=int,default=3)
    cli.add_argument("--timeout",type=float,default=5)
    cli.add_argument("--perf",default="perf")
    cli.add_argument("--routes",nargs="+",choices=("ordinary","split","named","quant-0","quant-1","quant-8","all-0","all-1","all-8"),
                     default=("ordinary","named","quant-0","quant-1","quant-8","all-0","all-1","all-8"))
    a=cli.parse_args()
    a.output.mkdir(parents=True)
    bins={"g3":a.g3.resolve(strict=True),"native":a.native.resolve(strict=True)}
    wr=csv.writer(sys.stdout,lineterminator="\n")
    wr.writerow(["case","round","engine","route","premises","selectors","cpu","verdict","instructions","instantiations","conflicts"])
    for case in a.cases:
        p=Problem(HERE.parent/"test_vectors"/(case+".smt2.gz"))
        directory=a.output/case
        directory.mkdir()
        for i in range(a.rounds):
            for route in a.routes[i%len(a.routes):]+a.routes[:i%len(a.routes)]:
                text,n=encode(p,route)
                for eng in list(bins)[::1 if i%2==0 else -1]:
                    v,c,out=run(a,directory,f"{i+1}-{eng}-{route}",bins[eng],text,a.timeout)
                    if v=="sat":
                        raise ValueError(f"SAT after enabling every premise in {case} {route} {eng}")
                    stats=dict(re.findall(r":([\w-]+)\s+(\d+)\b",out))
                    wr.writerow([case,i+1,eng,route,len(p.all),n,a.cpu,v,c,stats.get("quant-instantiations",0),stats.get("conflicts",0)])
                    sys.stdout.flush()


if __name__=="__main__":
    main()
