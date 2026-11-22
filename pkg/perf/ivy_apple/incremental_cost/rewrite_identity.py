"""Count disjoint outer rewrite calls in the ten first pushes and three incremental shell checks."""
import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys
from push_stages import HERE, PKG, replace_once
sys.path.insert(0,str(HERE.parent/"holdouts"))
from query_replay import CASES, query_inputs

FIRST = (1,4,7,10,13,16,19,22,25,26)


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ("output","goivy","spec","reference"):
        p.add_argument("--"+name,type=Path,required=True)
    p.add_argument("--cpu",type=int,required=True)
    p.add_argument("--rounds",type=int,default=3)
    a=p.parse_args()
    d=a.output.resolve()
    d.mkdir(parents=True)
    ivy=a.goivy.resolve()
    module=json.loads(subprocess.check_output(["go","list","-mod=readonly","-m","-json","github.com/glycerine/g3/pkg"],cwd=ivy))
    local=Path(module["Dir"])
    if not local.samefile(PKG):
        raise ValueError(f"unexpected module {local}")
    mapping={}
    def overlay(n,s):
        dest=d/n.name
        dest.write_text(s)
        mapping[str(n)]=str(dest)
        if n.is_relative_to(PKG):
            mapping[str(local/n.relative_to(PKG))]=str(dest)
    counter=(HERE/"push_stage_counter.go.txt").read_text()
    counter=replace_once(counter,"import (",'import (\n"github.com/glycerine/g3/pkg/ast"')
    counter=replace_once(counter,"active = true","Clear()\n active = true")
    counter=replace_once(counter,"active = false","Dump(ordinal)\n active = false")
    counter += """
type record struct {calls, cost uint64}
type key struct {kind string; unchanged, repeated bool}
var totals map[key]record
type descriptor struct {owner uintptr; root ast.ASTHandle}
var seen map[descriptor]bool
func Clear() { totals=make(map[key]record); seen=make(map[descriptor]bool) }
func Start() uint64 {
 if !active {return 0}
 depth++
 if depth==1 {return readCounter()}
 return 0
}
func End(start uint64) uint64 {
 if !active {return 0}
 depth--
 if start==0 {return 0}
 n:=readCounter()-start
 if n<=empty {return 0}
 return n-empty
}
func Add(n uint64, unchanged bool, kind string, owner uintptr, root ast.ASTHandle) {
 desc:=descriptor{owner,root}; repeated:=seen[desc]; seen[desc]=true
 k:=key{kind,unchanged,repeated}; v:=totals[k]; v.calls++;v.cost+=n;totals[k]=v
}
func Dump(i int) {
 for k,v:=range totals {
 fmt.Fprintln(os.Stderr,"IDENTITY",i,k.kind,k.unchanged,k.repeated,v.calls,v.cost)
 }
 seen=nil; totals=nil
}
"""
    overlay(PKG/"util/pushcounter/counter.go",counter)
    src=PKG/"ast/rewriter/rewriter_engine.go"
    s=replace_once(src.read_text(),"package rewriter",'package rewriter\nimport "github.com/glycerine/g3/pkg/util/pushcounter"')
    sig="func (r *rewriterEngine) rewrite(t ast.ASTHandle, out, proof ast.ASTRef) error {"
    s=replace_once(s,sig,sig.replace("error {","(resultErr error) {")+"""
 start:=pushcounter.Start()
 defer func(){
   n:=pushcounter.End(start)
   if n==0 || resultErr!=nil {return}
   h,e:=r.core.manager.RefGet(out); if e!=nil {panic(e)}
   pushcounter.Add(n,t==h,reflect.TypeOf(r.cfg).String(),reflect.ValueOf(r).Pointer(),t)
 }()
""")
    overlay(src,s)
    for src,search in [(PKG/"cmd_context/context.go","(status lbool.LBool, err error) {"),
                       (ivy/"smt/g3_solver.go","Push() { fail(s.c.Push()) }")]:
        s=src.read_text()
        s=re.sub(r"^package \w+$",lambda m:m.group()+'\nimport "github.com/glycerine/g3/pkg/util/pushcounter"',s,count=1,flags=re.M)
        if src.name=="context.go":
            sig=next(ln for ln in s.splitlines() if "func (c *Context) CheckSatDisplay(" in ln)
            s=replace_once(s,sig,sig+"\n defer pushcounter.Scope()()")
        else:
            s=replace_once(s,search,"Push() { defer pushcounter.Scope()(); fail(s.c.Push()) }")
        overlay(src,s)
    overlay_file=d/"overlay.json"
    overlay_file.write_text(json.dumps({"Replace":mapping}))
    for binary,cwd,args in [("shell",PKG,["./cmd/g3"]),("checker",ivy,["-tags=xtrace_off","./cmd/goivy_check"])]:
        subprocess.run(["go","build","-buildvcs=false","-mod=readonly","-pgo=off","-overlay="+str(overlay_file),"-o",d/binary,*args],cwd=cwd,check=True)
    wr=csv.writer(sys.stdout,lineterminator="\n")
    wr.writerow(["case","round","call","configuration","unchanged","repeat_engine_root","calls","instructions"])
    for case in ("rfn2",*CASES):
        for i in range(a.rounds):
            argv=["taskset","-c",str(a.cpu)]
            if case=="rfn2":
                args=[d/"checker","isolate=rfn2",a.spec.absolute()]
                source=None
            else:
                args=[d/"shell","-in","-smt2"]
                source=query_inputs(HERE.parent/"test_vectors"/(case+".smt2.gz"))["incremental"]
            run=subprocess.run(argv+args,input=source,text=True,capture_output=True,check=True,cwd=d,
                               env=dict(os.environ,GOMAXPROCS="1",XTRACE_OFF="1"))
            (d/f"{case}-{i+1}.log").write_text(run.stderr)
            expected=a.reference.read_text() if case=="rfn2" else "unsat\n"
            if run.stdout!=expected:
                raise ValueError(f"changed transcript {case}")
            for line in run.stderr.splitlines():
                if line.startswith("IDENTITY "):
                    _,ordinal,*values=line.split()
                    if case!="rfn2" or int(ordinal) in FIRST:
                        wr.writerow([case,i+1,ordinal,*values])
            sys.stdout.flush()


if __name__=="__main__":
    main()
