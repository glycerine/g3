"""Compare alternative preprocessing or quantifier schedules through a complete GoIvy checker."""
import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys
import time


def fresh_total(stderr, ordinals):
    counts = []
    for line in stderr.splitlines():
        if line.startswith("IVY-MUTATION Push "):
            _, _, _, count, empty, enabled, running, _ = line.split()
            if int(enabled) <= 0 or enabled != running:
                raise ValueError("incomplete solver-thread instruction coverage")
            counts.append(int(count) - int(empty))
    if not ordinals:
        return ""
    if max(ordinals) > len(counts):
        raise ValueError("checker lacks the requested push intervals")
    return sum(counts[i-1] for i in ordinals)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--output",type=Path,required=True)
    p.add_argument("--reference",type=Path,required=True,help="expected complete stdout")
    p.add_argument("--expected-exit",type=int,default=0)
    p.add_argument("--spec",type=Path,required=True)
    p.add_argument("--isolate")
    p.add_argument("--build",action="append",required=True,metavar="NAME=CHECKER")
    p.add_argument("--setting",help="set this diagnostic environment variable to the build name")
    p.add_argument("--pushes",type=int,nargs="+",help="external push ordinals from the mutation observer")
    p.add_argument("--rounds",type=int,default=3)
    p.add_argument("--cpu",type=int,required=True)
    p.add_argument("--timeout",type=float,default=90)
    p.add_argument("--perf",default="perf")
    a=p.parse_args()
    builds={}
    for spec in a.build:
        name, sep, binary=spec.partition("=")
        if not name or not sep or name in builds or Path(name).name!=name:
            p.error(f"invalid or duplicate build {spec}")
        builds[name]=Path(binary).resolve(strict=True)
    if a.pushes and (min(a.pushes)<1 or len(set(a.pushes))!=len(a.pushes)):
        p.error("push ordinals must be distinct and positive")
    if a.rounds<=0 or a.timeout<=0:
        p.error("rounds and timeout must be positive")
    a.output.mkdir(parents=True)
    reference=a.reference.read_bytes()
    command=(["isolate="+a.isolate] if a.isolate else [])+[a.spec.absolute()]
    wr=csv.writer(sys.stdout,lineterminator="\n")
    wr.writerow(["round","variant","cpu","status","instructions","first_pushes","elapsed","rss_kib"])
    for i in range(a.rounds):
        names=list(builds)
        offset=(i//2)%len(names)
        names=names[offset:]+names[:offset]
        for name in names[::1 if i%2==0 else -1]:
            d=(a.output/f"{i+1}-{name}").resolve()
            d.mkdir()
            env=dict(os.environ,GOMAXPROCS="1",XTRACE_OFF="1",IVY_THREAD_EVENT="1")
            if a.setting:
                env[a.setting]=name
            before=time.monotonic()
            proc=subprocess.run(["taskset","-c",str(a.cpu),a.perf,"stat","-x",",","-e","instructions:u","-o",d/"perf",
                                 "--","/usr/bin/time","-f","%M","-o",d/"rss","timeout","-k","2",str(a.timeout),
                                 builds[name],*command],env=env,cwd=d,capture_output=True)
            elapsed=time.monotonic()-before
            (d/"stdout").write_bytes(proc.stdout)
            (d/"stderr").write_bytes(proc.stderr)
            if proc.returncode in (124,137):
                status="timeout"
            elif proc.returncode==a.expected_exit and proc.stdout==reference:
                status="ok"
            else:
                status="wrong-transcript"
            values=[r for r in csv.reader((d/"perf").read_text().splitlines()) if len(r)>4 and r[2]=="instructions:u" and float(r[4])==100.]
            if len(values)!=1:
                raise ValueError(f"{d} lacks full instruction coverage")
            total=fresh_total(proc.stderr.decode(),a.pushes) if status=="ok" else ""
            wr.writerow([i+1,name,a.cpu,status,values[0][0],total,round(elapsed,3),(d/"rss").read_text().splitlines()[-1]])
            sys.stdout.flush()


if __name__=="__main__":
    main()
