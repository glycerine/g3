# SMT scope restoration performance study

**Latest result on amd64.** g3 now matches Z3’s 16-byte Boolean variable and 8-byte justification. Pinned scoped pop takes 19.32 µs in g3 and 12.34 µs in Z3. [Record layout, timings and storage costs](#matching-the-native-boolean-record)

**Raft now completes in 124.71 seconds on g3**, down from 223.30 seconds on the same core. Resetting an exhausted GoIvy solver cut peak RSS from 3,665,916 to 988,940 KiB. All 419 proof results agree. The same change took GoIvy with C++ Z3 from 65.08 to 32.70 seconds. Python Ivy with C++ Z3 took 34.40 seconds. [Solver lifetime experiment](#solver-lifetimes-change-the-raft-search) · [Earlier Raft profiles and trials](#raft-verification-with-python-z3-and-goivy-g3)

The preceding follow-up measures the revised solver through 1,024 workers and includes the committed benchmarks. [Follow-up results and reproduction](#follow-up-on-clause-storage-boolean-variables-and-independent-solvers)

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

## Follow-up on clause storage, Boolean variables and independent solvers

**Keep the AST arena.** Compared with `e81927069`, the solver changes in `8adb6701b` reduced a queue-enabled scoped round from 192 allocations to 64 and from 16,402 to 10,257 bytes. They cut pinned pop time by 12%. With 48 independent solvers, aggregate throughput rose from 127,000 to 163,000 rounds per second. Z3 still finished a single scoped pop 1.75× faster on the pinned CPU. [Solver changes](smtcore/smt_internalizer_state.go) · [Worker benchmark](smtcore/smt_scope_workers_benchmark_test.go) · [Raw samples](perf/scope_restoration/worker-samples.csv)

### Scope and whole-round timing

Medians on the Threadripper 3960X, Linux/amd64, Go 1.27.3 and default GC. The Go measurements alternate the starting and revised binaries over seven processes each, with 10,000 rounds per process. The native column uses seven invocations of this repository's release `test-z3`. All three pop columns time the same 128 atoms and 64 learned ternary clauses with the delayed activity queue enabled. The round columns include Go setup, validation and outer-scope cleanup. [Go benchmark](smtcore/smt_hotpath_benchmark_test.go) · [C++ benchmark](../src/test/smt_scope_restoration.cpp)

| CPU configuration | g3 pop before | g3 pop after | Z3 pop | g3 round before | g3 round after |
|---|---:|---:|---:|---:|---:|
| Default, 48 logical CPUs | 38.80 µs | 35.07 µs | 12.32 µs | 160.01 µs | 142.25 µs |
| Both binaries pinned to CPU 6 | 24.51 µs | 21.54 µs | 12.33 µs | 98.74 µs | 85.22 µs |

The original `make bench` fixture times only pop and omits the branching queue. Its three-run scoped median moved from 27.72 to 24.23 µs. Its surviving-atom case moved from 666 to 689 ns. The queue-enabled comparison above is the one to use for the parity target.

### What earned its place

- **One literal copy per ordinary clause.** Insertion keeps its scratch private while a proof or assignment callback can reenter the solver. The clause and its internal record then share the clause's owned literal array. Watch-only binary records own their pair. This change alone removed 64 allocations per round. [Clause insertion](smtcore/smt_internalizer.go) · [Nested insertion regression](smtcore/smt_clause_storage_test.go)
- **Bounded, solver-owned saved atoms.** Each short clause borrows a slot for up to four atoms and returns it after its references have been released. Sixteen slots share one lazily allocated page, and each solver can retain up to four pages. Longer clauses and overflow use ordinary allocations. A long-lived clause occupies one slot while its neighbors can be reused. The 64 clauses themselves still have independent identities. In this fixture each now accounts for one allocation. [Storage and release](smtcore/smt_clause.go) · [Mixed-lifetime and rollback regression](smtcore/smt_clause_storage_test.go)
- **Smaller Boolean records.** A Boolean variable takes 80 bytes on amd64, down from 128. Its tagged justification takes 32 bytes, down from 40. Only AND/OR gates carry operand and literal slices. A short gate and its operand array share an allocation. NOT keeps its child inline. Retiring a gate releases its cached arrays even if the solver keeps the dense variable table for another search. [Variable and gate records](smtcore/smt_justification_refs.go)
- **Less work at each variable's creation and retirement.** The hot path skips an already-sized activity heap, an inactive relevance queue, an empty theory undo, and an equality index that never reached the variable. The release build also erases the disabled variable trace at its call site. On their own, these changes moved pinned pop from 23.23 to 20.34 µs in five paired runs. Whole-round time moved from 90.74 to 83.72 µs. [Variable events](smtcore/smt_internalizer_state.go) · [Activity queue](smtcore/smt_activity_heap_queue.go) · [Heap operations](util/heap/heap.go)

Each storage or layout experiment below ran the actual Go solver. Medians are paired with the control in the same series. The differences between series illustrate the need to remeasure combinations.

| Alternative | Observed result | Decision |
|---|---|---|
| Solver-local stack of cold gate records | Pinned pop 22.93 → 24.12 µs | Keep direct gate access. |
| Reserve all variable vectors together and grow them at a common high-water mark | Pinned pop 23.55 → 23.08 µs. Whole round 91.19 → 91.34 µs. | The added growth policy bought little. |
| Reset each field of a reused leaf conditionally | Pinned pop 23.00 → 21.93 µs alone. Adding it to the contiguous atom buffer gave 22.12 µs in both versions. | Keep the simpler reset. |
| Put three saved atoms inside the short clause allocation | 64 allocations and 14,372 B/round, versus 64 and 10,277 B with a bounded buffer in the same series | Reuse the temporary atom storage independently of the clause. |
| Use one contiguous atom buffer, or a free list of short arrays | Both reached 64 allocations. The contiguous buffer can strand free space until its last owner releases it. The free list retained about 1 KiB more per warmed solver than that buffer in the 48-worker trial. | The committed pages admit independent release and grow in 16-clause increments. |

The individual trial patches apply to `e81927069:pkg`. The raw CSV retains the short variant names from the binaries. `hi` is common vector growth, `dense` is the gate stack, `slots` is the independently reusable atom pages, and `smallgate` is the compact record with an inline NOT child. `smsg` combines the pages and compact record before the variable-event changes. [Trial patches](perf/scope_restoration/worker_trials) · [Trial measurements](perf/scope_restoration/worker-samples.csv)

### Many independent solvers

Each worker owns its AST manager, context, solver, queue and rooted expressions. The benchmark hands the warmed solver to one worker goroutine for the timed rounds. Aggregate time is wall time divided by completed rounds. Mean pop time measures time spent inside a worker's pop, including scheduling and GC. These metrics answer different questions once multiple solvers run concurrently. Worker counts 1, 8 and 48 use seven samples of 16,000 total rounds. The 1,024-worker and larger-scope cases use five samples of 8,192 and 3,072 rounds respectively. [Benchmark implementation](smtcore/smt_scope_workers_benchmark_test.go)

| Workers | Atoms per scope | Aggregate µs/round before → after | Mean worker pop µs before → after |
|---:|---:|---:|---:|
| 1 | 128 | 141.18 → 111.53 | 34.40 → 27.78 |
| 8 | 128 | 24.83 → 20.02 | 45.84 → 36.03 |
| 48 | 128 | 7.88 → 6.15 | 51.73 → 41.45 |
| 1,024 | 128 | 5.84 → 5.36 | 48.85 → 44.58 |
| 48 | 2,048 | 79.01 → 77.69 | 726.58 → 777.22 |

The larger scope exposes the storage limit. Of 1,024 short clauses in a round, 64 get cached atom slots, and the remaining 960 allocate atom arrays. The round fell from 3,072 to 1,984 allocations. Aggregate time improved 1.7%, while mean pop latency rose 7.0%. Increasing the default pool size would charge every solver that reaches the larger high-water mark. This case warrants a separate latency profile before changing the bound.

**Memory after returning to an empty scope.** Five fresh processes per configuration ran with `-benchtime=1x`. Every worker built and emptied one full scope before measurement. The incremental column is live Go heap growth from an initialized fixture with its atoms already rooted. The full column also counts the manager, solver, benchmark roots and fixture. Both use forced GC and keep every solver reachable. Fresh processes avoid including heap left by Go benchmark calibration in the starting measurement.

| Workers × atoms | Incremental KiB per solver before → after | Full fixture KiB per solver before → after |
|---|---:|---:|
| 1,024 × 128 | 87.39 → 86.42 | 522.00 → 520.98 |
| 48 × 2,048 | 1,236.87 → 1,067.59 | 2,911.53 → 2,742.30 |

### Search controls and remaining work

Every end-to-end sample returned the same result with identical decisions, conflicts, propagations, binary propagations where reported, and restarts. The first four wall-time medians use seven alternating pinned processes per binary. The final row uses nine samples and 20 solves separated by SMT-LIB `reset` in one process. Its counters are those reported for the final solve. [Comparison runner](perf/scope_restoration/compare_solves.py) · [Random 3-SAT input](perf/scope_restoration/random180.smt2) · [Search counts and wall times](perf/scope_restoration/worker-solves.csv)

| Input and entry point | Answer | Decisions | Conflicts | Propagations | Before → after |
|---|---|---:|---:|---:|---:|
| Pigeonhole 8, default | unsat | 14,510 | 12,048 | 198,118 | 160.41 → 164.36 ms |
| Pigeonhole 8, SMT tactic | unsat | 7,255 | 6,024 | 99,059 | 147.29 → 145.41 ms |
| Pigeonhole 9, SMT tactic | unsat | 21,539 | 17,975 | 250,639 | 590.00 → 602.81 ms |
| Random 3-SAT, SMT tactic | sat | 481 | 364 | 15,396 | 27.79 → 28.70 ms |
| Same random formula, 20 fresh solves | sat | 481 | 364 | 15,396 | 417.55 → 415.72 ms |

The search results show no general solve-time gain. In seven pinned 100,000-iteration controls, Boolean propagation took 5.35 → 5.69 µs, relevance callbacks 14.12 → 13.76 µs and conflict resolution 220 → 216 ns. A separate 150,000-iteration, seven-sample propagation series gave 5.42 → 5.44 µs. Propagation remains a control to keep when changing the Boolean record.

**The next likely gain is in variable publication and watch restoration.** The revised pinned CPU profile attributed 53.2% of inner-pop samples to learned-clause reinitialization, including 27.5% to publishing Boolean variables. Scope teardown accounted for 40.7%. Bulk GC write barriers fell from 14.2% of samples to 9.2%. These are inclusive samples from 160,000 rounds, with 3.53 s of filtered CPU samples before and 3.27 s after. The pinned latency gap to Z3 is still 9.20 µs per round. [Restoration loop](smtcore/smt_clause_backtrack.go) · [C++ variable creation and undo](../src/smt/smt_internalizer.cpp) · [Pop-only profile instrumentation](perf/scope_restoration/worker_trials/pop-profile.patch)

1. Measure the stores and write barriers attributable to the live variable record and the two watch lists separately. Z3 keeps these vectors at their high-water size. The common-growth prototype here shows that allocating the vectors together is insufficient. A narrower trial could leave empty watch lists alone when their variable retires, preserving clause pointers and retry behavior for populated lists.
2. Benchmark selecting both watches during restoration against the current two scans. Preserve Z3's watch ordering on ties, then use mixed assigned/unassigned clauses and the larger-scope latency case to judge the result.
3. For end-to-end speed, profile a representative search separately. The 20-solve control spent essentially the same wall time before and after this work. The scope fixture alone cannot establish which change will accelerate the full search.

### How much of the pinned gap can we explain?

The 21.54 µs g3 pop and 12.33 µs Z3 pop differ by 9.20 µs using unrounded medians. Multiplying those times by the respective CPU profile shares gives an approximate breakdown. It compares the current Go profile with the earlier profile of the same native executable. The rows partition each pop. The phase times are estimates from sampling.

| Work during the inner pop | g3 | Z3 | Estimated excess |
|---|---:|---:|---:|
| Publish Boolean variables inside learned-clause restoration | 5.9 µs | 3.2 µs | 2.7 µs |
| Other learned-clause restoration | 5.5 µs | 3.5 µs | 2.0 µs |
| Everything outside learned-clause restoration | 10.1 µs | 5.6 µs | 4.5 µs |

**A separate GC control accounts for about one quarter of the gap in that run.** Five rotated runs of 10,000 rounds on CPU 6 with one Go processor gave 21.31 µs for g3 at `GOGC=100`, 18.92 µs at `GOGC=off`, and 12.26 µs for Z3. Disabling automatic GC for the benchmark process saved 2.39 µs of the 9.05 µs difference, leaving 6.67 µs. This treatment also changes heap growth and runtime pacing. Its effect overlaps the profile categories above. With surviving Boolean bindings, the same three processes took 282, 276 and 474 ns respectively. [Raw samples, series pinned-gc-control](perf/scope_restoration/worker-samples.csv)

One concrete difference is the amount of per-variable state. With the native release build flags, `sizeof(smt::bool_var_data)` is 16 bytes and its tagged justification occupies 8 of them. At the baseline for this experiment, `f20d66552`, the Go records occupied 80 and 32 bytes. The subsequent packing change brought them to 16 and 8 bytes. Z3 leaves the variable vectors at their high-water size on undo. g3 also tracks a creation serial, truncates multiple live slices, clears retired watch pointers, and records whether releasing an AST reference succeeded so a failed pop can resume. These source differences identify work to isolate. The profiles alone cannot price each one, and the variable record sizes alone cannot predict the timing ratio. [Z3 variable records](../src/smt/smt_bool_var_data.h) · [Z3 variable undo](../src/smt/smt_internalizer.cpp) · [g3 variable undo](smtcore/smt_internalizer_state.go)

To repeat the GC control, run both subcases of `BenchmarkSMTScopeRestorationWithQueue` with `-test.benchtime=10000x` under `taskset -c 6`, alternating `GOMAXPROCS=1 GOGC=100`, `GOMAXPROCS=1 GOGC=off`, and the pinned native benchmark.

### Reproducing the follow-up

From `pkg/`, build the benchmark and run the named cases. Use a fresh invocation for each memory sample. To reproduce the baseline, export the starting package tree and add only the committed worker benchmark to it. The two CLI binaries can also be built from those trees with `go build -o /tmp/g3-after ./cmd/g3` and the corresponding before path.

```sh
go test -c -o /tmp/scope-after ./smtcore
before=$(mktemp -d)
git -C .. archive e81927069:pkg | tar -x -C "$before"
cp smtcore/smt_scope_workers_benchmark_test.go "$before/smtcore/"
go -C "$before" test -c -o /tmp/scope-before ./smtcore

/tmp/scope-after -test.run='^$' -test.bench='^BenchmarkSMTScopeRestorationWithQueue$/^ScopedAtoms$' -test.benchtime=10000x
taskset -c 6 /tmp/scope-after -test.run='^$' -test.bench='^BenchmarkSMTScopeRestorationWithQueue$/^ScopedAtoms$' -test.benchtime=10000x
taskset -c 6 ../build/test-z3 smt_scope_restoration 10000
/tmp/scope-after -test.run='^$' -test.bench='^BenchmarkSMTScopeRestorationWorkers$' -test.benchtime=16000x
/tmp/scope-after -test.run='^$' -test.bench='^BenchmarkSMTScopeRestorationPeakIdle$/Workers1024Atoms128' -test.benchtime=8192x
/tmp/scope-after -test.run='^$' -test.bench='^BenchmarkSMTScopeRestorationPeakIdle$/Workers48Atoms2048' -test.benchtime=3072x
/tmp/scope-after -test.run='^$' -test.bench='^BenchmarkSMTScopeRestorationPeakIdle$/Workers1024Atoms128' -test.benchtime=1x
python3 perf/scope_restoration/compare_solves.py /tmp/g3-before /tmp/g3-after
python3 perf/scope_restoration/compare_solves.py --batch --runs 9 /tmp/g3-before /tmp/g3-after
```

Use the other `PeakIdle` subcase with `1x` for the 2,048-atom memory measurement. For a single-variable comparison, apply its trial patch with `patch -p1` inside a fresh baseline export. For a CPU profile, apply `pop-profile.patch`, rebuild, run the pinned queue-enabled case with `-test.benchtime=160000x -test.cpuprofile=pop.cpu`, and filter pprof with `-tagfocus='phase=pop' -focus='\.PopScopes' -relative_percentages -top -cum`. The full Go `smtcore` and `util/heap` tests passed, as did focused checked, debug and trace tests.

## Matching the native Boolean record

On linux/amd64, both solvers now fit four Boolean records in a 64-byte cache line. The g3 dense table saves **64 bytes per slot**. One million slots save about **61 MiB** in the record array alone. The assignment trail also benefits from the smaller reason. [Go layout and constructors](smtcore/smt_justification_refs.go) · [Z3 record](../src/smt/smt_bool_var_data.h) · [Z3 justification](../src/smt/smt_b_justification.h)

| Compiled size | g3 at `f20d66552` | g3 packed | Z3 release |
|---|---:|---:|---:|
| Boolean variable, including its justification | 80 B | 16 B | 16 B |
| Boolean justification | 32 B | 8 B | 8 B |

The first machine word in g3 is a GC-visible pointer. Its low two bits identify a clause, binary literal, axiom or theory object. Clauses use their aligned pointer directly. The other tags use byte offsets inside aligned Go allocations. A copied reason therefore keeps its target alive even after the creating solver is gone. The remaining two 32-bit words follow Z3’s field widths, including the cached atom flag, 24-bit assignment level, 23-bit internalization level and 8-bit theory ID. A push beyond the representable internalization level returns an error before changing solver state. The five formerly writable flag and level fields have getters and setters. Use `Equal` for justification comparisons because independently constructed reasons can have different storage addresses. [Layout, bounds and GC tests](smtcore/smt_justification_refs_test.go)

**Costs outside the record.** Z3 encodes a binary literal as an immediate integer. Go uses immutable literal cells so every tagged pointer is valid for the collector to follow. The first 256 literal IDs share 1 KiB process-wide. For larger live IDs each solver allocates 1 KiB per touched group of 256, with a pointer directory indexed by group. Once a group exists, propagation reuses it. A standalone larger binary reason costs 4 B and one allocation. Constructing a theory-object reason costs 16 B and one allocation for its interface. Axiom, clause and small binary reasons measured zero allocations. The old constructors measured zero in all five cases. The two hot propagation benchmarks below also measured zero allocations per round at the reported precision. [Constructor and binding benchmarks](smtcore/smt_boolean_record_benchmark_test.go) · [Propagation benchmarks](smtcore/smt_hotpath_benchmark_test.go)

**Single-solver time.** Medians of seven rotated runs on CPU 6 of the Threadripper 3960X, with Go 1.27.3, `GOMAXPROCS=1` and `GOGC=100`. Each scope sample contains 10,000 rounds with the branching queue active. Propagation and conflict resolution use 30,000 rounds. Both propagation rows do the same 127 implications. The second uses 1,024 preceding variables to exercise the solver-local binary pages. [Raw timing and allocation samples](perf/scope_restoration/boolean-record-samples.csv)

| Work | g3 before | g3 packed | Z3 |
|---|---:|---:|---:|
| Inner pop, surviving atoms | 283 ns | 283 ns | 469 ns |
| Inner pop, scoped atoms | 21.16 µs | 19.32 µs | 12.34 µs |
| Full Go round, scoped atoms | 83.21 µs | 81.08 µs | |
| Boolean propagation, small IDs | 5.60 µs | 5.07 µs | |
| Boolean propagation, large IDs | 5.52 µs | 5.09 µs | |
| Conflict resolution | 219 ns | 215 ns | |

Packing and relocating the metadata together saved **1.83 µs per scoped pop (8.7%)**, closing about **21% of the measured gap**. The remaining gap is **6.98 µs**, or **1.57×** native. The before and after scoped samples ranged from 20.88–21.41 µs and 19.09–19.53 µs respectively. This experiment prices the production change as a whole. It cannot assign separate cycle counts to denser reads, shorter copies and fewer write barriers. A separate, unpinned run of the requested Makefile targets gave 23.71 µs for g3 and 12.29 µs for Z3 on scoped atoms (1.93×), and 783 ns and 473 ns on surviving atoms.

**Storage and gate costs.** NOT children and AND/OR metadata use solver-local pages indexed by dense variable ID. Each page covers 32 slots. Leaf-only solvers allocate no gate pages. Pop clears the retired gate references and child handles, and the solver can reuse the pages. The experiment below creates 4,096 bindings of one shape after rooting all ASTs. Live heap growth includes solver vectors, capacity slack and gate metadata. The times cover publishing and retiring all 4,096 bindings with warmed vectors. Entries are medians of five samples. [Gate storage](smtcore/smt_internalizer_state.go) · [Sparse-map trial patch against the packed version](perf/scope_restoration/worker_trials/boolean-gate-maps.patch)

| Binding shape | Live B/binding before → packed | Whole batch before | Sparse-map trial | Indexed pages, kept |
|---|---:|---:|---:|---:|
| Leaf | 399.7 → 317.7 | 331 µs | 311 µs | 315 µs |
| NOT | 421.7 → 368.2 | 346 µs | 542 µs | 365 µs |
| Two-input AND | 533.7 → 480.2 | 576 µs | 875 µs | 629 µs |

All three shapes retain less memory. The gate-heavy publication controls still cost **5% more for NOT and 9% more for AND** than the original inline metadata. After 1,024 independent solvers each built and emptied a 128-atom scope, incremental retained heap per solver fell from **86.4 to 76.7 KiB**. With 48 solvers and 2,048 atoms each, it fell from **1,067.3 to 939.3 KiB**. These are medians of three fresh-process, forced-GC samples per revision with 48 Go processors. [Raw heap samples](perf/scope_restoration/boolean-record-samples.csv)

**End-to-end checks.** Seven alternating pinned runs produced identical answers and search counters at both revisions. Median wall time went from 167.4 to 162.7 ms on default pigeonhole 8, 145.7 to 143.5 ms on SMT pigeonhole 8, 603.7 to 585.5 ms on SMT pigeonhole 9, and 27.9 to 28.2 ms on random 3-SAT. Theory-heavy solve time remains unmeasured here. An allocation profile on such a workload would establish whether the new theory-interface boxes merit further work. For the remaining scoped-pop gap, profiling watch restoration and variable teardown with the compact table in place would distinguish where the remaining 6.98 µs goes. [Search counters and raw wall times](perf/scope_restoration/boolean-record-solves.csv)

### Reproducing the record comparison

From `pkg/`, supply the same benchmark sources to the starting revision and the current tree. The packed solver and benchmarks are in `b32617162`. For the optional `--maps` executable, run `patch -p1 -i perf/scope_restoration/worker_trials/boolean-gate-maps.patch` in a copy of that package tree containing the trial patch, then rebuild its test binary. The standard SMT core suite and the suite built with `-gcflags=all=-d=checkptr=2` both passed. The GC regression exercises detached clauses, theory objects, binary pages and extreme signed literals.

```sh
go test -c -o /tmp/record-after.test ./smtcore
before=$(mktemp -d)
git -C .. archive f20d66552:pkg | tar -x -C "$before"
cp smtcore/smt_boolean_record_benchmark_test.go smtcore/smt_hotpath_benchmark_test.go "$before/smtcore/"
go -C "$before" test -c -o /tmp/record-before.test ./smtcore
python3 perf/scope_restoration/compare_boolean_records.py /tmp/record-before.test /tmp/record-after.test ../build/test-z3
go -C "$before" build -buildvcs=false -o /tmp/record-before ./cmd/g3
go build -buildvcs=false -o /tmp/record-after ./cmd/g3
python3 perf/scope_restoration/compare_solves.py /tmp/record-before /tmp/record-after
```

## Raft verification with Python Z3 and GoIvy g3

On November 20, 2026, the Python and Go commands in `~/ivy/goivy/log1` and `log2` reported the same 419 passing obligations and final `OK`. After removing absolute path prefixes, blank lines and a progress separator, the ordered proof transcripts matched. GoIvy can also use its vendored C++ Z3 through `classic_z3`. That control holds the Go frontend constant. Python reported Z3 4.7.1.0 and GoIvy's vendored native executable reported 4.7.1. The installed GoIvy binary here links released g3 v0.1.4. The fresh Go builds below use an explicit replacement pointing at this g3 checkout. [Workload runner](perf/ivy_raft/measure.py) · [Build script](perf/ivy_raft/build.py)

| Full verification, wall seconds | User’s logs on rog | Fresh run on aorus |
|---|---:|---:|
| Python Ivy + native Z3 | 34.110 | 37.03 |
| GoIvy + native Z3 | 63.892 | 66.93 |
| GoIvy + g3 | 213.791 | 219.68 |
| GoIvy/g3 ÷ Python/Z3 | 6.27× | 5.93× |
| GoIvy/g3 ÷ GoIvy/native Z3 | 3.35× | 3.28× |

The fresh column contains individual, uninstrumented runs on the Threadripper 3960X, Go 1.27.3, with normal process affinity. GoIvy and the Raft files were from `f379234145ce`. The g3 baseline was `4b6fac6c2`, with the 16-byte Boolean record already present. Switching the frontend and bridge can also change which formulas reach Z3 and in which order. The 29.90 s separating the native controls on aorus cannot all be charged to Go parsing or FFI.

### Where the CPU time goes

Python's cProfile charged **29.17 s** directly to 419 calls to `Z3_solver_check_assumptions` out of 38.82 s profiled. Building the parser tables took another 3.25 s inclusive. Independently, Linux perf attributed **77.74%** of user cycles to `libz3.so` and 15.12% to Python. Native relevancy lookup was the largest individual symbol at 6.47%, followed by the depth-one E-matching parent search at 3.83%. [Python and native profiles](perf/ivy_raft/python-profile.txt)

The whole-process Go profile collected 234.31 CPU seconds. **222.80 s (95.1%)** fell inside SMT kernel checks. Quantifier propagation accumulated 105.38 s (45.0%), auxiliary model-based counterexample searches 71.41 s (30.5%), and application rewriting 65.17 s (27.8%). These call trees overlap. The largest individual instruction attribution was the paged AST node lookup at 27.50 s flat (11.7%). The expression table lookup accounted for 9.66 s flat (4.1%). GC mark draining accounted for 8.21 s inclusive (3.5%). The sampling attribution to a load includes dependent memory stalls. [Full Go CPU profile](perf/ivy_raft/raft.cpu.pprof) · [C++ candidate checker](../src/smt/smt_checker.cpp) · [Go AST page access](ast/ast_core_storage.go)

With the same Go frontend, pinned to CPU 6, native Z3 retired **396 billion instructions** and consumed **308 billion cycles**. The packed g3 baseline retired **1.680 trillion instructions** and consumed **985 billion cycles**. g3 executed 1.71 instructions per cycle against 1.29 for classic Z3. It incurred 16.6 billion cache misses against 6.25 billion. This workload spends substantially more instructions per proof. A larger cache-miss count contributes, but the miss rate alone would predict the wrong ordering. Counters include the same lightweight measurement wrapper and use 83% hardware-counter multiplexing. [Hardware counters](perf/ivy_raft/hardware.csv)

Two obligations accounted for 168.4 of the baseline's 246.6 s in this pinned hardware-counter run. Time between successive printed passes includes the frontend work between checks. A separate native diagnostic binary from GoIvy `d2f5d38ab5b7` reran the same input and passed the transcript check. Every search counter emitted by g3 at the two checkpoints agreed with its native counterpart, including decisions, conflicts, quantifier instantiations and resource-limit count. These are cumulative solver counters at the end of each check. This comparison strongly favors a per-operation cost explanation for these two checks. [Per-obligation timing](perf/ivy_raft/obligations.csv) · [Matched counters](perf/ivy_raft/search.csv)

| Printed obligation | Native interval | g3 interval | Quantifier instantiations | Decisions | Conflicts |
|---|---:|---:|---:|---:|---:|
| 134, `committed_iff_in_commit_log` | 29.13 s | 104.90 s | 2,697,925 | 107,552 | 1,717 |
| 275, `choosable_vote_msg` | 27.16 s | 63.49 s | 605,502 | 1,579,772 | 15,695 |

### What the 16-byte Boolean record buys on Raft

Three paired runs compared `f20d66552` (80 B per variable, 32 B per reason) with `4b6fac6c2` (16 B and 8 B). Old → packed wall times were **232.29 → 235.33**, **249.40 → 251.47** and **253.57 → 248.62 s**. The time difference changed sign. Peak RSS was higher for the packed build in all three pairs, by **394, 316 and 47 MiB**. This differs from the controlled binding benchmark where packing saves 64 B for each dense slot. [Raw process samples](perf/ivy_raft/runs.csv) · [Layout and binding measurements](#matching-the-native-boolean-record)

In separate diagnostic processes, forced collections after checks 134, 275 and 419 found **3.8, 5.2 and 6.3 MiB less** live Go heap in the packed build. Total bytes allocated through check 419 increased by 99.5 MiB out of 20.9 GiB. Peak RSS covers spans, uncollected allocations and memory outside the live Go heap. These three snapshots cannot attribute the peak increase to a particular allocation or GC cycle. The largest live categories at check 134 were AST pages, cached substitutions, the AST canonical table and the SMT expression table. [Exact heap and allocation readings](perf/ivy_raft/heap.csv) · [Sampled allocation sites](perf/ivy_raft/heap-sites.txt)

### Optimizations tested

All single-change trials used fresh checker processes, `XTRACE_OFF=1`, fixed CPU affinity and one Go processor. Variants ran on separate physical cores and rotated cores between rounds. Other benchmark processes shared the machine. The paired samples show both run-to-run variation and the effect size. A focused benchmark and allocation measurements give independent evidence for the changes retained. [Full-run samples](perf/ivy_raft/runs.csv) · [Focused benchmark samples](perf/ivy_raft/microbenchmarks.csv)

| Trial | Measured effect | Decision |
|---|---|---|
| Keep a validated slice operand directly in a short AST application | Canonical arities 1, 2 and 3 took 162 → 150, 193 → 178 and 215 → 191 ns. Arity 4 was the wider control. The three full runs varied. | Kept for the measured constructor gain. The debug and borrowed-argument paths retain their validation. |
| Read one AST header per recursive candidate visit | Recursive formula check 256.3 → 239.2 ns, already-assigned literal 36.69 → 37.53 ns. Full checks finished faster in two of three pairs. | Kept. Assignment, relevancy and current reference count still come from live state. |
| Give the quantifier queue a stack-local prepared instance | The packed baseline's heap profile charged about 0.54 GiB of allocation to preparation by check 134. The exported constructor still returns independently owned storage. | Kept. The combined run measures the allocation reduction below. |
| Check congruence and argument identity before querying E-matching relevancy | The three full runs changed by −0.92, −1.81 and −0.86 s, under 1% each at these run lengths. Moving an indexed access ahead of relevancy would also require establishing its bounds for discarded parents. | Left the original code in place. [Experimental patch](perf/ivy_raft/filter.patch) |
| Allocate NOT child arrays and AND/OR gate arrays separately | Live bytes per binding for leaf, NOT, AND and alternating gates went from 317.7, 368.2, 480.2, 424.2 to 317.7, 356.4, 461.3, 421.3. Nine rotated samples with a shared directory gave 325, 375, 692 and 542 µs per 4,096 bindings, versus 329, 379, 698 and 548 µs originally. Two independent directories raised the leaf median to 339 µs. | Kept the shared directory. The Boolean record remains 16 B. [Two-directory trial](perf/ivy_raft/split-directories.patch) |
| Train the Go compiler on the complete g3 CPU profile | Baseline → PGO was 243.78 → 243.75 and 261.69 → 241.63 s with swapped cores. The test repeats the training workload. | Supplied the profile and opt-in build command. These two pairs do not establish a portable default. |

The compiled changes are in `0f8dcc875` and `6094de7ab`. At check 134, cumulative allocation fell from 9.134 to 8.570 GB (538 MiB saved). At check 419 it fell from 22.554 to 21.454 GB, **1.02 GiB less allocation (4.9%)**. The instrumented runs went through 105 and 101 GC cycles respectively, including three forced collections each. Post-GC live heap at 419 was 2.239 and 2.237 GB. The old layout, packed layout and retained changes all agreed on every recorded solver search counter at checks 134, 275 and 419. [Heap measurements](perf/ivy_raft/heap.csv) · [Search counters](perf/ivy_raft/search.csv)

The combined changes cut user instructions from **1.680 to 1.655 trillion (1.5%)** on CPU 6. That counter run went from 985 to 920 billion cycles and 246.59 to 222.09 s. Wall time failed to show a consistent gain across the timing controls. The two pinned pairs were 228.26 → 233.04 and 227.49 → 225.42 s. With normal affinity, the sequential final-then-baseline pair gave **232.66 s baseline and 239.79 s final**. The retained wins are allocation volume, gate storage and the measured constructor and recursive-check costs. Whole-program speed parity remains open. [All timed runs](perf/ivy_raft/runs.csv) · [Hardware counters](perf/ivy_raft/hardware.csv)

The Boolean size and forced-GC tests passed with strict pointer checks, and the final record benchmark reported **16 B/record and 8 B/reason** for all five construction cases. A fresh unpinned `make bench` / `make cppbench` during this study gave 25.69 / 13.18 µs per scoped pop and 850 / 541 ns on surviving atoms. The earlier seven-sample pinned series isolates the effect of packing more closely. The AST, SMT core and rewriter suites passed, as did the checked-handle AST and SMT core suites and focused debug/trace tests.

The most promising subsequent experiment is to reduce repeated AST lookups during quantifier substitution or rewriting and count retired instructions on the two slow checks. For allocation, fingerprint insertion and quantifier queue insertion together accounted for about 1.77 GiB through check 134. Their entries and growth policies are better targets for a memory-management prototype than another reduction in Boolean slot width. A cross-check on an independent quantified input would establish how much transfers beyond Raft.

### Reproducing the Raft comparison

Run from `pkg/` with the Ivy Python environment active. The builder uses a temporary module file and supports a separately extracted g3 revision. For an 80-byte control, extract `f20d66552:pkg` with `git archive` into a temporary directory and pass that directory as the second argument to the builder. The timing harness fails on a different proof transcript or a nonzero exit. Its output columns are sample, variant, CPU, wall seconds, user seconds, system seconds, peak RSS in KiB and pass count.

```sh
ivy="$HOME/ivy"
input="$ivy/ivy-lang-examples/examples/raft/raft_no_assume_test.ivy"
out=$(mktemp -d)
python3 perf/ivy_raft/build.py "$ivy/goivy" . "$out/g3"
python3 perf/ivy_raft/build.py "$ivy/goivy" . "$out/classic" --classic
python3 perf/ivy_raft/build.py "$ivy/goivy" . "$out/g3-pgo" --pgo perf/ivy_raft/raft.cpu.pprof
XTRACE_OFF=1 python -O "$(command -v ivy_check)" "$input" > "$out/reference.log"
python3 perf/ivy_raft/measure.py --reference "$out/reference.log" --cpu 6 --sample 1 --label g3 --output "$out/g3.log" -- "$out/g3" "$input"
python3 perf/ivy_raft/measure.py --reference "$out/reference.log" --cpu 6 --sample 1 --label classic --output "$out/classic.log" -- "$out/classic" "$input"

python3 perf/ivy_raft/build.py "$ivy/goivy" . "$out/cpu-checker" --profile cpu
XTRACE_OFF=1 IVY_STUDY_CPU="$out/raft.pprof" "$out/cpu-checker" "$input"
go tool pprof -top -cum "$out/cpu-checker" "$out/raft.pprof"
python3 perf/ivy_raft/build.py "$ivy/goivy" . "$out/heap-checker" --profile heap
XTRACE_OFF=1 GOMAXPROCS=1 IVY_STUDY_HEAP="$out/check" "$out/heap-checker" "$input"
go tool pprof -top -alloc_space "$out/heap-checker" "$out/check-134.heap"

XTRACE_OFF=1 python -O -m cProfile -o "$out/ivy.prof" "$(command -v ivy_check)" "$input"
XTRACE_OFF=1 perf record -F 149 -e cycles:u --call-graph fp -o "$out/ivy.perf" -- python -O "$(command -v ivy_check)" "$input"
perf report -i "$out/ivy.perf" --stdio --no-children -g none --sort dso
go test ./ast -run '^$' -bench '^BenchmarkASTCanonicalApplication$'
go test ./smtcore -run '^$' -bench '^Benchmark(SMTCheckerCandidate|SMTBooleanBindingStorage)$'
```

The local Go installation used `/usr/local/go1.24.3/pkg/tool/linux_amd64/pprof` in place of `go tool pprof` for the displayed reports. Each Go CPU and heap profile uses the standard pprof format.

### Reusing child hashes during quantifier substitution

During application construction, g3 has already read each child's AST record to validate the operand. The old hash pass followed the newly stored child references back through the AST manager. For up to three inline operands we now save their hashes during validation and register the candidate with the same structural hash. The canonical table, collision checks and ordering of reference acquisitions are shared with the general constructor. A fresh baseline profile put 64.82 of 222.90 sampled CPU seconds under proof-free application rewriting, including work by other rewriters as well as variable substitution.

| Work per iteration | Before | Reuse child hashes |
|---|---:|---:|
| Construct an existing unary application | 143.2 ns | 132.0 ns |
| Construct an existing binary application | 171.7 ns | 149.5 ns |
| Construct an existing ternary application | 185.5 ns | 165.8 ns |
| Construct an existing four-argument application, general path | 335.0 ns | 333.0 ns |
| Substitute two variables, retain the 64 possible instances | 1,974 ns | 1,926 ns |
| Substitute two variables, retire the previous instance | 2,644 ns | 2,584 ns |

Medians of nine alternating samples per build on CPU 9, with one Go processor. The substitution body contains a shared predicate application, a negation and an ITE. All six benchmarks measured zero allocations per iteration at the reported precision. The substituter gained about 2.4% for retained instances and 2.3% for retired instances. Run `go test ./smtcore -run '^$' -bench '^BenchmarkSMTQuantifierBodySubstitution$' -benchtime=50000x`. [Raw benchmark samples](perf/ivy_raft/substitution-hash.csv) · [Substitution benchmark](smtcore/smt_quantifier_substitution_benchmark_test.go)

### Reading a quantifier candidate's Boolean assignment

The quantified-instance checker read the Boolean assignment to decide whether a term was internalized, looked up its relevance, and then read the assignment again. All three pieces of information belong to the same per-expression table entry. The checker now validates the entry's generation once, reads the assignment and relevance from it, and reuses the AST header when it needs a congruent enode. A negation still reads the opposite value of its operand, including when the negation has a Boolean variable of its own. Each solver owns its table.

| Candidate | Starting revision `a407397b9` | Reuse assignment only | Reuse table entry and AST header |
|---|---:|---:|---:|
| Assigned atom | 38.30 ns | 30.27 ns | 28.37 ns |
| Formula with shared ITE | 231.6 ns | 219.6 ns | 196.0 ns |

These are medians of nine rotated 300,000-iteration runs on CPU 9, all at zero allocations. The complete Raft check on CPU 6 took 211.46 s before and 223.30 s after, while retired instructions fell from 1.644 to 1.633 trillion. Other processes ran concurrently. We kept the 26% and 15% improvements in candidate checking. This pair provides no evidence of a whole-Raft wall-time gain from that change. [Raw candidate samples](perf/ivy_raft/candidate-read.csv) · [Checker benchmark](smtcore/smt_checker_benchmark_test.go)

### Solver lifetimes change the Raft search

Python Ivy and GoIvy produced byte-identical native Z3 assertion dumps at checks 1, 8, 25, 49, 50, 92, 133, 134, 274, 275 and 321. Even with the same assertions, at check 134 GoIvy with native Z3 created over eleven times as many quantifier instances as Python Ivy. Each GoIvy bridge keeps references to translated terms. The Raft caller also kept the native state of private solvers after proving their obligations unsatisfiable. Python ordinarily drops the references when the wrappers go out of scope.

We varied those two lifetimes independently in Python using the same native Z3 library. Holding both reproduced GoIvy classic's quantifier instantiations, decisions and conflicts at **every one of the 419 checks**. The Boolean variable count matched at 418 checks and the resource counter at 91. Setting `smt.macro_finder` on each solver as a separate Python control reproduced the ordinary Python search. Reference lifetimes therefore change how much search work the solver performs. AST ID reuse and expression-table ordering are possible mechanisms. [Per-check search counters](perf/ivy_raft/lifetime-search.csv) · [Python lifetime experiment](perf/ivy_raft/python_lifetimes.py)

| Python wrapper lifetime | Wall time with statistics | Peak RSS, KiB | New quantifier instances at 134 | New quantifier instances at 275 |
|---|---:|---:|---:|---:|
| Ordinary | 37.84 s | 167,648 | 242,379 | 51,181 |
| Hold ASTs | 41.75 s | 349,500 | 937,292 | 63,093 |
| Hold solvers | 46.54 s | 869,584 | 1,161,264 | 60,917 |
| Hold both | 71.42 s | 1,216,068 | 2,695,395 | 299,277 |

Each instance count subtracts the preceding check on the same solver. The CSV contains cumulative solver statistics and identifies each check. For example, at check 275, ordinary Python made 162,589 new decisions and the hold-both control made 1,227,742. GoIvy classic's counters in this comparison were collected at Ivy revision `d2f5d38ab`, before the solver reset. To reproduce a Python control in the Ivy environment, run `XTRACE_OFF=1 python -O perf/ivy_raft/python_lifetimes.py --hold both --stats /tmp/raft-counters.csv -- "$input"`. [Raw timing and hardware samples](perf/ivy_raft/lifetime-runs.csv)

GoIvy commit `7e323d8a3` resets the private solver in `GetSmallModelWithCond` when the final result is UNSAT. In nonincremental mode it also resets the previous solver after creating its replacement. A returned SAT model retains its original solver. These calls use the backend's ordinary reset operation. The GoIvy bridge and small-model tests passed with the change.

| Checker | Pinned CPU | Wall time before → after | Peak RSS before → after, KiB | User instructions before → after |
|---|---:|---:|---:|---:|
| GoIvy, g3 `4f9e3ca87` | 6 | 223.30 → **124.71 s** | 3,665,916 → **988,940** | 1.633 T → **942.3 B** |
| GoIvy, C++ Z3 | 12 | 65.08 → **32.70 s** | 1,189,516 → **321,660** | 395.6 B → **215.5 B** |

A second g3 run on CPU 3 took 123.32 seconds and 942.8 billion instructions. Every timed process returned the complete ordered Raft transcript. The remaining gap against the same GoIvy frontend is about **3.8× in elapsed time and 4.4× in instructions**. The comparison with ordinary uninstrumented Python Ivy is 124.71 / 34.40, or **3.6×**. These backend comparisons used different physical cores of the same CPU. Earlier trials on CPU 6 included the child-hash optimization at 211.46 s and a further operand-pointer experiment at 217.28 s. The latter reduced instructions by 0.34% and had mixed constructor timings. We left it out. [Full-run measurements](perf/ivy_raft/lifetime-runs.csv)

A fresh post-reset g3 CPU profile collected 121.41 sampled seconds. Model-based quantifier checking used 54.80 s inclusive, application rewriting 39.50 s inclusive and AST node address lookup 12.54 s flat. The categories overlap. The post-reset heap run allocated 12.76 GB cumulatively through check 419 and retained 205 MB after forced GC. Dense expression-table growth accounted for 1.22 GiB of cumulative allocation, clause storage for 0.99 GiB and fingerprint insertion for 0.92 GiB. The Go and Python hold-ASTs controls agreed on instantiations, decisions and conflicts at both checks 134 and 275. Quantifier rewriting and the memory required for lookup tables are measurable next targets.

The post-reset native and g3 statistics collectors agreed on **all 24 shared integer counters at all 419 checks**, including resource count, clause production, quantifier instantiations, decisions, propagations and conflicts. Each collector ran a new process and sampled its solver after every check. The timed checks summed to 34.14 s in C++ and 136.86 s in Go in these instrumented runs. Check 134 took 13.29 and 41.28 s respectively, check 251 took 2.82 and 12.90 s, and check 275 took 2.56 and 12.20 s. Each CSV row contains the common counter values once. These controls support measuring cost per search operation directly. [Matched search and check times](perf/ivy_raft/matched-search.csv)

### Hashing quantifier binding tuples

The fingerprint set, the quantified-body substitution cache and the congruence table for applications with more than two arguments use the same tuple mixer. The old mixer multiplied and shifted three times for every AST handle. One round over the slot, generation and manager identity is enough to distribute the measured tuples. All three tables still compare their full keys on collisions.

| Operation | Arity | Original | Slot alone | Slot, generation and manager |
|---|---:|---:|---:|---:|
| Find an existing fingerprint | 1 | 20.91 ns | 9.53 ns | 9.21 ns |
| Find an existing fingerprint | 3 | 28.97 ns | 17.02 ns | 17.42 ns |
| Find an existing fingerprint | 6 | 45.57 ns | 22.94 ns | 24.55 ns |
| Insert a fingerprint and amortize scope pop | 1 | 109.5 ns | 97.2 ns | 98.55 ns |
| Insert a fingerprint and amortize scope pop | 3 | 135.1 ns | 123.9 ns | 126.1 ns |
| Insert a fingerprint and amortize scope pop | 6 | 215.7 ns | 201.0 ns | 202.3 ns |

Medians of nine rotated runs, 250,000 iterations each. The fixture uses 1,024 distinct tuples of each arity. We kept the single round with the complete handle. In a concurrent comparison on separate pinned cores, the starting version took 135.62 s and 941.0 billion instructions and the new mixer took 132.68 s and 933.0 billion instructions. A second slot-only comparison also retired fewer instructions. The local improvement in successful lookup is 40–56%. The observed whole-solver instruction saving is 0.85%. Run `go test ./smtcore -run '^$' -bench '^BenchmarkSMTFingerprint$' -benchtime=250000x`. [Raw samples](perf/ivy_raft/fingerprint-hash.csv) · [Benchmark](smtcore/smt_fingerprint_benchmark_test.go)

### Reusing the quantifier queue between propagation rounds

Native Z3 resets the length of its vector of pending quantifier instances after propagation. g3 discarded the backing array on every round and scope pop. The queue now clears the processed fingerprint pointers and reuses the array during the search. A full queue reset still releases it.

| Eager batch | Original median | Reused array | Original allocation per batch | Reused array |
|---|---:|---:|---:|---:|
| 32 entries | 991.7 ns | 235.1 ns | 1,640 B, 6 allocations | 0 B, 0 allocations |
| 1,024 entries | 21.17 µs | 6.47 µs | 59,368 B, 11 allocations | 3 B, 0 allocations |

The benchmark inserts and consumes each entry in order through the production scheduler. Medians cover nine rotated runs with 15,000 batches, including initial growth. In complete, instrumented Raft processes, allocation through check 419 fell from 12,761,535,144 to 12,187,276,536 bytes, **574 MB less (4.5%)**. Post-GC live heap at completion was 205.35 MB in both runs. Uninstrumented wall time in one concurrent comparison was 135.62 s before and 140.36 s after on different cores, with 941.0 and 939.3 billion user instructions. The case for this change is repeatable batch speed and lower cumulative allocation. [Raw samples](perf/ivy_raft/quantifier-queue.csv) · [Queue benchmark](smtcore/smt_fingerprint_benchmark_test.go)

### Expression-table storage on Raft

Auxiliary solvers share an AST manager and each owns its expression table. A write at a high AST slot used to grow and copy a flat array through that slot, including every untouched slot below it. We compared the original 32-byte record, a 24-byte flat record and 256-entry pages of 24-byte records on amd64. The smaller record stores the standalone three-valued assignment in one signed byte. The page directory grows on demand and enodes still iterate in slot order.

| Measurement | Flat 32 B | Flat 24 B | Paged 24 B |
|---|---:|---:|---:|
| Fill 8,192 adjacent slots | 370.2 µs | 326.1 µs | 108.2 µs |
| Fill eight slots spaced 1,024 apart | 119.3 µs | 104.6 µs | 9.41 µs |
| Dense, populated lookup | 2.29 ns | 2.52 ns | 3.62 ns |
| Sparse, populated lookup | 2.01 ns | 2.20 ns | 3.30 ns |
| Full Raft cumulative allocation through check 419, GB | 12.19 | 11.84 | 11.08 |
| Full Raft live heap after GC at check 419, MB | 205.35 | 199.77 | 183.68 |

Medians cover nine rotated allocation or lookup samples per build. Filling the dense table allocated 1,102,016 bytes in the original and 216,440 bytes with pages. The sparse fixture fell from 860,736 to 52,952 bytes. The flat-32 whole-process allocation baseline includes the retained queue change. The 24-byte heap runs include the same change and the new tuple mixer. Full uninstrumented runs with the mixer and the three layouts, before the queue change, took 132.68, 131.74 and 131.57 seconds on pinned cores 3, 6 and 3. User instructions were 933.0, 935.4 and 936.0 billion. All produced the reference proof transcript. We kept the paged layout for the 1.11 GB reduction in allocation and 22 MB reduction in live memory. The lookup penalty makes this a memory improvement with a measurable CPU tradeoff. [Benchmark](smtcore/smt_expr_table_benchmark_test.go) · [Raw samples](perf/ivy_raft/expr-table.csv)

### First-use allocation for rewriter activity caches

Creating a variable-substitution engine also creates activity caches for its shifters. An unused cache used to allocate ten hash cells immediately. Its zero value now represents the empty table. On first insertion it allocates the same eight primary cells and two overflow cells. Subsequent expansions still happen before duplicate lookup, preserving the order in which live AST references are released.

| Create, look up and close | Original median | First-use allocation | Bytes and allocations before → after |
|---|---:|---:|---:|
| Empty cache | 221.1 ns | 97.2 ns | 720 B / 2 → 224 B / 1 |
| Insert one key first | 350.7 ns | 328.7 ns | 736 B / 3 → 720 B / 3 |

Nine rotated samples used 400,000 lifetimes per workload. On the paged expression-table build, the full Raft proof allocated 11,076,409,104 bytes through check 419 before this change and 10,631,441,088 afterward, a further **445 MB (4.0%)**. Live heap after GC was 183.68 and 183.24 MB. Wall times with heap instrumentation were 128.94 and 127.29 seconds on different pinned cores. The original post-reset solver allocated 12.76 GB, so the combined queue, expression-table and activity-cache changes have removed 2.13 GB from the full proof. [Raw samples](perf/ivy_raft/activity-cache.csv) · [Lifetime benchmark](ast/ast_activity_cache_benchmark_test.go)
