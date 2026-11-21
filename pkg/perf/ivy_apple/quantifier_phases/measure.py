"""Count retired instructions per quantifier candidate on one real Apple verification check."""
import argparse
from collections import defaultdict
from itertools import zip_longest
import csv
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from phase_counters import build, replace_once


HOOK = """
var qiCheckSequence int
func beginMeasuredCheck() func() {
    qiCheckSequence++
    if strconv.Itoa(qiCheckSequence) != os.Getenv("IVY_QI_CHECK") { return func() {} }
    runtime.LockOSThread()
    if err := os.Setenv("IVY_QI_ACTIVE", "yes"); err != nil { panic(err) }
    return func() {
        if err := os.Unsetenv("IVY_QI_ACTIVE"); err != nil { panic(err) }
        runtime.UnlockOSThread()
    }
}
"""


def instrument_queue(code):
    start = "void qi_queue::instantiate(entry & ent) {"
    code = replace_once(code, start, start + """
        qi_observation counter(static_cast<quantifier*>(ent.m_qb->get_data())->get_id(), ent.m_generation, ent.m_qb->get_num_args());
""")
    point = "if (m_checker.is_sat("
    code = replace_once(code, point, "counter.next(1);\n        " + point)
    point = 'TRACE("checker", tout << "instance already satisfied'
    code = replace_once(code, point, "counter.next(6);\n            " + point)
    code = replace_once(code, "expr_ref instance(m_manager);", "counter.next(2);\n        expr_ref instance(m_manager);")
    point = "m_subst(q, num_bindings, bindings, instance);"
    code = replace_once(code, point, point + "\n        counter.next(3);")
    point = "m_context.get_rewriter()(instance, s_instance, pr);"
    code = replace_once(code, point, point + "\n        counter.next(4);")
    point = "if (m_manager.is_true(s_instance)) {"
    code = replace_once(code, "        " + point, "        " + point + '\n            counter.outcome="true"; counter.next(6);')
    point = "m_context.internalize_instance(lemma, pr1, gen);"
    code = replace_once(code, point, 'counter.outcome="lemma"; counter.next(5);\n        ' + point + '\n        counter.next(6);')
    return code


def native_overlay(args, scratch):
    build_dir = args.build.resolve(strict=True)
    source = args.g3.parent / "src/smt/qi_queue.cpp"
    code = instrument_queue(source.read_text())
    altered = scratch / source.name
    altered.write_text(code)
    commands = subprocess.check_output(["make", "-sn", "-W", "../src/smt/qi_queue.cpp", "smt/qi_queue.o"],
                                       cwd=build_dir, text=True)
    command = shlex.split(next(line for line in commands.splitlines() if " -o " in line and line.endswith(" ../src/smt/qi_queue.cpp")))
    obj = scratch / "qi_queue.o"
    command[command.index("-o") + 1] = str(obj)
    command[-1] = str(altered)
    header = args.g3.parent / "src/test/perf/qi_candidate_measurement.h"
    subprocess.run([*command, "-I" + str(source.parent), "-include", header], cwd=build_dir, check=True)
    native_lib = scratch / "libz3_qi.a"
    shutil.copyfile(args.goivy / "z3vendor/native_lib/libz3.a", native_lib)
    subprocess.run(["ar", "rs", native_lib, obj], check=True)
    adapter = args.goivy / "smt/quantifier.go"
    patched = scratch / adapter.name
    patched.write_text(replace_once(adapter.read_text(), "${SRCDIR}/../z3vendor/native_lib/libz3.a", str(native_lib)))
    return {str(adapter): str(patched), str(adapter.resolve()): str(patched)}


def go_overlay(args, scratch):
    local = Path(subprocess.check_output(["go", "list", "-mod=readonly", "-m", "-f", "{{.Dir}}", "github.com/glycerine/g3/pkg"],
                                         cwd=args.goivy, text=True).strip())
    if local.resolve() != args.g3:
        raise ValueError("GoIvy must resolve g3 to the selected workspace")
    name = "smtcore/smt_quantifier_manager.go"
    src = (local / name).read_text()
    anchor = "for _, entry := range entries {\n\t\tvar prepared"
    src = replace_once(src, anchor, anchor.replace("var prepared", "qiStartEntry(q,entry)\n\t\tvar prepared"))
    anchor = "if instance == nil {\n\t\t\tcontinue"
    src = replace_once(src, anchor, anchor.replace("continue", "qiEndCounter();continue"))
    anchor = "if err = instance.Close(); err != nil {\n\t\t\treturn err\n\t\t}"
    src = replace_once(src, anchor, anchor + "\n\t\tqiEndCounter()")
    edits = {name: src}
    name = "smtcore/smt_qi_instance.go"
    src = (local / name).read_text()
    def before(point, extra):
        nonlocal src
        src = replace_once(src, point, extra + "\n" + point)
    before("satisfied, err := b.checker.run", "qiNextPhase(1)")
    before("if satisfied {\n\t\treturn nil", "if satisfied {qiNextPhase(6)} else {qiNextPhase(2)}")
    point = "return errors.Join(err, prepared.Close())\n\t}"
    src = replace_once(src, point, point + "\n qiNextPhase(3)")
    point = "rewritten, err := b.manager.RefGet(simplified)"
    if src.count(point) != 2:
        raise ValueError("the proof and proof-free rewriter boundaries changed")
    src = src.replace(point, "qiNextPhase(4)\n"+point)
    point = "if result.ReducedTrue() {\n\t\tif out != nil"
    src = replace_once(src, point, point.replace("if out", 'if qiObserved!=nil {qiObserved.outcome="true"};qiNextPhase(6)\n if out'))
    before("if err = internalizer.InternalizeInstance(result);", 'if qiObserved!=nil {qiObserved.outcome="lemma"};qiNextPhase(5)')
    point = 'return nil, errors.Join(fmt.Errorf("internalize instance lemma: %w", err), result.Close())\n\t}'
    src = replace_once(src, point, point + "\n qiNextPhase(6)")
    edits[name] = src
    edits["smtcore/qi_candidate_measurement.go"] = (HERE / "candidate.go.txt").read_text()
    mapping = {}
    for name, code in edits.items():
        f = scratch / Path(name).name
        f.write_text(code)
        for root in (local, args.g3):
            mapping[str(root / name)] = str(f)
    return mapping


def compile_one(args, engine):
    directory = args.output / engine
    directory.mkdir()
    mapping = native_overlay(args, directory) if engine == "native" else go_overlay(args, directory)
    overlay = directory / "solver.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    return build(engine, args, directory, HOOK, ("runtime", "os", "strconv"), go_overlay=overlay)


def measure(args, engine, name, check, cpu, reference, rnd):
    directory = args.output / f"{name}-{rnd}-{engine}"
    directory.mkdir()
    binary = args.output / engine / engine
    with (directory / "stderr").open("wb") as err:
        output = subprocess.check_output(["taskset", "-c", str(cpu), binary, "isolate="+name, args.spec],
                                         cwd=directory, stderr=err, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1",
                                                                           IVY_QI_CHECK=str(check)))
    if output != reference.read_bytes() or not output.endswith(b"OK\n"):
        raise ValueError(f"{engine} changed the proof transcript")
    return directory / "stderr"


PHASES = ("setup", "candidate_check", "substitution", "rewrite", "lemma", "internalize", "cleanup")


def candidate_rows(path):
    with path.open() as stream:
        ordinal = 0
        for line in stream:
            if not line.startswith("IVY-QI "):
                continue
            fields = line.split()[1:]
            ordinal += 1
            if len(fields) != 13 or int(fields[0]) != ordinal or fields[4] not in ("satisfied", "true", "lemma"):
                raise ValueError(f"invalid or nested candidate {ordinal} in {path}")
            yield fields[:5], tuple(map(int, fields[5:]))
        if ordinal == 0:
            raise ValueError(f"no candidate observations in {path}")


def summarize(name, check, cpu, rnd, files, destination):
    # Verify the entire candidate stream before writing a result. Stderr retains
    # the individual samples. Aggregation needs space only for distinct groups.
    totals = defaultdict(lambda: [0] * (2 + len(PHASES)))
    pairs = zip_longest(candidate_rows(files["g3"]), candidate_rows(files["native"]))
    for go, native in pairs:
        if go is None or native is None or go[0] != native[0]:
            raise ValueError(f"candidate sequences differ in {name} round {rnd}: {go} {native}")
        for engine, (identity, counts) in (("g3", go), ("native", native)):
            _, qid, generation, bindings, outcome = identity
            row = totals[engine, int(qid), int(generation), int(bindings), outcome]
            row[0] += 1
            for i, n in enumerate(counts, 1):
                row[i] += n
    with destination.open("w") as stream:
        out = csv.writer(stream, lineterminator="\n")
        out.writerow(["isolate", "check", "cpu", "round", "engine", "quantifier_id", "generation",
                      "bindings", "outcome", "candidates", "empty", *PHASES])
        for key, counts in sorted(totals.items()):
            out.writerow([name, check, cpu, rnd, *key, *counts])



def main():
    p = argparse.ArgumentParser(description=__doc__)
    for arg in ("g3", "goivy", "build", "output", "spec"):
        p.add_argument("--"+arg, required=True, type=Path)
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--case", action="append", required=True, nargs=4, metavar=("ISOLATE","CHECK","CPU","REFERENCE"))
    args = p.parse_args()
    if args.pairs < 1 or len({case[0] for case in args.case}) != len(args.case):
        p.error("use a positive pair count and distinct isolate names")
    args.g3 = args.g3.resolve(strict=True)
    args.goivy = args.goivy.resolve(strict=True)
    args.output = args.output.resolve()
    args.spec = args.spec.absolute()
    args.modfile = None
    args.output.mkdir(parents=True)
    with ThreadPoolExecutor(2) as pool:
        list(pool.map(lambda e: compile_one(args, e), ["g3", "native"]))

    def lane(case):
        name, check, cpu, ref = case
        for rnd in range(1, args.pairs + 1):
            order = ("g3", "native") if rnd % 2 else ("native", "g3")
            files = {e: measure(args, e, name, int(check), int(cpu), Path(ref), rnd) for e in order}
            summarize(name, check, cpu, rnd, files, args.output / f"{name}-{rnd}-summary.csv")

    with ThreadPoolExecutor(len(args.case)) as pool:
        list(pool.map(lane, args.case))


if __name__ == "__main__":
    main()
