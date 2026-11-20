# SMT scope restoration performance study

**Keep the paged AST arena. Your roughly 3× gap reproduces.** With one solver, the default 48-processor Go runtime, and the same branching queue, the final g3 takes **38.54 µs** per scoped pop versus **12.27 µs** for Z3. With one CPU available to each process, g3 takes **22.89 µs** versus **12.31 µs**. That leaves **10.59 µs** of single-CPU work to remove, about 83 ns per retired and recreated Boolean variable. The additional gap on the unrestricted runtime is highly sensitive to GC pacing. Separate Go heap objects for AST nodes bought 4% on pinned pop and cost 24% on real AST creation and deletion.

## What the benchmark measures

Each round adds 64 three-literal learned clauses over 128 Boolean constants. It pops the inner search scope, verifies the clauses, then discards the outer user scope. The constants have external roots for the entire run. In `ScopedAtoms`, pop retires 128 Boolean bindings, restores 192 literal occurrences and reinstalls 128 watches. **The underlying AST nodes survive.** In `SurvivingAtoms`, even the Boolean bindings survive. This makes it a good control for the fixed cost of pop. These timings describe Boolean backtracking. An arithmetic or quantified solve exercises other paths. [Go fixture](smtcore/smt_hotpath_benchmark_test.go) · [C++ fixture](../src/test/smt_scope_restoration.cpp)

### Before and after with the same fixture

Measured November 19, 2026. Median of seven runs of 10,000 rounds per case, rotating binary order on logical CPU 6 of a Ryzen Threadripper 3960X. Go 1.27.3, linux/amd64. The in-tree C++ binary used GCC release flags including `-O3 -DNDEBUG`. Both Go revisions were compiled with the final fixture and its checks for clause identity, literal membership and polarity, binding level and relevance. The queue comparison uses a monotonic clock immediately around pop in both languages. The Go fixture uses its standalone SMT state, and C++ constructs a full context. The queue setting removes one important difference between them. [Individual samples](perf/scope_restoration/samples.csv)

| Operation | g3 before | g3 after | Z3 |
|---|---:|---:|---:|
| Surviving pop, default queue | 332 ns | **318 ns** | 469 ns |
| Scoped pop, default queue | 28.40 µs | **22.89 µs** | **12.31 µs** |
| Scoped full round, default queue, including Go setup and verification | 98.96 µs | **89.47 µs** | unmeasured |
| Scoped pop, original Go fixture without queue | 18.87 µs | **16.52 µs** | — |

The production changes cut queue-enabled scoped pop time by **19.4%** and whole-round time by **9.6%**. Go is faster on the pinned surviving case. Scoped pop is still **1.86×** native. Across the seven runs, the final scoped times ranged from 22.73 to 24.74 µs in Go and 12.25 to 13.15 µs in C++.

The original unpinned Makefile targets also show a real gap. Before this study I reproduced 1,189 ns / 37.40 µs for Go. Afterward I measured 724 ns / 26.80 µs for Go and 460 ns / 12.31 µs for C++. Those are single runs. CPU affinity, the timer and the work immediately before pop matter, especially for the sub-microsecond surviving case.

### Why the Makefile numbers differed

The old Go fixture fetched one clause pointer by allocating and copying **all** live clause records after each insertion. With 64 insertions, that copies 2,080 records before the clock starts. C++ saves the pointer returned by clause creation. Go's `testing.B.StartTimer` also calls `runtime.ReadMemStats` before taking the timestamp. C++ reads `steady_clock` directly. The memory-statistics read falls outside the reported interval, but changes the conditions under which pop starts.

On the original solver, with the queue inactive, five pinned runs isolated the effects. With insertion copies and the Go test timer, surviving/scoped pop took **502 ns / 20.50 µs**. Reading the lemma directly with the same timer gave **364 ns / 19.03 µs**. A monotonic clock with insertion copies gave **426 ns / 20.07 µs**, and with direct reads it gave **313 ns / 18.99 µs**. The direct fixture eliminated about **184 KB of allocation per round**. These are fixture effects. The before/after production table above holds the fixture fixed.

## Why a single solver is slower on the 48-processor runtime

Five additional unpinned runs compared both solver revisions with the same queue-enabled fixture on the machine as normally configured. Scoped pop fell from **48.23 to 38.54 µs** (20.1%), and the full round from **175.80 to 158.93 µs**. Z3 took **12.27 µs** for scoped pop. Surviving pop was **574 → 581 ns**, versus **466 ns** in Z3. An independent comparison gave **38.83 / 12.35 µs** for the final scoped case. The unrestricted result supports the original observation even after the improvements.

Restricting affinity also changes Go's default `GOMAXPROCS`. I varied them independently on the final solver. Each process had **one** active solver. These medians are from seven rotated runs of 5,000 queue-enabled scoped rounds.

| CPUs allowed | `GOMAXPROCS` | GC | Scoped pop |
|---|---:|---|---:|
| All 48 | 48 | Default | 38.17 µs |
| CPU 6 | 48 | Default | 29.58 µs |
| All 48 | 1 | Default | 22.47 µs |
| CPU 6 | 1 | Default | 22.49 µs |
| All 48 | 48 | Off for the bounded run | 19.64 µs |

The unrestricted runtime can execute this operation in under 20 µs when automatic collection is disabled. In a separate seven-run series, `GOGC=400` took **25.31 µs** versus **37.98 µs** at the default. The memory tradeoff is measurable. In three traced 6,000-round runs, the process made 45 collections at the default, 11 at 400, and four benchmark-forced collections with automatic GC off. Median peak RSS was **20.8, 30.7 and 114.0 MiB**, respectively. These settings were experimental controls. A single-process runtime setting would affect every worker.

The profile of a 100,000-round unrestricted run still puts 53% of measured pop samples under clause restoration and 44% in `popTo`. Background marking accounted for 5.7% of **whole-process CPU samples**. The wall-time difference also involves how work is scheduled. Five whole-process counter runs over 6,000 rounds recorded medians of **2.28 vs 2.27 billion user cycles** at one versus 48 Go processors, while aggregate task-clock rose from **0.545 to 1.089 seconds** and context switches rose from **171 to 2,108**. These totals include setup, verification, GC workers and teardown. They establish sensitivity to GC and runtime scheduling. They cannot apportion the timed-pop difference between CPU frequency, migration, kernel time and GC work. [Hardware and scheduling counters](perf/scope_restoration/runtime-counters.csv)

## Where the scoped pop spends time

I tagged only the measured Go pop and filtered its CPU samples by both the tag and the `PopScopes` stack. The later user-scope cleanup also calls `PopScopes`, so filtering on the function alone mixes two different operations. The final Go profile had 4.02 sampled CPU seconds in the measured pop. For C++, I recorded user-space stacks and selected the call site between the two `steady_clock::now()` calls. Of 3,883 sampled stacks at that site, 2,122 included `reinit_clauses`. Percentages below are inclusive. A queue insertion called during variable publication contributes to both rows.

| Scoped pop CPU samples | Final g3 | Z3 |
|---|---:|---:|
| Restore/reinitialize learned clauses | 53.2% | 54.6% |
| Publish/create a Boolean variable | 29.9% | 26.0% |
| Activity queue creation callback | 13.2% | 10.1% |
| Activity queue deletion callback | 9.0% | 11.6% |
| Pop relevance marks and owners | 7.0% | 7.2% |

Both solvers spend about half the pop rebuilding learned clauses. Applied to their measured medians, that's roughly **12.2 µs in Go versus 6.7 µs in C++**. This estimate points to about half of the remaining gap in that subtree alone. Go's `popTo` takes another 39.6% of its samples. The Go bulk write-barrier routine accounts for 10.7% inclusive, held-reference decrement for 7.7%, and AST slot lookup for 4.5%. GC can be active because the surrounding round allocates even when the measured pop reports zero allocations. [Go clause restoration](smtcore/smt_clause_backtrack.go) · [Go variable creation and retirement](smtcore/smt_internalizer_state.go) · [C++ clause restoration and scope pop](../src/smt/smt_context.cpp)

Z3 indexes its per-variable vectors with a dense ID, initializes a compact flag/justification record, resets the two watch lists and calls its branching queue. Undo clears the expression binding and pops the owning stack. Go already uses dense variable IDs. It also publishes a 128-byte Boolean record, maintains separate expression, creation, activity, assignment, watch and trail slices, and preserves progress across errors. A retained nullary constant already has a direct path to its AST slot. C++ itself combines an AST allocator and reference counts with a separate scope region. [C++ Boolean creation and undo](../src/smt/smt_internalizer.cpp) · [C++ Boolean record](../src/smt/smt_bool_var_data.h) · [C++ AST manager](../src/ast/ast.h) · [Go AST slot layout](ast/ast_core_storage.go)

## Memory management experiments

These prototypes ran the actual solver. The table gives queue-enabled scoped-pop medians against the starting solver in each series. Whole-round allocation metrics include setup and cleanup. The individual-object prototype keeps handle identity, generations and reference counting, which isolates placement. The borrowed-leaf prototype separately tests ownership traffic. [Experimental Go changes](perf/scope_restoration) · [Raw measurements and iteration counts](perf/scope_restoration/samples.csv)

| Experiment | Control → trial, scoped pop | Result |
|---|---:|---|
| Fresh Go heap object for every AST node allocation, including recycled slots | 28.58 → 27.38 µs | 4.2% faster pop. Full round 99.38 → 97.31 µs. AST construction happens outside these rounds. |
| Store up to four saved atoms inside the short clause allocation | 28.58 → 28.49 µs | 192 → 128 allocations/round. Size-class rounding raised bytes/round from 16,407 to 17,431. |
| Solver-local free lists for short clause atom buffers | 28.67 → 27.99 µs | 192 → 128 allocations and 16,418 → 11,299 bytes/round. Full round 100.24 → 97.98 µs. The pool retains its high-water storage. |
| Borrow the 128 explicitly rooted leaves for the fixture lifetime | 28.61 → 28.17 µs | Suppressing their reference-count updates saved 1.5%. Each leaf still had exactly its original reference at teardown. This prototype still checks the per-node borrowing flag. |
| Disable GC for a bounded diagnostic run | 28.21 → 24.42 µs | A 13.4% pop reduction. The process accumulates garbage across rounds in this configuration. |

**What happens when nodes actually die?** A separate control creates 128 distinct bound-variable ASTs with `MkVar`, takes references, drops all of them and repeats. Seven alternating runs of 5,000 batches gave **24.08 µs, 1,925 B and 4 allocations per batch** with pages, versus **29.86 µs, 18,306 B and 132 allocations** with individual objects. The 128 extra allocations are exactly one per node. Together the two workloads favor keeping the arena and changing the solver-local data around it. A pointer-only solver with a different ownership contract would be a larger experiment.

**Can batching allocations help the unrestricted runtime?** I also put 32 short learned clauses and their saved atoms in each solver-local page. Each slot was handed out once, preserving clause identity. At 48 Go processors this took **37.98 → 36.52 µs** per pop and **156.25 → 148.47 µs** per round. Allocations fell from **192 to 66**, but bytes rose from **16,414 to 17,438** per round. A single surviving clause can keep a whole page of 32 × 248 bytes alive. I left this as a prototype given the small pop gain and its mixed-lifetime cost. Trying an eight-literal temporary alone still produced 192 allocations and took 38.77 µs. Combining it with pages took 37.92 µs. [Page prototype](perf/scope_restoration/short-clause-pages.patch) · [Temporary-buffer prototype](perf/scope_restoration/short-literal-buffer.patch)

### Changes kept

- Release builds now eliminate heap and case-split diagnostic calls at compile time. The baseline executable still issued four assertion-helper calls directly from `Int.Insert`, even with assertions disabled. Disassembly of the new release build shows all four gone. The public heap guards and debug behavior are preserved. In a five-run isolation series, scoped pop fell from **28.67 to 25.29 µs**. [Heap implementation](util/heap/heap.go) · [Case-split assertions](smtcore/smt_case_split_assertions.go)
- Watch selection and restoration now read the assignment vector of the already-open solver with a literal bounds check. The public entry point still checks solver ownership. This change alone took **28.67 to 25.94 µs**. Together with the heap change it reached **23.72 µs** in the same series. Commit `404afcdff`. [Watch selection](smtcore/smt_internalizer.go) · [Clause restoration](smtcore/smt_clause_backtrack.go)
- Live learned clauses now carry a **48-byte restoration record**, down from **152 bytes** on amd64. Snapshots and interrupted assignments carry their additional data only when needed. The initial 72-byte prototype took **23.51 to 22.83 µs** in a seven-pair test, and **365.25 to 359.21 µs** at 2,048 atoms and 1,024 clauses. The final 48-byte layout is in the main comparison. At 1,024 live candidates it needs 104 KiB less record payload per solver before slice-capacity rounding. Commit `f51f119b7`. [Restoration state and retry handling](smtcore/smt_learned_persistence.go)

On three other SMT benchmarks, five paired runs gave Boolean propagation **5.30 → 5.27 µs**, relevance callbacks **15.35 → 14.27 µs**, and conflict resolution **234.7 → 240.5 ns**. The conflict median rose 2.5% and its sample ranges overlapped. These controls and the scope benchmark cover a small subset of solver workloads.

## Most viable paths to parity

1. **Reduce the bytes allocated by clause setup and cleanup on the default runtime.** Removing 126 allocations with pages saved only 1.46 µs when bytes per round went up. Increasing the GC heap target saved 12.67 µs at a cost of about 10 MiB peak RSS. The next experiment should combine byte reduction with explicit solver-local ownership of temporary literal and atom storage, measure live bytes after a large scope is discarded, and repeat at 1, 8 and 48 active solvers. Use those throughput and memory results to choose a retention limit or GC setting.
2. **Reduce writes per Boolean variable retirement and publication.** Start with one high-water array for the hot variable fields and a live length. Separate the pointer-bearing data needed for gates and justifications, and clear retired references in contiguous spans wherever callback ordering permits. The 128-byte Boolean record alone accounts for 16 KiB published per scoped pop. Use stable dense IDs, preserve fresh creation serials, and compare both simple leaves and mixed scopes with gates, enodes, proofs and refusal/retry. Measure the full round and retained heap at large and small widths. This targets the largest Go subtree and some of the barrier time.
3. **Specialize the delayed-activity heap operations used during backtracking.** An insert or erase can use a validated dense ID and the existing inverse-position array. Measure a single membership lookup, one capacity check at growth, and sift operations that only write moved entries. Both implementations pay for this queue. The current profile implies roughly 5.1 µs/pop in Go's creation/deletion callbacks versus 2.7 µs in C++. Use identical activity streams and compare the resulting decision order, including equal priorities. The first queue experiment already saved more time than any AST storage experiment. [Go activity queue](smtcore/smt_activity_heap_queue.go) · [C++ activity queue](../src/smt/smt_case_split_queue.cpp)
4. **Reduce per-literal work within clause restoration.** Compare a walk of the existing reinit buckets with one active retry cursor against copying candidates into a second list. Then measure selecting both watches from the rebound assignment values while preserving Z3's tie order. Exercise partially surviving watch pairs and unit/conflicting clauses as well as the all-undefined case. Preserve the existing clause identity and order of callbacks. This targets the roughly 5.5 µs difference in the clause-restoration subtree. The queue work is also inside that subtree, so the estimates overlap.

The single-CPU target is 10.59 µs. Under the unrestricted default runtime the measured gap is 26.27 µs. I would judge each change by both the queue-enabled scoped pop and full-round time, then check a representative SAT/SMT solve for wall time **and** decisions, conflicts and propagations. For thousands of solver goroutines, retained bytes per idle solver also matter. A high-water pool that saves a few percent here needs a separate peak-to-idle memory measurement before adoption.

## Reproduction

Run from `pkg/`. Use the same otherwise-idle CPU for both binaries. The original Makefile target still reports the isolated pop in `ns/op`. In the new queue comparison, compare **`pop-ns/op`** to Z3. Go's **`ns/op`, `B/op` and `allocs/op`** in that comparison cover the complete round, including its validation.

```sh
go test -c -o /tmp/g3-scope.test ./smtcore
taskset -c 6 /tmp/g3-scope.test -test.run='^$' \
  -test.bench='^BenchmarkSMTScopeRestoration(WithQueue)?$' -test.benchtime=10000x
taskset -c 6 ../build/test-z3 smt_scope_restoration 10000
```

For the unrestricted comparison, omit `taskset`, use `GOMAXPROCS=48` and repeat with `GOGC=100`, `400` and `off` at a fixed iteration count. The short-clause page and eight-literal buffer patches apply independently directly to `eb3575f6d:pkg`, which includes the queue benchmark.

The four earlier storage prototypes and their study fixture apply to the recorded starting revision in a scratch directory. Apply the fixture alone for a baseline, and add one storage patch in a fresh copy for each comparison.

```sh
study=$PWD/perf/scope_restoration
scratch=$(mktemp -d)
git -C .. archive 6e22fe9b3:pkg | tar -x -C "$scratch"
cd "$scratch"
patch -s -p1 < "$study/fixture.patch"
# For the individual-object experiment, also apply the following patch.
patch -s -p1 < "$study/ast-heap-objects.patch"
go test -c -o "$scratch/study.test" ./smtcore
STUDY_STOPWATCH=1 STUDY_QUEUE=1 taskset -c 6 ./study.test -test.run='^$' \
  -test.bench='^BenchmarkSMTScopeRestoration$' -test.benchtime=8000x
taskset -c 9 ./study.test -test.run='^$' -test.bench='^BenchmarkStudyASTChurn$' \
  -test.benchtime=5000x
```

The study fixture also accepts `STUDY_COPIES=1` for the insertion-copy control. For a pop-only Go profile set `STUDY_PROFILE=1 STUDY_STOPWATCH=1 STUDY_QUEUE=1`, select `/ScopedAtoms`, run at least 140,000 rounds with `-test.cpuprofile=pop.cpu`, and use pprof with `-tagfocus='phase=pop' -focus='\.PopScopes' -relative_percentages -top -cum`. For C++ I used `perf record -e cpu-clock:u -F 1499 --call-graph dwarf,8192` for 180,000 scoped rounds and resolved the measured caller using disassembly. Its sampled caller offset was `benchmark_scope_restoration+0x8b7` in this build.

The full Go `smtcore` and `util/heap` package tests passed. Focused checked and debug/trace tests passed for heap assertions, assignment, queue behavior, learned-clause preservation, scope pop and error recovery.

## conclusions

**Keep the AST arena.** Your roughly 3× gap reproduces with one solver on the default 48-processor Go runtime. The changes I kept cut scoped-pop time by about 20%, but g3 still trails Z3 on that case. The profiles, prototype code, raw measurements and reproduction commands are in [PERF_STUDY.md](/wd_black1tb/home/jaten/go/src/github.com/glycerine/g3/pkg/PERF_STUDY.md).

| Scoped pop with the default branching queue | g3 before | g3 after | Z3 |
|---|---:|---:|---:|
| Unrestricted, 48 logical CPUs | 48.23 µs | **38.54 µs** | **12.27 µs** |
| Both processes pinned to one CPU | 28.40 µs | **22.89 µs** | **12.31 µs** |

Two findings guide the next work.

- **Replacing the arena bought little.** Giving every AST node its own Go heap object improved pinned pop by 4%, but made creating and releasing 128 real AST nodes **24% slower**, adding exactly 128 allocations. The atoms in the scope benchmark already live across every round. What gets torn down and rebuilt is their solver state.
- **Allocation outside the timer affects pop.** On the unrestricted runtime, changing `GOGC` from 100 to 400 reduced scoped pop from **37.98 to 25.31 µs**, while peak process RSS rose from **20.8 to 30.7 MiB**. With automatic GC disabled for a bounded run, pop reached **19.64 µs**. GC pacing explains a substantial part of the observed gap, and there is still solver work to remove.

I kept three improvements: compiling away disabled activity-heap checks, avoiding repeated solver-owner checks during watch selection, and shrinking each live clause-restoration record from **152 to 48 bytes**. The full `smtcore` and `util/heap` package tests passed, along with focused checked and debug/trace tests.

**I’d target bytes allocated per search round and writes per Boolean variable next.** About 53% of scoped-pop CPU samples land in learned-clause restoration, including 30% in Boolean variable publication. Reducing temporary clause storage addresses the default-runtime penalty. Reducing the state written when each variable retires and returns addresses the remaining single-CPU gap. I’d measure both changes with multiple active solvers, where throughput and retained memory per solver matter together.
