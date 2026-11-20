# SMT scope restoration performance study

**Latest result on amd64.** g3 now matches Z3’s 16-byte Boolean variable and 8-byte justification. Pinned scoped pop takes 19.32 µs in g3 and 12.34 µs in Z3. [Record layout, timings and storage costs](#matching-the-native-boolean-record)

**The original Raft spec took 8.49 seconds on g3 and 35.93 seconds on native Z3** in a 22-run-per-backend crossover with the same GoIvy frontend. All 419 named obligations matched the Python reference. Reusing identical premises cut g3 from 10.56 seconds and 82.54 billion instructions to 8.49 seconds and 64.79 billion. The native SMT kernel still executes equivalent proof work faster. [Whole-checker measurements](perf/ivy_raft/order-premise-release.csv) · [Premise reuse and kernel comparison](#reusing-exact-premises-across-related-goals)

**The ordinary g3 solver completed Raft in 124.71 seconds**, down from 223.30 seconds on the same core. Resetting an exhausted GoIvy solver cut peak RSS from 3,665,916 to 988,940 KiB. All 419 proof results agree. The same change took GoIvy with C++ Z3 from 65.08 to 32.70 seconds. Python Ivy with C++ Z3 took 34.40 seconds. [Solver lifetime experiment](#solver-lifetimes-change-the-raft-search) · [Earlier Raft profiles and trials](#raft-verification-with-python-z3-and-goivy-g3)

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

### Reading quantifier substitution operands once

The proof-free beta reducer now obtains the expression kind, reference count, ground flag and either application arity or bound-variable index from a single AST slot lookup. Shared nodes and substitutions under binders still use the existing cache, index-shifting and reconstruction rules.

In nine alternating runs of 220,000 substitutions on one core, the median fell from **1,938 to 1,768 ns** when instances survived and from **2,597 to 2,433 ns** when each instance retired. Both workloads used zero allocations per operation. The corresponding full Raft experiment retired 933.7 billion user instructions before and 925.1 billion after. Single wall-time observations were 122.58 and 121.21 seconds. The final traversal matched the original 24 search counters on every one of the 419 verification conditions. Run `go test ./smtcore -run '^$' -bench '^BenchmarkSMTQuantifierBodySubstitution$' -benchtime=220000x`. [Individual substitution timings](perf/ivy_raft/substitution-shape.csv)

### Expression substitution during theory rewriting

Z3's theory rewriter asks its current substitution table for each term it visits. On g3 this lookup took 3.2% of the matched Raft kernel CPU profile. A hit probed the same dictionary twice. A miss still located the table by looking up its owner and reading a 16-byte arena payload. The AST manager now remembers its most recently used substitution, keyed by the full slot and generation, and clears that reference on destruction. The optional proof and dependency tables already encode the policy for this live owner. Checked and debug builds also validate the arena payload.

In eleven alternating samples on CPU 18, cycling over 512 present and 512 absent keys, median hits went from **48.20 to 24.02 ns** and misses from **43.75 to 25.33 ns**, with zero allocations in either build. Run `go test ./ast -run '^$' -bench '^BenchmarkExprSubstitutionFind$' -benchtime=2000000x -cpu=1`. [Benchmark](ast/ast_expr_substitution_benchmark_test.go) · [Raw lookup timings](perf/ivy_raft/expr-substitution-lookup.csv)

The 419-obligation matched kernel went from **11.75 to 11.53 s** and **94.57 to 92.83 billion** median user instructions over nine alternating pairs on CPU 6. All 18 search counters matched on every obligation. On the original bit-vector solver, a separate pair retired **925.93 and 916.54 billion** user instructions with all 24 search counters matching on every obligation. These runs took 135.34 and 131.64 s on different cores. [Whole-corpus timings and counters](perf/ivy_raft/expr-substitution-raft.csv)

### Shifting variables under binders

The variable shifter now reads the kind, reference count, ground flag and arity or variable index together. The traversal frame already holds the kind and arity and can reuse them when it returns from a child. In nine rotated samples of 200,000 shifts, forward shifts fell from **2,586 to 2,422 ns** and inverse shifts from **2,569 to 2,415 ns**, with zero allocations. The benchmark checks the exact result under a binder with a shared subexpression on every iteration. Run `go test ./ast/rewriter -run '^$' -bench '^BenchmarkQuantifiedVariableShift$' -benchtime=200000x -cpu=1`. [Benchmark](ast/rewriter/var_shifter_benchmark_test.go) · [Raw measurements](perf/ivy_raft/variable-shift.csv)

Over 15 alternating pairs on CPU 17, the median for all 419 matched kernel checks fell from **11.52 to 11.40 s** and from **92.74 to 92.33 billion** user instructions. All 18 per-obligation search counters matched in every run. [Individual kernel runs](perf/ivy_raft/variable-shift-raft.csv)

### Finding free variables under quantifiers

The free-variable collector now follows Z3's rule of memoizing nodes with more than one reference. It skips ground applications, obtains a variable index or application arity together with the node kind, and starts the worklist in a 16-element stack buffer. The map key contains a slot and binder depth. A node can represent different free variables at different depths, and the root keeps its reachable slots live for the entire visit. [Collector](ast/rewriter/used_vars_ufbv.go)

On a shared expression visited at depths zero, one and two, the median fell from **4,780 to 260 ns**. Each collection previously allocated 4,856 bytes in 11 allocations and now allocates zero. This fixture includes a shared ground subtree that can be skipped entirely. Eleven alternating samples used 300,000 iterations on CPU 7. Run `go test ./ast/rewriter -run '^$' -bench '^BenchmarkUsedVarsUnderQuantifiers$' -benchtime=300000x -cpu=1`. [Benchmark and depth regression](ast/rewriter/used_vars_ufbv_test.go) · [Raw samples](perf/ivy_raft/used-vars-lookup.csv)

On all 419 actual obligations, five fresh processes per build gave median allocated bytes of **2,639,772,064 before and 2,566,255,384 after**, a saving of **73.5 MB and 328,956 allocations**. Median benchmark time on CPU 21 was 10.96 and 10.91 s. A separate nine-pair run on CPU 18 measured 91.77 and 91.08 billion user instructions with identical search counters. Its wall medians were 12.10 and 12.34 s while other work ran on the machine. The repeatable whole-corpus benefit is the reduction in allocations and instructions. [Allocations](perf/ivy_raft/used-vars-allocations.csv) · [Matched checks](perf/ivy_raft/used-vars-raft.csv)

### Calling the rewriter configuration directly

The C++ rewriter specializes its callbacks on a concrete configuration. g3 previously dispatched through two Go interfaces for each externally configured traversal. Both interfaces described the same operations. The engine now calls the existing public `Configuration` directly, including for proof-producing traversals. One implementation supplies the default methods. [Traversal](ast/rewriter/traversal.go) · [Default callbacks](ast/rewriter/rewriter_configuration.go)

In twelve alternating pairs on CPU 19, the median for the 419 matched queries went from **11.23 to 11.08 s** and **91.17 to 90.10 billion** user instructions. The search counters agreed for every obligation in every pair. The checked, debug and trace build also passed the focused proof, rewriting and quantifier tests. [Individual runs](perf/ivy_raft/rewriter-callbacks.csv)

### Repeated operator and declaration reads during rewriting

The application rewriter now inspects a parent only after a child can affect the result. Ordinary ITE folding needs the condition. Short-circuit model evaluation can also use each rewritten AND/OR operand. One lookup identifies the parent operator, and one lookup distinguishes true and false in the condition. The application factory separately skips a second declaration-family lookup on the ordinary matching-arity path. The first lookup already checks the function declaration and supplies its flags. Debug and checked builds retain the extra validation.

The combined production changes retired **89.45 billion instructions versus 90.16 billion** on the 419 matching-search Raft obligations. Median time was **11.27 versus 11.36 s**, across 18 samples per build on two cores. All 18 counters matched per obligation. In the existing canonical-application benchmark, the arity check alone reduced calls with one, two and three arguments from 140.0, 162.7 and 172.2 ns to 134.4, 156.7 and 169.4 ns. Four-argument calls took 345.8 and 346.7 ns. Forwarding a declaration-record pointer through the lower factory saved another 0.28 billion instructions in isolation, with no improvement on the two-argument microbenchmark. Combining declaration and reference-count reads in the rewriter added no further gain. [Canonical benchmark](ast/ast_app_benchmark_test.go) · [Factory samples](perf/ivy_raft/declaration-applications.csv) · [Factory replay](perf/ivy_raft/declaration-kernel.csv) · [Operator variants](perf/ivy_raft/rewrite-constant-fold.csv) · [Combined replay](perf/ivy_raft/rewrite-arity-integrated.csv)

With the same corpus and fixed search, compiling for `GOAMD64=v1`, `v2` and `v3` gave medians of 90.14, 90.05 and 89.71 billion instructions. Median times were 11.42, 11.71 and 11.32 s over 20 runs per target on the Threadripper. The production build keeps its portable amd64 target. [Target-level samples](perf/ivy_raft/amd64-kernel.csv)

### Order abstraction for the concrete Raft verifier

The timed input was the original `examples/raft/raft_no_assume_test.ivy`. Its eight abstract types have concrete bit-vector interpretations of widths two, three and four. The GoIvy frontend generated the original assertions for both backends. The proof strategy receives those SMT assertions, with no file names, conjecture names, expected answers or Ivy type metadata. In an earlier diagnostic, checking the separate verification-oriented source without the finite interpretations took 11.96 s on g3. That timing is for a different input.

| Whole process, six samples per backend | Pure-Go g3 with order proof | GoIvy with native C++ Z3 |
|---|---:|---:|
| Wall time, median (range) | **25.94 s** (25.84–26.82) | **32.53 s** (32.29–32.77) |
| Median peak RSS | 114,672 KiB | 321,036 KiB |
| Median user cycles | 108.88 billion | 138.03 billion |
| Median user instructions | 230.58 billion | 215.52 billion |
| Ordered verification transcript | 419 PASS, final OK | 419 PASS, final OK |

Each backend ran three times on each of logical CPUs 6 and 12. The two series used opposite orders. Each process ran on one core with `GOMAXPROCS=1`, tracing off, and solver reset after each completed action. Every timed run was checked against Python Ivy's reference transcript, including the obligation names and order. These crossover numbers are for the in-process prototype. The integrated, resource-bounded implementation took 25.81 s in a separate complete run. The wall-time improvement over native is 20.3% on this input. [Raw crossover timings and counters](perf/ivy_raft/order-crossover.csv) · [Transcript checker](perf/ivy_raft/measure.py)

#### Running the proof directly in the SMT kernel

The transformed problem needs quantified uninterpreted functions and order axioms. g3's general strategy adds tactic preprocessing and then invokes the SMT kernel. Selecting `NewSimpleSolver` for the independent proof calls that kernel directly. In the earlier strategic run, CPU sampling attributed 14.70 s to the tactic solver and 11.17 s to its nested SMT kernel setup and check in a 25.51 s profile. The strategy choice also changes the resulting search.

| Backend | Median wall time | Range | Median peak RSS | Median user instructions |
|---|---:|---:|---:|---:|
| g3, direct SMT kernel for the order proof | **11.57 s** | 11.44–12.12 s | 83,858 KiB | 94.27 billion |
| g3, general strategy for the same order proof | 27.76 s | 26.78–29.52 s | 111,792 KiB | 230.32 billion |
| GoIvy, C++ Z3 on the concrete problem | 35.34 s | 33.45–36.13 s | 321,220 KiB | 215.52 billion |

Each backend ran twice per core on cores 6 and 12. The checker output matched Python in all twelve executions. The direct kernel took 58% less time than the previous order strategy and 67% less than native Z3 checking the original finite-domain assertions. The machine also ran regression tests on other cores during part of this series. Across all four direct-kernel samples the instruction counts agree to within 0.2%. An additional integrated run on CPU 7 took 14.07 s with 94.19 billion instructions. [Individual runs and order](perf/ivy_raft/order-kernel.csv)

GoIvy commit `b20d9b188` pins g3 `v0.1.6-0.20261120122859-fe15489ffd20` in its module file. A fresh `go build -tags=xtrace_off ./cmd/goivy_check` from that GoIvy checkout ran the original Raft file in **11.46 s**, used **80,616 KiB** peak RSS and retired **94.25 billion** user instructions. Its 419 named obligations and final `OK` matched Python Ivy. The `xtrace_off` tag disables trace logging for timing. A second build with tracing enabled reproduced all 27,782 Python trace events and all 27,855 normalized output lines for the Multi-Paxos fragment error reported in `log3`. Both checkers exited with status 1 at the unquantified variable on line 253. GoIvy commit `6f5c188a6` contains that fix and its regression test. [Production and prototype measurements](perf/ivy_raft/order-integration.csv)

On the standalone fixtures the direct order proof took 26.3 ms on check 1 and 219 ms on check 274, including parsing, context creation and destruction. Simply forcing the general proof solver into incremental mode took 19.60 s on the full spec. Using the SMT-LIB command context to execute the general strategy took 27.37 s. Disabling macro discovery on that strategy raised elapsed time to 115.08 s, including concrete fallbacks. All three produced the reference transcript. Resource exhaustion during assertion insertion as well as search gives control back to the concrete solver. The bounded-retry, concrete model, scope, and checked-handle tests pass with the direct kernel.

#### Why the relaxation proves the concrete obligation

A bit-vector used as a node identifier costs the concrete solver every axiom and instantiation that finite bit-vector interpretation entails. In these obligations most uses depend on equality, unsigned order and uninterpreted functions. The proof attempt assigns a fresh domain to each bit-vector function position, bound variable, and numeral occurrence. It unifies two domains wherever passing an argument, testing equality, comparing values or joining conditional branches requires the same type. Positions of equal bit width can therefore remain distinct. No arithmetic or signed comparison is translated in a retained formula.

For any concrete model, give each abstract domain a separate copy of all values of the original width. Interpret each function by transporting its concrete inputs and output through those copies. Interpret the strict-order relation as unsigned less-than and each numeral as its own value. The added axioms express irreflexivity, transitivity, totality, zero as the least value, and the order and distinctness of observed numerals. All hold in that model. Renaming binders preserves quantifier scope. Dropping a whole theory-only **top-level conjunct** can only weaken the assertions. Consequently a concrete satisfying model supplies an abstract satisfying model. Refuting the abstraction rules out all concrete models.

The extra solver runs on the caller's goroutine. It may return only a refutation to the verifier. A satisfiable abstraction, unsupported operation, or speculative resource limit sends the obligation to the original solver with its concrete assertions and scope intact. Core and model APIs continue to use the ordinary solver. The private attempt has a three-million-step ceiling. Across 419 successful prototype checks the median was 24,579 steps, the 99th percentile 507,411 and the largest 2,118,282. Explicit finite source limits bypass the extra search. [Inference and eligibility](api/internal/order/relax.go) · [Solver entry point](api/api_order_abstraction.go) · [Correctness controls](api/api_order_abstraction_test.go)

A deliberately false `X:node = Y:node` conjecture added to a temporary copy of the concrete wrapper produced the **same failed conjecture**, 427 other passing checks, exit code 1 and byte-identical output on both backends. An independent two-bit test asks for five distinct values. The relaxation admits a larger domain and the original solver proves the concrete formula unsatisfiable. Additional tests cover satisfiable formulas with concrete models, nested binders, all four unsigned comparisons in both directions and at equality, signed arithmetic fallback, push/pop, previous model availability and resource-limited retry.

The finite-model validator tests the direction that matters for soundness. It generates function tables and nested formulas over known bit-vector domains, chooses assertions true in that concrete model, and evaluates each emitted assertion in the transported model. The generator includes Boolean function arguments, conditionals, shared widths, shadowed variables and multiple quantifiers. A test-only Go overlay adds SMT-LIB comments identifying the source function and width behind each generated symbol. Quantified order axioms can be reused when both their text and domain sizes recur. In a 500-seed reproduction it checked 6,054 emitted assertions over 2,883 inferred domains. All 50 sampled g3 proof attempts were inconclusive as expected, and native Z3 confirmed SAT in all 25 sampled source queries. Reversing the strict unsigned comparisons in a separate build made the validator flag seed 0. To repeat, run `python3 perf/ivy_raft/order_models/run.py --count 500 --prove-every 10` and optionally supply `--native /path/to/z3 --native-every 20`. [Generator and model evaluator](perf/ivy_raft/order_models/finite_model.py) · [Driver](perf/ivy_raft/order_models/run.py)

With 32 KiB left in the owner's arena budget, the auxiliary solver could fail during construction while the existing concrete solver could still decide the query. GoIvy previously propagated that speculative error as a panic. Commit `38db621ab` routes an unsuccessful attempt through the original solver and preserves the concrete result. The regression warms up the concrete solver, leaves 32 KiB available, and checks the same UNSAT verification condition through the normal bridge.

#### Counterexamples on deliberately broken protocols

I changed 14 individual Raft rules, including initial leadership, double voting, log validity, and the value chosen at commit. Each mutation ran with the original concrete widths and with every width set to one. Each backend received the same generated source and the same list of proof obligations. All **28 negative inputs** produced counterexamples with pure-Go g3, GoIvy using the native C++ backend, and Python Ivy using native Z3. The checks compare the action, property or guarantee, and PASS or FAIL in order for all 419 obligations. Each negative input failed between one and eight obligations. A control that exchanged a passed and a failed label, keeping both totals constant, was detected. [Input generator](perf/ivy_raft/mutations.py) · [Transcript comparator](perf/ivy_raft/compare_mutant.py)

The initial 240-second limit covered all 28 g3 runs, 23 native GoIvy runs and 21 Python runs. The twelve incomplete native-backed runs finished with seed 2 under the 600-second retry limit. I also repeated all 28 g3 inputs with the group-scoped kernel at `819119283`. The same named decisions reproduced. To regenerate the inputs, run `python3 perf/ivy_raft/mutations.py /path/to/examples/raft/raft_no_assume_test.ivy /tmp/raft-mutants` with a new output directory. Run each checker on a generated wrapper and compare its stdout and stderr with the reference using `compare_mutant.py`. [Initial runs and failed obligation names](perf/ivy_raft/order-mutated-raft.csv) · [Retries and batch results](perf/ivy_raft/order-mutated-raft-retries.csv)

#### What remains slower

At matched concrete search the Go engine still takes about two minutes and native about 33 seconds. Applying the same offline order translation to all 419 captured queries and launching a fresh process per query took 30.63 s total in g3 and 8.47 s in C++ Z3. That experiment includes startup and parsing and omits the Ivy frontend. It shows the benefit of the proof strategy separately from the remaining cost of Go solver operations. For an individual hard goal, check 274, the saved fresh-context benchmark took 15.65 s concretely and 1.77 s with the relaxation. For easy check 1 the times were 17.9 and 55.0 ms. Avoiding speculative setup on easy checks and reusing work between related obligations are high-value next experiments.

Run `go test ./api -run '^$' -bench '^BenchmarkRaftOrderAbstraction$' -benchtime=1x -cpu=1` from `pkg` for the two captured conditions. Each iteration creates a fresh context, loads the original concrete assertions, selects the incremental path, and checks the expected answer. For an end-to-end comparison, build both GoIvy backends with `perf/ivy_raft/build.py` and run `perf/ivy_raft/measure.py` with a Python reference run and the original concrete wrapper. [Benchmark](api/api_order_abstraction_benchmark_test.go) · [Captured inputs](api/test_vectors/raft_order)

#### Same input and same search in both SMT kernels

I saved the generated SMT-LIB for all 419 order proofs and replayed the same bytes through both simple solvers with macro discovery enabled. The control uses Ivy's own Z3 4.7.1 Python bindings. Both drivers start a fresh context per obligation and check for UNSAT. Each backend ran three times on each of cores 6 and 12 in opposite orders. All 18 recorded search counters matched **for each individual obligation in every run**, including 38,690 quantifier instantiations, 23,033 decisions, 2,363 conflicts and 66,398 propagations across the corpus. [Per-obligation counters and timings](perf/ivy_raft/order-matched-search.csv)

| Median across six processes | g3 | C++ Z3 |
|---|---:|---:|
| Total elapsed, including the driver | 11.54 s | 2.83 s |
| Create contexts and configure solvers | 0.043 s | 0.044 s |
| Parse and assert | 1.370 s | 0.238 s |
| Check the assertions | 9.930 s | 2.311 s |
| Release contexts and solvers | 0.157 s | 0.069 s |
| User instructions | 94.66 billion | 20.66 billion |

The check itself accounts for **7.62 s** of the **8.71 s** wall-time gap. Parsing and asserting accounts for another **1.13 s**. Individual medians need not sum to the full process time, which includes reading inputs and collecting statistics. The Go kernel retired **4.58×** as many instructions on this search. A Go CPU profile attributed 35.3% of samples inclusively to application rewriting and 9.0% directly to the inlined AST node lookup. This supports further work on the cost per rewrite and reference access. The optimized end-to-end comparison above uses an additional proof strategy in GoIvy. Supplying the same transformed problem to C++ changes the comparison substantially. [Individual crossover runs](perf/ivy_raft/order-matched-runs.csv)

Reproduce the corpus in one process per backend from `pkg`. Run `go test ./api -run '^$' -bench '^BenchmarkRaftOrderKernel$' -benchtime=1x -cpu=1` and run `python perf/ivy_raft/replay_order_kernel.py` with Ivy's Python environment active. The committed benchmarks check the answers and time loading and solving. The crossover drivers also read the statistics after each check. [Go benchmark](api/api_raft_kernel_benchmark_test.go) · [Native control](perf/ivy_raft/replay_order_kernel.py) · [Compressed SMT-LIB corpus](api/test_vectors/raft_order/kernel.smt2.gz)

#### Where the extra instructions go

The user-mode hardware counters below cover g3 at `c3f2cd110` and native Z3 replaying those same 419 queries. Each event group ran six times per backend, alternating order across CPUs 8 and 21. All 18 search counters agreed per obligation in all 36 runs. Branch events ran continuously. The kernel multiplexed the cache and TLB event groups at 83% and scaled their counts. [All counter readings](perf/ivy_raft/order-kernel-pmu.csv)

| Median events over the complete replay | g3 | C++ Z3 |
|---|---:|---:|
| Instructions | 92.67 billion | 20.72 billion |
| Branches | 19.30 billion | 4.21 billion |
| Mispredicted branches | 185 million | 107 million |
| L1 data loads | 42.86 billion | 10.09 billion |
| L1 data load misses | 846 million | 322 million |
| Instruction TLB load misses | 5.93 million | 0.51 million |

g3 executed **4.48× as many instructions**, **4.58× as many branches** and **4.25× as many L1 data loads**. Its branch miss rate was 0.96% versus 2.54% and its L1 data load miss rate was 1.97% versus 3.19%. The absolute instruction volume and load count make repeated traversal, dispatch and handle resolution better targets than predicting existing branches. These process-wide counters include the drivers and runtime. They cannot assign misses to a particular structure.

The clean GoIvy binaries were also rerun with the *original concrete* input and transcript check. Across eight runs per build, the `c3f2cd110` pin took 12.53 s and 92.41 billion instructions, the previous pin took 12.30 s and 94.23 billion, and C++ took 37.16 s and 215.52 billion. The machine ran other studies during this crossover. The instruction reduction reproduced end to end. Wall time for this particular Go optimization was more stable in the paired kernel experiment. [Production crossover](perf/ivy_raft/order-substitution-pinned.csv)

#### Direct pointers, AST pages and garbage collection

A hardware cycles profile sampled three complete replays per backend. The largest native userspace entries included AST interning at 3.13%, the C allocator's internal allocation routine at 2.79%, application rewriting at 2.02% and AST deletion at 1.74%. On the Go build with the free-variable collector change, runtime heap scanning had a 2.65% entry, application rewriting 2.14%, declaration identity 1.20% and AST argument access 1.16%. These percentages are relative to each backend's own cycle count. Inlining can attribute a lookup to its caller. [Top 30 self-cost samples per backend](perf/ivy_raft/order-kernel-cycle-profile.csv)

The following controls use the same 419 obligations at `c3f2cd110` and validate the search counters. The medians come from six rotated runs per AST representation and seven per garbage collector setting.

| Change | Median instructions per corpus | Consequence |
|---|---:|---|
| Local Go pointer to the application record during rewriting | 92.76 billion vs 92.71 billion | Extending the pointer lifetime to the rewrite frame raised the count to 93.79 billion. |
| Separate node headers and application records into groups of 2, 4, 8, 16 or 64 | 94.19–98.24 billion vs 92.68 billion | Each grouping cost instructions. Removing record padding alone gave 92.62 billion. |
| AST page size of 16, 32, 128, 256 or 512 slots | 92.59–92.72 billion vs 92.79 billion at 64 slots | Adjusting the page size moved instruction count by at most 0.23%. |
| Raise `GOGC` from 100 to 400 | 85.42 billion vs 92.62 billion | Peak RSS rose from 71,448 to 116,492 KiB. |
| Disable GC for the replay | 81.96 billion | Peak RSS reached 2,558,168 KiB. With a 512 MiB runtime limit, the replay used 82.41 billion instructions and 535,772 KiB peak RSS. |

Disabling collection leaves **3.96×** the native instruction count over the same search. These experiments favor reducing work at repeated AST access sites and shrinking allocations where profiles identify a specific owner. They offer little evidence for changing global AST page size or retaining pointers in every rewrite frame. [Pointer timings](perf/ivy_raft/raft-app-pointers.csv) · [Pointer patch](perf/ivy_raft/app-pointer-control.patch) · [Layout timings](perf/ivy_raft/ast-record-groups.csv) · [Four-record patch](perf/ivy_raft/ast-node-group-control.patch) · [Page timings](perf/ivy_raft/ast-record-pages.csv) · [GC settings](perf/ivy_raft/order-kernel-gc.csv)

#### Memory per quantifier matcher and per solver

An allocation profile of the 419 matched kernel checks attributed about 190 MB to constructing quantifier matchers. Each matcher embedded two 64-by-64 tables of path-tree pointers for finding matching parents and children. Together these used 96 KiB on amd64 before inserting a pattern. Each quantifier manager constructs an eager and a lazy matcher. The new layout allocates one hash row on its first insertion. Each cell has a stable address for scope rollback. [Implementation](smtcore/smt_mam_impl.go)

| Layout | Allocated bytes per 419 checks | Live heap with 64 idle solvers | Median time per 419 checks |
|---|---:|---:|---:|
| Embedded tables | 2,832 MB | 305.1 MB | 11.60 s |
| Allocate a whole table on its first insert | 2,668 MB | 255.0 MB | 11.47 s |
| Allocate eight rows at a time | 2,665 MB | 251.5 MB | 11.62 s |
| Allocate individual rows | **2,640 MB** | **251.4 MB** | **11.45 s** |

These are decimal MB. The retained layout saves **193 MB allocated** over the corpus and **53.7 MB live** after 64 independent solvers load obligation 1, push, check UNSAT and pop. The live difference is about **839 kB per solver**. The application budget reports the same 2.95 MB per idle solver for both layouts. The difference is visible on the Go heap. The timings use seven rotated runs per layout with all 18 search counters matched on every obligation. Allocation figures are medians of five runs and idle heap figures are medians of three fresh processes. The additional small allocations increased the corpus allocation count from 14.985 to 14.991 million. [Allocation samples](perf/ivy_raft/quantifier-matcher-allocations.csv) · [Idle heap samples](perf/ivy_raft/quantifier-matcher-owners.csv) · [Kernel timings](perf/ivy_raft/quantifier-matcher-raft.csv)

Reproduce the worker measurement with `go test ./api -run '^$' -bench '^BenchmarkRaftIdleSolverMemory$' -benchtime=1x -cpu=1`. It measures groups of 1, 8, 32 and 64 independent concrete solvers, and the corresponding groups using the auxiliary order proof. Heap readings follow full collections while all owners are still live. [Worker benchmark](api/api_raft_worker_memory_benchmark_test.go)

#### Numeric storage with 1,024 independent solvers

The first integer allocation previously reserved 1,024 records in each numeric slab. A solver owns several such slabs, most with only a few live values after the first Raft obligation. The default now starts the first page with 64 records, grows it to 1,024 when slot 65 is first used, and allocates subsequent pages at full size. The slot-to-page calculation stays the same at every size. Generation metadata remains at a stable address through promotion, recycling and trimming. Explicit page sizes still govern every page. [Slab implementation](util/numeral/mpz_slab.go) · [Growth and retry tests](util/numeral/mpz_slab_test.go)

| Median per independent owner | Previous first page | Short first page |
|---|---:|---:|
| Live heap after a concrete proof and pop | 3.927 MB | **3.423 MB** |
| Live heap while the successful order-proof batch is open | 4.844 MB | **4.172 MB** |
| Live heap after closing the batch | 3.592 MB | **3.088 MB** |
| Budget charged after closing the batch | 2.869 MB | **2.362 MB** |

At 1,024 owners the measured live-heap savings are **492 MiB** after closing the proof batches and **656 MiB** with the batches still open. Owners loaded the same obligation, performed the same checks and stayed reachable through each full GC. They ran sequentially on one CPU. Peak process RSS during the run stayed around 4–5 GiB in both layouts. Reproduce with `G3_RAFT_MEMORY_OWNERS=1024 GOMEMLIMIT=6GiB go test ./api -run '^$' -bench '^BenchmarkRaftIdleSolverMemory$' -benchtime=1x -cpu=1`. [Worker benchmark](api/api_raft_worker_memory_benchmark_test.go) · [64-owner samples](perf/ivy_raft/numeric-owners.csv) · [256- and 1,024-owner samples](perf/ivy_raft/numeric-large-owners.csv)

In nine paired runs, building an eight-cell slab fell from **7.73 to 2.27 µs** and from 102,560 to 18,592 allocated bytes. Reading eight cells in rotation took 21.85 ns per lookup in both layouts. Building 262,144 cells took 8.08 and 8.06 ms respectively. The 419 matching-search Raft checks retired 90.15 billion instructions in both layouts, with medians of 11.23 and 11.28 s. This change addresses retained memory and small-owner setup. [Numeric benchmark](util/numeral/mpz_slab_benchmark_test.go) · [Final layout samples](perf/ivy_raft/numeric-cells.csv) · [Matched Raft replay](perf/ivy_raft/numeric-raft.csv)

Two controls explain the page design. Using 64 records on *every* page raised dense slab creation from 8.06 to 10.40 ms. Giving the first page a different slot-index mapping avoided copying its records on promotion, but added eight instructions to the common eight-cell lookup even with a bit mask. Keeping the ordinary index and copying the first 64 records once preserved the read cost. [Fixed page sizes](perf/ivy_raft/numeric-page-sizes.csv) · [Addressing and promotion counters](perf/ivy_raft/numeric-page-addressing.csv)

#### Reusing a solver for one group of Ivy final conditions

A clean build of GoIvy at `6dabf6e73`, pinned to g3 `df807d946`, verified the original Raft spec in **11.28 s** against **37.73 s** for the same GoIvy source built with native Z3. Each backend ran twelve times across two pinned CPUs. Every run matched all 419 named obligations and the final `OK`. Median peak RSS was 78,454 KiB for g3 and 322,210 KiB for Z3. The native build used the ordinary concrete solver. [Original-input runs](perf/ivy_raft/order-configuration-pinned.csv)

The next experiment gave each incremental group of final conditions one private proof solver. For every eligible condition the verifier rebuilds the relaxation from the current concrete assertions, pushes a fresh solver frame and pops it after an UNSAT result. A successful proof can reuse kernel storage for the next condition. Any inconclusive check releases the private kernel before the concrete fallback. Leaving the group releases the kernel even when the concrete solver remains live.

| Original Raft, twelve runs per lifetime | Fresh for each attempt | Scoped to a group |
|---|---:|---:|
| Median whole-spec time | 13.20 s | **12.06 s** |
| User instructions | 89.91 billion | **82.80 billion** |
| Peak RSS | 79,264 KiB | 92,734 KiB |

The comparison alternated lifetime on two pinned CPUs. The machine also ran package tests during these measurements. A separate ten-run prototype crossover gave 11.46 s for a fresh solver, 10.72 s for popping the preceding frame on the next call and 10.47 s for popping on return. Immediate pop also gives the caller a simple lifetime guarantee. [Production crossover](perf/ivy_raft/order-batch-raft.csv) · [Prototype crossover](perf/ivy_raft/order-batch-lifetimes.csv)

Across 64 independent concrete solvers, keeping all batches active after the first proof took **4.844 MB of live Go heap per solver**. Closing every batch while leaving every concrete solver live reduced this to **3.592 MB per solver**, the same as using one fresh proof solver per call. The concrete owners then had the same 2.869 MB of budget charges per solver. Five fresh-process observations per lifetime also cover groups of 1, 8 and 32. [Worker samples](perf/ivy_raft/order-batch-memory.csv) · [Benchmark](api/api_raft_worker_memory_benchmark_test.go)

At GoIvy `9d6a47b95` with the numeric-page change, twelve runs of each ordinary release binary on CPUs 6 and 21 gave **11.13 s, 83.20 billion instructions and 91,440 KiB peak RSS** for g3, versus **40.23 s, 215.54 billion and 321,580 KiB** for the native backend. Each process checked the same original mixed-width input and produced the Python reference transcript. The elapsed times include other concurrent studies on the host. [Raw comparison](perf/ivy_raft/numeric-clean-raft.csv)

#### Reusing exact premises across related goals

GoIvy often asks whether the same assumptions imply a different conclusion. The private g3 proof solver now keeps one solver scope containing the common assumptions. Each attempt checks the complete current query and pops its suffix. Sharing requires an exact sequence of complete assertions and compatible declarations for every symbol. A changed signature starts a new kernel. A changed premise pops the previous one. An inconclusive attempt releases the private state before GoIvy checks the original concrete assertions. On the original input 414 of 419 attempts belonged to batches. There were 203 transitions within a live kernel and 201 of them reused premises. [Solver and parser scopes](api/api_order_batch.go) · [Regression tests](api/api_order_batch_test.go)

Across 22 uninstrumented processes per backend, median g3 time went from **10.56 to 8.49 s**, instructions from **82.54 to 64.79 billion** and peak RSS from **90,988 to 80,056 KiB**. Native Z3 took **35.93 s, 215.53 billion instructions and 320,592 KiB**. All 66 checker runs reproduced the same 419 named decisions. The two Go builds used the same GoIvy checkout with local module replacements. The comparison uses the ordinary generated symbol names. An isolated control reasserting every premise took 83.58 billion instructions. Encoding signatures into names made performance highly sensitive to the spelling. [Whole-checker crossover](perf/ivy_raft/order-premise-release.csv) · [Declaration and scope controls](perf/ivy_raft/order-premise-controls.csv)

The saved GoIvy formula corpus differs from the earlier Python-exported fixture in 381 of 419 texts. Flattening nested AND and OR nodes makes all 419 syntax trees equal. Replaying the exact GoIvy bytes through both simple kernels with fresh contexts gave median times of **11.35 s for Go and 2.99 s for C++**. Decisions, conflicts, propagations, quantifier instantiations and resource counts agreed on every check. Replaying the actual GoIvy scope sequence gave **8.59 s for Go and 2.95 s for C++**. Four search counters agreed together on 398 of the 419 scoped checks. The native driver uses Python to bind declarations to their original sorts and to add the parsed formulas. Those operations count toward its load time. Run `go test ./api -run '^$' -bench '^BenchmarkRaftOrderScopes$' -benchtime=1x -cpu=1` and, in Ivy’s Python environment, `python perf/ivy_raft/replay_order_scopes.py` with and without `--fresh`. [Go benchmark](api/api_raft_order_scopes_benchmark_test.go) · [Native driver](perf/ivy_raft/replay_order_scopes.py) · [18 samples per configuration](perf/ivy_raft/order-premise-replay.csv)

At 1,024 live independent solvers, keeping the parser and premises cost **83,096 additional bytes per active solver** on the captured hard obligation. After closing the batches, both versions held **3,087,932 bytes per concrete owner**. Each of the 28 deliberately broken Raft inputs produced the same 419 ordered decisions, including the expected failures. Separately, 40,000 constructed finite models satisfied all 484,151 generated assertions. On 8,000 of those inputs g3 first proved a deliberately contradictory scope, popped it and correctly declined to prove the satisfiable original. Native Z3 confirmed SAT on all 400 sampled originals. Reproduce this last run with `python3 perf/ivy_raft/order_models/run.py --scope-control --start 2150000 --count 40000 --prove-every 5 --native ../build/z3 --native-every 100`. [Worker memory](perf/ivy_raft/order-premise-memory.csv) · [Mutated protocols](perf/ivy_raft/order-premise-negatives.csv) · [Finite-model check](perf/ivy_raft/order-premise-models.csv)

#### Alternatives tested on the original obligations

* Holding the auxiliary solver until the next proof reduced whole-spec median time from **11.11 to 9.70 s** over eight runs per lifetime. Median peak RSS rose from 81,934 to 94,032 KiB. Across 64 idle owners, the retained auxiliary contexts added **108.8 MB live after GC**. Popping the last frame immediately still added 107.0 MB. All four lifetime variants returned near their initial live heap after closing every owner. This early experiment kept the auxiliary solver on the persistent concrete owner. The group-scoped measurement above bounds the lifetime to the caller’s sequence of final conditions. [Whole-spec samples](perf/ivy_raft/order-reuse.csv) · [Independent owner groups](perf/ivy_raft/order-idle-owners.csv)

* A prototype constructed the translated terms directly with typed AST factories. It verified all 419 obligations in 18.53 s and retired 160.53 billion instructions. The integrated SMT-LIB path took 11.46 s and retired 94.25 billion. Direct construction changed the order of AST creation and solver setup. Eliminating parsing alone does not predict the cost of the resulting search.
* Forcing the same simple kernel on the *original* bit-vector checks reached 250 passes in 270.08 s before the experiment's timeout. The ordinary concrete solver completed all 419 in 124.71 s. The kernel selection is specific to the independent order proof.

* A bit-parallel evaluator agreed with the existing auxiliary SMT results on 42,579 eligible queries out of 44,546. Using its witnesses reduced one full run to 102.87 s from roughly 123 s. Emitting tautologies alone gave 120.24 s and combining both choices gave 115.82 s. Earlier attempts to skip auxiliary checks changed search order enough to regress badly. The evaluator remains an experiment for finite problems where the order relaxation is inconclusive.
* Rechecking the previous exact UNSAT core across nearby obligations discharged 171 of 419. Those checks represented only 3.71 seconds of original solver time. Most expensive action transitions needed different premises.
* Splitting four shared Boolean conditional guards reduced one hard check from roughly 41 seconds to 8.35 seconds across six branches. Results varied widely by obligation. Partial binder expansion and relevance filtering also helped individual goals. These candidates need inexpensive selection criteria and concrete fallback before they merit a default path.

#### Sensitivity to concrete domain size

Each row changes only the eight bit-vector interpretations in a copy of the original Ivy wrapper. Both backends check the same source for each row. Each completed run matched the 419 named obligations and final `OK`. These are single process observations on separate pinned cores. A timeout records the elapsed limit and number of obligations completed. [Input generator](perf/ivy_raft/widths.py) · [Timings, limits and memory](perf/ivy_raft/order-widths.csv)

| Input | g3 with order proof | Native concrete Z3 |
|---|---:|---:|
| All widths 1 | 11.60 s | **1.96 s** |
| All widths 2 | 11.19 s | 105.60 s |
| All widths 3 | 11.14 s | 180 s limit, 133 passed |
| All widths 4 | 11.15 s | 180 s limit, 250 passed |
| Mixed widths 2, 3 and 4 | 12.01 s | 28.77 s |
| All widths 8 | 12.01 s | 900 s limit, 274 passed |
| All widths 16 | 12.40 s | 107.47 s |

Finite domain size changes the native search enough to reverse the winner. Encoding one-bit domains as Booleans in the auxiliary g3 proof took 10.17 s. Preserving concrete two-bit domains in the auxiliary reduced the original mixed-width Raft input to 10.81 s, but raised the all-two-bit input to 222.98 s. Selecting exact three-bit domains raised the original input to 100.13 s. These results favor retaining the general order encoding for the original benchmark. A fast one-bit concrete path remains an open opportunity. [Partial-width controls](perf/ivy_raft/order-partial-widths.csv)

#### Compact expressions for the order translation

The temporary expression used to translate a quantified assertion occupied 80 bytes on amd64. It included an operator string, two integers and two slice headers. The new 20-byte record identifies its operator and sort by number. An application indexes one contiguous array of children. A quantifier uses the same two fields for its bound declarations and stores its single body in the value field. Every child, binder and sort belongs to one inference. The existing expanded-tree work limit bounds their indices. [Expression layout](api/internal/order/relax.go) · [Construction and binder lookup](api/internal/order/terms.go)

| Translating obligation 274 | Time per translation | Allocated bytes | Allocations |
|---|---:|---:|---:|
| Previous 80-byte record | 636 µs | 636,187 | 2,148 |
| 28-byte record with separate child and binder spans | 448 µs | 353,627 | 949 |
| 20-byte record with one shared span | 472 µs | **274,933** | **947** |
| 16-byte record with tagged operator and value | **419 µs** | 286,830 | 948 |

The 20-byte layout cut allocated bytes by **57%** on obligation 274 and **43%** on obligation 1. Whole-spec medians over eight rotated runs were 64.78 billion instructions for the original, **64.44 billion** for the 20-byte record and 64.45 billion for the tagged record. Elapsed times were 10.07, 10.01 and 10.17 seconds on the shared host. The translator accounts for a small fraction of the complete proof. The 20-byte layout keeps the operator separate from its operand, uses the least memory on the larger case, and produces byte-identical SMT-LIB on all 419 actual GoIvy queries. This record is private to translation. The SMT solver uses a separate 16-byte Boolean variable record. [Per-obligation samples](perf/ivy_raft/order-expression-micro.csv) · [Whole-spec samples](perf/ivy_raft/order-expression-raft.csv)

Run `go test ./api -run '^$' -bench '^BenchmarkRaftOrderInference$' -benchtime=1200x -cpu=1` to compare the two captured obligations. The benchmark times translation after parsing and checks every generated problem against the initial result. [Benchmark](api/api_raft_order_inference_benchmark_test.go)

#### Finishing SAT watch-list propagation

At the end of Boolean propagation, g3 had already validated the watch list and its backing array. A second lookup repeated that validation before truncating the list, even when every watch survived. The solver now uses the validated record and returns immediately when the length is already correct. Moving watches still clears the retired suffix, and an extension that replaces the backing array still invalidates the iterator. [Propagation](sat/sat_solver_propagation.go) · [Watch storage](sat/sat_solver_watch_store.go)

On a SAT replay of the 419 all-one-bit Raft obligations, the median over 17 alternating runs fell from **56.44 to 54.85 billion instructions**. Time spent in SAT solving fell from **4.93 to 4.78 s**. Whole-process wall times were 8.30 and 8.31 s on the shared host. Both builds also returned SAT for all 2,500 generated witnesses. These are measurements of the general SAT backend on the one-bit encoding. The ordinary mixed-width Raft checker uses the SMT order proof.

On the eight-hole pigeonhole contradiction, 34 samples per build on two pinned cores gave **56.09 to 56.05 billion instructions**. Native Z3 retired **8.05 billion**, with the same 31,417 decisions, 25,200 conflicts and 17 recorded search counters. This leaves a large implementation-cost difference in general SAT. Reusing the record earlier at the *start* of propagation helped one-bit Raft but increased instructions on pigeonhole 8, so that change was discarded. Three planted SAT instances also exercised the frontend. Run `python3 perf/ivy_raft/propagation_dimacs.py --rounds 5 -- /path/to/g3 ../build/z3`. [DIMACS generator](perf/ivy_raft/propagation_dimacs.py) · [Raw runs and counters](perf/ivy_raft/sat-watch-completion.csv)

#### A checked propositional replay for the all-one-bit input

Changing all eight interpreted sorts to one bit reverses the original comparison. Both values can then be enumerated for every quantified variable. The experimental translator gives each uninterpreted function its complete Boolean truth table, builds shared AND and ITE gates and emits the implications needed by each asserted polarity. Theory-only top-level conjuncts can still be dropped. A refutation proves the source obligation. A satisfying assignment to this relaxation requires the ordinary concrete checker to supply a source result or model.

The saved benchmark sends exactly the same short clauses to g3's general SAT solver or to a small bounded conflict-driven solver. A separate reverse unit propagation checker requires a justification for each learned clause, root unit and final conflict in the residual CNF. Sixteen unit-propagation sweeps and variable renumbering precede that certificate. Independent exhaustive truth-table tests cover the sweeps as well as the residual solver. The prototype is loaded through a Go test overlay. [Replay driver](perf/ivy_raft/onebit/run.py) · [Certificate checker](perf/ivy_raft/onebit/rup.go.txt) · [Source obligations](api/test_vectors/raft_order/onebit.jsonl.gz)

| One-bit measurement | Result |
|---|---:|
| Obligations proved by the unit sweeps alone | 33 of 419 |
| CDCL conflicts across the remaining obligations, positive phase | 19,384 |
| Median residual CNF for one obligation | 1,509 variables, 2,829 clauses |
| Largest search on an obligation | 511,833 propagation steps, 903 conflicts |
| Full GoIvy check with the initial checked prototype | 3.46 s elapsed, 2.14 s user CPU |
| Native Z3 on the identical one-bit Ivy source | 3.72 s elapsed, 2.33 s user CPU |

The last two rows are medians of 20 rotated runs per backend across two shared cores. The checked prototype retired 16.70 billion instructions versus 13.52 billion for native. In a separate 20-run crossover, deferring gate emission until all required polarities were known and storing the inputs as three signed 32-bit literals reduced g3 from 16.71 to **15.44 billion instructions**, 2.18 to **2.03 s** user CPU. These measurements concern the all-one-bit variant. On the original mixed-width spec this experimental path is inapplicable. [Checked checker crossover](perf/ivy_raft/onebit-checker.csv) · [Gate-storage crossover](perf/ivy_raft/onebit-gate-storage.csv) · [Per-obligation decisions and residual sizes](perf/ivy_raft/onebit-search.csv)

Fourteen deliberately incorrect one-bit protocols reproduced every expected failure and all other decisions. On each failing obligation the auxiliary proof was inconclusive and the concrete checker supplied the result. Fourteen mixed-width companions also matched. The saved replay driver agreed with the original prototype on all 419 per-obligation variable counts, clause counts, steps, decisions and conflicts. Run `python3 perf/ivy_raft/onebit/run.py --truth-tables --models 2500 --rounds 3` for a CSV covering the Raft corpus and 2,500 generated satisfying formulas and contradictory companions. The truth-table control enumerates up to ten variables per CNF and checks three decision policies on 70,000 inputs. [Broken-spec outcomes](perf/ivy_raft/onebit-mutants.csv) · [Exhaustive controls](perf/ivy_raft/onebit/truth_test.go.txt)

#### What an unused rewriter retains

Across 419 matched kernel checks, 410,069 of 495,079 activity caches closed without ever storing an entry (82.8%). Another 19,251 held at most one entry. The cache now creates its hash table, eviction queue and deletion bookkeeping together at the first valid insertion. The constructor captures the eviction limit and reserves the logical header up front. A populated cache can still reuse its storage across resets. [Cache implementation](ast/ast_activity_cache.go)

| Measurement | Original | First-insertion storage |
|---|---:|---:|
| Construct and close one empty traversal | 1,934 ns, 1,848 B | 1,639 ns, 1,320 B |
| Construct, clean up and close one traversal | 3,167 ns, 2,544 B | 2,507 ns, 1,488 B |
| Live heap per solver with its proof batch open | 4,255,240 B | 4,097,192 B |
| Live heap per solver after closing its proof batch | 3,087,932 B | 2,937,452 B |

The first two rows are medians of 21 rotated samples with 200,000 lifetimes each. The heap rows are medians of three independent groups of 1,024 live solvers. Each solver checked the same obligation on one goroutine. The first-insertion layout saves **147 MiB across 1,024 idle solvers**. The CSV also includes a separately varied change to the two variable shifters. Run `go test ./ast/rewriter -run '^$' -bench '^BenchmarkTraversalLifetime$' -cpu=1` and `G3_RAFT_MEMORY_OWNERS=1024 go test ./api -run '^$' -bench '^BenchmarkRaftIdleSolverMemory$/^OrderBatch$/^1024$' -benchtime=1x -cpu=1`. [Traversal benchmark](ast/rewriter/var_shifter_benchmark_test.go) · [Lifetime samples](perf/ivy_raft/rewriter-owner-micro.csv) · [Heap measurements](perf/ivy_raft/rewriter-owner-memory.csv)

A separate integration check built GoIvy `594f703d0` using its local g3 module replacement at `b614b914a`. Each backend produced all 419 named results in 16 runs on two shared cores. Median user CPU was **10.18 s for g3 and 46.74 s for native Z3**. Median elapsed times were 15.06 and 67.05 s under competing load. Retired instructions were **64.15 and 215.53 billion**. The same g3 checkout reproduced all 27,855 normalized output lines of the Python Ivy Paxos control, including 27,782 trace lines and the diagnostic for the unquantified variable. [Per-run results and revisions](perf/ivy_raft/local-frontier-raft.csv)

#### Constructing variable shifters on first use

The matched kernel created 126,452 rewriter engines. Of their two embedded shifters, 1,479 forward and 173 inverse shifters received a shift request. A frequently used shifter still needs its full traversal stack. The engine now constructs both root caches at their original point and activates each traversal on its first shift or explicit cleanup. This preserves its initial eviction limit, allocation traces and failure points. The enclosing owner reserves capacity for both traversals, including when embedded in a variable substituter. No shared pool participates. [Shifter implementation](ast/rewriter/var_shifter.go) · [Constructor and failure tests](ast/rewriter/var_substitution_test.go)

Across five groups of 1,024 solvers, the shifter change saved a further **190 MiB idle** and **200 MiB with proof batches open**. The application budget reported exactly 2,355,389 bytes per idle solver on both builds. Taken together with the first-insertion cache change, measured idle Go heap fell from **3,087,932 to 2,742,412 bytes per solver**, or **337 MiB saved across the group**. The isolated empty traversal fell from 1,320 B and seven allocations with first-insertion caches to 632 B and five allocations with both changes. Explicit cleanup activates the traversals, increasing the corresponding count from 13 to 15 allocations. Across 17 complete proofs per layout the original, first-insertion, shifter-only and combined versions retired 64.43, 64.31, 64.11 and 63.97 billion instructions. All 419 results matched on every run. [Release heap comparison](perf/ivy_raft/rewriter-shifter-memory.csv) · [Four-way Raft comparison](perf/ivy_raft/rewriter-owner-raft.csv)

#### Passing a resolved Boolean operator to the rewriter

The theory rewriter knows both the family and the operator before it calls the Boolean rewriter. Passing the operator to the existing reduction switch saves another read of the declaration. The public Boolean entry point continues to resolve the operator from its input declaration and run the debug family check. [Theory dispatch](ast/rewriter/th_rewriter.go) · [Boolean reduction](ast/rewriter/rewriter_configuration.go)

Over 13 rotated runs of the 419-obligation matched kernel, the retained variant (`minimal` in the CSV) reduced median instructions from **89.31 to 88.66 billion** and user CPU from 13.61 to 13.07 s. All 18 integer search counters matched on each obligation. Over 17 samples of the public theory rewriter, median times for negation, conjunction and equality went from 395 to 359, 505 to 475 and 453 to 380 ns respectively. The uninterpreted-constant control took 509 and 506 ns. Adding a public combined family/kind accessor saved a further 0.09 billion instructions in this run. The retained change uses the existing AST interface. Run `go test ./api -run '^$' -bench '^BenchmarkRaftBooleanRewrite$' -benchtime=300000x -cpu=1`. [Benchmark](api/api_raft_boolean_rewrite_benchmark_test.go) · [Kernel samples](perf/ivy_raft/boolean-dispatch-kernel.csv) · [Public API samples](perf/ivy_raft/boolean-dispatch-micro.csv)

#### Compilation profiles for the current checker

The saved Go 1.27.3 compilation profile combines five mixed-width checker runs with premise reuse, five all-one-bit prototype runs and one earlier end-to-end Raft run using the direct order kernel. The GoIvy entry point appears in all three sets of CPU samples. Six compiler configurations were rotated across two cores, with 18 complete runs of each configuration on each input. Both SHT isolates and both TLS isolates were held out of training. All runs matched their corresponding native transcripts. [Training profile](perf/ivy_raft/raft-blended.pprof) · [Per-run results](perf/ivy_raft/pgo-checker-crossover.csv)

| Input | Instructions with PGO off | With blended PGO | Median user CPU, off → blended |
|---|---:|---:|---:|
| Original Raft | 64.19 billion | 59.28 billion | 10.64 → 9.83 s |
| All-one-bit Raft, ordinary checker | 64.25 billion | 59.31 billion | 10.36 → 10.39 s |
| Sharded hash table, process isolate | 9.68 billion | 9.07 billion | 1.66 → 1.63 s |
| Sharded hash table, transfer isolate | 10.74 billion | 9.98 billion | 1.82 → 1.66 s |
| TLS, stream-data isolate | 0.98 billion | 0.96 billion | 0.16 → 0.17 s |
| TLS, stream-position isolate | 1.23 billion | 1.19 billion | 0.23 → 0.18 s |

On the original Raft workload the blended profile removed **7.6% of instructions**. User CPU improved on four of the six inputs. The two shortest runs took fractions of a second, and timing varied with other work on the host. These results support testing the profile with the intended Go version and target machine. With a local g3 module replacement, run the following in the GoIvy module, setting `g3pkg` to the local g3/pkg directory.

```sh
go build -mod=readonly -tags=xtrace_off -pgo="$g3pkg/perf/ivy_raft/raft-blended.pprof" -o /tmp/goivy-pgo ./cmd/goivy_check
go build -mod=readonly -tags=xtrace_off -pgo=off -o /tmp/goivy-no-pgo ./cmd/goivy_check
```

#### Watch and proof storage in the one-bit experiment

The checked one-bit prototype now stores original three-literal clauses inline. A watch holds a 32-bit clause index and a 32-bit blocking literal. The first watch lists use adjacent, capacity-limited spans, so a later append to a full list gets its own storage. The proof checker indexes immutable input clauses directly and keeps learned clauses separately. This preserves input clause numbers through tautologies and gives the proof checker an independent view of the formula.

Across 20 rotated runs per layout, the combined changes reduced the complete one-bit check from **15.44 to 14.40 billion instructions** and median user CPU from 1.98 to 1.95 s. In the standalone replay, instructions fell from 22.92 to 21.90 billion and median user CPU moved from 3.11 to 3.13 s. The existing general SAT solver control stayed at 24.51 billion instructions on the generated satisfiable problems. The three-round release replay reproduced all 419 original decision, conflict and step counts and returned the expected answers for each of 2,500 finite models and their contradictory companions. The command was `python3 perf/ivy_raft/onebit/run.py --truth-tables --models 2500 --start 2850000 --rounds 3`. [Eight storage layouts](perf/ivy_raft/onebit-watch-storage.csv) · [Search implementation](perf/ivy_raft/onebit/search.go.txt) · [Proof checker](perf/ivy_raft/onebit/rup.go.txt)

#### Quantified binder and SMT-LIB output storage

Order inference now reuses a single binder stack while reading sibling quantifiers. It restores the previous stack length immediately after reading a body, including on errors. The renderer reserves eight bytes per term as an initial estimate and writes known punctuation, sort names and indices directly to the output builder. All storage belongs to one translation. All 419 resulting SMT-LIB files matched the original output byte for byte.

On obligation 274 the committed variant reduced median translation time from **377 to 321 µs**, heap allocation from **274,933 to 224,241 B**, and allocations from **947 to 698**. These are medians of 13 runs of 1,200 translations. In 13 complete Raft runs per variant the instruction count moved from 64.44 to 64.21 billion and user CPU from 9.82 to 9.97 s. Reusing an additional 64 KiB across checks brought the hard translation down to 271 µs and 78,135 B, with essentially the same whole-check instruction count as the committed variant. Run `go test ./api -run '^$' -bench '^BenchmarkRaftOrderInference$' -benchtime=1200x -cpu=1`. [Inference benchmark](api/api_raft_order_inference_benchmark_test.go) · [Translation measurements](perf/ivy_raft/order-translation-micro.csv) · [Complete checks](perf/ivy_raft/order-translation-raft.csv)

#### Releasing references owned by the rewriter cache

Every published cache cell owns its key and its value. Its release code now calls the AST manager's existing `DecRefHeldConsumed` operation. A surviving node needs one refcount decrement. Deleting the final reference still goes through the checked destructor path, which reports whether a failing deletion consumed the reference. The cache uses that result to track interrupted cleanup.

In 16 rotations the 419 matched kernel checks went from **88.90 to 88.52 billion instructions** and 14.76 to 14.47 s of user CPU. In 13 complete Raft runs per variant the median went from 63.73 to 63.41 billion instructions and 11.53 to 11.39 s of user CPU. A separate 13-round comparison with Boolean dispatch enabled timed 100,000 reset-and-refill cycles of 32 entries each. Instructions fell from **4.11 to 3.37 billion**, and cycle time from **4,531 to 4,276 ns**. Three million read-only finds took 0.66 billion instructions on either build. Run `go test ./ast -run '^$' -bench '^BenchmarkActivityCacheCycle$' -cpu=1`. [Cache benchmark](ast/ast_activity_cache_benchmark_test.go) · [Two-layout release comparison](perf/ivy_raft/rewriter-cache-release.csv) · [Standalone variants](perf/ivy_raft/rewriter-cache-micro.csv) · [Matched kernel](perf/ivy_raft/rewriter-cache-kernel.csv) · [Complete Raft checks](perf/ivy_raft/rewriter-cache-raft.csv)

Clearing a reference whose target is already empty used to enter the general AST assignment path. The release build now validates the destination and returns immediately when it owns no node. A manager with pending deletions still performs its usual cleanup. The assignment comparison exercises an empty live reference, a stale reference and a malformed source handle on both paths. [Assignment](ast/ast_ref.go) · [Comparison test](ast/ast_ref_lockfree_test.go)

With the resolved Boolean dispatch in both builds, empty-reference assignment reduced the median over ten matched kernel runs from **87.75 to 87.30 billion instructions**. Seven complete Raft runs per build went from 63.08 to 62.79 billion. Each of 600,000 public negation and equality rewrites saved about 108 instructions, with zero allocations in both builds. [Matched kernel](perf/ivy_raft/rewriter-null-reference-kernel.csv) · [Whole Raft](perf/ivy_raft/rewriter-null-reference-raft.csv) · [Public rewriter benchmark](perf/ivy_raft/rewriter-null-reference-micro.csv)

#### Memory for interrupted rational parameter replacement

Each parameter block previously carried a full continuation for overwriting an owning rational. Scalar-only blocks never used it. The context now allocates the continuation on the first actual replacement and reuses it after clearing the previous generation's fields. The budget reserves the original logical capacity at block allocation, preserving the failure points exercised by the destructor and recovery tests.

Across three groups of 1,024 independent Raft solvers, median live heap per idle solver fell from **2,742,412 to 2,679,564 bytes**. That saves **61.4 MiB across 1,024 solvers**. Active heap fell from 3,892,584 to 3,828,008 bytes. Idle budget charges stayed at 2,355,389 bytes per solver. A competing implementation discarded the frame on each generation and added 32 heap allocations per 32-block numeral replacement cycle. The retained version performed the same cycle with zero allocations after warmup, taking 31.7 µs versus 31.3 µs originally. Scalar-only cycles took 9.23 versus 9.28 µs. Run `go test ./util/params -run '^$' -bench '^BenchmarkParamsReplacementBlocks$' -cpu=1`. [Parameter benchmark](util/params/util_params_replacement_benchmark_test.go) · [Ownership samples](perf/ivy_raft/parameter-replacement-owners.csv) · [Scalar and numeral samples](perf/ivy_raft/parameter-replacement-micro.csv)

#### Which AST lookups paid off on complete proofs

Compiling frequently reused quantifier-free substitution bodies into traversal instructions improved the isolated substitution benchmark. A four-entry cache lowered median time for retained instances from **5,508 to 5,240 ns** and for retired instances from **7,822 to 7,482 ns** across 19 rotations. On 15 runs of the matched kernel, median instructions increased from 88.16 to 88.25 billion. On the complete Raft checker they increased from 64.36 to 64.46 billion. The additional plan storage and lookup did not earn a place in the solver. The saved patch and benchmark allow another comparison on workloads with more reuse. [Prototype against b614b914a](perf/ivy_raft/substitution-plan.patch) · [Substitution benchmark](smtcore/smt_quantifier_substitution_benchmark_test.go) · [Substitution samples](perf/ivy_raft/substitution-plan-micro.csv) · [Matched kernel](perf/ivy_raft/substitution-plan-kernel.csv) · [Complete checks](perf/ivy_raft/substitution-plan-raft.csv)

A separate experiment cached the ITE, AND or OR operator in spare space in a rewriter frame. Reading it alongside the first argument gave the lowest matched instruction count, 87.86 versus 87.99 billion over 14 runs. Median CPU was 12.04 versus 11.61 s. Twelve full Raft proofs per build retired 63.56 versus 63.60 billion instructions. Reading the operator at node visitation or at the first folding attempt also failed to produce a consistent whole-check improvement. The ordinary rewriter keeps its original frame. [First-argument prototype against 0e5619ed2](perf/ivy_raft/rewrite-fold-kind.patch) · [Matched searches](perf/ivy_raft/rewrite-fold-kind-kernel.csv) · [Whole-spec runs](perf/ivy_raft/rewrite-fold-kind-raft.csv)

#### Sensitivity to search order and proof budgets

A controlled change to the order solver's seed or to the spelling of a generated function name can change its search. With 48 seeds, starting a fresh frame finished all 48 inputs inside the trial timeout and the common-prefix policy finished 47. Each policy finished 47 of 48 spelling variants. Every completed check agreed with the reference on all 419 obligations. [Perturbation runs including timeouts](perf/ivy_raft/proof-search-perturbations.csv)

An early limit followed by one retry within the original overall budget gave mixed results. Across 40 seeds, an 800,000-step framed first attempt improved median user CPU from 11.81 to 11.00 s. Across 40 equivalent spellings it increased CPU from 12.94 to 16.39 s. All 400 runs, covering five policies, completed with the expected decisions. The evidence favors keeping the existing single budget as the default. [Complete retry results](perf/ivy_raft/proof-retry-policies.csv)

#### One-bit proofs with g3's standard SAT solver

A retained formula whose bit-vector sorts all have width one can be evaluated over both values. Each uninterpreted function gets a complete Boolean truth table shared by every occurrence, including occurrences under different quantifiers. Quantifiers enumerate the assignments of their own binders. The compiler emits clauses for the polarities actually used and propagates units before starting the existing g3 SAT solver. The order abstraction may discard conjuncts, so a SAT answer remains inconclusive. Expansion, clause loading and search all poll the source cancellation signal. Arity, size and work limits bound expansion. The larger-width case keeps its order proof. [Compiler](api/internal/order/boolean_problem.go) · [Unit propagation](api/internal/order/boolean_units.go) · [Solver ownership and proof limits](api/api_order_boolean.go)

Successive Raft goals share clauses even after each problem renumbers its residual variables by first occurrence. The batch compares the actual signed clauses before reusing a prefix. It guards the new suffix with a fresh selector, checks under the opposite assumption, then permanently disables that suffix. A changed prefix, a full batch or an inconclusive attempt releases the previous solver. The source solver keeps its own assertions and model. The auxiliary SAT solver disables six preprocessing passes on the unit-reduced input. In 22 standalone runs per mode on the 419 residual CNFs, creating a fresh solver took 33.75 billion instructions. Retaining a checked prefix and guarding the last 64 rows took 11.53 billion. Turning off redundant preprocessing reduced the whole one-bit verifier from 21.82 to about 17.8 billion instructions. [Ten prefix policies](perf/ivy_raft/onebit-sat-prefixes.csv) · [Eleven preprocessing settings](perf/ivy_raft/onebit-sat-preprocessing.csv)

All 419 answers agreed on every complete Raft run. Across three replays, the independent checked CDCL solver reproduced each saved variable count, clause count, unit result, decision count, conflict count and step count using the production encoder. All 28 combinations of a broken protocol and domain width reproduced the reference's named successes and failures. Run the API benchmark with `go test ./api -run '^$' -bench '^BenchmarkRaftBooleanProofs$' -cpu=1 -benchtime=1x`. The model check can be repeated with `python3 perf/ivy_raft/onebit/batch/check.py --count 1000 --native ../build/z3`. It supplies an explicit model, pushes a four-clause contradiction that requires search, proves it twice and pops it before checking that the original model is still possible. [API benchmark](api/api_raft_boolean_benchmark_test.go) · [Scope and cancellation tests](api/api_order_boolean_test.go) · [Mutant outcomes](perf/ivy_raft/onebit-batch-mutants.csv) · [Witness generator and driver](perf/ivy_raft/onebit/batch/check.py)

#### One-bit checker timing, profiles and independent owners

A second crossover built the order-only control and the Boolean solver through GoIvy's local module replacement. The binaries were checked for the expected Go symbols. Across 24 complete one-bit checks per engine on two pinned CPUs, median wall time was **12.29 s for order-only g3, 2.95 s for g3 with Boolean proofs and 2.97 s for native Z3**. The corresponding instruction counts were 62.68, 17.86 and 13.52 billion. At mixed widths, eight runs per engine took 12.21, 11.97 and 59.94 s. All runs compared 419 named outcomes. The native one-bit build still retired about 24% fewer instructions than the Boolean g3 build. Sampling userspace cycles at 499 Hz put unit reduction and SAT propagation at 6.87% and 6.04% of one-bit g3 cycles respectively, taking the median share across nine profiles. [Complete paired checks](perf/ivy_raft/onebit-public-checker.csv) · [Cycle profiles for both engines and widths](perf/ivy_raft/onebit-public-profiles.csv)

On obligation 001, the live heap with 1,024 active one-bit proof batches fell from **3,883,736 to 2,877,536 bytes per solver**. On obligation 274 it fell from **12,261,207 to 4,393,309 bytes per solver** at 256 owners. The measurements use three independent processes per setting and a full collection before sampling. Across seeds 3,000,000 through 3,199,999, the finite-model driver checked one million generated assertions against explicit models. All 200,000 batches left the satisfiable input inconclusive, twice proved the pushed four-clause contradiction by SAT search, then left the original input inconclusive after pop. Native Z3 independently returned SAT and UNSAT on each of 382 sampled pairs. [Owner measurements](perf/ivy_raft/onebit-public-owners.csv) · [Model coverage](perf/ivy_raft/onebit-public-models.csv) · [Model replay](perf/ivy_raft/onebit/batch/check.py)

#### Keeping bit-vector cleanup state on demand

A bit-vector trailing rewriter had three embedded 88-byte call records. A polynomial configuration carried four 24-byte parameter decrement receipts. Sampling 17,638 rewriters at close during the matched kernel replay found every one of these fields empty. The full mixed-width verifier also had them empty at all 17,203 sampled closes. These counts measure state at close. The owner now allocates the three call records together on first use and saves a decrement receipt only when cleanup stops after consuming a reference. The original budget reservation and trace amounts still apply. A regression test interrupts a real parameter block's final decrement, restores its numeral service and retries cleanup. [Trailing calls](ast/rewriter/bv_trailing.go) · [Parameter snapshots](ast/rewriter/bv_polynomial_config.go) · [Cleanup retry](ast/rewriter/bv_polynomial_config_test.go)

Five fresh-process runs per layout with 1,024 independent owners reduced the live heap after closing each proof batch from **2,679,564 to 2,587,564 bytes per solver**. That saves **89.84 MiB across 1,024 solvers**. With each proof batch still active, the median fell from **3,828,008 to 3,733,064 bytes per solver**. Both versions reported 2,355,389 bytes of idle budget charges per owner. The earlier 3,087,932-byte baseline and the new measurement differ by **488.64 MiB per 1,024 idle owners**. On a fixed earlier revision, the five-layout comparison saved 64,000 bytes per idle solver from the grouped call records alone and 28,000 from the receipts alone. The 11 matched proof replays per layout had nearly identical instruction counts, 86.47 versus 86.46 billion. User CPU was 12.50 versus 13.63 seconds on the shared host. The retained gain here is memory. [Current owner crossover](perf/ivy_raft/bit-vector-rewriter-owners.csv) · [Five storage layouts](perf/ivy_raft/bit-vector-call-layouts.csv) · [Matched search controls](perf/ivy_raft/bit-vector-call-kernel.csv) · [Reproducible benchmark](api/api_raft_worker_memory_benchmark_test.go)

#### Selecting the watches of a restored ternary clause

Z3 scans twice to select its two watches. With three undefined literals, the first scan chooses the third and moves the original first literal to that position. The second scan then chooses the original first. g3 now recognizes these three assignments and makes the same rotation in one step. Other assignments retain the two scans and their scope-level tie breaks. A regression compares the resulting literal order against the two scans for all 27 ternary assignments. [Watch selection](smtcore/smt_clause_backtrack.go) · [Exhaustive assignment test](smtcore/smt_learned_assignment_restore_test.go)

Across 44 runs per build, alternating 10,000 rounds per sample on CPUs 13 and 23, the queued scoped pop fell from **32.16 to 30.84 µs**, against **17.53 µs** for Z3. The queued surviving pop took 588, 565 and 672 ns respectively. The original Go Makefile fixture without the activity queue went from 24.53 to 23.45 µs on the scoped case. Absolute times varied with background load. Two earlier comparisons of the scoped queue also favored the rotation, from 22.65 to 20.53 µs and from 28.69 to 25.86 µs. On the matched 419-query kernel replay, the median retired instruction count moved from 86.78 to 86.72 billion over 16 runs per build. This is primarily a backtracking gain. The Go queue benchmark's comparable metric is `pop-ns/op`. Its `ns/op` includes setup and outer-scope cleanup. [Paired Go and native measurements](perf/scope_restoration/ternary-watch-release.csv) · [Alternative watch selections](perf/scope_restoration/ternary-watch-controls.csv) · [Benchmark fixtures](smtcore/smt_hotpath_benchmark_test.go)

#### Directly refuting a universal instance

The verifier often asks for a ground instance of one of its own premises. For example, a premise can require `P(x, x)` for every `x`, while the violation requires `not P(h(a), h(a))`. g3 now finds opposite signed requirements and matches their complete terms in the type-checked order representation. Every occurrence of a binder must receive exactly the same ground term of the same inferred sort. The search descends through a quantifier only when its polarity requires the body at every assignment. Bindings live for one match attempt. The matcher polls the source cancellation signal and charges up to 32,768 visits against the proof budget. A successful match needs no auxiliary solver. The ordinary Boolean and order proofs remain available after a miss. [Matcher](api/internal/order/instance.go) · [Regression cases](api/api_order_instance_test.go)

This proves **234 of 419 obligations** directly at both original and one-bit widths through GoIvy's default solver. Restricting matches to predicate applications proved 52. Searching for correlated requirements throughout the input and decomposing Boolean equality raised coverage to 242. The simple solver presents the assertions in a different form and exposed 16 direct matches. On mixed-width obligation 001, median preparation cost fell from 91.7 to 71.6 µs and from 159 to 108 allocations. Obligation 274 still needs search and incurs the cost of an unsuccessful match. [Coverage across configurations](perf/ivy_raft/instances/coverage.csv) · [Preparation benchmark](api/api_raft_order_inference_benchmark_test.go) · [Preparation measurements](perf/ivy_raft/instances/preparation.csv)

In 14 rotated, complete checks per engine on CPU 7, with compiler PGO off, the original Raft input fell from **8.80 to 7.16 s** and from **62.79 to 49.48 billion instructions**. Native Z3 took **44.01 s** and 215.52 billion instructions. On one-bit Raft, elapsed time fell from **2.17 to 1.68 s** against **2.24 s** for Z3. The corresponding instruction counts were 17.85, 13.99 and 13.51 billion. All 419 named outcomes and the final result matched. All 28 corrupted-protocol runs also reproduced their named passes and failures. These are complete GoIvy timings on a shared machine. The same-search SMT kernel comparison above measures a different workload. [Whole-check measurements](perf/ivy_raft/instances/checker-cpu7.csv) · [Corrupted protocol results](perf/ivy_raft/instances/mutants.csv)

An independent run used 200,000 satisfiable inputs across seeds 3,800,000 through 3,999,999. One third used evaluated random finite models. The remaining inputs varied eleven patterns with explicit distinct constants, multiple sorts, repeated terms, polarity, and shadowed quantifiers. None yielded a false refutation. On all 133,333 inputs with a pushed contradictory instance, the new path proved it and stopped proving it after pop. Native Z3 checked 797 sampled inputs and the concrete g3 solver checked 98. Run `python3 perf/ivy_raft/instances/check.py --count 1000 --native ../build/z3`. [Witness driver](perf/ivy_raft/instances/check.py) · [Seed ranges and totals](perf/ivy_raft/instances/model-coverage.csv)

#### Allocating the scalar and addition numeral frames on first use

The scalar bit-vector rewriter and the polynomial addition bit reader each had an embedded numerical invocation. Both now allocate a frame on first use and keep its identity for retries and cleanup. A completed addition rewrite that skips the bit reader allocates no frame. An unused child still performs the same manager readiness and cleanup checks. The construction budget continues to reserve the full frame. [Numeral frame ownership](ast/rewriter/bv_numeral_call.go) · [Addition bit reader](ast/rewriter/bv_rewriter_add.go) · [Failure injection and retry](ast/rewriter/bv_rewriter_add_caller_numeric_test.go)

With 1,024 independent solvers on mixed-width obligation 001, median live heap after closing the batch fell from **2,587,547 to 2,547,547 bytes per solver**, saving **39.06 MiB per 1,024 solvers**. Both versions charged 2,355,373 logical budget bytes per solver. This obligation now finishes directly. On obligation 274, which keeps an auxiliary solver active, the median live heap at 64 owners went from 12,102,391 to 12,059,787 bytes per solver. The twelve matched 419-query replays per version reproduced all 18 counters on each obligation and took 86.67 versus 86.69 billion instructions. [Heap measurements](perf/ivy_raft/bit-vector-numeral-owners.csv) · [Matched searches](perf/ivy_raft/bit-vector-numeral-kernel.csv)

#### Addressing one-bit function interpretations by their first SAT variable

Each function's truth table occupies a consecutive range of SAT variables. The compiler now records the first variable and computes the offset for a known argument. A symbolic argument still emits the same if-then-else gate. The work limit charges for every table row as before. Across eight prototype layouts, all 419 raw clause sequences and all 419 reduced sequences were byte-identical. On a separate three-round replay, all 2,500 finite models and their contradictory variants at seeds 4,200,000–4,202,499 produced the expected answers. [One-bit replay](perf/ivy_raft/onebit/run.py)

On one-bit obligation 274, with direct instance matching enabled, median proof preparation fell from **1.317 to 1.248 ms** across 30 runs per implementation. Allocation fell from **282 to 214 allocations** and from 798,008 to 795,480 bytes per preparation. The checker experiment isolated the eight layouts before direct matching was introduced. On CPUs 20 and 4, the selected change removed 67 and 71 million instructions per full check, respectively, from a baseline of roughly 17.9 billion. Run `go test ./api -run '^$' -bench '^BenchmarkRaftProofPreparation$/OneBit/274' -benchtime=800x -cpu=1`. [Preparation samples](perf/ivy_raft/onebit-truth-table-preparation.csv) · [Full checker samples](perf/ivy_raft/onebit-truth-table-layouts.csv)
