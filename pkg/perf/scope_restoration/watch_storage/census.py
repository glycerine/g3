"""Sample the occupancy and capacity of a solver's two watch lists per Boolean variable."""
import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys

PKG = Path(__file__).resolve().parents[3]
COLUMNS = ("pop", "active_headers", "reserved_headers", "active_clause_only", "active_literal_only",
           "active_both", "active_empty", "clause_backings", "literal_backings", "clause_slots", "literal_slots")
OBSERVER = '''
func (s *SMTInternalizerState) sampleWatchDirectory() {
 s.watchSamplePop++
 if (s.watchSamplePop-1)%64 != 0 || len(s.watches)<4 {return}
 active:=len(s.watches)
 var clauseOnly, literalOnly, both, empty, clauseBuffers, literalBuffers, clauseSlots, literalSlots int
 for i:=range s.watches[:cap(s.watches)] {
  w:=&s.watches[:cap(s.watches)][i]
  cs,ls:=w.Clauses(),w.Literals()
  if i<active{
   switch{case len(cs)>0 && len(ls)>0:both++;case len(cs)>0:clauseOnly++;case len(ls)>0:literalOnly++;default:empty++}
  }
  if cap(cs)>0{clauseBuffers++;clauseSlots+=cap(cs)}
  if cap(w.literals)>0{literalBuffers++;literalSlots+=cap(w.literals)}
 }
 fmt.Fprintln(os.Stderr,"IVY-WATCHES",s.watchSamplePop,active,cap(s.watches),clauseOnly,literalOnly,both,empty,clauseBuffers,literalBuffers,clauseSlots,literalSlots)
}
'''


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "reference", "output"):
        p.add_argument("--" + name, required=True, type=Path)
    p.add_argument("--modfile", type=Path)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--isolate", required=True)
    a = p.parse_args()
    root = a.output.resolve()
    root.mkdir(parents=True)
    state = PKG / "smtcore/smt_internalizer_state.go"
    source = state.read_text()
    declaration = "type SMTInternalizerState struct {"
    pop = "func (s *SMTInternalizerState) PopScopes(count uint32) error {"
    if source.count(declaration) != 1 or source.count(pop) != 1:
        raise ValueError("internalizer layout changed")
    imports = [f'"{k}"' for k in ("fmt", "os") if f'"{k}"' not in source]
    source = source.replace("import (", "import (\n" + "\n".join(imports), 1)
    source = source.replace(declaration, declaration + "\nwatchSamplePop uint64")
    source = source.replace(pop, pop + "\nif s!=nil{s.sampleWatchDirectory()}")
    observer = OBSERVER
    # The baseline keeps the whole physical literal backing in the slice;
    # the compact form stores the used length but retains identical capacity.
    watch = (PKG / "smtcore/watch_list.go").read_text()
    if re.search(r"\bliteralBlock\s+\*\[\]SMTLiteral\b", watch):
        observer = observer.replace("cap(w.literals)", "w.studyLiteralCapacity()")
        observer += "\nfunc(w *SMTWatchList) studyLiteralCapacity()int{if w.literalBlock==nil{return 0};return len(*w.literalBlock)}\n"
    patched = root / state.name
    patched.write_text(source + observer)
    flags = ["-modfile=" + str(a.modfile.resolve(strict=True))] if a.modfile else []
    module = Path(subprocess.check_output(["go", "list", "-mod=readonly", *flags, "-m", "-f", "{{.Dir}}",
                                           "github.com/glycerine/g3/pkg"], cwd=a.goivy, text=True).strip())
    if module.resolve(strict=True) != PKG:
        raise ValueError("GoIvy must use a local replacement pointing at this g3 checkout")
    overlay = root / "overlay.json"
    files = {str(base / state.relative_to(PKG)): str(patched) for base in (PKG, module)}
    overlay.write_text(json.dumps({"Replace": files}))
    binary = root / "checker"
    subprocess.run(["go", "build", "-tags=xtrace_off", "-pgo=off", "-buildvcs=false", "-mod=readonly",
                    *flags, "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"],
                   cwd=a.goivy.resolve(strict=True), check=True)
    with (root / "stderr").open("wb") as err, (root / "stdout").open("wb") as out:
        subprocess.run(["taskset", "-c", str(a.cpu), binary, "isolate=" + a.isolate, a.spec.absolute()],
                       cwd=root, stdout=out, stderr=err, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"),
                       check=True)
    reference = a.reference.read_bytes()
    if not reference.rstrip().endswith(b"\nOK") or (root / "stdout").read_bytes() != reference:
        raise ValueError("incomplete or changed proof transcript")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "sample", *COLUMNS])
    n = 0
    with (root / "stderr").open() as err:
        for line in err:
            if line.startswith("IVY-WATCHES "):
                row = list(map(int, line.split()[1:]))
                if len(row) != len(COLUMNS) or sum(row[3:7]) != row[1]:
                    raise ValueError(line)
                n += 1
                writer.writerow([a.isolate, n, *row])
    if n == 0:
        raise ValueError("no sampled pops")


if __name__ == "__main__":
    main()
