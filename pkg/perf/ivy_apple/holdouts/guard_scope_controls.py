"""Known SAT models and UNSAT scopes for a quantified base reused across checks."""
import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys


def case(n,flip):
    clauses=["(set-option :smt.macro_finder true)","(declare-sort U 0)","(declare-const u U)",
             "(declare-const g3!premise!0 Bool)","(declare-const choose Bool)"]
    facts=[]
    for i in range(n):
        pred=f"p{i}"
        clauses.append(f"(declare-fun {pred} (U) Bool)")
        facts.append(f"(forall ((x U)) (= ({pred} x) choose))")
    clauses.append("(assert (and "+" ".join(facts)+"))")
    name,opp=("choose","(not choose)") if flip else ("(not choose)","choose")
    value="true" if flip else "false"
    alt="false" if flip else "true"
    end=f"(p{n-1} u)"
    cmds=[
        ("(push 1)",None),
        (f"(assert {name})",None),
        ("(check-sat)","sat"),
        (f"(get-value ({end}))",f"(({end} {value}))"),
        ("(push 1)",None),
        (f"(assert {opp})",None),
        ("(check-sat)","unsat"),
        ("(pop 2)",None),
        (f"(check-sat-assuming ({opp}))","sat"),
        (f"(get-value ({end}))",f"(({end} {alt}))"),
        ("(push 1)",None),
        ("(assert (not (= (p0 u) "+end+")))",None),
        ("(check-sat)","unsat"),
        ("(pop 1)",None),
        ("(check-sat-assuming (g3!premise!0))","sat"),
        ("(get-value (g3!premise!0))","((g3!premise!0 true))"),
        ("(check-sat-assuming ((not g3!premise!0)))","sat"),
        ("(get-value (g3!premise!0))","((g3!premise!0 false))"),
        ("(reset)",None),
        ("(assert false)",None),
        ("(check-sat)","unsat"),
    ]
    return "\n".join(clauses+[q for q,_ in cmds])+"\n",[v for _,v in cmds if v]


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("--g3",type=Path,required=True)
    p.add_argument("--native",type=Path,required=True)
    p.add_argument("--modes",nargs="+",required=True)
    p.add_argument("--sizes",nargs="+",type=int,default=(4,32,224))
    p.add_argument("--timeout",type=float,default=20)
    a=p.parse_args()
    writer=csv.writer(sys.stdout,lineterminator="\n")
    writer.writerow(["engine","mode","predicates","polarity","expected_responses","matched"])
    for n in a.sizes:
        for flip in (False,True):
            query,expected=case(n,flip)
            for name,mode,binary in [("native","default",a.native),*(("g3",m,a.g3) for m in a.modes)]:
                proc=subprocess.run([binary,"-in","-smt2"],input=query,text=True,capture_output=True,
                                    timeout=a.timeout,env=dict(os.environ,GOMAXPROCS="1",G3_PREMISE_GUARD=mode))
                actual=proc.stdout.splitlines()
                ok=proc.returncode==0 and actual==expected
                writer.writerow([name,mode,n,flip,len(expected),int(ok)])
                if not ok:
                    raise ValueError(f"{name} {mode} {n} {flip}: {actual} {proc.stderr[:160]}")
                sys.stdout.flush()


if __name__=="__main__":
    main()
