# Raft verification and SMT scope restoration performance

The AST arena is part of g3's design. Current performance work keeps it and measures how often the solver traverses expressions, looks up Boolean state, and invokes callbacks during a proof.

**g3 now wins all three original GoIvy workloads at the default GC setting.** In four alternating pairs per workload on CPU 22, the complete run took 4.06 s versus 46.73 s on Raft, 83.70 s versus 100.72 s on Apple `dramc_nb2`, and 9.87 s versus 26.57 s on Apple `rfn2`. All 12 paired proof transcripts matched. Linked Z3 won all 42 pairs across seven independent short specifications. The two GoIvy configurations also responded differently to a larger heap on `dramc_nb2`. [Complete checker comparison](#ordinary-goivy-verification-at-91d1597f3) · [Remaining short-query costs](#three-reproducible-obligations-that-still-favor-z3) · [GC and resident memory](#the-effect-of-go-heap-growth-on-dramc_nb2)

**Ground Boolean proofs brought `cmpl_live` below linked Z3 in retired instructions.** Across the first three paired rounds the complete checker used 121.15 billion instructions against 124.13 billion for Z3. The same change took `cf_live` from 122.95 to 119.71 billion, against 107.68 billion for Z3. All 882 private refutations were confirmed by the original solver. [Measurements and controls](#ground-boolean-proofs-on-the-apple-liveness-checks) · [Earlier premise selection](#selecting-positive-conjuncts-for-ground-proofs)

**A four-operand application now fits entirely in the same 128-byte arena slot.** A repeated construction costs 1,265 instructions in g3 and 605 in Z3, down from 2,970 in g3. Whole-proof instruction counts improved in all 32 matched pairs. [Arena measurements](#four-application-operands-in-a-128-byte-arena-slot)

**A 256-entry rewriter cache now uses 16,592 live heap bytes per solver, down from 28,896.** All eight measurements across 1,024 independent managers reproduced the 12,304-byte saving. The same change reduced instructions in all 32 paired Raft and Apple proof runs. [Cache memory and proof measurements](#manager-local-references-in-the-rewriter-cache)

**Reusing validated application operands reduced instructions by 3.1% on complete Raft checks and 3.7–4.9% on three Apple isolates.** Constructing a repeated binary application fell from 156 to 84 ns, with native Z3 at 37 ns. [Constructor and proof measurements](#reusing-validated-operands-in-the-arena-backed-application-constructor)

**Compact saved clause atoms cut post-pop heap use by 4,760 bytes per solver across 1,024 solvers.** Releasing three saved atoms took about a third fewer instructions. In 32 timed pairs, scoped pop took 20.04 µs before, 19.18 µs after and 13.06 µs in native Z3. [Clause layout, timings and controls](#compact-saved-atoms-in-learned-clauses)

**Batching reference decrements reduced scoped relevance rollback from 25,200 to 19,357 instructions.** Native Z3 used 8,036. The complete scoped pop used 2.4% fewer instructions in the final nine-round comparison. [Reference retirement and proof measurements](#releasing-owned-references-during-relevance-rollback)

**Reusing a Boolean variable’s heap position cut a scoped pop from 21.81 to 19.21 µs.** Z3 took 13.46 µs in the same 36-pair comparison. The activity queue now uses 192 instructions per removal, down from 208. [Heap deletion and controls](#removing-a-boolean-variable-from-the-activity-heap)

**A byte per e-graph trail entry and shared storage for Boolean handles cut idle scope memory by 8.4% across 1,024 solvers.** Each solver saved 5,857 bytes after a 128-atom scope. With 2,048 atoms the saving was 78,824 bytes per solver. [Worker memory and pop measurements](#compact-e-graph-trails-across-independent-solvers)

**Preparing frequently checked quantifier bodies cut g3’s 16-equality queue check from 2,254 ns to 1,185 ns.** On Apple’s `dramc_nb2` the hardest check retired 9.5% fewer instructions on the solver thread, with the same proof search. [Quantifier measurements and memory cost](#preparing-frequently-checked-quantifier-bodies)

**Apple `rfn2` now takes 8.02% fewer instructions and 10.92% less elapsed time** across 12 paired whole-isolate runs. Reusing solver-local child records nearly halved the number of child tests, with all 10,342 Apple checks matching native search. [Whole-isolate and worker measurements](#reusing-child-records-in-asserted-disjunction-watches)

**Mixed-width Raft uses 6.05% fewer instructions and 5.59% less wall time when private proofs share equivalent Boolean definitions.** Both figures are medians across 24 paired runs of all 419 obligations. Native Z3 checked all 592 rewritten assertions. [Raft measurements and controls](#equivalent-boolean-definitions-in-private-raft-proofs)

**Raft 251 now takes 111 decisions and 548 quantifier instantiations. Raft 321 takes 129 and 204, including its side proof.** That specialization reduced median paired elapsed time by 2.07% on the full 419-obligation specification. Median elapsed times were 4.35 s before and 4.27 s after, on pinned CPUs. [Raft 251 measurements](#a-shared-transition-guard-reduces-raft-251s-search) · [Raft 321 measurements](#partial-instances-of-a-universal-consequence-on-raft-321)

**Specializing counterexamples to point updates reduced obligation 251 to 288 decisions and 1,872 quantifier instantiations.** The change won all 16 paired full Raft runs, with a median 5.5% reduction in elapsed time. An independent native check verified all 23 specializations. [Proof and timing](#point-updates-in-raft-transitions)

**A ground instance of the induction hypothesis reduced Raft verification from 6.67 s to 5.70 s** in a new comparison of two g3 builds on the same logical cores. g3 generated 54 such lemmas. Obligation 251 went from 9,198 to 4,565 quantifier instantiations, and obligation 321 went from 4,012 to 1,109 decisions. [Experiment and independent lemma checks](#ground-instances-of-induction-hypotheses)

**GoIvy with g3 beats native Z3 on the original Raft specification.** With normal CPU affinity and the default Go runtime, the median at the preceding solver revision `4f0c0457b` was **8.91 s with g3 and 44.40 s with native Z3** in the same GoIvy frontend. g3 won all nine pairs. In a separate pinned comparison with both frontends, the medians were **6.83 s for GoIvy/g3, 42.14 s for GoIvy/Z3 and 43.95 s for Python Ivy/Z3**. All 419 named proof outcomes agreed in every run. [Final comparison and reproduction](#final-raft-comparison) · [Default-affinity runs](perf/ivy_raft/frontier-4f0c0457b/host-default.csv) · [Three-checker runs](perf/ivy_raft/frontier-4f0c0457b/three-checkers.csv)

**The proof trace explains where the time went.** g3 discharged 234 obligations by finding contradictory ground instances. Its private order solver proved the other 185. On the original mixed-width input, search took 75.5% and loading formulas into that solver took 20.5% of g3’s measured proof-attempt time. A separate benchmark that gives the same search to Go and C++ still shows about four times as many retired instructions in Go. The constructor profiles break down the remaining instruction cost inside the paged AST arena. [Live proof measurements](#where-the-final-checker-spends-its-time) · [Matched SMT work and hardware counters](#what-a-matched-smt-search-costs-in-go-and-c)

**g3 matches Z3’s 16-byte SMT Boolean variable and 8-byte justification.** In the latest 24 pinned samples per engine with the branching queue active, scoped pop took **20.62 µs in g3 versus 12.84 µs in Z3**. Surviving pop took **313 ns versus 500 ns**. [Scope timings and instruction counts](#november-21-scope-restoration) · [Record layout](#matching-the-native-boolean-record) · [Independent-worker benchmark](smtcore/smt_scope_workers_benchmark_test.go)

**The solver thread still does more work on the Apple checks.** Across the 2,479 `dramc_nb2` verification calls, g3 retired 47.71 billion instructions and native Z3 retired 15.70 billion. Across 26 `rfn2` calls the counts were 93.08 and 35.48 billion. The complete checker also pays for Ivy formula generation and solver setup between checks. [Measured boundaries](#november-21-release-comparison)

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

**What happens when nodes actually die?** A separate control creates 128 distinct bound-variable ASTs with `MkVar`, takes references, drops all of them and repeats. Seven alternating runs of 5,000 batches gave **24.08 µs, 1,925 B and 4 allocations per batch** with pages, versus **29.86 µs, 18,306 B and 132 allocations** with individual objects. The 128 extra allocations are exactly one per node. Together the two workloads favor keeping the arena and changing the solver-local data around it.

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

#### SAT assignment and the unread tail of a watch list

A variable assignment or backtrack updates both literal polarities. The common path now validates the owner once. Malformed columns retain the former sequence of partial writes and errors. On conflict, the solver moves the unread watches with a single overlapping copy when the cursor bounds are valid. The tail keeps its original order. The change applies to the general SAT solver. [Assignment](sat/sat_solver_assignment.go) · [Watch cleanup](sat/sat_solver_propagation.go) · [Partial-column regression](sat/sat_solver_assignment_storage_test.go)

In 24 rotations per build on each of two pinned CPUs, the original 419 one-bit queries, reduced before search, went from **11.48 to 11.00 billion instructions**. The complete one-bit GoIvy check with direct instance matching took **14.00 versus 13.70 billion instructions** across 36 runs per build. Its median elapsed time fell on both cores, from 2.44 to 2.38 s and from 2.24 to 2.20 s. Mixed-width Raft was essentially unchanged. Pigeonhole-8 used **56.04 versus 55.34 billion instructions** with identical values for all 17 recorded search counters. Its median elapsed time increased slightly on both cores. [Full check timings](perf/ivy_raft/sat-assignment-raft.csv) · [SAT replays and counters](perf/ivy_raft/sat-assignment-search.csv)

Run `python3 perf/ivy_raft/onebit/run.py --bench-residual 3` to benchmark the production SAT search separately. It prepares all 419 simple-solver inputs before timing. Unit propagation refutes 33 and the SAT solver searches the remaining 386 in order. Run `python3 perf/ivy_raft/propagation_dimacs.py /path/to/g3 --holes 8 --planted 180` for the DIMACS control. [Residual benchmark](perf/ivy_raft/onebit/residual_test.go.txt) · [DIMACS generator](perf/ivy_raft/propagation_dimacs.py)

#### Loading assertions on independent solver goroutines

The public API worker benchmark exposed an ownership bug. Two private SMT-LIB parsers could temporarily install their parameter services as the process default. If the first parser and its solver closed before the second parser finished, the second parser tried to restore the first solver's closed arena. A two-worker regression schedules that ordering with channels. It reproduced the failure before the fix and passed 50 consecutive times afterward. Each solver stays on one goroutine throughout the test. [Overlapping parser regression](api/api_parser_independent_test.go)

Private parsers now borrow their solver's AST manager and parameter service for the whole parse. Only a main command session installs the process parameter binding. AST printers and diagnostic formatters use the manager's parameter service when present. The fix adds no lock to the worker path. The AST, command context, SMT-LIB parser and API suites passed, as did the GoIvy SMT and bridge tests. The Paxos liveness replay still matched the Python reference on all 27,855 normalized output lines. [Parser ownership](cmd_context/parser_context.go) · [Command session binding](parsers/smt2/commands.go) · [Private and main session tests](parsers/smt2/command_globals_test.go)

#### Testing ground-instance proofs against independent native answers

Ten deterministic transformations per obligation retain 0–100% of the premises, reorder them, or negate the goal. For each of the 419 mixed-width and 419 one-bit originals, a fresh native Z3 process checked each transformation. The direct matcher returned a proof on **675 of 4,190 inputs at each width**. Native Z3 returned UNSAT for every one of those 1,350 proofs. It returned SAT on **2,822 mixed-width** and **2,937 one-bit** inputs. The direct matcher declined every SAT case. Of the other cases, native Z3 reported unknown on 201 mixed-width and six one-bit queries and terminated by signal on one mixed-width query. The ordinary timeout was 600 ms. Nine direct proofs needed a second native check with 10 s allowed. [Counts by transformation](perf/ivy_raft/instances/perturbations.csv) · [Per-query results](perf/ivy_raft/instances/perturbation-queries.csv.gz)

Run `python3 perf/ivy_raft/instances/perturb.py --native ../build/z3 --input api/test_vectors/raft_order/mixed.jsonl.gz` from `pkg/`. Omitting the input selects the one-bit corpus. Each row records the native answer, direct proof decision and retry count. [Reproduction script](perf/ivy_raft/instances/perturb.py) · [Go API probe](perf/ivy_raft/instances/perturb_test.go.txt)

With Raft domain widths of 2, 3, 4, 8 and 16, ground-instance matching reduced the complete checker from about **62.0 to 48.8 billion instructions** at every width. Each build reproduced all 419 obligations in every run. On the independent SHT and TLS specifications and their two variants, the instruction changes were below 1%. Native Z3 still used roughly half as many instructions as g3 on SHT. [Twelve rotations per input and build](perf/ivy_raft/instances/transfer.csv)

#### Independent solvers on the complete Raft proof corpus

The API benchmark assigns each goroutine a contiguous, disjoint range of the 419 original verification conditions. Each worker creates, uses and closes its own context, solver and proof batch. The measurement includes SMT-LIB parsing, push, proof, pop and cleanup. The captured queries omit the Ivy frontend. Dividing the sequence also means each worker has to rebuild its own premise state. [Worker benchmark](api/api_raft_boolean_benchmark_test.go) · [Original mixed widths](api/test_vectors/raft_order/mixed.jsonl.gz) · [One-bit input](api/test_vectors/raft_order/onebit.jsonl.gz)

| Workers | Mixed, 1 Go CPU | Mixed, 4 Go CPUs | Mixed, 12 Go CPUs | One-bit, 12 Go CPUs | Mixed peak RSS, 12 Go CPUs |
|---:|---:|---:|---:|---:|---:|
| 1 | 8.29 s | 9.44 s | 8.39 s | 2.71 s | 92 MiB |
| 4 | 8.89 s | 3.32 s | 3.80 s | 1.05 s | 140 MiB |
| 16 | 10.03 s | 2.91 s | **2.57 s** | **0.76 s** | 250 MiB |
| 64 | 12.28 s | 3.72 s | 3.05 s | 0.93 s | 565 MiB |

Medians of nine rotated runs per combination on logical CPUs 0–11 of the shared host. All 216 runs proved every obligation, with the private-parser ownership fix in the measured binary. On 12 Go CPUs, 16 workers gave 3.26× the throughput of one on mixed-width Raft and 3.57× on one-bit Raft. Sixty-four workers spent more time and memory on this fixed-size corpus. Earlier 1,024-owner measurements characterize resident solver memory. Run `go test ./api -run '^$' -bench '^BenchmarkRaftIndependentProofs$' -benchtime=1x -cpu=1,4,12`. [Raw timings and allocations](perf/ivy_raft/independent-proof-workers.csv)

#### Halving the small-block allocator's free-list metadata

A pooled block used to carry both a 64-bit successor and a Boolean free flag, which takes 16 bytes on amd64. The allocator now uses one 8-byte word. Zero denotes a live or unused block, one marks the last free block, and every other free block stores its successor location plus one. Consolidation can still walk blocks in their original order and reuse every location even across chunk boundaries. [Allocator](arena/small_bytes.go) · [Cross-chunk reuse and accounting tests](arena/small_bytes_test.go)

Across four rotated 1,024-owner groups, the single-word layout saved **32.75 MiB** of idle Go heap on mixed-width obligation 001. At 64 active owners on obligation 274 it saved **83.6 KiB per solver**. Both sides of this seven-layout experiment used the same experimental AST directory. A separate comparison on the production paged directory checked all 419 obligations in 21 runs per build and width. Mixed Raft took 7.04 and 7.01 s, with 49.348 and 49.347 billion instructions. One-bit Raft took 1.70 s on both builds. The whole-check gain here is memory. [Owner measurements](perf/ivy_raft/allocator/free-link-owners.csv) · [Production checker comparison](perf/ivy_raft/allocator/free-link-checker.csv)

A second prototype followed Z3 more closely and stored the successor in the first eight freed payload bytes, with one occupancy bit per block for consolidation. It saved another 32 MiB per 1,024 easy solvers. Its median reuse cost increased 4.0% across 16 size, alignment and working-set combinations. The single-word external links improved 14 of 16 combinations, with a median reduction of 1.6%. Each combination had 24 samples. Run `go test ./arena -run '^$' -bench '^BenchmarkSmallBytesBlockReuse$' -benchtime=6000x -cpu=1`. [Working-set benchmark](arena/small_bytes_reuse_benchmark_test.go) · [Working-set samples](perf/ivy_raft/allocator/free-link-working-sets.csv.gz) · [Seven complete checker controls](perf/ivy_raft/allocator/free-link-layouts.csv) · [C++ small-object allocator](../src/util/small_object_allocator.cpp)

#### What a matched SMT search costs in Go and C++

A fresh simple solver replayed each of the 419 order problems with the same macro-finder setting. Decisions, conflicts, propagations, quantifier instantiations and the other 14 recorded counters matched at every check. The GoIvy shortcut saves whole searches on the real specification. This replay isolates the cost of performing the same searches. With Ivy's Python environment active, run `python perf/ivy_raft/replay_order_kernel.py --verify-search` to check both backends against the recorded per-obligation counters. [Go benchmark](api/api_raft_kernel_benchmark_test.go) · [Native replay and validation](perf/ivy_raft/replay_order_kernel.py) · [Expected counters](perf/ivy_raft/order-matched-search.csv)

| Median over one complete replay | g3, production paged lookup | g3, direct slot pointers | Native Z3 |
|---|---:|---:|---:|
| Instructions, CPU 11 | 86.58 B | 84.06 B | 21.27 B |
| Cycles, CPU 11 | 69.97 B | 69.50 B | 18.75 B |
| Branches, CPU 11 | 18.07 B | 18.16 B | 4.32 B |
| Mispredicted branches, CPU 11 | 230 M | 227 M | 117 M |
| L1 data-cache loads, CPU 15 | 39.57 B | 39.17 B | 10.33 B |
| L1 data-cache load misses, CPU 15 | 758 M | 723 M | 306 M |
| Elapsed inside the 419 checks, CPU 11 | 15.70 s | 15.63 s | 3.76 s |

Each event group had 16 rotated samples per build. Branch counters ran continuously and the data-cache counters ran for 82–83% of each sample. g3 retired **4.07 times as many instructions**, performed **3.83 times as many data loads**, and took **3.73 times as many cycles** as Z3. Instructions per cycle were about 1.24 for g3 and 1.13 for Z3 on CPU 11. These counts point to the amount of work per solver operation. Reducing page-address arithmetic alone can recover only a small part of the difference. [All PMU samples, phase times and event coverage](perf/ivy_raft/ast_lookups/matched-pmu.csv.gz)

Per-manager instrumentation counted **505.88 million** calls to `nodeAt` during the checks and another **33.66 million** during loading. Checks on the primary AST managers made 144.97 million `appAt` calls, 66.93 million `validNode` calls, 28.33 million `GetArg` calls and 5.18 million `SubstitutionShape` calls. These methods call one another, so adding their counts would double-count work. Ten of the 419 queries account for **56.9%** of the checking-phase node reads. Queries 275 and 131 each made over 71 million. Run `python3 perf/ivy_raft/ast_calls.py` for the per-obligation counts and use `--sites nodeAt` for source locations. The instrumented runs also check all 18 search counters. [Counter driver](perf/ivy_raft/ast_calls.py) · [Per-query counts](perf/ivy_raft/ast_lookups/primary-call-counts.csv.gz)

Profiles of the complete GoIvy checks show why the two widths respond to different changes. With direct instance matching enabled, the mixed-width run sampled 2.93% in Go's GC span-scanning helper and 1.46% in application rewriting. The one-bit run sampled 7.92% in SAT propagation and 4.44% in Boolean unit reduction. On mixed-width native Z3, relevancy lookup alone sampled 6.02%. These are flat cycle shares from six mixed and twelve one-bit profiles per build. [Repeated profiles with symbol names and ranges](perf/ivy_raft/ast_lookups/checker-profile.csv)

#### Direct AST slot pointers and physical node placement

The prototype keeps the same stable, Go-managed 64-slot pages and indexes a pointer to each slot directly. Go can then skip the page and offset arithmetic. On the actual mixed-width checker it removed **1.44 billion instructions** in a 37-pair confirmation, from 49.23 to 47.79 billion. Elapsed time moved from **8.71 to 9.07 s**. On two earlier cores it moved from 10.14 to 10.09 s and from 6.97 to 7.02 s. On the matched kernel, user CPU moved from **12.93 to 12.82 s** and instructions from 85.92 to 83.47 billion. [37-pair confirmation, including sibling CPU occupancy](perf/ivy_raft/ast_lookups/slot-directory-confirmation.csv) · [Two-core full-check measurements](perf/ivy_raft/ast_lookups/slot-directory-checker.csv)

The direct directory also added **4,784 live bytes per idle solver** on the easy case and **155,673 bytes per active solver** on the hard case. The hard measurement used 64 solvers. At the same occupancy it would add about 152 MiB across 1,024. Traversing 256 applications improved from 13.00 to 11.95 ns, while traversing 4,096 went from 21.88 to 25.11 ns. Keeping the paged directory preserves the lower footprint. The prototype, including failure-injection tests for growing and retrying its directory, is available to test other CPUs. Run `go test ./ast -run '^$' -bench '^BenchmarkASTNodeTraversal$' -cpu=1` before and after applying the patch in a checkout of `058660be8`. [Prototype patch](perf/ivy_raft/ast-slot-directory.patch) · [Traversal benchmark](ast/ast_node_directory_benchmark_test.go) · [Traversal samples](perf/ivy_raft/ast_lookups/slot-traversal.csv) · [Worker heap samples](perf/ivy_raft/ast_lookups/slot-directory-owners.csv)

Holding the direct directory and its logical growth policy fixed, individual Go allocations per slot and pages of 4, 8, 16, 32 or 64 slots all retired 84.8–84.9 billion instructions on the earlier matched replay. Individual allocations and 64-slot pages both took 15.62 s median. Giving the first 64–4,096 slots a separate contiguous fast path added instructions on the same search. The results favor improving high-frequency accesses on the existing pages. [Physical placement and directory controls](perf/ivy_raft/ast_lookups/placement-kernel.csv.gz)

The source-location counters identify which calls to combine. During checking, `GetArg` invoked each of `nodeAt` and `appAt` 28.33 million times. Declaration-identity reads and function-flag reads accounted for 14.20 and 11.48 million of the 36.43 million declaration-record lookups. Of the 66.93 million node validations, conversion from external handles accounted for 26.39 million and expression-substitution keys accounted for 7.11 million. The location totals agree exactly with the independent per-method counts in both phases. [All source locations for the four methods](perf/ivy_raft/ast_lookups/call-sites.csv)

#### Running 2,048 resident solvers

Each worker constructs its own context, loads its query, waits for all other workers to finish loading, proves the query and closes its solver on the same goroutine. A full GC at the barrier measures the live Go heap. The timer starts when the coordinator releases the barrier. The separate process wall time also includes loading the input, the barrier measurement and test setup.

| Query | Resident workers | Live heap per worker | Time from release to last close | Whole process |
|---|---:|---:|---:|---:|
| Mixed 001 | 256 | 2.40 MiB | 54 ms | 1.05 s |
| Mixed 001 | 1,024 | 2.40 MiB | 199 ms | 3.67 s |
| Mixed 001 | 2,048 | 2.40 MiB | 409 ms | 6.83 s |
| One-bit 274 | 128 | 2.56 MiB | 558 ms | 1.26 s |
| Mixed 274 | 128 | 2.56 MiB | 8.90 s | 9.96 s |

Medians of five runs on 12 Go CPUs, with the single-word free lists in production. All workers proved their obligation on both builds in all 60 groups. At 2,048 workers the free-list change reduced loaded heap by 33,537 bytes per worker, or 65.5 MiB for the group. Peak process RSS varied with GC and task scheduling. The workload holds 2,048 independent copies of an easy query. Obligation 274 illustrates how much the cost of proving depends on the input. Run `G3_RAFT_MEMORY_QUERY=mixed/001 G3_RAFT_MEMORY_OWNERS=2048 go test ./api -run '^$' -bench '^BenchmarkRaftResidentWorkerProofs$' -benchtime=1x -cpu=12`. [Benchmark and barrier](api/api_raft_worker_memory_benchmark_test.go) · [Group timings, loaded heap and RSS](perf/ivy_raft/allocator/resident-worker-proofs.csv)

#### Reading and recycling vector descriptors

A vector descriptor's slot number and generation remain 32 bits each. A live slot links to itself, a free slot links to the next free slot, and recycling a slot at its last generation retires it. This brings the generation and free-list metadata from 12 to 8 bytes per slot. Read, write and recycle compute the page address once. Page trimming keeps the generation even when it releases the payload. [Slab implementation](arena/descriptor_slab.go) · [Reuse and generation tests](arena/concrete_test.go)

In five runs with 2,048 resident solvers, this saved **12,286 live bytes per loaded solver**, about **24 MiB** for the group. The comparison uses the production paged AST directory and holds the small-block free-list layout fixed. A random checked read among 1,024 live descriptors fell from **5.48 to 3.55 ns**. Recycling an aligned 56-byte block among 8,192 live blocks fell from **61.72 to 50.72 ns**. Both measurements had 15 rotated samples per layout. Run `go test ./arena -run '^$' -bench 'BenchmarkDescriptorReadLive|BenchmarkSmallBytesBlockReuse' -cpu=1`. [Benchmarks](arena/descriptor_slab_benchmark_test.go) · [Working-set samples](perf/ivy_raft/allocator/descriptor-working-sets.csv.gz) · [Resident heap](perf/ivy_raft/allocator/descriptor-resident.csv)

Across 21 production pairs the complete mixed check took 7.60 and 7.38 s, and the one-bit check took 1.74 and 1.90 s. Instructions fell from 49.33 to 49.17 billion on mixed and from 13.49 to 13.41 billion on one-bit. The matched kernel retired 83.58 and 83.27 billion instructions. Separate 23-round controls compared shared and duplicated page-lookup code on the archived build. Median user CPU was 11.15 s before and 11.05 s with the shared helper. The reproducible benefits are descriptor access time and resident heap. [Full checker and kernel measurements](perf/ivy_raft/allocator/descriptor-release.csv) · [Three lookup implementations](perf/ivy_raft/allocator/descriptor-kernel.csv)

#### An 8-byte reason in the general SAT solver

The SMT Boolean record already uses 16 bytes, including an 8-byte justification. The general SAT solver has a separate reason column that checks clause allocator identity, slot, generation and storage kind. I prototyped 16-, 12- and 8-byte representations of that column. The 8-byte version reconstructs the allocator identity from the sole owner. It stores a full record in a side map for values that exceed the packed range. The benchmark reads each written reason back and compares the complete locator. [SMT record](#matching-the-native-boolean-record) · [Column benchmark](sat/sat_justification_benchmark_test.go) · [8-byte prototype](perf/ivy_raft/allocator/compact-sat-reasons.patch)

Across 128 active one-bit solvers on obligation 274, the 8-byte column saved **60 KiB per solver**, or 7.50 MiB for the group. After closing the batch the sizes matched within 8 bytes per solver. At 16,384 live entries, the median cost of writing and reading a binary justification rose from **12.93 to 16.97 ns**, and a clause justification from **15.36 to 18.53 ns**. There were 20 samples per layout and fixture. [Resident heap](perf/ivy_raft/allocator/sat-reason-owners.csv) · [Five column layouts](perf/ivy_raft/allocator/sat-reason-column.csv)

Across 32 runs of the 419 residual SAT problems, instructions rose from **11.52 to 11.62 billion**. The complete one-bit Raft checker went from **13.67 to 13.73 billion** in 28 runs per build. The general SAT column therefore retains its 32-byte record. To reproduce the size comparison, run `go test ./sat -run Justification -bench '^BenchmarkSATJustificationColumn$' -cpu=1` before and after applying the prototype patch at the repository root. [SAT replay and search counters](perf/ivy_raft/allocator/sat-reason-search.csv.gz) · [Complete checker](perf/ivy_raft/allocator/sat-reason-checker.csv)

#### Building the one-bit Boolean circuit

The encoder now stores three signed 32-bit literals per gate. A conjunction has a zero third operand, while a conditional always has three nonzero operands. The two existing limits of 100,000 inputs and 100,000 gates keep each literal well within a signed 21-bit field. Packing three fields into a 64-bit Go map key preserves operand order and signs. The backwards pass that determines which polarities are needed also counts the exact number of CNF rows. The builder reserves those rows once and retains its original emission order. All the storage belongs to one proof attempt. [Encoder](api/internal/order/boolean_problem.go) · [Gate identity and exhaustion tests](api/internal/order/boolean_problem_test.go)

| Measurement | Before | After |
|---|---:|---:|
| Encode one-bit Raft 274, CPU 6 | 872 µs | 586 µs |
| Bytes allocated during that encoding | 603,904 | 248,992 |
| Allocations during that encoding | 74 | 19 |
| Prepare the whole proof input for 274, CPU 6 | 1,076 µs | 797 µs |
| Prepare the whole proof input for 274, CPU 28 | 1,068 µs | 789 µs |
| Bytes allocated preparing the proof input | 795,480 | 443,384 |
| Complete one-bit spec, wall time | 1.76 s | 1.63 s |
| Complete one-bit spec, instructions | 13.41 billion | 13.14 billion |

The isolated benchmark used 21 rotations per core. Obligation 274 generated the same 3,102 variables and 5,354 clauses. Repeating a single predicate 1,024 times exercises the opposite case. Bounding the initial gate capacity by the number of input variables reduced allocation there from 93,704 to 35,788 bytes. A handwritten hash table was faster on some small circuits but used 116,020 bytes on the repeated input and 306,080 on 274. The full-checker crossover used 21 runs per binary and width. The mixed-width spec, which uses the order solver, retired 49.19 and 49.23 billion instructions. Run `python3 perf/ivy_raft/onebit/run.py --bench-compiler 400` from `pkg/`. [Five encoder fixtures](perf/ivy_raft/onebit/compiler_benchmark_test.go.txt) · [Production samples](perf/ivy_raft/onebit/encoder-production.csv.gz) · [Complete checks](perf/ivy_raft/onebit/encoder-release.csv) · [Twelve gate layouts](perf/ivy_raft/onebit/gate-layouts.csv.gz) · [Layout checks on two cores](perf/ivy_raft/onebit/gate-checker.csv.gz)

The raw clauses, reduced clauses, variable counts and charged work matched byte for byte on all 419 queries. Three replay rounds agreed on 2,500 fresh satisfiable formulas, their contradictory companions and all Raft answers with both general SAT and the independent certificate-checking solver. The public batch API also checked 10,000 separate known models and 20,000 refutations, with native Z3 confirming all 19 independently sampled pairs. The seed ranges started at 6,100,000 and 6,000,000 respectively. [CNF replay](perf/ivy_raft/onebit/run.py) · [Public batch validation](perf/ivy_raft/onebit/batch/check.py)

#### Which solver operations read the AST

The primary manager read application children 28,330,645 times during the 419 matched checks. The ordinary rewriter accounted for 9,749,206 calls, the de Bruijn variable shifter for 3,311,529, and the candidate checker's congruent-enode lookup for 1,658,912. The variable shifter also made 3,524,333 of the 5,176,306 substitution-shape reads. In contrast, the beta reducer made 776,649. Changes confined to beta reduction have a correspondingly smaller opportunity on this corpus. [Caller counts by source line](perf/ivy_raft/ast_lookups/cross-package-calls.csv)

The theory rewriter made all 6,618,903 checking-phase calls to the dependency-returning substitution lookup. A separate instrumented replay found 36,821 hits, 4,449,552 lookups in an empty table and 6,332,934 queries for a node that had never been inserted as a substitution key in its lifetime. A definite miss can potentially avoid a table read while retaining the validation of input handles. Run `python3 perf/ivy_raft/ast_calls.py --callers` to identify callers across packages. The probe counts return PCs on each manager, resolves inlined caller locations after each phase and validates all 18 search counters per obligation. Totals agree with the independent method instrumentation wherever the probes cover the same method. [Counter driver](perf/ivy_raft/ast_calls.py) · [Call-site probe](perf/ivy_raft/ast_callers.go.txt) · [Substitution table size, membership and hits by obligation](perf/ivy_raft/ast_lookups/substitution-membership.csv.gz)

#### Testing the compiler profile on the current checker

I built GoIvy against the local replacement at `af769fa16` with PGO off, with the original Raft profile and with a new Go 1.27.3 profile. The new profile combines five runs on uniform three-bit Raft, five on one-bit Raft at `03403f7a2` and an earlier CPU profile of the odd-numbered matched-kernel obligations. Each build used the same GoIvy source and `xtrace_off`. [Original profile](perf/ivy_raft/raft-blended.pprof) · [New profile for replication](perf/ivy_raft/raft-proof-blended.pprof)

| Profile | Original Raft wall | Original instructions | One-bit wall | One-bit instructions |
|---|---:|---:|---:|---:|
| Off | 9.11 s | 49.08 B | 2.10 s | 13.12 B |
| Original Raft blend | 8.38 s | 45.25 B | 2.13 s | 12.71 B |
| New proof blend | 8.73 s | 45.30 B | 2.11 s | 12.22 B |

Each timing is the median of 24 runs across two cores. All 419 named results agreed. On the held-out two-bit and four-bit specifications the instruction decrease against PGO off was about 7.7% with either profile. Wall times varied by input and core. The original profile gave the best wall time on the original specification. The new one reduced one-bit instructions further, with essentially the same elapsed time across all three builds. Both are explicit `-pgo` build options. The production comparison with native Z3 uses the ordinary checker. [All measurements, including SHT and TLS](perf/ivy_raft/onebit/compiler-pgo-checker.csv.gz) · [Earlier 209-obligation held-out kernel replay](perf/ivy_raft/onebit/compiler-pgo-held-out.csv.gz)


#### Verifying the local replacement on the whole spec

The ordinary GoIvy executable at `594f703d0` resolved g3 from the user's local `go.mod` replacement. I built both backends with `-mod=readonly` and timed the original mixed-width specification on CPUs 0 and 10 with one Go CPU. At `af769fa16`, g3 won every paired run. The native/g3 wall-time ratio ranged from 4.77 to 7.74, with a median paired ratio of 5.87. The ratio of the two overall medians is 6.08. Separately, a tracing-enabled GoIvy executable matched all 27,855 normalized Python output lines, including all 27,782 trace events, and reported the expected Multi-Paxos fragment error at line 253. GoIvy’s SMT and bridge tests also passed. [Timings, memory, hardware counters and both revisions](perf/ivy_raft/frontier-af769fa16.csv)

On the unrestricted 48-CPU configuration, one invocation of `make bench` measured 865 ns for surviving and 23.78 µs for scoped atoms. `make cppbench` measured 513 ns and 14.10 µs. Pinning both backends and activating the same branching queue gave a scoped-pop ratio of 1.51 across 18 runs per build. The 16-byte record solved the storage difference. Scope restoration and a generic matched SMT search remain performance targets in their own right. [Three scope controls on two cores](perf/scope_restoration/frontier-af769fa16.csv) · [Go benchmark with independent workers](smtcore/smt_scope_workers_benchmark_test.go)

#### Rejecting impossible expression-substitution lookups

The live AST record now remembers whether its current generation has ever been inserted into a substitution. The first insertion sets the flag. Erasing a key from one substitution can leave it present in another, so the flag stays set for the lifetime of the node. Reusing an AST slot starts a fresh record with the flag cleared. On amd64 the Boolean fits in the existing node padding. All lookups still check the substitution owner, key generation and expression kind before inspecting the flag. [Implementation](ast/ast_expr_substitution_table.go) · [Two-table regression](ast/ast_expr_substitution_test.go)

| Lookup against a 512-entry substitution | Before | After |
|---|---:|---:|
| Key present | 26.36 ns | 25.69 ns |
| Node never used as a key | 26.48 ns | 13.80 ns |
| Key present only in another substitution | 26.28 ns | 27.72 ns |

Medians of 70 samples per operation and build on two cores. The fast miss applies to 95.7% of the lookup calls in the matched checking phase. The candidate in the third row correctly performs a table lookup. Run `go test ./ast -run '^$' -bench '^BenchmarkExprSubstitutionFind$' -benchtime=500000x -cpu=1`. [Benchmark](ast/ast_expr_substitution_benchmark_test.go) · [Lookup measurements](perf/ivy_raft/ast_lookups/substitution-miss-lookup.csv.gz)

In 24 paired production runs, the 419-query matched kernel went from **82.99 to 82.74 billion instructions** and from **13.04 to 12.31 s** of user CPU. The complete mixed-width check went from **49.11 to 48.95 billion instructions** and from **8.30 to 8.48 s** elapsed. The one-bit check took 2.05 and 2.04 s. The wall-time improvement is specific to the lookup and this matched-kernel series. Native Z3 and g3 agreed on all 18 per-query search counters. [Production crossover](perf/ivy_raft/ast_lookups/substitution-miss-release.csv)

#### Reusing a bound variable's sort hash

Of the **661,082** variable registrations during the 419 matched checks, **631,049** (95.5%) requested an existing variable. The variable factory already knows the de Bruijn index and has validated its sort. It now supplies the hash computed from those two values to registration. This avoids reading the temporary variable back and validating its sort a second time. Insertion, candidate reclamation, failure handling and tracing still use the common registration code. [Variable factory](ast/ast_core_factories.go) · [Shared hash calculation](ast/ast_table_hash.go)

| Live canonical variables | CPU 10 before → after | CPU 30 before → after |
|---|---:|---:|
| 8 | 106.0 → 85.6 ns | 76.3 → 54.2 ns |
| 256 | 104.0 → 87.5 ns | 76.5 → 56.2 ns |
| 4,096 | 125.3 → 102.3 ns | 90.8 → 73.0 ns |

These are medians of 35 runs per build on each CPU, with 180,000 lookups and zero allocations per operation. At the observed call count, a 20 ns saving accounts for about 13 ms of a complete matched replay. Run `go test ./ast -run '^$' -bench '^BenchmarkASTCanonicalVariable$' -benchtime=180000x -cpu=1`. [Benchmark](ast/ast_app_benchmark_test.go) · [Raw samples](perf/ivy_raft/ast_lookups/canonical-variable.csv.gz)

Applications are the larger opportunity. The checking phase requested 2,801,475 applications, including 1,293,374 duplicates. Binary and ternary applications accounted for 1,088,858 of those duplicates. For all node kinds combined, 3,575,777 lookups visited 5,950,828 hash-table slots, or 1.66 per lookup. This favors reducing the cost of building and comparing common candidates. The interning probe keeps counters per AST manager and verifies all 18 search counters for each obligation. Run `python3 perf/ivy_raft/ast_calls.py --interning` from `pkg/`. [Per-obligation counts](perf/ivy_raft/ast_lookups/interned-nodes.csv.gz) · [Instrumentation](perf/ivy_raft/ast_calls.py)

The complete checker at `4f0c0457b` took 10.03 s on mixed widths and 2.34 s on one-bit, versus 9.93 s and 2.34 s on its immediate predecessor in 21 alternating pairs. The matched kernel retired 82.13 versus 82.19 billion instructions. Only the constructor timing supports a material speedup from the variable change alone. Passing the hash one step further, from registration into the interning table, increased the eight-variable lookup from 50.2 to 60.9 ns and changed matched-kernel user time from 9.89 to 9.94 s. That version remains experimental. [Bound-variable checker comparison](perf/ivy_raft/ast_lookups/variable-hash-release.csv) · [Table-hash microbenchmarks](perf/ivy_raft/ast_lookups/table-hash-lookup.csv.gz) · [Table-hash checker comparison](perf/ivy_raft/ast_lookups/table-hash-release.csv.gz)

#### Why caching a shifted expression can cost more than recomputing it

The variable shifter saw 212,804 non-ground roots in the matched checking phase. In 90,644 cases, one of the last 64 calls had the same input and parameters and its result was still live. The shifter rebuilt 1,365,074 applications. Of 631,196 free-variable visits, 617,858 changed the de Bruijn index. A cache therefore has genuine reuse to exploit. [Per-obligation repetition counts](perf/ivy_raft/ast_lookups/shifter-repetition.csv.gz)

| Policy on the archived solver | Different searches out of 419 | Matched corpus instructions | Full mixed-width checker instructions |
|---|---:|---:|---:|
| Existing traversal | 0 | 87.00 B | 49.41 B |
| Reuse the previous whole result | 3 | 89.14 B | 165.41 B |
| Reuse one of the last 16 whole results | 3 | 90.12 B | 53.33 B |
| Cache four individual shifted variables | 0 | 86.59 B | 49.20 B |

The complete-checker figures are medians of 20 runs per policy. All 419 answers agreed. Reusing a whole result changed the instantiation and decision counts on obligations 134, 275 and 321. With a one-entry cache, decisions at 134 increased from 2,055 to 2,711. Skipping the traversal also skips its temporary node construction and reference-count activity. The measurements establish the change in search. They do not identify the first internal operation that changed a decision. The four-variable policy avoided these three divergences, but a separate 15-pair matched replay took 11.96 s of user CPU versus 11.47 s. A cache hit would also bypass the ordinary temporary-node allocation under finite budgets. The production shifter keeps the original traversal. [All changed counters](perf/ivy_raft/ast_lookups/shift-cache-search.csv) · [Thirteen rotated kernel comparisons](perf/ivy_raft/ast_lookups/shifter-search-cost.csv.gz) · [Complete checker comparisons](perf/ivy_raft/ast_lookups/ast-lookup-checker-trials.csv.gz) · [Sixteen-entry prototype for 058660be8](perf/ivy_raft/ast_lookups/shift-cache-16.patch.gz)

Holding application payload pointers on rewriter frames added work on this corpus. The matched search rose from 86.75 to 88.26 billion instructions. Fusing parent and declaration lookups in the traversal reduced a separate matched series from 86.88 to 86.68 billion. Direct canonical-table hints for four through 64 variables also had small effects. The per-run results cover each baseline and each variant on identical proof inputs. [Kernel comparisons](perf/ivy_raft/ast_lookups/rewrite-kernel-trials.csv.gz) · [Whole-checker comparisons](perf/ivy_raft/ast_lookups/ast-lookup-checker-trials.csv.gz) · [Shifter repetition probe](perf/ivy_raft/ast_lookups/shifter-repetition.patch.gz)

A final prototype carried the candidate node and application records through registration. On `4f0c0457b`, all 18 counters matched across the 419 kernel queries. In 14 alternating runs per build, it reduced instructions from 82.16 to 81.88 billion. Matched-kernel elapsed time was 15.10 versus 15.07 s, full mixed-width Raft was 9.42 versus 9.34 s, and one-bit was 2.18 versus 2.21 s. Across 60 canonical-construction samples per build, a four-argument application fell from 341.8 to 333.6 ns and a unary application rose from 135.8 to 139.6 ns. The extra registration branches remain in a saved prototype for independent testing. [Prototype for 4f0c0457b](perf/ivy_raft/ast_lookups/registration-app.patch.gz) · [Constructor samples](perf/ivy_raft/ast_lookups/registration-construction.csv.gz) · [Checker and matched search](perf/ivy_raft/ast_lookups/registration-release.csv.gz)

## Final Raft comparison

The original mixed-width input is `examples/raft/raft_no_assume_test.ivy`. Each timed process loads the specification, checks it, prints all 419 named proof results in order and exits successfully with `OK`. The comparison verifies the entire ordered proof transcript against Python Ivy. GoIvy's ordinary g3 and `classic_z3` builds use the same frontend. Both were built with the user's local module replacement and `-mod=readonly`. Timings use the shared Ryzen Threadripper 3960X on Linux/amd64, Go 1.27.3, with tracing compiled out and PGO off. [Checker harness](perf/ivy_raft/compare_checkers.py) · [Transcript comparison](perf/ivy_raft/measure.py)

| Original spec | Runs per engine | GoIvy/g3 | GoIvy/native Z3 | Python Ivy/native Z3 |
|---|---:|---:|---:|---:|
| Default runtime, CPUs 0–47 available | 9 | **8.91 s** | 44.40 s | — |
| Alternating all three, pinned to CPU 18 or 34 | 18 | **6.83 s** | 42.14 s | 43.95 s |
| Hardware counters, pinned to CPU 0 or 10 | 16 | **8.13 s** | 49.98 s | — |

The table gives median elapsed times. The median **within-pair** speedups over native GoIvy were **5.07×, 5.91× and 5.96×** respectively. g3 won all 43 pairs. In the three-checker comparison, it also won all 18 pairs against Python Ivy, with a median within-pair speedup of 6.40×. Both GoIvy timings varied substantially by core. For example, g3 took 6.29 s on CPU 0 and 9.67 s on CPU 10. The hardware counters were much steadier at **49.06 billion instructions with g3 versus 215.52 billion with native Z3**. Median peak RSS in that run was 75.7 versus 314.7 MiB. The default-runtime and three-checker series used GoIvy `594f703d0`; the final hardware and trace checks used `ee84dd456`, with the same GoIvy directory contents. [Default runtime](perf/ivy_raft/frontier-4f0c0457b/host-default.csv) · [Both frontends](perf/ivy_raft/frontier-4f0c0457b/three-checkers.csv) · [Hardware counters and revisions](perf/ivy_raft/frontier-4f0c0457b/paired-pmu.csv)

After the other experiments finished, five more alternating pairs on CPU 6 took **5.79 s with g3 and 32.82 s with native Z3**. Each process reproduced the 419-result reference transcript. Across all 48 original-spec pairs, the smallest observed speedup was 4.27×, in the default-affinity series. [Final five pairs](perf/ivy_raft/frontier-4f0c0457b/last-core6.csv)

The all-one-bit variant exercises a different backend. g3 won **15 of 16 pairs** against GoIvy/native Z3, with a median within-pair speedup of **1.38×**. The medians on CPU 17 were 1.54 and 2.11 s. CPU 37 gave 4.72 and 6.50 s under contention. The retired instruction counts were close at 13.13 and 13.34 billion respectively. [One-bit paired runs](perf/ivy_raft/frontier-4f0c0457b/onebit-pmu.csv)

On the user's original Makefile targets with default CPU settings, `make bench` measured **723 ns / 24.96 µs** for surviving / scoped atoms, versus **477 ns / 12.74 µs** from `make cppbench`. Those are single invocations. Over 18 pinned runs per configuration, the standard Go fixture gave 361 ns / 15.19 µs, the queue-enabled Go fixture gave 306 ns / 19.39 µs, and the queue-enabled native fixture gave 488 ns / 12.70 µs. The isolated queue-enabled scoped-pop gap remains **1.53×**. [Pinned fixture comparisons](perf/scope_restoration/frontier-4f0c0457b.csv)

A separate eight-round control used GoIvy builds with tracing available and `XTRACE_OFF=1` at runtime. Original Raft took 6.62 s with g3 and 38.50 s with native Z3. With tracing compiled out on the same core the medians were 6.01 s and 40.41 s. The trace-capable frontend prints some source paths with the prefix `<IVY_EXAMPLES>`. The timing harness accepts that spelling and verifies identical source files, line numbers, names and results. [Both build configurations and widths](perf/ivy_raft/frontier-4f0c0457b/trace-builds.csv)

On uniform two-bit Raft the medians were 6.05 s for g3 and 94.26 s for native Z3. On uniform 16-bit Raft, g3 completed all six runs at a median of 9.57 s. Native Z3 finished five at a median of 171.11 s and reached obligation 412 before the other run's 200 s limit. Native Z3 was faster on two independent protocols. The SHT transfer isolate took 2.31 s with g3 versus 1.08 s with native Z3, and the TLS stream-position isolate took 0.285 s versus 0.260 s. Each backend agreed on all 83 SHT and five TLS results. [Six rotations per specification](perf/ivy_raft/frontier-4f0c0457b/transfer.csv)

To repeat the default-runtime comparison on a remote Linux machine, activate the Ivy Python environment and run the following from `g3/pkg`. The `go list` line prints the directory selected by the remote machine's replacement. The runner writes one complete transcript and timing record per execution. Add `--cpu N` to the runner to pin the checkers to the same logical CPU.

```sh
ivy_dir="$HOME/ivy/goivy"
input="$HOME/ivy/ivy-lang-examples/examples/raft/raft_no_assume_test.ivy"
out=$(mktemp -d)
go -C "$ivy_dir" list -mod=readonly -f '{{.Dir}}' github.com/glycerine/g3/pkg/ast
go -C "$ivy_dir" build -mod=readonly -tags=xtrace_off -o "$out/g3" ./cmd/goivy_check
go -C "$ivy_dir" build -mod=readonly -tags=xtrace_off,classic_z3 -o "$out/native" ./cmd/goivy_check
XTRACE_OFF=1 python -O "$(command -v ivy_check)" "$input" > "$out/reference.log"
python3 perf/ivy_raft/compare_checkers.py --rounds 9 \
    --g3 "$out/g3" --native "$out/native" --python-ivy "$(command -v ivy_check)" \
    --reference "$out/reference.log" --output "$out/runs" "$input" > "$out/runs.csv"
```

### Where the final checker spends its time

In 24 instrumented executions of each specification, 234 of the 419 obligations had a contradictory ground instance. On mixed widths the other 185 proved unsatisfiable in the private order solver. On one-bit the other 185 used the Boolean solver. All proof attempts succeeded. The instrumented build recorded elapsed time inside each attempt separately from the timings in the main comparison. [Per-obligation measurements](perf/ivy_raft/frontier-4f0c0457b/proof-phases.csv.gz) · [Timing instrumentation for 4f0c0457b](perf/ivy_raft/frontier-4f0c0457b/proof-phases.patch.gz)

| Median accumulated time over 419 attempts | Original mixed widths | All one-bit |
|---|---:|---:|
| Read assertions and prepare proof input | 0.103 s | 0.184 s |
| Load the translated query into the order solver | 1.154 s | — |
| Order solver's `Check` or complete Boolean check | 4.244 s | 0.686 s |
| Total inside the proof attempts, including setup and cleanup | 5.624 s | 0.869 s |

The five most expensive mixed-width attempts consumed 43.3% of the measured proof time. Obligation 321, `choosable_vote_msg`, took 0.78 s median inside its proof. It added **4,012 of 16,900 decisions** and **4,676 of 34,716 quantifier instantiations** across the 185 order checks. Obligation 251 generated the most instantiations, **9,198**. These are differences between the counters immediately before and after each private solver check. Six separate executions agreed on all 18 raw counters at both points for every query. On GoIvy/native Z3, obligation 134 still dominated the actual spec at 14.95 s and 18.14 s median on the two cores. The corresponding g3 intervals were 0.32 and 0.35 s. [Before and after each auxiliary check](perf/ivy_raft/frontier-4f0c0457b/auxiliary-search.csv.gz) · [Pass-to-pass timings for all 419 named obligations](perf/ivy_raft/frontier-4f0c0457b/per-obligation.csv.gz)

Sampling user-mode cycles in the complete processes gives different targets at each width. On mixed-width Raft, Go runtime span scanning accounted for 2.42% in `tryDeferToSpanScan` and 1.67% in `scanObjectsSmall`. Application rewriting accounted for 1.69% in `processApp`. GoIvy/native spent 6.34% in SMT relevance lookup and 5.0% inserting ASTs into its canonical table. On one-bit Raft, g3's SAT propagation loop accounted for 7.71% and Boolean problem reduction for 4.25%. These percentages use each process's own total cycles as the denominator. The profiles used six complete runs per engine at mixed widths and nine at one-bit. [Thirty leading functions per backend, with samples per run](perf/ivy_raft/frontier-4f0c0457b/function-samples.csv.gz)

To reproduce the stage counts, apply either instrumented patch in a temporary checkout of `4f0c0457b`, build GoIvy against that tree and run the original spec. The time probe emits one `RAFT-PHASE` line per obligation on stderr. The counter probe emits one `BEGIN-ORDER-PROOF` marker per obligation and an `ORDER-COUNTERS` pair around each call to the order solver's `Check`. [Proof-stage instrumentation](perf/ivy_raft/frontier-4f0c0457b/proof-phases.patch.gz) · [Live search counter instrumentation](perf/ivy_raft/frontier-4f0c0457b/auxiliary-search.patch.gz)

### Correctness checks on the final solver

Fourteen independently modified Raft protocols ran at both original and one-bit widths. All 28 inputs had failing obligations. g3 agreed with both Python Ivy and GoIvy/native Z3 on **all 11,732 named decisions**, including the same **77 failures**, and returned the expected nonzero exit in all 28 cases. GoIvy's SMT and bridge tests also passed. The tracing build reproduced Python's Multi-Paxos fragment diagnostic at line 253, all 27,782 trace events and all 27,855 normalized output lines. The earlier fix for this `log3` divergence is GoIvy commit `6f5c188a6`. [Mutations and results](perf/ivy_raft/frontier-4f0c0457b/protocol-mutations.csv) · [Mutation generator](perf/ivy_raft/mutations.py)

In a separate check with premise-sampling seed offset 882,000,000, I deleted, shuffled or negated assertions in both Raft corpora. The direct ground-instance proof accepted **1,414 of 8,380** resulting inputs. The independent native solver confirmed every one as unsatisfiable. It also found **5,793 satisfiable** inputs, and g3 declined the direct proof on all of them. The bounded native run returned unknown on 151 other inputs and aborted on one. g3 declined all 152. At mixed-width obligation 349 the first validation run exhausted its confirmation timeout. The original query returned native UNSAT with three independently timed native configurations, including the default search, and the entire mixed-width validation then completed with a 90-second confirmation allowance. The other 10,000 known-model inputs used seeds starting at 9,200,000, with 20,000 contradictory companions and 19 pairs checked by native Z3. [Per-query perturbation results](perf/ivy_raft/frontier-4f0c0457b/formula-perturbations.csv.gz) · [Perturbation driver](perf/ivy_raft/instances/perturb.py) · [Known-model results](perf/ivy_raft/frontier-4f0c0457b/known-models.csv) · [Known-model driver](perf/ivy_raft/onebit/batch/check.py)

I retried the 152 inconclusive native controls with a different seed, the native SMT tactic and longer time limits. Native resolved another 78 as SAT and 38 as UNSAT. Thirty-five remained unknown and one native process aborted. The aggregate evidence therefore includes 5,871 confirmed satisfiable inputs on which g3 declined a direct proof. [Native results for each retry](perf/ivy_raft/frontier-4f0c0457b/native-control-retries.csv.gz)

The pure-Go GoIvy checker also cross-compiled for Linux/arm64 through the local replacement. All reported execution measurements are from Linux/amd64. An optional 386 build exposed an existing untyped `math.MaxUint32` comparison that overflows `int` in the AST abstractor, also present at the archived baseline. [Integer-size comparison](ast/ast_expr_abstract.go)

The final Go test sweep used `-count=1` across 112 packages. Tests passed in 107 packages and five packages had no test files. The sweep excluded `portcheck` as requested. Focused proof and AST tests also passed with `g3checked,g3debug,g3trace`, and `go vet` passed for the API, AST, rewriter, arena and SMT core. Reapplying the shifter instrumentation reproduced all 28 measured counters in both phases of every obligation and all 18 reference search counters.

### Assigning the proof corpus to independent solvers

The final solver checked the 419 captured mixed-width queries in **7.63 s with one worker and 1.84 s with 16** on 12 Go CPUs. The one-bit times were **2.11 s and 0.46 s**. These medians cover nine runs per configuration and include SMT-LIB parsing and cleanup. The Ivy frontend is outside this benchmark. Each worker creates its own context and keeps its solver and proof batch on its goroutine. The separate resident-worker benchmark measures what 2,048 live solvers cost. [Final worker measurements](perf/ivy_raft/frontier-4f0c0457b/workers.csv.gz) · [Corpus benchmark](api/api_raft_boolean_benchmark_test.go) · [Resident-worker measurements](#running-2048-resident-solvers)

The partition benchmark assigns the same 419 inputs once each, using 16 owners throughout. Alternating blocks of 16 gives several workers expensive obligations while preserving some adjacent premise reuse. The table gives median times for equal contiguous ranges and for round-robin blocks of 16.

| Available Go CPUs | Mixed contiguous | Mixed blocks of 16 | One-bit contiguous | One-bit blocks of 16 |
|---|---:|---:|---:|---:|
| 1, CPU 43 | 10.56 s | 10.85 s | 3.17 s | 3.24 s |
| 4, CPUs 12–15 | 2.35 s | 2.56 s | 0.734 s | 0.796 s |
| 12, CPUs 0–11 | 1.68 s | 1.56 s | 0.435 s | 0.372 s |
| 12, CPUs 24–35 | 1.77 s | 1.64 s | 0.460 s | 0.393 s |

On the first 12-CPU set, single-query round-robin raised total allocation across owners from 1.207 to 1.569 GB on mixed widths and from 0.407 to 0.584 GB on one-bit. Blocks of 16 used 1.241 and 0.423 GB respectively. Both 12-CPU sets favored these larger blocks. The one- and four-CPU controls favored contiguous assignment in median elapsed time. All **420 runs** completed all 419 proofs. These are separate process series on a shared host. To repeat, run `go test ./api -run '^$' -bench '^BenchmarkRaftWorkerPartitions$' -benchtime=1x -cpu=1,4,12`. [Partition benchmark](api/api_raft_worker_partition_benchmark_test.go) · [Five schedules including input-byte balancing](perf/ivy_raft/frontier-4f0c0457b/worker-partitions.csv.gz)

### Priorities after Raft parity

1. **Reduce the work in the 185 residual order proofs.** Start with obligations 321 and 251. They concentrate decisions and quantifier instantiations respectively, and the five slowest checks consume 43% of the time inside proof attempts. A new directly checkable contradiction that applies to one of these queries can eliminate both translation loading and search. Use the saved concrete SAT inputs and the 28 protocols with real failures to check each additional proof rule. [Live search work](perf/ivy_raft/frontier-4f0c0457b/auxiliary-search.csv.gz) · [Soundness controls](perf/ivy_raft/frontier-4f0c0457b/formula-perturbations.csv.gz)
2. **Reduce repeated application construction in general SMT search.** Within the matched kernel, 1.09 million binary and ternary application requests reached registration with a structurally identical node already present. A short-application path that preserves handle generations, budgeting, references and traversal order has a much larger target than the remaining variable hash calculation. The existing paged arena kept live heap lower in the multi-owner trials. Use the 18 per-query search counters to detect changes in proof work alongside instructions, wall time and worker memory. [Interning counts](perf/ivy_raft/ast_lookups/interned-nodes.csv.gz) · [Direct-pointer experiment](#direct-ast-slot-pointers-and-physical-node-placement)
3. **Distribute complete, independently checkable obligations when the frontend can supply them.** At 12 Go CPUs the 16-owner replay delivered a fourfold reduction over one owner, including parsing and cleanup. Contiguous groups preserve premise reuse and blocks of 16 divided the expensive checks more evenly on both measured 12-CPU sets. The worker benchmark can be run unchanged on the remote boxes to choose a partition size for their CPU and memory limits. [Measured schedules](perf/ivy_raft/frontier-4f0c0457b/worker-partitions.csv.gz)

## Ground instances of induction hypotheses

On the original Raft input, a counter on each materialized quantifier instance identified the six-variable `log_matching` invariant as the largest contributor to both selected obligations. It produced 4,595 instances at 251 and 881 at 321. Adding the counter reproduced the baseline search counters on every obligation. The property being checked describes the new system state, while the universally quantified induction hypothesis describes the old state. Their structure often identifies the ground arguments needed in a useful instance of the hypothesis.

g3 now looks for this correspondence when an order proof reaches a negated ground implication. It considers universal implications with at least four bound variables and requires consistent variable bindings, matching sorts, and a one-to-one correspondence between function symbols. An instance must bind every variable and replace fewer distinct function symbols than it binds variables. The matcher spends at most 4,096 search steps and adds at most one lemma. The correspondence chooses the ground arguments. The lemma itself retains every original function symbol. The private solver must still return UNSAT to establish the proof. [Implementation](api/internal/order/induction_instance.go) · [Satisfiable and malformed-match controls](api/api_order_instance_test.go)

| Original mixed-width Raft | Decisions before | Decisions after | Instantiations before | Instantiations after |
|---|---:|---:|---:|---:|
| 251, `log_matching` | 1,264 | **846** | 9,198 | **4,565** |
| 321, `choosable_vote_msg` | 4,012 | **1,109** | 4,676 | **1,106** |
| All 185 private order checks | 16,900 | **13,096** | 34,716 | **25,612** |

In 22 alternating pairs on CPUs 4 and 10 of the shared Threadripper host, the ordinary release build took a median of 5.70 s against 6.67 s for the previous g3 binary. The new build won all 22 pairs, with a median within-pair speedup of 1.18×. Both used GoIvy `ee84dd456`, one Go CPU, the local g3 replacement, and identical 419-result transcripts. [Per-process timings](perf/ivy_raft/induction/paired.csv) · [Per-obligation search counts](perf/ivy_raft/induction/search.csv.gz)

Of the 185 residual queries, 54 received a lemma. For each, an independent checker reconstructed a substitution into one original universal premise. Native Z3 proved that the premise alone entails the emitted lemma, with the goal and every other premise removed. The 28 modified Raft specifications agreed with both native reference frontends on 11,732 outcomes, including all 77 expected failures. To repeat the independent lemma checks from `pkg`, run `python3 perf/ivy_raft/induction/verify.py --native ../build/z3`. [Archived assertions](perf/ivy_raft/induction/final-assertions.csv.gz) · [Verifier](perf/ivy_raft/induction/verify.py) · [Negative protocol controls](perf/ivy_raft/induction/protocol-mutations.csv)

## Apple ordering and liveness

The input was `ivy-lang-examples/doc/examples/apple/ord_live.ivy`, last changed in Ivy commit `40fdb8ac3`. Each backend used the same GoIvy frontend with tracing compiled out, `GOMAXPROCS=1`, and one pinned logical CPU on a Ryzen Threadripper 3960X. We alternated the order of the two binaries on each CPU. On `isolate=dramc_nb2`, g3 beat native Z3 in all six pairs. All twelve runs exited successfully and produced byte-identical complete output, with 2,479 passes and a final `OK`. [Paired runs](perf/ivy_apple/dramc_nb2-backends.csv) · [Reproduction driver](perf/ivy_apple/compare.py)

| `dramc_nb2` | GoIvy + g3 | GoIvy + native Z3 |
|---|---:|---:|
| Median whole process, six runs | 173.79 s | 224.89 s |
| Total time in verification checks, separate instrumented run | 6.93 s | 3.05 s |
| Longest check, number 71 | 1.91 s | 1.75 s |

The Python/Go golden test reads one trace record from each child before advancing. The live comparison continued to advance while it appeared stalled. Two independent trace-enabled Go builds, before and after the frontend changes below, produced identical streams from start to finish. Each emitted 8,599,630 trace records and 21,638,409,694 bytes of output. That run compared 8,622,828 lines, including ordinary checker output, in 481 s. This measures two Go processes and their streaming comparator. The Python/Go golden test has its own elapsed time. [Golden reader](https://github.com/glycerine/ivy/blob/f9b70a4bc/goivy/parser_golden_test.go#L985)

The baseline Go CPU profile contained 160.28 sampled seconds. Of those, 40.04 s were spent collecting names in `usedSymbolNamesClauses`, and 20.82 s comparing actions during graph name lookup. Both the sequence and environment action equality methods serialized their entire subtree. Their canonical forms always begin with distinct tags, so GoIvy can answer this particular comparison immediately. With symbol handling held fixed across the pair, the change reduced median time from 136.80 s to 98.81 s and won all six end-to-end pairs on two CPUs. On a 128-statement input a single such comparison went from about 186 µs and 2,074 allocations to 5.82 ns and zero allocations. GoIvy commit `f9b70a4bc` includes the implementation, regression and benchmark. [Complete isolate runs](perf/ivy_apple/action-lookup.csv) · [Microbenchmark samples](perf/ivy_apple/action-lookup-micro.csv)

**For improving the solver, use the sibling isolate `cf_live`.** Both backends passed its 336 checks with identical output. On the same logical CPU, separate instrumented runs spent 27.62 process CPU seconds across verification checks in g3 and 6.04 in native Z3. Check 130 alone consumed 12.02 and 2.19 CPU seconds respectively. For `dramc_nb2`, all 2,479 attempts at the optional order abstraction declined and the concrete solver decided them. Timings of individual Apple checks depend on the incremental solver state. [Measured check times and clock types](perf/ivy_apple/solver-times.csv)

Run the comparison in a new output directory. Supply a standard GoIvy binary and one built with `-tags=classic_z3,xtrace_off`, linked to the native fork. Build the Go backend with `-tags=xtrace_off`. The driver keeps both outputs, checks every byte and reports wall time, CPU time, peak RSS and pass count.

```sh
python3 perf/ivy_apple/compare.py --spec ~/ivy/ivy-lang-examples/doc/examples/apple/ord_live.ivy \
  --isolate dramc_nb2 --cpu 5 --pairs 3 --output /tmp/ord-live-pairs \
  /path/to/goivy-g3 /path/to/goivy-native-z3
```

## Point updates in Raft transitions

On obligation 251 the two log witnesses can exchange places in the goal. Both are fresh constants. The induction hypothesis at those arguments describes the old logs, and the transition definitions show that writes to either log occur only at one named log. If both witnesses differed from the written log, the pre- and postconditions would agree. In any counterexample at least one witness therefore denotes that log. Exchanging the two witness interpretations preserves every premise and the ground goal, so the solver can use the written log for the second witness.

The proof also applies to one witness without a symmetry check. The implementation looks for explicit universal point updates and verifies every surviving arm of the conditional. It requires a structurally identical read of the old function outside the write. Multiple private witnesses require equal sorts and exchange symmetry in both ground formulas. The transformed formula lives only in the private proof solver. The procedure found 18 single-witness and five symmetric specializations among the 185 residual Raft queries.

| Mixed-width Raft search | Before | After |
|---|---:|---:|
| Obligation 251 decisions | 846 | 288 |
| Obligation 251 instantiations | 4,565 | 1,872 |
| Obligation 251 conflicts | 196 | 115 |
| Obligation 251 resource steps | 229,413 | 58,060 |
| All residual decisions | 13,096 | 12,184 |
| All residual instantiations | 25,612 | 22,043 |
| All residual conflicts | 1,989 | 1,871 |
| All residual resource steps | 3,538,863 | 3,190,807 |

Obligation 321 still used 1,109 decisions and 1,106 instantiations. Its change reaches a derived predicate containing an existential and a universal quantifier. The explicit point-update test declined it. [Per-obligation counters](perf/ivy_raft/transition/search.csv.gz)

The complete checker won all eight pairs on each of two cores. Medians went from 5.56 to 5.30 s on CPU 15 and from 11.44 to 10.96 s on CPU 3, which shared CPU time during this run. The median paired reductions were 5.5% elapsed and 5.4% user CPU. The original and specialized builds used the same GoIvy frontend in each pair. All 419 ordered outcomes matched. Across 28 protocol mutations, all 11,732 outcomes matched the retained independent references, including 77 intentional failures. [Timings](perf/ivy_raft/transition/paired.csv) · [Mutation results](perf/ivy_raft/transition/protocol-mutations.csv)

A separate Python checker reconstructed the ground equations directly from the original universal definitions. Native Z3 checked that the old and new property cannot disagree when every candidate witness differs from the write location. For the two-witness cases it also checked exchange symmetry using only declarations, and the verifier checked freshness against every original premise. These checks exclude the rest of the protocol so unrelated contradictions cannot hide an invalid specialization. Reproduce with `python3 -B perf/ivy_raft/transition/verify.py --native ../build/z3`. [Verifier](perf/ivy_raft/transition/verify.py) · [Actual final assertions](perf/ivy_raft/transition/final-assertions.csv.gz)

## Search divergence in the Apple liveness kernel

The standalone `cf_live` condition 130 supplied a reproducible first difference. The initial 2,589 E-matches agreed in identity and order. For a particular pair of bindings, the native quantifier checker recognized an already-false equality between the zero clock value and `cfabric.t_rd_min`. g3 had the same enodes and assignment. It consulted the disequality trail, which had yet to record that fact. g3 instantiated a quantifier that native Z3 skipped, creating different terms before the first decision. A diagnostic build of each kernel traced the recursive checks for that same binding and isolated the difference to this equality.

The checker now consults the congruence table for the equality and reads the Boolean class representative. The test covers both operand orders, a false assignment before propagation, relevancy, and scope rollback. All four captured Apple conditions now match native decisions, conflicts, quantifier instantiations and resource counts. The existing 419-obligation kernel replay also matched all 18 counters on both engines. [Checker and regression](smtcore/smt_checker_test.go) · [Standalone reproducer](perf/ivy_apple/kernel.py)

| Captured condition | Decisions before → after | Instances before → after | Median g3 wall before → after | Native wall |
|---|---:|---:|---:|---:|
| `cf_live` 130 | 51,074 → 5,605 | 50,443 → 18,948 | 3.00 → 0.94 s | 0.28 s |
| `dramc_nb2` 71 | 457 → 457 | 30,704 → 30,704 | 1.40 → 1.41 s | 0.36 s |
| `dramc_nb2` 63 | 2,750 → 2,750 | 78,892 → 78,892 | 3.37 → 3.11 s | 1.11 s |
| `dramc_nb2` 369 | 2,014 → 5,571 | 6,435 → 8,553 | 0.29 → 0.39 s | 0.11 s |

Each row uses nine rotated runs of all three binaries on CPU 20. The change slowed condition 369, where the previous search found a shorter proof. On condition 130 instructions fell from 18.62 billion to 6.46 billion. Native Z3 retired 1.54 billion. With matched search, g3 still retired 4.2–4.5 times the native instructions on these four conditions. On condition 71, sampled g3 stacks charged about 69% of CPU to quantifier instantiation, including rewriting, substitution and internalization. This makes cost per ground instance a directly measurable target. [Raw timings, instructions and counters](perf/ivy_apple/kernel-before-after.csv)

The live incremental `cf_live` condition 130 retained solver history and showed a smaller reduction, from 424,530 decisions and 71,360 instances to 310,951 and 55,679. An independent observer in the native comparison binary measured 248,065 decisions and 49,969 instances. The remaining live difference requires comparing the incremental histories. The captured SMT-LIB files reproduce the standalone results. To measure on another host, build `./cmd/g3` and this fork's native `z3`, and run this command.

```sh
python3 -B perf/ivy_apple/kernel.py --cpu 5 --rounds 9 /path/to/g3 /path/to/z3
```

GoIvy commit `916881ab7` also delays collecting destination symbols until the first local skolem actually requires renaming. With the action lookup improvement in both builds, `dramc_nb2` median elapsed time went from 154.92 to 100.63 s across eight paired runs, and median process CPU went from 149.97 to 100.50 s. All 2,479 checks agreed. The full traced run matched the original 8,599,630 records, 8,622,828 output lines and 21,638,409,694 bytes. On a 128-symbol destination the no-skolem benchmark went from 108,816 B and 660 allocations per call to 416 B and three allocations. [Complete runs](perf/ivy_apple/skolem-renaming.csv) · [Microbenchmark](perf/ivy_apple/skolem-renaming-micro.csv)

### The 102-minute golden run and all nine Apple isolates

The live `TestGoldenAll` log brackets `dramc_nb2` between 23:39:40.362 and 01:22:08.865 UTC on November 20–21, 2026. The test matched 8,599,630 trace events. Python Ivy with native Z3 and GoIvy with g3 each printed 2,479 passes and a final `OK`. The two child exit timestamps were 1.23 s apart, and the test advanced to `e2e`. Both producers feed bounded pipes and the parent compares one event from each before advancing. The **102 min 28 s** measures this joint run. To measure a trace producer independently, the standalone driver drains its output, keeps ordinary checker messages, and records elapsed time, child CPU time, trace bytes, event count and progress. [Golden comparison loop](https://github.com/glycerine/ivy/blob/916881ab7/goivy/parser_golden_test.go#L985) · [Independent trace driver](perf/ivy_apple/trace_single.py)

The independent Python Ivy run with native Z3 finished in **6,561.40 s (109 min 21 s)**. The initial standalone GoIvy runs took **515.94 s with g3** and **356.76 s with native Z3**. All three emitted 8,599,630 trace records and 2,479 passes. The ordinary checker output agreed byte for byte after normalizing the example directory in Python’s source locations. In a further 14 rotated standalone runs on CPUs 4 and 9, the current GoIvy frontend with g3 had a median of **295.36 s** (four runs), and the same frontend with native Z3 had a median of **391.51 s** (three runs). The installed older GoIvy/g3 checker had a median of 363.00 s (three runs). With native Z3, restoring the earlier action-lookup and transition code gave 505.47 s (four runs). These timings include frontend processing and draining more than 21 GB of trace output per execution. They explain how a GoIvy process can stay alive for over 100 minutes while the paired golden checker waits on Python. [All 17 runs and output counts](perf/ivy_apple/dramc_nb2-traced-frontends.csv)

With tracing compiled out, 135 complete runs covered every isolate declared by the same Apple specification. All three binaries used GoIvy `916881ab7`, one pinned CPU per isolate, and `GOMAXPROCS=1`. Each round rotated the order of the binaries. Every execution exited successfully and matched its isolate’s complete reference output byte for byte. The table gives median whole-process elapsed seconds. Concurrent work on the host varied by core, so compare engines within each row. [All samples including CPU time and RSS](perf/ivy_apple/isolate-backends.csv) · [Two-backend reproduction driver](perf/ivy_apple/compare.py)

| Isolate | Checks | Runs per engine | g3 before equality lookup | g3 at `31c87480f` | Native Z3 |
|---|---:|---:|---:|---:|---:|
| `dramc_nb2` | 2,479 | 3 | 92.38 | **93.01** | 99.86 |
| `cf_live` | 336 | 9 | 48.50 | **38.52** | 21.27 |
| `rfn` | 33 | 9 | 17.63 | **22.63** | 12.36 |
| `rfn2` | 26 | 9 | 35.14 | **25.32** | 16.04 |
| `cmpl_live` | 437 | 3 | 32.80 | **26.78** | 25.10 |
| `cf_pio_live` | 2,215 | 3 | 113.42 | **117.39** | 117.35 |
| `e2e` | 2,378 | 3 | 94.23 | **99.98** | 91.38 |
| `sys_live` | 1,738 | 3 | 79.40 | **79.49** | 72.89 |
| `tprops` | 700 | 3 | 37.18 | **35.27** | 35.61 |

The largest median gaps to native Z3 in this panel occur on `cf_live` (17.25 s) and `rfn` (10.27 s). The equality lookup improved the first and lengthened the proof search on the second. Keep both as end-to-end controls for changes to quantifier matching, substitution and incremental search. `dramc_nb2` remains the frontend and trace-throughput control. In the earlier instrumented `dramc_nb2` run, the actual solver checks took 6.93 s in g3 and 3.05 s in native Z3. [Solver-call measurements](perf/ivy_apple/solver-times.csv)

## Unchanged log entries in Raft obligation 251

Obligation 251 compares entries in two logs after a point update. The previous specialization established that one log in a counterexample may be chosen as the updated log. If the other log had that same identity, both equalities in the conclusion would compare an expression with itself. The counterexample therefore uses two different logs. At the other log, the update preserves both the entry predicate and the entry value. g3 now gives these two equalities and the disequality to the private order solver. It checks the shape of the conclusion, the individual update definitions, and the arguments at each access. The same reasoning applies to obligations 113 and 205. [Implementation](api/internal/order/transition_unchanged.go) · [Changed ground assertions](perf/ivy_raft/unchanged/final-assertions.csv)

| Search work | Decisions before → after | Quantifier instances before → after | Conflicts before → after | Resource counts before → after |
|---|---:|---:|---:|---:|
| Obligation 251 | 288 → **193** | 1,872 → **921** | 115 → **72** | 58,060 → **36,891** |
| All 185 private checks | 12,184 → **11,843** | 22,043 → **20,603** | 1,871 → **1,805** | 3,190,807 → **3,105,501** |

In 24 alternating pairs of full 419-check runs on CPUs 8 and 13, median elapsed time went from 5.13 s to 5.01 s. The candidate won 21 pairs. The median within-pair reduction was 1.2% in elapsed time and 1.4% in user CPU time. Both builds used GoIvy `f3dee089f` with one Go CPU and identical output. Across 28 mutated protocols, the new build agreed with both native frontends on all 11,732 outcomes, including 77 failures. [Paired timings](perf/ivy_raft/unchanged/paired.csv) · [Changed search counters](perf/ivy_raft/unchanged/search.csv) · [Mutation runs](perf/ivy_raft/unchanged/protocol-mutations.csv)

A separate native verifier uses the counterexample goal and ground defining equations at the two accesses. It first confirms these assumptions are jointly satisfiable. Native Z3 then refutes the negation of each of the nine new facts individually, with the universal induction hypothesis and other protocol premises absent. Run `python3 -B perf/ivy_raft/unchanged/verify.py --native ../build/z3` from `pkg`. [Verifier](perf/ivy_raft/unchanged/verify.py)

### Enumeration ownership and incremental search

On the complete incremental `cf_live` run, the first 167,963 normalized trace events agreed. The first difference was the ID assigned to the same new disjunction. Native Z3 reused an older ID after its theory-rewriter cache evicted a term. g3’s cache started 25 entries larger. A census at cache creation found the same 5,146 applications, 550 quantifiers, 45 variables and 86 sorts. GoIvy had also left 25 unused enumeration tester declarations in the g3 AST table. The native reference-counted API releases these temporary declarations after constructing the enumeration values. GoIvy now holds the testers across value construction and releases them in declaration order. Adjusting only the cache size failed to reproduce the native search. [GoIvy fix and regression test](https://github.com/glycerine/ivy/commit/f3dee089f)

With g3 at `31c87480f`, this adapter change raised the number of Apple checks matching native Z3 on **all 18 recorded search counters from 10,140 to 10,311 of 10,342**. All 336 `cf_live` checks now match. On incremental check 130, decisions fell from 310,951 to **248,065**, quantifier instances from 55,679 to **49,969**, conflicts from 626 to **479**, and resource counts from 10,451,086 to **7,910,191**. Six complete isolates matched every counter, including all 2,479 `dramc_nb2` checks. The 31 remaining divergent checks were in `rfn` (5), `rfn2` (6), and `sys_live` (20). Several searches got longer. For example, `rfn2` made 752,558 decisions after the change, up from 652,108, while native made 805,401. Every solver produced the same complete checker transcript on its isolate. [Per-check statistics for all three builds](perf/ivy_apple/enum-lifetime-search.csv.gz)

To reproduce the comparison, use the GoIvy checkout and its local g3 replacement at the versions being measured. The driver builds release and classic-Z3 variants, observes each live solver before and after every check, and verifies the full transcripts. On the original checkout, an independent rerun of `rfn2` and `cf_live` reproduced all 18 values on all 724 recorded rows. [Driver](perf/ivy_apple/search_counters.py) · [Native observer](../src/test/perf/ivy_solver_statistics.cpp)

```sh
python3 -B perf/ivy_apple/search_counters.py --goivy ~/ivy/goivy --cpu 5 \
  --spec ~/ivy/ivy-lang-examples/doc/examples/apple/ord_live.ivy cf_live rfn2
```

On `dramc_nb2`, independent trace-disabled runs of **Python Ivy with native Z3** took 6,657.22 s with the ordinary interpreter and 3,013.84 s with `python -O`. Their user CPU times were 5,429.88 s and 2,625.15 s. Both transcripts matched GoIvy’s 2,479 results and final `OK` after normalizing the include and specification paths. These measurements put the 102-minute lockstep golden test in context. The live GoIvy solver comparisons and standalone Python timings measure different executables and execution modes.

### Registration order in model-based quantifier instantiation

In the incremental Apple `rfn` trace, the first 6,979,998 normalized events agreed. When the model checker restricted a fresh Skolem constant to its instantiation set, g3 selected `addr_type!val!0` and native Z3 selected `addr_type!val!5` in the same disjunction. Both had constructed the same terms. For an equality on an uninterpreted sort, Z3 visits the relevant context terms in enode registration order. g3 enumerated the AST arena slots. When two terms collide in the instantiation set’s hash table, visitation order affects which value the inverse map encounters first. g3 now reads the internalizer’s existing ordered enode stack. A context that also accepts caller-owned enodes tracks their insertion and removal in that same order. [Model finder](smtcore/smt_model_finder_auf_literals.go) · [Collision and scope regression](smtcore/smt_model_finder_auf_graph_test.go)

This change reproduced three more native searches, bringing the exact matches on all 18 counters to **10,314 of 10,342**. On `rfn`, the first trace difference moved to event 10,878,688, and 31 of 33 checks now match. g3 made 617,667 decisions on that isolate versus 617,670 in Z3. `rfn2` still had six divergent searches and `sys_live` had twenty. All nine checker transcripts agreed, as did the original 419 standalone Raft kernels on all retained search counters. [Per-check candidate counters](perf/ivy_apple/enode-order-search.csv.gz) · [Parent and native reference](perf/ivy_apple/enum-lifetime-search.csv.gz) · [Reproduction driver](perf/ivy_apple/search_counters.py)

| Complete isolate | Alternating pairs | Previous g3 median wall, s | Registration order median wall, s |
|---|---:|---:|---:|
| `rfn` | 10 | 23.75 | 26.66 |
| `rfn2` | 10 | 32.19 | 32.47 |
| `sys_live` | 6 | 85.08 | 80.81 |
| `dramc_nb2` | 4 | 98.92 | 95.01 |

Registration order restores native behavior and removes a temporary slice from the model finder. Its effect on elapsed time varies. On `rfn`, the previous search used 487,858 decisions, and reproducing the native search made g3 **15.3% slower** by median within-pair elapsed time. On `rfn2`, decisions fell from 752,558 to 703,015 and the median paired time changed by 0.3%. This makes the now-matched `rfn` checks suitable for studying the cost of executing the same proof in each implementation. [All 60 timing samples](perf/ivy_apple/enode-order-pairs.csv)

## Checking the update in Raft obligation 321

The pre- and post-state properties read three changed predicates. One is a vote message that can change only at a particular node, term and destination. The other two describe the term bound and whether a log entry is choosable. If those two predicates change only in directions that preserve the property, a counterexample must involve the vote message at the updated tuple. The original point-update rule could establish the vote message’s footprint but could not look through the nested quorum quantifiers.

g3 now proposes the two direction implications from the ground property and tries to refute their negation on a fresh private solver. The check uses a subset of the original premises and at most one eighth of the remaining search allowance, capped at 150,000 resource steps. If the side condition is proved, g3 substitutes the verified update coordinates into the ground counterexample. Otherwise it checks the original problem. Raft obligations 321 and 413 satisfied their conditions. Obligation 367 had a genuine countermodel to its condition and used the ordinary proof. [Candidate construction](api/internal/order/transition_case.go) · [Captured conditions, original inputs and specialized assertions](perf/ivy_raft/conditional/cases.json.gz)

| Search work | Decisions | Quantifier instances | Conflicts | Resource counts |
|---|---:|---:|---:|---:|
| 321 before | 1,109 | 1,106 | 55 | 198,951 |
| 321 side condition | 119 | 53 | 11 | 10,337 |
| 321 specialized query | 228 | 383 | 19 | 76,624 |
| **321 combined** | **347** | **436** | **30** | **86,961** |
| All 185 residual queries before | 11,843 | 20,603 | 1,805 | 3,105,501 |
| **All queries including side checks** | **10,788** | **19,372** | **1,757** | **2,904,525** |

A first experiment used all original premises for the side proof. Including both solver calls, it used 390 decisions and 519 instantiations on 321. Selecting the ground clauses, definitions and short related axioms reduced the cost of the side proof. The older single-query disjunction formulations generally took as much search as the original query. The standalone statistics driver collects the actual incremental calls and their before/after counters. [Per-obligation search deltas](perf/ivy_raft/conditional/search.csv) · [Measurement driver](perf/ivy_raft/conditional/search.py)

On the ordinary 419-check Raft specification, the production candidate won all 32 alternating pairs on CPUs 4 and 9. Median wall time went from 4.73 s to 4.58 s. Median paired reductions were **3.4% elapsed and 3.6% user CPU**. The parent build was g3 `7dbe1e9d4`. Both used GoIvy `f3dee089f` with one Go CPU. All 11,732 results on 28 mutated protocols agreed with both reference frontends, including 77 intentional failures. Native Z3 also verified each side condition or counterexample and independently proved both accepted substitutions using satisfiable local assumptions. [Paired timings](perf/ivy_raft/conditional/paired.csv) · [Mutation controls](perf/ivy_raft/conditional/protocol-mutations.csv) · [Independent verifier](perf/ivy_raft/conditional/verify.py)

```sh
python3 -B perf/ivy_raft/conditional/verify.py --native ../build/z3
python3 -B perf/ivy_raft/conditional/search.py --goivy ~/ivy/goivy --cpu 4 \
  --spec ~/ivy/ivy-lang-examples/examples/raft/raft_no_assume_test.ivy
```

## AST lifetime during nested quantifier abstraction

On Apple `sys_live`, the first difference in the solver traces was a bound variable’s AST ID. Both solvers created a temporary variable while abstracting a nested quantifier. Z3 released it when the recursive call returned and reused its ID in the next sibling. g3 held the variable through the outermost call. Each recursive abstraction now releases its own temporary references after its caller has acquired the result. In the new focused test, the previous implementation gives the sibling ID 9 and the native lifetime gives it ID 8. [Implementation](ast/ast_expr_abstract.go) · [Regression test](ast/ast_expr_abstract_test.go)

This change extended the equal prefix of the normalized `sys_live` trace from 706,151 to **2,863,954 events**. All 18 search counters stayed unchanged on every one of the 10,342 Apple checks. Timing was mixed. Across 20 alternating pairs, median paired elapsed time improved 2.6% on `rfn` and 1.6% on `rfn2`, and increased 5.9% on `sys_live` and 2.9% on `dramc_nb2`. We retained the lifetime correction for trace fidelity. The comparison used the same GoIvy and g3 code with only the abstraction implementation varied. [All 40 runs](perf/ivy_apple/nested-abstraction-pairs.csv)

## Why the two solvers chose opposite substitutions in Apple `rfn2`

At the first differing substitution both terms were uninterpreted constants of depth one. Both implementations broke the tie by declaration ID. The newly declared `__new_prm:M` had ID 2147484579 in g3 and 2147484228 in Z3. The other constant had ID 2147484262 in both. The difference arose six checks earlier. A congruence-table lookup for an unseen declaration registers the declaration in Z3, takes a reference, and can still return an empty result. g3 returned immediately on a miss. This changed the table order when the solver reset, the release order of three datatype recognizers, and which declaration IDs the AST manager recycled. Both lookup entry points now establish the table on first use. The regression test checks the references and registration order before and after reset. [Congruence table](smtcore/smt_cg_table.go) · [Test](smtcore/smt_cg_table_test.go)

Matching the registration lifetime moved `rfn2` from 20 to **26 of 26** identical checks across 18 search counters, bringing the nine-isolate total to **10,320 of 10,342**. Decisions in `rfn2` rose from 703,015 to the native 805,401. Its other counters include 270,033 quantifier instantiations and 2,121 conflicts in each engine. All 419 Raft kernel controls also matched. [Per-check measurements](perf/ivy_apple/register-on-lookup-search.csv.gz)

Over eight alternating pairs on `rfn2`, the change reduced median paired elapsed time by 0.9%. The whole-isolate gap to Z3 remained. In six separate alternating pairs, median elapsed times were 27.76 s in g3 and 15.07 s in native Z3. The individual solver calls took 21.77 s and 9.66 s of process CPU in the instrumented run. Four-pair controls on `cf_live` and `rfn` showed increases of 3.7% and 3.8% in median paired user CPU respectively. Samples were taken on a shared host. [All 44 timing samples](perf/ivy_apple/register-on-lookup-pairs.csv)

## Apple ordering now matches native incremental search on all nine isolates

The first remaining difference on `rfn` occurred at trace event 10,878,688. The quantifier checker examined terms 60685 and 31684 while deciding whether a candidate instance was already satisfied. A false equality had been recorded on their argument classes. The congruent equality for this lookup had an inactive Boolean literal. g3 accepted the earlier disequality as sufficient evidence and skipped the instance. Z3 looks up the congruent equality, follows its Boolean class root, and accepts either the distinguished false literal or a relevant literal assigned false. The integrated checker now makes the same decision. The focused test reproduces the earlier answer by marking the equality inactive after propagating the disequality. [g3 checker](smtcore/smt_checker.go) · [C++ implementation](../src/smt/smt_context.cpp) · [Regression test](smtcore/smt_checker_test.go)

All 18 counters now agree on **10,342 of 10,342** incremental checks. All nine ordered output transcripts match and the 419 Raft kernel controls agree. The two previously divergent isolates had these aggregate counts. [Per-check measurements](perf/ivy_apple/congruent-equality-search.csv.gz)

| Isolate | Exact checks before → after | Decisions before → after | Instantiations before → after |
|---|---:|---:|---:|
| `rfn` | 31 → 33 of 33 | 617,667 → 617,670 | 244,513 → 244,505 |
| `sys_live` | 1,718 → 1,738 of 1,738 | 789,825 → 652,725 | 456,515 → 378,332 |

For the three isolates with earlier trace differences, both engines flushed their trace through the final check. The comparison matched each event's terms and IDs and verified that each instance referenced the corresponding E-match in both allocators. It allowed whitespace and differences in the printed fingerprint addresses. [Trace comparator](perf/ivy_apple/compare_traces.py)

| Isolate | Matching events | E-matches | Instance references checked |
|---|---:|---:|---:|
| `rfn` | 13,886,335 | 1,073,860 | 266,793 |
| `rfn2` | 15,098,872 | 1,325,930 | 295,786 |
| `sys_live` | 11,093,797 | 1,431,309 | 442,293 |

Across eight alternating pairs, `sys_live` used 5.6% less median paired user CPU and finished 5.1% sooner. On `rfn`, median paired CPU increased 5.7% over eight pairs. On `dramc_nb2` it increased 4.6% over four pairs, with identical search counts. Four separate `sys_live` pairs against native measured median elapsed times of 82.48 s for GoIvy/g3 and 77.98 s for GoIvy/Z3. These are whole-isolate timings on a shared machine. With equal search events on the profiled isolates, CPU profiles can now isolate implementation costs without simultaneously changing the search trajectory. [All 48 timing samples](perf/ivy_apple/congruent-equality-pairs.csv)

The independent scope-restoration control used six interleaved runs each of `make bench` and `make cppbench` on one CPU, 10,000 iterations per case. Surviving atoms took a median **343 ns/op in g3 and 479 ns/op in Z3**. Scoped atoms took **15,221 ns/op and 12,567 ns/op** respectively, a remaining 1.21× ratio. Both Go cases reported zero allocations per operation in every run. [Raw measurements](perf/ivy_apple/scope-restoration-control.csv) · [Go benchmark](smtcore/smt_hotpath_benchmark_test.go)

## One pointwise implication discharges the quantified Raft side condition

The expensive side condition on Raft 321 asked g3 to compare two quorum predicates at a specific witness and two log predicates at another witness. The quorum definitions contain the same existential and universal binders. Their bodies differ only in a negated application of the log predicate. Proving that the old log predicate implies the updated one at every argument therefore gives both required ground implications. The private solver checks that universal inclusion using the transition premises. The matcher compares binder positions, sorts, fixed arguments and polarity. A mismatched argument or polarity retains the previous side condition. Obligation 367 retains its satisfiable side condition and takes the ordinary fallback. [Implementation](api/internal/order/transition_predicate.go) · [Polarity and binding tests](api/api_order_instance_test.go)

| Raft obligation | Side decisions | Side instantiations | Total decisions | Total instantiations |
|---|---:|---:|---:|---:|
| 321 | 119 → 33 | 53 → 18 | 347 → 261 | 436 → 401 |
| 413 | 113 → 23 | 54 → 16 | 129 → 39 | 115 → 77 |

Only these two side searches changed among the 188 private solver calls. Across the 419-obligation run, private decisions fell from 10,788 to 10,612 and instantiations from 19,372 to 19,299. The main search on 321 still took 228 decisions and 383 instantiations. Native Z3 verified both implications using only the two wrapper definitions and the candidate inclusion. Dropping the inclusion yields a satisfiable counterexample in each control. All 11,732 named outcomes from the 28 protocol mutations agreed with native Z3 and Python Ivy, including 77 deliberate failures. [All solver calls](perf/ivy_raft/conditional/inclusion-search.csv) · [Native checks](perf/ivy_raft/conditional/verify.py) · [Mutation results](perf/ivy_raft/conditional/inclusion-mutations.csv)

In 28 alternating release pairs, median paired wall time improved 0.33% and user CPU improved 0.42%, with 14 wins on each metric. A separate 16-pair hardware-counter run measured 0.33% fewer median paired retired instructions, with 15 wins. Median wall time in that run increased 0.33%. The retained benefit is the reproducible search and instruction reduction. Whole-protocol elapsed time remains within the variation of these runs. [Release pairs](perf/ivy_raft/conditional/inclusion-paired.csv) · [Hardware counters](perf/ivy_raft/conditional/inclusion-pmu.csv)

### A compiled substitution plan across the nine Apple isolates

At the enumeration-lifetime checkpoint, a prototype compiled quantifier bodies into a list of variable reads and application reconstructions. The four-build comparison rotated the original GoIvy/g3 build, the enumeration fix, the fix with the prototype, and GoIvy/native Z3. All 192 runs reproduced the complete native transcript for their isolate. This comparison predates the later changes that brought every search counter into agreement. [All measurements](perf/ivy_apple/enumeration-substitution-panel.csv) · [Benchmark prototype against g3 `31c87480f`](perf/ivy_apple/ground-substitution-experiment.patch)

Against the enumeration fix alone, the prototype reduced median paired user CPU by 7.7% on `rfn2`, 5.4% on `cmpl_live`, and 1.7% on `cf_live`. It increased CPU by 9.2% on `sys_live`, 8.0% on `e2e`, 4.9% on `rfn`, and 2.0% on `dramc_nb2`. The measurements used eight rounds each on `rfn2`, `rfn` and `cf_live` and four on each other isolate. The implementation remains a benchmark patch. The largest remaining difference in this panel was `rfn2`, where the enumeration build took a median 31.56 s and native took 15.74 s. [Paired comparison driver](perf/ivy_apple/compare.py)

### Ground proofs for Raft 251 and 321

Both obligations have short proofs using ordinary ground assertions. For 251, 32 assertions suffice. For the main query of 321, 41 suffice after the usual directional side condition has been proved. Each of the 73 assertions was checked independently by giving native Z3 its original parent assertion and the negation of the proposed ground consequence. In both cases the ground background alone was satisfiable, and adding the goal gave UNSAT. The fixtures preserve the original SMT-LIB queries, the ground terms, and their parent assertion numbers. [Fixtures](perf/ivy_raft/ground_instances/cases.json.gz) · [Independent verifier and benchmark](perf/ivy_raft/ground_instances/bench.py)

| Obligation | Engine | Full query | Ground proof supplied | Declarations only |
|---|---|---:|---:|---:|
| 251 | g3 | 83.75 ms | 30.84 ms | 24.32 ms |
| 251 | Z3 | 23.60 ms | 7.58 ms | 5.92 ms |
| 321 | g3 | 265.76 ms | 28.84 ms | 25.20 ms |
| 321 | Z3 | 71.34 ms | 6.76 ms | 5.77 ms |

These are median elapsed times per check on CPU 14 over 12 rotated rounds, with 25 fresh checks and SMT-LIB resets per process. All 3,600 answers were checked. The declarations-only run measures the cost of setting up the same signature and checking an empty assertion set. Much of g3's elapsed time on a supplied ground proof is already present in that control. The full-query to ground-proof reduction measures the value of knowing the instances in advance. Discovering them takes additional time. [144 timing samples](perf/ivy_raft/ground_instances/standalone.csv)

In the normal 419-obligation incremental GoIvy run, replacing just the two main queries by their verified ground proofs reduced 251 from 193 decisions and 921 instantiations to 168 and zero. The main check of 321 went from 228 decisions and 383 instantiations to five and zero. Both private solvers returned UNSAT. The side condition for 321 still used 33 decisions and 18 instantiations. Simply appending valid instances to the full premises had different effects on the two obligations. It reduced 321's main search to nine decisions and 130 instantiations, but increased 251's instantiations to 1,075. Appending only the instances from large binders made 321 substantially more expensive. The replay driver checks the entire original main query before replacing it. [Incremental replay driver](perf/ivy_raft/ground_instances/replay.py) · [Nine replay configurations](perf/ivy_raft/ground_instances/replay.csv)

A prototype that found instances by testing candidate bindings against successive ground models did discover proofs independently. One configuration required 42 model checks and 655 ground assertions for 251, and another needed 19 checks and 444 assertions for 321. The native ground solver alone spent 456 ms and 197 ms respectively across those calls, excluding the Python instance search. These experiments favor pruning transition cases before quantifier search on 251, and pursuing selective instantiation on 321 where the matched seeds reduced both counters. The precomputed proofs provide a reproducible control for future discovery algorithms. [All 16 exploratory discovery runs](perf/ivy_raft/ground_instances/discovery.csv)

The GoIvy revisions used in the original measurements were rebased on branch `abcopt`. Original `f9b70a4bc`, `916881ab7`, and `f3dee089f` correspond respectively to `7cefc7ccf`, `65110187a`, and `00ed3294a` in the verified rebase. All three patches and the complete GoIvy source tree at the study tip matched. GoIvy's checked-in module at that tip selects g3 v0.1.6. For local reproduction set `GOFLAGS=-modfile=/absolute/path/to/study.mod` for GoIvy commands, using a copy of its module file with a replacement pointing at the g3 checkout. At g3 `cb3a96b96` the rebased GoIvy run reproduced the recorded 188 private solver measurements across all 419 obligations. [Reference search counters](perf/ivy_raft/conditional/inclusion-search.csv)

### A shared transition guard reduces Raft 251's search

In three Raft obligations, four updated functions all kept their old values under the same Boolean choices. The old invariant already holds at the ground arguments used for the counterexample. If all four functions keep their old values, the updated invariant must also hold there. g3 now requires a transition branch that can change a value, and simplifies the four defining equations under that requirement. It recognizes the guard from the input formulas. An argument-dependent condition, a mismatched guard, or an exhausted matching budget keeps the original equations. [Implementation](api/internal/order/transition_guard.go) · [Counterexample controls](api/api_order_instance_test.go)

| Obligation | Decisions | Quantifier instantiations | Conflicts | Resource count |
|---|---:|---:|---:|---:|
| 113 | 74 → 32 | 258 → 83 | 21 → 3 | 11,633 → 5,921 |
| 205 | 50 → 18 | 221 → 105 | 21 → 9 | 10,873 → 6,725 |
| 251 | **193 → 111** | **921 → 548** | **72 → 18** | **36,891 → 25,287** |

All 419 named results agreed with the reference. Only these three private checks changed. For each of them, native Z3 independently proved that the ground counterexample and ten original update instances imply the guard. It also proved each of the four rewritten universal definitions equivalent to its original under that guard, in both directions. Positive controls admitted models. All 28 mutated specifications matched the Python and native references across 11,732 named outcomes, including the same 77 failures. [All search counters](perf/ivy_raft/update_guards/search.csv) · [Independent proof checks](perf/ivy_raft/update_guards/verify.py) · [Mutations](perf/ivy_raft/update_guards/mutations.csv)

Median paired elapsed and user time were unchanged to the timer's resolution over 28 alternating pairs of complete release runs. In a separate 16-pair hardware-counter experiment the candidate retired fewer instructions in every pair, with a median reduction of 0.59%. Median paired elapsed time fell 0.56% in that experiment. The search reduction is substantially larger than its effect on an entire 419-obligation run. [Release pairs](perf/ivy_raft/update_guards/pairs.csv) · [Hardware counters](perf/ivy_raft/update_guards/pmu.csv)


### Partial instances of a universal consequence on Raft 321

The violated conclusion requires a particular log entry to be absent. One universal invariant equates pairs of log entries when their histories agree. The new proof matches the absent entry against each side of that equality. Each match fixes three of the invariant's six variables. The other three stay universally quantified. Once the directional side condition succeeds, the main solver receives both partially instantiated invariants and the checked ground implications. Omitting the two quantified wrapper definitions weakens the primary query, so only an UNSAT answer can close the obligation. An inconclusive result still goes to the concrete solver. [Matcher](api/internal/order/transition_conclusion.go) · [Counterexample regressions](api/api_order_instance_test.go)

| Obligation | Main decisions | Main instantiations | Main resource count | Decisions including side proof | Instantiations including side proof |
|---|---:|---:|---:|---:|---:|
| 321 | **228 → 96** | **383 → 186** | **76,624 → 9,700** | 261 → 129 | 401 → 204 |
| 413 | 16 → 0 | 61 → 6 | 27,430 → 1,143 | 39 → 23 | 77 → 22 |

All other primary and side checks retained their four recorded counters. Raft 367 retained its original primary query after its side condition returned SAT. Native Z3 proved each of the four added formulas from its single original premise and admitted models for that premise alone and for the negated partial instance alone. The normal transcript and all 11,732 mutated outcomes matched the references, including all 77 failures. [All search counters](perf/ivy_raft/conclusion_instances/search.csv) · [Independent native verifier](perf/ivy_raft/conclusion_instances/verify.py) · [Mutation results](perf/ivy_raft/conclusion_instances/mutations.csv)

Both changes mattered on 321. Keeping the wrapper definitions and adding the two instances raised the main query to 863 decisions and 1,349 instantiations. Omitting just the wrappers gave 366 and 570. Omitting the wrappers and supplying both instances gave 96 and 186, lower than supplying either alone. All eight configurations discharged both targeted queries in the private solver. [Eight controlled configurations](perf/ivy_raft/conclusion_instances/ablation.csv) · [Reproduction driver](perf/ivy_raft/conclusion_instances/ablate.py)

Across 28 alternating release pairs, the candidate finished sooner in 25 and improved median paired wall and user time by 2.07% and 2.12%. Across a separate 16 hardware-counter pairs, it retired fewer instructions in every pair, with a median reduction of 1.71%. Wall time fell 1.97% and user time fell 2.11% in that experiment. The largest remaining resource counts on the original specification are 551,472 for obligation 274 and 530,264 for 275. [Release pairs](perf/ivy_raft/conclusion_instances/pairs.csv) · [Hardware-counter pairs](perf/ivy_raft/conclusion_instances/pmu.csv)

### Splitting the matched invariant on Raft 251

The invariant asserts that two logs agree on a Boolean entry and on its source. The prototype emitted one universally quantified implication for each direction of Boolean agreement, plus a third for source equality. It preserved the antecedent and binders in all three rules. Raft 251 went from 111 to 51 decisions, 548 to 471 instantiations and 25,287 to 21,574 resource steps. Two other obligations changed. All 419 original outcomes and all 11,732 mutation outcomes agreed with the reference, including 77 failures. [Prototype patch against `4bf9534f1`](perf/ivy_raft/invariant_directions/split.patch) · [All search counters](perf/ivy_raft/invariant_directions/search.csv) · [Mutation results](perf/ivy_raft/invariant_directions/mutations.csv)

The full run took a median 4.52 s before and 4.50 s after. The paired median change was −0.22% elapsed over 28 alternating pairs on CPUs 7 and 21. In a separate 16-pair hardware-counter run the paired median changes were +0.06% instructions and +1.66% elapsed. I kept the production rule as it was. Reproduce the measurements with the saved patch applied at the repository root, the same local g3 replacement in GoIvy, and the full Raft input. The search driver also runs against the unpatched baseline. [Timing pairs](perf/ivy_raft/invariant_directions/pairs.csv) · [Hardware counters](perf/ivy_raft/invariant_directions/pmu.csv) · [Search driver](perf/ivy_raft/conditional/search.py) · [Timed full-run driver](perf/ivy_raft/measure.py)

### Where the current Raft and scope benchmarks spend CPU

I profiled 24 complete GoIvy runs against g3 `4bf9534f1` and the rebased GoIvy tree at `00ed3294a`. Each run reproduced the native/Python reference transcript for all 419 obligations. Four private proof attempts accounted for **33.67 of 108.45 sampled CPU seconds (31.0%)**. Within those four attempts, **24.96 seconds (74.1%)** were inside the model checker's auxiliary counterexample routine. That routine evaluates a universally quantified body in the candidate model, substitutes fresh constants for its variables and asks a second SMT context whether any assignment violates it. The profile labels cover the private proof attempt and its callees. The whole-process column also includes frontend work and the concrete solver. [Merged CPU profile](perf/ivy_raft/quantifier_profile/raft.pprof) · [Profile driver](perf/ivy_raft/quantifier_profile/profile.py) · [C++ model checker](../src/smt/smt_model_checker.cpp)

| Function | All 419 obligations | Within obligations 92, 134, 274 and 275 |
|---|---:|---:|
| Model checker auxiliary counterexamples | 38.69 s | 24.96 s |
| Internalizing pending assertions | 36.09 s | 8.12 s |
| Rewriting applications | 31.53 s | 12.98 s |
| Loading the next private solver batch | 27.11 s | 0.54 s |
| AST node address lookup, flat CPU | 7.52 s | 3.13 s |

The first four rows include callees and overlap. Two paths warrant separate experiments. On the four expensive checks, reducing the number of counterexample searches reaches a greater share of the measured time than speeding up AST address lookup alone. Across the full workload, loading and internalizing the other obligations also accounts for substantial time. Reproduce with the driver’s `--goivy`, `--spec`, `--reference`, `--cpus 4 10 22`, `--runs 24` and `--output raft.pprof` arguments. Inspect the four attempts with `go tool pprof -top -cum -relative_percentages -tagfocus='^vc:(92|134|274|275)$' raft.pprof`. [Private solver counters for each obligation](perf/ivy_raft/conclusion_instances/search.csv)

A separate single-CPU profile ran 450,000 scoped restorations with the activity queue enabled, 128 atoms and 64 learned clauses. The mean was 20.54 µs inside pop and 88.11 µs for the whole round. Of 9.41 sampled CPU seconds tagged inside pop, 4.38 s were in learned-clause reinitialization and 4.11 s in undoing scoped state. Expression-table reads took 1.00 s flat. Restoring Boolean variables took 1.65 s inclusive. For the scope benchmark, retirement and reconstruction remain the largest measured targets. [Scope CPU profile](perf/ivy_raft/quantifier_profile/scoped-pop.pprof) · [Phase-label patch](perf/scope_restoration/worker_trials/pop-profile.patch) · [Scope benchmark](smtcore/smt_hotpath_benchmark_test.go)

### Discharging a model check when the evaluated body is true

On the private order-abstraction solver, g3 now stops a counterexample search when the candidate model has reduced the entire quantified body to the canonical Boolean `true`. Every binding satisfies that body. g3 can therefore return the same answer it would get by introducing Skolem constants and asking the auxiliary solver to satisfy `not true`. The scope and evaluated expression still unwind through their ordinary cleanup paths. The default solver retains Z3's Skolem creation order. Enabling this shortcut on every solver changed search counters on four Apple isolates. Enabling it only on the private refutation context preserved all 18 counters across all 10,342 Apple checks, including the 2,479 checks on `dramc_nb2`. [Model checker](smtcore/smt_model_checker.go) · [Scope and counterexample controls](smtcore/smt_model_checker_auxiliary_test.go) · [Both configurations against native](perf/ivy_raft/trivial_model_checks/apple.csv)

| Complete Raft run | Parent | Private shortcut |
|---|---:|---:|
| Private SMT calls | 188 | 188 |
| Decisions | 10,308 | 9,633 |
| Quantifier instantiations | 18,383 | 17,288 |
| Conflicts | 1,655 | 1,634 |
| Resource count | 2,775,418 | 2,550,642 |

The location of the check mattered. Testing immediately after generating the Skolems and universe restrictions left decisions, instantiations and conflicts unchanged. Testing before their creation changed subsequent search order. On obligation 275 alone, instantiations fell from 3,226 to 1,988 and resource steps from 530,264 to 408,525. On 274, instantiations increased from 2,378 to 2,518, while decisions fell from 1,778 to 1,520. All 419 original results and all 11,732 results from 28 mutated protocols matched the reference, including 77 intentional failures. [Six placement controls](perf/ivy_raft/trivial_model_checks/placement.csv) · [Search by obligation](perf/ivy_raft/trivial_model_checks/search.csv) · [Mutation results](perf/ivy_raft/trivial_model_checks/mutations.csv)

Against parent `ef2decbe3`, the release build reduced median paired wall time by **3.21%** and user CPU by **3.28%** across 28 alternating pairs, winning 20 of 28 on each. Median elapsed times were 4.93 s before and 4.66 s after. In a second 16-pair run it retired fewer instructions in every pair, a median paired reduction of **3.91%**, with **4.93%** lower median paired elapsed time. Both builds used the local g3 replacement, GoIvy `00ed3294a`, one Go CPU and the original mixed-width specification. [Release pairs](perf/ivy_raft/trivial_model_checks/pairs.csv) · [Hardware counters](perf/ivy_raft/trivial_model_checks/pmu.csv) · [Full-run benchmark](perf/ivy_raft/measure.py)

### Where Raft and Apple allocate after the private model-check shortcut

On g3 `7974daf84` and GoIvy `00ed3294a`, three complete allocation-profiled runs per workload gave these estimates. Each process used one Go CPU, sampled one allocation per 64 KiB on average, and ran a GC before writing its profile. Each Raft run matched all 419 named obligations. Each `rfn2` run matched the native checker's complete stdout, including its 26 passes and final `OK`. The figures below are cumulative allocations per run. Peak or retained worker memory needs a separate measurement. [Raft profile](perf/ivy_raft/allocation_profiles/raft.pprof) · [Apple rfn2 profile](perf/ivy_raft/allocation_profiles/rfn2.pprof)

| Whole-checker workload | Allocated MiB per run | Allocated objects per run | Largest individual allocation sites |
| --- | ---: | ---: | --- |
| Mixed-width Raft | 1,005 | 6.46 million | Rewriter activity-cache hash cells contributed 4.27% of bytes. Congruence-closure node construction contributed 3.34% of objects. |
| Apple `rfn2` | 4,787 | 37.04 million | GoIvy's `NodeArgs` contributed 19.98% of objects. `fmt.Sprintf` contributed 9.40% of bytes directly. Congruence-closure node construction contributed 2.75% of bytes. |

Of the estimated 22.2 million allocations directly in `NodeArgs` across all three Apple runs, 98.3% came from `symbolsIluAstRec`. Its recursive symbol traversal builds temporary slices to visit fixed-arity children. This identifies a specific frontend experiment independently of the solver's search order. On the solver side, the two profiles motivate measuring compact, owner-local activity-cache storage and allocating an enode together with its short argument array, as the native enode constructor does. Allocation counts alone do not predict the elapsed-time benefit.

The profiling driver now accepts `--allocation-rate 65536 --runs 3` along with the spec path, GoIvy checkout, reference stdout, CPU list and output path. For Apple add `--isolate rfn2` and pass the same spec path spelling used in the reference. Inspect the saved profiles with `go tool pprof -top -alloc_space` or `-alloc_objects`. The default CPU mode retains the per-obligation labels. [Profile driver](perf/ivy_raft/quantifier_profile/profile.py)

### A separate enode-pointer column and the cost of watched-literal restoration

Four full profiles of Apple `rfn2` on g3 `7974daf84` collected 161.23 s of CPU samples. Kernel search accounted for 77.2% inclusively, quantifier instantiation for 31.2%, and the Ivy symbol walker for 4.2%. The expression-table read alone accounted for 6.4% directly. Almost half its samples were for relevance lookups. These costs explain why shrinking a record can help a table benchmark while having little effect on the full check. [Apple CPU profile](perf/ivy_apple/expression_pages/rfn2.pprof)

I tested keeping the full 32-bit Boolean variable ID and moving the enode pointer out of each 24-byte expression record. Each page then holds 256 contiguous 16-byte records and an enode-pointer column. The sparse variant allocates that column only on its first non-nil enode. On the pointer-free dense fill fixture this reduced bytes allocated from 216,440 to 161,528. Median dense lookup went from 4.29 to 3.77 ns across eight samples per variant. After a peak of 2,048 bindings, incremental idle heap per solver fell from 911,448 to 884,824 bytes over three runs with 48 independent solvers. With 1,024 smaller solvers it fell from 69,638 to 67,952 bytes, about 1.65 MiB saved across the group. [Table and worker measurements](perf/ivy_apple/expression_pages/table-and-worker.csv) · [Worker benchmark](smtcore/smt_scope_workers_benchmark_test.go) · [Sparse layout patch](perf/ivy_apple/expression_pages/sparse.patch)

The full checker used `ef2decbe3` for both sides of this layout experiment. In six alternating rounds each on `rfn` and `rfn2`, the sparse column increased median paired instructions by 0.29% and 0.12%. On `sys_live` the change rounded to zero. Adding a presence flag gave +0.12%, +0.07% and −0.08% respectively. All 80 isolate transcripts matched. The smaller layout is a measured option for a memory-constrained deployment. Production keeps the inline pointer for now. Each saved patch applies independently at the repository root. [Whole-isolate measurements](perf/ivy_apple/expression_pages/checker.csv) · [Expression-table benchmark](smtcore/smt_expr_table_benchmark_test.go)

For scope restoration I compared checking the previous variable ID, deferring the first assignment lookup until the second watch is false, returning both assignments from watch selection, and reranking three watches from their cached values. With 128 Boolean atoms, 64 learned clauses and the delayed activity queue enabled, the direct three-watch ranking made scoped pop **7.35% slower** across 12 rotated rounds. Deferring the first lookup changed that median by **−0.37%**. In 16 additional native-controlled rounds on CPU 17 the latter change was **+0.48%**. The current Go implementation took 0.80× native time for surviving atoms and 1.49× for scoped atoms in median within-round comparisons with the queue enabled. Absolute times rose under concurrent load. All seven prototype builds passed the exhaustive ternary watch-order and learned-assignment restoration checks. Production keeps the existing selection algorithm. [384 Go measurements](perf/scope_restoration/watch_selection/restoration.csv) · [160 native-controlled measurements](perf/scope_restoration/watch_selection/native.csv) · [Comparison driver and seven prototype patches](perf/scope_restoration/watch_selection)

### Repeated preprocessing in private Raft refutations

Tracing one complete mixed-width Raft run showed that the first macro search changed the pending assertions in 108 of 277 primary batches. The second search left all 277 batches identical and took 93 ms. The final simplifier also left all 277 identical and took 39 ms. The auxiliary solver's final simplifier changed 94 of 2,309 batches. Skipping these optional passes can therefore change the search. The private refutation context now runs the first macro search and simplifier and expands previously discovered definitions at the start of later batches. Only an UNSAT refutation discharges the obligation. The ordinary solver keeps its full preprocessing sequence. [Per-pass measurements](perf/ivy_raft/repeated_preprocessing/pass_cost.csv) · [Pass selection experiments](perf/ivy_raft/repeated_preprocessing/passes.csv) · [Scoped macro regression](smtcore/smt_asserted_macros_test.go)

Compared with g3 `7974daf84` on GoIvy `00ed3294a`, this retired **3.61% fewer instructions** in the median across 16 alternating pairs, with fewer instructions in every pair. In a separate 28-pair release experiment, median paired wall time fell **1.91%** and user time fell **1.85%**. The candidate finished faster in 15 of the 28 pairs under concurrent load. All complete runs matched the ordered 419-obligation transcript. Both builds used one Go CPU and the same local g3 replacement. [Release times](perf/ivy_raft/repeated_preprocessing/pairs.csv) · [Hardware counters](perf/ivy_raft/repeated_preprocessing/pmu.csv) · [Individual pass variants](perf/ivy_raft/repeated_preprocessing/variants.csv) · [Full-run benchmark](perf/ivy_raft/measure.py)

Across 188 private kernel searches, resource count fell from 2,550,642 to 2,461,941. Decisions changed from 9,633 to 9,632, with 17,288 quantifier instantiations and 1,634 conflicts on each build. Across all 28 width and mutation cases, the 11,655 passes and 77 failures matched the references. Nine Apple isolates reproduced native stdout and all 18 per-check counters at each of 10,342 checks, including 2,479 on `dramc_nb2`. The Apple CSV reports the number of differing checks in each counter column. [Search counts](perf/ivy_raft/repeated_preprocessing/search.csv) · [Mutations](perf/ivy_raft/repeated_preprocessing/mutations.csv) · [Apple checks](perf/ivy_raft/repeated_preprocessing/apple.csv)

### Cost of executing the same Apple search in g3 and Z3

On `ord_live.ivy` isolate `rfn2`, g3 and native Z3 matched on all 18 recorded search counters at every one of the 26 checks. They took 805,401 decisions and instantiated 270,033 quantifiers each. Six paired complete runs with GoIvy `00ed3294a` and g3 `213f7d64a` found a 2.36× difference in retired instructions. Both binaries used the same GoIvy source and input, pinned to the same logical CPU with Go PGO off, one Go CPU and tracing disabled. Each reproduced the known-good full transcript. [Per-check search comparison](perf/ivy_raft/repeated_preprocessing/apple.csv) · [Complete runs](perf/ivy_apple/incremental_cost/whole.csv)

| User-mode measure | g3 median | Native median | Median within-pair g3/native |
|---|---:|---:|---:|
| Whole run, instructions | 177.69 billion | 75.42 billion | 2.36× |
| Whole run, cycles | 225.35 billion | 124.70 billion | 1.82× |
| Whole run, user CPU | 56.55 s | 31.20 s | 1.83× |
| Whole run, branches | 36.44 billion | 15.26 billion | 2.39× |
| Whole run, branch misses | 390 million | 411 million | 0.95× |
| Inside the 26 solver calls, instructions | 129.51 billion | 35.48 billion | 3.65× |
| Outside those calls, instructions | 48.69 billion | 39.80 billion | 1.22× |

Five further alternating pairs per phase enabled the PMU only inside the GoIvy verification-condition adapter, then only outside it in separate processes. Roughly **91% of the excess instructions arose inside solver calls**, estimated from the two phase median differences. That boundary includes the g3 private refutation attempt where applicable. The remaining phase includes assertion construction and solver operations performed between checks. The host also ran other experiments, so wall times are sensitive to scheduling. [Phase counters](perf/ivy_apple/incremental_cost/phases.csv) · [Phase benchmark and exact measurement boundary](perf/ivy_apple/phase_counters.py)

Across six additional pairs, g3 executed 2.40× as many L1 data loads, with 1.45× as many L1 data-load misses. The absolute count of whole-run branch misses was similar. This makes repeated loads within each solver operation a priority for measurement. [Cache counters](perf/ivy_apple/incremental_cost/cache.csv)

Six whole-isolate user-cycle profiles per backend locate related work in both implementations. In g3, relevance handler traversal accounted for 4.05% of samples, relevance lookup 3.64%, Boolean propagation 3.12% and the AST negation test 3.10%. In native Z3, the disjunction relevance handler took 7.23%, assignment relevance dispatch 6.32% and relevance lookup 4.33%. The two binaries had 154,615 and 82,235 samples respectively. Function shares are within each binary and include inlining effects. The next measurements should test the number of handler visits, the loads per visit and whether combining AST and Boolean-state lookups reduces instructions in the expensive checks. [All sampled symbols](perf/ivy_apple/incremental_cost/symbols.csv.gz) · [Native relevance implementation](../src/smt/smt_relevancy.cpp) · [Go relevance implementation](smtcore/smt_relevancy.go)

Use `perf/ivy_apple/compare.py --perf /path/to/perf` with the two release checkers for whole-run counters. For phase counters, run `python3 perf/ivy_apple/phase_counters.py --goivy /path/to/goivy --spec /path/to/ord_live.ivy --reference /path/to/rfn2.stdout --cpu 24 --pairs 5 --select inside outside --output /tmp/rfn2-phases`. Supply `--modfile` if needed for the local g3 replacement. A one-based check number selects a single call while the complete isolate still runs. Use the same spelling of the input path used to generate the reference stdout. [Whole-isolate benchmark](perf/ivy_apple/compare.py)

Repeating the entire isolate twice per backend for each of the 26 individually selected calls localized **84.0% of the excess solver instructions to three preservation checks of the same invariant**, `rfn2.invar494` at line 3134 of `ord_live.ivy`. The largest was check 4, `ext:arm.step_north`, with 71.15 billion g3 instructions and 20.86 billion native. Check 22, `ext:memc.step`, used 19.62 and 5.21 billion. Check 13, `ext:dramc.step_rd`, used 18.46 and 4.39 billion. Check 4 alone accounted for 53.6% of the difference. The per-check median counts sum to 129.23 billion in g3 and 35.48 billion in Z3, close to the independent all-checks measurement. All 104 runs reproduced the full reference transcript. [Per-check instruction measurements](perf/ivy_apple/incremental_cost/checks.csv)

### Allocation in the GoIvy symbol walker

GoIvy commit `3ef4b9020` gives each recursive symbol-walker call three stack slots for the children of fixed-arity expressions. An implication fills two slots and an if-then-else fills three. Applications and variadic connectives return their existing argument slice. The public `NodeArgs` and the walker share the same type switch. Traversal order, repeated occurrences, and early termination by the caller are preserved. The benchmark is in the GoIvy commit as `goivy/ivylogic_symbols_benchmark_test.go`. Run `go test -run '^$' -bench '^BenchmarkSymbolsIluAstTraversal$' -benchmem` from the GoIvy module. [All 90 microbenchmark measurements](perf/ivy_apple/symbol_traversal/micro.csv)

| Fixture or workload | Paired observations | Change with stack storage |
|---|---:|---|
| Fixed-arity symbol traversal | 15 | 190 → 0 allocations per traversal, median 12.52 → 4.82 µs |
| Application containing two of those trees | 15 | 380 → 0 allocations per traversal, median 26.16 → 9.77 µs |
| `rfn2`, whole-isolate allocation profiles | 3 | 7.28 million fewer objects and 214 MB fewer bytes |
| `sys_live`, whole-isolate allocation profiles | 3 | 56.63 million fewer objects and 1.66 GB fewer bytes |

The object and byte differences are medians of within-pair differences, estimated by Go allocation sampling at 64 KiB. Separate release pairs measured **1.06% fewer instructions and 4.49% less elapsed time on `rfn2`** across eight pairs. On `sys_live`, the reductions were **3.06% and 5.07%** across five pairs. On `rfn`, instructions fell 0.90% but elapsed time rose 2.87%. The mixed-width Raft checker retired 0.10% fewer instructions over 14 pairs. Shared-host elapsed times varied. The release panel used g3 `7974daf84`, and the allocation and parity checks used `213f7d64a`. [Allocation profiles summarized by run](perf/ivy_apple/symbol_traversal/allocations.csv) · [Apple release panel](perf/ivy_apple/symbol_traversal/apple.csv) · [Raft release panel](perf/ivy_apple/symbol_traversal/raft.csv)

The direct-recursion prototype also eliminated allocations and ran the fixed-arity microbenchmark in 3.04 µs. It duplicated the child definitions across the walker and `NodeArgs`. Its additional instruction saving on the full `rfn2` isolate was about 0.2 percentage points. The selected implementation keeps a single definition of each node's children. Nine Apple isolates matched native stdout and all 18 per-check counters across 10,342 checks. All 188 private Raft searches matched the parent on decisions, instantiations, conflicts and resources. All 28 mutated specifications matched the references, including 77 failing obligations. [Apple parity](perf/ivy_apple/symbol_traversal/parity.csv) · [Raft search reference](perf/ivy_raft/repeated_preprocessing/search.csv) · [Mutation results](perf/ivy_apple/symbol_traversal/mutations.csv) · [Direct-recursion prototype](perf/ivy_apple/symbol_traversal/direct.patch) · [Independent formatting prototype](perf/ivy_apple/symbol_traversal/text.patch)

### Reusing the batch of newly relevant expressions

g3 delivered each relevance mark to its theory callback immediately, then copied the whole batch of AST handles and discarded the copy. Successful internal propagation now clears the batch in place and keeps up to 512 handles (8 KiB) per solver for the next round. External calls to `DrainNewlyRelevant` still return their own snapshot. A failed propagation returns before clearing the batch. [Propagation](smtcore/smt_theory_propagation.go) · [Queue implementation](smtcore/smt_relevancy.go) · [Snapshot and scope tests](smtcore/smt_relevancy_test.go)

In four paired allocation profiles of complete `rfn2` runs, the change saved a median **159 MB and 608,189 objects per run**. Eliminating the unused snapshot saved 47 MB. Reusing the append buffer saved a further 113 MB at the mark site. These are differences estimated separately by allocation sampling at 64 KiB. Across seven rotated rounds on each isolate, median paired whole-check instructions fell 0.35% on `rfn2` and 0.34% on `rfn`. Across 14 Raft rounds the reduction was 0.12%. The search results, all 18 counters at every one of 10,342 Apple checks, and all 11,732 named Raft mutation outcomes matched. [Allocation profiles by run and site](perf/ivy_apple/relevance_batches/allocations-rfn2.csv) · [Apple candidate measurements](perf/ivy_apple/relevance_batches/apple.csv) · [Raft candidate measurements](perf/ivy_apple/relevance_batches/raft.csv) · [Apple parity](perf/ivy_apple/relevance_batches/parity.csv) · [Mutation results](perf/ivy_apple/relevance_batches/mutations.csv)

| Marks per scope | Median full-round time before → after | Allocations per round before → after |
|---:|---:|---:|
| 1 | 743 → 620 ns | 2 → 0 |
| 32 | 8.27 → 7.01 µs | 7 → 0 |
| 512 | 114.85 → 106.84 µs | 11 → 0 |
| 1,024 | 234.18 → 235.36 µs | 13 → 12 |

The 1,024-mark round exercises the retention limit. Each pair used the same pinned CPU, with alternating order across 14 pairs per size. Run `go test ./smtcore -run '^$' -bench '^BenchmarkSMTTheoryRelevanceBatch$' -benchmem` from `pkg`. [All 112 measurements](perf/ivy_apple/relevance_batches/micro.csv) · [Benchmark](smtcore/smt_hotpath_benchmark_test.go)

### Which expressions account for relevance propagation on Apple

Instrumenting the same operations in both backends found that g3 and native Z3 propagated the same nodes, visited the same watches and examined the same number of disjunction children on all 395 checks in `rfn2`, `rfn` and `cf_live`. Eighteen of the 20 counters matched at every check. The two that varied count entries to the mark and propagate functions, including entries that find no new work. On `rfn2`, each engine propagated 2,795,024 nodes, delivered 4,091,064 assignment events and visited 9,466,533 negative watches. [Per-check counters for both engines](perf/ivy_apple/incremental_cost/relevance.csv.gz)

Both engines called disjunction propagation 9,676,384 times on `rfn2`. They tested 28,092,262 children, found an already-relevant true child 8,637,852 times and selected a child to mark 474,601 times. In g3, **26,956,203 child tests (95.96%)** came from disjunctions whose parent had no Boolean variable. Such root clauses already supply their child expressions and SAT literals when they are asserted. Keeping those bindings available to the watch handler could avoid repeat lookups on most child tests. Ordinary gates with a Boolean variable accounted for the remaining 1,136,059 tests. [Disjunction counts by path](perf/ivy_apple/incremental_cost/root-gates.csv.gz) · [Root assertion](smtcore/smt_internalizer.go)

Reproduce with `python3 perf/ivy_apple/relevance_counts.py --goivy /path/to/goivy --spec /path/to/ord_live.ivy --reference /path/to/rfn2.stdout --cpu 24`. Add `--root-gates` for the g3 breakdown. The driver instruments temporary builds, verifies the native relevance object against GoIvy's linked archive and checks the complete stdout against the supplied reference. An independent rebuild reproduced every recorded value on all 26 checks of `rfn2`. [Counter driver](perf/ivy_apple/relevance_counts.py) · [Native observation patch](../src/test/perf/ivy_relevance_counts.patch)

### Equivalent Boolean definitions in private Raft proofs

In 35 of the 188 private solver inputs for mixed-width Raft, two transition flags had the same defining expression. The private order translator now recognizes premises of the form `a = E` and `b = E` and prints the first Boolean symbol at uses of the second. Every model satisfying both premises assigns those names the same value. The matcher checks the typed term structure and abstract sorts, draws definitions only from premises, and starts over when a conditional refutation changes the premises. It remembers up to eight definitions and charges its comparisons to the existing proof budget. [Implementation](api/internal/order/boolean_definitions.go) · [Positive and negative tests](api/api_order_boolean_definitions_test.go)

Across 24 paired release runs of the original 419-obligation input, this reduced retired instructions by **6.05%**, wall time by **5.59%** and user CPU time by **5.58%**. These are medians of within-pair percentage changes on two pinned CPUs. The mixed, two-, three-, four-, eight- and sixteen-bit width fixtures retired 5.89–6.12% fewer instructions across six pairs each. The one-bit fixture uses the separate Boolean proof path and measured +0.16%. All 132 runs reproduced the reference transcript. The comparison used GoIvy `3ef4b9020`, the local g3 module, one Go CPU, tracing disabled and PGO off. [All release timings and hardware counters](perf/ivy_raft/boolean_definitions/release.csv) · [Width fixtures](perf/ivy_raft/widths.py) · [Full-run benchmark with optional hardware counters](perf/ivy_raft/measure.py)

Total solver resources across the 188 searches fell from 2,461,941 to 2,176,281. Obligation 274 dropped from 519,639 to 94,982. Obligation 275 rose from 399,704 to 526,290. Decisions fell from 9,632 to 9,534, while quantifier instantiations rose from 17,288 to 17,635 and conflicts from 1,634 to 1,692. Simply deduplicating identical premise lines in the original problems left every search statistic unchanged. Limiting the definition table to eight or 32 entries produced the same search statistics on this input. In the implementation panel, `stock` is the parent, `broad` uses 32 entries, `small` uses eight, `narrow` admits only negated bodies, `no-charge` omits the budget deduction and `spelling` caches printed function names. [Candidate search counts](perf/ivy_raft/boolean_definitions/search.csv) · [Parent search counts](perf/ivy_raft/repeated_preprocessing/search.csv) · [120 implementation comparisons](perf/ivy_raft/boolean_definitions/variants.csv)

Native Z3 independently proved the flag equality from the two defining premises in all 35 changed inputs. With either premise omitted, each of the 70 controls admitted a model in which the flags differed. Under the proved equality, Z3 checked all 592 rewritten assertions for equivalence, including quantified assertions and goals. Run `python3 perf/ivy_raft/boolean_definitions/verify.py` from `pkg` to reproduce all 697 checks using this fork's native executable. Nine Apple isolates matched native stdout and all 18 counters on 10,342 checks. All 28 Raft mutation cases matched, including 77 failing obligations. [Native verification](perf/ivy_raft/boolean_definitions/verify.py) · [Saved original and transformed queries](perf/ivy_raft/boolean_definitions/queries.json.gz) · [Native control results](perf/ivy_raft/boolean_definitions/native.csv) · [Mutations](perf/ivy_raft/boolean_definitions/mutations.csv) · [Apple parity](perf/ivy_raft/boolean_definitions/apple.csv)

### Comparing a single asserted-disjunction watch in g3 and Z3

The native and Go fixtures assert the same disjunction of alternating positive and negative Boolean atoms. Both check that the parent is relevant, has no Boolean variable, and has no marked witness before the assignment. They then deliver a real assignment callback, check that it marked the true child, and repeatedly deliver the same event. This times the already-satisfied path taken 8.64 million times on `rfn2`. Twelve rotated rounds on CPU 23 measured 500,000 callbacks per case. The three experimental Go builds used the same solver sources as the Go control except for their respective watch patches. [Go fixture](smtcore/smt_root_relevance_benchmark_test.go) · [Native fixture](../src/test/perf/asserted_or_witness.cpp) · [All 360 measurements](perf/ivy_apple/root_watches/native.csv)

| Width and position of the true child | Native Z3, ns | g3 control, ns | g3 with SAT literals, ns | g3 with current expression records, ns | g3 with a remembered witness, ns |
|---|---:|---:|---:|---:|---:|
| 4, first | 16.2 | 102.4 | 24.5 | 20.5 | 19.0 |
| 4, last | 27.4 | 184.7 | 25.7 | 27.9 | 17.6 |
| 16, last | 63.6 | 619.6 | 33.4 | 51.3 | 17.6 |
| 64, last | 214.2 | 2,240.5 | 69.7 | 160.5 | 17.5 |

The Go control spends about 10× native time finding a true final child at width 64. Direct SAT literals cut that case below native time while preserving the linear scan. Remembering the child avoids the scan when its current assignment and relevance mark still qualify it. This fixture always repeats the same event. The whole-isolate measurements exercise changes of assignment, relevance, and scope. Reproduce the current checkout and native control with `python3 perf/ivy_apple/root_watches/compare.py --cpu 23 --rounds 12` from `pkg`. The driver also accepts previously built Go test executables. Each experimental patch applies independently to g3 `681bb232d` from the repository root. [Paired benchmark driver](perf/ivy_apple/root_watches/compare.py) · [SAT literals](perf/ivy_apple/root_watches/dense.patch) · [Current records](perf/ivy_apple/root_watches/refs.patch) · [Remembered witness](perf/ivy_apple/root_watches/witness.patch)

### Reusing child records in asserted-disjunction watches

On Apple `rfn2`, remembering the last true, relevant child reduced the number of child tests from **28,092,262 to 14,222,885** across 26 incremental checks, a **49.37% reduction**. Native Z3 and g3 still visited 9,676,384 disjunctions, found 8,637,852 existing witnesses, and selected 474,601 new witnesses. Each of those three counts matched on every check. The previous observer counted the generic OR path. The updated observer also counts the cached-witness test and every subsequent child examined. [Native and Go operations](perf/ivy_apple/root_watches/operations.csv.gz) · [Counts by gate type](perf/ivy_apple/root_watches/paths.csv.gz)

An asserted OR usually has no Boolean variable of its own. During assertion, g3 already identifies each child's literal and its underlying expression. The shared watch now keeps pointers into the solver's existing expression pages. It rechecks both the relevance and assignment of its last witness. If the witness has changed, it scans in source order and marks the first true child only after ruling out an existing true, relevant child. When the parent later acquires a Boolean variable, the existing gate supplies its signed SAT literals. The focused tests cover scope pop, Boolean ID reuse, explicit assignments, nested negation, and detaching and rebinding relevance. Generic pair and term-ITE handlers keep their additional operands in a separate record.

In 21 rotated rounds on CPU 23, each sample delivered 500,000 callbacks. The release g3 baseline was `fa19e73c8`. The updated build used the same Go toolchain and fixture. The first and last child positions measure how much work depends on the location of an existing witness. [All 378 measurements](perf/ivy_apple/root_watches/current-micro.csv)

| Width | True child | Original g3 ns/callback | Updated g3 ns/callback | Native Z3 ns/callback |
|---|---|---:|---:|---:|
| 4 | First | 101.20 | 25.48 | 23.18 |
| 4 | Last | 222.20 | 26.07 | 35.33 |
| 16 | First | 114.90 | 25.84 | 23.49 |
| 16 | Last | 744.80 | 26.64 | 79.83 |
| 64 | First | 101.60 | 24.95 | 22.02 |
| 64 | Last | 2,689.00 | 28.45 | 294.01 |

A separate worker benchmark keeps 1,024 independent single-goroutine solver contexts live and asserts eight four-child clauses per context. Across ten fresh processes per build, the median additional live heap per solver with the scopes active rose from **13,592.5 B to 13,781 B**. After popping the scopes, it fell from **12,312.5 B to 11,605 B**. These are increments over the same fixture with its AST roots already held. Use `go test ./smtcore -run '^$' -bench '^BenchmarkSMTAssertedOrWorkerMemory$' -benchtime=1x` for this fixture. [Worker measurements](perf/ivy_apple/root_watches/workers.csv)

All nine Apple isolates reproduced their native transcripts and matched all 18 native search counters at each of 10,342 checks. All 188 retained Raft private proof searches matched the preceding frontier. The 28 Raft mutation controls reproduced all 11,655 expected passes and 77 expected failures.

Across 48 paired release runs on GoIvy `3ef4b9020`, g3 `0a27fdff9` reduced instructions in every Apple pair. Each process used one logical CPU, `GOMAXPROCS=1`, tracing disabled and PGO off. Each pair ran on the same CPU. These figures are medians of within-pair percentage changes on a shared machine. The control used g3 `fa19e73c8`. [All 96 release measurements](perf/ivy_apple/root_watches/release.csv)

| Workload | Pairs | Instructions | User CPU | Elapsed |
|---|---:|---:|---:|---:|
| Apple `rfn2` | 12 | −8.02% | −10.29% | −10.92% |
| Apple `rfn` | 10 | −7.06% | −8.86% | −8.83% |
| Apple `cf_live` | 8 | −3.36% | −2.78% | −1.56% |
| Raft, original widths | 18 | −0.12% | −0.32% | +0.61% |


## Which Apple checks and operations explain the difference?

After the watch change, **82.7% of the remaining solver instruction gap on `rfn2` comes from three checks of the same invariant**. Each row below is the median of five complete runs per backend with hardware counters enabled only for the selected interval. Both backends use GoIvy `3ef4b9020`. The g3 build uses `0a27fdff9`. All 40 runs reproduced the full native transcript. [Exact measurements](perf/ivy_apple/incremental_cost/watches-phases.csv) · [Counter driver](perf/ivy_apple/phase_counters.py)

| Interval | g3, billion instructions | Z3, billion instructions | Excess |
|---|---:|---:|---:|
| All 26 checks | 114.57 | 35.48 | 79.09 |
| Check 4, `ext:arm.step_north` | 60.95 | 20.86 | 40.09 |
| Check 13, `ext:dramc.step_rd` | 17.70 | 4.39 | 13.31 |
| Check 22, `ext:memc.step` | 17.25 | 5.21 | 12.04 |

All three preserve `rfn2.invar494` at `ord_live.ivy` line 3134. Check 4 previously took 71.15 billion instructions in g3. The watch change and the intervening revisions brought it down by 10.20 billion. The native counter stayed at 20.86 billion. [Earlier per-check counts](perf/ivy_apple/incremental_cost/checks.csv)

Sampling once every 800,009 user instructions also measures all obligations in one pass. The driver records monotonic entry and exit times for each solver check, bins hardware samples into those windows, and puts the rest in row zero. Across three independent pairs it estimated 114.88 and 35.48 billion instructions inside g3 and native checks respectively. The medians for checks 4, 13 and 22 were within 0.4% of the directly gated counts in both engines. An individual short check can finish between samples. [All six sampled runs](perf/ivy_apple/incremental_cost/watches-check-samples.csv) · [Per-check sampling driver](perf/ivy_apple/check_samples.py)

### Instruction stacks before the watch change

The earlier profiles show why the quantifier candidate checker and the rewriter are worth investigating. The table counts samples whose stack included each operation during check 4, multiplied by the sampling period. The dense run used g3 `213f7d64a`, GoIvy `00ed3294a` and a period of 4,800,019 instructions. It recorded 14,775 g3 and 4,345 native samples. The coarser replication used g3 `7b3cd82ef`, GoIvy `3ef4b9020` and a period of 28,800,019. Figures are estimates in billions of instructions. [Dense counts](perf/ivy_apple/incremental_cost/check4-stack-operations.csv) · [Replication and all-check counts](perf/ivy_apple/incremental_cost/prewatch-stack-operations.csv)

| Operation | g3 | Z3 | Excess | Replication excess |
|---|---:|---:|---:|---:|
| Quantifier instantiation queue | 34.56 | 9.75 | 24.81 | 25.34 |
| Rewriter | 13.46 | 2.59 | 10.87 | 11.00 |
| Candidate checker | 14.11 | 5.44 | 8.67 | 8.29 |
| Relevance assignment | 12.62 | 3.24 | 9.38 | 9.30 |
| Disjunction relevance | 8.64 | 1.79 | 6.85 | 6.45 |

The queue calls the candidate checker and the rewriter. Adding their inclusive estimates would count some instructions twice. The CSVs also assign each sample to its nearest recognized operation for an additive view. Symbolization and compiler inlining affect attribution. The all-check replication puts 61.49 versus 16.36 billion instructions under the queue. This supports working on the cost of checking and rewriting each proposed instantiation while holding the native search trace fixed. To repeat the measurement, run the stack recorder with `--select 4 inside`, then pass its completed `PHASE-PAIR-ENGINE` directories and the same `--period` to the summarizer. [Stack recorder](perf/ivy_apple/phase_stacks.py) · [Operation and leaf summarizer](perf/ivy_apple/stack_costs.py)

### Replaying a quantifier candidate with a known answer

Both implementations now have a benchmark for the same disjunction of equalities, `f(x) = f(a₀) ∨ … ∨ f(x) = f(aₙ)`. Each check gets one binding for `x`. The solver must look up `f(x)` by congruence and find the matching equality. The fixtures check that the first and last bindings succeed and that a binding with no registered application fails. The timed direct-check rows invoke g3's `IsSatisfiable` and this fork's `checker::is_sat`. The Go queue rows include instance preparation and the already-satisfied return through the quantifier manager. [Go benchmark](smtcore/smt_quantifier_candidate_benchmark_test.go) · [C++ benchmark](../src/test/perf/quantifier_candidate.cpp)

| Equalities | Matching child | g3 direct | Z3 direct | g3 via queue |
|---:|---|---:|---:|---:|
| 4 | First | 292.5 ns | 95.3 ns | 387.6 ns |
| 4 | Last | 585.8 ns | 206.0 ns | 712.7 ns |
| 16 | First | 269.1 ns | 93.1 ns | 380.5 ns |
| 16 | Last | 1,984.0 ns | 714.5 ns | 2,061.0 ns |

These are medians of 11 rotated runs of 100,000 calls on CPU 43 with g3 `0a27fdff9` and `GOMAXPROCS=1`. Even when the candidate answer and scan order are fixed, checking the last of 16 equalities costs 1.27 µs more in g3. Queue dispatch adds another 77 ns in that case. Build a g3 test binary with `go test -c -o /tmp/g3-candidate.test ./smtcore`, then run `python3 perf/ivy_apple/candidate_checker/compare.py --cpu 0 --go g3 /tmp/g3-candidate.test` with a configured native Makefile build in the fork. [Rotated measurements](perf/ivy_apple/candidate_checker/baseline.csv) · [Comparison driver](perf/ivy_apple/candidate_checker/compare.py)

### Which Apple obligations account for the solver gap

Three paired runs of every isolate sampled one user instruction in 800,009 and attributed it to the active solver check. All 54 runs reproduced their complete native transcript. The totals below sum samples **inside** solver calls. Each row identifies the check with the largest median paired excess in that isolate. Both engines used GoIvy `3ef4b9020`. g3 used the asserted-disjunction change at `0a27fdff9`. [All 10,342 checks, three pairs](perf/ivy_apple/incremental_cost/nine-isolate-samples.csv.gz) · [Top eight per isolate with action and obligation](perf/ivy_apple/incremental_cost/nine-isolate-largest-gaps.csv) · [Sampling driver](perf/ivy_apple/check_samples.py) · [Ranking and transcript lookup](perf/ivy_apple/incremental_cost/rank_checks.py)

| Isolate | All checks, g3 / Z3 (billions of instructions) | Largest gap, check number and source line | That check, g3 / Z3 (billions) | Share of isolate solver gap |
|---|---:|---|---:|---:|
| `cf_live` | 127.17 / 34.00 | 130, line 1703 | 41.19 / 12.70 | 30.6% |
| `cf_pio_live` | 120.46 / 31.61 | 126, line 2529 | 41.90 / 11.87 | 33.8% |
| `cmpl_live` | 17.58 / 4.29 | 35, line 2439 | 6.02 / 1.37 | 35.0% |
| `dramc_nb2` | 63.05 / 15.71 | 71, line 2195 | 25.89 / 6.99 | 39.9% |
| `e2e` | 135.94 / 36.54 | 99, line 2817 | 35.86 / 11.03 | 25.0% |
| `rfn` | 100.36 / 29.50 | 5, line 1982 | 53.04 / 17.02 | 50.8% |
| `rfn2` | 114.88 / 35.48 | 4, line 3134 | 61.01 / 20.86 | 50.6% |
| `sys_live` | 104.75 / 29.32 | 404, line 3193 | 18.44 / 5.54 | 17.1% |
| `tprops` | 3.06 / 0.65 | 218, line 1792 | 0.67 / 0.16 | 21.2% |

The most expensive excess in six isolates occurs while checking `ext:arm.step_north`. Two others concentrate on `ext:cfabric.step`. On `dramc_nb2`, check 71 is the `l2s_progress_made[1]` obligation. The first hardware-counter run enabled just for that call counted 25.97 billion g3 instructions and 6.99 billion native instructions. The sample-derived estimate picks the same target from 2,479 possible checks in one pass per engine. A short check can have zero samples. Its resource use is below the resolution of that run.

Two independently recorded stack pairs for check 71 put the quantifier queue on 76.5% and 76.9% of g3 samples and 71.3% and 71.8% of native samples. Within the complete check, the solver rewriter appeared on 28.6% and 29.5% of g3 samples versus 19.8% and 23.9% of native samples. These are inclusive stack counts. The queue invokes the rewriter and the candidate checker. This identifies work on candidate evaluation and rewriting that can benefit both the refinement and liveness isolates. [Stack samples and nearest-operation attribution](perf/ivy_apple/incremental_cost/dramc-check71-stack-samples.csv) · [Stack recorder](perf/ivy_apple/phase_stacks.py)

To reproduce the ranking, collect a CSV with `check_samples.py` for the isolate and pass it to `rank_checks.py` with `--transcript ISOLATE NATIVE_STDOUT`. Check numbers are one-based. The percentage uses the median within-pair difference for the selected check divided by the median within-pair difference for all checks in the isolate.

### Counting instructions inside a scoped pop

I gated a hardware counter around the queue-enabled scope benchmark in both engines. The Go fixture pins its goroutine to one OS thread. Linux counts only user instructions on that thread between enable and disable. Every column below is the median of 11 runs on CPU 22. Each run excludes eight warm-up rounds, measures 5,000 intervals, and subtracts its own engine's empty-interval cost from the same round of the comparison. Enabled time equalled running time for every counter. Both fixtures verify that the 64 learned clauses retain their identity, bindings, literals and polarity. [Thread counter samples](perf/scope_restoration/pop_phases/scoped-thread.csv) · [Measurement driver](perf/scope_restoration/pop_phases/measure.py) · [Native probe](../src/test/perf/scope_phase_main.cpp)

| Instructions per scoped pop or phase | g3 before | g3 with single-pointer clear | Native Z3 |
|---|---:|---:|---:|
| Complete pop | 237,816 | 228,023 | 144,723 |
| Relevance pop | 25,213 | 25,209 | 8,036 |
| Undo core trail | 73,811 | 63,552 | 19,658 |
| Restore learned clauses | 116,871 | 116,827 | 89,214 |

Native `undo_mk_bool_var` removes the variable from the branching queue, clears the expression-to-variable binding and pops its owning expression stack. It retains the per-variable arrays at their high-water size and resets the two watch lists when it publishes the ID again. Go drops its pointers to retired clauses, clears any gate and justification references, shortens its live vectors and releases the variable's AST reference. Those differences explain why the part timed as trail undo covers more work in Go. The complete pop also includes the subsequent republication in both engines. At the original layout the largest phase excess was **54,153 instructions in undo**, followed by **27,657 in restoration** and **17,177 in relevance**. The phase runs use separate processes, so their median differences need not sum to the total. [Native variable publication and undo](../src/smt/smt_internalizer.cpp) · [Go variable publication and undo](smtcore/smt_internalizer_state.go)

A retiring watch list often contains exactly one clause in this fixture. Storing nil directly into that slot avoids Go's general pointer-range clearing call and drops the reference before the slot can be reused. Longer lists retain the same bulk clear. The change reduced median paired trail-undo instructions by **13.64%**, and complete-pop instructions by **4.36%**. A separate process-wide FIFO counter on CPU 32 also measured a reduction. In two uninstrumented 12-pair series, queue-enabled scoped-pop latency fell **6.99% on CPU 25 and 4.53% on CPU 28**. Whole-round elapsed time fell 6.20% and 2.72% respectively. Native remains faster on the scoped case. [Pointer clearing](smtcore/watch_list.go) · [Uninstrumented times](perf/scope_restoration/pop_phases/elapsed.csv) · [Process counter control](perf/scope_restoration/pop_phases/scoped-process.csv)

All 18 counters at each of the 10,342 Apple checks, the 188 Raft proof searches and all 28 Raft mutation transcripts matched their references after the change. To repeat the phase measurement on Linux/amd64 with the in-tree native Makefile build, run the command below from `pkg/`. It creates the supplied output directory and compiles private instrumented copies of both solvers. Omitting `--thread-event` selects process-wide counters through `perf stat` control FIFOs. The per-phase wall time of an instrumented run includes measurement overhead. The elapsed-time comparison above uses the ordinary benchmark.

```sh
python3 perf/scope_restoration/pop_phases/measure.py --cpu 0 --thread-event instructions:u \
  --pairs 11 --iterations 5008 --dir /tmp/g3-scoped-pop > /tmp/g3-scoped-pop.csv
```

### Compiler profiles across Raft and Apple

I combined the checked-in Raft and Apple `rfn2` Go CPU profiles and used the result as an explicit `go build -pgo` input. In 47 sets of five complete executions, each set used one CPU, the same GoIvy revision and the same specification. The five builds were the g3 revision before the watch change (`fa19e73c8`), the watch change (`0a27fdff9`), each of those with the combined compiler profile, and native Z3 through GoIvy. All 235 executions reproduced the reference outcomes. The table compares the two builds of `0a27fdff9` and gives the native instruction count for scale. Differences are median within-set changes. [All five builds and 235 runs](perf/ivy_apple/compiler_profiles/watches-raft-apple.csv)

| Specification or isolate | Sets | g3 PGO off, billion instructions | g3 blended PGO, billion | Native, billion | Paired g3 instruction change | Paired g3 elapsed change |
|---|---:|---:|---:|---:|---:|---:|
| Original mixed-width Raft | 10 | 30.60 | 28.15 | 215.25 | −8.03% | −3.93% |
| `rfn2` | 10 | 161.41 | 148.54 | 73.18 | −7.94% | −3.99% |
| `rfn` | 6 | 140.39 | 129.27 | 60.93 | −7.90% | +9.59% |
| `cf_live` | 5 | 231.74 | 214.44 | 124.55 | −7.46% | −1.40% |
| `dramc_nb2` | 6 | 524.92 | 504.11 | 589.76 | −4.00% | −16.56% |
| `sys_live` | 5 | 441.63 | 420.62 | 412.16 | −4.67% | −4.94% |
| `e2e` | 5 | 561.82 | 534.86 | 527.13 | −4.78% | +12.91% |

The profile lowered instruction counts on each workload, including five Apple isolates outside the training pair. Elapsed results varied. These lanes ran concurrently with other measurements, and `rfn` and `e2e` both took longer with PGO in this series. On `dramc_nb2`, the elapsed medians were 146.28 s for g3, 122.70 s for g3 with PGO and 132.53 s for native. Those times include the frontend and all 2,479 proofs. The separate solver-only measurements above account for the much larger g3 instruction count on check 71.

A second elapsed-time comparison ran ten alternating pairs per workload across two CPUs. The median within-pair time reductions were 4.94% for `e2e` (8 of 10 wins), 3.53% for Raft (7), 1.09% for `rfn` (7) and 3.69% for `rfn2` (7). Only one pair per Apple isolate and two Raft pairs had both runs below 20% SMT-sibling activity. These observations support an opt-in profile. The samples include the load observations needed for replication on a quieter host. [All 40 elapsed-time pairs](perf/ivy_apple/compiler_profiles/elapsed-pairs.csv)

To build the same profile from `pkg/`, run the first command below. Run the build command in the GoIvy checkout with its local g3 replacement. For a paired control, substitute `-pgo=off` and choose another output binary. The comparison driver accepts both binaries and the original reference transcript. It records how busy the chosen logical CPU and its SMT sibling were during each timed process. [Apple input profile](perf/ivy_apple/expression_pages/rfn2.pprof) · [Raft input profile](perf/ivy_raft/quantifier_profile/raft.pprof) · [Elapsed-time comparison driver](perf/ivy_apple/compiler_profiles/compare.py)

```sh
go tool pprof -proto -output /tmp/g3-apple-raft.pprof \
  perf/ivy_apple/expression_pages/rfn2.pprof perf/ivy_raft/quantifier_profile/raft.pprof

go build -mod=readonly -buildvcs=false -tags=xtrace_off -pgo=/tmp/g3-apple-raft.pprof \
  -o /tmp/goivy_check_pgo ./cmd/goivy_check
```

### Live memory per solver for scoped candidate checks

I measured 1,024 independent solvers to establish the per-worker cost of storing reusable quantifier syntax. Each solver already owns the quantified AST and its 16-arm fixture at the first forced GC. Each then pushes one scope, registers the quantifier and checks 32 satisfied candidates through the queue. I sampled live heap again with the scope active and after every solver popped its scope. All solvers stayed reachable through the final sample. The benchmark verifies the number of instances and the removal of every registration. [Go benchmark](smtcore/smt_quantifier_candidate_benchmark_test.go)

Across eight fresh single-goroutine processes pinned to CPU 16, the baseline at `9509d8fe6` used **2,688 additional live bytes per solver in scope and 2,640 after pop** in every run. These are deltas from the prepared fixture. They include the solver's reusable high-water storage. A candidate optimization can use this same fixture to measure its added cost after a registration has been retired. [Eight population measurements](perf/ivy_apple/candidate_checker/worker-memory.csv) · [Comparison driver](perf/ivy_apple/candidate_checker/worker_memory.py)

```sh
go test -c -o /tmp/g3-worker.test ./smtcore
python3 perf/ivy_apple/candidate_checker/worker_memory.py --go baseline /tmp/g3-worker.test --cpu 16
```

### Where Boolean variable retirement costs instructions

I placed a thread counter around each of the 128 variable retirements in a scoped pop. Each phase has its own binary and just one enable/disable pair inside the variable loop. This avoids charging inactive probes to another phase. I subtracted each engine's empty interval on the same CPU and in the same round. The table shows the median across nine alternating pairs per CPU, with 1,200 measured pops per run on CPU 32 and 1,800 on CPU 27. All 252 counter readings ran without multiplexing. The Go solver includes the singleton watch clear from `9509d8fe6`. [Raw measurements](perf/scope_restoration/pop_phases/variable-thread.csv) · [Paired measurement driver](perf/scope_restoration/pop_phases/variables.py)

| User instructions per variable | g3, CPU 32 | Native, CPU 32 | g3, CPU 27 | Native, CPU 27 |
|---|---:|---:|---:|---:|
| Complete Boolean retirement body | 456.2 | 130.3 | 456.6 | 130.3 |
| Activity queue deletion and its guards | 207.7 | 85.3 | 207.8 | 85.3 |
| Quantifier/index checks and binding removal | 54.0 | 9.0 | 54.0 | 9.0 |
| Clear and shorten per-variable Go storage | 95.7 | 0.0 | 96.5 | 0.0 |
| Release the owning reference | 64.0 | 24.0 | 64.0 | 24.0 |
| Everything after the queue through the end of the body | 235.9 | 33.0 | 236.5 | 33.0 |

Queue deletion accounts for **122 of the 326 excess instructions per variable**, or about **15,670 per 128-variable pop**. Clearing and shortening the Go vectors accounts for another 96 per variable. Native does that portion of its work when publishing a variable again. The complete-pop measurement includes republication. The Go binding interval includes equality unindexing when applicable and updates to the solver's expression record. The reference interval includes the resumable Go release and native's owning `expr_ref_vector::pop_back`. The enclosing trail loop falls outside the measured variable body. The individual intervals use separate compilations and can have different register spills. [Go retirement](smtcore/smt_internalizer_state.go) · [Native retirement](../src/smt/smt_internalizer.cpp)

A separate 1,000-pop Go count found an average of 51.64 queue entries at the last heap position, 23.53 at another leaf and 52.83 at an interior position. Most retirements therefore need the source heap's replacement ordering. The next queue experiment compares deletion using the already known inverse position and an inlined activity comparison, including ties. [Heap position counter](perf/scope_restoration/pop_phases/heap_positions.py)

Run the following command on Linux/amd64 from `pkg/` with the in-tree native build available.

```sh
python3 perf/scope_restoration/pop_phases/variables.py --cpu 0 --pairs 9 --iterations 1808 \
  --dir /tmp/g3-retired-variables > /tmp/g3-retired-variables.csv
```

### Instructions on the solver thread for each Apple check

GoIvy runs each incremental check on a locked OS thread for this measurement. A Linux hardware counter measures user-mode instructions on that thread between entry and return from the same adapter call in each backend. Each check also has an immediately preceding empty interval. The analysis subtracts that control and rejects multiplexed readings. Other threads, including concurrent GC workers, can contribute to the existing process-wide measurements. Each binary must reproduce the entire reference stdout before the driver emits any data. These are medians of four pairs each on `cf_live` and `dramc_nb2` and five each on `rfn` and `rfn2`, at g3 `dc99ddee8` and GoIvy `3ef4b9020`. [Measurements for all 2,874 checks in both engines](perf/ivy_apple/incremental_cost/solver-thread-counts.csv.gz) · [Measurement driver](perf/ivy_apple/incremental_cost/thread_checks.py) · [Counter boundaries](perf/ivy_apple/incremental_cost/thread_counter.go.txt)

| Isolate | Checks | g3, all checks (B instructions) | Z3, all checks (B) | Largest excess at check | g3 there (B) | Z3 there (B) | Share of isolate gap |
|---|---:|---:|---:|---:|---:|---:|---:|
| `dramc_nb2` | 2,479 | 60.92 | 15.70 | 71 | 24.92 | 6.99 | 40% |
| `rfn2` | 26 | 111.41 | 35.48 | 4 | 59.05 | 20.86 | 50% |
| `rfn` | 33 | 97.44 | 29.50 | 5 | 51.48 | 17.02 | 51% |
| `cf_live` | 336 | 123.16 | 34.00 | 130 | 39.95 | 12.70 | 31% |

The largest gap on each isolate is the same obligation identified by process-wide sampling. On `dramc_nb2` check 71 alone, matching Z3 would save 17.93 billion of the 45.21 billion extra instructions g3 executed in solver calls. Frontend translation and time spent on other threads lie outside these intervals. [Obligation names and source locations](perf/ivy_apple/incremental_cost/nine-isolate-largest-gaps.csv)

Three separate pairs counted all process threads, enabling their counters only inside or only outside solver calls. Inside `dramc_nb2` checks, the medians were 63.15 billion instructions in g3 and 15.74 billion in Z3. Outside checks, including frontend translation, g3 used 457.26 billion and native used 605.87 billion. On check 71 alone all process threads used 25.89 billion and 6.99 billion. This explains how a complete g3 invocation can finish earlier even as its solver thread spends substantially more on the same proof. [Process-wide phase measurements](perf/ivy_apple/incremental_cost/dramc-phases.csv) · [Gating driver](perf/ivy_apple/phase_counters.py)

Run from `pkg/` on Linux/amd64 with access to hardware counters, passing complete known-good stdout from the same input path.

```sh
python3 perf/ivy_apple/incremental_cost/thread_checks.py --goivy "$HOME/ivy/goivy" \
  --spec "$HOME/ivy/ivy-lang-examples/doc/examples/apple/ord_live.ivy" \
  --isolate dramc_nb2 /tmp/ord-dramc-native.stdout --cpu 0 --pairs 3 \
  --output /tmp/ord-thread-runs > /tmp/ord-thread-counts.csv
python3 perf/ivy_apple/incremental_cost/rank_checks.py /tmp/ord-thread-counts.csv
```

### Preparing frequently checked quantifier bodies

The same quantifier body can be checked against many bindings. Z3 follows AST pointers and reads tags and children in place. The previous Go checker repeatedly recovered the same operator, child handles and declaration flags from the arena. After a registration receives its third candidate, g3 can now index its immutable syntax once. The checking goroutine reads its own current assignments, relevance, equivalence roots and AST reference counts on each candidate. A successful registry deletion releases the prepared syntax. Bodies above the preparation limits, one-off checks, tracing, handle-validation builds and armed failure injection use the ordinary walker. [Go body preparation](smtcore/smt_candidate_body.go) · [Go evaluation](smtcore/smt_checker_candidate.go) · [Native checker](../src/smt/smt_checker.cpp)

These are medians of 11 rotated rounds on CPU 42. Each row measures 100,000 queries with identical bindings. “Direct” starts at the checker. “Queue” includes g3’s dispatch of the already-satisfied candidate. The standalone g3 API has no quantifier registration. It took 2,156 ns for the final row in the new binary. The extra benchmark entry calls the same registered checker used by the queue. [Individual samples](perf/ivy_apple/candidate_checker/prepared-speed.csv) · [Go benchmark](smtcore/smt_quantifier_candidate_benchmark_test.go) · [Native benchmark](../src/test/perf/quantifier_candidate.cpp)

| Equalities | Match | g3 queue before | g3 queue after | g3 registered direct | Z3 direct |
|---:|---|---:|---:|---:|---:|
| 4 | First | 411 ns | 315 ns | 178 ns | 107 ns |
| 4 | Last | 761 ns | 499 ns | 357 ns | 228 ns |
| 16 | First | 412 ns | 324 ns | 179 ns | 107 ns |
| 16 | Last | 2,254 ns | 1,185 ns | 1,042 ns | 799 ns |

The three-engine comparison used one pinned CPU per isolate and measured every check on the solver thread. The table gives medians of four runs per engine for `dramc_nb2` and `cf_live`, and five for `rfn` and `rfn2`. The baseline predates this change, with GoIvy fixed at `3ef4b9020`. Each number counts billions of user instructions after subtracting the paired empty interval. Native’s figures show how much of the gap remains when each solver performs the same search. [All check-level measurements](perf/ivy_apple/candidate_checker/prepared-thread-counts.csv.gz) · [Measurement driver](perf/ivy_apple/incremental_cost/thread_checks.py)

| Isolate | g3 before | g3 after | Z3 | Hardest check | g3 there, before → after | Z3 there |
|---|---:|---:|---:|---:|---:|---:|
| `dramc_nb2` | 61.08 | 57.42 | 15.70 | 71 | 24.99 → 22.61 | 6.99 |
| `rfn2` | 111.25 | 102.29 | 35.48 | 4 | 59.02 → 53.37 | 20.86 |
| `rfn` | 97.41 | 90.58 | 29.50 | 5 | 51.42 → 47.24 | 17.02 |
| `cf_live` | 122.85 | 115.54 | 34.00 | 130 | 39.98 → 37.08 | 12.70 |

The optimized build matched all 10,342 native Apple check results and 18 search counters per check. It also matched the 28 Raft reference variants and the 188 private Raft search records. On `rfn` check 29, 611,204 ordered records of candidates, temporary ASTs, disequality checks and congruence bucket registrations matched the previous build. This last check caught an early prototype that returned the correct verdict but reused a negative equality at a different reference count. A focused test now varies the owning references and checks which sort the solver’s last temporary equality holds. [Regression tests](smtcore/smt_checker_candidate_test.go)

Across nine runs with 1,024 independent solvers, per-solver live heap growth for one active 16-equality registration rose from 2,688 to 4,936 bytes. After pop, growth was 2,640 and 2,680 bytes respectively, relative to the original worker with the quantified AST already present. The 2,248-byte active premium buys a 47% reduction in the full-width queue check. The 40-byte post-pop difference measures retained scratch in this fixture. The registration bound limits preparation to 256 nodes, 1,024 child edges and depth below 128. [Worker measurements](perf/ivy_apple/candidate_checker/prepared-workers.csv) · [Worker driver](perf/ivy_apple/candidate_checker/worker_memory.py)

### Compact e-graph trails across independent solvers

Every Boolean variable, enode and equality previously put a 16-byte record on g3’s ordered rollback trail. Its largest field saved the current theory-attachment depth, even when there were no attachments. Z3 places each theory attachment in the same chronological trail as the equality operations it must precede during undo. g3 now uses a one-byte tag for each operation and keeps the payloads on their existing typed stacks. The owning Boolean trail and the expression-by-variable vector also share one handle array. Their separate lengths allow a reference release to resume after the live variable has been removed. [Go rollback](smtcore/smt_internalizer_state.go) · [Native Boolean and theory attachments](../src/smt/smt_internalizer.cpp) · [Rollback and retry tests](smtcore/smt_theory_equality_rollback_test.go)

The worker measurements cover nine alternating pairs of fresh processes. Each worker has its own solver and manager. Its original fixture stays live while the benchmark measures the incremental heap retained after warming and popping its scope. The two populations exercise different peak sizes. [Worker samples](perf/scope_restoration/compact_trail/workers.csv) · [Comparison driver](perf/scope_restoration/worker_memory.py) · [Worker benchmark](smtcore/smt_scope_workers_benchmark_test.go)

| Independent solvers | Atoms each | Before, bytes per solver | Compact trail | Saved |
|---:|---:|---:|---:|---:|
| 1,024 | 128 | 69,998 | 64,141 | 5,857 (8.37%) |
| 48 | 2,048 | 911,467 | 832,643 | 78,824 (8.65%) |

On two physical cores, median instructions in the scoped trail-undo loop fell from 62,797 to 61,373 and from 62,906 to 61,215. Native used 19,653 on each core. Complete-pop medians increased 1.60% and 0.16% respectively. Across 24 paired timings, the queue-enabled scoped pop took 2.01% less elapsed time in the median and won 14 pairs. The shorter surviving-atom control took 4.52% longer in the median. The repeatable benefit is the reduction in resident memory. Every process verified restored clause identity and bindings. The final solver matched all 10,342 Apple checks and their search counters, the 28 Raft variants and the 188 private search records. [Hardware counters](perf/scope_restoration/compact_trail/thread-instructions.csv) · [Elapsed samples](perf/scope_restoration/compact_trail/times.csv)

A forced failure between removing a theory variable from a node and removing it from the equivalence root exposed a separate ownership issue. The previous implementation had already discarded the payload. The compact trail leaves it in place and marks the first removal complete, so a retry can finish the second removal before undoing the older equality. The regression test makes the root attachment temporarily unavailable, restores it, and checks both final classes and their theory variables. [Interrupted-pop test](smtcore/smt_theory_equality_rollback_test.go)

### How often proofs rebuild an existing AST application

On `dramc_nb2`, 4,015,983 of 4,675,807 eligible application requests (85.9%) identified a term already in the solver’s canonical table. Check 71 alone found 706,419 existing terms. For this measurement an observer probed the unchanged table before the usual construction and registration. It compared the declaration, arity and complete argument handles. The ordinary registration still ran on every request. The observer covered the full release-mode constructor taking up to three inline arguments from a slice. The final column counts duplicates found by ordinary registration across **all** recorded arities and input forms. Each isolate reproduced native’s full transcript and 18 search counters per check. A second run with the checked-in driver reproduced all 130 check-by-arity rows for `rfn2`. [All 2,874 checks by arity](perf/ivy_apple/application_reuse/counts.csv.gz) · [Counting driver](perf/ivy_apple/application_reuse/count.py)

| Isolate | Constructor requests | Eligible requests | Existing terms | Found in first bucket | All registration duplicates |
|---|---:|---:|---:|---:|---:|
| `dramc_nb2` | 5,105,239 | 4,675,807 | 4,015,983 | 3,320,863 | 4,031,934 |
| `rfn2` | 3,892,613 | 3,501,523 | 2,668,746 | 2,256,428 | 2,678,024 |
| `rfn` | 3,473,503 | 3,082,253 | 2,217,110 | 1,860,877 | 2,223,577 |
| `cf_live` | 8,979,881 | 8,361,692 | 7,193,276 | 6,383,958 | 7,220,944 |

The corresponding microbenchmarks construct the same applications with the two arena-backed AST managers. Repeating the same binary application took **155.0 ns in g3 and 38.20 ns in native Z3**. Rotating through 256 binary applications and retaining them all raised those medians to 184.1 and 39.23 ns, including acquisition and release of the result. Releasing the last owner every time raised the cost to 406.9 and 54.85 ns. Four arguments cross the inline-storage boundary in Go. At that arity, the single repeated application took 362.8 and 42.74 ns. Nine alternating pairs used CPU 30 and 200,000 calls per workload. These times measure a constructor and its specified ownership operations. They do not estimate the percentage of a whole proof spent there. [Timings for all 16 workloads](perf/ivy_apple/application_reuse/baseline.csv) · [Go fixtures](ast/ast_app_benchmark_test.go) · [Native fixtures](../src/test/perf/ast_application_reuse.cpp)

To repeat the timing from `pkg/` with the project's release C++ build in `../build`, use a permitted logical CPU for this machine.

```sh
go test -pgo=off -c -o /tmp/g3-ast-reuse.test ./ast
python3 perf/ivy_apple/application_reuse/compare.py --cpu 0 --rounds 9 --go g3 /tmp/g3-ast-reuse.test
```

### Removing a Boolean variable from the activity heap

Z3 retrieves an integer’s position in the case-split heap and moves the last entry into the resulting hole. g3 previously checked membership at the caller, looked up the position again in `Erase`, wrote the replacement and then read it back to sift. The Go queue now passes the position and replacement directly to the same activity sift used by priority updates. Strict score comparisons preserve the order of ties, including unordered floating-point scores. A 20,000-operation test compares the complete array ordering with the generic comparator heap after each operation. Checked and debug builds also ran the owner, epoch-exhaustion, invalid-position and score-ordering tests. [Go queue](smtcore/smt_activity_heap_queue.go) · [Heap implementation](util/heap/heap.go) · [Comparison and boundary tests](util/heap/activity_test.go) · [Native source](../src/util/heap.h)

Twelve rounds on each of two physical cores measured the deletion callback for every one of the 128 retired variables. On both cores the cost fell from **208 to 192 user instructions per variable**. Native used **85**. The complete trail undo used 3.5% and 3.3% fewer instructions, respectively. The complete scoped pop used 1.2% and 2.8% fewer. Each figure subtracts its own empty interval and every recorded counter ran for its full enabled time. The position counter still saw 51.64 last entries, 23.53 other leaves and 52.83 interior entries per pop. A separate release-only implementation duplicated both sift loops and reached 179 instructions. The shared sift keeps priority-update and deletion ordering in one implementation. [Per-round hardware counters](perf/scope_restoration/activity_erase/instructions.csv) · [Alternative sift measurements](perf/scope_restoration/activity_erase/sift-experiments.csv) · [Position counter](perf/scope_restoration/pop_phases/heap_positions.py)

Across 36 paired elapsed-time measurements on CPUs 16 and 20, the queue-enabled scoped medians were **21.81 µs before, 19.21 µs after and 13.46 µs in native Z3**. The optimized build won 30 pairs, with a median within-pair reduction of 7.95%. Surviving atoms retired no queue entries and measured a within-pair change of +0.12% (18 wins). A separate series on CPUs 12 and 31 overlapped with the counter measurements and produced substantially longer absolute times. Its 24 pairs gave an 11.43% median scoped reduction. These wall-clock results include effects of the shared machine. The instruction savings identify the portion attributable to less queue work. [36 elapsed-time pairs](perf/scope_restoration/activity_erase/elapsed.csv) · [Concurrent elapsed series](perf/scope_restoration/activity_erase/concurrent-elapsed.csv) · [Timing driver](perf/scope_restoration/watch_selection/compare.py)

The heap change reproduced all 10,342 Apple results and their 18 per-check search counters. All 28 Raft variants and 188 records from the private Raft proof searches also matched the previous solver.

### Releasing owned references during relevance rollback

Relevance rollback first unmarks popped expressions, then releases the references in insertion order. In this fixture, an external owner keeps each atom alive. g3 previously checked whether the AST manager could proceed on every decrement. It can check once for a run of shared references because decrementing a count greater than one cannot invoke a destructor. A last reference follows the ordinary destructor path. If a destructor interrupts the batch, the vector retains exactly the references it still owns for a retry. Rewriter root vectors now use the same operation. [AST reference handling](ast/ast_core_accessors.go) · [Relevance rollback](smtcore/smt_relevancy.go) · [Destructor regression](ast/ast_core_delete_continuation_test.go)

Nine interleaved comparisons on CPU 43 measured 2,400 pops per process after warmup, with an empty control for every engine and round. Relevance rollback used **25,200 instructions before and 19,357 after**, a 23.2% reduction. Z3 used 8,036. The complete scoped pop fell from **224,545 to 219,153** instructions, with Z3 at **144,720**. An earlier eleven-round run on the same core measured 25,214 and 19,356 in the relevance phase. The final run includes the query guards for an interrupted pop. Each counter ran throughout its entire enabled interval. [Final measurements](perf/scope_restoration/held_references/retry-safe.csv) · [Earlier measurement with trail and clause phases](perf/scope_restoration/held_references/instructions.csv) · [Pinned benchmark driver](perf/scope_restoration/pop_phases/measure.py)

The error tests exercise both possible ownership outcomes. A numeric destructor fails after consuming its input. A manager callback refuses a watch decrement before consuming it. Retrying the first must preserve the next reference. Retrying the second must preserve the older handler for the same expression. The previous relevance implementation could decrement an external owner's reference on retry or lose the refused watch reference on cleanup. [Relevance and watch retry tests](smtcore/smt_relevancy_retirement_test.go)

The complete release-binary instruction measurements compare the heap change alone (`heap`) with the additional batched decrements (`roots`), before the interrupted-query guards. Median within-pair reductions were **0.35% on Apple `rfn`**, **0.33% on `rfn2`**, **0.08% on `cf_live`**, and **0.58% on the 419 Raft obligations**. The lower count occurred in 7/7, 7/7, 3/5 and 13/14 pairs respectively. On `cf_live` the change was smaller than the run-to-run variation. [Apple process counters](perf/scope_restoration/held_references/apple.csv) · [Raft process counters](perf/scope_restoration/held_references/raft.csv)

With the query guards in place, all 10,342 Apple results and their 18 search counters matched. The 28 Raft variant outputs and 188 private search records also matched.

### Where time goes inside one restored learned clause

The fixture restores 64 learned ternary clauses per pop. A pair of hardware reads bracketed one operation in every clause, in separate Go and in-tree native builds. Each row is the median of eleven alternating pairs on CPU 15. Every process measured **115,200 clause visits** after warmup. Counts below subtract the same engine's empty interval for that round. The phases have separate instrumented binaries, so instruction scheduling and call boundaries can differ slightly. [Go clause restoration](smtcore/smt_clause_backtrack.go) · [Native restoration](../src/smt/smt_context.cpp) · [Observer and build driver](perf/scope_restoration/pop_phases/clauses.py)

| Work measured per restored clause | g3 instructions | Z3 instructions |
|---|---:|---:|
| Complete clause | 1,727 | 1,382 |
| Restore literal bindings | 1,116 | 1,040 |
| Select watches | 99 | 117 |
| Install watches | 91 | 63 |
| Complete the clause after installing watches | 385 | 150 |
| ↳ Check assignments on the watched literals | 47 | 9 |
| ↳ Release the three retained atom references | 310 | 137 |

Releasing the saved atoms accounts for **173 of the 345 additional instructions per restored clause**, about half the gap within that operation. The independent second series reproduced the enclosing post-watch interval at 384 instructions in g3 and 150 in Z3. In native code `clause::release_atoms` walks tagged expression pointers. In this g3 build every saved atom carries a 16-byte AST handle and a sign, and each decrement independently checks the manager's state. Solver-local references that preserve the arena's ownership and retry rules are a concrete target. [First series](perf/scope_restoration/pop_phases/clause-instructions.csv) · [Assignment and reference-release series](perf/scope_restoration/pop_phases/clause-retirement.csv) · [Native reference release](../src/smt/smt_clause.cpp)

To repeat both sets of boundaries in one run with a fresh output directory, use an available Linux/amd64 CPU and the in-tree release build.

```sh
python3 perf/scope_restoration/pop_phases/clauses.py --cpu 0 --iterations 1808 \
  --pairs 11 --dir /tmp/g3-restored-clauses > /tmp/g3-restored-clauses.csv
```


### Where the rewriter spends instructions on the difficult Apple checks

On `rfn2` check 4, **24% of g3's instruction samples** had the solver rewriter on the stack. On `dramc_nb2` check 71 the share was **31–32%**. Two frame-pointer profiles per check sampled every 800,011 user instructions. They cover only the selected solve call and verify the complete isolate output. These runs include the prepared candidate checker, activity-queue change and batched root retirement, preceding the interrupted-relevance query guards. [Operation counts](perf/ivy_apple/rewriter_profiles/frame-solver-operations.csv) · [Sampling driver](perf/ivy_apple/phase_stacks.py)

| Physical frame directly below the innermost rewriter frame | `rfn2` check 4, two runs | `dramc_nb2` check 71, two runs |
|---|---:|---:|
| RewriterMakeApp | 3,948 / 4,090 | 3,169 / 2,840 |
| ReduceApp | 3,324 / 3,185 | 1,300 / 1,238 |
| cacheResult | 1,401 / 1,349 | 760 / 735 |
| GetSubst | 1,199 / 1,191 | 711 / 788 |
| All samples with a rewriter frame | 16,824 / 16,619 | 9,352 / 9,036 |
| All samples in the selected call | 68,991 / 68,971 | 28,820 / 29,374 |

One stack contributes to one callee row. A sample attributed to `RewriterMakeApp` includes work deeper in that call, such as checking argument sorts, finding the canonical application and handling references. The two profiles per check independently identify application construction as a larger target than the result cache. Physical frames omit inlined calls. The operation CSV also provides inclusive categories that can overlap. Use `phase_stacks.py --engines g3 --call-graph fp --period 800011` and pass the completed run directories to `stack_costs.py --rewriter-callees --period 800011` to reproduce the classification. [All callee samples](perf/ivy_apple/rewriter_profiles/frame-rewriter-callees.csv) · [Classification code](perf/ivy_apple/stack_costs.py)

A separate eight-pair release-binary timing series measured the original prepared-candidate change on five complete workloads. Median within-pair elapsed changes were −5.39% on `rfn`, −2.38% on `rfn2`, −2.27% on `dramc_nb2`, +1.89% on `cf_live` and +2.03% on Raft. Each row records host and sibling activity. The direct queue and solver-thread instruction measurements above isolate the checker improvement more precisely than these whole-process times. [All 80 release runs](perf/ivy_apple/candidate_checker/prepared-release-wall.csv)

### Compact saved atoms in learned clauses

A learned clause can survive the Boolean variables that originally supplied its literals. It therefore retains the original expressions and their signs. The previous representation stored a 16-byte public handle and one sign in a 24-byte record. The clause now keeps the arena slot and its full generation in eight bytes, with one additional bit per sign. The buffer stores the common AST manager once. Acquiring a reference validates ownership. Reading a position supplies the original public handle. Release walks forward, stops at the first failure, and clears each position as soon as it consumes the reference. [Clause storage](smtcore/smt_clause.go) · [Manager-owned references](ast/ast_retained_literals.go) · [Generation, cross-manager and sign tests](ast/ast_retained_literals_test.go) · [Interrupted destructor test](ast/ast_core_delete_continuation_test.go)

Across ten paired runs of 1,024 solvers, the median additional live heap per solver after pop fell from **64,141 to 59,381 bytes**. At 48 solvers with 2,048 atoms each the medians were 832,643 and 827,884 bytes. The similar absolute saving reflects the fixed short-clause pool capacity. In an earlier eight-round layout comparison, a 12-byte record saved 3,904 bytes per worker. Keeping the first 64 sign bits inline saved a further 40 bytes over the selected bit slice. [Final worker measurements](perf/scope_restoration/saved_clause_atoms/workers.csv) · [Four layouts](perf/scope_restoration/saved_clause_atoms/layout-workers.csv) · [Worker benchmark](smtcore/smt_scope_workers_benchmark_test.go)

Each of nine paired instruction runs per CPU measured 115,200 three-atom releases and 1,800 complete pops per process. Reading and reconstituting handles costs extra instructions when rebinding the original expressions. Release recovers that cost in this fixture. The complete-pop counter varied more than the measured clause phase and moved in opposite directions across the two CPUs. [Per-round counters and empty controls](perf/scope_restoration/saved_clause_atoms/instructions.csv) · [Clause phase driver](perf/scope_restoration/pop_phases/clauses.py) · [Complete pop driver](perf/scope_restoration/pop_phases/measure.py)

| Instructions, median after subtracting the empty interval | CPU 33 before → after | CPU 38 before → after |
|---|---:|---:|
| Rebind one ternary clause's atoms | 1,114 → 1,164 | 1,078 → 1,137 |
| Release its three atoms | 310 → 207 | 305 → 201 |
| Complete the clause after installing watches | 386 → 282 | 381 → 277 |
| Restore all clauses in one pop | 116,919 → 115,495 | 112,164 → 109,806 |
| Complete scoped pop | 213,240 → 215,695 | 205,965 → 202,705 |

In a separate elapsed series, queue-enabled scoped pop improved in 25 of 32 pairs, with a median within-pair reduction of **5.28%**. For surviving atoms, the medians were 326 ns before, 344 ns after and 512 ns in Z3. That control was 4.29% slower within pairs. All timings used the same in-tree native fixture, 2,800 iterations per run and alternating build order on CPUs 12 and 46. [Elapsed runs, including the queue-disabled control](perf/scope_restoration/saved_clause_atoms/elapsed.csv) · [Timing driver](perf/scope_restoration/watch_selection/compare.py)

The final build reproduced the verdict and all 18 search counters for every one of the 10,342 Apple checks. It also matched all 28 Raft variant outcomes and 188 private Raft search records. Both AST and SMT core package suites passed, along with focused runs using checked and debug/trace builds. [Release and partial-failure tests](ast/ast_retained_literals_test.go) · [Pool reuse with mixed clause lifetimes](smtcore/smt_clause_storage_test.go)


Across complete release runs the clause change alone had a much smaller effect than it did on scope restoration. Median within-pair instruction changes were −0.21% on `rfn`, −0.03% on `rfn2`, −0.30% on `cf_live`, and +0.10% on Raft. There were seven, seven, six and twelve pairs, respectively. The principal benefit is the retained memory per solver. [Apple runs](perf/scope_restoration/saved_clause_atoms/apple-process.csv) · [cf_live runs](perf/scope_restoration/saved_clause_atoms/cf-process.csv) · [Raft runs](perf/scope_restoration/saved_clause_atoms/raft-process.csv)

### What a canonical application costs on the benchmark thread

The Go and in-tree C++ fixtures repeatedly construct applications that share the same declaration and arguments. A hardware counter now brackets the 200,000 calls on the executing OS thread, excluding setup and teardown. The rotating fixture also acquires and releases the result on every iteration. Each loop measurement has an empty-control run for the same engine, workload and round. The reported results subtract that control before dividing by the iteration count. Nine rounds on CPU 40 ran every counter throughout its enabled interval. [Instruction counts](perf/ivy_apple/application_reuse/constructor-instructions.csv.gz) · [Per-thread measurement driver](perf/ivy_apple/application_reuse/instructions.py) · [Go benchmark](ast/ast_app_benchmark_test.go) · [Native benchmark](../src/test/perf/ast_application_reuse.cpp)

| Instructions per request | g3 before | Reuse validated operands | Also return the inline arena slot directly | Also specialize the table comparison | Native Z3 |
|---|---:|---:|---:|---:|---:|
| Same unary application | 1,605 | 1,168 | 1,065 | 1,037 | 406 |
| Same binary application | 1,922 | 1,275 | 1,172 | 1,138 | 456 |
| Same ternary application | 2,183 | 1,339 | 1,236 | 1,179 | 500 |
| Same four-argument application | 4,333 | 2,966 | 2,966 | 2,982 | 605 |
| Rotating binary, none retained | 4,012 | 3,319 | 3,323 | 3,334 | 641 |
| Rotating binary, all retained | 2,179 | 1,532 | 1,429 | 1,395 | 465 |

The prototype's short match uses the existing canonical table and the complete arena references. A miss uses the prevalidated operands for child acquisition. Returning a just-placed inline duplicate directly saves a further 103 instructions in each of the first three rows. It has no separate argument payload or acquired children. Four-argument applications allocate and reclaim the ordinary payload even on a hit. The remaining gap is largest for a new application that must be destroyed before the next request. Independent timings on CPU 9 put the repeated binary case at 152 ns before, 94 ns with direct slot return and 43 ns in Z3. [Independent timings, ten rounds](perf/ivy_apple/application_reuse/constructor-timings.csv.gz)

To repeat the instruction study on a Linux/amd64 host, select a permitted CPU and a fresh output directory. Add repeated `--go NAME OVERLAY` options to compare local Go variants against the same in-tree C++ build.

```sh
python3 perf/ivy_apple/application_reuse/instructions.py --cpu 0 \
  --dir /tmp/g3-app-instructions --rounds 9 > /tmp/g3-app-instructions.csv
```

### Reusing validated operands in the arena-backed application constructor

A repeated binary application now takes **84 ns and 1,177 instructions**, down from 156 ns and 1,922 instructions. Native Z3 used 37 ns and 456 instructions in the same fixtures. The implementation places the candidate in the AST arena and reads each operand’s identity and hash once. The canonical table compares the declaration and every operand. An exact inline match has already passed the sort checks, and its temporary slot returns directly to the free list. A new application checks its domain before publication and acquires the validated children. Four to sixteen arguments use the ordinary arena payload even on a match. Coercion, diagnostics, failure injection and wider inputs retain their checked construction paths. [Application constructor](ast/ast_app_slice.go) · [Collision, ownership and generation tests](ast/ast_app_slice_test.go)

| Instruction cost per request | g3 before | g3 after | Native Z3 |
|---|---:|---:|---:|
| Repeated unary | 1,605 | 1,071 | 406 |
| Repeated binary | 1,922 | 1,177 | 456 |
| Repeated ternary | 2,183 | 1,241 | 500 |
| Repeated four-argument | 4,333 | 2,970 | 605 |
| Rotating binary, all 256 retained | 2,179 | 1,434 | 465 |
| Rotating binary, newly created and destroyed | 4,012 | 3,324 | 641 |

Nine rounds on CPU 33 measured instructions on the benchmark thread. Twelve timing rounds on CPU 17 used the uninstrumented Go and C++ benchmarks. Each round executed 200,000 requests per case. The four-argument timing fell from 336 to 220 ns, compared with 42 ns in native Z3. [Instruction counts and corresponding empty intervals](perf/ivy_apple/application_reuse/release-instructions.csv.gz) · [Uninstrumented timings](perf/ivy_apple/application_reuse/release-timings.csv.gz)

| Complete proof | Paired runs | Before, billion instructions | After, billion instructions | Median paired instruction change | Median paired wall change |
|---|---:|---:|---:|---:|---:|
| Apple `rfn` | 7 | 132.68 | 127.59 | −3.84% | −3.05% |
| Apple `rfn2` | 7 | 152.02 | 146.33 | −3.72% | −6.39% |
| Apple `cf_live` | 6 | 221.25 | 210.36 | −4.91% | +1.02% |
| Raft, 419 obligations | 12 | 30.36 | 29.44 | −3.08% | −0.11% |

All 32 before-and-after pairs retired fewer instructions. The shared machine gave mixed elapsed times for Raft and `cf_live`. A separate verification compared all 10,342 Apple outcomes and 18 counters per check with native Z3, all 28 Raft variants with their reference transcripts, and all 188 private Raft search records with the previous g3. The independent application observer reproduced all 130 `rfn2` check-by-arity rows, including registration duplicates. [Whole-proof times and PMU counts](perf/ivy_apple/application_reuse/release-proofs.csv) · [Application observer](perf/ivy_apple/application_reuse/count.py)

### Instructions inside one repeated application

For a repeated binary application at `5719b5678`, dispatch and preparation show the largest measured differences from Z3. Dispatch runs from public constructor entry to the arena placement call. Preparation starts when placement returns and ends at the canonical table lookup. It includes declaration checks, child identity and hash reads, and, at arity four, the separately owned argument payload. The Go implementation spends 241 more instructions in binary dispatch and 197 more in preparation. Placement itself differs by 45. At arity four, preparation alone differs by 1,252 instructions. [Per-stage measurements](perf/ivy_apple/application_reuse/release-phases.csv.gz) · [Phase driver and instrumentation boundaries](perf/ivy_apple/application_reuse/phases.py)

| Stage, instructions per request | Binary before | Binary after | Binary native | Four before | Four after | Four native |
|---|---:|---:|---:|---:|---:|---:|
| Dispatch | 359 | 283 | 42 | 374 | 298 | 45 |
| Place candidate node | 83 | 82 | 37 | 83 | 82 | 37 |
| Prepare candidate for lookup | 864 | 408 | 211 | 2,812 | 1,605 | 353 |
| Look up the existing application | 352 | 256 | 163 | 584 | 505 | 166 |
| Return duplicate storage | 116 | 30 | 34 | 330 | 352 | 34 |

Each stage used its own instrumented executable, with one counter interval per constructor request and an empty interval measured in a separate executable. Eight rounds on each of CPUs 37 and 41 counted 50,000 intervals per arity in every run. Both cores produced the same rounded medians in the table. Instrumentation changes code generation around the boundaries. Use whole-loop counts for the total cost and compare individual rows across engines to locate the differences.

A separate sampling run used the ordinary executables, four million requests per case and three rounds. In the repeated binary fixture, the largest Go leaf counts were in slice registration, declaration flags and the application constructor. For four arguments, arena descriptor access and sort lookup also ranked highly. When every result immediately lost its last reference, publishing and deleting the node accounted for about 351 and 339 sampled instructions per request. Native Z3’s corresponding registration and deletion functions accounted for about 141 and 125. Each estimate multiplies leaf samples by the sampling period and divides by requests. Inlining assigns work to the containing native symbol. The most promising follow-ups for duplicates are passing already-read declaration flags into the constructor and using the wide payload’s existing descriptor validation. Fresh nodes also warrant a separate measurement of publication and deletion. [Raw leaf counts by executable and round](perf/ivy_apple/application_reuse/release-leaf-samples.csv.gz) · [Sampling driver](perf/ivy_apple/application_reuse/leaves.py)

### Manager-local references in the rewriter cache

An activity cache belongs to one AST manager. Its entries and eviction queue can identify an expression by its arena slot and generation, saving the eight-byte manager identity in each stored handle. Incoming handles still carry their manager identity. When the key and value are live expressions of this manager, insertion uses their validated records to acquire the references. Evicting an entry can retire either operand, so insertion checks again on that path. The cache keeps the original chained-table traversal and reference release order. Regressions cover equal slot numbers in different managers, a value retired during eviction, and a failed insertion after acquiring the key. [Implementation](ast/ast_activity_cache.go) · [Ownership tests](ast/ast_activity_cache_test.go)

| Measurement | Before | After |
|---|---:|---:|
| 256 live cache entries per manager | 28,896 B | 16,592 B |
| Same cache after reset | 28,896 B | 16,592 B |
| Find an existing entry | 23.16 ns | 21.41 ns |
| Reset and reinsert 32 entries | 4.18 µs | 2.72 µs |
| Create, insert, look up and close a one-entry cache | 624 ns, 720 B | 477 ns, 504 B |

The heap benchmark measured bytes above the same rooted expressions with 1,024 independent managers and eight fresh processes per executable. Reset retains backing arrays for reuse. The timing medians use 12 alternating pairs of uninstrumented binaries on CPU 31. Find and the 32-entry cycle allocated zero bytes per operation in both builds. To reproduce, compile the AST test binary with `go test -c -o /tmp/cache.test ./ast`, then run `python3 perf/ivy_apple/rewriter_cache/compare.py --go current /tmp/cache.test --cpu 1 --rounds 8`. Add `--memory` for the live-heap experiment. To repeat the comparison at the parent, supply the same worker benchmark to that checkout and pass a second `--go` argument. [Worker benchmark](ast/ast_activity_worker_memory_benchmark_test.go) · [Timing fixtures](ast/ast_activity_cache_benchmark_test.go) · [Raw heap measurements](perf/ivy_apple/rewriter_cache/release-memory.csv) · [Raw timings](perf/ivy_apple/rewriter_cache/release-micro.csv)

| Complete proof | Pairs | Before, billion instructions | After, billion instructions | Median paired instruction change | Median paired wall change |
|---|---:|---:|---:|---:|---:|
| Apple `rfn` | 7 | 127.49 | 126.07 | −1.11% | −3.58% |
| Apple `rfn2` | 7 | 146.35 | 144.54 | −1.15% | −4.01% |
| Apple `cf_live` | 6 | 211.77 | 208.81 | −1.28% | −0.87% |
| Raft, 419 obligations | 12 | 29.39 | 29.07 | −1.14% | −7.12% |

The elapsed percentages are medians of within-pair changes on a shared machine. For `cf_live`, three of six pairs finished sooner. Hardware instruction counts decreased in every pair. A separate run matched all 10,342 native Apple outcomes and all 18 counters per check, all 28 Raft reference transcripts, and all 188 private Raft search records from the previous build. [Whole-proof measurements](perf/ivy_apple/rewriter_cache/release-proofs.csv)

### Whole Apple isolates and time inside their solver calls

GoIvy at `3ef4b9020` ran its uninstrumented g3 and `classic_z3` backends on the same CPU in alternating order, with PGO and tracing off. g3 used `ed3be7f3c` and native used GoIvy’s linked `z3vendor/native_lib/libz3.a`. All 28 runs reproduced their isolate’s full reference transcript. On this machine the previously long-running `dramc_nb2` completed all 2,479 checks in every run. Its three g3/native times were 117/163, 132/132 and 142/154 seconds. [All runs, times, memory and hardware counters](perf/ivy_apple/incremental_cost/release-native-proofs.csv) · [Whole-isolate driver](perf/ivy_apple/compare.py)

| Isolate | Checks | Whole-run pairs | Whole wall seconds, g3 / native | Whole instructions, billions, g3 / native | Instructions inside checks, billions, g3 / native |
|---|---:|---:|---:|---:|---:|
| `dramc_nb2` | 2,479 | 3 | 132.4 / 154.3 | 509.0 / 590.8 | 50.47 / 15.70 |
| `rfn2` | 26 | 4 | 47.9 / 31.4 | 144.3 / 73.1 | 96.67 / 35.48 |
| `rfn` | 33 | 4 | 29.6 / 20.0 | 126.0 / 60.9 | 85.14 / 29.50 |
| `cf_live` | 336 | 3 | 54.2 / 32.6 | 208.7 / 125.1 | 104.20 / 34.00 |

The last column uses separate runs with a per-thread user-mode counter around each verification call. Each call had a measured empty interval. The table reports the median of each run’s sum after subtracting those intervals. There were three complete four-build rounds on `dramc_nb2`, five on `rfn` and `rfn2`, and four on `cf_live`. Every interval ran without multiplexing. The counts across earlier g3 builds show where progress reached actual specifications. For `dramc_nb2`, the sum fell from 56.93 billion before the application change, to 51.80 billion with the new constructor, to 50.47 billion with the compact rewriter cache. For `rfn2` the corresponding totals were 102.12, 97.69 and 96.67 billion. Whole-process and solver-thread counters have different scope. Work between verification calls includes solver mutations as well as the Ivy frontend. [Per-check counts for all four builds](perf/ivy_apple/incremental_cost/release-check-instructions.csv.gz) · [Per-thread counter driver](perf/ivy_apple/incremental_cost/thread_checks.py)

Two earlier paired DWARF profiles per backend isolated the hardest individual call in each of these workloads. The g3 build included the prepared quantifier-body change and preceded the latest application and cache improvements. Each physical instruction sample contributes to its nearest named solver operation. The estimates below average the two profiles at 2,400,017 instructions per sample. Inlining and incomplete unwind information affect the attribution. [Exclusive and inclusive samples](perf/ivy_apple/rewriter_profiles/dwarf-solver-operations.csv.gz) · [Physical leaves grouped by their nearest operation](perf/ivy_apple/rewriter_profiles/dwarf-operation-leaves.csv.gz) · [Sampling driver](perf/ivy_apple/phase_stacks.py)

| Estimated billion instructions | `rfn2` check 4, g3 / native | `dramc_nb2` check 71, g3 / native |
|---|---:|---:|
| Full call | 55.22 / 20.86 | 23.58 / 6.99 |
| Solver rewriter | 9.96 / 1.89 | 4.92 / 1.08 |
| Quantifier queue | 6.98 / 1.38 | 4.88 / 1.00 |
| Application construction | 5.43 / 1.38 | 3.63 / 0.85 |
| Candidate checker | 6.19 / 4.54 | 2.56 / 1.63 |
| Congruence table | 3.23 / 0.98 | 1.15 / 0.26 |
| Scope pop | 2.92 / 1.20 | 1.32 / 0.42 |

On `rfn2` check 4, the solver rewriter, quantifier queue and application constructor together account for about 17.7 billion of the 34.4 billion additional instructions. The queue’s leaves include Boolean-variable lookup, clause construction and internalization. The rewriter’s include expression-substitution key validation, application traversal and activity-cache lookup. The largest candidate-checker leaf on both calls is the reference-count accessor, estimated at 1.45 billion instructions on `rfn2` and 0.69 billion on `dramc_nb2`. These measurements favor reusing validated arena records and eliminating repeated property reads along the existing solver paths. [Attribution rules and `--operation-leaves` option](perf/ivy_apple/stack_costs.py)

### Quantifier fingerprints after scope retirement

A long-lived fingerprint set reused a temporary slice of equivalence-class roots. A narrower lookup shortened its length but left roots reachable in the backing array. Popping all the fingerprints or resetting the set still kept those enodes alive. The set now tracks the widest part of the buffer used since the last pop, clears those references on pop, and releases the buffer on reset. A published fingerprint owns its copy of the roots, including when a caller keeps that fingerprint across a pop. [Implementation](smtcore/fingerprints.go) · [Wide tuple followed by a narrow miss, nested pop and reset](smtcore/fingerprints_test.go)

| Additional live Go heap per worker | Before | After |
|---|---:|---:|
| Wide fingerprint installed | 5,488 B | 5,488 B |
| Last fingerprint scope popped | 5,264 B | 656 B |
| Fingerprint set reset | 4,744 B | 8 B |

Each work group held 1,024 independent AST managers. Each worker inserted a tuple with 16 distinct roots, performed a one-argument miss, and popped the scope. Eight fresh processes per build returned the same values after forced collections. This saves 4.5 MiB across the 1,024 workers after pop. The 656 bytes include the table and scratch capacity retained for reuse. The benchmark measures incremental heap over the worker’s live ASTs. The separate arity-one, three and six timing fixtures had mixed results on the shared machine. The reason to keep this change is bounded memory after a worker retires its scope. Reproduce the heap measurement with `GOMAXPROCS=1 go test ./smtcore -run '^$' -bench '^BenchmarkFingerprintWorkerMemory$' -benchtime=1x -count=1`. [Worker benchmark](smtcore/smt_fingerprint_worker_memory_benchmark_test.go) · [Raw heap measurements](perf/ivy_apple/fingerprints/memory.csv) · [Lookup and insertion timings](perf/ivy_apple/fingerprints/micro.csv)

### Ordinary Raft and incremental Apple mutations

On the ordinary 419-obligation `raft_no_assume_test.ivy`, g3 finished sooner in all 14 matched pairs across two CPUs. Median elapsed times were 6.04 s for g3 and 59.74 s for linked native Z3. The median within-pair speedup was 10.13×. g3 retired 29.06 billion instructions and used 70,422 KiB peak RSS, compared with 215.26 billion and 321,100 KiB for native. These are whole-checker measurements at g3 `ed3be7f3c` and GoIvy `3ef4b9020`, with the same 419 outcomes as the reference. They include the Raft-specific solving improvements documented above. [Raw runs](perf/ivy_raft/release-native-checks.csv) · [Raft measurement driver](perf/ivy_raft/measure.py)

Apple also exercises the incremental solver *between* verification conditions. The following measurements instrument the shared adapter's Push, Assert, Pop and Reset API boundaries. The observer excludes calls made inside a verification condition and measures the calling thread, including synchronous preprocessing. Each entry is the median per-run sum in **billions of instructions, g3 / native**, after subtracting the corresponding empty intervals.

| Isolate | Pairs | Push | Assert | Pop | Reset |
|---|---:|---:|---:|---:|---:|
| `dramc_nb2` | 3 | 36.895 / 9.469 | 9.964 / 2.296 | 0.826 / 0.185 | 1.194 / 0.484 |
| `rfn2` | 4 | 13.140 / 3.531 | 1.243 / 0.290 | 0.220 / 0.053 | 0.462 / 0.155 |
| `rfn` | 4 | 11.981 / 3.215 | 1.130 / 0.264 | 0.147 / 0.037 | 0.422 / 0.136 |
| `cf_live` | 3 | 19.590 / 5.356 | 1.522 / 0.354 | 1.182 / 0.274 | 0.731 / 0.213 |

Each pair used the same GoIvy source at `a6d3417b1`, g3 at `ed3be7f3c`, and GoIvy’s linked native library. The complete ordered mutation sequences and proof transcripts matched. All intervals had full hardware-counter coverage. On `dramc_nb2` both engines executed 2,479 pushes, 2,586 assertions, 2,479 pops and 87 resets. Push alone accounted for 27.43 billion extra instructions. On `rfn2` there were only 26 pushes, with an excess of 9.61 billion instructions across them. Optimizing synchronous assertion processing at push will affect both workloads. [Mutation boundary driver](perf/ivy_apple/incremental_cost/mutation_costs.py) · [Every timed operation](perf/ivy_apple/incremental_cost/mutation-instructions.csv.gz)

### Candidate-checker equivalence and memoization cost

The reference-count accessor was a prominent physical leaf in the hardest Apple checks. I tested fewer reference-count reads at leaves, marking repeated edges in the prepared body, and retrieving the memo entry at the same time as checking its eligibility. On `rfn` check 29, the observer compared 611,204 ordered records byte for byte, including both sides of 157,133 candidate evaluations, 79,885 temporary equalities, 217,022 disequality requests and 31 congruence bucket creations. The trace records the participating AST IDs and candidate number. It can be reused on a new candidate optimization with `effects.py --variant before BEFORE_OVERLAY --variant after AFTER_OVERLAY` and a complete reference proof transcript. [Observer](perf/ivy_apple/candidate_checker/effects.py) · [Go instrumentation](perf/ivy_apple/candidate_checker/effects.go.in) · [Equivalence results](perf/ivy_apple/candidate_checker/memo-effects.csv)

After the rewriter-cache improvement, the alternative memo lookups had small and inconsistent effects. On the complete `rfn2` run, median instruction counts across configurations ranged from about 144.1 to 144.9 billion around a 144.4-billion baseline. On ordinary Raft all were within 0.2% of baseline. The deployed checker keeps its existing representation. The repeatable side-effect comparison is valuable here because a change in temporary AST creation or declaration registration can change later search even when the answer remains UNSAT. [Whole-proof measurements with complete rounds](perf/ivy_apple/candidate_checker/memo-lookup-proofs.csv)

### Expression substitution on the rewriter path

The rewriter asks for the definition, proof and dependency of successive terms in the same substitution. A live manager already remembers that table. The lookup now verifies the table owner and generation, then validates the expression slot and reads its record once. A miss leaves all three caller outputs untouched. Pending work, checked builds and a change of table take the ordinary lookup path. [Lookup](ast/ast_expr_substitution_ops.go) · [Ownership and lifetime tests](ast/ast_expr_substitution_test.go)

| Substitution probe | Before, instructions/op | After, instructions/op |
|---|---:|---:|
| Definition found | 373 | 284 |
| Expression never used as a key | 238 | 147 |
| Key present in a different substitution | 340 | 252 |

The table gives medians from 12 alternating rounds on CPU 33. Each measurement subtracts a one-million-operation process from a three-million-operation process and divides by the two million extra operations. Every case measured zero allocations. On ordinary Raft, the change reduced whole-process instructions in all 12 pairs, by a median 0.68%. Elapsed times on that shared CPU varied too much to infer a latency gain. The nine Apple proof transcripts and 18 per-check counters, 28 Raft mutation decisions and 188 private Raft search records matched the control. [Lookup fixture](ast/ast_expr_substitution_benchmark_test.go) · [Counter driver](perf/ivy_apple/benchmark_slope.py) · [Raw lookup counts](perf/ivy_raft/ast_lookups/substitution-cached-lookup.csv)

### Why the Apple frontend changes the whole-checker comparison

On the rebased GoIvy frontend, uninstrumented builds running at the default Go GC setting took 152.83 s with g3 and 182.37 s with linked Z3 on `dramc_nb2` (three pairs). On `rfn2` they took 49.57 s and 35.03 s (four pairs). Each pair produced identical stdout through the final `OK`. The frontend Go sources were pinned at `a6d3417b1`; GoIvy `4a97afff6` changes the g3 module version and has the same frontend Go code. The g3 solver in these controls used the compact rewriter cache from `ed3be7f3c`. [Uninstrumented controls](perf/ivy_apple/frontend_memory/release-controls.csv)

| Isolate | GOGC | Whole instructions, billions, g3 / Z3 | Go GC cycles, g3 / Z3 | Peak process RSS, MiB, g3 / Z3 |
|---|---:|---:|---:|---:|
| `dramc_nb2` | 100 | 525.6 / 624.5 | 2,447 / 6,131 | 270 / 373 |
| `dramc_nb2` | 200 | 425.5 / 448.9 | 971 / 2,650 | 407 / 281 |
| `dramc_nb2` | 400 | 378.3 / 334.8 | 410 / 752 | 784 / 552 |
| `rfn2` | 100 | 146.1 / 75.2 | 249 / 367 | 280 / 143 |
| `rfn2` | 400 | 133.2 / 63.0 | 46 / 62 | 531 / 264 |

Each cell is a median of three `dramc_nb2` or four `rfn2` observations. Counters cover the entire process. Runtime metrics cover the Go heap in each executable, excluding the C++ heap. On `dramc_nb2`, g3 and Z3 allocated 114.92 and 112.99 GB on the Go heap. At GOGC=100 the Go runtime attributed 30.94 and 68.37 CPU seconds to GC. At GOGC=400 those times fell to 6.07 and 9.32 seconds and the whole-process instruction ordering reversed. A bound on per-worker memory matters for the intended parallel workload. These runs support reducing the frontend allocations that drive collections. [GC and allocation observations](perf/ivy_apple/frontend_memory/gc-pacing.csv) · [Reproduction driver](perf/ivy_apple/frontend_memory.py)

Two independent instrumented pairs for each isolate had identical call counts across backends. On `dramc_nb2` there were 323,761,938 `symbolsIluAstRec` visits, 61,366,444 `renameASTRec` visits, and 2,323,722 `sliceSexp` calls. On `rfn2` the corresponding counts were 32,879,610, 3,486,885 and 32,630. The much smaller frontend workload on `rfn2` makes the cost of its fourth solver check easier to see. [Six shared frontend counters](perf/ivy_apple/frontend_memory/call-counts.csv)

### Four application operands in a 128-byte arena slot

A node slot already had room for a fourth 8-byte operand reference. A four-argument application can now keep its full key in that slot. Repeating it avoids allocating and releasing a separate argument payload. The page size, reservation and generation sequence remain the same. Checked construction, tracing and armed allocation failures keep their original payload paths where required. Exact-arity construction also carries the validated declaration through coercion and registration. The constructor still shapes variadic, associative and chainable applications according to their flags. [Slot layout](ast/ast_core_storage.go) · [Constructor](ast/ast_core_factories.go) · [Fourth-child identity and allocation tests](ast/ast_inline_four_test.go)

| Repeated application | Before, instructions | After, instructions | Native Z3, instructions |
|---|---:|---:|---:|
| One operand | 1,071 | 830 | 406 |
| Two | 1,177 | 935 | 456 |
| Three | 1,241 | 996 | 500 |
| Four | 2,970 | 1,265 | 605 |

These are medians from nine matched rounds of 200,000 operations on CPU 5, after subtracting the empty interval on the same thread. All counters had full coverage. Ten separate timing rounds on CPU 33 put repeated four-operand construction at 297.7 ns before, 120.0 ns after and 52.2 ns in native Z3. The four-operand kernel took 57.4% fewer instructions. In the same timing comparison, changing the layout alone yielded 174.7 ns and carrying the declaration alone yielded 269.0 ns. [Timed Go and C++ fixtures](perf/ivy_apple/application_reuse/compare.py) · [Thread-counter driver](perf/ivy_apple/application_reuse/instructions.py) · [Every interval](perf/ivy_apple/application_reuse/fourth-operand-instructions.csv.gz) · [Timing alternatives](perf/ivy_apple/application_reuse/declaration-dispatch-timings.csv.gz)

All 32 full-proof pairs retired fewer instructions. The median within-pair reductions were 0.98% on ordinary Raft (12 pairs), 0.90% on Apple `rfn2` (7), 1.09% on `rfn` (7) and 1.43% on `cf_live` (6). Elapsed time on `rfn2` rose 0.95% across those pairs. Both builds used the substitution optimization and the same pre-streaming GoIvy frontend. Nine Apple transcripts and their 18 search counters, 28 Raft mutations and 188 Raft search records agreed. [Complete arena proof runs](perf/ivy_apple/application_reuse/fourth-operand-proofs.csv)

The independent substitution comparisons finished with 0.49% fewer instructions on `rfn` and 0.63% fewer on `rfn2` in all seven pairs for each. On `cf_live` the median reduction was 0.39% across six pairs. [All substitution proof pairs](perf/ivy_raft/ast_lookups/substitution-cached-proofs.csv)

### Streaming canonical action keys in GoIvy

GoIvy formats compound actions when computing their canonical keys. Previously, a sequence formatted each child into a separate string, joined those strings, and copied the result into its own string. Ancestors repeated the copies. GoIvy `7c0004f8e` writes six compound action types into the same builder while preserving their tags, field order and IDs. The key for a depth-16 fixture fell from 328,781 to 12,569 allocated bytes, from 387 to 27 allocations, and from 180.5 to 21.2 µs. An independent instruction-slope measurement fell from 975,076 to 119,077 instructions per key. The GoIvy commit includes `BenchmarkNestedActionKey` and a separate formatter that checks the result. [Timing and allocations](perf/ivy_apple/action_keys/lookup.csv) · [Instruction measurements](perf/ivy_apple/action_keys/instructions.csv)

On `dramc_nb2`, four complete g3 pairs had 14.16% fewer instructions and 9.98% less elapsed time by median within-pair change. The release binaries ran 2,479 checks each. Native Z3 also benefits because it uses the same GoIvy frontend. On `rfn2`, the whole-process instruction reductions were 0.86% for g3 and 3.03% for native across four pairs each. These comparisons held the solver fixed at the fingerprint-retirement revision, used the rebased GoIvy code as the control, and verified the entire proof output. Run the checker comparisons with explicit `--labels before after` when the binary filenames are identical. [Whole-proof observations](perf/ivy_apple/action_keys/proofs.csv) · [Release comparison](perf/ivy_apple/compare.py)

Four traced `dramc_nb2` builds produced byte-identical output across 21,638,409,694 bytes and 8,622,828 lines, including 8,599,630 XTrace events. The comparison consumed the streams as they were generated. The four builds were the control, the shipped renderer, the renderer with an experimental symmetric-equality shortcut, and that experimental pair plus the preserved fixed-arity traversal. Only the renderer was integrated into `abcopt` in this measurement. [Trace counts](perf/ivy_apple/action_keys/trace.csv) · [Streaming trace comparison](perf/ivy_apple/compare_frontend_traces.py)

With the new frontend and g3 `fff9dd429`, the ordinary Raft checker passed all 419 obligations on ten more paired runs. The medians on CPUs 16 and 45 were **7.58 s for g3 and 74.26 s for native Z3**. Whole-process retired instructions were 28.50 and 215.20 billion and peak RSS was 71,386 and 321,198 KiB. The median within-pair speedup was 9.96×. [Raw Raft runs](perf/ivy_raft/streaming-release-checks.csv) · [Two-backend driver with hardware counters](perf/ivy_raft/compare_checkers.py)

### Boolean sort checks, quantifier lemmas and sorted auxiliary clauses

A Boolean sort test was reading the expression’s sort and then asking the manager for its Boolean sort on each visit. Applications already cache their result sort in the arena slot. The rewriter and internalizer now compare those references directly after validating the manager, slot and generation. Quantifier instantiation also knows the width of an existing disjunction before it constructs the new lemma. It allocates the final operand list once. [Boolean predicate](ast/ast_core_accessors.go) · [Invalid and stale handle tests](ast/ast_bool_sort_predicate_test.go) · [Lemma construction](smtcore/smt_qi_instance.go)

Z3 sorts auxiliary clauses by literal index. A variable's two polarities occupy adjacent indices, so the simplifier can detect duplicates and complements by looking at the last retained literal. g3 now does the same for the two clause kinds that sort their inputs. Other clause kinds retain their original input order and membership checks. The benchmark adds the complement of the largest variable to a descending list and checks that simplification leaves the solver and caller's input intact. [Native simplifier](../src/smt/smt_internalizer.cpp) · [Go implementation](smtcore/smt_internalizer.go) · [Clause benchmark and tests](smtcore/smt_clause_simplify_benchmark_test.go) · [Lemma benchmark](smtcore/smt_qi_lemma_benchmark_test.go)

| Operation | Median instructions, before → after | Median time, before → after | Allocated bytes, before → after |
|---|---:|---:|---:|
| Simplify 32 variables and a final complement, auxiliary clause | 21,588 → 8,408 | 2,682 → 1,036 ns | 864 → 248 |
| Same inputs, auxiliary lemma | 21,877 → 8,875 | 2,592 → 919 ns | 864 → 248 |
| Finish a quantified lemma with 16 disjuncts | 32,631 → 30,140 | 4,840 → 3,736 ns | 1,040 → 368 |

Instruction counts use eight alternating rounds and the difference between 20,000 and 70,000 operations. The independent timing runs used six rounds for clauses and eight for lemmas. The clause fixtures dropped from eight to five allocations, and the wide lemma from five to two. [Counter measurements](perf/ivy_apple/formula_construction/instructions.csv) · [Clause alternatives](perf/ivy_apple/formula_construction/auxiliary-clauses.csv) · [Lemma alternatives](perf/ivy_apple/formula_construction/quantifier-lemmas.csv)

Both release checkers used GoIvy `7c0004f8e` and the four-operand arena layout. On `rfn2`, instructions fell in seven of seven whole-proof pairs by a median 1.18%, and elapsed time fell by 5.88%. On `rfn` the corresponding reductions were 1.26% and 5.23% across seven pairs. On `cf_live` all six pairs used fewer instructions, a median 0.82% reduction. On ordinary Raft, instructions fell by a median 0.43% across twelve pairs, with eleven reductions and a 3.35% increase in median paired elapsed time on that shared CPU. All nine Apple proof transcripts and 18 per-check counters agreed, as did 28 Raft mutant decisions and 188 private Raft search records. [Release runs](perf/ivy_apple/formula_construction/proofs.csv)

### Storage for two watch lists per Boolean variable

The Boolean variable record remains **16 bytes**, including its 8-byte justification. Its two watch lists live in a separate directory. On amd64, each directory entry previously occupied 56 bytes even when neither kind of watch was present. The new 40-byte entry holds the usual clause slice, a pointer to the literal backing’s immutable slice header, and a count of live literals. Copies keep independent lengths and the propagation loop can still modify its borrowed view. A new literal backing needs one extra slice header. Z3 stores a single pointer in its directory and packs both types into the pointed-to allocation. [Go storage](smtcore/watch_list.go) · [Copy and mutable-view tests](smtcore/watch_list_test.go) · [C++ storage](../src/smt/watch_list.h)

| Extra live heap per worker after scope pop | Original, 56 B | Suffix length, 48 B | Literal header out of line, 40 B | Both headers out of line, 32 B |
|---|---:|---:|---:|---:|
| 48 workers × 2,048 atoms | 827,883 B | 795,132 B | **803,339 B** | 811,547 B |
| 1,024 workers × 128 atoms | 59,381 B | 59,397 B | **55,317 B** | 55,845 B |

Six fresh processes per layout measured live heap after collection. The selected layout saved 24,544 bytes per large worker and 4,064 per small worker, about 4 MiB across 1,024 warmed solvers. A separate sweep of 2,048 headers per solver saved 32,768 bytes when all were empty and 8,192 bytes when every list had a watch. This explains why the smallest directory entry did not yield the smallest worker heap. On the real Apple runs, the largest sampled directories reserved 76,361 entries on `rfn2` and `dramc_nb2`, and 95,670 on `cf_live`. Eight Raft pairs measured 28.55 billion instructions before and 28.59 billion after. The measured benefit is lower worker memory. [Worker benchmark](smtcore/smt_scope_workers_benchmark_test.go) · [Worker measurements](perf/scope_restoration/watch_storage/workers.csv.gz) · [Occupancy benchmark](smtcore/smt_watch_storage_benchmark_test.go) · [Density sweep](perf/scope_restoration/watch_storage/directory.csv.gz) · [Apple census and driver](perf/scope_restoration/watch_storage/census.py) · [Apple samples](perf/scope_restoration/watch_storage/occupancy.csv.gz) · [Pop timings](perf/scope_restoration/watch_storage/timing-alternatives.csv.gz) · [Raft process counts](perf/scope_restoration/watch_storage/raft.csv)

### GoIvy symbol traversal after the rebase

GoIvy `ef53df6a4` uses the existing concrete child accessors for common fixed-arity expressions. This lets Go keep temporary argument lists on the stack. Repeated subexpressions still yield their symbols at every occurrence. All four traversal variants produced byte-identical `dramc_nb2` traces through 8,622,828 lines, including 8,599,630 trace events and all 2,479 passing checks. The separate Multi-Paxos `log3` reproduction also matched Python on all 27,855 normalized lines, its 27,782 trace events and the expected diagnostic at line 253. The GoIvy commit includes the traversal benchmark and tests for suppression of later callbacks after an early stop and for a caller mutating a parent between visits. [Apple trace results](perf/ivy_apple/symbol_traversal/rebased-trace.csv)

On the fixed-arity fixture, traversal fell from 51.06 to 7.36 µs and from 190 heap allocations to zero across twelve trials. Visiting an application that referenced the tree twice fell from 105.41 to 15.64 µs and from 380 allocations to zero. On a complete `dramc_nb2` proof with the same g3 backend, instructions fell from **449.54 to 417.34 billion**, a 7.16% median paired reduction on CPU 19 across three rounds. With native Z3 the reduction was 7.17% across three pairs on CPU 29. Elapsed changes on those CPUs were −7.96% and +9.63%, respectively. On `rfn2`, five g3 pairs retired 2.27% fewer instructions. [Traversal trials](perf/ivy_apple/symbol_traversal/rebased-micro.csv) · [Complete proof pairs](perf/ivy_apple/symbol_traversal/rebased-proofs.csv)

Go runtime counters on `dramc_nb2` give an independent explanation. After the action-rendering change, the g3 checker allocated 309.70 million objects over the run. The traversal change removed **67.78 million** of them, bringing cumulative allocation from 60.22 to 58.44 GB. With the native backend, the counts were 299.29 and 231.51 million objects. Each build ran three times with default GC pacing. An earlier control on the pre-streaming frontend also varied a retained pointer-free buffer from 0 to 64 MiB. The native-backed checker's GC cycles fell from 5,955 to 1,207 and its instruction count from 615.95 to 362.98 billion. That was an experimental change to GC pacing. It demonstrates how easily a whole-checker comparison can mask the cost of the solver itself. [Allocation measurements](perf/ivy_apple/symbol_traversal/rebased-runtime.csv) · [GC pacing control](perf/ivy_apple/frontend_memory/ballast.csv) · [Measurement driver](perf/ivy_apple/frontend_memory.py)

### Where the remaining arena constructor time goes

The four-operand slot change made placement and disposal inexpensive. The following thread-counter medians separate a request for the same live four-argument application into stages. Each stage subtracts an empty observer interval from its own measurement. The boundaries include different pieces of the enclosing function, so summing the rows need not reproduce a separately timed complete request. [Counter driver and boundaries](perf/ivy_apple/application_reuse/phases.py) · [Five rounds, all arities and stages](perf/ivy_apple/application_reuse/fourth-operand-phases.csv.gz)

| Retired instructions per stage | g3, before fourth inline operand | g3, fourth operand inline | Native Z3 |
|---|---:|---:|---:|
| Dispatch to the application constructor | 298 | 212 | 45 |
| Place candidate | 82 | 84 | 37 |
| Prepare operands and hash | 1,605 | 596 | 353 |
| Canonical lookup | 505 | 276 | 166 |
| Discard duplicate | 352 | 30 | 34 |

Preparation now accounts for about 243 excess instructions at this arity, dispatch for 167 and lookup for 110. An exact, specialized four-operand hash cut a complete repeated request from 1,265 to 1,150 instructions, with native at 605. Its median whole-proof changes were −0.16% on Raft, −0.14% on `rfn2`, +0.03% on `rfn` and +0.12% on `cf_live` in the available matched rounds. Reusing a pointer to the newly placed slot also added work on this fixture. The production constructor retains its established hash and placement code. [All prototype counts](perf/ivy_apple/application_reuse/slot-and-hash-probes.csv.gz) · [Whole-proof trials](perf/ivy_apple/application_reuse/slot-and-hash-proofs.csv.gz)

Two other microbenchmarks show why changes here need proof-level validation. A narrow operand accessor reduced median complete `rfn2` instructions by 0.27%, with a 0.06% increase on `cf_live`. A specialized congruence probe reduced the median ordered lookup from 32.62 to 18.98 ns over ten rounds, but increased erase-and-insert from 265.3 to 295.5 ns. It changed `rfn2` instructions by −0.01% and ordinary Raft by +0.02%. Both fixtures are retained for future work. [Operand benchmark](ast/ast_operand_read_benchmark_test.go) · [Operand timings](perf/ivy_apple/application_reuse/operand-read-alternatives.csv.gz) · [Operand proof runs](perf/ivy_apple/application_reuse/operand-read-proofs.csv.gz) · [Congruence benchmark](smtcore/smt_cg_lookup_benchmark_test.go) · [Probe timings](perf/ivy_apple/candidate_checker/congruence-probes.csv.gz) · [Congruence proof runs](perf/ivy_apple/candidate_checker/congruence-proofs.csv)

### Inserting into a growing rewriter cache

The rewriter's activity cache grows its chained hash table before looking up a key. Z3 examines the primary bucket once during insertion. g3 previously located that bucket twice on a miss. g3 now carries the bucket through insertion and uses the Go zero value for a free cell. A collision still moves the previous head into the cellar. This preserves the order in which the cache releases arena references, which can affect recycled expression IDs and subsequent search. [Go hash table](ast/ast_activity_chashtable.go) · [Native hash table](../src/util/chashtable.h) · [Ownership and growth tests](ast/ast_activity_cache_test.go)

Eight alternating instruction-slope trials compared 6,000 and 21,000 operations per process. Filling the cache with 32 entries and resetting it fell from **21,821 to 19,930 instructions** (8.7%). A full one-entry lifetime fell from 2,844 to 2,752. A lookup hit took 207 instructions in both builds. The measurements include separate controls for the free-cell encoding and the insertion change. [Benchmark](ast/ast_activity_cache_benchmark_test.go) · [Instruction counts](perf/ivy_apple/rewriter_cache/growth-instructions.csv)

The median paired reduction in whole-process instructions was **0.26% on `rfn2`** (all seven pairs improved), **0.36% on `cf_live`** (all six), **0.16% on `rfn`** (five of seven), and **0.16% on Raft** (nine of twelve). Wall times varied in both directions. All 10,342 checks across nine Apple isolates matched native output and 18 search counters per check. The 28 Raft mutant runs agreed on all 11,732 outcomes, including the 77 expected failures. The 188 Raft private-search records also matched the control. [Whole-proof observations](perf/ivy_apple/rewriter_cache/growth-proofs.csv)

### November 21 release comparison

Both standalone checkers used rebased GoIvy `ef53df6a4`. The Go backend used g3 `8fd73e8b3` and the native backend used GoIvy's linked Z3 through `classic_z3`. Both disabled XTrace and PGO. These are medians of complete processes, with the two engines alternating on the same pinned logical CPU for each workload. All 419 named Raft outcomes matched the reference, and all repetitions of each Apple isolate produced the same complete transcript and final `OK`. [Raft driver and controls](perf/ivy_raft/compare_checkers.py) · [Apple driver](perf/ivy_apple/compare.py)

| Workload | Checks | Pairs / CPU | g3 elapsed | Z3 elapsed | g3 / Z3 instructions, billions | g3 / Z3 peak RSS, MiB |
|---|---:|---:|---:|---:|---:|---:|
| Ordinary Raft | 419 | 5 / 44 | 6.11 s | 65.45 s | 28.40 / 215.20 | 68 / 313 |
| Apple `dramc_nb2` | 2,479 | 3 / 37 | 84.88 s | 104.66 s | 415.98 / 475.58 | 261 / 206 |
| Apple `rfn2` | 26 | 4 / 24 | 34.25 s | 20.19 s | 137.61 / 69.33 | 261 / 148 |

Raft uses g3's specialized proof path. The two Apple isolates exercise the general solver. Across individual `dramc_nb2` pairs the native/g3 wall-time ratios were 1.12×, 1.44× and 1.02×. Host load varied during these runs. Instruction counts and phase boundaries give a more repeatable comparison of the engines. [Raft observations](perf/ivy_raft/nov21-release.csv) · [Apple observations](perf/ivy_apple/incremental_cost/nov21-release.csv)

A second pair of binaries enabled a thread-local instruction counter only during each verification call. All intervals ran for their entire enabled time. On `rfn2`, the total was **93.08 billion for g3 and 35.48 billion for Z3** in three pairs. Check 4 alone accounted for **48.70 versus 20.86 billion**. On `dramc_nb2`, the totals were **47.71 versus 15.70 billion** in two pairs. Check 71 alone used **19.79 versus 6.99 billion**. Ranking the per-check excess puts 48.3% of the total gap on `rfn2` check 4, with checks 13 and 22 contributing a further 18.8% and 14.8%. On `dramc_nb2`, check 71 accounts for 40.0% and the next largest contributes 5.5%. The preceding solver revision and its four paired measurements on the sibling isolate `rfn` are included in the data. [Counter and verification driver](perf/ivy_apple/incremental_cost/thread_checks.py) · [Per-check observations by solver revision](perf/ivy_apple/incremental_cost/nov21-check-instructions.csv.gz)

Two sampled runs per engine of just these hard checks identified the following costs. Each instruction sample belongs to the nearest named operation on its stack. Values are median sample counts times the period of 20,000,003 user instructions. Frame-pointer unwinding in Go and DWARF unwinding in native Z3 differ, so these are estimates. For `rfn2` check 4 the general solver rewriter accounted for **8.88 versus 2.03 billion instructions**, and the quantifier queue for **6.33 versus 1.44 billion**. On `dramc_nb2` check 71 those costs were **4.73 versus 1.19** and **4.24 versus 1.01 billion**. Together the two groups explain about 42% and 53% of the measured hard-check gaps, respectively. [Stack driver](perf/ivy_apple/phase_stacks.py) · [Classification](perf/ivy_apple/stack_costs.py) · [Operations and leaf symbols](perf/ivy_apple/incremental_cost/nov21-hard-check-stacks.csv.gz)

### The first push on a fresh Apple solver

The external `Push` commits and simplifies pending assertions, internalizes formulas, and propagates them before the next check begins. The new counter measures each adapter operation and excludes calls made inside a verification condition. A separate observer walks each base assertion on its first push, counts distinct expression handles and records which handles appeared in earlier bases. It follows application arguments and quantifier bodies. Triggers, declaration metadata and binder sorts are outside this count. The observer stores handle values without acquiring arena references. Each of these isolates used one AST context and multiple fresh solvers. [Counter](perf/ivy_apple/incremental_cost/mutation_costs.py) · [Base observer](perf/ivy_apple/incremental_cost/base_overlap.py) · [Join and rank by push](perf/ivy_apple/incremental_cost/rank_pushes.py)

| Isolate | All pushes | First pushes | Extra g3 instructions across all pushes | Fraction of excess on first pushes | Base node occurrences previously seen in the context |
|---|---:|---:|---:|---:|---:|
| `rfn2` | 26 | 10 | 8.90 B | **99.98%** | 61.72% |
| `dramc_nb2` | 2,479 | 86 | 25.64 B | **99.72%** | 86.40% |

The instruction differences sum the median paired excess at each ordinal. Those first pushes all had one base assertion. Every base root was distinct, including the 12 roots on `cf_live`, where 64.36% of expression occurrences had appeared in an earlier base. Each fresh solver saw its base for the first time. An exact whole-base memo would have had **zero hits** on all three isolates, and sharing mutable solver state would conflict with independent workers. Immutable per-expression properties and repeated traversals within an individual solver are narrower opportunities. On `rfn2`, pushes 4 and 22 alone contributed 2.77 and 1.57 billion excess instructions. On `dramc_nb2`, pushes 57, 3 and 291 contributed 3.04, 2.71 and 2.22 billion. [All counter intervals](perf/ivy_apple/incremental_cost/nov21-mutation-instructions.csv.gz) · [Every base and its overlap](perf/ivy_apple/incremental_cost/nov21-base-overlap.csv)

Three independent stack samples per backend and isolate on the preceding watch-storage revision `f040eb871` gated recording to the external pushes. Each sample represents about 10 million instructions. The rows below are inclusive named operations. Substitution occurs within propagation and macro finding occurs within assertion reduction. The rows therefore overlap. [Push sampling driver](perf/ivy_apple/incremental_cost/push_profile.py) · [All sampled symbols](perf/ivy_apple/incremental_cost/nov21-push-stacks.csv.gz)

| Sampled operation during external pushes | `rfn2` g3 / Z3, billions | `dramc_nb2` g3 / Z3, billions |
|---|---:|---:|
| Entire push window | 13.15 / 3.51 | 37.39 / 9.59 |
| Core propagation | 6.53 / 2.01 | 12.77 / 3.99 |
| Cached variable substitution | 3.88 / 1.29 | 7.38 / 2.53 |
| Reduce pending assertions | 5.04 / 1.31 | 16.24 / 4.14 |
| Macro finding within reduction | 1.97 / 0.55 | 4.50 / 1.26 |
| Value propagation within reduction | 0.65 / 0.10 | 6.55 / 1.12 |

The assertion-pass breakdown assigns each stack to its outermost named pass. On `dramc_nb2`, value propagation and macro finding together account for about **8.67 billion of the 12.10 billion excess instructions** in assertion reduction. In the Go value-propagation stack the expensive call is rewriting the pending assertion. Inside the macro finder most work descends into macro expansion. Both paths reach the general rewriter. [Pass classifier](perf/ivy_apple/incremental_cost/push_pass_costs.py) · [All three rounds](perf/ivy_apple/incremental_cost/nov21-push-passes.csv) · [Go value propagation](smtcore/smt_asserted_propagate.go) · [Native reduction and propagation](../src/smt/asserted_formulas.cpp)

### November 21 scope restoration

Five alternating runs of the original, unpinned `make bench` and `make cppbench` targets on the development host gave medians of **774 versus 484 ns** for surviving atoms and **23.86 versus 12.53 µs** for scoped atoms, at 10,000 iterations. Affinity, CPU load and Go benchmark timing overhead matter at these durations. A separate comparison used the dedicated pop interval on CPUs 11 and 33, twelve rounds per CPU. With the branching queue active in both solvers, the medians were **313 versus 500 ns** for surviving atoms and **20.62 versus 12.84 µs** for scoped atoms. The Go queue-free scoped control measured **14.49 µs**. [Make targets](perf/scope_restoration/watch_storage/make-targets.csv) · [Pinned pop timings](perf/scope_restoration/watch_storage/final-scope.csv) · [Go benchmark](smtcore/smt_hotpath_benchmark_test.go) · [Native benchmark](../src/test/smt_scope_restoration.cpp)

Hardware counters put a complete queue-enabled scoped pop at **213,672 instructions in g3 and 144,718 in Z3**. Undoing internalized variables consumed **58,617 and 19,654**, respectively, about 57% of the full gap. Giving quantifiers and gates distinct rollback kinds, singly or together, or guarding deletion from an absent activity queue, moved the complete pop by under 1% in eight rounds. The combined gate/quantifier variant increased whole-proof instructions on `rfn2` by 0.19% in the three paired trials. The next scope experiment should break down actual undo work per variable, including owned reference releases and activity-queue deletion. [Phase counter](perf/scope_restoration/pop_phases/variables.py) · [Rollback measurements](perf/scope_restoration/watch_storage/rollback-variants.csv.gz) · [Whole proofs](perf/scope_restoration/watch_storage/rollback-proofs.csv.gz) · [Combined prototype](perf/scope_restoration/watch_storage/rollback-kinds.patch.gz)

A separate beta-reducer prototype avoided a repeated cache lookup on a newly pushed frame and a configuration callback with a constant result. The retained-instance substitution benchmark fell from 13,761 to 13,545 instructions and the retired-instance benchmark from 20,922 to 20,724. With the production activity cache in both builds, the complete `rfn2` instruction saving was under 0.2% and the Raft saving under 0.1% in the sampled pairs. A second baseline even changed the sign on Raft. That tradeoff favors keeping the simpler production resume loop and measuring how often its callers request the same rewrite. [Substitution benchmark](smtcore/smt_quantifier_substitution_benchmark_test.go) · [Instruction counts](perf/ivy_raft/ast_lookups/beta-resume-instructions.csv.gz) · [Whole proofs](perf/ivy_raft/ast_lookups/beta-resume-proofs.csv.gz) · [Prototype](perf/ivy_raft/ast_lookups/beta-resume.patch.gz)

### Measuring repeated macro expansion before adding a cache

The macro finder walks a formula with the currently installed definitions. The observation build ran every original expansion and recorded each declaration lookup. A pass counted as negative only when it returned the identical expression and found no installed macro. With an unchanged set of definitions, an observer remembered those input expressions by manager, arena slot and generation. It cleared the observations on definition insertion, shrink and parameter changes. Three isolates reproduced their complete native transcripts with the observer enabled. [Counter and observer](perf/ivy_apple/incremental_cost/macro_passes.py) · [All pass counts and instructions](perf/ivy_apple/incremental_cost/nov21-macro-passes.csv)

| Isolate | Instructions inside macro expansion steps | Negative passes | Previously seen negative roots, unlimited | Previously seen negative roots, 128 direct-mapped entries |
|---|---:|---:|---:|---:|
| `rfn2` | 875 M | 554 M | 31 M | 4 M |
| `dramc_nb2` | 1,951 M | 1,248 M | 89 M | 16 M |
| `cf_live` | 1,055 M | 692 M | 52 M | 19 M |

Each numeric column counts instructions. The negative passes account for 82–83% of the lookup calls. Even a cost-free, unbounded negative **whole-formula** cache would have removed at most 31 million instructions from the 137.6-billion-instruction `rfn2` run. These counters cover the expansion step itself. The enclosing macro finder also normalizes changed results, allocates and tears down rewrites, publishes batches and simplifies the updated assertions.

Two release prototypes reduced each macro lookup to a single validated arena access. One reused the existing node header and the other read only the optional application declaration. They preserved complete proof output. Across four alternating rounds on each of `rfn2` and `rfn`, and three on `cf_live`, all six median instruction changes were within ±0.08%. Neither accessor entered the production path. [Proof pairs](perf/ivy_apple/incremental_cost/nov21-macro-declaration.csv) · [Header prototype](perf/ivy_apple/incremental_cost/macro-declaration-header.patch.gz) · [Declaration-only prototype](perf/ivy_apple/incremental_cost/macro-declaration-decl.patch.gz)

A more productive prototype checks whether a subtree contains any declaration with an active macro. It remembers negative answers for 1,024 exact, generation-checked arena handles in one solver's macro manager. It visits application arguments and quantifier bodies, following the macro expander's pattern policy. The experimental table allocates 16 KiB when first used. Two rounds on different pinned CPUs timed the screen and then ran and timed **every original expansion**. In each round every negative prediction resulted in an identical expression and zero macro hits. [Subtree screen](perf/ivy_apple/incremental_cost/macro_screen.go.txt) · [Both rounds](perf/ivy_apple/incremental_cost/nov21-macro-screen.csv)

| Isolate | Negative predictions / all passes | Screen cost over all passes | Original work in predicted negative passes | Potential net reduction |
|---|---:|---:|---:|---:|
| `rfn2` | 8,417 / 9,978 | 31 M instructions | 547 M | 516 M |
| `dramc_nb2` | 24,276 / 28,204 | 75 M | 1,238 M | 1,164 M |
| `cf_live` | 12,454 / 14,431 | 38 M | 677 M | 639 M |

The last column subtracts measured screening cost from measured original work and rounds the difference. It is an estimate of work that could be avoided. For `dramc_nb2` it represents about 0.28% of the complete checker. A release shortcut would have to preserve proof and dependency production, reference lifetimes, resource-limit accounting and cancellation. The prototype left the expansion enabled and ran with complete transcript checks. It establishes a promising implementation target of limited size.

### Priorities after the November 21 measurements

1. **Optimize general rewriting and quantifier instantiation on `rfn2` check 4 and `dramc_nb2` check 71.** These two operations account for about 11.7 and 6.8 billion excess instructions on those single checks. Instrument each candidate instance from substitution through lemma internalization in both engines. Compare visits per instance, Boolean-table probes, clause creation and owned-reference transfers. The largest named Go rewriter leaves include `processApp`, expression-substitution lookup and activity-cache lookup. Within the quantifier queue, Boolean-variable lookup, expression-table writes and clause internalization recur. These are concrete operations to benchmark while keeping the search counters fixed. [Classified instruction leaves](perf/ivy_apple/incremental_cost/nov21-hard-check-operation-leaves.csv.gz) · [Instance construction](smtcore/smt_qi_instance.go)
2. **Use the expensive first pushes as the preprocessing workload.** On `dramc_nb2` push 57 and `rfn2` push 4, measure the number of assertion-rewrite visits and distinct visited nodes in each pass. Value propagation and macro finding together explain most of the excess in `dramc_nb2` assertion reduction. The subtree prototype narrows a specific saving in the macro finder. A larger gain requires reducing visits or per-visit cost in the general rewriter used by both passes. Its cache can remain private to each solver. [Per-operation counts](perf/ivy_apple/incremental_cost/nov21-mutation-instructions.csv.gz) · [Assertion-pass breakdown](perf/ivy_apple/incremental_cost/nov21-push-passes.csv)
3. **For the scope benchmark, concentrate on releasing an internalized variable.** Variable undo explains about 39,000 excess instructions per queue-enabled pop, while changing the trail kind had little effect. Keep the 16-byte Boolean record and the current arena. Measure one removal with its activity-queue and reference costs, then verify any improvement in the queue-enabled scope benchmark and in whole proofs. [Rollback counts](perf/scope_restoration/watch_storage/rollback-variants.csv.gz) · [Independent-worker benchmark](smtcore/smt_scope_workers_benchmark_test.go)

### Matched quantifier candidates on the two most expensive Apple checks

Release binaries built from g3 `22d3a8b7b` and GoIvy `ef53df6a4` reproduced the whole-process results with the local module replacement, PGO off, XTrace off, and one Go CPU. Eight alternating pairs on CPU 20 gave median `rfn2` times of **20.06 s for g3 and 11.72 s for linked Z3** (1.71×). Four pairs on CPU 39 gave **60.98 s and 72.02 s** on `dramc_nb2`. The corresponding median instruction totals were 138.16 versus 69.23 billion and 415.14 versus 470.60 billion. All transcripts matched. [Release observations](perf/ivy_apple/quantifier_phases/nov22-release-baseline.csv)

In each of four additional paired runs per isolate, a thread-local Linux hardware counter measured the stages of every candidate in the selected verification call. Each pair matched the complete sequence of ordinals, quantifier IDs, generations, binding counts and outcomes. On `rfn2` check 4 both engines skipped 560,747 satisfied candidates, rewrote 1,513 instances to true and internalized 140,020 lemmas. On `dramc_nb2` check 71 the counts were 180,262, 2,043 and 101,424. The last two outcomes incur substitution and rewriting costs. [Measurement driver](perf/ivy_apple/quantifier_phases/measure.py) · [Go observer](perf/ivy_apple/quantifier_phases/candidate.go.txt) · [Native observer](../src/test/perf/qi_candidate_measurement.h)

| Median retired instructions, billions | rfn2 g3 | rfn2 Z3 | dramc_nb2 g3 | dramc_nb2 Z3 |
|---|---:|---:|---:|---:|
| Check whether bindings already satisfy the body | 8.60 | 5.46 | 3.58 | 2.13 |
| Substitute bindings | 2.56 | 0.88 | 1.93 | 0.67 |
| Rewrite the resulting instance | 7.17 | 1.73 | 4.24 | 1.04 |
| Construct the lemma | 1.07 | 0.46 | 0.74 | 0.32 |
| Internalize the lemma | 5.00 | 1.08 | 3.72 | 0.81 |

Intervals start and end at the queue's stage transitions. Counts include the observer's boundary code after subtracting one adjacent empty read per transition. The counter ran for the full enabled time in every observation. Setup and cleanup use different bookkeeping in each observer and are available separately in the CSV. The figures cover work inside candidate processing and exclude the rest of the verification call. On `rfn2`, the measured excess is **5.44 billion instructions during instance rewriting and 3.92 billion during lemma internalization**. The corresponding excess on `dramc_nb2` is 3.20 and 2.91 billion. The CSV retains totals by quantifier, generation, outcome and round. The driver keeps individual candidate observations in the selected output directory and checks them before aggregating. [All four matched rounds](perf/ivy_apple/quantifier_phases/nov22-candidate-instructions.csv.gz)

### Both engines visit the same nodes on the expensive checks

A separate operation-count build instrumented the Go solver and rebuilt the C++ fork, including every instantiation of its rewriter template. The following counts are identical between engines. Both builds ran the complete GoIvy isolates and matched the reference transcripts. [Census driver](perf/ivy_apple/quantifier_phases/operations.py) · [Native hooks](../src/test/perf/qi_operation_measurement.h) · [Operation counts](perf/ivy_apple/quantifier_phases/nov22-operation-counts.csv)

| Operation inside candidate processing | rfn2 check 4 | dramc_nb2 check 71 |
|---|---:|---:|
| Node visits during substitution | 1,385,055 | 1,058,818 |
| Node visits during instance rewriting | 2,907,596 | 1,688,398 |
| Application-frame resumes during rewriting | 2,102,143 | 1,167,641 |
| Activity-cache lookups during rewriting | 2,679,326 | 1,463,882 |
| Activity-cache insert calls during rewriting | 657,644 | 290,084 |
| Internalizer entries while adding lemmas | 2,767,051 | 1,897,556 |

On check 4, both engines also made 593,078 clause-construction calls inside lemma internalization. Dividing the measured rewrite-stage instructions by its node visits gives about **2,467 instructions per visit for g3 and 595 for Z3**, including all work below those visits. Dividing the internalization stage by its entries gives about **1,806 and 389 instructions per entry**. The fourfold difference in these two stages comes from the cost of performing the same high-level work. The CSV also includes implementation-specific expression lookup and result-stack counters. Native Boolean lookups span multiple accessors and g3's expression-table read covers additional kinds of state, so their counts have different meanings.

On the selected first pushes, each individual assertion-reduction pass also had exactly the same **total and distinct** node visits across engines. On `rfn2` push 4 the first macro-finding pass visited 97,500 nodes representing 4,097 distinct expressions. On `dramc_nb2` push 57 it visited 119,847 nodes representing 5,329 expressions. Value propagation made 32,257 visits to 2,441 distinct expressions and 42,476 visits to 3,740 respectively. The Go observer identified a live expression by manager, slot and generation. The native observer removed each address from its set on deletion so that reuse of the address counted as a new expression. These counts include repeated calls with changed substitutions or macro sets. They give an upper bound on opportunities to reuse results. [Per-pass observations](perf/ivy_apple/quantifier_phases/nov22-operation-passes.csv)

### Ground instances at the constants in the verification goal

A model of a positive universal premise must satisfy every well-sorted ground instance of that premise. On the three expensive `rfn2` obligations the goal supplies one unambiguous constant per uninterpreted sort. g3 now tries those terms as bindings in the existing optional verification proof. It takes universal premises only from positive top-level conjunctions, substitutes in declaration order with the port's de Bruijn substitution, and checks the resulting ground formulas, ground premises and goal in a private solver and arena. A private UNSAT discharges the obligation. A private SAT, unknown, exhausted budget or ineligible input leaves the original incremental solver available for the decision and counterexample. This path runs when the existing order abstraction is inapplicable. [Grounding plan](api/api_goal_instances.go) · [Private check](api/api_goal_instances_check.go) · [Scope, model and cancellation tests](api/api_goal_instances_test.go)

| Complete release run | Checks | Rounds | Prior g3 | g3 with ground instances | Linked Z3 | Instructions in billions, prior / new / Z3 |
|---|---:|---:|---:|---:|---:|---:|
| Apple `rfn2` | 26 | 9 | 39.03 s | **12.04 s** | 24.24 s | 137.53 / **55.12** / 69.28 |
| Apple `rfn` | 33 | 6 | 25.17 s | **8.69 s** | 16.16 s | 120.08 / **51.41** / 57.43 |

Each round ran all three release binaries on one pinned CPU using the same GoIvy frontend and specification. Run order rotated. Other experiments ran on the host. Absolute times therefore differ from the quieter baseline earlier in this report. The complete stdout matched linked Z3 on every run. The measured release used the initial threshold of 96 compatible universals and at least three times as many compatible universals as ground premises. On `rfn2`, this admitted seven attempts, six refutations and one concrete fallback. The refutations included checks 4, 13 and 22. The proof changes the search algorithm. The general rewriter's same-search instruction gap measured above still applies. [Complete release observations](perf/ivy_apple/goal_instances/nov22-rfn-release.csv)

A separate verification build ran the **original incremental solver even after every successful ground proof**. Across all 10,342 checks on nine Apple isolates, linked Z3 matched g3 on the entire proof transcript and all 18 per-check search counters, including resource steps. The original solver independently confirmed all 39 private refutations. The verification was repeated after fixing ownership of a translated ground premise that can simplify away during assertion. Both runs produced identical per-check CSVs. [Verification driver](perf/ivy_apple/goal_instances/verify.py) · [Per-check results](perf/ivy_apple/goal_instances/nov22-verified-search.csv.gz)

| Standalone exported obligation | Ordinary g3, ms | Ground proof, ms | Ordinary / ground, billion instructions per extra operation |
|---|---:|---:|---:|
| `rfn2` check 4 | 837.1 | 52.4 | 6.057 / 0.425 |
| `rfn2` check 13 | 258.0 | 46.1 | 2.028 / 0.365 |
| `rfn2` check 22 | 493.6 | 59.6 | 3.471 / 0.469 |

Each operation parses the same captured formula and opens and closes a fresh solver. Eight rounds on CPU 5 measured two and five iterations in alternating order. Instruction slopes subtract the process totals and divide by three. Wall-time medians use the five-iteration measurement. The standalone fixture omits GoIvy's preceding incremental checks. Run it from `pkg/` with `go test ./api -run '^$' -bench '^BenchmarkAppleGoalInstances$' -benchtime=5x`. The native capture driver takes a reference transcript and verifies the complete exporting run. [Go benchmark](api/api_goal_instances_benchmark_test.go) · [Captured fixtures and license](perf/ivy_apple/test_vectors) · [Capture driver](perf/ivy_apple/goal_instances/capture.py) · [All fixture rounds](perf/ivy_apple/goal_instances/nov22-fixture-instructions.csv)

### Cost of attempting a private proof on an inexpensive goal

On `sys_live`, the original ground-instance method proved ten obligations, but the complete checker retired **12.4 billion more instructions** than the build without that method. On `e2e` it proved seven and spent **7.6 billion more**. A cheap check in a warm incremental solver can cost less than translating premises and opening a private solver. The revised implementation requires at least three distinct non-nullary uninterpreted function declarations in the goal as well as the original witness and premise thresholds. Predicate declarations count. Repeated applications of the same declaration count once. These are admission criteria for spending private search time. [Eligibility implementation](api/api_goal_instances.go) · [Scope and eligibility tests](api/api_goal_instances_test.go)

| Complete isolate, billion retired instructions | Before ground instances | Initial ground method | Three-function gate |
|---|---:|---:|---:|
| `rfn2` | 137.88 | 55.37 | 55.28 |
| `rfn` | 120.10 | 51.31 | 51.37 |
| `dramc_nb2` | 415.62 | 417.14 | 416.43 |
| `cmpl_live` | 126.88 | 127.77 | 127.36 |
| `sys_live` | 354.40 | 366.84 | 354.74 |
| `e2e` | 447.24 | 454.79 | 447.44 |

Each cell is the median of four complete, transcript-checked runs on the same pinned CPU for that isolate. Six variants rotated in each round. The wider trial required five functions and 16 distinct goal nodes. The premise-threshold trial varied the minimum compatible rule count and its ratio to the ground premises. Every release run retained the native proof transcript. With the three-function gate, an independent replay confirmed all 18 accepted refutations on nine isolates. All 18 search counters matched linked Z3 on every one of the 10,342 original checks. [All 144 goal-eligibility runs](perf/ivy_apple/goal_instances/nov22-goal-eligibility.csv) · [All 120 premise-threshold runs](perf/ivy_apple/goal_instances/nov22-instance-thresholds.csv) · [Per-check independent verification](perf/ivy_apple/goal_instances/nov22-gated-search.csv.gz)

The four paired ordinary Raft runs for the initial ground method took 5.92 s for g3 and 61.23 s for linked Z3. The 28 Raft protocol mutations also matched the native and Python references, including the positions of failed checks, across 11,732 outcomes. On five `dramc_nb2` triples from the same release study, the medians were 65.44 s before ground instances, 72.79 s with the initial method and 80.77 s for linked Z3. Concurrent host load affected elapsed time. The corresponding instruction medians were 414.96, 416.72 and 473.92 billion. [Raft runs](perf/ivy_apple/goal_instances/nov22-raft-release.csv) · [Raft mutations](perf/ivy_apple/goal_instances/nov22-raft-mutations.csv) · [Apple triples](perf/ivy_apple/goal_instances/nov22-dramc-release.csv) · [Release comparator](perf/ivy_apple/compare.py)

The next expensive calls moved. In five rotating rounds with the initial ground proof, `rfn2` check 7 took **7.04 billion** instructions on the solver thread against **2.07 billion** in Z3. Check 26 took 2.72 against 0.98. On `dramc_nb2`, check 71 took 19.82 against 6.99. These figures include the private proof attempt and subsequent concrete check when needed. Every observation subtracts its adjacent empty interval. All counters ran for their full enabled time. Between checks, push operations still contributed 9.06 billion excess instructions on `rfn2` and 25.76 billion on `dramc_nb2`, summed over ordinals using their per-engine medians. [Per-check hardware counts](perf/ivy_apple/goal_instances/nov22-residual-checks.csv.gz) · [Per-mutation hardware counts](perf/ivy_apple/goal_instances/nov22-residual-mutations.csv.gz) · [Check measurement driver](perf/ivy_apple/incremental_cost/thread_checks.py) · [Mutation measurement driver](perf/ivy_apple/incremental_cost/mutation_costs.py)

### Reading the assertions of a simple SMT solver

The public assertion snapshot first gets a count and then requests each formula by index. On the simple SMT backend, both operations copied the entire kernel assertion vector, so a 1,024-formula snapshot copied the vector 1,025 times. Reading its count and indexed handle directly gives linear time and one allocation for the caller's result. The kernel's separate snapshot method still returns an owned copy. The ordinary GoIvy combined solver reads through its first backend, so these results measure the simple-solver API specifically. [Indexed access](smtcore/smt_solver.go) · [Snapshot loop](solver/solver.go)

| Assertions | Before, µs | After, µs | Allocated bytes before / after | Allocations before / after |
|---:|---:|---:|---:|---:|
| 2 | 0.239 | 0.101 | 224 / 32 | 4 / 1 |
| 32 | 11.48 | 0.337 | 34,304 / 512 | 34 / 1 |
| 256 | 448.1 | 3.51 | 2,109,440 / 4,096 | 258 / 1 |
| 1,024 | 6,049 | 16.88 | 33,603,584 / 16,384 | 1,026 / 1 |

The table gives medians of eight alternating pairs on CPU 23, with 12 snapshots per size per run and one Go scheduler thread. Construction, assertion and disposal were outside the timer. The small-case timings are sensitive to host load. Run `go test ./api -run '^$' -bench '^BenchmarkSimpleSolverAssertionSnapshot$' -benchmem` from `pkg/`. [Go benchmark](api/api_assertion_snapshot_benchmark_test.go) · [All snapshot measurements](perf/ivy_apple/goal_instances/nov22-assertion-snapshot.csv)

### Refuting quantified Boolean goals with ground implications

On Apple `dramc_nb2` check 71, one ground instance per supported quantifier was enough to refute the goal. A diagnostic run started the attempt after 50,000 steps of ordinary search. It took **8.34 ms** to build and check a relaxation with 4,776 variables and 6,082 clauses. The original incremental search then continued and independently returned UNSAT. The complete measured interval, including the auxiliary attempt, was 4.367 s. The previous thread-counter study put this check at 19.82 billion g3 instructions and 6.99 billion Z3 instructions. [Per-check shadow timings](perf/ivy_apple/goal_instances/nov22-projection-shadow.csv.gz) · [Previous hardware counts](perf/ivy_apple/goal_instances/nov22-residual-checks.csv.gz)

For a universal formula `Q` and a ground instance `B` of its body, every source model satisfies `Q ⇒ B`. For an existential the valid implication is `B ⇒ Q`. Giving each closed quantifier its own Boolean variable preserves these implications under negation, disjunction, equivalence and conditionals. The implementation picks an existing ground constant of the correct sort for each bound variable. It shares terms only when their declaration handles and full argument sequences match. It retains Boolean connectives and reflexivity and symmetry of equality. Dropping other theory constraints gives a relaxation. A contradiction in it proves the source UNSAT. A satisfying assignment, interrupted attempt or exhausted private allowance sends the caller back to its original solver. The input and temporary tables belong to one proof attempt. [Construction and binder admission](api/internal/order/quantified_goal.go) · [Ground term identity and substitution](api/internal/order/quantified_ground.go) · [Boolean encoding](api/internal/order/quantified_boolean.go)

| Complete isolate | Matched rounds | Previous, billion instructions | All quantified goals | Single or nested quantifier |
|---|---:|---:|---:|---:|
| `sys_live` | 4 | 354.80 | 300.88 | 300.49 |
| `e2e` | 3 | 446.52 | 388.03 | 387.34 |
| `dramc_nb2` | 3 | 417.78 | 411.68 | 410.39 |
| `cf_live` | 4 | 185.70 | 197.24 | 185.77 |
| `cf_pio_live` | 2 | 487.10 | 502.13 | 485.51 |
| `cmpl_live` | 4 | 127.28 | 128.24 | 128.27 |

Each cell is the median of the same completed rounds on a pinned CPU for that isolate. The six gate variants rotated. The previous mode includes the ground-goal method. Seeking a private proof on every quantified goal added about 3–6% on the two cache-coherence isolates. The retained rule tries a goal with one distinct quantified subformula or a nested quantifier. This preserved the large gains on `sys_live` and `e2e`. On `cmpl_live` successful private proofs still cost about 1 billion more instructions in aggregate. [Matched release measurements](perf/ivy_apple/goal_instances/nov22-projection-matched.csv)

Independent replay forced the original incremental solver to check every query after the optional proof. It confirmed **208 private UNSAT results**, including the 18 from the preceding ground method. Trying all quantified goals as a separate control gave 276 confirmed refutations. Across all nine isolates, the 18 original search counters matched linked Z3 at each of 10,342 checks in both runs. A second control generated **168 satisfiable queries** from the seven captured Apple formulas using twelve seeded interpretations and one- and two-element domains for each uninterpreted sort. It retained each premise satisfied by the interpretation and selected a satisfied polarity of the goal. G3 and Z3 both returned SAT on every explicit model certificate. The public private-proof API accepted zero refutations of those source queries. [Gated replay](perf/ivy_apple/goal_instances/nov22-projection-verified.csv.gz) · [Unrestricted replay](perf/ivy_apple/goal_instances/nov22-projection-ungated-verified.csv.gz) · [Finite-model driver](perf/ivy_apple/goal_instances/finite_models/verify.py) · [All satisfiable controls](perf/ivy_apple/goal_instances/finite_models/nov22-satisfiable-apple.csv)

Run `go test ./api -run '^$' -bench '^BenchmarkAppleQuantifiedGoal$' -benchmem` from `pkg/` for the captured `dramc_nb2` check. The fresh variants include solver construction and parsing. The prepared variant measures private refutation of a loaded formula. The Go tests exercise quantifier polarity, overlapping declaration names, two sorts, compound terms, resource exhaustion, cancellation, pop and concrete model recovery. [Benchmark](api/api_goal_instances_benchmark_test.go) · [Semantic and lifetime tests](api/api_quantified_goal_test.go)

### Can the private proof avoid the first push?

GoIvy has the source assertions and current verification condition before it opens the incremental scope. An experimental adapter passed those borrowed expressions to the existing private proof at that point. A refutation completed the check immediately. Otherwise the adapter opened the original scope and called the concrete solver. This measured how much base preprocessing could actually disappear. Each mode ran five times on the same CPU for an isolate. All complete proof transcripts matched. [Raw process counters and push counts](perf/ivy_apple/goal_instances/nov22-before-scope.csv)

| Complete workload | Checks | Pushes avoided | Current order, billion instructions | Proof before push, billion instructions |
|---|---:|---:|---:|---:|
| Ordinary Raft | 419 | 419 | 28.37 | 27.27 |
| Apple `rfn2` | 26 | 6 | 55.34 | 54.94 |
| Apple `rfn` | 33 | 4 | 51.39 | 51.41 |
| Apple `dramc_nb2` | 2,479 | 104 | 410.10 | 410.73 |
| Apple `sys_live` | 1,738 | 33 | 300.92 | 301.00 |

The change saved 3.9% of retired instructions on ordinary Raft, where every check used the private proof. On the Apple isolates a later concrete check still processes a fresh solver's pending base. Avoiding 104 scopes on `dramc_nb2` produced no whole-run saving. The production adapter keeps its current call order. The more promising target for Apple is the per-expression work in preprocessing a base that a concrete check ultimately needs.

The two patches reproduce the experiment on g3 `8c4be47e9` and GoIvy `ef53df6a4`. Build the GoIvy checker with `-tags=xtrace_off -pgo=off` and run with `GOMAXPROCS=1`. An empty `IVY_BEFORE_SCOPE` selects the original order. The value `once` attempts the proof before opening the scope and dispatches directly to the concrete check on fallback. The `twice` control allows the normal proof dispatch again after a miss. Set `IVY_PROOF_EVENTS=yes` to count scopes and successful refutations. [g3 experimental patch](perf/ivy_apple/goal_instances/before-scope-g3.patch) · [GoIvy experimental patch](perf/ivy_apple/goal_instances/before-scope-goivy.patch)

### Direction of Boolean unit propagation

The private Boolean prover emits a gate after its operands and asserts its root. A forward sweep can move a root assignment through only one level of a long implication chain. Starting at the root lets one sweep fix descendants, and reversing direction on the next sweep propagates their consequences back up. The production reducer now alternates these directions for the same bounded number of sweeps. Both directions poll the owning solver's resource limit after the same number of clause visits. [Implementation](api/internal/order/boolean_units.go) · [Single-sweep contradiction and satisfiable control](api/internal/order/boolean_units_test.go)

| Standalone Apple `dramc_nb2` check 71 | Forward only | Backward only | Backward, then alternating |
|---|---:|---:|---:|
| Clauses left from 5,646 after 16 sweeps | 5,079 | 1,583 | 1,583 |
| Instructions per reduction, millions | 7.418 | 5.731 | 3.659 |
| Instructions per prepared proof, millions | 78.01 | 41.97 | 39.75 |
| Allocations per prepared proof | 27,617 | 8,407 | 8,407 |
| Bytes per prepared proof, MiB | 6.04 | 3.14 | 3.15 |

The instruction estimates subtract the shorter process from the longer process within each round, then divide by the additional operations. The table gives medians of 12 rotating rounds with one pinned CPU per benchmark. The reduction ran 20 and 120 times. The complete prepared proof ran 2 and 12 times and includes projection, reduction and any remaining SAT search. The saved standalone SMT2 and live incremental check have different clause counts. Run `go test ./api -run '^$' -bench 'BenchmarkApple(BooleanUnitReduction|QuantifiedGoal)' -benchmem` from `pkg/`. [Go benchmarks](api/api_goal_instances_benchmark_test.go) · [All process and allocation counters](perf/ivy_apple/goal_instances/nov22-unit-benchmarks.csv)

Across 315 generated Boolean problems from six Apple isolates, changing the 16-sweep order increased immediate contradictions from **163 to 249**. On `cf_pio_live` they increased from 14 to 57 out of 81, and on `cmpl_live` from 1 to 17 out of 18. This census also generates the flat quantified goals excluded by the production admission rule. Each alternative operates on its own copy of the same input. The replay driver independently reproduced all 432 observations for `cmpl_live` directly from the checker. [Per-problem observations for four schedules and six bounds](perf/ivy_apple/goal_instances/nov22-unit-schedules.csv.gz) · [Replay driver](perf/ivy_apple/goal_instances/unit_schedules.py)

| Complete Raft workload | Checks | Forward, billion instructions | Backward-first alternating | Linked Z3 |
|---|---:|---:|---:|---:|
| One-bit fields | 419 | 12.921 | **11.686** | 13.178 |
| Mixed widths | 419 | 28.338 | 28.355 | 161.837 |
| Ordinary | 419 | 28.310 | 28.358 | 215.205 |

On one-bit Raft, unit-propagation resource charges across 185 Boolean problems fell from 12.56 million to 3.56 million. All four release modes, including backward-only and linked Z3, produced identical complete proof transcripts in six rotating rounds per workload. The instruction saving was concentrated on one-bit Raft. The ordinary and mixed-width results each moved less than 0.2%. [All 72 complete runs](perf/ivy_apple/goal_instances/nov22-unit-raft.csv)

After the change, independent replay confirmed the same 208 private refutations on nine Apple isolates. Every one of the 10,342 concrete results and all 18 recorded search counters reproduced the saved comparison with linked Z3. The 168 finite-model controls again yielded SAT on both certificate checks and zero private refutations. [Per-check reference values](perf/ivy_apple/goal_instances/nov22-projection-verified.csv.gz) · [Independent replay](perf/ivy_apple/goal_instances/verify.py) · [Satisfiable controls](perf/ivy_apple/goal_instances/finite_models/verify.py)

### Building the model finder's graph from each quantifier's own records

Z3 analyzes a quantifier when registering it. Later, when building a model, it visits that quantifier’s records in insertion order to construct the graph and populate instantiation sets. g3 already owned the same lists. It also copied their contents into one list, searched the growing list to suppress duplicates, and scanned the entire list twice for every quantifier. The Go model finder now traverses the registered lists directly. A quantifier can occur twice in the vector supplied by the macro finder. Each occurrence now sees exactly its own array-select records and charges the second-pass resource checkpoint once per record, as in C++. [C++ implementation](../src/smt/smt_model_finder.cpp#L1822) · [Go graph](smtcore/smt_model_finder_auf_graph.go) · [Repeated-quantifier regression](smtcore/smt_model_finder_auf_select_test.go)

| Incremental verification call | Records fed to the old copy | Function-variable comparisons | Entries inspected in both population scans | g3 before → direct, billion instructions | Linked Z3, billion instructions |
|---|---:|---:|---:|---:|---:|
| `rfn2` 7 | 9,798 | 6,597,867 | 4,232,736 | 6.892 → 6.486 | 2.065 |
| `rfn` 21 | 12,318 | 10,401,132 | 7,095,168 | 8.397 → 7.614 | 5.605 |
| `cf_live` 130 | 11,980 | 7,546,294 | 5,001,760 | 34.327 → 33.857 | 12.700 |

The comparison count includes the old append helper at registration and during replay. It excludes a separate literal-comparison loop, which ran 67,500 times on `rfn2` 7. The work counts come from an observer build. Instruction medians come from four rotating rounds of separate binaries, each measured on the pinned verification thread with an adjacent empty interval subtracted. Against `4e202c71d`, complete release processes saved a median within-pair **1.86%** on `rfn`, **0.95%** on `rfn2`, **0.26%** on `cf_live` and **0.13%** on `cmpl_live`. Six rounds on each isolate covered four variants. `populate` changes only the population scans, `direct` also removes the copy, and `direct-clean` also omits the redundant registration scan. The retained implementation inlines the remaining appends. [Visit counts](perf/ivy_apple/model_finder/nov22-graph-visits.csv) · [Per-call hardware counters](perf/ivy_apple/model_finder/nov22-incremental.csv.gz) · [All 96 complete runs](perf/ivy_apple/model_finder/nov22-release.csv)

| Registered quantifiers, eight records each | Before, µs/build | Direct lists, µs/build | Heap bytes/build before → direct |
|---:|---:|---:|---:|
| 8 | 55.5 | 31.1 | 17,488 → 7,824 |
| 32 | 351.7 | 106.3 | 63,597 → 23,213 |
| 128 | 3,871.6 | 432.5 | 256,749 → 84,013 |
| 512 | 51,882.6 | 1,893.0 | 1,367,661 → 347,053 |

The benchmark registers real quantified expressions before timing. Each iteration resets and rebuilds the graph. The table uses medians of nine rounds with 12 builds per sample on CPU 11. Reproduce it from `pkg/` with `go test ./smtcore -run '^$' -bench '^BenchmarkSMTModelFinderAUFGraph$' -cpu=1 -benchtime=12x -benchmem`. Across all nine Apple isolates, the final code reproduced the complete linked-Z3 transcripts, all 18 original solver counters on each of 10,342 checks, and the same 208 independently confirmed private refutations. [Benchmark](smtcore/smt_model_finder_auf_benchmark_test.go) · [All 144 microbenchmark samples](perf/ivy_apple/model_finder/nov22-graph-benchmark.csv) · [Reference values](perf/ivy_apple/goal_instances/nov22-projection-verified.csv.gz)

For physical source-line attribution on the original search, sample `rfn2` check 7 with `phase_stacks.py --engines g3 --call-graph fp --select 7 --period 1500007 --pairs 4 --keep-binaries` and the checker's usual GoIvy, spec, CPU, reference and output arguments. Then use `source_lines.py --binary OUTPUT/bin/g3 --period 1500007 OUTPUT/7-1-g3/stacks.gz`. In the first run, 102 of the 4,622 samples landed on the old function-variable comparison line. Every sample contributes to exactly one physical leaf and one enclosing stage. The subsequent visit census measures how often the loop ran. [Stack capture](perf/ivy_apple/phase_stacks.py) · [Physical source-line reader](perf/ivy_apple/quantifier_phases/source_lines.py) · [Complete first profile](perf/ivy_apple/model_finder/nov22-rfn2-7-lines.csv.gz)

### Ten complete workloads and the remaining incremental checks

The release crossover at `8c4be47e9` covered ordinary Raft and all nine Apple isolates. It includes the quantified Boolean projection and precedes the two subsequent changes to unit propagation and the model finder. All three builds used GoIvy `ef53df6a4`, `GOMAXPROCS=1`, disabled PGO and the same original specification and CPU within each workload. Each of six rounds ran g3, its preceding implementation and linked Z3. Every complete proof transcript matched. Other studies ran on the host, so the wall times describe this run’s load. [All 180 process measurements](perf/ivy_apple/goal_instances/nov22-ten-workloads.csv)

| Complete workload | Checks | g3 median, s | Linked Z3 median, s | g3, billion instructions | Linked Z3, billion instructions |
|---|---:|---:|---:|---:|---:|
| Ordinary Raft | 419 | 6.95 | 70.39 | 28.38 | 215.20 |
| Apple `rfn2` | 26 | 13.88 | 29.27 | 55.16 | 69.24 |
| `rfn` | 33 | 12.84 | 19.82 | 51.35 | 57.48 |
| `dramc_nb2` | 2,479 | 138.82 | 164.27 | 410.15 | 480.14 |
| `sys_live` | 1,738 | 102.97 | 117.03 | 300.57 | 337.91 |
| `e2e` | 2,378 | 134.03 | 156.72 | 387.48 | 435.61 |
| `tprops` | 700 | 53.41 | 54.84 | 162.42 | 168.75 |
| `cf_pio_live` | 2,215 | 174.17 | 177.37 | 486.32 | 539.68 |
| `cmpl_live` | 437 | 40.70 | 39.05 | 128.29 | 124.45 |
| `cf_live` | 336 | 66.00 | 47.14 | 185.38 | 107.59 |

The clearest remaining whole-workload gap is `cf_live`. Its 336 verification calls account for 98.53 billion g3 instructions and 34.00 billion linked-Z3 instructions. Check 130 alone accounts for 21.59 billion of that difference. Checks 58 and 48 add 4.84 and 3.26 billion. The similar `cf_pio_live` check 126 costs 35.94 billion versus 11.87 billion although g3 finishes the whole isolate faster. On `cmpl_live`, checks 35 and 177 together contribute 5.05 billion of the 9.79 billion additional instructions inside verification. On `rfn2`, g3 now executes 12.07 billion in all verification calls versus 35.48 billion for linked Z3. The general check 7 remains individually expensive. These totals sum the median of three thread-local, empty-interval-adjusted observations per check. [All 62,052 call measurements](perf/ivy_apple/goal_instances/nov22-nine-isolate-checks.csv.gz) · [Per-call measurement driver](perf/ivy_apple/incremental_cost/thread_checks.py)

### Ground witnesses for interacting quantifiers on Apple

The expensive `cf_live` check 130 combines quantified statements about the same clock domain. A negated universal supplies a counterexample term. A positive universal can then be instantiated at that term. The private prover at `ac4050bca` gives each closed quantified formula a Boolean representative and adds implications justified by its quantifier and polarity. It introduces fresh constants where an existential witness suffices and instantiates the other direction with well-sorted ground terms. These choices give a relaxation. Only a completed UNSAT result discharges the verification condition. Timeout, a satisfiable relaxation or an unsupported input returns control to the original solver. All generated terms and the auxiliary solver belong to a separate arena within the checking goroutine. [Admission](api/api_witness_instances_plan.go) · [Ground formulas](api/api_witness_ground.go) · [Private check and release](api/api_witness_check.go)

| `cf_live` verification call | Earlier g3, billion instructions | g3 with witness instances | Linked Z3 |
|---|---:|---:|---:|
| 130 | 34.074 | **1.354** | 12.700 |
| 58 | 7.535 | **0.465** | 2.553 |
| 56 | 2.826 | 5.175 | **0.945** |
| All 336 calls | 98.545 | **56.964** | 34.002 |

Each entry uses the median of three empty-interval-adjusted instruction counts on the verification thread. The total sums the per-call medians. Check 56 paid for an inconclusive private attempt and then ran the concrete solver. This makes the cost of a miss visible alongside the cost of a proof. The source goals, the incremental scope history and the proof transcript were the same for all three builds. [All 3,024 intervals](perf/ivy_apple/witness_instances/nov22-cf-live-checks.csv.gz) · [Counter boundaries and replay](perf/ivy_apple/incremental_cost/thread_checks.py)

| Complete checker, six rotated rounds | Earlier g3 | Bounded witness implementation | Linked Z3 |
|---|---:|---:|---:|
| `cf_live`, median seconds | 46.61 | **33.27** | 30.94 |
| `cf_live`, median billion instructions | 185.01 | **142.83** | 107.46 |
| `cmpl_live`, median seconds | 36.55 | 36.00 | 36.89 |
| `cmpl_live`, median billion instructions | 127.74 | 127.71 | 124.43 |

The witness implementation saved **22.8%** of complete `cf_live` instructions and still retired **32.9%** more than linked Z3. The complete-process counters include the GoIvy frontend, setup, verification and teardown. The linked build used the same GoIvy revision and specification with one CPU per isolate and one Go processor. Other studies ran on the host. An earlier six-round crossover before the final input bounds gave 28.24 → 21.36 s for g3 and 19.99 s for Z3 on `cf_live`. The corresponding instruction medians were 185.28 → 142.67 and 107.07 billion. Compare wall times within a crossover. [Final bounded crossover](perf/ivy_apple/witness_instances/nov22-release-short.csv)

### Verification and storage costs of the witness matcher

The original incremental solver independently confirmed all **242** private refutations across nine Apple isolates. The preceding implementation proved 208. All 10,342 original verdicts and all 18 recorded search counters, including resource steps, matched linked Z3 in the verification run that also performed the concrete check on every privately proved obligation. Concrete work can differ in ordinary release runs where a private proof skips a concrete check and its effects on later search. [Per-check verification records](perf/ivy_apple/witness_instances/nov22-verified.csv.gz) · [Verification driver](perf/ivy_apple/goal_instances/verify.py)

Satisfiable controls came from explicit interpretations of the uninterpreted sorts and functions in the captured formulas. Both shells accepted all 192 certificates built from eight fixtures, twelve seeds and domain sizes one and two. Fresh always-true and always-false predicates introduced quantified goals over every uninterpreted sort. All 192 entered the new admission rule and zero were privately refuted. A second run explicitly forced the matcher on all 192 and on 168 earlier controls that had missed its admission rule. It produced zero false refutations. To reproduce, run the finite model generator with `--with-quantified-witnesses`, then pass its output directory to `finite_trials.py`. [Model generator](perf/ivy_apple/goal_instances/finite_models/verify.py) · [Forced-proof driver](perf/ivy_apple/witness_instances/finite_trials.py) · [192 certificates](perf/ivy_apple/witness_instances/nov22-admitted-finite-models.csv) · [Admission and forced trials](perf/ivy_apple/witness_instances/nov22-explicit-admission.csv) · [168 earlier controls](perf/ivy_apple/witness_instances/nov22-forced-finite-models.csv)

A candidate tuple on the private arena needs only its slot IDs while its terms are held. Reusing ranking storage and carrying scores in the candidate records took the `cf_live` 130 prototype from 205.6 to 171.6 ms and from 61.06 to 48.86 MB allocated per proof. On the subsequent nine-round comparison, paged per-slot term records reduced 48.85 to 45.86 MB and 175.1 to 169.3 ms. Compiling repeated substitution templates in another prototype raised allocation to 50.34 MB for a smaller time saving. The retained benchmark skips parsing, creates a fresh private arena each iteration and reports proofs per operation. Run `go test ./api -run '^$' -bench '^BenchmarkAppleWitnessInstances$' -benchtime=3x -benchmem` from `pkg/`. The early prototype timings used different input validation and private solver setup. [Benchmark](api/api_witness_instances_benchmark_test.go) · [Dense tuples](perf/ivy_apple/witness_instances/nov22-dense-micro.csv) · [Term storage](perf/ivy_apple/witness_instances/nov22-tables-micro.csv) · [Template alternatives](perf/ivy_apple/witness_instances/nov22-recipe-micro.csv) · [Private solver alternatives](perf/ivy_apple/witness_instances/nov22-solver-micro.csv)

### Where the remaining Apple push instructions go

| All external Push calls within the isolate | g3, billion instructions | Linked Z3, billion instructions |
|---|---:|---:|
| `cf_live` | 18.60 | 5.36 |
| `cf_pio_live` | 30.84 | 8.25 |
| `cmpl_live` | 15.15 | 4.17 |
| `rfn2` | 12.65 | 3.53 |

The table gives medians of per-round sums after subtracting an adjacent empty interval for each operation. On `cf_live`, Assert added 1.54 vs 0.36 billion instructions, Pop 1.01 vs 0.27, and Reset 0.70 vs 0.21. Separate instruction sampling across all pushes attributed about 14.82 billion instructions outside assertion reduction in g3 versus 3.98 billion in Z3. The first g3 sample run recorded 2,925 of 5,143 samples under propagation, 1,751 under variable substitution and 1,177 under theory rewriting. These inclusive stacks overlap. Macro finding inside assertion reduction accounted for about 2.29 vs 0.63 billion instructions. A whole-process sampling window can include GC work on another thread. The per-thread intervals and sampled stacks therefore have different denominators. [Mutation intervals](perf/ivy_apple/witness_instances/nov22-mutation-calls.csv.gz) · [Pass estimates](perf/ivy_apple/witness_instances/nov22-push-passes.csv) · [g3 stacks](perf/ivy_apple/witness_instances/nov22-g3-push-stacks.csv.gz) · [Linked Z3 stacks](perf/ivy_apple/witness_instances/nov22-native-push-stacks.csv.gz) · [Sampling driver](perf/ivy_apple/incremental_cost/push_profile.py)

Two experiments follow directly from these observations. First, inspect the actual tuples in the native proof of `cf_live` 56 and test the missing instantiations with satisfiable controls. Second, compare substitution implementations on complete first pushes. A per-substitution speedup must cover the cost of preparing and retaining a plan over the lifetime of an independent solver.

### Three-variable relation axioms and ground goals with several sorts

The saved Z3 proof for `cf_live` 56 uses 19 distinct quantifier/tuple pairs. One invokes transitivity at **three** clock terms. g3 previously proposed a complete tuple when a single application mentioned every binder. Two applications of a binary relation can mention all three variables together while neither does so alone. The matcher now also instantiates the original quantified body at bounded triples of distinct, same-sort terms for a single binary predicate. Which instances it proposes affects cost and coverage. A guard on the axiom remains part of the formula. Tests cover both a transitive relation and a satisfiable guarded relation with the same three edges. [Native proof tuples](perf/ivy_apple/witness_instances/nov22-cf-live-56-native-tuples.csv) · [Matcher](api/api_witness_match.go) · [Relation controls](api/api_witness_relations_test.go)

On `rfn2` 7 the final goal is ground. There is one named constant in each of several domains. Instantiating only at the goal constants misses necessary intermediate terms. When at least four uninterpreted sorts each have an unambiguous constant, g3 can also try witnesses and applications from the premises. Goals with a function of arity three or higher get a second round. Broadening this to two sorts raised complete `dramc_nb2` from 409.3 to 425.8 billion instructions and `e2e` from 387.3 to 392.7 billion. The four-sort criterion kept both near baseline in the paired trial. [Ground-goal planner](api/api_goal_instances.go) · [All 96 domain trials](perf/ivy_apple/witness_instances/nov22-ground-domain-variants.csv)

| Complete isolate, six rotated rounds | Earlier g3, s | g3, s | Z3, s | Earlier g3, billion instructions | g3 | Z3 |
|---|---:|---:|---:|---:|---:|---:|
| `rfn2` | 10.45 | **9.53** | 19.88 | 54.79 | **49.12** | 69.23 |
| `cf_live` | 31.41 | **27.06** | 26.27 | 142.73 | **132.38** | 107.11 |
| `cmpl_live` | 33.06 | 33.28 | 33.98 | 127.74 | 127.39 | 124.30 |

Against the preceding two-binder matcher, instructions fell **10.3%** on `rfn2` and **7.3%** on `cf_live`. The latter remained 3.0% behind Z3 in median wall time and 23.6% above it in retired instructions. The difference between those ratios matters for a CPU-bound worker pool. Under a different concurrent host load, the bounded two-binder build on `cf_pio_live` already used 451.57 billion instructions against native’s 534.06 billion. [Complete three-binder crossover](perf/ivy_apple/witness_instances/nov22-three-binder-release.csv) · [Four-isolate prototype comparison](perf/ivy_apple/witness_instances/nov22-three-binder-variants.csv) · [Earlier bounded crossover](perf/ivy_apple/witness_instances/nov22-bounded-release.csv) · [Earlier ten-workload crossover](perf/ivy_apple/witness_instances/nov22-initial-ten-workloads.csv)

The original solver confirmed **266 private refutations**, up from 242, with the same outcomes and all 18 native search counters on each of the 10,342 Apple checks. All 360 previously certified finite models also survived the public and forced proof attempts. The prepared Go benchmark now requires a refutation on each iteration for `cf_live` 56, `cf_live` 130 and `rfn2` 7. The standalone fixtures start with a fresh solver. [Per-check replay](perf/ivy_apple/witness_instances/nov22-three-binder-verified.csv.gz) · [Per-model results](perf/ivy_apple/witness_instances/nov22-three-binder-finite.csv) · [Prepared benchmarks](api/api_witness_instances_benchmark_test.go) · [Captured rfn2 check 7](perf/ivy_apple/test_vectors/rfn2-7.smt2.gz)

### Selecting positive conjuncts for ground proofs

A trial over one uninterpreted sort now starts with the ground conjuncts of the premises. In two passes it adds quantified conjuncts that share a relatively rare function or predicate with the goal or a conjunct selected in the preceding pass. The frequency threshold for a conjunct is twice the number of premises containing its rarest declaration. Flattening stops at every connective except a positive top-level conjunction. The private search therefore receives actual source assumptions, including any guards. The source solver retains the complete input. The batch reuses its index only when the entire ordered sequence of premise handles, including arena identity and generation, matches. [Selection and bounds](api/api_witness_premises.go) · [Guard, open-input and scope tests](api/api_witness_premises_test.go)

| Complete checker, six rounds | Previous g3, s | Selected g3, s | Linked Z3, s | Previous g3, billion instructions | Selected g3 | Linked Z3 |
|---|---:|---:|---:|---:|---:|---:|
| Ordinary Raft | 4.58 | 4.71 | 47.86 | 27.85 | 27.85 | 215.20 |
| `rfn2` | 8.65 | 9.01 | 24.84 | 49.16 | 48.99 | 69.23 |
| `rfn` | 12.29 | 12.28 | 22.38 | 50.53 | 50.46 | 57.40 |
| `cmpl_live` | 38.68 | 38.22 | 38.34 | 127.23 | 127.33 | 124.38 |
| `cf_live` | 33.20 | 32.95 | 33.62 | 132.46 | 123.14 | 107.38 |

The table compares identical transcripts across rotated engine order on a pinned CPU per workload. Other jobs shared the host. `cf_live` retired 7.0% fewer instructions than the previous g3 and 14.7% more than linked Z3. The wall times were much closer. All 266 private refutations were independently confirmed on the original incremental solver. Verdicts and the 18 recorded search counters agreed with native on every one of 10,342 Apple checks. The public proof and forced matcher also preserved the 360 certified finite models. [All 90 complete runs](perf/ivy_apple/witness_instances/nov22-selected-release.csv) · [Concrete replay](perf/ivy_apple/witness_instances/nov22-selected-verified.csv.gz) · [Satisfiable controls](perf/ivy_apple/witness_instances/nov22-selected-finite.csv)

| Private phases, median of three per-isolate sums, billion instructions | `cf_live` before | Selected | `cf_pio_live` before | Selected |
|---|---:|---:|---:|---:|
| Translate selected assertions | 0.599 | 0.427 | 0.639 | 0.449 |
| Generate ground instances | 2.925 | 1.145 | 3.028 | 1.135 |
| Assert ground formulas | 1.365 | 0.753 | 1.426 | 0.759 |
| Run the private solver | 12.080 | 6.312 | 13.252 | 6.511 |

Each interval used a hardware counter attached to the verification thread and an adjacent empty read. The search interval measures all private ground plans, including the simpler one-tuple plan. The three preparation rows cover the witness matcher. The phase totals exclude GoIvy, public solver search, selection itself and private context teardown. A standalone driver reproduces the counters on Linux/amd64 with a GoIvy checkout using the local replacement and a linked-Z3 transcript from the same specification path. [Driver](perf/ivy_apple/witness_instances/phase_instructions.py) · [All 4,086 intervals](perf/ivy_apple/witness_instances/nov22-selected-phases.csv.gz)

Prepared inputs show the tradeoff between less work on a successful subset and an extra attempt on a miss. In eight alternating pairs, `cf_live` 56 went from 525.59 to 79.56 ms and from 72.59 to 12.24 MB per proof. On check 130 the subset was satisfiable. Falling back to the full set brought the cost from 327.82 to 412.27 ms and 44.12 to 55.63 MB. Each iteration still proved both original obligations. `rfn2` 7 stayed at 37.9 MB per proof. The benchmark excludes parsing and includes the fresh private solver, its search and its release. [Prepared Go benchmark](api/api_witness_instances_benchmark_test.go) · [All 48 measurements](perf/ivy_apple/witness_instances/nov22-selected-micro.csv)

### Ground Boolean proofs on the Apple liveness checks

Linked Z3's standalone proof for `cmpl_live` check 43 contains zero quantifier instantiations. g3 can prove the same obligation using the final Boolean goal and the ground conjuncts of the original premises. It splits conjunctions only in positive, asserted positions. Each ground theory atom has one propositional variable per exact arena handle, so a concrete model always gives an assignment to the projected problem. A contradiction discharges the query. A satisfying assignment or an exhausted private budget returns control to the concrete solver. The production path admits a signed ground goal over at least two distinct Boolean constants and reuses the existing bounded CNF prover. [Projection](api/internal/order/ground_boolean.go) · [Captured check 43](perf/ivy_apple/test_vectors/cmpl_live-43.smt2.gz)

| Complete workload | Previous g3, billion instructions | Ground Boolean proofs, billion instructions | Linked Z3, billion instructions |
|---|---:|---:|---:|
| `cmpl_live` | 126.67 | 121.15 | 124.13 |
| `cf_live` | 122.95 | 119.71 | 107.68 |
| `rfn2` | 48.99 | 48.98 | 69.25 |
| Ordinary Raft | 27.93 | 27.82 | 215.20 |

These medians cover the first three complete rounds with the same frontend, CPU and proof transcript for each engine. The shared host had substantial timing variation. On `cf_live`, for example, these three ground-projection runs had a 53.09 s median while the first six had a 32.46 s median. Retired instructions put g3 below linked Z3 on `cmpl_live` and still above it on `cf_live`. [Release measurements](perf/ivy_apple/goal_instances/ground_booleans/nov22-release.csv)

On the standalone check 43, nine pinned rounds measured a median **875.0 ms** and **129.74 MB** per fresh concrete proof, **43.2 ms** and **7.40 MB** per fresh projected proof, and **0.622 ms** and **0.291 MB** per projected proof once the source assertions were loaded. Fresh includes parsing and creating the solver. Prepared uses a sequence of checks on the same batch. Run `go test ./api -run '^$' -bench '^BenchmarkAppleGroundBooleans$' -benchmem` from `pkg/`. [Benchmark and focused truth-table tests](api/api_ground_boolean_test.go) · [All 27 measurements](perf/ivy_apple/goal_instances/ground_booleans/nov22-prepared.csv)

Across nine Apple isolates, private proofs increased from **266 to 882** of 10,342 checks. The verification build also ran each original incremental check. All verdicts and all 18 per-check search counters matched linked Z3. Both shells accepted 192 new explicit finite-model certificates with Boolean goals. The public proof path returned inconclusive on these and all 360 earlier known-satisfiable controls. Of the 192 new inputs, 144 also entered the ground Boolean projection in the direct test, with zero false refutations. The generator preserves the original satisfiable premises and adds goals using implication, conditional, equality, XOR and distinct. To repeat, run the finite-model generator with `--ground-booleans`, then run `projection_trials.py` on its output directory. [Concrete confirmations](perf/ivy_apple/goal_instances/ground_booleans/nov22-verified.csv.gz) · [Certificate results](perf/ivy_apple/goal_instances/ground_booleans/nov22-certificates.csv) · [All direct trials](perf/ivy_apple/goal_instances/ground_booleans/nov22-finite.csv.gz) · [Generator](perf/ivy_apple/goal_instances/finite_models/verify.py) · [Direct runner](perf/ivy_apple/goal_instances/finite_models/projection_trials.py)

### Where ground Boolean proofs save solver work

| Verification calls within the isolate | Previous g3, billion instructions | g3 with ground Boolean proofs | Linked Z3 |
|---|---:|---:|---:|
| `cf_live` | 38.560 | 35.691 | 34.003 |
| `cmpl_live` | 13.307 | 7.261 | 4.288 |
| `tprops` | 2.492 | 2.316 | 0.647 |
| `rfn2` | 6.222 | 6.374 | 35.480 |

On `cmpl_live` check 43, the Boolean proof reduced g3's median from 3.913 billion instructions to 8.6 million. Linked Z3 used 506.6 million. On `cf_live` check 69, the same figures were 1.307 billion, 6.7 million and 4.5 million. Check 177 now dominates the remaining `cmpl_live` search, at 3.85 billion instructions in g3 and 1.40 billion in Z3. On `rfn2`, check 26 still took 2.71 billion versus 0.98 billion, even though g3 used fewer instructions over the full set of calls. [13,491 individual counter intervals](perf/ivy_apple/goal_instances/ground_booleans/nov22-check-instructions.csv.gz)

Each cell is the median of three whole-isolate sums. The counter surrounds the verification call on its solver thread and subtracts an adjacent empty interval. Every process reproduced the linked-Z3 proof transcript. All recorded counters ran for their full enabled interval. Time spent parsing Ivy or opening an incremental scope falls outside this measurement. [Counter and reproduction driver](perf/ivy_apple/incremental_cost/thread_checks.py)

### Substituting bound variables in the general rewriter

The C++ rewriter pushes the result of a completed application reduction once. The Go rewriter pushed it twice, with an intervening pop and two reference count updates. The Go completion path now performs the single push. Variable substitution also finishes ground leaves and bound variables directly. A newly pushed substitution frame has already missed the cache during its visit, so its first step can proceed to its children. The resource-limit poll still runs on every frame step. A differential test compares completed terms, resource counts, and the first successful budget with the general rewriter on a shared DAG. [Native reduction](../src/ast/rewriter/rewriter_def.h) · [Go frame handling](ast/rewriter/rewriter_engine.go) · [Budget test](ast/rewriter/var_substitution_test.go)

| Instance lifetime | Before, instructions/op | After, instructions/op | Before, ns/op | After, ns/op |
|---|---:|---:|---:|---:|
| Retained | 14,473 | 13,991 | 1,931 | 1,781 |
| Released on each iteration | 21,499 | 21,018 | 2,888 | 2,780 |

The table gives medians of nine alternating pairs pinned to CPU 14, with 20,000 substitutions per process and zero timed allocations. The instruction figure includes process startup and benchmark setup divided by 20,000. On complete GoIvy runs, median instructions fell from 120.19 to 119.11 billion on `cf_live`, 49.17 to 48.77 billion on `rfn2`, 121.82 to 121.58 billion on `cmpl_live`, and 402.79 to 402.16 billion on `dramc_nb2`. Each used four paired rounds on a shared host. The original incremental solver confirmed all 882 independent refutations and matched linked Z3 on the verdict and 18 counters of every Apple check. [Substitution benchmark](smtcore/smt_quantifier_substitution_benchmark_test.go) · [36 microbenchmark runs](perf/ivy_apple/rewriter_cache/nov22-beta-substitution.csv) · [32 complete runs](perf/ivy_apple/rewriter_cache/nov22-beta-release.csv) · [10,342 checked obligations](perf/ivy_apple/rewriter_cache/nov22-beta-confirmations.csv)

### Complete workload panel at the ground Boolean change

The six-round panel at `c4b5a5b68` reproduced the linked-Z3 transcript on every run. On `rfn2`, g3 took 12.94 s against 30.88 s for linked Z3. The same frontend ran both solvers with the same CPU affinity for each workload. Other experiments shared the host, so the instruction counts give a more stable comparison than elapsed time.

| Workload | Checks | g3, s | Linked Z3, s | g3, billion instructions | Linked Z3, billion instructions |
|---|---:|---:|---:|---:|---:|
| Ordinary Raft | 419 | 5.52 | 50.56 | 27.81 | 215.19 |
| `dramc_nb2` | 2,479 | 119.77 | 152.75 | 402.55 | 479.82 |
| `rfn2` | 26 | 12.94 | 30.88 | 49.04 | 69.32 |
| `cf_live` | 336 | 32.46 | 37.91 | 120.07 | 107.67 |
| `cf_pio_live` | 2,215 | 131.06 | 175.06 | 427.31 | 538.89 |
| `cmpl_live` | 437 | 76.75 | 81.18 | 121.24 | 124.29 |
| `e2e` | 2,378 | 100.99 | 144.89 | 378.51 | 434.16 |
| `rfn` | 33 | 12.42 | 23.34 | 50.59 | 57.48 |
| `sys_live` | 1,738 | 85.31 | 107.53 | 297.35 | 337.96 |
| `tprops` | 700 | 39.49 | 40.72 | 162.12 | 168.57 |

On `cf_live`, g3 finished sooner in this panel but retired more instructions. It was the only one of the ten workloads with that instruction gap. The raw results include the preceding g3 build, each process's peak RSS, and all four hardware counters. [All 180 complete runs](perf/ivy_apple/goal_instances/ground_booleans/nov22-ten-workloads.csv)

### Repeated concrete queries on an incremental solver

The Apple checker sometimes pushes a scope and asks exactly the same question it recently proved in a previous scope. On `cf_live`, recognizing duplicates in the shared GoIvy frontend reduced g3 from 120.13 to 113.08 billion instructions but raised linked Z3 from 107.63 to 110.20 billion. The shipped change lives in g3's existing proof batch. It remembers eight UNSAT goals under one ordered sequence of up to 128 premise handles. An arena owner, slot and generation identify each formula. The batch owns the history and holds no AST alive. [Frontend experiment](perf/ivy_apple/goal_instances/recent_checks/nov22-frontend-experiment.csv) · [Batch history](api/api_order_recent.go)

The source solver can change between the batch's proof attempt and the caller's concrete check. g3 rechecks every assertion and requires zero explicit or tracked assumptions before recording a concrete UNSAT result. It uses only a solver with an original assertion vector. The bare SMT kernel exposes preprocessed formulas, and a failed scope or assertion update can leave the two components of a combined solver out of sync. Both cases bypass reuse. Finite limits and cancellation retain their original behavior. The tests include same-length premise changes, slot recycling, scope mutations, eviction and all 128 combinations of a four-bit assignment and a signed goal (16 assignments × 8 goals). [Source check](api/api_simple_solver_check.go) · [Regression tests and benchmark](api/api_order_recent_test.go)

| Full checker, four pairs | Previous g3, billion instructions | g3 with batch history |
|---|---:|---:|
| `cf_live` | 119.14 | 112.04 |
| `cmpl_live` | 121.61 | 121.62 |
| `rfn2` | 48.44 | 48.53 |

The captured `cf_live` check 42 costs 84.79 ms and 5.23 MB per iteration with the previous batch. After one warm concrete proof, the new batch measured **80.8 µs and 28.2 kB** per push, assert, proof and pop. Nine alternating pairs used five iterations for concrete checks and 512 for warm hits. Ordinary concrete checks in the same runs took 85.03 and 83.01 ms respectively. [Captured query](perf/ivy_apple/test_vectors/cf_live-42.smt2.gz) · [36 prepared measurements](perf/ivy_apple/goal_instances/recent_checks/nov22-prepared.csv) · [24 complete runs](perf/ivy_apple/goal_instances/recent_checks/nov22-short-release.csv)

Batch proofs increased from 882 to 1,359 across 10,342 Apple checks. An instrumented build also executed the original check for each shortcut. All verdicts and all 18 per-check search counters matched the linked-Z3 reference in that replay. The release build also reproduced all 11,732 named decisions on 28 mutated Raft specifications, including the failing obligations. [Apple concrete confirmations](perf/ivy_apple/goal_instances/recent_checks/nov22-confirmations.csv.gz) · [Raft positive and negative cases](perf/ivy_apple/goal_instances/recent_checks/nov22-raft-mutants.csv)

The longer refutation-history comparison also measured `cf_pio_live` and `dramc_nb2`. Median instructions across four pairs fell from 425.81 to 420.77 billion on the former and rose from 400.72 to 402.16 billion on the latter. Each pair ran the same complete checker on one pinned CPU. [All 40 runs](perf/ivy_apple/goal_instances/recent_checks/nov22-release.csv)

### Which general-solver calls still cost more

Sampling the solver thread across every call in four isolates shows how the target has shifted. At the ground Boolean frontier, g3 already spent far less on `rfn2` check 4. The remaining `rfn2` general-solver target is check 26. `cf_live` and `cmpl_live` also have expensive calls even where the complete checker finishes sooner.

| Obligation | All samples, g3 / Z3 | General rewriter | Instantiation queue | Application construction | Candidate checker |
|---|---:|---:|---:|---:|---:|
| `rfn2` 4 | 119 / 13,036 | 41 / 1,202 | 0 / 896 | 7 / 876 | 0 / 2,825 |
| `rfn2` 26 | 1,688 / 611 | 291 / 77 | 369 / 81 | 133 / 54 | 182 / 124 |
| `cf_live` 47 | 913 / 302 | 384 / 113 | 163 / 37 | 228 / 103 | 28 / 18 |
| `cf_live` 122 | 1,149 / 342 | 354 / 81 | 206 / 35 | 145 / 74 | 88 / 64 |
| `cmpl_live` 177 | 2,374 / 877 | 579 / 140 | 360 / 85 | 225 / 97 | 339 / 219 |

Each cell gives median sample counts over three complete runs per engine, before the concrete-history change. Linux perf sampled every 1,600,033 retired user instructions. Samples had to match both the time interval of a verification call and the Linux thread running it. Each stack was assigned to its nearest classified frame. In `cmpl_live` check 177, the rewriter difference alone corresponds to roughly 0.70 billion instructions. These estimates have sampling error and exclude other runtime threads. Reproduce with `phase_stacks.py --by-check --select inside --pairs 3 --period 1600033`, using `--engines g3 --call-graph fp` and `--engines native --call-graph dwarf,8192` in separate output directories. [Per-check counts](perf/ivy_apple/incremental_cost/nov22-all-check-stacks.csv.gz) · [Thread and window coverage](perf/ivy_apple/incremental_cost/nov22-all-check-stack-coverage.csv) · [Selected leaf symbols](perf/ivy_apple/incremental_cost/nov22-residual-check-leaves.csv.gz) · [Profiling driver](perf/ivy_apple/phase_stacks.py)

### A scope boundary replaces the Boolean creation-stamp array

Boolean variable IDs are dense. Before a backjump, g3 already knows the first ID created in the scope it is about to pop. A retained lemma now records an ID only when it lies below that boundary. It retains the expression in either case. This implements the surviving-variable test used by the in-tree C++ solver and eliminates the separate eight-byte creation stamp per Boolean, its per-solver counter and a second slice header for the Boolean trail. The main Boolean record remains 16 bytes with an eight-byte justification. A regression test pops a nongate NOT, recreates the same expression at the same ID, and verifies the learned literal is normalized again. Interrupted reference releases retain their original place on the core trail. [Go implementation](smtcore/smt_search.go) · [Scope state](smtcore/smt_internalizer_state.go) · [Native backjump](../src/smt/smt_context.cpp) · [Recycled-ID tests](smtcore/smt_lemma_literal_cache_test.go)

| Queue-enabled fixture | Before | After |
|---|---:|---:|
| Scoped pop, instructions | 208,713 | 201,950 |
| Scoped trail undo, instructions | 58,230 | 57,269 |
| Scoped clause restoration, instructions | 113,632 | 107,561 |
| Surviving pop, instructions | 2,342 | 2,323 |
| Post-pop bytes per solver, 1,024 workers × 128 atoms | 55,317 | 53,269 |
| Post-pop bytes per solver, 48 workers × 2,048 atoms | 803,340 | 782,867 |

Instructions are medians of seven alternating pairs on one thread, each with 1,500 measured pops after warmup and its matching empty interval subtracted. Heap readings are medians of five pairs after collection. The smaller fixture saves 2 MiB across 1,024 independent solvers. The larger fixture saves 20,473 bytes per solver at the capacity reached by its Boolean slice. Four whole-checker pairs changed by less than 0.4% in either direction on `cf_live`, `cmpl_live` and `rfn2`. Both sides of these particular comparisons used the same experimental arena application lookup. [Thread counters](perf/scope_restoration/boolean_lifetime/nov22-solver-thread.csv) · [Worker measurements](perf/scope_restoration/boolean_lifetime/nov22-workers.csv) · [Worker benchmark](smtcore/smt_scope_workers_benchmark_test.go) · [Whole checkers](perf/scope_restoration/boolean_lifetime/nov22-checkers.csv)

Earlier prototypes assigned an epoch to each pushed scope, looked up the owning scope on demand, or threaded the surviving bound through clause reconstruction. Capturing the bound before the pop removes the lifetime array without another generation counter, a scope search, or a dependency on a future scope's ID. [Epoch and trail experiments](perf/scope_restoration/boolean_lifetime/nov22-epochs.csv) · [Survivor and literal-buffer experiments](perf/scope_restoration/boolean_lifetime/nov22-alternatives.csv)

### Binary relations in violated postconditions

Some Apple checks ask whether a transition can violate one of several postconditions about named objects. When a negated implication has a conjunctive consequent, g3 looks for a binary Boolean relation over one uninterpreted sort and at least three distinct constant terms of that sort. The constants can denote the same object. The existing private matcher takes one round over the selected connected premises with width three. Only its UNSAT answer discharges a check. The batch can also remember a prior independent refutation under the same guarded input identity. [Goal planner](api/api_goal_instances.go) · [Selected-premise check](api/api_goal_instances_check.go) · [Admission tests](api/api_goal_relation_test.go)

| Complete workload, four pairs | Previous g3, billion instructions | With relation selection and batch reuse |
|---|---:|---:|
| `cf_live` | 110.67 | 106.81 |
| `cf_pio_live` | 418.78 | 413.60 |
| `cmpl_live` | 120.72 | 120.88 |
| `dramc_nb2` | 401.30 | 401.20 |
| `rfn2` | 48.32 | 48.40 |

Both builds in each pair included the Boolean scope boundary and the same candidate arena lookup. Requiring a violated postcondition mattered. In an earlier trial, trying any ground goal that used the relation raised `cf_pio_live` from about 420 to 424 billion instructions, even though some individual calls became cheaper. The complete-checker comparison captures work in later scopes and in different solvers. [All 40 release runs](perf/ivy_apple/goal_instances/ground_relations/nov22-release.csv) · [Alternative admission rules](perf/ivy_apple/goal_instances/ground_relations/nov22-admission-trials.csv)

The independent paths discharged 1,391 of the 10,342 Apple checks, up from 1,359. Running the original solver as well on every obligation matched the linked-Z3 verdict and all 18 search counters. For a negative control, the generator built 240 satisfiable problems from eight captured obligations, ten seeds and domains of sizes one through three. The models cover empty, total, equality, cyclic and arbitrary binary relations. Both shells accepted every explicit model. All 240 source queries reached the new matcher, both the selected-premise call and the public proof entry returned inconclusive, and neither produced a false refutation. [Apple counters](perf/ivy_apple/goal_instances/ground_relations/nov22-confirmations.csv.gz) · [Certified model results](perf/ivy_apple/goal_instances/ground_relations/nov22-explicit-models.csv) · [Generator](perf/ivy_apple/goal_instances/finite_models/verify.py) · [Direct matcher runner](perf/ivy_apple/goal_instances/finite_models/projection_trials.py)

### Cost of retiring one scoped Boolean

After eliminating the creation stamps, retiring one variable on the queue-enabled fixture still took **416 g3 instructions versus 130 in Z3**. Median isolated costs were 190 versus 85 for the branching queue, 51 versus 9 for removing bindings, 84 versus zero for logically removing the variable slots, and 63 versus 24 for releasing the expression. Native keeps its arrays at their high-water mark and uses the trail length to determine which IDs exist. Each measurement subtracts a matching empty interval. The regions were timed in separate instruments, so their medians need not sum to the whole. Seven pairs covered 128,000 variable retirements per phase per process. [Per-variable counters](perf/scope_restoration/boolean_lifetime/nov22-variable-phases.csv) · [Instrumentation and native boundaries](perf/scope_restoration/pop_phases/variables.py)

The complete-checker comparison can now run Raft and all nine Apple isolates with one command. Each workload stays on one CPU and alternates which engine runs first. The driver checks the number of obligations, compares every transcript with its paired run and optionally with a saved reference, and requires full hardware-counter coverage. Use the same spelling of the specification path as in the reference because GoIvy includes paths in its output. [Ten-workload driver](perf/ivy_apple/workload_panel.py) · [Individual comparisons](perf/ivy_apple/compare.py)

### Inconsistent premises on `rfn2` check 26

After the earlier reductions on check 4, the expensive remaining call on `rfn2` had a constant true final assertion. The premises already implied a contradiction. For this shape g3 tries the ground facts and short universal formulas that occur as positive top-level conjuncts. A trial requires at least eight of each. A universal qualifies when the existing premise traversal visits at most 12 distinct expression nodes in that conjunct. The solver checks that the full input is closed, translates the selected formulas to a private arena, and uses at most 80,000 steps. UNSAT for this subset proves UNSAT for the original conjunction. Other answers resume the ordinary proof path. [Selection and translation](api/api_short_premises.go) · [Alternative selection trials](perf/ivy_apple/goal_instances/short_premises/nov22-selection-trials.csv)

| Median retired instructions | Previous g3 | With short premises | Linked Z3 |
|---|---:|---:|---:|
| Check 26, solver thread, five rounds | 2,657.52 million | 96.28 million | 978.06 million |
| Complete `rfn2`, four paired runs | 48.45 billion | 45.19 billion | |
| Complete `dramc_nb2`, four paired runs | 401.28 billion | 399.25 billion | |
| Complete `tprops`, four paired runs | 161.52 billion | 160.49 billion | |
| Complete `cf_live`, four paired runs | 106.91 billion | 106.39 billion | |
| Complete `cmpl_live`, four paired runs | 121.09 billion | 121.07 billion | |

The thread counts subtract an empty interval in the same process. The full-checker comparison includes identical arena lookup code on both sides. On the captured check, seven alternating pairs using fresh solvers, including parsing, measured 1,547 versus 55 ms and 117.32 versus 10.59 MB allocated. Timings were collected alongside other experiments. The instruction measurements and complete transcripts provide a second comparison. Run `go test ./api -run '^$' -bench '^BenchmarkAppleShortPremises$' -benchtime=3x -benchmem` from `pkg/`. [Captured query](perf/ivy_apple/test_vectors/rfn2-26.smt2.gz) · [Benchmark and regression tests](api/api_short_premises_test.go) · [All 390 check intervals](perf/ivy_apple/goal_instances/short_premises/nov22-rfn2-calls.csv.gz) · [40 complete runs](perf/ivy_apple/goal_instances/short_premises/nov22-release.csv) · [Captured results](perf/ivy_apple/goal_instances/short_premises/nov22-captured.csv)

Independent proofs increased from 1,391 to 1,421 across the nine Apple isolates. Replaying the original solver on every one of the 10,342 checks matched linked Z3 on every verdict and all 18 per-check search counters. The finite-model controls include disjunctions whose individual disjuncts would be unsound to select independently. Both shells accepted all 240 model certificates, every example reached the new selection, and all private attempts correctly declined to refute those inputs. [Apple replay](perf/ivy_apple/goal_instances/short_premises/nov22-confirmations.csv.gz) · [240 finite-model controls](perf/ivy_apple/goal_instances/short_premises/nov22-explicit-models.csv) · [Model generator](perf/ivy_apple/goal_instances/finite_models/verify.py)

### Seven independent Ivy examples expose the cost of creating a strategic solver

I chose seven non-Apple, non-Raft examples that completed with named proof decisions. Three of the examples have a failing obligation. The paired checkers agreed on all 505 passes, all three failures, and the complete stdout of each specification. The failure runs also exited with status 1 and reported the expected number of failed checks. These results describe the seven selected specifications. [Examples and paired checker driver](perf/ivy_apple/holdout_examples.py)

| Whole spec, billion instructions | g3 | Linked Z3 | g3 / Z3 inside Assert calls |
|---|---:|---:|---:|
| Toy consensus | 11.55 | 4.13 | 6.04 / 1.37 |
| Ticket | 7.21 | 3.02 | 2.27 / 0.51 |
| Progress | 2.38 | 0.86 | 1.51 / 0.35 |
| Ping-pong | 1.78 | 0.51 | 1.52 / 0.35 |
| Leader election ring | 1.34 | 0.48 | 0.71 / 0.16 |
| Election between nodes | 1.52 | 0.53 | 0.70 / 0.16 |
| Deliberately faulty ring | 0.63 | 0.18 | 0.35 / 0.08 |

Each number is the median of three runs. The first two columns measure the complete process. The last column sums the solver-thread counters around assertion calls made outside verification checks, subtracting their empty intervals. The `short` build contains the short-premise path and the 4,096-entry arena lookup experiment. A separate `general` build skips independent proof attempts. On toy consensus it retired 10.57 billion instructions, including 1.54 billion in verification checks. With private attempts enabled the verification calls took 2.52 billion. [63 complete runs](perf/ivy_apple/holdouts/nov22-strategic-checkers.csv) · [Individual verification calls](perf/ivy_apple/holdouts/nov22-strategic-call.csv.gz) · [Individual push, assert, pop and reset calls](perf/ivy_apple/holdouts/nov22-strategic-mutations.csv.gz) · [Counter driver](perf/ivy_apple/holdouts/probe.py)

Most of the assertion cost occurs on the **first assertion after creating a solver**. One ping-pong run had 1,890 instruction samples in the g3 assertion API. Of those, 1,886 included solver initialization and 1,616 included construction of the default strategy. Its tactic constructors repeatedly read parameter sets for their nested constituents. Across the complete ping-pong run g3 made 932,611 named parameter queries. In 267,241 the local and fallback sets both contained zero entries. In another 548,184 the local set had two, three or four entries and the fallback had zero. All counts refer to actual calls, so they include repeated lookups. [Three paired stack profiles on each of three examples](perf/ivy_apple/holdouts/nov22-assert-stacks.csv.gz) · [Parameter sizes](perf/ivy_apple/holdouts/nov22-lookup-counts.csv) · [Profiling driver](perf/ivy_apple/holdouts/assert_stacks.py)

A fresh strategic solver asserting just `true` took **108.60 million instructions in g3 and 27.96 million in native Z3**. With `smt.macro_finder=true` the counts were 125.52 and 29.90 million. This fixture reuses the context, one expression and the options across solver lifetimes. It measures creation, first assertion and release without a search. Each of seven paired rounds subtracts the process counts for three solver lifetimes from those for thirteen, then divides by ten. The native control links this fork's static library. [Go benchmark](api/api_solver_startup_benchmark_test.go) · [C++ control](../src/test/perf/strategic_solver_startup.cpp) · [Instruction slopes](perf/ivy_apple/holdouts/nov22-startup-instructions.csv) · [Comparison driver](perf/ivy_apple/holdouts/startup.py)

### Repeated application construction in the existing AST arena

A repeated four-operand application now takes **490 retired instructions in g3 and 605 in this fork of Z3**. Before this change g3 took 1,265. A successful lookup in the canonical table records a weak position in a manager-local array. On the next request g3 verifies the declaration, arity and every operand against the live arena slot, including owner and generation. A hit consumes the same free-slot generation as ordinary construction. A stale position or a different middle operand goes through normal registration. The input arena and solver ownership model stay the same. [Implementation](ast/ast_app_slice.go) · [Lifetime and collision tests](ast/ast_app_hint_test.go)

| Median instructions per application, nine rounds | Previous g3 | g3 with hints | Native Z3 |
|---|---:|---:|---:|
| Repeat one unary application | 830 | 422 | 406 |
| Repeat one binary application | 935 | 442 | 456 |
| Repeat one ternary application | 996 | 470 | 500 |
| Repeat one four-operand application | 1,265 | 490 | 605 |
| Rotate among 256 resident four-operand applications | 1,540 | 802 | 623 |
| Construct and release a fresh four-operand application | 4,069 | 4,083 | 828 |

Each row subtracts an empty interval from a 200,000-iteration measurement on the same thread. A run of 32 unsuccessful probes disables speculation until another canonical hit. The first hint occupies one scalar. Larger tables grow on hits up to 1,024 positions, or 4 KiB per active manager. Increasing the cap to 4,096 positions saved less than 0.4% of whole-process instructions on each of three tested isolates while permitting 12 KiB more per manager. The 1,024-solver scope fixture held about 53 KiB per idle solver in both builds. [Instruction measurements](perf/ivy_apple/application_reuse/nov22-hints-instructions.csv.gz) · [Six capacity trials](perf/ivy_apple/application_reuse/nov22-hint-size-trials.csv) · [Observed manager capacities](perf/ivy_apple/application_reuse/nov22-hints-capacity.csv) · [Worker memory](perf/ivy_apple/application_reuse/nov22-hints-workers.csv)

Four paired complete runs reduced median instructions from 45.56 to 45.23 billion on `rfn2`, 107.29 to 106.36 on `cf_live`, 121.28 to 121.01 on `cmpl_live` and 414.44 to 413.80 on `cf_pio_live`. Replaying all 10,342 Apple checks matched every linked-Z3 verdict and all 18 search counters. In the measured runs, 1,421 checks had independent refutations available. The request counter includes both ways of answering a duplicate. The separate phase probe disables speculation to expose each stage of the canonical path. [Complete runs](perf/ivy_apple/application_reuse/nov22-hints-checkers.csv) · [Per-check replay](perf/ivy_apple/application_reuse/nov22-hints-confirmations.csv.gz) · [Application requests during rfn2](perf/ivy_apple/application_reuse/nov22-hints-rfn2-requests.csv)

### Publishing and retiring a fresh arena application

Constructing and immediately releasing a fresh four-operand application fell from **4,083 to 2,950 instructions**, a 27.8% reduction. Native took 828. The previous code validated each operand, then resolved its handle again during publication to retain the child and compute the parent's flags. The new path borrows each validated slot until publication completes. Arena pages have stable addresses while the canonical table grows. On the last reference to an inline application, deletion visits the declaration and operands directly in source order. A diamond-shaped fixture checks that repeated occurrences of the same leaf count separately and that the cascade returns the expected placements in order. [Construction and publication](ast/ast_app_slice.go) · [Deletion](ast/ast_core_delete.go) · [Lifetime regression](ast/ast_app_lifetime_test.go)

The 256-resident four-operand fixture stayed at 802 instructions, and the single resident fixture stayed at 490. Four paired whole-checker runs went from 45.38 to 45.03 billion instructions on `rfn2`, 106.49 to 105.84 on `cf_live`, 399.95 to 399.72 on `dramc_nb2` and 120.63 to 120.95 on `cmpl_live`. The nine-isolate replay produced the same 10,342 rows, including all 18 search counters and private-proof indicators, as the preceding arena change. The remaining construction gap is concentrated in fresh terms. The repeated-term gain also depends on how many distinct terms remain live. [Seven-round microbenchmark](perf/ivy_apple/application_reuse/nov22-lifetime-instructions.csv.gz) · [32 complete checker runs](perf/ivy_apple/application_reuse/nov22-lifetime-checkers.csv) · [Retirement experiments](perf/ivy_apple/application_reuse/nov22-app-lifecycle-prototypes.csv.gz) · [Publication experiments](perf/ivy_apple/application_reuse/nov22-app-publish-prototypes.csv.gz)

### Go compiler profiles trained on different verification workloads

Go profile-guided optimization reduced complete g3 instruction counts by **5.3–5.5%** on four Apple isolates excluded from training. The same experiment with the linked-Z3 checker reduced its counts by **2.2–4.9%**. Each backend had its own training binary and profile. Native Z3's linked C++ library was identical in the ordinary and profile-guided builds, so the latter comparison measures compiler changes to the Go portion of the native checker. [All 64 measurements](perf/ivy_apple/compiler_profiles/nov22-independent-training.csv)

| Held-out workload, billion instructions | g3 ordinary | g3 PGO | Linked Z3 ordinary | Linked Z3 PGO |
|---|---:|---:|---:|---:|
| rfn2 | 48.70 | 46.10 | 69.26 | 67.75 |
| cf_live | 107.73 | 101.79 | 106.97 | 103.98 |
| tprops | 161.96 | 153.09 | 167.80 | 160.75 |
| dramc_nb2 | 401.63 | 380.51 | 478.20 | 454.55 |

Go 1.27.3 built g3 commit `1e6ef5d93` and GoIvy `ef53df6a4`. The trainer ran ordinary Raft and Apple `cmpl_live` three times apiece per backend, verified each transcript, and merged six CPU profiles with `go tool pprof -proto`. Each held-out workload then ran four counterbalanced rounds on a pinned core. The ordinary comparator used `-pgo=off`. To reproduce the profile-guided build, pass the absolute path of the corresponding saved profile to `go build -pgo=...` in GoIvy, with `-tags=xtrace_off` for g3 or `-tags=classic_z3,xtrace_off` for linked Z3. The trainer also builds both checkers from freshly collected profiles. [Training and build driver](perf/ivy_apple/compiler_profiles/train_checker.py) · [g3 profile](perf/ivy_apple/compiler_profiles/nov22-g3.pprof) · [Linked-Z3 frontend profile](perf/ivy_apple/compiler_profiles/nov22-native.pprof)

### Charging private search by the number of selected premises

Across nine Apple isolates and five independent specifications the short-premise path ran 53 times and proved 37 contradictions. The successful Apple attempts took at most 140 child resource steps per selected premise. In toy consensus all seven unsuccessful attempts exceeded 192 steps per premise and all seven successful ones used at most 112. The production allowance now scales at 192 steps per selected premise, bounded by the existing 80,000-step ceiling and the caller's limit. A 128-step trial missed the captured proof for `rfn2` check 26. Exhausting the allowance falls back to the ordinary solver. [Every measured attempt](perf/ivy_apple/goal_instances/short_premises/nov22-child-effort.csv) · [Resource limit](api/api_order_abstraction.go)

Seven pairs on complete toy consensus lowered median instructions from 11.60 to 11.16 billion. Ticket was nearly flat at 7.19 and 7.17 billion. In four pairs per Apple isolate, the corresponding figures were 45.27 and 45.29 on `rfn2`, 106.39 and 106.27 on `cf_live`, and 121.17 and 121.45 on `cmpl_live`. An independent replay reproduced the same 1,421 private proofs and all 18 linked-Z3 search counters at each of the 10,342 Apple checks. [Independent specifications](perf/ivy_apple/goal_instances/short_premises/nov22-scaled-holdouts.csv) · [Apple checkers](perf/ivy_apple/goal_instances/short_premises/nov22-scaled-checkers.csv)

### Why the first assertion costs more on a fresh strategic solver

A first assertion initializes the strategic solver's entire tactic tree. A second assertion on the same solver skips this construction. The g3 parameter getters were repeatedly interning option names that could not occur in the short local or fallback option lists. The production getter now rules out definitely absent names with a 64-bit filter attached to each parameter list. A possible hit still uses the existing lookup and kind rules. Validation can rename a key, so it must add the normalized spelling to the filter. A preliminary variant missed that case and was discarded. Numeric and null keys use the general path. Declaration-generated description groups can also be recognized when a tactic has already collected them into the same destination. Erasing a description invalidates that destination's group cache. [Getter and key updates](util/params/util_params_ref.go) · [Description groups](util/params/util_param_descrs_group.go)

| Million instructions per additional fresh solver | Previous g3 | Name lookup | Name lookup and catalogs | Linked Z3 |
|---|---:|---:|---:|---:|
| Default | 108.84 | 73.03 | 72.49 | 27.96 |
| GoIvy macro finder | 125.08 | 89.58 | 77.98 | 29.90 |

Each observation differences independent three- and thirteen-operation processes. Seven rounds per configuration measure creating, asserting into and releasing a fresh solver. The repeated lifetimes share a context. In five rounds per complete held-out checker, toy consensus fell from 11.54 to 8.99 billion instructions, ping-pong from 1.77 to 1.15 billion and ticket from 7.14 to 6.23 billion. All seven independent examples improved. Four rounds per Apple workload showed a smaller effect on the longer runs. The nine-isolate concrete replay retained all 1,421 private proofs and matched 18 native counters per check across 10,342 checks. [Startup observations](perf/ivy_apple/holdouts/nov22-parameters-startup.csv) · [Independent checkers](perf/ivy_apple/holdouts/nov22-parameters-checkers.csv) · [Apple checkers](perf/ivy_apple/holdouts/nov22-parameters-apple.csv) · [Alternative lookup strategies](perf/ivy_apple/holdouts/nov22-parameters-alternatives.csv.gz) · [Alternative startup measurements](perf/ivy_apple/holdouts/nov22-parameters-alternative-startup.csv)

Isolating the tactic constructors explains much of the remaining fixed cost. With default options the complete strategic constructor cost 70.85 million g3 instructions and 27.33 million native instructions. Its QF_FP branch alone cost 13.54 versus 2.45 million. QF_FPLRA cost 13.75 versus 3.12 million. The two branches together account for roughly half the remaining excess even when the eventual assertion is a Boolean constant. The phase driver builds the real constructors and subtracts the corresponding repeated setup. [Reproduction driver](perf/ivy_apple/holdouts/startup_phases.py) · [360 phase observations](perf/ivy_apple/holdouts/nov22-constructor-phases.csv.gz)

### Scope storage, fingerprint chains and the choice of solver

Keeping the capacity of all Boolean columns resident, with one common growth decision, cut one synthetic scoped pop from 200,434 to 198,285 solver-thread instructions. Its clearest saving was in undo, from 56,854 to 54,584. Four rounds per Apple workload showed little change across six variants. Separate experiments with a direct saved-literal binding check and specialized heap insertion and tail removal also gave mixed full-pop results. The original 16-byte Boolean record and 8-byte justification remain the production layout. [Column measurements](perf/scope_restoration/boolean_lifetime/nov22-boolean-capacity-phases.csv.gz) · [Column checker comparisons](perf/scope_restoration/boolean_lifetime/nov22-boolean-capacity-checkers.csv.gz) · [Binding and heap measurements](perf/scope_restoration/boolean_lifetime/nov22-dense-scope-ops-phases.csv.gz) · [Binding and heap checker comparisons](perf/scope_restoration/boolean_lifetime/nov22-dense-scope-ops-checkers.csv.gz)

The existing chained quantifier fingerprint table already finds a hit in 1.22 entries on average on both Apple liveness workloads and 1.33 on rfn2. A miss visits 0.57–0.72 entries. Seven experiments covered moving a hit to the front, alternative key mixing, common arities and skipping an identical second probe. Across three rounds per isolate the alternatives all stayed within half a billion instructions of the original on these 45–121 billion instruction runs. Remembering the first reusable slot during AST lookup also gave mixed results. A separate prototype that reused the last insertion's hash during deletion failed the wrong-hash deletion control. [Observed fingerprint probes](perf/ivy_apple/fingerprints/nov22-fingerprint-probes.csv) · [Fingerprint strategies](perf/ivy_apple/fingerprints/nov22-fingerprint-native-keys-checkers.csv.gz) · [AST table comparisons](perf/ivy_apple/application_reuse/nov22-table-holes-checkers.csv.gz)

Using the simple solver for every GoIvy obligation is a useful estimate of what avoiding strategy construction could save. In three complete pairs it reduced ping-pong from 1.78 to 0.20 billion instructions and toy consensus from 11.59 to 3.99 billion. Linked Z3 took 0.52 and 4.13 billion respectively. But it also increased Apple rfn2 from 45.21 to 95.70 billion and rfn from 49.23 to 112.36 billion. A global solver switch would regress both. These results favor reducing construction cost while preserving the strategy and incremental behavior selected for each check. [All 81 complete runs](perf/ivy_apple/holdouts/nov22-simple-ceiling-checkers.csv.gz)

### Matching the native tactic constructors

In this fork the QF_NRA and QF_NIA factories return plain sequences. The Go port wrapped each sequence in a parameter-setting tactic and then pushed the options through every child again. The floating-point route also wrapped its entire sequence. Its first simplifier should receive its two local options at construction, while two later simplifiers explicitly retain them. The QF_FPLRA wrapper reapplies the caller options once, as in C++. The production constructors now follow those lifetimes and keep the same tactic alternatives. Reusing the public default-tactic factory for the strategic solver was also measured. It increased the instruction counts on all four independent examples compared with correcting the individual constructors. [Native QF_NRA](../src/tactic/smtlogics/qfnra_tactic.cpp) · [Native QF_NIA](../src/tactic/smtlogics/qfnia_tactic.cpp) · [Native QF_FP](../src/tactic/fpa/qffp_tactic.cpp) · [Native QF_FPLRA](../src/tactic/fpa/qffplra_tactic.cpp)

With name lookup and catalog reuse already enabled, seven rounds put fresh-solver startup at **72.79 → 57.89 million instructions** with default options and **77.89 → 63.01 million** with macro finder. The default fixture fell from 18,742 to 16,391 Go allocations per solver. In five rounds per whole checker toy consensus fell from 8.58 to 7.86 billion instructions, ping-pong from 1.16 to 0.97 billion and ticket from 6.22 to 5.96 billion. Four rounds on each of three Apple isolates retained complete output. The nine-isolate concrete replay reproduced the same private-proof indicators and all 18 linked-Z3 search counters. The portable shell control covers 92 known answers across the four affected logics, default and explicitly named tactics, and both macro-finder settings. [Startup slopes](perf/ivy_apple/holdouts/nov22-tactic-release-startup.csv) · [Seven checker comparisons](perf/ivy_apple/holdouts/nov22-tactic-release-checkers.csv.gz) · [Apple comparisons](perf/ivy_apple/holdouts/nov22-tactic-release-apple.csv) · [Shell control](perf/ivy_apple/holdouts/portfolio_controls.py) · [Shell outcomes](perf/ivy_apple/holdouts/nov22-tactic-controls.csv)

The isolated route experiments put most of the saving in QF_NRA. That constructor appears both directly and inside the floating-point branches. The cumulative time of nested constructors cannot be added to the time of their parents. [Per-route startup experiments](perf/ivy_apple/holdouts/nov22-tactic-alternatives-startup.csv) · [Whole independent checkers](perf/ivy_apple/holdouts/nov22-tactic-alternatives-checkers.csv.gz) · [Whole Apple checkers](perf/ivy_apple/holdouts/nov22-tactic-alternatives-apple.csv)

### Reading small option lists once per rewriter update

The Boolean, bit-vector and array rewriters read several options from the same short pair of parameter lists. They now copy the names and scalar values once per synchronous update. Local values precede module defaults, and the kind is part of each lookup. An oversized list, numeric or null key, or unreached owner error takes the ordinary getter path. No snapshot survives the update. [Scalar option lookup](util/params/util_params_scalars.go) · [Fallback, kind, mutation and capacity tests](util/params/util_params_scalars_test.go)

Five rounds on a fresh default solver gave **57.85 → 54.25 million instructions** at 16,391 allocations per operation. With GoIvy's macro finder the change was **63.22 → 60.08 million** at 16,444 allocations. In independent whole processes toy consensus used 7.89 → 7.70 billion instructions, ping-pong 0.97 → 0.93 and ticket 5.95 → 5.89. Caching the options on additional arithmetic and theory parameter wrappers reduced toy consensus as far as 7.56 billion, but allocated an extra 557 KB per fresh solver. A pointer-based version added 803 KB and 1,930 allocations on the default fixture. The production version uses the three local readers and adds no measured allocations. [Final layout trials](perf/ivy_apple/holdouts/nov22-local-readers-startup.csv) · [Whole checkers](perf/ivy_apple/holdouts/nov22-local-readers-checkers.csv.gz) · [Initial size and empty-list experiments](perf/ivy_apple/holdouts/nov22-stacked-options-startup.csv) · [Initial checker comparisons](perf/ivy_apple/holdouts/nov22-stacked-options-checkers.csv.gz)

### Memory for 1,024 independent strategic solvers

Each measured worker owns a separate API context, AST arena and default solver. The benchmark asserts true into each solver and keeps every worker alive until the memory reading. Constructing them sequentially on one core isolates retained state and startup cost. Peak RSS measures the whole process, including initialization and cleanup. It is the comparable memory metric across the two runtimes. The Go live heap after collection and Z3's allocator estimate use different accounting. [Go worker benchmark](api/api_solver_workers_benchmark_test.go) · [Native worker benchmark](../src/test/perf/strategic_solver_workers.cpp) · [Reproduction driver](perf/ivy_apple/holdouts/strategic_workers.py)

| Live workers | g3 peak RSS | Native peak RSS | g3 live heap per worker | Native estimated allocation per worker |
|---:|---:|---:|---:|---:|
| 16 | 76.8 MiB | 53.3 MiB | 2,398,544 B | 2,740,998 B |
| 128 | 378.2 MiB | 334.6 MiB | 2,398,528 B | 2,737,524 B |
| 1,024 | 2.83 GiB | 2.52 GiB | 2,398,523 B | 2,737,090 B |

For 1,024 workers the three-round median time to initialize the cohort was 11.17 s for g3 and 4.02 s for native. This is the fully initialized strategic API. The smaller SMT-core worker benchmark measures a different population of owners and remains useful for tracking scope storage. [24 final observations](perf/ivy_apple/holdouts/nov22-strategic-workers.csv) · [Earlier constructor configurations](perf/ivy_apple/holdouts/nov22-strategic-worker-constructors.csv) · [SMT-core worker benchmark](smtcore/smt_scope_workers_benchmark_test.go)

A separate live heap profile of 128 g3 workers attributed 8.05% of sampled live bytes directly to parameter-entry pages, about 7% to the bit-vector rewriter constructor, 5.51% to traversal construction, 4.65% to parameter-block allocation and 4.62% to numeral cell slabs. To reproduce the profile, select only Workers128, pass `-benchtime=1x -memprofilerate=4096` to `go test`, and use `-args -g3-strategic-worker-heap=/tmp/workers.pprof`. The profile is taken before closing any worker. [Saved Go heap profile](perf/ivy_apple/holdouts/nov22-workers-128.pprof)

### Target obligations and first pushes at the November 22 option-reader revision

Across five paired runs of all 26 rfn2 conditions, g3 used **3.01 billion solver-thread instructions inside verification calls, against 35.48 billion for linked Z3**. On the November 21 frontier the same comparison was 93.08 against 35.48. Check 4 now takes 0.160 billion versus 20.858 billion. The independent proof changes which algorithm g3 executes. A separate replay also runs the concrete solver and compares its 18 search counters to native at every check. [Per-call measurements](perf/ivy_apple/incremental_cost/nov22-completion-checks.csv.gz) · [Earlier measurements](#november-21-release-comparison)

| Selected obligation, billion instructions | g3 | Linked Z3 |
|---|---:|---:|
| rfn2 4 | 0.160 | 20.858 |
| rfn2 13 | 0.134 | 4.388 |
| rfn2 22 | 0.178 | 5.213 |
| rfn2 26 | 0.096 | 0.978 |
| cf_live 122 | 1.950 | 0.547 |
| cf_live 50 | 1.495 | 0.539 |
| cmpl_live 177 | 3.803 | 1.403 |

Three pairs captured every verification and mutation on cf_live and cmpl_live. Inside checks, cf_live used 22.85 versus 34.00 billion instructions and cmpl_live used 6.64 versus 4.29 billion. The 2.40 billion excess on cmpl_live 177 is slightly larger than the entire isolate's net excess inside checks. Faster conditions offset part of it. Across the three isolates, 4,898 check intervals and 15,798 mutation intervals all ran for the full enabled counter time. [Verification calls](perf/ivy_apple/incremental_cost/nov22-completion-checks.csv.gz) · [Push, pop, assert and reset intervals](perf/ivy_apple/incremental_cost/nov22-completion-mutations.csv.gz)

The cost between rfn2 checks is still concentrated on the first pushes. All 26 external pushes used 12.23 versus 3.53 billion instructions. The ten expensive first pushes account for 99.98% of that difference. Across cf_live, external pushes cost 17.90 versus 5.36 billion, and across cmpl_live they cost 14.78 versus 4.17 billion. Three independent instruction-sampling rounds attribute about 8.02 versus 2.22 billion rfn2 push instructions to work outside assertion reduction and 1.96 versus 0.55 billion to macro finding within reduction. These process samples can include Go GC on another thread. The exact interval figures count only the solver thread. [Pass estimates](perf/ivy_apple/incremental_cost/nov22-completion-push-passes.csv) · [g3 leaf and inclusive stacks](perf/ivy_apple/incremental_cost/nov22-completion-push-g3.csv.gz) · [Linked-Z3 stacks](perf/ivy_apple/incremental_cost/nov22-completion-push-native.csv.gz) · [Push profiler](perf/ivy_apple/incremental_cost/push_profile.py)

### November 22 complete verification comparison

Six paired runs per workload used the same GoIvy revision, local g3 module replacement, input, logical CPU and complete reference transcript. Both frontends used ordinary Go compilation with PGO off. The native frontend linked this repository’s Z3 fork. Each workload ran on a single pinned CPU. Five workloads could run in parallel and the host also ran experiments. The code under test was g3 `4dd108954` and GoIvy `ef53df6a4`. The seconds and instructions below are separate medians for each backend. The final column counts individual wall-time comparisons. [120 complete runs](perf/ivy_apple/holdouts/nov22-release-panel.csv) · [Paired benchmark](perf/ivy_apple/workload_panel.py)

| Complete workload | Checks | g3 seconds | Z3 seconds | g3 billion instructions | Z3 billion instructions | g3 faster pairs |
|---|---:|---:|---:|---:|---:|---:|
| raft | 419 | 3.87 | 39.19 | 26.22 | 215.19 | 6/6 |
| dramc_nb2 | 2,479 | 78.29 | 83.70 | 394.40 | 471.26 | 5/6 |
| rfn2 | 26 | 6.76 | 16.84 | 44.45 | 69.30 | 6/6 |
| cf_live | 336 | 18.48 | 22.43 | 105.32 | 106.45 | 5/6 |
| cmpl_live | 437 | 21.90 | 20.73 | 119.40 | 123.30 | 5/6 |
| rfn | 33 | 8.58 | 15.45 | 48.80 | 57.47 | 6/6 |
| sys_live | 1,738 | 47.06 | 53.08 | 283.78 | 334.62 | 6/6 |
| e2e | 2,378 | 64.14 | 78.04 | 368.38 | 425.91 | 6/6 |
| cf_pio_live | 2,215 | 71.68 | 101.49 | 407.38 | 529.38 | 6/6 |
| tprops | 700 | 27.49 | 27.03 | 158.16 | 166.37 | 4/6 |

g3 retired fewer whole-process instructions on all ten workloads. Raft and rfn2 finished faster in every pair. Their median individual speedups were 10.35× and 2.52×. On dramc_nb2, five of six pairs favored g3, with a median individual speedup of 1.09×. The two closest wall-time comparisons, cmpl_live and tprops, changed ordering depending on whether the comparison used paired ratios or separate medians. The sample does not establish a stable elapsed-time lead on either isolate.

Six more pairs per workload on seven independent short examples confirmed 505 expected passes and three expected failures per complete set. Linked Z3 was faster on all seven separate medians. Toy consensus took 1.638 versus 0.950 s and 7.679 versus 4.133 billion instructions. Ping-pong took 0.204 versus 0.126 s and 0.928 versus 0.516 billion instructions. Creating fresh strategic solvers accounts for much of the short-run penalty. [84 short-example runs](perf/ivy_apple/holdouts/nov22-release-examples.csv) · [Independent example driver](perf/ivy_apple/holdout_examples.py) · [First assertion on a fresh solver](#why-the-first-assertion-costs-more-on-a-fresh-strategic-solver)

An independent concrete replay of all 10,342 Apple conditions agreed with linked Z3 on all 18 counters at every check, including 1,421 conditions for which g3 also found a private refutation. The 28 Raft mutation and width combinations produced all 77 expected failures across 11,732 checks. Both shells returned the same result on 240 finite-model certificates. The private path returned no refutation on any of the 240 explicit countermodels. The four arithmetic and floating-point logics passed all 92 known-answer controls with both shells. The Paxos `log3` replay reproduced all 27,855 normalized output lines from PyIvy, including 27,782 trace records and the expected diagnostic at line 253. [Concrete counters](perf/ivy_apple/application_reuse/nov22-hints-confirmations.csv.gz) · [Raft mutations](perf/ivy_apple/holdouts/nov22-release-raft-mutants.csv) · [Finite-model certificates](perf/ivy_apple/goal_instances/finite_models/nov22-release-certificates.csv) · [Explicit countermodels](perf/ivy_apple/goal_instances/finite_models/nov22-release-explicit-models.csv) · [Tactic controls](perf/ivy_apple/holdouts/nov22-tactic-controls.csv)

### Interned option names and grouped parameter descriptions

Creating a fresh strategic solver, asserting true and closing it with GoIvy's macro-finder option now takes **56.21 million instructions**, down from **60.10 million**. Linked Z3 uses **29.90 million**. With default options the corresponding medians were 55.46, 53.31 and 27.96 million. Each estimate subtracts the instruction count for three operations from the count for thirteen and divides by ten. The comparison alternated the two ordinary Go builds and the linked native implementation on one CPU for nine rounds. The macro-finder fixture still reports 16,444 allocations per fresh Go solver. [54 startup measurements](perf/ivy_apple/holdouts/nov22-spelling-startup.csv) · [Benchmark driver](perf/ivy_apple/holdouts/startup.py)

Short, already-interned option names can go straight to the symbol table. Longer inputs and misses retain the original first-NUL handling. Scalar reads can also compare against an interned spelling directly. Five SMT parameter collectors now insert their existing static descriptions as groups, preserving order, duplicates and the nested context-solver entries. These changes avoid repeated spelling scans and catalog calls while retaining eager construction and solver-local state.

All seven short verification examples used fewer median instructions in six pairs against the previous build. Toy consensus fell from 7.668 to 7.468 billion, ping-pong from 0.925 to 0.877 billion and ticket from 5.849 to 5.799 billion. The larger Apple comparisons showed little movement. Four pairs gave 44.56 to 44.62 billion on rfn2, 105.30 to 105.14 on cf_live and 119.63 to 119.17 on cmpl_live. The independent concrete replay reproduced all 18 native counters on every one of the 10,342 Apple conditions. The touched packages passed their complete ordinary and checked/debug/trace tests. [84 short-example comparisons](perf/ivy_apple/holdouts/nov22-spelling-short.csv.gz) · [Reference counters](perf/ivy_apple/application_reuse/nov22-hints-confirmations.csv.gz)

### Which parts of strategy construction pay for themselves?

In 17 complete GoIvy workloads, the default tactic tree was constructed 520 times. Every tree supplied parameter descriptions exactly once. None received an Apply, UpdateParams, Cleanup or statistics call. Each workload used one AST manager for all its trees. The incremental solver still performed the proofs. This gives a way to measure the cost of eager construction separately from proof search. Multiplying the measured fresh-solver excess at revision `7ed2aec0a` by the observed constructor count would account for about 94% of the whole-program instruction gap on ping-pong (13 constructors), 41% on toy consensus (52) and 18% on ticket (19). These are estimates from an isolated fixture. The proportions vary substantially even among short examples. [Per-tree calls and manager identities](perf/ivy_apple/holdouts/nov22-strategy-usage.csv.gz) · [Observer](perf/ivy_apple/holdouts/strategy_usage.py) · [Startup measurements](perf/ivy_apple/holdouts/nov22-spelling-startup.csv)

Removing an apparently unused branch can change the proof search. The default factory starts with 137 live arena records in a fresh API context. Constructing its linear integer/real arithmetic tactic adds 18 live records and changes five earlier records. The final bit-vector tactic adds two live records and changes two earlier records. Removing the bit-vector branch preserved all 262 parameter rows by name, but only 57 stayed at the same position. Nine of the 26 rfn2 checks then had different concrete search counters, with 129 counter values changed. Retaining just the two AST-producing branches and the ordinary SMT branch matched all 18 counters on all 10,342 Apple checks and reduced fresh macro-finder startup from 59.94 to 13.04 million instructions. That is a diagnostic lower bound because it omits reachable tactics and public descriptions. [Arena observations](perf/ivy_apple/holdouts/nov22-strategy-arena-construction.csv) · [Catalog order and contents](perf/ivy_apple/holdouts/nov22-strategy-catalog-summary.csv) · [Per-isolate counter differences](perf/ivy_apple/holdouts/nov22-deferred-constructor-counters.csv) · [Branch comparison](perf/ivy_apple/holdouts/nov22-strategy-owners-startup.csv.gz) · [Reproduction driver](perf/ivy_apple/holdouts/strategy_construction.py)

Instruction sampling of fresh macro-finder solvers on revision `4dd108954` identified nested constructors. The QF_NRA/NLSAT constructor appeared on 34.6% of g3's 12,177 samples and 25.5% of the 5,985 linked Z3 samples. g3's theory rewriter constructor appeared on 31.0% and Go's allocator on 19.5%. The native theory rewriter constructor appeared on 22.1%. These inclusive shares overlap. The profiles cover three pinned rounds per engine with a period of 1,200,011 retired instructions and zero reported lost samples. [Complete stack counts](perf/ivy_apple/holdouts/nov22-startup-stacks.csv.gz) · [Sampling driver](perf/ivy_apple/holdouts/startup_stacks.py)

A prototype kept each floating-point branch's parameters and constructed its real tactic on first use. Deferring one branch reduced macro-finder startup from 59.93 to 51.34 million instructions. Deferring both reached 39.01 million and reduced allocations from 16,444 to 11,576. With 128 independent solvers the Go heap fell from 2,398,527 to 1,952,031 bytes per initialized solver. Toy consensus fell from 7.68 to 6.72 billion instructions and ping-pong from 0.92 to 0.68 billion. On cf_live the three layouts were within 0.11 billion instructions of each other. The two-branch variant preserved every ordered public description and every Apple counter. Across native Z3, eager g3 and the two prototypes, all 552 arithmetic and floating-point controls returned the expected verdict, including 32 actual applications of deferred branches. [Startup](perf/ivy_apple/holdouts/nov22-deferred-float-startup.csv.gz) · [Worker memory](perf/ivy_apple/holdouts/nov22-deferred-float-workers.csv.gz) · [Short examples](perf/ivy_apple/holdouts/nov22-deferred-float-short.csv.gz) · [Apple runs](perf/ivy_apple/holdouts/nov22-deferred-float-apple.csv.gz) · [Known answers](perf/ivy_apple/holdouts/nov22-deferred-float-controls.csv.gz) · [Experimental patch](perf/ivy_apple/holdouts/deferred-float-experiment.patch)

**A public resource limit distinguishes the prototype from the ordinary solver.** With an arena budget of 1,664 or 1,792 KiB, the eager solver reported exhaustion when the caller requested parameter descriptions. The two-branch prototype returned the descriptions, accepted the assertion and then reported exhaustion at Check. At 2,048 KiB, both deferred variants changed that failure boundary. At 2,560 KiB all three successfully solved the floating-point formula. Five changes to global options between construction and Check at 32 MiB yielded the same verdicts and pre-check statistic keys. The resource result rules out silently enabling this prototype on every solver. A viable design would specify delayed initialization explicitly or preserve the original reservation and option semantics while deferring expensive work. [495 observed API stages](perf/ivy_apple/holdouts/nov22-deferred-float-contracts.csv.gz)

### Allocation, worker memory and repeated arena reads

Placing nine theory-rewriter children and their traversal together in typed Go storage eliminated 2,300 allocations per fresh macro-finder solver, from 16,444 to 14,144. Seven pinned rounds put startup at 59.74 million instructions before and 59.77 million after. Per-operation allocation increased from 2,640,654 to 2,673,355 bytes. With 128 initialized workers, live Go heap rose from 2,398,528 to 2,429,760 bytes per worker. Ping-pong stayed at 0.92 billion instructions. In the same comparison, reading small arithmetic and common option lists directly reduced startup to 56.01 million instructions with the original allocation count. Repeated constructor work is the better target here. [Startup comparisons](perf/ivy_apple/holdouts/nov22-rewriter-allocation-startup.csv.gz) · [Worker memory](perf/ivy_apple/holdouts/nov22-rewriter-allocation-workers.csv.gz) · [Short examples](perf/ivy_apple/holdouts/nov22-rewriter-allocation-short.csv.gz) · [Apple checkers](perf/ivy_apple/holdouts/nov22-rewriter-allocation-apple.csv.gz)

Parameter pages of 16, 64 and 128 entries used 2,389,935, 2,398,527 and 2,422,815 live heap bytes per worker in 128-worker cohorts. Their median macro-finder startup costs were 60.68, 59.64 and 59.49 million instructions. The 64-entry page remains a reasonable choice for this workload. Carrying a direct arena record pointer on the general rewriter's traversal frame also increased work. On rfn2 the original, carried-pointer and guarded-pointer variants retired 44.61, 44.81 and 45.03 billion instructions. On cf_live the same three used 105.30, 106.02 and 106.18 billion. These experiments retain the AST arena. [Page sizes](perf/ivy_apple/holdouts/nov22-parameter-pages-startup.csv) · [Page memory](perf/ivy_apple/holdouts/nov22-parameter-pages-workers.csv) · [Five traversal variants](perf/ivy_apple/application_reuse/nov22-borrowed-record-checkers.csv.gz)

### Reading arithmetic and traversal options on a fresh solver

Arithmetic simplification and the common theory rewriter now read a small parameter list once per update. The ordinary getters and the batch reader share the same typed declarations for each option's name and default. Larger lists, unusual keys and invalid parameter references use the existing getters. Both module snapshots retain their original lifetimes. The extra state is local to the update. [Arithmetic update](ast/rewriter/arith_rewriter.go) · [Theory rewriter update](ast/rewriter/th_rewriter.go) · [Small and large list equivalence](ast/rewriter/scalar_update_test.go)

Relative to `7ed2aec0a`, nine rotated rounds reduced fresh macro-finder startup from **56.09 to 52.07 million instructions**, a 7.2% reduction. The default solver fell from **53.28 to 48.52 million**, an 8.9% reduction. Linked Z3 measured 29.90 and 27.96 million respectively. The macro-finder build still allocated 16,444 times per operation. In six alternating pairs on each short example, all seven improved. Toy consensus went from 7.411 to 7.142 billion instructions, ticket from 5.815 to 5.700 billion and ping-pong from 0.875 to 0.820 billion. [54 startup measurements](perf/ivy_apple/holdouts/nov22-rewriter-settings-startup.csv) · [84 whole-example runs](perf/ivy_apple/holdouts/nov22-rewriter-settings-short.csv.gz)

The independent Apple replay matched the original concrete solver's 18 counters on all 10,342 checks and retained all 1,421 private refutations. All 322 API-stage observations across the previous and new builds matched exactly, including the first error, arena usage, returned statistic names and verdict under 20 budgets and five intervening global-option changes. All 240 explicit countermodels survived. The 28 Raft mutation runs, the 92 arithmetic and floating-point known answers per engine and the Paxos log3 transcript also matched their controls. [Budget and option observations](perf/ivy_apple/holdouts/nov22-rewriter-settings-contracts.csv.gz) · [Concrete reference counters](perf/ivy_apple/application_reuse/nov22-hints-confirmations.csv.gz)

The preceding spelling and catalog change also had its own six-pair native comparison. Its three original workloads took 4.26 versus 50.73 seconds on Raft, 94.75 versus 133.87 on dramc_nb2, and 7.56 versus 20.50 on rfn2. Each pair compared identical proof transcripts. The earlier revision's serial and matched-PGO runs provide separate controls on compiler and machine-load effects. [120 ordinary checker runs](perf/ivy_apple/holdouts/nov22-spelling-panel.csv) · [70 short-spec runs](perf/ivy_apple/holdouts/nov22-spelling-examples.csv) · [Earlier ordinary serial comparison](perf/ivy_apple/holdouts/nov22-release-panel-serial.csv) · [Earlier matched-PGO panel](perf/ivy_apple/holdouts/nov22-release-panel-pgo.csv)

The unmodified Makefile targets remain a distinct measurement. In five alternating host-default rounds of 10,000 iterations, surviving atoms took **857 versus 662 ns** and scoped atoms took **18.31 versus 16.02 µs** for g3 and Z3. With both processes pinned to CPU 7 on the same shared host, the medians were 1,124 versus 1,440 ns and 34.02 versus 33.27 µs. The Go fixture uses its ordinary queue configuration. These elapsed-time comparisons vary enough with placement to warrant keeping both the original target and the separate matched-queue solver-thread measurements. [40 Makefile observations](perf/scope_restoration/nov22-make-targets.csv) · [Matched-queue benchmark](smtcore/smt_hotpath_benchmark_test.go) · [Per-variable phase counts](perf/scope_restoration/boolean_lifetime/nov22-variable-phases.csv)

### Ordinary GoIvy verification at 91d1597f3

The release includes the arithmetic and theory rewriter option change. Four alternating pairs of each original workload ran through the same GoIvy frontend on logical CPU 22, with the ordinary compiler configuration, one Go runtime processor and default GC settings. All 12 pairs favored g3 and produced identical proof transcripts. The table reports the median elapsed time for each engine and their ratio. The medians of the *individual paired* speedups were 10.58×, 1.24× and 2.62× respectively. Individual pairs ranged from 9.68–12.90× on Raft, 1.19–1.33× on dramc_nb2 and 2.34–3.20× on rfn2. Other activity on this host varied during the runs. [All 24 runs, counters and RSS](perf/ivy_apple/holdouts/nov22-rewriter-settings-serial.csv)

| Workload | Named checks | g3 | Linked Z3 | Ratio of medians, Z3/g3 | Instructions, g3 / Z3 (billions) |
|---|---:|---:|---:|---:|---:|
| Raft | 419 | 4.06 s | 46.73 s | 11.50× | 25.94 / 215.20 |
| Apple dramc_nb2 | 2,479 | 83.70 s | 100.72 s | 1.20× | 393.99 / 477.06 |
| Apple rfn2 | 26 | 9.87 s | 26.57 s | 2.69× | 44.37 / 69.32 |

A second comparison ran ten workload lanes concurrently, assigning a fixed CPU to both engines in each lane. All ten separate elapsed-time and instruction medians favored g3. On tprops, elapsed time differed by only 0.40 s across five pairs, with g3 winning three. These runs are more informative about whole-process instructions than about absolute elapsed times. The rfn2 lane took 32.70/83.15 s here, versus 9.87/26.57 s running the three workloads in series. Its instruction totals agreed to within 0.2% per engine between the two experiments. [All 100 measurements](perf/ivy_apple/holdouts/nov22-rewriter-settings-panel.csv) · [Direct paired Go-build comparisons](perf/ivy_apple/holdouts/nov22-rewriter-settings-apple.csv)

| Workload | g3 seconds | Z3 seconds | g3 / Z3 billion instructions | g3 elapsed-time wins |
|---|---:|---:|---:|---:|
| Raft | 4.26 | 44.68 | 25.94 / 215.20 | 5/5 |
| rfn2 | 32.70 | 83.15 | 44.31 / 69.44 | 5/5 |
| dramc_nb2 | 76.52 | 145.57 | 394.04 / 478.67 | 5/5 |
| cf_live | 19.89 | 25.28 | 105.10 / 106.99 | 4/5 |
| cf_pio_live | 101.69 | 126.87 | 406.49 / 535.58 | 5/5 |
| cmpl_live | 20.76 | 25.06 | 119.06 / 124.08 | 4/5 |
| e2e | 87.69 | 96.74 | 368.18 / 430.63 | 4/5 |
| rfn | 7.84 | 14.14 | 48.52 / 57.37 | 5/5 |
| sys_live | 57.26 | 84.89 | 283.75 / 336.75 | 3/5 |
| tprops | 32.93 | 33.33 | 157.66 / 167.71 | 3/5 |

The same ten-workload panel also ran with a separate Go compiler profile for each backend. At `91d1597f3`, three pairs on each workload put Raft at 4.42/46.99 s, dramc_nb2 at 79.22/86.13 s and rfn2 at 7.45/16.75 s. All ten g3 instruction medians were lower. Linked Z3 won the tprops elapsed-time median, 30.30 s against 33.34 s for g3. An earlier four-pair run with matching profiles at `7ed2aec0a` provides a second compiler control. [60 current PGO runs](perf/ivy_apple/holdouts/nov22-rewriter-settings-matched-pgo.csv) · [80 earlier PGO runs](perf/ivy_apple/holdouts/nov22-spelling-matched-pgo.csv) · [Profile sources](perf/ivy_apple/compiler_profiles)

**rfn2 check 4 now takes 0.181 billion solver-thread instructions in g3 and 20.858 billion in linked Z3.** A private proof discharges the obligation. Across all 26 checks the totals were 3.162 and 35.480 billion. The first pushes still cost more in Go. Its ten expensive pushes used 12.145 billion instructions against 3.530 billion for the same native calls. Across *all* pushes the figures were 12.147 and 3.531 billion. These are medians of per-round sums over five rounds, after subtracting the empty counter read at each call. On cf_live, the three-round figures were 22.89/34.00 billion inside Check and 17.89/5.36 billion inside Push. On cmpl_live they were 6.54/4.29 billion and 14.74/4.17 billion respectively. The measured intervals run on the solver's OS thread. [Individual checks](perf/ivy_apple/incremental_cost/nov22-current-checks.csv.gz) · [Every push, pop, assertion and reset](perf/ivy_apple/incremental_cost/nov22-current-mutations.csv.gz)

### Three reproducible obligations that still favor Z3

Seven independent short specifications produced 505 expected passes and three expected failures with both backends. Linked Z3 won every elapsed-time pair, six per example. The complete g3/Z3 runs retired 7.216/4.132 billion instructions on toy consensus, 5.716/3.017 billion on ticket, and 0.818/0.515 billion on ping-pong. Five additional pairs measured each API boundary separately, including 73 actual Check calls on the 72 named toy-consensus passes. Each instrumented run had to reproduce its ordinary transcript. [84 complete short runs](perf/ivy_apple/holdouts/nov22-rewriter-settings-holdouts.csv) · [23,790 solver-thread intervals](perf/ivy_apple/holdouts/nov22-short-intervals.csv.gz) · [Interval driver](perf/ivy_apple/holdouts/short_intervals.py)

| Median solver-thread instructions (billions), g3 / Z3 | Check | Assert | Push |
|---|---:|---:|---:|
| Toy consensus | 2.000 / 0.412 | 2.258 / 1.372 | 0.700 / 0.169 |
| Ticket | 2.363 / 0.594 | 0.870 / 0.514 | 0.706 / 0.164 |
| Ping-pong | 0.0033 / 0.0006 | 0.581 / 0.346 | 0.041 / 0.011 |

Toy-consensus check 18 accounts for **0.779 billion excess instructions**, 49% of the total Check gap. Ticket checks 127 and 290 contribute 0.197 and 0.212 billion. Ping-pong spends most of its excess on Assert. This separates two targets that a single short runtime can obscure. Improving even every proof search on ping-pong could recover little, while eliminating all of its startup penalty would leave most of the measured toy-consensus check 18 gap.

The three captured inputs replay in either shell with the same macro-finder setting. The incremental replay adds a push before the final assertion. Every engine produced the same answer and exactly the same decisions, quantifier instantiations, conflicts and resource counts for each route, in seven pairs. The figures include shell startup, parsing and assertion. A captured formula in a new process has a different history from the corresponding check in the live GoIvy solver. [Replay driver](perf/ivy_apple/holdouts/query_replay.py) · [84 replays](perf/ivy_apple/holdouts/nov22-short-query-replays.csv) · [Captured SMT inputs](perf/ivy_apple/test_vectors)

| Obligation | Fresh g3 / Z3 (M instructions) | Incremental g3 / Z3 (M instructions) | Incremental decisions / instantiations / conflicts |
|---|---:|---:|---:|
| Toy consensus 18 | 1,559 / 461 | 725 / 224 | 1,866 / 905 / 41 |
| Ticket 127 | 328 / 133 | 310 / 119 | 708 / 1,389 / 50 |
| Ticket 290 | 483 / 168 | 326 / 120 | 310 / 1,241 / 37 |

Five more incremental pairs on CPU 5 reproduced all four search counters on all three queries. Median whole-shell instructions in millions were 737.7/223.8 on toy consensus, 310.1/119.0 on ticket 127 and 326.3/120.2 on ticket 290. This second placement reproduced the large per-search gap. [Second-CPU confirmation](perf/ivy_apple/holdouts/nov22-short-query-second-cpu.csv)

Six alternating pairs of instruction profiles per incremental query sampled every 710,009 user instructions. The nearest named solver operation received each stack's vote once. Relative to Z3, **the general rewriter accounted for approximately 239, 76 and 88 million excess instructions** on the three rows above. The quantifier queue contributed approximately 40, 36 and 37 million. Model checking added about 71 million on toy consensus. No lost samples were reported. These estimates identify work per search step in a case where the search counters agree. [Sampling driver](perf/ivy_apple/holdouts/query_stacks.py) · [Per-run symbol and operation counts](perf/ivy_apple/holdouts/nov22-short-query-stacks.csv.gz)

On toy consensus, g3's theory `ReduceApp` was the first callee below a rewriter frame in 1,016 samples across six runs. The corresponding native `reduce_app` had 115. g3's physical leaves in the nearest-rewriter sample group included `processApp` (109 samples), `GetArg` (76) and `declIdentity` (53). In C++, `reduce_app_core` dispatches from a declaration and reads operands through a pointer array. Go's implementation crosses manager accessors to read declaration identity, sort family and the operands of equality. These measurements favor avoiding repeated decoding within one reduction. They do not identify any one accessor as the entire gap. [Rewriter callees and leaves](perf/ivy_apple/holdouts/nov22-short-query-rewriter.csv.gz) · [Go dispatch](ast/rewriter/th_rewriter.go) · [C++ dispatch](../src/ast/rewriter/th_rewriter.cpp)

### The effect of Go heap growth on dramc_nb2

Both backends use the GoIvy frontend. A three-round experiment at `7ed2aec0a` measured each process's peak RSS and Go runtime GC counters along with elapsed time and retired instructions. Changing GOGC affects frontend allocation and collection under both backends. All proof transcripts matched the default run. The fall in whole-process instructions with more heap is especially large for the linked-Z3 process. Process peak RSS includes native allocations. [36 complete runs](perf/ivy_apple/holdouts/nov22-runtime-gc-budget.csv) · [Runtime metrics and paired driver](perf/ivy_apple/frontend_memory.py)

| dramc_nb2 GOGC | g3 / Z3 seconds | g3 / Z3 billion instructions | g3 / Z3 peak MiB | g3 / Z3 GC CPU seconds |
|---|---:|---:|---:|---:|
| 100 (default) | 76.44 / 119.36 | 394.29 / 478.58 | 221 / 208 | 14.93 / 38.27 |
| 200 | 85.72 / 85.83 | 332.45 / 341.89 | 320 / 263 | 8.57 / 14.04 |
| 400 | 57.41 / 51.19 | 298.81 / 281.15 | 593 / 449 | 3.14 / 4.21 |

At GOGC 400, **linked Z3 completed dramc_nb2 faster while using less memory**. For rfn2 the larger heap still favored g3 at 35.36 versus 59.90 billion instructions, with median wall times of 7.91 versus 24.13 s. Concurrent host load affected elapsed times, including at the intermediate GC setting. Per-run counters and peak memory make the runtime tradeoff visible. GOGC controls the process as a whole, so setting it inside an individual solver would also change other workers.

Four further pairs per setting targeted comparable resident memory by choosing the GOGC settings in advance. The 300 MiB target produced median peaks of **279/289 MiB**, with 340.54/320.97 billion instructions and 128.77/117.58 s for g3/Z3. The 450 MiB target produced **461/475 MiB**, 310.67/281.20 billion instructions and 56.49/53.24 s. The GOGC pairs were 175/240 and 295/400 respectively. Peak RSS is an observation, so neither target is an enforced quota. The two measured memory bands both favored linked Z3 on this isolate. Separate rfn2 runs exercised the same paired driver with the current g3 build. [16 dramc_nb2 runs](perf/ivy_apple/holdouts/nov22-runtime-matched-rss.csv) · [Current rfn2 control](perf/ivy_apple/holdouts/nov22-runtime-paired-rfn2.csv)

A fresh confirmation with the current `91d1597f3` solver used three CPU-pinned pairs per memory band. At the lower band wall time was effectively tied, **64.72/65.18 s** for g3/Z3, with 279/299 MiB peak RSS and 339.61/319.75 billion instructions. At the upper band, g3 took **57.98 s versus 53.86 s for Z3**, using 467/477 MiB and 310.59/281.76 billion instructions. The same frontend and identical proofs show a repeatable instruction advantage for linked Z3 at both memory bands. Shared-host elapsed-time rankings at the lower band changed between the two build comparisons. [12 current matched-memory observations](perf/ivy_apple/holdouts/nov22-current-matched-rss.csv)

### How much could a longer refutation history save?

The current incremental proof path remembers eight recent goals under matching premises. A diagnostic observer compared earlier confirmed refutations with each later assertion set, within each solver batch. It tested inclusion using both whole assertions and their conjunctions flattened into terms, with history windows of 1, 8, 32 and unbounded. Across 17 specifications only dramc_nb2 offered extra matches. A 32-entry implementation found the same 36 extra private refutations, increasing the dramc_nb2 count from 326 to 362. Independent concrete replay still agreed on all 18 counters at every one of the 10,342 Apple checks. [Per-workload candidate counts](perf/ivy_apple/incremental_cost/nov22-premise-history.csv)

The 36 newly covered calls *together* cost a median of 996,619 solver-thread instructions with the existing eight entries. The wider history reduced that to 122,158 over three rounds, against about 394 billion whole-process instructions. Four paired complete runs measured 394.27 and 393.83 billion with the original and wider histories. The measured saving inside the 36 calls explains less than one million instructions. The ordinary solver keeps eight entries. [All 2,479 calls in both builds](perf/ivy_apple/incremental_cost/nov22-history-depth-checks.csv.gz) · [Whole-checker comparisons](perf/ivy_apple/incremental_cost/nov22-recent-history-checkers.csv) · [32-entry experimental patch](perf/ivy_apple/incremental_cost/history-32-experiment.patch)

Five small changes to scoped Boolean retirement also gave little return. The variants skipped empty watch or justification writes, shared the watch-pair lookup, skipped a vacuous quantifier lookup or cleared just the expression presence bits. Seven rotated rounds of 1,000 counter intervals each put the combined variant at 55,446 instructions for undo against 56,621 initially. The full scoped pop moved from 197,542 to 196,859, a 0.35% reduction. cf_live took 105.16 billion before and after. This makes repeated expression processing a higher-priority target than those retirement writes. [Combined experimental patch](perf/scope_restoration/pop_phases/retirement-writes-experiment.patch) · [196 phase measurements](perf/scope_restoration/nov22-retired-writes.csv.gz) · [Paired Apple runs](perf/scope_restoration/nov22-retired-writes-apple.csv) · [Scoped-pop driver](perf/scope_restoration/pop_phases/measure.py)

### What the short-case experiments recovered

Combining declaration family and operator lookup, carrying the operator into the arithmetic reducer, and reusing an arena slot for argument and numeral reads saved 0.33% and 0.41% of incremental replay instructions on ticket 127 and 290, measured as median paired changes. The complete ticket checker used 0.35% *more* instructions with all changes. A second experiment grouped equality operands, used a single lookup for bit-equality recognition and read the expression's sort family directly. Its combined replay savings were 0.09% and 0.11% on the same queries. It changed total toy-consensus instructions by less than 0.02%. All four search counters agreed in the replays. The ordinary path retains its existing dispatch. [Declaration replays](perf/ivy_apple/holdouts/nov22-declaration-replays.csv.gz) · [Short checkers](perf/ivy_apple/holdouts/nov22-declaration-short.csv.gz) · [Apple controls](perf/ivy_apple/holdouts/nov22-declaration-apple.csv) · [Equality replays](perf/ivy_apple/holdouts/nov22-equality-replays.csv.gz) · [Short checkers](perf/ivy_apple/holdouts/nov22-equality-short.csv.gz) · [Apple controls](perf/ivy_apple/holdouts/nov22-equality-apple.csv)

The samples below the theory reducer distinguish the two remaining search workloads. On toy consensus, 431 g3 samples passed through arithmetic comparison, 131 through monomial cancellation and 159 through rational-to-numeral construction. In native code 55 passed through the arithmetic comparison, eight through monomial cancellation and ten through numeral construction. These inclusive counts overlap. On ticket 127 and 290, the g3 Boolean reducer appeared in 104 and 125 samples, with 57 in Boolean flattening on each. Arithmetic comparisons and Boolean simplification provide independent targets. [Every sampled callee, leaf and inclusive frame](perf/ivy_apple/holdouts/nov22-short-reduce-app.csv.gz)

The first assertion exercises a different path. In the current whole-spec assertion profiles the default strategic factory appeared in 7,614 of 8,437 g3 assertion samples on toy consensus, 2,815 of 3,135 on ticket and 1,992 of 2,115 on ping-pong. On ping-pong alone 816 assertion samples included the nonlinear arithmetic tactic constructor. Trying seven layouts and search orders for scalar snapshots yielded 51.55–52.41 million instructions per fresh macro-finder solver versus 51.98 million in the same baseline run. Enlarging the symbol catalog lookup at the preceding revision yielded 55.93–56.43 million versus 56.05 million. Changing the cost of constructing an entire reachable tactic is a more promising startup target. The observed resource-limit behavior of deferred construction still needs to be preserved. [Five-pair assertion profiles](perf/ivy_apple/holdouts/nov22-rewriter-settings-assert-stacks.csv.gz) · [Startup alternatives](perf/ivy_apple/holdouts/nov22-lookup-trials-startup.csv.gz) · [Complete short checkers](perf/ivy_apple/holdouts/nov22-lookup-trials-short.csv.gz) · [Apple controls](perf/ivy_apple/holdouts/nov22-lookup-trials-apple.csv.gz) · [Constructor resource-limit controls](#which-parts-of-strategy-construction-pay-for-themselves)

To reproduce the positive control from `pkg/`, build the native shell from this repository's `src/` tree and run on an available Linux CPU with user-mode instruction counters. Each output directory must be new. The replay fails if either verdict or any of the four search counters differs. Additional `--go-build NAME=PATH` options rotate alternative builds through the same inputs.

```sh
go build -pgo=off -o /tmp/g3-short-query ./cmd/g3
GOMAXPROCS=1 python3 perf/ivy_apple/holdouts/query_replay.py \
    --g3 /tmp/g3-short-query --native ../build/z3 --cpu 14 \
    --pairs 7 --output /tmp/g3-short-comparison > /tmp/g3-short-comparison.csv
python3 perf/ivy_apple/holdouts/query_stacks.py \
    --g3 /tmp/g3-short-query --native ../build/z3 --cpu 14 \
    --queries toy-consensus-18 --pairs 6 --output /tmp/g3-short-stacks > /tmp/g3-short-stacks.csv
```

The arithmetic microbenchmarks also show how much runtime noise matters for a single proof. With Go collection disabled for a short-lived replay, decoding one arena header and returning an already-interned small positive numeral cut toy-consensus check 18 by **3.04 million instructions (0.43%)**, winning all 12 pairs. On the ordinary complete toy-consensus run the same variant used 0.14% more instructions. Reading Boolean marks only once, omitting the clear of a pointer-free scratch slice and giving the flattened argument list eight inline slots cut each ticket replay by about 0.1%. The combined complete ticket run used 0.22% more instructions. [180 GC-off replays](perf/ivy_apple/holdouts/nov22-short-query-gc-off.csv.gz) · [Experimental patches against 91d1597f3](perf/ivy_apple/holdouts/rewriter_trials)

A rewriter-local cache of the eight integers −8 through −1 reduced toy-consensus check 18 by 2.82% and the complete example by 0.55% in the paired instruction comparisons. **The cache returned a numeral of the wrong sort after the manager changed logic.** The control keeps the integer expressions live, selects QF_NRA, and compares subsequent requests for −1 and 1 with the arithmetic plugin. Three candidate caches returned a stale *semantic* result even though the arena handles were live and their generations matched. The ordinary implementation passed in ordinary and checked/debug/trace builds. This control is now part of the rewriter suite. [Regression test](ast/rewriter/arith_rewriter_test.go) · [Experimental eight-value cache](perf/ivy_apple/holdouts/rewriter_trials/negative-numeral.patch)

A second prototype placed eight weak coefficients per requested numeric type in the arithmetic plugin and invalidated them when the manager switched integer conversion mode. A lookup validated its arena generation and kept the existing path for traced or fault-injected creation and explicit numeral invocations. Both focused test configurations passed, including the logic-change regression. With at least 1 MiB of arena budget headroom, the corrected prototype saved **12.96 million instructions (1.84%)** on the GC-off toy-consensus replay in all eight pairs. The ordinary complete toy-consensus checker improved **0.30%** in seven of eight pairs. Ticket and rfn2 had median paired changes of +0.04% and +0.08%. This remains experimental. The extra plugin fields count toward each manager's arena budget, and the headroom cutoff has not been shown to preserve every exhaustion boundary. [Corrected cache](perf/ivy_apple/holdouts/rewriter_trials/plugin-numeral-headroom.patch) · [Matched replays](perf/ivy_apple/holdouts/nov22-headroom-replays.csv.gz) · [32 ordinary short runs](perf/ivy_apple/holdouts/nov22-headroom-short.csv) · [Six rfn2 runs](perf/ivy_apple/holdouts/nov22-headroom-rfn2.csv)

Monomial recognition from a single arena header and direct access to the two operands of a binary product reduced toy-consensus checker instructions by 0.30% together. The same combination increased ticket by 0.44%. The raw data include the arithmetic, Boolean and coefficient variants and a restrictive plugin-cache control enabled only with an unlimited budget. The shell's usual standalone budget is finite. The headroom experiment above measures enabled reuse there. [Matched proof searches](perf/ivy_apple/holdouts/nov22-short-rewriter-trials-replays.csv.gz) · [Whole short specifications](perf/ivy_apple/holdouts/nov22-short-rewriter-trials-short-examples.csv.gz) · [Apple controls](perf/ivy_apple/holdouts/nov22-short-rewriter-trials-apple.csv.gz)

### Priorities after the November 22 measurements

1. **Reduce the cost of first-push preprocessing on the Apple isolates.** Ten calls still explain 8.61 billion excess instructions on rfn2. Isolate the first-push substitution and macro-finder work on the same live bases, measuring the number of nodes visited, substituted or rebuilt alongside retired instructions. A reduction in repeated traversal can benefit several isolates. The before-scope experiment showed why skipping a private proof's push seldom eliminates this work when the same solver later takes a concrete check. [Current push intervals](perf/ivy_apple/incremental_cost/nov22-current-mutations.csv.gz) · [Pass-level measurements](#where-the-remaining-apple-push-instructions-go) · [Before-scope experiment](#can-the-private-proof-avoid-the-first-push)
2. **Separate repeated arithmetic normalization from Boolean flattening.** Use toy-consensus 18 as the matched-search control for the arithmetic comparison. Count repeated coefficient and power-product decompositions per comparison before changing storage. Use ticket 127 and 290 for Boolean work and the quantifier queue. Preserve the per-query resource, conflict, decision and instantiation counts. Accept a change when it also moves the corresponding complete example. The observed savings from shortening individual accessors were small compared with the rewriter gap. [Captures and measured costs](#three-reproducible-obligations-that-still-favor-z3)
3. **Keep reducing eager startup with its API behavior under test.** Fresh macro-finder solvers still cost 52.07 million versus 29.90 million instructions. The nonlinear arithmetic and theory-rewriter constructors show up on the assertion stack even in ping-pong. A manager-local immutable parameter schema or deferring pure work within an already-reserved tactic are candidates. Check ordered descriptions, option changes, resource-limit failure stages and live heap per independent solver before accepting either. [Resource contract](#which-parts-of-strategy-construction-pay-for-themselves) · [Worker benchmarks](perf/ivy_apple/holdouts/strategic_workers.py)
4. **Use a memory budget as a second acceptance condition for dramc_nb2.** The default-runtime win matters to current users. At comparable resident memory, a linked-Z3 process can still finish faster. Record both default and matched-memory runs, including GC CPU and peak RSS per process, when changing allocation rates or how much each worker retains. [Memory comparisons](#the-effect-of-go-heap-growth-on-dramc_nb2)

### Where the proof developed for rfn2 check 4 actually runs

The ground-instance proof discharged six checks in `rfn2` and twelve in three other Apple isolates. Its first success in `rfn2` was check 4. The expanded-witness method already discharged check 1. In the earlier `rfn` isolate, the exact check-4 method discharged checks 5, 9, 13 and 25. Its introduction reduced that whole isolate from 25.17 to 8.69 seconds in six paired rounds. The planner selects terms by sort and universals by formula structure. Its thresholds for spending private search time were tuned on the measured workloads. A verified contradiction in a subset of ground consequences proves the full input inconsistent. [Grounding rule](api/api_goal_instances.go) · [Earlier timings and admission trials](#ground-instances-at-the-constants-in-the-verification-goal)

| Workload | Verification calls | Check-4 method, attempts / proofs | Check-26 short-premise method, attempts / proofs |
|---|---:|---:|---:|
| Apple rfn | 33 | 7 / 4 | 1 / 1 |
| Apple rfn2 | 26 | 7 / 6 | 2 / 2 |
| Apple cmpl_live | 437 | 4 / 4 | 3 / 3 |
| Apple dramc_nb2 | 2,479 | 6 / 4 | 3 / 2 |
| Ordinary Raft | 419 | 0 / 0 | 0 / 0 |
| Toy consensus | 73 | 0 / 0 | 14 / 7 |
| Six other independent short specifications | 436 | 0 / 0 | 2 / 0 |

I instrumented the current GoIvy adapter at its verification entry point and g3 at the private subproblem call, recording each plan's concrete Go type and result. Every complete stdout and exit status matched its linked-Z3 reference across 3,903 verification calls. The 3,904 CSV rows include two plans tried in order on `rfn2` check 7. An empty plan cell means this subproblem entry point was never reached during the check. The separate order proof and Boolean projections can settle checks earlier. The counts measure the deployed choice of proof on these inputs. They leave open whether forcing the method on other formulas would succeed or save time. Raft and the seven unrelated examples supplied 928 calls with zero attempts by the original check-4 method. The check-26 method transferred to toy consensus on calls 3, 11, 33, 34, 35, 71 and 72. [Every call and attempt](perf/ivy_apple/goal_instances/nov22b-plan-coverage.csv.gz) · [Coverage driver](perf/ivy_apple/goal_instances/plan_coverage.py)

The observer used g3 `a366e4240`, GoIvy `0bbf80c87`, and the active local g3 replacement. To reproduce, run the driver with `--goivy PATH --cpu N --output NEW_DIR` and repeat `--workload NAME SPEC ISOLATE_OR_DASH EXPECTED_EXIT REFERENCE_STDOUT` for each checker invocation. Use the same spelling of the specification path as the reference run because GoIvy includes it in its output.

### First Push on the ten fresh rfn2 solvers

Across five pinned rounds the ten calls used 12.145 billion instructions in g3 and 3.530 billion in linked Z3. The second and eighth fresh solvers account for 4.05 billion of the 8.61 billion difference. The table identifies them by external Push ordinal so the instruction sampler can record each call independently. The solver-thread counter subtracts an adjacent empty interval. Each table entry is a median. [Per-call counters](perf/ivy_apple/incremental_cost/nov22-current-mutations.csv.gz) · [Fresh solver ordinals](perf/ivy_apple/incremental_cost/nov21-base-overlap.csv) · [Ranking driver](perf/ivy_apple/incremental_cost/rank_pushes.py) · [Push-specific sampler](perf/ivy_apple/incremental_cost/push_profile.py)

| External Push | g3 instructions | Linked Z3 instructions | Quantifier instances in each engine | Clauses created in each engine |
|---|---:|---:|---:|---:|
| 4 | 3.705 B | 1.144 B | 10,574 | 19,661 |
| 22 | 2.120 B | 0.628 B | 3,780 | 6,439 |

Both engines reported 25,753 instantiations and 44,888 new clauses across the pushes. All 18 solver counters matched on Push 4, including 1,388,825 resource steps and 19,393 new Boolean variables. On Push 22, g3 made four more Boolean variables and four more equalities and used 48 more resource steps. Across all 26 pushes, three had any counter difference. On the independent Apple isolates, all 18 counters matched on all 437 pushes of `cmpl_live` and on 334 of 336 pushes of `cf_live`. The before/after statistics were acquired in separate runs from the instruction measurements. The complete proof transcript matched in every run. These counts locate the largest gap in the implementation cost of comparable work. [Every Push and statistic](perf/ivy_apple/incremental_cost/nov22b-push-search-statistics.csv.gz) · [Statistics driver, use `--operation push`](perf/ivy_apple/search_counters.py)

During the ten first pushes the beta reducer made 33,168 calls, visited 2,154,115 terms and rebuilt 1,068,165 applications. Only 67,282 visits belonged to an exact repeat of the same root, ordered bindings and shifts within a pass and solver. This last census covers binding tuples of at most eight. On the largest push alone, the beta reducer outside assertion reduction visited 990,695 terms across 11,353 calls but saw only 763 distinct source terms. A different binding can require a different result for any of those terms. The theory rewriter also visited 1,383,586 terms during the ten calls. The observer runs only on the first Push of each new or reset solver. [Census by solver and pass](perf/ivy_apple/incremental_cost/nov22b-first-push-work.csv.gz) · [Reproduction driver](perf/ivy_apple/incremental_cost/first_push_work.py)

Two experiments put bounds on the opportunity. Reusing the known application declaration during ordinary reconstruction saved a paired median of 50 million instructions across the ten pushes in five rounds. A solver-local cache keyed by the source subexpression and only the bindings it uses saved 941 million in five paired rounds, with a range of 897 million to 1.013 billion. This second result is **experimental**. The shortcut also skipped resource-limit polls inside repeated subtrees. A run of its 4,096-entry sibling used 1,214,355 resource steps on Push 4 versus the original 1,388,825. The 16,384-entry measurement establishes the saving of that implementation under the default limit. It cannot establish the saving with equivalent cancellation behavior. [Reconstruction trials](perf/ivy_apple/incremental_cost/nov22b-push-rebuild-trials.csv.gz) · [Reconstruction patch](perf/ivy_apple/incremental_cost/nov22b-rebuild.patch.gz) · [Subexpression cache trials](perf/ivy_apple/incremental_cost/nov22b-push-projection-trials.csv.gz) · [Experimental cache patch](perf/ivy_apple/incremental_cost/nov22b-projection-diagnostic.patch.gz)

Push 4 allocated 125,280,280 Go heap bytes in 400,952 objects and triggered four collections. Push 22 allocated 54,980,288 bytes in 247,085 objects and triggered two. In a separate five-pair diagnostic, disabling GC for the process lowered the ten-push sum by 763 million instructions at the paired median. That recovered about 9% of the measured gap and changed the heap growth policy. The samples measure the whole Go heap while the adapter call runs. They isolate a different cost from the retired-instruction counter on the solver thread. [Per-Push heap deltas](perf/ivy_apple/incremental_cost/nov22b-push-heap.csv.gz) · [GC counter comparison](perf/ivy_apple/incremental_cost/nov22b-push-gc-instructions.csv.gz) · [Heap measurement driver](perf/ivy_apple/incremental_cost/push_heap.py)

### Reusing the traversal of a quantified body on fresh solver pushes

The first-push rewriter was rediscovering the shape of the same quantified body for different ground bindings. The solver now records a bounded postorder traversal after four instance-cache misses on a body. On a later miss it fills the bound variables and reconstructs the applications with the ordinary arena constructor. The arena still interns every application and the rewriter retains its normal activity-cache entries. Each solver owns its traversal data, capped at 128 bodies and 128 KiB of compiled arrays plus bounded scratch space. A folding conditional, a nested binder on the substitution path, a finite resource limit, a trace, or an armed allocation failure takes the ordinary traversal. The compiled path checks cancellation twice per distinct non-ground application, matching the number of polls in the original traversal of the supported DAG. [Implementation](ast/rewriter/ground_instance.go) · [Conditional, limit, retirement and reuse tests](ast/rewriter/ground_instance_test.go)

Across nine alternating baseline/change pairs on CPU 27, the ten fresh pushes in rfn2 fell from a median **12.244 to 10.972 billion instructions**. The paired saving was **1.241 billion**, with all nine between 1.180 and 1.294 billion. This recovers about 14% of the previously reported 8.61-billion gap. A new five-pair comparison on CPU 18 measured **11.043 billion in the changed g3 versus 3.531 billion in linked Z3**, leaving a paired median gap of **7.513 billion**. Both measurements sum the ten individual solver-thread counters within each round before taking the median and subtract the adjacent empty read from each interval. [Old/new push intervals, including an independent cf_live run](perf/ivy_apple/incremental_cost/nov22b-ground-body-pushes.csv.gz) · [New g3 versus linked Z3](perf/ivy_apple/incremental_cost/nov22b-ground-body-linked.csv.gz) · [Measurement driver](perf/ivy_apple/incremental_cost/mutation_costs.py)

The saved streaming benchmark makes 1,024 different ordered substitutions per batch into a shared conditional DAG through the ordinary instance cache. Nine alternating 65,536-iteration runs on CPU 20 measured 2,782 to 1,957 ns/op. Whole-process instructions for the benchmark fell from 2.036 to 1.394 billion. On complete checkers, old/new billion-instruction medians were 44.63/42.98 for rfn2, 48.51/47.07 for rfn, 119.22/117.69 for cmpl_live and 282.89/278.94 for sys_live. Whole-checker wall times varied under shared load. The three small incremental shell obligations gained only 1–3 million instructions in the comparison of separate medians. This substitution change addresses a disproportionately expensive operation on the Apple pushes. [Go benchmark](smtcore/smt_quantifier_substitution_benchmark_test.go) · [Microbenchmark observations](perf/ivy_apple/incremental_cost/nov22b-ground-body-stream.csv.gz) · [Complete checkers](perf/ivy_apple/incremental_cost/nov22b-ground-body-checkers.csv.gz) · [Three shell replays](perf/ivy_apple/incremental_cost/nov22b-ground-body-replays.csv.gz)

All 18 g3 statistic deltas matched the previous build at each of 799 pushes across rfn2, cf_live and cmpl_live. With every private refutation independently checked by the ordinary solver, all 18 concrete statistics and the proof outcomes matched linked Z3 at every one of 10,342 checks across the nine Apple isolates. [Concrete results](perf/ivy_apple/incremental_cost/nov22b-ground-body-concrete.csv.gz) · [Concrete replay driver](perf/ivy_apple/goal_instances/verify.py) · [Push counters](perf/ivy_apple/search_counters.py)

### Why matched incremental queries retire more instructions in g3

Seven hardware-counter pairs measured one corresponding function at a time in both shells on three saved incremental queries. Both shells returned UNSAT with identical decisions, instantiations, conflicts and resource steps in every comparison. The new measurements count calls as well as instructions inside the selected function on the solver thread. They include its callees. Nested rows in the table must therefore be read separately. The native comparison uses this fork's C++ shell, compilation flags, objects and link order. A timer around the selected C++ method recompiles its containing translation unit, which can affect inlining. Broader operation timings and uninstrumented whole-process totals provide independent checks. [Measurement driver](perf/ivy_apple/holdouts/query_operations.py) · [Every count](perf/ivy_apple/holdouts/nov22b-matched-replay-operations.csv.gz) · [Independent whole-shell replays](perf/ivy_apple/incremental_cost/nov22b-ground-body-replays.csv.gz)

| Incremental obligation | Operation | Calls in each engine | g3 / Z3 million instructions |
|---|---|---:|---:|
| Toy consensus 18 | Check-sat command | 1 | 583.3 / 166.5 |
| | Theory application reduction | 41,444 | 126.25 / 14.78 |
| | Arithmetic inequalities | 2,908 | 58.45 / 7.68 |
| | Boolean AND/OR literal scan | 4,779 | 6.74 / 1.23 |
| | Cached quantifier-body instantiation | 906 | 9.83 / 3.76 |
| Ticket 127 | Check-sat command | 1 | 165.7 / 54.6 |
| | Theory application reduction | 17,879 | 18.57 / 2.53 |
| | Boolean AND/OR literal scan | 3,718 | 5.03 / 0.98 |
| | Cached quantifier-body instantiation | 1,389 | 12.78 / 4.85 |
| Ticket 290 | Check-sat command | 1 | 184.7 / 56.8 |
| | Theory application reduction | 21,262 | 22.75 / 2.96 |
| | Boolean AND/OR literal scan | 4,748 | 5.56 / 1.05 |
| | Cached quantifier-body instantiation | 1,241 | 12.30 / 4.09 |

On toy consensus the two shells processed exactly the same number of theory applications. The cost averaged **3,046 versus 357 instructions per call** to dispatch and apply the theory rule to the rewritten arguments. The complete application reducer therefore explains about 111 million extra instructions. Arithmetic inequalities account for about 51 million within that work. Neither ticket query called the arithmetic inequality routine. Across all three, the check-sat command itself still cost 3.0–3.5 times as many instructions in Go. Shell initialization and parsing account for only part of the whole-process difference on these controls. [Go dispatch](ast/rewriter/th_rewriter.go) · [Native dispatch and direct argument array](../src/ast/rewriter/th_rewriter.cpp)

The C++ numeral reader checks the application tag, loads its declaration, and reads its two parameters. The Go reader validates and resolves the expression, declaration information and parameter records through its manager. The measured rational-read entry points on toy consensus cost 10.1 versus 1.1 million instructions; the entry counts themselves differed (27,147 versus 44,472). Both engines also cache small nonnegative numeral applications. On a miss the Go construction goes through the plugin's owned numeral, declaration and application factories. The measured construction entry points took 27.5 versus 2.8 million instructions with 2,713 versus 2,414 entries. These observations include work outside inequalities and overlap the theory measurements. [Go reader](ast/ast_arith_numeral_text.go) · [Go constructor](ast/ast_arith_numeral.go) · [C++ reader and constructor](../src/ast/arith_decl_plugin.cpp)

A prototype tested two changes together. It reads a live application's cached declaration and typed numeral parameters directly from the arena. It also recognizes a comparison already in normal form when an uninterpreted integer term or integer variable is bounded by a small integer literal on the right. The original reducer revisits the two operands while testing powers, cancelling monomials, checking bounds and computing coefficient GCDs. On the same 2,908 comparisons, the prototype reduced the Go interval from **58.45 to 42.98 million instructions**, and the complete theory reducer from **126.25 to 108.72 million**. In seven paired whole-shell runs with GC disabled to reduce runtime variation, the median saving was **17.73 million**. On the full toy-consensus specification the saving was only 15.03 million in five paired ordinary runs, roughly 0.2%. The unchanged search counters make the local improvement attributable to cheaper operations. The whole-program effect leaves most of the original gap open. [Paired replays](perf/ivy_apple/holdouts/nov22b-comparison-prototype-replays.csv.gz) · [Exact prototype method counts](perf/ivy_apple/holdouts/nov22b-comparison-prototype-operations.csv.gz) · [Complete checkers](perf/ivy_apple/holdouts/nov22b-comparison-prototype-examples.csv.gz) · [Experimental patch](perf/ivy_apple/holdouts/rewriter_trials/nov22b-atomic-comparison-experiment.patch.gz)

To reproduce a single row from pkg/, build the C++ shell in the repository's build directory and use a new output directory on a Linux host with user-mode instruction counters:

```sh
python3 perf/ivy_apple/holdouts/query_operations.py \
    --operation arithmetic-comparison --queries toy-consensus-18 \
    --cpu 7 --pairs 7 --output /tmp/paired-comparison > /tmp/paired-comparison.csv
```

### What can be eliminated on a complete incremental check?

On the current build, assertion preprocessing consumed **5.165 billion** of the **11.043 billion** instructions in the ten fresh rfn2 pushes. The corresponding native pass consumed **1.321 billion** of **3.531 billion**. Eliminating the Go/native preprocessing difference would recover just over half the outstanding 7.51-billion gap. The table gives medians of five rounds. Each round sums the counters on the ten pushes. Macro expansion is part of macro finding. The other measured stages can also invoke rewriting internally. [All push and pass measurements](perf/ivy_apple/incremental_cost/nov22b-first-push-stages.csv.gz) · [Go and linked Z3 driver](perf/ivy_apple/incremental_cost/push_stages.py)

| Stage | Calls per engine | g3 instructions | Z3 instructions |
|---|---:|---:|---:|
| Reduce pending assertions | 10 | 5.165 B | 1.321 B |
| Find macros, both passes | 20 | 1.987 B | 0.553 B |
| Expand macros | 9,154 | 1.315 B | 0.398 B |
| Infer quantifier patterns | 10 | 0.734 B | 0.175 B |
| Propagate values | 10 | 0.662 B | 0.164 B |
| Maximize bit-vector sharing, including ordinary reduction | 10 | 0.627 B | 0.162 B |
| Instantiate a quantifier body | 28,523 | 2.158 B | 1.308 B |

Changing how a solver sees the goal can change how many instances it creates. In three rounds on CPU 13, introducing a fresh activation literal for the last assertion improved ticket 127 from 308 to 247 million whole-process instructions and reduced instances from 1,389 to 845. The same encoding worsened toy consensus 18 from 734 to 1,694 million and increased instances from 905 to 1,602. Turning off macro finding reduced ticket 290 from 326 to 256 million, but raised toy consensus to 6.33 **billion**. Delaying eager instantiation and disabling E-matching both regressed all three complete queries. Linked Z3 showed the same directional effects and the same search counts on the corresponding routes. An UNSAT result from the base alone settled two queries whose final assertion simplifies to true. Ticket 127's base was SAT and required its final assertion. These are fresh processes containing one incremental check. [126 shell runs](perf/ivy_apple/holdouts/nov22b-scope-strategies.csv.gz) · [Query encodings and timeout controls](perf/ivy_apple/holdouts/scope_strategies.py)

### Eager work that pays for itself after the push

I tried postponing quantifier instantiation while the general SMT kernel prepares a new scope. Ground propagation and E-matching still run. The prototype snapshots pending instance entries when a scope opens and restores the parent queue on pop. In five paired rfn2 runs this cut the ten first pushes from **10.99 to 6.19 billion instructions**. Across the complete checker it *increased* instructions from **43.02 to 45.20 billion**. On cf_live it increased the total from 101.03 to 134.20 billion, and on Raft from 25.99 to 75.10 billion. All complete proof transcripts matched. Allowing two eager rounds essentially restored the original performance. The experiment favors retaining the base instances that many later scopes use. [Paired process and first-push counters](perf/ivy_apple/incremental_cost/nov22b-bulk-schedule.csv.gz) · [Deferred queue patch](perf/ivy_apple/incremental_cost/nov22b-queue-park.patch.gz) · [Two-round control](perf/ivy_apple/incremental_cost/nov22b-queue-two-rounds.patch.gz) · [Complete checker comparison driver](perf/ivy_apple/incremental_cost/compare_schedules.py)

A separate experiment changed the sequence of assertion preprocessing passes *only while a Push commits pending formulas*. A shorter sequence retaining the first macro search, negation-normal-form conversion, the first ordinary reduction and pattern inference brought the ten rfn2 pushes from **10.97 to 9.15 billion**. It improved the full rfn2, cf_live, toy consensus and ticket runs. Applying just negation-normal-form conversion and pattern inference cut the ten pushes further to **6.32 billion**. It improved the two Apple isolates but regressed both independent short examples. The most aggressive variants, which also omitted negation-normal-form conversion, made the checker fail during Push. Their partial-process counts are excluded from the speed comparisons. Search and resource counts can change with these strategies. The public preprocessing policy stays intact. [Every run and failure status](perf/ivy_apple/incremental_cost/nov22b-bulk-pipeline.csv.gz) · [Experimental pass choices](perf/ivy_apple/incremental_cost/nov22b-push-pipeline.patch.gz)

| Complete checker, billion instructions | Ordinary | Shorter sequence | NNF and patterns only |
|---|---:|---:|---:|
| rfn2 | 43.15 | 40.83 | 38.82 |
| cf_live | 100.90 | 99.24 | 93.29 |
| Toy consensus | 7.24 | 6.90 | 11.36 |
| Ticket | 5.71 | 5.32 | 6.11 |

A disjoint timer around outermost general rewriter calls counted **6.05 billion** instructions on the ten first pushes. Calls that returned their exact input handle accounted for **3.83 billion**. Only **1.17 billion** fell in unchanged calls where the same rewriter had already seen the same root on that push. A cache would also have to establish that options, substitutions, bindings, cancellation behavior and any reference-sensitive rules are compatible. The 3.83-billion number is a generous ceiling for eliminating these whole traversals. On the three short replays the unchanged intervals inside check-sat were 71.5, 21.9 and 39.8 **million**. Even a free identity test at every such entry would leave most of their whole-query gaps. [Three rounds by configuration, push and result](perf/ivy_apple/incremental_cost/nov22b-outer-rewrite-identity.csv.gz) · [Observer and empty-interval subtraction](perf/ivy_apple/incremental_cost/rewrite_identity.py)

### Changing the proof search by enabling quantified premises at check time

On the saved **rfn2 check 13**, both shells can cut instruction usage by roughly an order of magnitude while proving the complete original obligation. For each original assertion, flatten its positive top-level conjunction, put its quantified conjuncts behind a fresh Boolean selector, and pass that selector to check-sat-assuming. All original conjuncts are enabled at the check. On this input there is one selector. Both solvers went from **20,748 to 2,674 quantifier instances and 204 to 5 conflicts**. g3 fell from **13.80 to 1.20 billion instructions** and Z3 from **4.19 to 0.42 billion**. g3 beat the ordinary Z3 route on this input. Z3 still cost fewer instructions when given the same encoding. These are independent shell replays, distinct from GoIvy's private refutations and from the complete isolate. [All shell counters](perf/ivy_apple/holdouts/nov22b-guarded-premises.csv.gz) · [Original, named and guarded encodings](perf/ivy_apple/holdouts/guarded_premises.py)

| Captured query | Ordinary g3 | Guarded g3 | Ordinary Z3 | Guarded Z3 |
|---|---:|---:|---:|---:|
| rfn2 13 | 13.80 B | **1.20 B** | 4.19 B | 0.42 B |
| rfn2 7 | 6.63 B | **2.02 B** | 2.14 B | 0.67 B |
| rfn2 4 | 12.54 B | 4.42 B | 4.35 B | 1.47 B |
| rfn2 22 | 5.42 B | 2.79 B | 1.81 B | 0.94 B |
| rfn2 26 | 3.97 B | 8.22 B | 1.44 B | 2.63 B |
| cf_live 42 | 5.64 B | 3.17 B | 2.03 B | 1.09 B |
| cmpl_live 43 | 5.27 B | 3.47 B | 1.83 B | 1.14 B |
| Ticket 127 | 0.31 B | 0.40 B | 0.12 B | 0.14 B |
| Ticket 290 | 0.33 B | 0.36 B | 0.12 B | 0.13 B |
| Toy consensus 18 | 0.73 B | 7.47 B | 0.22 B | 2.15 B |

The table uses the same rule on every row, two alternating rounds for Apple, three for the independent examples, and complete-process user-mode instruction counters. On toy consensus a shared guard was about **ten times worse** in both engines. Having all premises forced at a later search level changes preprocessing, decision order and learned clauses. A planner cannot infer the direction from the presence of quantifiers alone.

An offline experiment asked the native solver to find an UNSAT core, then replayed just those conjuncts from scratch in each ordinary solver. On rfn2 13 the core contained eight of 537 conjuncts and g3 solved it in 60 million instructions. On ticket 127 it contained 30 of 121 and g3 solved it in 88 million. Discovering those cores cost the native solver an additional 464 million and 89 million respectively. On toy consensus the known 51-conjunct core was **slower** than the original 79, taking 2.07 billion in g3. Retaining all conjuncts with up to two bound variables per binder gave four SAT answers and one timeout on the five rfn2 captures. A SAT answer from a weakened formula cannot be used as a counterexample to the full obligation. [Subset, discovery and replay costs](perf/ivy_apple/holdouts/nov22b-premise-subsets.csv.gz) · [Reference-core and arity driver](perf/ivy_apple/holdouts/premise_slices.py)

I also tried selecting premises using just declaration occurrences in the verification goal. One expansion through the rarest shared declarations refuted cf_live 42 with 11 of 687 premises and 71 million instructions. Keeping its ground facts as well refuted cmpl_live 43 with 231 of 718 and 129 million. The same family of rules produced no refutation on the four tested rfn2 goals or ticket 127 within a 45,000-step budget. The CSV records selection wall time in Python separately from shell instructions, and records budget failures as inconclusive. [All selection attempts](perf/ivy_apple/holdouts/nov22b-selected-by-goal.csv.gz) · [Selection driver](perf/ivy_apple/holdouts/goal_symbols.py)

### A sevenfold reduction in the actual ten first pushes

A fresh activation literal on a large quantified base cuts the **ten first pushes in the GoIvy rfn2 checker from 11.07 to 1.59 billion user instructions**. Linked Z3 took 3.53 billion on the same ten operations in the paired-engine study above. The g3 numbers are medians of five alternating rounds. The complete uninstrumented rfn2 checker improved from 43.01 to 37.90 billion. About half the push savings survived through the end of the checker. [All runs, peak RSS and exit status](perf/ivy_apple/incremental_cost/nov22b-guard-scope-panel.csv.gz)

The prototype keeps every public assertion on the first solver. When the second solver receives a quantified assertion with at least 192 conjuncts after flattening top-level conjunctions, it places the *whole assertion* behind a fresh private Boolean. At check time it assumes that Boolean along with all user assumptions. During the guarded push it keeps negation-normal-form conversion and pattern inference. On return to the outermost scope after two concrete checks it permanently asserts the Boolean, so later sibling scopes can reuse unconditional propagation. The 192-conjunct threshold was an experimental choice. No spec name, assumed proof result or previously computed core enters the decision. [Go patch against 4df533a51](perf/ivy_apple/incremental_cost/nov22b-guard-base.patch.gz)

| Complete GoIvy run | Ordinary g3, billion instructions | Guard and promote after two checks | Rounds |
|---|---:|---:|---:|
| rfn2 | 43.01 | **37.90** | 5 |
| rfn | **47.08** | 81.98 | 3 |
| cf_live | 101.11 | **96.56** | 3 |
| cmpl_live | 117.75 | **114.92** | 3 |
| sys_live | 279.04 | **265.36** | 2 |
| dramc_nb2 | **388.14** | 410.18 | 2 |
| Ordinary Raft | 26.01 | 25.98 | 3 |
| Toy consensus | 7.22 | 7.23 | 5 |
| Ticket | 5.70 | 5.71 | 5 |

On dramc_nb2 the median peak also rose from 227 to 322 MiB at the same default GC settings. On rfn, promoting after one concrete check with the same shortened preprocessing sequence cost 86.99 billion. Measuring individual verification calls in that variant localized **24.45 billion extra instructions to check 29**, 7.89 billion to check 21, 6.23 billion to check 17 and 3.98 billion to check 30. All four reached the concrete solver in both builds. The counter includes the private proof attempt and a route observer on both sides. The result explains the risk of changing search order for a reusable parent scope. It rules out enabling the prototype globally on the strength of its first-push speedup. [Per-check counters, routes and wall times](perf/ivy_apple/incremental_cost/nov22b-guard-check-costs.csv.gz)

Every complete run listed in the table matched its linked-Z3 reference transcript and exit status. Four additional controls exercised learning-switch, list reversal, arrayset and a client/server spec with a genuine counterexample. Small artificial formulas also checked satisfiable models in nested scopes, contradictions, multi-pop, assumptions, a similarly spelled user symbol and reset. Those checks cover the observations in the CSV. The prototype still needs public unsat-core and complete model-enumeration coverage before use as a public solver policy. [Control inputs and expected observations](perf/ivy_apple/holdouts/guard_scope_controls.py) · [Control results](perf/ivy_apple/holdouts/nov22b-guard-scope-controls.csv.gz)
