# Raft verification and SMT scope restoration performance

## November 23 verification results

Eliminating existential witnesses defined by equalities reduced the complete 1,986-check Apple `ord_live2.ivy` verification from **3.54 to 2.84 trillion instructions**. Linked Z3 still used 1.97 trillion. On 8,192 independent positive square roots, g3 took **0.212 s** against Z3’s **0.327 s** by solving once and reconstructing every variable. [Whole Apple verification](#eliminating-equality-defined-existential-variables-in-verification-conditions) · [Nonlinear search and exact models](#solving-repeated-independent-polynomial-systems-once)

On a generated nonlinear arithmetic benchmark with 4,096 roots sharing an unknown real parameter, g3 went from **89.13 s to 0.111 s**, versus **0.649 s for Z3**. Both the parameter and every root retain their exact algebraic values. [Shared-parameter measurements](#shared-parameters-in-repeated-polynomial-constraints)

For 256 distinct square roots joined by inconsistent order constraints, g3 went from **2.977 s to 0.028 s**, versus **0.388 s** for Z3. With compact exact models, checking **16,384** satisfiable distinct roots took **0.586 s** in g3 and **0.820 s** in Z3. Printing every model value took **1.130 s** and **0.999 s**, respectively. [Model representation and output](#compact-exact-square-roots-in-models) [Signed square-root models and contradictions](#ordered-square-roots-in-nonlinear-real-arithmetic)

Model evaluation now uses the defining polynomial of an algebraic number to evaluate an exact power when possible. Eight reads of 256 quadratic equations fell from **5.58 to 0.45 billion instructions**, versus **0.39 billion for Z3**. [Products, powers and negative controls](#evaluating-powers-from-the-defining-polynomial)

On five pinned pairs of whole-verifier runs, g3 now completes abstract raft_no_assume in **2.85 seconds** versus **3.49 seconds** for linked Z3, and raftok in **2.61 versus 2.83 seconds**. On Apple rfn2, g3 uses 14.68 billion instructions versus 51.78 billion for Z3. On that panel the largest deficit was TLB at 100.12 versus 69.55 billion. A subsequent 418-input survey found a larger one on the 795-check issue73 order specification. First-push preprocessing and concrete checks both contribute. Three incremental Raft controls also expose a roughly threefold cost per equivalent search. Earlier frontend work saved 231 billion instructions on TLB for both backends by changing private name generation. [Wide finite models and current results](#checking-wide-finite-models-with-ordered-decision-diagrams) · [Equivalent-search controls](#three-raft-cores-with-matched-search-statistics) · [First-push and check census](#where-the-general-solver-still-spends-more-instructions) · [Original scope benchmark](#november-23-makefile-scope-benchmark)

During value propagation on TLB’s 58 first pushes, g3 and linked Z3 both made 87,961 rewrite calls and took 1,120,822 traversal steps. g3 spent **2.74 billion instructions** inside those calls and Z3 spent **0.84 billion**. Only 568 of the Go steps revisited the same root under identical rewrite settings. Caching repeated roots across phases saved 1.4% of the whole verification in an experimental build. [Measurements and reproducer](#rewriting-on-tlbs-58-first-pushes)

## November 22–23 verification results

A private eight-byte counter in GoIvy now issues fragment variable names without a table of issued names. It removed 136,961,124 suffix probes and about 231 billion instructions from a complete TLB verification on either backend. The remaining lexical map resolves references to their binders. The arena, compact Boolean records and disabled-trace guards are retained. [Fresh-name benchmark](#fresh-variable-names-in-the-ivy-fragment-checker) · [Trace allocations](#disabled-trace-arguments-accounted-for-40-gb-of-allocation)

With GoIvy `59aa2840e`, before the final frontend allocation change, g3 won all seven default-GC pairs on the finite-domain Raft wrapper, Apple rfn2, cf_live and dramc_nb2, and the numeric example. Linked Z3 still won abstract Raft, raftok, TLB, Flash and hybrid broadcast. At matched high memory on dramc_nb2 the median was **24.80 s for g3 versus 24.51 s for linked Z3**. Linked Z3 won nine of eleven pairs there. [Complete timings, instructions and memory](#complete-verification-and-scope-benchmarks-at-the-end-of-the-study)

The original 8.61-billion-instruction first-push penalty on rfn2 is gone. The new first-push priority is TLB, at **20.39 billion g3 instructions versus 5.92 billion for Z3**. On identical repeated search, rewriting explains 46 million of the 149 million extra instructions per ticket check and 576 million of the 1.22 billion extra per toy-consensus check. The rest spans quantifier handling, internalization, model work, relevance, arithmetic and memory management, in different proportions by workload. [Actual first pushes](#preprocessing-on-the-first-physical-push) · [Matched-search profiles](#where-each-additional-matched-check-spends-instructions)

The final frontend, GoIvy `48ab9a5dd`, also avoids collecting every source symbol when choosing public action intermediate names. On Apple dramc_nb2 it saved **about 350 MB of cumulative Go allocation per verification on each backend**. On the retained builds g3 used **161.83 billion instructions versus 168.05 billion for linked Z3** at default GC. In the five-pair comparison on a single CPU, dramc_nb2 took **21.26 / 22.81 s** at default GC and **17.18 / 16.84 s** with larger heaps, at measured peak RSS of **471 / 448 MiB**. TLB still favored Z3, **106.63 versus 69.82 billion**. All 87 standalone proof transcripts matched the previous revision and agreed across backends. [Public names, allocations and complete verification](#choosing-public-action-names-with-less-temporary-storage)

## Earlier performance milestones

The AST arena is part of g3's design. Current performance work keeps it and measures how often the solver traverses expressions, looks up Boolean state, and invokes callbacks during a proof.

**Abstract Raft now takes 45.61 billion instructions in g3, down from 100.25 billion. Linked Z3 takes 24.92 billion.** A conditional point-update proof now handles arbitrary uninterpreted domains. It also cut raftok from 78.39 to 42.37 billion instructions. [Complete runs and per-check results](#conditional-point-updates-on-uninterpreted-domains)

**Keeping the committed assertion prefix in place saved 4.97 GB of allocation per TLB run.** The previous code copied 127.6 million committed rows while replacing 1.14 million pending rows. All 18 search counters matched linked Z3 after every one of 22,858 ordinary solver checks across six workloads. [Measurements and reproducible benchmark](#keep-committed-assertions-in-place-during-incremental-preprocessing)

**One check-local counter removed 231 billion instructions from a complete TLB run on each backend.** GoIvy used to restart a suffix search for every repeated variable in the fragment checker. Six paired runs with the counter took median 30.75 s for g3 and 10.99 s for linked Z3, with identical proof outcomes. [Counter, allocation and search measurements](#fresh-variable-names-in-the-ivy-fragment-checker)

**g3 won the original finite-domain Raft test wrapper and both Apple isolates at the default GC setting.** In four alternating pairs per workload on CPU 22, the complete run took 4.06 s versus 46.73 s on `raft_no_assume_test.ivy`, 83.70 s versus 100.72 s on Apple `dramc_nb2`, and 9.87 s versus 26.57 s on Apple `rfn2`. All 12 paired proof transcripts matched. Linked Z3 won all 42 pairs across seven independent short specifications. The two GoIvy configurations also responded differently to a larger heap on `dramc_nb2`. [Abstract and finite-domain Raft comparison](#abstract-raft-and-the-finite-domain-test-wrapper) · [Complete checker comparison](#ordinary-goivy-verification-at-91d1597f3) · [Remaining short-query costs](#three-reproducible-obligations-that-still-favor-z3) · [GC and resident memory](#the-effect-of-go-heap-growth-on-dramc_nb2)

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

#### November 23 comparison with the preserved commit

GoIvy `abcopt` at `01aa2c6fa` contains `ef53df6a4`. Its two symbol-traversal benchmarks are byte-identical to those introduced in `3ef4b9020`. I kept the walker from `abcopt`. The preserved commit sends fixed-arity expressions through a type switch with three stack slots. The current walker dispatches directly on the common types, including equality, negation, implication, application and quantifiers. Equivalence and if-then-else still go through `NodeArgs`.

All three builds passed the 18 focused traversal tests, including repeated occurrences, early termination and a callback that changes the parent between visits. Across five rounds on CPU 46 the current walker took **1.26 µs versus 3.78 µs** for the preserved walker on the fixed-arity fixture, and **2.54 µs versus 7.48 µs** on the application fixture. Both used zero heap allocations on these fixtures. On a separate equivalence and if-then-else tree, the current walker took 3.12 µs and allocated 62 times. Adding the old scratch space just to the fallback reduced that to 1.45 µs and zero allocations. [Benchmark and Go overlay builder](perf/ivy_apple/symbol_traversal/audit_walker.py) · [Fallback fixture](perf/ivy_apple/symbol_traversal/iff_ite_benchmark_test.go.txt) · [All traversal observations](perf/ivy_apple/symbol_traversal/nov23-audit-micro.csv)

| Complete verification | Current walker | Preserved walker | Current walker with scratch fallback |
|---|---:|---:|---:|
| Apple rfn2 | 16.982 | 17.164 | 16.979 |
| Apple dramc_nb2 | 161.926 | 165.444 | 161.898 |
| TLB | 106.492 | 106.583 | 106.548 |

Entries are median billions of retired instructions from four release runs per variant per workload. Each comparison used the same g3 solver, GoIvy module replacement, CPU and input. All 36 runs matched the reference transcripts and exit statuses, including both expected TLB failures. The scratch fallback moved all three medians by less than 0.06%. The older complete walker cost 2.2% more instructions on dramc_nb2. To rerun the focused tests and three fixtures, use `python3 perf/ivy_apple/symbol_traversal/audit_walker.py --goivy ~/ivy/goivy --cpu 46 --output /tmp/goivy-walker-audit`. [Complete verification measurements](perf/ivy_apple/symbol_traversal/nov23-audit-proofs.csv)

#### Measurements on the original rebased checkout

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

### Deferring an unused strategy removes most of the cost of a fresh solver

The fresh macro-finder solver benchmark fell from **51.60 to 2.42 million instructions** when the default strategic factory deferred construction of its nonincremental tactics. The same linked-Z3 operation cost 29.90 million. These are seven rotating slope measurements. Each operation creates a solver, sets the option, asserts true and closes it. No earlier solver needs to build a catalog in this experiment. On **1,024 independent contexts and solvers** the prototype took **1.38 s and 820 MiB peak** to initialize, compared with **8.73 s and 2,785 MiB** for ordinary g3 and **3.67 s and 2,586 MiB** for linked Z3. Initialization was sequential on a pinned CPU in all three. Each worker retained its own solver state. [Startup instructions and allocations](perf/ivy_apple/holdouts/nov22b-deferred-strategy-startup.csv.gz) · [Live worker bytes, peak RSS and elapsed time](perf/ivy_apple/holdouts/nov22b-deferred-strategy-workers.csv.gz) · [Go worker benchmark](api/api_solver_workers_benchmark_test.go)

The experiment builds the ordinary incremental SMT solver immediately. It keeps the outer tactic solver and its public assertions, supplies parameter descriptions directly from the tactic collectors, and constructs the real nonincremental tactic when called. The constructors of two QSAT kernels and a one-bit bit-vector blaster create arena expressions even on incremental-only workloads. Completely skipping those constructors changed search order. Keeping the actual QSAT and bit-vector tactic owners cost 8.30 million instructions per fresh macro-finder solver. Keeping just their cost-model and bit-vector AST owners reached 2.42 million. The latter diagnostic uses the default QI cost expressions. Unusual settings, delayed globals and resource failures need separate treatment in a public implementation. [Experimental Go patch against aa22c8a8f](perf/ivy_apple/holdouts/nov22b-deferred-strategy.patch.gz)

| Complete GoIvy checker, billion instructions | Ordinary | Keep tactic owners | Keep AST owners |
|---|---:|---:|---:|
| Ping-pong | 0.82 | 0.28 | **0.21** |
| Progress | 1.39 | 0.85 | **0.78** |
| Toy consensus | 7.22 | 5.05 | **4.79** |
| Ticket | 5.72 | 4.89 | **4.77** |
| Raft | 25.94 | 25.21 | **25.07** |
| rfn2 | 43.13 | 42.77 | **42.52** |
| rfn | 47.06 | **46.67** | 46.80 |

Each short-example cell is the median of five rounds. Raft and Apple used three. Reference transcripts and exit status matched, including expected failures. Both owner-preserving configurations reproduced all **18 concrete solver counters on each of 10,342 Apple checks** when the original incremental solver independently checked every private refutation. They matched the complete public catalog at all **262 positions**, and the floating-point and arithmetic shell controls returned all 92 expected answers. An earlier cached catalog dropped eight repeated names from the public enumeration. The source deliberately erases and reinserts two parameters while traversing its tactic branches. Comparing only the final name-to-description mapping would have missed that bug. [Complete checker runs](perf/ivy_apple/holdouts/nov22b-deferred-strategy-panel.csv.gz) · [Independent verification and public controls](perf/ivy_apple/holdouts/nov22b-deferred-strategy-controls.csv.gz)

### An incremental solver factory with 44× lower startup cost

GoIvy can now use `NewSolverForIncrementalChecks` for the default logic. A fresh incremental caller constructs its ordinary SMT backend immediately on first use and waits to construct the separate default tactic portfolio until an operation needs it. With macro finding enabled, the measured cost of creating a solver, setting the option, asserting true and closing it fell from **51.61 million to 1.17 million instructions**. Linked Z3 took **29.90 million**. Without options, the three costs were **48.50, 0.89 and 27.96 million**. These are nine rotating pairs of 3- and 13-operation runs on CPU 33, with the context and asserted term live throughout each run. The benchmarks exercise both public constructors. [Startup measurements](perf/ivy_apple/holdouts/nov22b-incremental-factory-startup.csv.gz) · [Go benchmark](api/api_solver_startup_benchmark_test.go) · [Paired C++ and Go driver](perf/ivy_apple/holdouts/startup.py)

Construction order matters to search. Building the incremental SMT backend before the bit-vector terms changed decisions and instantiations on Apple. The factory retains the initial bit-zero, quantifier cost expressions and bit-one terms in their original arena order. One owner per kind suffices. Explicit SMT and context parameters can validate against the incremental backend's descriptors. Full catalog queries and tactic-specific options construct the full portfolio. All 262 catalog positions matched the regular factory. An independent run forced the concrete solver to check all **10,342 Apple obligations**, including private refutations. Every final outcome and all 18 recorded search counters per check matched linked Z3. [Factory and arena term owners](tactic/portfolio/incremental_default.go) · [API tests](api/api_incremental_strategy_test.go) · [Concrete counters by check](perf/ivy_apple/holdouts/nov22b-incremental-factory-concrete.csv.gz) · [Construction-order experiment](perf/ivy_apple/incremental_cost/nov22b-cold-arena-order.patch.gz)

| Measurement | Ordinary g3 | Incremental g3 | Linked Z3 |
|---|---:|---:|---:|
| Initialize 1,024 independent workers, seconds | 9.04 | **1.39** | 3.70 |
| Peak process memory, MiB | 2,785 | **800** | 2,585 |
| Live bytes per worker | 2,398,795 | **767,275** | 2,737,090 |
| Complete Ping-pong, billion instructions | 0.819 | **0.195** | 0.521 |
| Complete Progress, billion instructions | 1.38 | **0.77** | 0.86 |
| Complete Toy consensus, billion instructions | 7.22 | 4.73 | **4.13** |
| Complete Ticket, billion instructions | 5.72 | 4.79 | **3.02** |
| Complete Raft, billion instructions | 25.88 | **24.90** | 215.20 |
| Complete rfn2, billion instructions | 42.98 | **42.45** | 69.30 |

Worker medians use three fresh processes per size, with separate contexts and arenas initialized sequentially on CPU 34. Live bytes use Go heap accounting in g3 and the native allocator in Z3. Peak RSS covers the whole process in both. The complete-process measurements use seven rotating rounds on the seven independent small specifications and five on the Apple and Raft examples. All transcripts and exit statuses matched their references, including real counterexamples. The startup change pays off most when solver setup accounts for much of the job. [Worker measurements](perf/ivy_apple/holdouts/nov22b-incremental-factory-workers.csv.gz) · [Worker benchmark](api/api_solver_workers_benchmark_test.go) · [All complete runs](perf/ivy_apple/holdouts/nov22b-incremental-factory-checkers.csv.gz)

The ordinary public factory retains eager tactic creation. The incremental factory reads constructor-specific global settings and charges tactic memory when it first needs the portfolio. In a floating-point check with a 1 MiB budget, ordinary initialization exhausted the budget on Assert, and the incremental factory reached Check before exhausting it. With 32 MiB, both returned SAT with default settings and unknown when global rewriter.max_steps was set to zero or one after Assert. The final resource-release control returned the arena charge to zero on all 70 runs that parsed the input and reached solver operations, including allocation failures. Debug and trace builds, AST tracing and armed allocation failure injection use eager initialization. [Stage-by-stage budget controls](perf/ivy_apple/holdouts/nov22b-incremental-factory-budgets.csv.gz) · [Release accounting after low-budget and floating-point runs](perf/ivy_apple/holdouts/nov22b-incremental-factory-release.csv.gz)

## Proving an obligation before the first push

The ten fresh solvers in Apple `rfn2` now finish all 26 obligations without a physical push. In five paired runs, their first pushes previously retired 11.025 billion instructions. Trying and validating the proofs before Push cost 4.309 billion instructions across the entire isolate. The previous checks also cost 2.913 billion. These disjoint, solver-thread intervals explain most of the drop in whole-process instructions. [Costs by solver, operation and invocation](perf/ivy_apple/incremental_cost/nov22b-prepush-owner-costs.csv.gz) · [Operation counter driver](perf/ivy_apple/incremental_cost/verification_stages.py)

| Complete workload | Solvers reaching first Push, before → after | Physical pushes, before → after | First-push instructions, billions, before → after |
|---|---:|---:|---:|
| Apple rfn2 | 10 → 0 | 26 → 0 | 11.025 → 0 |
| Apple rfn | 9 → 4 | 33 → 16 | 10.051 → 4.204 |
| Raft | 14 → 0 | 419 → 0 | 1.016 → 0 |
| Toy consensus | 47 → 29 | 73 → 55 | 0.668 → 0.563 |
| Ticket | 17 → 17 | 297 → 297 | 0.671 → 0.690 |

g3 commit `60392bc24` lets a client offer a temporary Boolean goal while the incremental solver still owns its original assertions. After a batch has proved a sibling, it can try a small set of related positive conjuncts selected from those assertions. A successful private worker must refute the selected premises together with the actual goal. The source solver retains its assertions, models and scopes. GoIvy commit `b72feb155` uses this API with the default logic and tracing off. If a goal is inconclusive, the adapter pushes, asserts the goal and runs the original concrete check, then uses ordinary scoped checks for the rest of that batch. The same pattern applies across specs. A failed private attempt also has a measurable cost. [Go implementation and limits](api/api_order_abstraction.go) · [Adversarial inputs and benchmark](api/api_verification_goal_test.go)

With the trace guards described below and default Go GC settings, whole-process median instructions on `rfn2` fell from **37.878 to 28.361 billion** in five rounds. The same change took Raft from 24.710 to 23.678, Toy consensus from 4.419 to 3.903, and Ticket from 4.172 to 4.181 billion. Progress increased from 0.648 to 0.661 billion. Calling the private prover a second time on the first failure increased overhead on short proofs. The shipped fallback calls the ordinary solver directly at that point. A separate seven-round comparison before adding the trace guards took `rfn2` from 42.513 to 33.278 billion. [Complete checker runs with fallback controls](perf/ivy_apple/incremental_cost/nov22b-prepush-fallback.csv.gz) · [Independent comparison on the earlier frontend](perf/ivy_apple/incremental_cost/nov22b-prepush-controls.csv.gz)

Across nine Apple isolates, the optimized checker proved 243 obligations before any push and 1,335 in a scope. The remaining 8,764 used the ordinary backend. A separate run forced all 10,342 original queries through the scoped solver. Its 18 recorded search counters agreed with linked Z3 on every check. The optimized execution, forced execution and linked Z3 agreed on outcomes. The broader transcript comparison covered 11,270 checks including Raft and seven shorter specs. For an independent check on premise selection, standalone Z3 refuted all 26 original `rfn2` queries including the temporary goal. A saved `rfn` query reached the 25-second offline limit on the first attempt and returned UNSAT in three follow-ups with macro finding or incremental scopes. The 28 Raft mutation runs reproduced the reference passes and failures. [Three-execution verification driver](perf/ivy_apple/goal_instances/verify.py) · [All Apple comparisons](perf/ivy_apple/incremental_cost/nov22b-prepush-counter-parity.csv.gz) · [Broader outcomes and independent replays](perf/ivy_apple/incremental_cost/nov22b-prepush-replay.csv.gz) · [Raft mutant controls](perf/ivy_apple/incremental_cost/nov22b-prepush-raft-mutants.csv.gz)

## Disabled trace arguments accounted for 40 GB of allocation

Both engines paid to construct trace arguments in the shared GoIvy frontend. On `dramc_nb2`, a Go heap allocation profile attributed 12.79 GB to a single call in `GoalAddPrem`. GoIvy commit `37006f57b` guards 22 expensive trace calls and exits `CanonSnapshot` before rendering when tracing is disabled or suppressed. The guards cover canonicalization, formatting and tree walks while the result is still inexpensive to discard. The per-site profile attributes an allocation once, at the outermost matching trace site on the sampled stack. Runtime allocation counters independently measured the complete process. [Allocation profiler](perf/ivy_apple/trace_allocations.py) · [Per-site heap samples](perf/ivy_apple/holdouts/nov22b-xtrace-heap.csv.gz)

| dramc_nb2 at GOGC=100, medians of four complete runs | g3 before | g3 guarded | Linked Z3 before | Linked Z3 guarded |
|---|---:|---:|---:|---:|
| Cumulative Go allocation, decimal GB | 58.18 | **17.87** | 56.51 | **16.21** |
| Go objects allocated, millions | 237.55 | 174.85 | 231.51 | 168.81 |
| GC cycles | 1,530 | 496.5 | 4,052.5 | 918 |
| Peak process RSS, MiB | 216.8 | 219.8 | 207.6 | 217.9 |

The savings concern temporary objects created over the course of the run. Peak RSS at default GC settings stayed near 220 MiB. Go heap metrics exclude C++ allocations, while RSS covers the entire process. Before and after runtime panels used different pinned CPUs on a shared machine. In a separate test with treatment and control on the same CPU, `dramc_nb2` instructions dropped from **300.72 to 232.15 billion for g3** at GOGC=295, and **281.51 to 212.26 billion for linked Z3** at GOGC=400. On `cf_live` the corresponding reductions were 76.09 to 64.38 and 78.16 to 67.10 billion. [Runtime counters before](perf/ivy_apple/holdouts/nov22b-xtrace-before-memory.csv.gz) · [Runtime counters after](perf/ivy_apple/holdouts/nov22b-xtrace-after-memory.csv.gz) · [Matched dramc_nb2 process counters](perf/ivy_apple/holdouts/nov22b-xtrace-dramc.csv.gz) · [Matched process counters on other specs](perf/ivy_apple/holdouts/nov22b-xtrace-instructions.csv.gz)

With tracing enabled, the original and guarded binaries emitted byte-identical output on six specs, including **2,334,030,415 bytes** for `rfn2` and **3,650,179,852 bytes** for `cf_live`. Two pipes were compared through EOF and both exit codes were checked. A separate active-tracing comparison of the production checker against the original PyIvy Paxos `log3` reference matched all 27,855 normalized lines, including the expected diagnostic. [Enabled-trace byte comparisons](perf/ivy_apple/holdouts/nov22b-xtrace-enabled.csv.gz)

## Fresh intermediate names in action composition

GoIvy commit `b6dd84126` skips name collection when the two actions modify disjoint sets of variables. On `dramc_nb2`, 115,812 of 118,657 compositions needed no intermediate symbol. Those calls used to visit **218,280,371 AST nodes** to prepare an unused renamer. The remaining 2,845 compositions visited 11,640,948 nodes. On `rfn2` the guard removed 25,701,310 visits and retained 68,429. The commit includes a microbenchmark and tests where an existing transition, first precondition, or second precondition occupies the preferred spelling. Active trace output matched through EOF on `rfn2`, `cf_live` and the failing ring example. [Visits by workload and intersection](perf/ivy_apple/holdouts/nov22b-compose-symbol-visits.csv.gz)

| Complete run | g3 before → after, billion instructions | Linked Z3 before → after, billion instructions | Paired rounds |
|---|---:|---:|---:|
| dramc_nb2 | 231.99 → **195.58** | 213.89 → **175.62** | 5 |
| rfn2 | 22.56 → **18.10** | 57.29 → **52.90** | 6 |
| rfn | 34.27 → **30.83** | 47.18 → **43.92** | 5 |
| cf_live | 63.54 → **57.51** | 67.13 → **61.21** | 4 |
| Toy consensus | 3.306 → **3.229** | 3.330 → **3.264** | 7 |

Each pair ran the same complete specification on the same pinned CPU, with GOGC 295 for g3 and 400 for linked Z3. Every transcript and exit status matched its reference. A separate four-pair `dramc_nb2` panel at default GOGC measured cumulative Go allocation falling from 17.87 to **16.69 GB** with g3 and from 16.21 to **15.03 GB** with linked Z3. With the guard, g3 and native took median 35.13 and 32.25 seconds at measured peaks of 217 and 207 MiB. At GOGC 175 and 240 they took 28.94 and 26.18 seconds at 274 and 291 MiB. Retired instructions are more repeatable than elapsed time on this shared host. [All process runs](perf/ivy_apple/holdouts/nov22b-compose-instructions.csv.gz) · [GC and peak-memory comparisons](perf/ivy_apple/holdouts/nov22b-compose-memory.csv.gz)

### Can a single counter supply the remaining names?

A monotonic 64-bit counter generates a unique suffix until exhaustion. A generated-symbol namespace disjoint from the input namespace makes these identifiers fresh without a membership table. The counter belongs to the checker that constructs the formulas and must outlive all symbols it issues. The existing GoIvy constructor accepts arbitrary strings. Its structural key contains the symbol's spelling and sort, so a numeric suffix alone gives separate identities only among symbols issued by that counter. The experiment used a counter on the existing per-session Ivy configuration. All measured outcomes matched with both counter spellings. The experimental prefixes were unique in these inputs. The production guard preserves the established names.

To separate the cost of looking up a name from the effect of giving the solver different symbols, a recording build saved every requested base and returned spelling. A second build checked the requested base and replayed the recorded answer without traversing the input AST. On `dramc_nb2`, the original algorithm issued 14,337 names and found its preferred spelling occupied 5,062 times. Replaying those spellings improved g3 from 195.19 to 193.98 billion instructions. Using the counter with a descriptive prefix instead took 200.78 billion. The replay includes the cost of reading its small input once per process. [Requests and existing spellings](perf/ivy_apple/holdouts/nov22b-name-counter-requests.csv.gz) · [Recording overlay](perf/ivy_apple/holdouts/nov22b-name-record.patch.gz) · [Replay overlay](perf/ivy_apple/holdouts/nov22b-name-repeat.patch.gz)

| Workload and engine | Existing names | Same names replayed | Descriptive prefix + counter |
|---|---:|---:|---:|
| rfn2, g3 | 18.11 | 18.21 | 18.41 |
| rfn2, linked Z3 | 52.92 | 52.78 | 77.20 |
| rfn, g3 | 30.87 | 30.88 | 50.55 |
| rfn, linked Z3 | 43.82 | 43.83 | 33.88 |
| Toy consensus, g3 | 3.32 | 3.32 | 4.39 |
| Toy consensus, linked Z3 | 3.28 | 3.28 | 3.53 |

The table uses median whole-process instructions in billions. On `rfn2`, linked Z3 went from 270,033 to **439,351 quantifier instantiations** and 805,401 to **975,636 decisions**. The effect reverses on `rfn`, where linked Z3 made 169,531 instantiations with the counter and 244,505 with the existing names. Comparison of the actual native inputs explains part of the difference. In 19 of 26 `rfn2` checks the ordered assertion bodies still matched under a consistent renaming after expanding SMT-LIB lets. In three, a symbol shared in the original assertions split into distinct symbols. Four showed a change of expression arity during that structural comparison. That comparison diagnoses changes in what the backend sees. The end-to-end proof outcomes were verified separately. Across these workloads the largest repeatable gain came from eliminating scans in calls that request zero names. [Counter benchmark runs](perf/ivy_apple/holdouts/nov22b-name-counter-instructions.csv.gz) · [Per-check native search counters](perf/ivy_apple/holdouts/nov22b-name-counter-search.csv.gz) · [Query structure comparison](perf/ivy_apple/holdouts/nov22b-name-counter-query-structure.csv.gz) · [Descriptive prefix prototype](perf/ivy_apple/holdouts/nov22b-name-counter.patch.gz) · [Short prefix prototype](perf/ivy_apple/holdouts/nov22b-name-short.patch.gz)

On checks 22–24 of `rfn2`, the two split declarations are the intermediate functions for `cmpl_live.cf_cpl` and `cmpl_live.arm_cpl`. Within the first assertion, the clauses using one pair contain the Boolean selector `__ts0_e` and the clauses using the other pair contain its negation. Setting the selector to each Boolean value, folding Boolean constants, and comparing **both complete assertions** gives a sort-preserving bijection of symbols in each of the six comparisons. A single interpretation for the chosen pair suffices on either branch. The counter introduces separate declarations into the unsimplified search problem. The four checks with different expression arity remain outside this equivalence result. [Branch comparisons](perf/ivy_apple/holdouts/nov22b-name-branch-equivalence.csv.gz)

## Dependency checks in GoIvy action updates

GoIvy commit `8b926986f` skips a second large source of symbol collection. A named or derived update already knows which symbols the action modified. When that set is empty or the output is already present, the result is immediate. Otherwise the checker walks the current definition until it finds the first reference with a matching name and sort. This preserves edits to definitions between calls. The independent microbenchmark and tests are in `goivy/module_derivedupdate_dependency_test.go` in the same commit. With a 128-term body, the empty-set case took about four nanoseconds and allocated zero bytes. The previous implementation allocated 20,720 bytes per call. Active trace output was byte-identical on Apple `rfn2`, Apple `cf_live` and a failing leader election example. [Before](perf/ivy_apple/holdouts/nov22b-update-dependencies-before.bench.gz) · [After](perf/ivy_apple/holdouts/nov22b-update-dependencies-after.bench.gz)

| Complete run | g3 before → after, billion instructions | Linked Z3 before → after, billion instructions | Pairs |
|---|---:|---:|---:|
| dramc_nb2 | 194.95 → 179.10 | 175.58 → 158.53 | 5 |
| rfn2 | 18.21 → 16.53 | 52.99 → 51.51 | 5 |
| rfn | 30.90 → 29.61 | 44.02 → 42.92 | 4 |
| cf_live | 57.43 → 55.26 | 61.24 → 58.97 | 4 |

These pairs used GOGC 295 for g3 and 400 for the linked frontend and matched the reference output and exit code. In a separate six-round allocation experiment at default GOGC, g3's total Go heap allocation on `rfn2` fell from 1.642 to 1.465 GB. Replaying the same names without a lookup used 1.464 GB. Using the descriptive counter and issuing a distinct name every time used 1.486 GB. Cumulative allocation includes the frontend and all subsequent solver work. [Full instruction panel](perf/ivy_apple/holdouts/nov22b-update-dependencies.csv.gz) · [Default-GC memory and instruction measurements](perf/ivy_apple/holdouts/nov22b-update-name-allocations.csv.gz)

## Abstract Raft and the finite-domain test wrapper

The commonly timed `examples/raft/raft_no_assume_test.ivy` includes `raft_no_assume.ivy` and adds eight interpretations including small bit-vector domains. Running the underlying protocol directly leaves its domains abstract. Both engines passed every check in the three runs below. The mechanism that proves the finite-domain checks cheaply does not apply to the original abstract domains. The distinction changes which engine wins and which code is worth optimizing.

| Input | Checks | Ordinary g3 | Linked Z3 | g3 general solver with ordinary scopes | Paired rounds |
|---|---:|---:|---:|---:|---:|
| raft_no_assume.ivy | 419 | 101.72 | 24.92 | 75.84 | 7 |
| raftok.ivy | 420 | 79.33 | 19.98 | 60.12 | 7 |
| raft_no_assume_test.ivy | 419 | 23.69 | 214.83 | 717.72 | 5 |

Numbers are medians of complete process instructions in billions at default GOGC. Restoring ordinary scopes and keeping all private proof attempts cost 101.73 and 79.83 billion on the two abstract inputs. Looking only at logical verification intervals, abstract Raft check 275 cost 31.114 billion instructions in g3 and 3.756 billion in linked Z3. Checks 321 and 134 cost 9.42 versus 1.13 billion and 11.15 versus 4.50 billion. These measurements include the solver's accumulated state before each check. Standalone SMT inputs retain the actual assertions and allow a distinct fresh-solver comparison. [Whole-process controls](perf/ivy_apple/holdouts/nov22b-abstract-raft-controls.csv.gz) · [Individual checks](perf/ivy_apple/holdouts/nov22b-abstract-logical-intervals.csv.gz) · [Per-check driver](perf/ivy_apple/incremental_cost/logical_checks.py) · [Twelve SMT inputs](perf/ivy_raft/test_vectors)

A one-pair survey covered 188 Ivy inputs in ten directories. For the 87 inputs that completed within 75 seconds and printed at least one PASS or FAIL marker in the first panel, a repeated comparison at default GC found **43** with lower median g3 instruction counts and **44** with lower linked-Z3 counts. The selection includes inputs designed to exhibit failures. Both exit status and the complete stdout agreed on every corrected pair. Five ABC-dependent cases were rerun for four pairs with the project's `abc` executable beside each checker. The other 82 used five pairs. Counts on inputs invoking ABC also include the child process. Cases are listed individually so that parser or external-tool work can be distinguished from SMT work. [Cases](perf/ivy_apple/holdouts/nov22b-survey-cases.txt) · [Driver](perf/ivy_apple/holdouts/spec_survey.py) · [Discovery](perf/ivy_apple/holdouts/nov22b-spec-discovery.csv.gz) · [Confirmation](perf/ivy_apple/holdouts/nov22b-spec-confirmation.csv.gz) · [ABC-present replacements](perf/ivy_apple/holdouts/nov22b-spec-companions.csv.gz)

The largest completed non-Raft opportunity in this sample was `examples/liveness/tlb.ivy`. Both engines printed 18,951 passes and two failures, with 461.31 billion instructions for g3 and 308.47 billion for linked Z3. Hybrid reliable broadcast took 16.25 versus 6.62 billion for 839 passing checks. `flash2.ivy` took 11.51 versus 4.59 billion for 2,147 passes and 65 failures. The initially censored `examp1_numeric.ivy` was rerun twice with a 480-second limit. Both engines passed 14 checks and then reported an inconclusive solver result. The linked checker took 29.91–30.96 seconds and g3 took 79.55–89.95 seconds. The SMT diagnostics differed in let sharing, but their declaration signatures and expanded assertion bodies matched. This case measures the cost of reaching an inconclusive answer. [Long-limit samples](perf/ivy_apple/holdouts/nov22b-numeric-inconclusive.csv.gz)

## Fresh variable names in the Ivy fragment checker

GoIvy commit `51c432daf` gives each macro-free fragment check a private `uint64` counter. Every bound and free variable goes through the same allocator. Even an input variable spelled `V!feu!0` receives its own fresh identity. The counter requires eight bytes and no table of issued names. The per-formula binding map still connects each variable occurrence to the correct binder. Fragment diagnostics recover the original visible spellings on demand. Active traces use the original naming path. Macro formals can share spellings with constants in the fragment dependency graph, so this commit preserves their existing path. The generated variables are private to fragment analysis and never become SMT assertions.

On `examples/liveness/tlb.ivy`, the old checker assigned 21,856 names across three fragment checks. It issued 28 unsuffixed names and tried 136,961,124 suffix candidates to obtain the other 21,828. Of those candidate suffixes, 136,939,296 were already occupied. The first check alone requested one base spelling 16,335 times. The counter issued one ID per request. [Per-check counts](perf/ivy_apple/holdouts/nov22b-fragment-probes.csv.gz) · [Counting patch for GoIvy 8b926986f](perf/ivy_apple/holdouts/nov22b-fragment-name-probes.patch.gz)

| Complete TLB run | Instructions before → after, billions | Wall time before → after | Total Go heap allocation before → after | Go allocations before → after, millions |
|---|---:|---:|---:|---:|
| g3 | 461.21 → 230.08 | 52.33 → 30.75 s | 15.50 → 11.85 GB | 273.73 → 46.00 |
| Linked Z3 | 308.62 → 77.59 | 32.33 → 10.99 s | 6.39 → 2.74 GB | 247.20 → 19.47 |

Instruction counts and wall times are medians of six alternating pairs per backend on CPU 30 at default GOGC with GOMAXPROCS=1. Allocation medians come from four additional pairs per backend on CPU 32. They include all Go code in the process. Peak RSS in the timing panel fell from 140.6 to 132.0 MiB on g3 and from 146.5 to 128.2 MiB with linked Z3. Every run printed 18,951 passes and two expected failures with the same stdout, stderr and exit status. Both engines saved essentially the same absolute number of instructions because they share this frontend. The remaining difference is 152.5 billion instructions. [Process runs](perf/ivy_apple/holdouts/nov22b-fragment-tlb.csv.gz) · [Allocation runs](perf/ivy_apple/holdouts/nov22b-fragment-memory.csv.gz) · [Worktree comparison driver](perf/ivy_apple/holdouts/fragment_names.py)

The microbenchmark constructs one quantified formula and alpha-converts it repeatedly within one fragment check. At 4,096 repetitions, the median fell from 1.166 s to 10.06 ms. Allocated bytes fell from 146.61 MB to 3.90 MB and allocations from 33,323,440 to 90,012. The source benchmark is `BenchmarkFragmentRepeatedVariableNames` in GoIvy's `fragment_variable_names_test.go` at commit `51c432daf`. Run it with `go test . -run '^$' -bench '^BenchmarkFragmentRepeatedVariableNames$' -benchmem -count=5`. [All benchmark samples](perf/ivy_apple/holdouts/nov22b-fragment-names.bench.gz)

A final replication at GoIvy `48ab9a5dd` kept the source-compatible renamer and the counter in the same executable. I alternated their order for nine rounds on CPU 8, with one Go CPU, default GC and one full conversion batch per sample. Moving from 1,024 to 4,096 repeated variables increased the counter's median time from **2.11 to 8.28 ms** and its total allocation from **0.974 to 3.899 MB**. The source renamer grew from **78.73 ms to 1.548 s** and from **7.415 to 146.605 MB**. Those totals include building the renamed formulas and their lexical bindings. Issuing a suffix retains only the eight-byte counter. [All 54 observations](perf/ivy_apple/holdouts/nov22b-fragment-counter-confirm.csv.gz)

Before/after linked-Z3 runs on 181 additional specifications agreed on every exit status and stdout. Stderr was byte-identical for 179. The other two inputs panicked in the same mixin validation with stack traces differing only in hexadecimal addresses. Across four g3 repetitions on each of nine real fragment errors, both output streams matched byte for byte. Three active-trace controls matched stdout byte for byte on Apple `rfn2` (2,334,030,415 bytes), a fragment error (119,927,841 bytes), and a failing ring proof (1,117,729 bytes). [Per-spec comparisons](perf/ivy_apple/holdouts/nov22b-fragment-specs.csv.gz) · [Fragment-error repetitions](perf/ivy_apple/holdouts/nov22b-fragment-diagnostics.csv.gz)

### One counter across assumptions, assertions and macros

GoIvy commit `0afb029cf` extends the same check-local `uint64` sequence to macro inputs. The three input groups share the allocator. It issues a decimal suffix in constant space, with an exhaustion check before reuse. Lexical substitution still maps a source variable to its current binder, so a nested quantifier can shadow an outer variable and restore its meaning on exit.

The scope of the freshness guarantee matters in real Ivy syntax. For example, cloning a `some` binder preserves its original body, and macro stratification uses the same key for a constant and a variable of the same name and sort. A conventional uppercase macro constant can therefore refer to a source-spelled variable. The existing alpha-conversion walk detects these cases and uses source spellings throughout that check. Private-looking source variables are safe when the walk renames all of them. Active tracing also uses source spellings. [Counter and regressions in GoIvy, commit 0afb029cf](https://github.com/glycerine/ivy/commit/0afb029cf)

| Complete Apple isolate | g3 before → after, GOGC 100 | Linked Z3 before → after, GOGC 100 | g3 before → after, GOGC 295 | Linked Z3 before → after, GOGC 400 |
|---|---:|---:|---:|---:|
| dramc_nb2 | 215.33 → 199.13 | 215.44 → 198.74 | 178.13 → 162.65 | 157.67 → 142.33 |
| rfn2 | 20.96 → 19.06 | 56.42 → 54.57 | 16.52 → 14.76 | 51.39 → 49.74 |
| cf_live | 63.62 → 60.51 | 66.61 → 63.38 | 55.21 → 51.84 | 58.86 → 55.63 |

Values are median whole-process user instructions in billions from four alternating pairs per isolate, backend and GC setting. Both revisions used the same g3 solver and linked Z3. All 172 measured runs across eight workloads matched the reference exit status and both output streams. TLB already used the counter and stayed at 230.2 billion instructions for g3 and 77.8 billion for linked Z3. [All 172 process runs](perf/ivy_apple/holdouts/nov22b-fragment-macros-instructions.csv.gz)

Each backend also repeated 181 specifications against its own previous revision. All exit statuses and stdout matched, and 179 stderr streams per backend matched byte for byte. The remaining two contained the same pre-existing mixin panic with different addresses. Permuting variable names and their corresponding Skolem names in nine failing fragment examples gave 108 variants. Every variant retained the same fragment error, exit status and byte-identical output. The three active-trace controls above again matched byte for byte on stdout. [362 corpus comparisons](perf/ivy_apple/holdouts/nov22b-fragment-macros-specs.csv.gz) · [108 diagnostic comparisons](perf/ivy_apple/holdouts/nov22b-fragment-permuted-errors.csv.gz) · [Reproduction driver](perf/ivy_apple/holdouts/fragment_diagnostics.py)

### Keep committed assertions in place during incremental preprocessing

The C++ implementation truncates the formula vector at the first pending assertion, retains its storage and appends the replacement formulas. g3 used to allocate a new slice and copy every committed formula/proof pair into it. In a complete `tlb.ivy` run the kernel replaced 1,143,142 pending rows in 235,079 calls. It copied an additional **127,614,575 committed rows**. In 226,493 calls even the pending formula/proof sequence was identical. The allocation profile attributed 42.2% of total allocated bytes inclusively to the swap across three runs. [C++ vector operation](../src/smt/asserted_formulas.cpp#L329) · [Per-workload census](perf/ivy_apple/holdouts/nov22b-pending-census.csv.gz) · [Counting patch against 8d7225c0f](perf/ivy_apple/holdouts/nov22b-pending-census.patch.gz)

Commit `1023a538a` keeps the committed prefix in place. It reserves room for the incoming rows, saves the old pending owners beyond that range and publishes the replacements. If ownership acquisition fails partway, the kernel releases the acquired owners and restores the pending rows. The proof-enabled regression invalidates the second incoming proof, then checks exact restoration, reference counts and a subsequent successful replacement. [Implementation](smtcore/smt_asserted_macros.go) · [Benchmark and rollback test](smtcore/smt_asserted_batch_benchmark_test.go)

| Complete workload | Instructions at GOGC 100, billions | Allocated GB at GOGC 100 | Instructions at GOGC 295, billions |
|---|---:|---:|---:|
| TLB, 18,953 checks | 229.76 → 213.21 | 11.85 → 6.89 | 196.20 → 189.66 |
| Flash, 2,212 checks | 11.38 → 11.06 | 0.400 → 0.308 | 10.24 → 10.08 |
| Apple rfn2, 26 checks | 19.07 → 19.09 | 1.448 → 1.445 | 14.84 → 14.77 |
| Apple dramc_nb2, 2,479 checks | 199.37 → 198.92 | 14.527 → 14.498 | 163.65 → 164.03 |

Each entry is the median of four alternating before/after pairs, or three pairs for dramc_nb2. Both executables used GoIvy `0afb029cf`, one pinned CPU and GOMAXPROCS=1. The timing version of the patch reads the batch length directly, while the commit stores it in a local. All 92 process runs across nine workloads matched their reference exit status, stdout and stderr. TLB's median peak RSS at default GC was 138.2 → 132.1 MiB. At GOGC 295 it was 207.0 → 210.8 MiB. Multiple workload lanes shared the host, so instruction counts are the primary comparison here. [Raw process, RSS and allocation samples](perf/ivy_apple/holdouts/nov22b-pending-runs.csv.gz)

| Committed rows | Pending rows | Before ns/op | After ns/op | Allocated bytes/op before → after |
|---:|---:|---:|---:|---:|
| 0 | 1 | 127.2 | 78.60 | 32 → 0 |
| 128 | 1 | 2,411 | 78.42 | 12,288 → 0 |
| 4,096 | 1 | 39,385 | 78.77 | 303,104 → 0 |
| 4,096 | 32 | 40,745 | 1,811 | 303,104 → 0 |

These are medians of seven alternating runs with warm kernel storage. Run `go test ./smtcore -run '^$' -bench '^BenchmarkSMTReplacePendingAssertions$' -benchmem -count=7` on the two revisions. The raw file also includes prototypes that used a small stack buffer or appended extra temporary storage. [All 168 benchmark observations](perf/ivy_apple/holdouts/nov22b-pending-micro.bench.gz)

With both backends on their ordinary incremental solvers, **all 18 per-check counter deltas agreed across 22,858 checks**. TLB contributed 18,953, Flash 2,212, hybrid reliable broadcast 839, abstract Raft 419, raftok 420 and the numeric example 15. The g3 revision before the change produced the same counters. The numeric example retained its one inconclusive result. The two backends printed that query with different SMT-LIB let sharing and identical commands after lexical expansion. Focused fast and checked builds and the full Go test suite passed. [Per-check observations](perf/ivy_apple/holdouts/nov22b-pending-search.csv.gz) · [Reproduction and counter comparison](perf/ivy_apple/holdouts/complete_search.py)

The instruction samples help prioritize the next changes. On TLB before the allocation fix, the nearest named stack category accounted for a median **57.74 versus 14.84 billion** instructions in the solver rewriter, **28.40 versus 5.90 billion** in the quantifier queue, and **35.87 billion** in Go GC. On abstract Raft, corresponding rewriter costs were **34.96 versus 6.65 billion** and model-checker costs were **22.35 versus 4.85 billion**. These are estimates from three instruction-sampled runs per engine, with each sample assigned to at most one nearest category. A profile after the allocation fix is needed to quantify remaining GC cost. [Sample counts, periods and categories](perf/ivy_apple/holdouts/nov22b-pending-stacks.csv.gz)

### Conditional point updates on uninterpreted domains

The finite Raft wrapper and the abstract Raft specification have 419 checks each, but their domain semantics differ. The abstract specification permits an arbitrary number of nodes. Commit `d0907eb8e` admits uninterpreted source sorts to the existing direct universal-instance and conditional point-update proofs. Each inferred sort can use a copy of its source domain. An uninterpreted sort therefore has no assumed cardinality and cannot enter the two-element Boolean encoding. A private solver may discharge the source obligation only by establishing the required refutation. The release regression includes both a valid update proof and a satisfiable transition requiring at least three distinct values. [Proof construction](api/internal/order/boolean_problem.go) · [Semantic and cache regressions](api/api_order_instance_test.go)

| Complete GoIvy run | Checks | Prior g3 | Current g3 | Linked Z3 | Current g3 / linked Z3 |
|---|---:|---:|---:|---:|---:|
| Abstract Raft, `raft_no_assume.ivy` | 419 | 100.25 | 45.61 | 24.92 | 1.83× |
| `raftok.ivy` | 420 | 78.39 | 42.37 | 19.99 | 2.12× |
| Finite wrapper, `raft_no_assume_test.ivy` | 419 | 23.64 | 23.57 | 214.86 | 0.11× |
| Apple rfn2 | 26 | 19.10 | 19.09 | 54.59 | 0.35× |
| TLB | 18,953 | 213.68 | 213.76 | 77.63 | 2.75× |

Values are median whole-process retired user instructions in billions, with one pinned CPU per workload, GOMAXPROCS=1 and default GC. Abstract Raft and raftok each ran seven alternating triples, the wrapper five, rfn2 four and TLB three. All 123 complete process runs across nine workloads matched the reference status and output. CPU siblings ran other benchmark lanes, so use instruction counts for the revision comparison. [All process measurements](perf/ivy_apple/holdouts/nov22b-transition-runs.csv.gz)

The gain comes from discharging 58 additional obligations before the source solver is pushed in each of the two abstract Raft specifications. On abstract Raft, 1 of 419 checks previously completed before the push, and 59 now do. On raftok the count rose from 2 to 60. Across each entire specification's verification intervals, median solver-thread instructions fell from 98.78 to 44.24 billion and from 75.06 to 39.76 billion, respectively. Linked Z3 used 23.35 and 18.46 billion. Across five workloads, all 21,050 individual answers agreed with the ordinary linked solver in every repetition. A separate 181-spec corpus had matching exit status and stdout throughout. Two pre-existing mixin panics differed in hexadecimal addresses in stderr. The full API package and checked proof-path tests passed. [137,625 measured check intervals](perf/ivy_apple/holdouts/nov22b-transition-checks.csv.gz) · [Corpus comparisons](perf/ivy_apple/holdouts/nov22b-transition-corpus.csv.gz)

A proof batch checks for a positive universal premise with at least four binders, the shape the existing point-update matcher needs. It reuses that admission decision on the same manager and exact ordered prefix, including handle generations. This removed a 3.3-billion-instruction repeated scan on TLB. Requiring the quantified body itself to begin with implication improved abstract Raft to 40.12 billion but raised raftok to 53.53 billion in the same test. The release keeps the broader admission rule. [246 admission comparisons](perf/ivy_apple/holdouts/nov22b-transition-admission.csv.gz) · [Bounded scan and cache](api/internal/order/transition_premise.go)

### Matched search inside the numeric queue's model checker

The last of 15 checks in `doc/examples/cav2024/examp1_numeric.ivy` returns UNKNOWN in both engines after 14 successful checks. This example disables the EPR restriction and axiomatizes a recursive integer count. The complete g3 run takes about 596 billion instructions and the linked Z3 run about 212 billion. The first hardware sample attributed 512.7 billion g3 instructions and 180.3 billion Z3 instructions inclusively to model checking. These numbers include calls made by the auxiliary solver. The nearest named stack group alone cannot distinguish its search, rewriting and rollback. [g3 instruction sample](perf/ivy_apple/holdouts/nov22b-numeric-g3-first-profile.csv.gz) · [Z3 instruction sample](perf/ivy_apple/holdouts/nov22b-numeric-native-first-profile.csv.gz)

I instrumented the complete and restricted auxiliary checks separately. There were **21,002 calls and 7,085,640 decisions** inside the auxiliary solver. Two unary quantifiers accounted for **6,356,201 decisions (89.7%)**, 889,469 conflicts (87.0%) and 25,841,840 added equalities (93.0%). Their evaluated formulas have one free integer. A representative captured query contains 110 conditionals over that integer. Returning directly through the kernel removed the public solver's result assembly and left the complete run at roughly 596 billion instructions. [Per-call census](perf/ivy_apple/holdouts/nov22b-numeric-auxiliary-calls.csv.gz) · [Measurement and vector capture patch against d0907eb8e](perf/ivy_apple/holdouts/nov22b-numeric-auxiliary-observer.patch.gz)

I captured ten queries selected by decisions, elapsed time, conflicts, equalities and created Boolean variables. Six are satisfiable and four are unsatisfiable. I ran each query in a fresh shell and with a push before the first assertion, with three alternating pairs per route. The two engines agreed on the verdict **and on all four search counters in all 120 runs**. The median of the ten per-query instruction ratios was **3.53×** in fresh processes and **2.91×** with a push. The incremental ratios range from 2.74× to 3.19×. Each ratio includes process startup. One incremental query took 5,636 decisions and 411 conflicts in both engines but consumed roughly 323 million g3 instructions versus 107 million Z3 instructions. This reproduces the general solver's overhead independently of GoIvy, model construction and quantifier selection. [Ten captured inputs](perf/ivy_apple/test_vectors/numeric_auxiliary/) · [All paired results](perf/ivy_apple/holdouts/nov22b-numeric-auxiliary-replay.csv.gz)

Run `python3 perf/ivy_apple/holdouts/query_replay.py --suite numeric-auxiliary --g3 /path/to/g3 --native ../build/z3 --cpu 2 --pairs 3 --output /tmp/g3-numeric-replay` from `pkg`. The driver checks the expected answer, decisions, conflicts, added equalities and resource count on every run. [Replay driver](perf/ivy_apple/holdouts/query_replay.py)

### Fresh-name issuance and lexical lookup

GoIvy allocates each private fragment variable from one check-local `uint64` counter across assumptions, assertions and eligible macros. Issuing `V!feu!0`, `V!feu!1` and subsequent suffixes takes no lookup, lock or table of previously issued names. Each source variable occurrence still needs its declaration. For example, the two occurrences of `X` in `forall X. p(X, X)` must both receive the binder's new name. Under a nested binder also named `X`, the same spelling refers to the inner declaration until that binder ends. The original diagnostic and trace spellings can be reconstructed separately.

GoIvy commit `402ff7c31` adds `BenchmarkFragmentBindingsWidth` for bound variables, free variables and shadowing. Every use includes a distinct Go variable object with the original name and sort. Across six runs per case, a table-free slice scan cut a four-variable quantifier from 3,616 to 1,920 B/op. At 512 bound variables a linear scan took 3.82 ms, versus 1.16 ms with the existing map. Sorting the bound declarations gave 0.79 ms and 254,656 B/op, versus 522,352 B/op with the map. The sorted prototype still scans free variables linearly. A small slice switching to a typed name index gave 0.40 ms on 512 free variables. These timings were taken under concurrent benchmark load. [450 benchmark results](perf/ivy_apple/holdouts/nov22b-bindings-width.bench.gz) · [Linear scan](perf/ivy_apple/holdouts/nov22b-bindings-linear.patch.gz) · [Sorted scope](perf/ivy_apple/holdouts/nov22b-bindings-sorted.patch.gz) · [Typed index](perf/ivy_apple/holdouts/nov22b-bindings-buckets.patch.gz) · [Small slice and index](perf/ivy_apple/holdouts/nov22b-bindings-small-index.patch.gz)

On complete TLB checks, replacing the lexical map with a slice saved **14.25 MB out of 6,886.02 MB** in g3 and **11.62 MB out of 2,736.73 MB** with linked Z3. The corresponding g3 instruction medians were 212.73 and 212.48 billion. On Apple dramc_nb2 the g3 allocation saving was 29.40 MB out of 14,561.73 MB. All 120 uninstrumented runs and all 128 allocation comparisons preserved the expected verdicts and output. The four measured workloads support keeping the existing lexical lookup. The larger fresh-name gain comes from eliminating the search for unused suffixes. On TLB that change alone removed 136,961,124 suffix probes, including 136,939,296 collisions. [Complete runs](perf/ivy_apple/holdouts/nov22b-bindings-runs.csv.gz) · [Allocation measurements](perf/ivy_apple/holdouts/nov22b-bindings-memory.csv.gz) · [Earlier fresh-name probe counts](perf/ivy_apple/holdouts/nov22b-fragment-probes.csv.gz)

### Deciding one-variable integer model checks by their comparison boundaries

Commit `476ca633f` reduced complete verification of `doc/examples/cav2024/examp1_numeric.ivy` from **81.20 s to 13.86 s**. Linked Z3 took **30.92 s**. The six alternating triples ran on CPU 2 with the same GoIvy frontend, GOGC 100, one Go processor and hardware user-instruction counters. The machine also ran other benchmarks. All 18 processes completed with the same 14 successful checks and final UNKNOWN. The comparison expanded SMT-LIB lets when verifying the diagnostic query. [Paired runs](perf/ivy_apple/holdouts/nov22b-integer-regions-complete.csv.gz)

| Complete numeric queue | Retired instructions, billions | Wall time, s | Peak RSS, MiB |
|---|---:|---:|---:|
| Prior g3 | 597.41 | 81.20 | 244.85 |
| g3 with integer regions | 111.07 | 13.86 | 81.65 |
| Linked Z3 | 211.86 | 30.92 | 108.13 |

The expensive auxiliary questions ask whether an evaluated model body can be false at any integer. Within a branch of the model's nested conditionals, each supported integer term has the form `a*x+b`. A comparison can change truth at an integer root or on passing a root. The checker visits the floor of the root and the following integer, which preserves equality at a singleton and distinguishes strict from non-strict inequalities. Both infinite tails are covered. Coefficient, graph and work limits send undecided cases to the ordinary solver. No specification, quantifier name or known answer enters the decision. The solver retains its existing AST arena. [Integer region calculation](smtcore/smt_integer_regions.go) · [Model-checker entry point](smtcore/smt_model_integer.go)

Witness selection accounts for much of the speedup. The checker first tests values from the current instantiation set. If a value violates the body, it recovers the matching ground term and its generation through the ordinary model finder. When that fails, the interval sweep can produce a new numeral witness. A prototype that accepted arbitrary unrestricted witnesses first timed out after 500 s. Checking only the existing terms and sending every other question to the auxiliary solver cost about 375 billion instructions. Together, term recovery and the universal decision brought this workload below native Z3. The ordinary auxiliary solver still performs the 2.91×-cost matched searches measured above when given the ten standalone SMT inputs directly. [Witness policy experiments, including censored runs](perf/ivy_apple/holdouts/nov22b-integer-regions-variants.csv.gz)

For independent verification, a diagnostic build printed the original evaluated AST of every body the new path decided. Native Z3 found the negation unsatisfiable on **all 3,000 universal results**, without restrictions on the integer variable. It found the negation satisfiable **at each of the 2,999 exact witnesses**. A separate experiment applied the interval calculation at the ordinary auxiliary check and compared both decisions on its original search path. All **10,998** applicable queries agreed. The release also preserved results across the 181-file regression panel and 26 arithmetic-related files, including existing frontend failures. [Native replay results](perf/ivy_apple/holdouts/nov22b-integer-regions-oracle.csv.gz) · [Capture patch](perf/ivy_apple/holdouts/nov22b-integer-regions-capture.patch.gz) · [Ordinary auxiliary comparisons](perf/ivy_apple/holdouts/nov22b-integer-regions-auxiliary-oracle.csv.gz) · [Holdout comparisons](perf/ivy_apple/holdouts/nov22b-integer-regions-holdouts.csv.gz)

Run `python3 perf/ivy_apple/holdouts/integer_regions_oracle.py --native ../build/z3` to replay every captured body. The Go regression independently evaluates 200 generated formulas at 83 consecutive integers and checks every interval skipped between them. The benchmark uses 110 nested conditionals. In five pinned runs, deciding the compiled body took about 207 µs and zero allocations. Use `go test ./smtcore -run IntegerRegions -bench SMTIntegerRegions` to run both checks. [Native replay driver](perf/ivy_apple/holdouts/integer_regions_oracle.py) · [Replay inputs](perf/ivy_apple/test_vectors/numeric_auxiliary/integer-regions.jsonl.gz) · [Tests and benchmark](smtcore/smt_integer_regions_test.go) · [Benchmark samples](perf/ivy_apple/holdouts/nov22b-integer-regions-micro.bench.gz)

### Reusing ground instances across sibling verification conditions

On TLB, retaining the Boolean consequences of a batch’s quantified premises reduced whole-process instructions from **213.16 to 123.64 billion**. Linked Z3 used **77.69 billion**. The same change reduced Apple dramc_nb2 from **200.22 to 188.84 billion**, compared with **198.75 billion** for linked Z3. These medians compare g3 before and after `db3938f31` using the same GoIvy frontend. The eleven workload lanes used fixed CPUs, GOMAXPROCS 1 and GOGC 100. The host ran other experiments, so the instruction comparison is more reproducible than elapsed time. [All 138 runs](perf/ivy_apple/holdouts/nov22b-ground-premises-runs.csv.gz)

| Complete specification | Previous g3, billion instructions | Updated g3 | Linked Z3 |
|---|---:|---:|---:|
| TLB | 213.16 | 123.64 | 77.69 |
| Apple dramc_nb2 | 200.22 | 188.84 | 198.75 |
| Apple cf_live | 60.30 | 63.09 | 63.44 |
| Hybrid reliable broadcast | 16.01 | 16.80 | 6.45 |
| Flash2 | 11.04 | 10.80 | 4.57 |
| Abstract Raft | 46.12 | 46.02 | 24.91 |
| raftok | 42.91 | 42.87 | 19.98 |
| Apple rfn2 | 19.07 | 19.06 | 54.56 |
| Numeric queue | 110.23 | 110.38 | 211.86 |

A solver batch builds a bounded set of ground instances, choosing one term per sort. An asserted universal entails each instance. For an existential, an instance implies the existential. The first goal can supply substitution terms. Only the shared premises enter the saved Boolean state. For each sibling the checker adds the literals implied by that goal, propagates them, and undoes its own assignments. A conflict proves UNSAT. A miss continues through the existing solver. Reuse requires the same manager and ordered list of premise handles including generations. A 32-link implication benchmark took a median **77.4 µs and 63,032 bytes** to rebuild for each goal, or **549 ns and zero allocations** with reuse. [Implementation](api/internal/order/ground_premises.go) · [Regression tests and benchmark](api/api_ground_premises_test.go) · [Five microbenchmark repetitions](perf/ivy_apple/holdouts/nov22b-ground-premises-micro.bench.gz)

Much of the TLB gain occurred *after* the newly discharged goals. The 168 checks moved before Push fell from 19.79 to 1.87 billion solver-thread instructions. The 18,667 checks that still ran in scopes fell from 145.74 to 83.62 billion. Conversely, on cf_live the 63 checks moved before Push fell from 15.20 to 1.72 billion, while the remaining 271 scoped checks rose from 19.91 to 35.94 billion. On dramc_nb2 1,899 checks moved before Push and 557 stayed in scopes. Counting additional early proofs alone would misstate the gains on all three specifications. Across seven workloads, the prior, updated and native checkers agreed on all **25,265** answers in both repetitions. [Per-check measurements](perf/ivy_apple/holdouts/nov22b-ground-premises-checks.csv.gz) · [Stage totals](perf/ivy_apple/holdouts/nov22b-ground-premises-stages.csv.gz)

A fresh linked-Z3 solver independently returned UNSAT for **all 2,251** checks newly completed before Push. It reasserted the original typed terms after resetting its state for each check. This covered 1,899 dramc_nb2, 168 TLB, 121 hybrid and 63 cf_live checks. Text replay confirmed 193 of 306 sampled queries. The native printer gave overloaded constants ambiguous names in the other 113 exports, so those files could not validate the result. Reasserting the original ASTs preserves declaration identity. A separate 181-spec corpus retained every exit status and stdout transcript. Two pre-existing mixin panic reports differed only in addresses. [Independent typed replays](perf/ivy_apple/holdouts/nov22b-ground-premises-ast-oracle.csv.gz) · [Executable replay driver](perf/ivy_apple/holdouts/ground_premise_replay.py) · [Corpus results](perf/ivy_apple/holdouts/nov22b-ground-premises-corpus.csv.gz)

Projecting a quantified arithmetic variable into opaque Boolean atoms added about **50 billion instructions** on the numeric queue. The release leaves that case to the integer solver. Requiring every binder to be uninterpreted also excluded useful dramc_nb2 proofs. Nine admission variants checked these tradeoffs on seven workloads. [Admission trials](perf/ivy_apple/holdouts/nov22b-ground-premises-admission.csv.gz)

### Preserving ordinary solver models while accelerating verification

The public shell’s exact-model comparisons exposed five SAT cases where the new integer witness order changed the printed model. On the saved toy-consensus obligation, it also increased decisions from 1,866 to 2,612. Commit `8f8fcf180` enables comparison-region witnesses for incremental verification clients and private refutation contexts. The ordinary solver again follows the source search order. All shell script fixtures passed. Across thirteen captured inputs, five alternating incremental pairs matched linked Z3 on every answer and every recorded search counter. [130 replay measurements](perf/ivy_apple/holdouts/nov22b-integer-regions-scope-replay.csv.gz)

GoIvy’s numeric queue retained the speedup under that policy. Three rotating triples took medians of **13.82 s before the policy change, 13.09 s after, and 29.87 s with linked Z3**. Whole-process instructions were **110.99, 111.12 and 211.86 billion**. All fourteen successful checks and the final UNKNOWN agreed. [Complete numeric, toy-consensus and raftok comparisons](perf/ivy_apple/holdouts/nov22b-integer-regions-scope-runs.csv.gz)

### Flattening clause conjunctions in the shared GoIvy frontend

Both backends allocated **1.71 GB less Go heap on dramc_nb2** and **0.88 GB less on TLB** after GoIvy commit `6d847b88d`. The previous clause constructor built a slice for each nested conjunction and repeatedly copied its children up the tree. The new constructor appends the leaves in source order to one owned result. It preserves the existing quantifier boundaries, definition order and last-definition lookup. On flat input it also avoids intermediate copies when joining clause groups. [GoIvy implementation and benchmark](https://github.com/glycerine/ivy/commit/6d847b88d) · [96 allocation measurements](perf/ivy_apple/holdouts/nov22b-clause-conjunction-memory.csv.gz)

| Complete specification | g3 before → after, billion instructions | Linked Z3 before → after, billion instructions |
|---|---:|---:|
| Apple dramc_nb2 | 188.17 → 179.33 | 198.60 → 186.40 |
| TLB | 123.82 → 117.93 | 77.69 → 71.49 |
| Apple cf_live | 62.99 → 61.61 | 63.40 → 61.67 |
| Apple rfn2 | 19.01 → 17.99 | 54.62 → 53.43 |

The instruction panel used four alternating pairs per backend on each of eight specifications, with a fixed CPU per workload, GOMAXPROCS 1 and GOGC 100. Both frontend revisions used the same solver. Every exit status and proof outcome matched. A separate 181-spec comparison per backend preserved stdout and exit status. Stderr matched byte for byte for 179 inputs per backend. Two existing panic backtraces differed only in addresses. Three active-trace controls, including the 2.33 GB rfn2 trace, preserved both output streams byte for byte. [128 complete runs](perf/ivy_apple/holdouts/nov22b-clause-conjunction-runs.csv.gz) · [362 corpus comparisons](perf/ivy_apple/holdouts/nov22b-clause-conjunction-corpus.csv.gz) · [210 prototype runs](perf/ivy_apple/holdouts/nov22b-clause-conjunction-variants.csv.gz)

On a nested 65-formula conjunction the median fell from 19.61 to 1.58 µs, 37,696 to 2,288 B/op and 67 to 9 allocations. A flat 128-formula input fell from 3.16 to 1.39 µs, and joining eight 16-formula inputs fell from 5.14 to 1.57 µs. Nine alternating pairs used 50,000 iterations per case on CPU 46 with one Go processor. Run `go test . -tags=xtrace_off -run '^$' -bench '^BenchmarkClauseConjunction$' -benchtime=50000x -benchmem` in GoIvy. [54 microbenchmark samples](perf/ivy_apple/holdouts/nov22b-clause-conjunction-micro.csv.gz)

### How much of the threefold incremental gap survives solver reuse?

On ten numeric queries, **each additional push, assertion, check and pop cost g3 3.00–3.20× as many instructions as Z3**. Each process loaded the declarations and base assertions once, then checked the same scoped goal 1, 8 or 64 times. The estimate is the median of six paired differences between the 64- and 8-check totals, divided by 56. All ten queries matched on the answer, decisions, added equalities, conflicts and cumulative resource count at every iteration. With Go garbage collection disabled, the ratio was still 2.76–2.87×. Disabling collection changes allocation and memory usage as well as GC work. [936 measured processes](perf/ivy_apple/holdouts/nov22b-warm-query-runs.csv.gz) · [3,328 per-engine search observations](perf/ivy_apple/holdouts/nov22b-warm-query-search.csv.gz)

| Saved query | g3, millions of instructions per additional check | Z3 | g3 with GC off | Checks with all four counters equal |
|---|---:|---:|---:|---:|
| Numeric aux-012725 | 107.1 | 35.6 | 98.5 | 64/64 |
| Numeric aux-011440 | 208.5 | 68.5 | 191.2 | 64/64 |
| Ticket 127 | 120.0 | 33.7 | 106.4 | 64/64 |
| Ticket 290 | 209.0 | 57.0 | 192.4 | 64/64 |
| Toy consensus 18 | 1,675.0 | 505.9 | 1,633.9 | 45/64 |

All 64 toy-consensus answers agreed. Its search counters diverged on iteration 46, so the full 56-check slope includes different search work. The two ticket slopes held at 3.56× and 3.66× under matched search. The timed runs print just the answers. A separate run records the cumulative search statistics at each iteration. Run the saved benchmark with `warm_queries.py --g3 … --native … --output … --cpu N --pairs 6 --iterations 1 8 64`. [Benchmark source](perf/ivy_apple/holdouts/warm_queries.py)

A separate instruction-sampling experiment classified each stack by its nearest recognized operation in twenty fresh runs per engine and input. On aux-012725, the estimate for arithmetic was 26.6 million instructions in g3 against 10.2 million in Z3, Boolean propagation 17.3 against 4.6, relevance 28.4 against 15.5, congruence lookup 15.6 against 4.4 and rewriting 14.1 against 2.0. Go GC accounted for 47.6 million and solver construction for 39.9 million against 17.8 million. Native allocation is charged to its enclosing operation unless it matches a separate rule. These samples cover the entire one-check process, including startup, teardown and statistics output. The warm slopes above measure the recurring cost independently. [Disjoint sample counts](perf/ivy_apple/holdouts/nov22b-matched-stack-categories.csv.gz) · [Classification rules](perf/ivy_apple/holdouts/query_stack_groups.csv) · [Classifier](perf/ivy_apple/holdouts/matched_stack_counts.py)

Toy consensus has a more concentrated opportunity. On its first, matched check, rewriting used 327.8 million instructions in g3 against 74.6 million in Z3. The two ticket queries spent 75.1 million and 88.6 million excess instructions in rewriting. Solver reuse and the arithmetic integer-region proof both leave this ordinary search path available for controlled comparisons. Reproduce the profiles with `query_stacks.py --queries empty aux-012725 ticket-127 ticket-290 toy-consensus-18 --statistics --pairs 20 --period 1000003` and the same binary, CPU and output options as the warm benchmark. [Sampling driver](perf/ivy_apple/holdouts/query_stacks.py)

## Sharing forbidden macro declarations on scope pop

Z3's macro manager and macro utility consult the same set of forbidden declarations. g3 had been sorting the manager's keys, materializing a slice and constructing a second lookup table on every pop. Commit `37dc00a96` lets both readers consult the solver-local set directly. The manager takes a snapshot only on a partial failure that could otherwise expose unpublished changes. Nine alternating pairs of 12,000 pops each took a median 6,104 to 16.10 ns for 64 declarations and 65,830 to 15.98 ns for 512. The 512-declaration pop went from 35,544 bytes and five allocations to zero. [In-tree native manager](../src/ast/macros/macro_manager.cpp) · [Go benchmark and error-path tests](ast/macros/macro_forbidden_test.go) · [All 36 benchmark rows](perf/ivy_apple/holdouts/nov22b-macro-forbidden-micro.csv.gz)

On a complete TLB run the median fell from 117.58 to 113.75 billion instructions in three alternating pairs. In a separate five-pair heap measurement, cumulative allocations fell from 4,867.10 to 4,638.75 MB, a saving of 228.35 MB and about 295,000 objects per run. The nine other whole-proof controls changed by less than 1.4% in either direction. All ten specifications retained their reference outcomes. The two Go versions and in-tree Z3 also agreed at both GC settings on the answer and four search counters over 13 saved incremental queries. [60 complete runs](perf/ivy_apple/holdouts/nov22b-macro-forbidden-runs.csv.gz) · [40 heap measurements](perf/ivy_apple/holdouts/nov22b-macro-forbidden-memory.csv.gz) · [780 replay measurements](perf/ivy_apple/holdouts/nov22b-macro-forbidden-replay.csv.gz)

## What remains on the first physical push

The timing boundary is the actual call to the incremental solver from GoIvy, after any pre-push proof attempt. The table gives median solver-thread instructions in billions across four alternating pairs. “Later checks” counts verification calls in the second and subsequent physical scopes of each batch. The counter excludes work in other threads and subtracts an empty measurement. The whole-process panel also includes formula generation, pre-push proofs, independent checks and cleanup. [Measurement driver](perf/ivy_apple/incremental_cost/batch_operations.py) · [Totals by operation](perf/ivy_apple/incremental_cost/nov22b-batch-operations.csv.gz) · [Individual first pushes](perf/ivy_apple/incremental_cost/nov22b-first-physical-pushes.csv.gz)

| Specification | First pushes, g3 / Z3 | First push, g3 / Z3 (B) | Later checks, g3 / Z3 (B) |
|---|---:|---:|---:|
| TLB | 58 / 58 | 23.85 / 5.92 | 49.23 / 36.07 |
| Apple cf_live | 9 / 12 | 14.20 / 5.35 | 17.53 / 33.32 |
| Apple dramc_nb2 | 17 / 86 | 15.02 / 9.46 | 16.30 / 14.92 |
| Apple rfn2 | 0 / 10 | 0 / 3.53 | 0 / 0.60 |
| Abstract Raft | 12 / 14 | 0.87 / 0.25 | 42.32 / 23.06 |
| raftok | 12 / 15 | 0.86 / 0.27 | 39.27 / 18.16 |

The original 8.61-billion-instruction first-push penalty on rfn2 is gone. All 26 obligations finish before a physical push. TLB now offers the largest measured first-push gap here, 17.93 billion instructions across 58 batches. The dominant excess on both abstract Raft specifications comes later, inside the verification calls. Choosing those boundaries separately gives different optimization targets for TLB and Raft.

## Renaming the formula after choosing the fresh name

The private suffix counter resolved name selection. Each subsequent symbol substitution still cloned the formula. The common fixed-arity GoIvy nodes first built an argument slice and then unpacked it into the clone. Applications copied an already private slice a second time. GoIvy commit `59aa2840e` clones fixed-arity fields directly and gives each new application its own argument storage. Binder variables retain their original sharing. Replacing a function by one with a different range, arity or sort follows the existing application constructor. [GoIvy implementation and regression benchmark](https://github.com/glycerine/ivy/commit/59aa2840e)

| Measure | Before | After |
|---|---:|---:|
| Rename one quantified predicate, ns/op | 5,803 | 2,942 |
| Bytes / allocations per operation | 5,504 / 117 | 3,680 / 51 |
| Apple dramc_nb2 whole-process instructions, g3 | 179.02 B | 167.77 B |
| Apple dramc_nb2 whole-process instructions, linked Z3 | 186.53 B | 173.07 B |
| Go heap allocation over dramc_nb2, g3 | 12.83 GB | 12.21 GB |
| Go heap allocation over dramc_nb2, linked Z3 | 11.23 GB | 10.54 GB |

The microbenchmark used nine alternating pairs of 30,000 iterations. Six complete specifications used five pairs per backend. Heap profiles were sampled separately in four pairs and report cumulative Go allocations in decimal GB. The g3 median fell by 25.9 million allocated objects on dramc_nb2. All 87 reference proof transcripts matched for both backends. The active-tracing builds emitted identical stdout and stderr for ring and rfn2, including 2,334,030,415 stdout bytes on rfn2. [Microbenchmark samples](perf/ivy_apple/holdouts/nov22b-rename-quantified-micro.csv.gz) · [Complete runs](perf/ivy_apple/holdouts/nov22b-rename-quantified-runs.csv.gz) · [Heap profiles by run](perf/ivy_apple/holdouts/nov22b-rename-quantified-memory.csv.gz) · [Corpus outcomes](perf/ivy_apple/holdouts/nov22b-rename-quantified-corpus.csv.gz)

I also tried allocating cloned GoIvy nodes in type-specific blocks shared across a batch. On dramc_nb2 the best block-growth prototype used 163.65 B instructions versus 167.58 B for direct cloning in the same experiment. It requires carrying a new storage owner through the traversal. Direct cloning delivered the larger independently measured gain and has a simpler ownership rule. These experiments concern GoIvy formula construction. The g3 solver continues to use its arena. [Storage variants](perf/ivy_apple/holdouts/nov22b-rename-storage-layouts.csv.gz)

## Which lower-level changes affect matched replay?

The saved incremental queries isolate a single push, assertion and check inside a fresh executable. Ten numeric auxiliary queries and three Ivy queries ran at default GC and with GC off. Each of the 104 groups (study, query and GC setting) returned the same answer and four search counters across every implementation and the in-tree Z3 control. The table gives the median of the per-query instruction ratios to the contemporaneous g3 control, with GC off. The earlier 8-to-64-check slope separately measures warmed, repeated work. [7,800 individual replays](perf/ivy_apple/holdouts/nov22b-internal-layout-replay.csv.gz) · [Repeated-check benchmark](perf/ivy_apple/holdouts/warm_queries.py)

| Prototype | Numeric | Three Ivy queries | Complete-run observation |
|---|---:|---:|---|
| Tagged equality reasons plus a compact assignment trail | 0.998 | 1.000 | TLB 117.38 → 117.45 B instructions |
| Lazy model-finder sets and direct iteration | 0.999 | 1.000 | TLB 117.78 → 117.74 B |
| Faster arithmetic comparison and weakly cached numeral results | 0.980 | 0.999 | Numeric example 110.84 → 105.60 B |
| Transfer rewrite-result ownership and specialize child publication | 0.999 | 0.997 | TLB 113.22 → 112.73 B |

The 16-byte Boolean record and its 8-byte Boolean justification are already in the retained solver. The first row experiments with the separate equality-explanation and undo records. The arithmetic experiment gives a reproducible saving on numeric input. Its best configuration recovers about 2% on isolated numeric replay and leaves the two ticket obligations almost unchanged. None of these changes explains the 2.6–3.2× repeated-check ratio. The stronger target is eliminating entire repeated evaluations, especially when the model or the unchanged assertion prefix already determines the answer. [Complete-run measurements](perf/ivy_apple/holdouts/nov22b-internal-layout-runs.csv.gz) · [Prototype patches against g3 a7c09920a](perf/ivy_apple/holdouts/nov22b-internal-layout-prototypes.tar.gz)

## Checking quantified formulas against the candidate model

In model-based quantifier instantiation, the solver asks whether its candidate model has an assignment that makes a quantified body false. Setting up an auxiliary SMT search for each question was expensive on both abstract Raft examples. Commit `83bfb17e3` evaluates small, explicitly interpreted function tables, equalities and Boolean operations before setting up that search. This path uses the actual formula and model supplied to the verification solver. It requires no knowledge of an Ivy spec, action or predicate name. [Implementation](smtcore/smt_model_table_check.go) · [Ground-model comparison tests](smtcore/smt_model_table_test.go)

For a frozen uninterpreted sort the checker visits its registered universe. For an open sort it includes every named value used in the supported tables and adds one anonymous representative per binder of that sort. For example, two variables can denote the same unseen value or two different unseen values. Both assignments are represented. Any permutation of unseen elements preserves equality and the compiled function tables. The checker can certify a body only after covering every tuple. It can submit a false tuple for instantiation only if it has concrete values and the ordinary model finder can recover all the original bindings. Unsupported models, large products, exhausted evaluation budgets and mixed internalized arithmetic use the ordinary solver. The feature is enabled in the incremental verification factory.

| Complete verification, median billions of instructions | Before | After |
|---|---:|---:|
| Abstract Raft | 45.93 | 31.45 |
| raftok | 42.86 | 31.61 |
| Hybrid reliable broadcast | 16.61 | 14.25 |
| Flash | 10.84 | 9.43 |
| Nested ticket | 5.20 | 4.98 |
| TLB | 112.94 | 111.72 |
| Apple dramc_nb2 | 167.40 | 167.32 |
| Apple rfn2 | 17.31 | 17.31 |

Four alternating pairs per workload used the same GoIvy build and pinned CPU. A separate run of 87 standalone proofs produced byte-identical stdout and stderr for g3 and linked Z3. The corpus includes expected proof failures. [Complete measurements on 11 workloads](perf/ivy_apple/holdouts/nov22b-model-table-whole.csv.gz) · [Corpus outcomes and exit codes](perf/ivy_apple/holdouts/nov22b-model-table-corpus.csv.gz)

In a separate instrumented run, the ordinary auxiliary solver also searched all 36,800 questions certified by the new checker. It returned UNSAT every time. After deduplicating within each workload, the in-tree C++ Z3 executable returned UNSAT on all 5,206 saved SMT-LIB questions. The existing independent model evaluator confirmed that each of the 3,132 concrete witness tuples made its body false. The archive includes the complete typed questions, including explicit distinctness constraints for printed model values. [Audit totals](perf/ivy_apple/holdouts/nov22b-model-table-audit.csv.gz) · [Native answers](perf/ivy_apple/holdouts/nov22b-model-table-native.csv.gz) · [SMT-LIB questions](perf/ivy_apple/holdouts/nov22b-model-table-questions.tar.gz) · [Audit instrumentation](perf/ivy_apple/holdouts/model_tables_audit.patch.gz) · [Capture and replay driver](perf/ivy_apple/holdouts/model_tables_replay.py)

## Preprocessing on the first physical push

Commit `a94d21dd9` runs one macro search and simplification sequence when an incremental verification solver first commits a nonempty base. The original schedule ran a second macro finder and final reduction in that same push. Later batches still expand definitions removed by the first macro search. The option is consumed even if the first batch consists entirely of definitions and leaves zero asserted rows. The ordinary shell, proof-producing configurations, explicit resource limits and source debugging use their original schedule. The regression checks the retained definition in a generated model and across a subsequent push and pop. [First-push implementation](smtcore/smt_kernel.go) · [Regression](smtcore/smt_asserted_macros_test.go)

The table reports the median sum of instructions in the first physical push of each fresh verification batch, in billions. Each row has three runs per build. The thread counter subtracts an empty measurement and each observation ran without multiplexing. A batch that proves every goal in the precheck contributes zero physical pushes. [Individual pushes](perf/ivy_apple/holdouts/nov22b-first-schedule-pushes.csv.gz) · [All operation totals](perf/ivy_apple/holdouts/nov22b-first-schedule-batches.csv.gz) · [Reproduction driver](perf/ivy_apple/incremental_cost/batch_operations.py)

| Workload | First pushes, g3 / Z3 | Prior g3 | Current g3 | Linked Z3 |
|---|---:|---:|---:|---:|
| TLB | 58 / 58 | 24.00 | 20.39 | 5.92 |
| Apple cf_live | 9 / 12 | 14.22 | 13.25 | 5.35 |
| Apple dramc_nb2 | 17 / 86 | 14.90 | 13.69 | 9.46 |
| Apple rfn2 | 0 / 10 | 0 | 0 | 3.53 |
| Flash | 28 / 28 | 1.44 | 1.27 | 0.35 |
| Abstract Raft | 12 / 14 | 0.87 | 0.79 | 0.25 |
| raftok | 12 / 15 | 0.88 | 0.76 | 0.27 |
| Nested ticket | 14 / 14 | 0.72 | 0.64 | 0.18 |

The four-pair end-to-end comparison on TLB went from 111.72 to 108.06 B instructions. Most of that saving was inside the first pushes. On raftok the whole run went from 31.60 to 24.79 B. Its pushes saved 0.12 B and its subsequent batched verification calls fell from 28.61 to 22.03 B. Changing the first normalization also changed search work. Abstract Raft paid a small price for that choice, rising from 31.75 to 32.19 B. All eleven complete workloads matched the reference outcomes, and the 87 standalone g3 and linked-Z3 transcripts matched byte-for-byte. [Whole processes](perf/ivy_apple/holdouts/nov22b-first-schedule-whole.csv.gz) · [Independent corpus](perf/ivy_apple/holdouts/nov22b-first-schedule-corpus.csv.gz)

I also compared 510 runs that omitted individual passes or checked whether an earlier pass had already processed the identical batch. Omitting pattern inference took TLB from about 112 B to 626 B instructions in the completed run. Omitting the initial macro search caused timeouts on both abstract Raft cases. Fourteen runs that omitted negation-normal-form processing exited with errors. These observations explain why the retained schedule keeps the first discovery pass, NNF and pattern inference. [All variants, exits and reference comparisons](perf/ivy_apple/holdouts/nov22b-first-schedule-ablations.csv.gz)

## Where each additional matched check spends instructions

I sampled 4 and 32 consecutive push/assert/check/pop operations in the same shell, then subtracted the shorter profile and divided by 28. A separate statistics run verified every answer and all four cumulative search counters at each of the 32 iterations for all five inputs. The shorter horizon also keeps toy consensus inside its matched-search interval. The following figures are median estimates in **millions of instructions per additional check**, with each cell showing g3 / in-tree Z3. Six pairs per query and five on toy consensus gave 116 sampled processes. A full matched check includes parsing and popping the scoped goal. The command column attributes samples with a visible check-sat caller. [Sample counts and caller classifications](perf/ivy_apple/holdouts/nov22b-warm-profiles.csv.gz) · [160 matched iterations](perf/ivy_apple/holdouts/nov22b-warm-profiles-search.csv.gz) · [Sampling benchmark](perf/ivy_apple/holdouts/warm_stack_profile.py)

| Saved input | All instructions | Inside check-sat | Rewriter | Rewriter called from quantifiers |
|---|---:|---:|---:|---:|
| Ticket 127 | 111.1 / 31.6 | 106.9 / 30.9 | 29.5 / 9.7 | 25.1 / 8.8 |
| Ticket 290 | 202.8 / 53.5 | 198.1 / 53.3 | 62.9 / 16.5 | 37.1 / 10.6 |
| Numeric aux-012725 | 109.2 / 35.6 | 96.0 / 32.2 | 13.1 / 2.2 | — |
| Numeric aux-011440 | 225.1 / 72.4 | 208.1 / 67.5 | 17.3 / 2.6 | — |
| Toy consensus 18 | 1,693.5 / 471.2 | 1,666.9 / 469.3 | 738.7 / 163.1 | 193.2 / 44.8 |

On ticket 290, the rewriter accounts for 46.4 M of the 149.3 M excess instructions. Internalization outside the rewriter adds 25.1 M, quantifier processing outside the rewriter adds 24.0 M and Go GC about 19.4 M. On toy consensus, rewriting accounts for 575.6 M of the 1,222.3 M excess. The portions of that rewrite gap called from internalization, model evaluation and quantifiers are approximately 194.9 M, 154.2 M and 148.4 M. The caller columns subdivide rewriting and overlap the total. Numeric aux-011440 has a more distributed cost, including about 17.7 M extra relevance instructions, 16.6 M in arithmetic, 14.7 M in rewriting and 12.4 M in congruence lookup. Sampling gives estimates, and medians of categories need not sum exactly to the median total. [Classifier rules](perf/ivy_apple/holdouts/query_stack_groups.csv)

## Complete verification and scope benchmarks at the end of the study

The two uninstrumented GoIvy executables used frontend `59aa2840e`, g3 `a94d21dd9` through the local Go module replacement, and the linked C++ fork in this repository. Builds used Go PGO off and tracing compiled off. Within each pair both processes used the same pinned CPU, one Go CPU and the same input. The order alternated. Each entry below is a median of complete processes. Instructions cover both the frontend and the solver. The controls check individual printed verdicts, stderr, and exit status. The numeric example has 14 passes and one inconclusive result. Its printed diagnostic SMT commands match after expanding lets. TLB includes two expected failures and Flash includes 65. [All 306 complete runs](perf/ivy_apple/holdouts/nov22b-final-verification.csv.gz) · [Paired benchmark](perf/ivy_apple/holdouts/paired_verification.py)

| Workload | Pairs | Wall seconds, g3 / Z3 | Billion instructions, g3 / Z3 | Peak MiB, g3 / Z3 | g3 wall wins |
|---|---:|---:|---:|---:|---:|
| Finite-domain Raft wrapper | 7 | 3.243 / 36.250 | 23.57 / 214.80 | 60.4 / 313.8 | 7 |
| Abstract Raft | 7 | 4.249 / 3.531 | 32.14 / 24.85 | 74.9 / 58.0 | 0 |
| raftok | 7 | 3.331 / 2.861 | 24.98 / 19.93 | 66.9 / 58.9 | 0 |
| rfn2 | 7 | 2.437 / 11.763 | 17.31 / 52.71 | 103.4 / 131.9 | 7 |
| tlb | 7 | 14.670 / 10.322 | 108.14 / 70.98 | 125.4 / 141.0 | 0 |
| cf_live | 7 | 8.251 / 10.753 | 59.23 / 60.35 | 229.0 / 146.6 | 7 |
| dramc_nb2 | 7 | 23.488 / 25.634 | 165.96 / 172.83 | 214.9 / 212.2 | 7 |
| Numeric example | 7 | 14.371 / 31.701 | 111.05 / 211.79 | 82.1 / 109.6 | 7 |
| Flash | 7 | 1.394 / 0.710 | 9.16 / 4.55 | 47.9 / 42.1 | 0 |
| Hybrid reliable broadcast | 7 | 2.056 / 0.973 | 14.01 / 6.31 | 57.8 / 51.4 | 0 |
| toy-consensus | 9 | 0.565 / 0.501 | 3.55 / 3.52 | 49.5 / 50.8 | 0 |
| ticket | 9 | 0.628 / 0.354 | 4.25 / 2.25 | 49.8 / 41.1 | 0 |
| progress | 9 | 0.112 / 0.120 | 0.59 / 0.71 | 40.8 / 36.3 | 9 |
| ping-pong | 9 | 0.048 / 0.084 | 0.14 / 0.48 | 33.4 / 35.2 | 9 |
| leader-ring | 9 | 0.098 / 0.085 | 0.56 / 0.42 | 36.7 / 34.5 | 0 |
| election-btw | 9 | 0.116 / 0.095 | 0.68 / 0.48 | 37.2 / 34.8 | 0 |
| ring-bug | 9 | 0.044 / 0.049 | 0.13 / 0.17 | 33.1 / 33.7 | 8 |

One additional pair on each of 87 standalone specs matched stdout and stderr byte for byte and reproduced the exit status. g3 used fewer instructions on 51 and less elapsed time on 53. These single-pair counts describe the selected corpus. The repeated panel above gives stronger evidence for a particular workload. [Standalone proof list](perf/ivy_apple/holdouts/nov22b-verification-cases.txt) · [174 measurements](perf/ivy_apple/holdouts/nov22b-final-corpus.csv.gz) · [Corpus driver](perf/ivy_apple/holdouts/spec_survey.py)

For the high-memory dramc_nb2 comparison I first measured five repeats at each of four neighboring GC settings per engine. I selected the median RSS closest to the earlier 467 MiB target independently for each checker. The resulting settings were GOGC=415 for g3 and 565 for linked Z3. Eleven fresh pairs on CPU 44 then measured **476.6 / 479.3 MiB**, **130.93 / 122.17 billion instructions** and **24.802 / 24.513 s**. The median of the eleven paired wall ratios was 1.020. At the default GOGC=100, the seven primary pairs on CPU 23 measured **214.9 / 212.2 MiB**, with g3 faster in all seven. A second nine-pair default panel on CPU 24 measured 218.6 / 199.2 MiB and 23.060 / 24.948 s, with g3 faster in all nine. RSS is process peak, including C++ allocations. The panels used different cores on a shared host. [Calibration samples](perf/ivy_apple/holdouts/nov22b-final-rss-calibration.csv.gz) · [Independent measurement pairs](perf/ivy_apple/holdouts/nov22b-final-verification.csv.gz)

I ran the repository's exact `make bench` and `make cppbench` targets in alternating order for 25 pairs, pinned to CPU 45 with GOMAXPROCS=1 and GOGC=100. For 128 atoms and 64 clauses the median surviving-atom pop took **361 ns in Go and 482 ns in C++**. The scoped-atom pop took **13,704 ns and 12,611 ns**, an 8.7% gap. Both Go medians were zero allocations per operation. These microseconds measure a single scope restoration. Whole verification above includes all frontend and search work. [All 100 benchmark rows](perf/scope_restoration/nov22b-final-make-targets.csv) · [Make target driver](perf/scope_restoration/make_targets.py) · [Independent-worker benchmark](smtcore/smt_scope_workers_benchmark_test.go)

## Why the larger heap changes the Apple comparison

Nine additional pairs at each GC setting on **the same CPU 45** measured Go runtime activity with identical dramc_nb2 proof output. With default GC, the Go part of the linked-Z3 process spent 5.38 CPU seconds collecting garbage, including 3.73 seconds of allocation assists. g3 spent 3.60 and 1.71 seconds respectively. With the larger heaps both spent about 0.8 CPU seconds collecting and less than 0.2 seconds in assists. Raising the target reduced linked-checker instructions by roughly **50 billion** and g3 instructions by **34 billion**. The GoIvy frontend is present in both processes. Total Go-allocated bytes were approximately 10.5 GB for linked Z3 and 12.2 GB for g3 in either regime. These numbers exclude native heap allocations. The independent uninstrumented high-memory pair medians were within 3 MiB. [36 runtime measurements](perf/ivy_apple/holdouts/nov22b-final-heap.csv.gz) · [Heap instrumentation](perf/ivy_apple/frontend_memory.py)

| Settings, g3 / linked Z3 | Wall s | Billion instructions | Peak MiB | GC cycles | GC CPU seconds |
|---|---:|---:|---:|---:|---:|
| GOGC 100 / 100 | 23.05 / 24.57 | 165.90 / 172.76 | 215.7 / 202.8 | 353 / 580 | 3.60 / 5.38 |
| GOGC 415 / 565 | 18.54 / 18.05 | 131.57 / 122.71 | 478.5 / 459.8 | 72 / 77 | 0.82 / 0.80 |

## Can consecutive verification batches keep their common premises?

The instrumented GoIvy checker captured each first physical push, subsequent permanent assumptions, and every temporary physical goal and its result, in order. On TLB the 58 fresh solver batches contained 31,665 top-level conjuncts in total. Only 1,281 conjunct positions belonged to an identical leading prefix with the previous batch, while 24,514 matched the previous batch as a multiset. No consecutive complete TLB bases were identical. The verifier used one AST context. The replay keeps one solver on one worker and pops every retained scope as soon as any of its premises disappears. Compound goals retain their original Boolean structure. Printed overloads are resolved from the declaration sorts before combining batches. [Seven typed input streams](perf/ivy_apple/holdouts/nov22b-physical-streams.tar.gz) · [Capture instrumentation against GoIvy 59aa2840e](perf/ivy_apple/holdouts/nov22b-capture-physical-stream.patch.gz) · [Collector](perf/ivy_apple/holdouts/capture_batch_stream.py)

In the initial TLB experiment, keeping the common intersection of four **upcoming** batches cut g3 first-push-only replay from 20.03 to 11.62 billion instructions. Including the first three physical goals of each batch, 172 checks in total, took 23.61 billion separately and 15.68 billion with common scopes. The resulting schedule has access to future batches. Reordering alone took 24.49 billion on the 172 checks. [First-goal and push controls](perf/ivy_apple/holdouts/nov22b-tlb-first-goals.csv.gz) · [Exact scripts and expected answers](perf/ivy_apple/holdouts/nov22b-tlb-first-goal-scripts.tar.gz)

The complete replay below includes ALL physical checks, including satisfiable counterexamples. The online policy intersects the current base with the two preceding bases and promotes matching conjuncts in order, in scopes of up to 64. A second control sees groups of four future bases. These are medians of three runs per engine and policy, in **billions of instructions for the solver executable**. Parsing and solver setup are included. GoIvy's higher-level proof methods are absent, and the saved stream starts after successful pre-push proofs. The 270 completed measurements reproduced all recorded answers. The two ordinary shell Raft streams repeatedly reached the 180-second cutoff. Their partial counts and exits are included in the raw file. The complete GoIvy Raft times appear above. [All runs and timeouts](perf/ivy_apple/holdouts/nov22b-batch-stream-runs.csv.gz) · [Common-scope counts](perf/ivy_apple/holdouts/nov22b-batch-stream-groups.csv.gz) · [Replay generator](perf/ivy_apple/holdouts/batch_prefix_replay.py)

| Workload | Physical checks | Separate batches, g3 / Z3 | Two earlier bases, g3 / Z3 | Four-batch lookahead, g3 / Z3 |
|---|---:|---:|---:|---:|
| TLB | 18,667 | 175.91 / 50.07 | 168.81 / 47.88 | 163.42 / 46.73 |
| Apple cf_live | 271 | 70.75 / 22.59 | 85.66 / 27.65 | 104.72 / 33.27 |
| Apple dramc_nb2 | 557 | 75.24 / 23.71 | 89.73 / 28.07 | 97.19 / 30.69 |
| Flash | 2,212 | 10.27 / 2.83 | 10.00 / 2.75 | 10.08 / 2.79 |
| Nested ticket | 340 | 4.22 / 1.28 | 4.28 / 1.30 | 4.60 / 1.40 |

Avoiding initial preprocessing can change subsequent quantifier search. TLB's online saving contracts to **4.0%** over all goals. Apple cf_live grows **21.1%** on the same policy. For reproduction, unpack the stream archive and run the generator, then feed its decompressed SMT-LIB to each shell and compare the full output to the emitted answer file.

```sh
python3 perf/ivy_apple/holdouts/batch_prefix_replay.py --events tlb/events.csv.gz --policies separate 2x64 lookahead4 --output /tmp/tlb-replay
gzip -dc /tmp/tlb-replay/2x64.smt2.gz | taskset -c 8 perf stat -e instructions:u -- g3 -smt2 -in > /tmp/tlb-replay/g3.out
diff -u /tmp/tlb-replay/2x64.answers /tmp/tlb-replay/g3.out
```

## Reusing a proof core across related verification batches

An UNSAT core identifies premises sufficient to refute a particular temporary goal. With the same goal and every one of those premises present, the result applies to the new batch even if other premises have changed. I tested this inside one sequential verification worker. It records the goal and exact premise identities. A miss falls back to a solver. An inconclusive mining attempt also falls back. Every distinct returned core was replayed in the in-tree C++ solver with only its listed premises and its goal. [Reproducible online replay and verifier](perf/ivy_apple/holdouts/batch_core_replay.py) · [Core premise identities for the first replication](perf/ivy_apple/holdouts/nov22b-core-history-certificates.csv.gz)

The first study replayed all physical goals. On TLB, waiting until a goal's second occurrence and mining at most once per goal took **100.9 billion instructions versus 176.1 billion** in ordinary g3. I then rebuilt the shell and repeated the comparison with the saved portable benchmark, five fresh runs per cell. Both runs use only earlier observations. In the repeat study, 653 TLB cores justified 13,109 skipped solver calls. The numbers below sum the instruction counts of the mining and ordinary solver processes. They exclude the Python driver and the independent audit. The separate complete-checker trial below includes bookkeeping overhead. [All exploratory and replicated runs](perf/ivy_apple/holdouts/nov22b-core-history-runs.csv.gz)

| Stream | Checks | g3 ordinary → learn on second occurrence, B instructions | Z3 ordinary → same policy | g3 checks reused |
|---|---:|---:|---:|---:|
| TLB | 18,667 | 181.69 → 106.10 | 50.19 → 29.45 | 13,109 |
| Flash | 2,212 | 11.07 → 10.01 | 2.84 → 2.57 | 1,172 |
| Apple cf_live | 271 | 73.53 → 83.53 | 22.59 → 26.92 | 96 |
| Apple dramc_nb2 | 557 | 78.58 → 117.79 | 23.71 → 40.22 | 149 |
| Nested ticket | 340 | 4.56 → 5.02 | 1.28 → 1.41 | 63 |

**The complete TLB checker gave the opposite result.** A GoIvy prototype performed the lookup and core extraction inside its existing single-goroutine SMT context, while keeping each original solver for ordinary checks and counterexamples. Four rounds on the full TLB spec used **107.85 billion** instructions in the retained checker versus **117.40 billion** when collecting cores on repeated goals in batches of 150 or more. The instrumented trial learned 653 certificates and reused them 13,109 times, exactly the headline hit count. The ordinary standalone shell has to search cases that the production checker can already refute cheaply. Even perfect agreement on which results can be reused does not predict the net cost. [280 end-to-end runs and verdict controls](perf/ivy_apple/holdouts/nov22b-complete-core-reuse.csv.gz) · [Prototype against GoIvy 59aa2840e](perf/ivy_apple/holdouts/nov22b-core-reuse.patch.gz)

Flash did benefit from learning once on a repeated goal in any batch, moving from 9.15 to 8.65 billion instructions. That configuration moved raftok from 24.93 to 31.94 billion and cf_live from 59.40 to 63.63 billion. Across 24 instrumented complete runs, native Z3 confirmed all **2,272 emitted certificates**, including repeated certificates in different configurations. Production keeps its existing proof methods. Any future dependency reuse should obtain premises as a byproduct of a proof the worker already paid for. [Native audit counts](perf/ivy_apple/holdouts/nov22b-complete-core-audit.csv.gz) · [Exact native replay queries](perf/ivy_apple/holdouts/nov22b-complete-core-queries.tar.gz)

## Which first-push work remains worth pursuing?

I sampled instructions inside the dynamic extent of each first physical Push on its calling OS thread. The figures are median billions of instructions attributed to the nearest recognized Go caller. This profile used g3 `a7c09920a` and GoIvy `6d847b88d`, before shortening the first-push schedule. The later exact boundary measurements give the costs on the retained solver. Sampling used a period of 2,000,003 retired user instructions, Go frame pointers and native DWARF unwinding. Only pairs with a sample in every expected first-push interval in both engines are included. Several C++ stacks did not unwind to a named phase, notably on cf_live, so subtracting individual C++ categories would overstate attribution. [Counts by push, leaf and caller](perf/ivy_apple/holdouts/nov22b-first-push-stack-counts.csv.gz) · [GoIvy interval observer](perf/ivy_apple/holdouts/nov22b-first-push-observer.patch.gz) · [Stack classifier](perf/ivy_apple/holdouts/first_push_profile.py) · [Retained first-push schedule](#preprocessing-on-the-first-physical-push)

| Workload | Complete profile pairs | Formula preprocessing | Internalization | Macro traversal | Pattern inference | BV sharing |
|---|---:|---:|---:|---:|---:|---:|
| TLB | 5 | 10.75 | 3.95 | 4.14 | 2.72 | 0.97 |
| Apple cf_live | 4 | 2.97 | 8.52 | 1.54 | 0.65 | 0.22 |
| Apple dramc_nb2 | 2 | 4.15 | 7.03 | 2.24 | 0.91 | 0.30 |
| Flash | 4 | 0.61 | 0.32 | 0.22 | 0.15 | 0.05 |

On TLB, processing formulas and finding macros dominate. The raw leaves repeatedly show expression-substitution lookup, application traversal and the activity cache. On cf_live, more samples land in internalization and its callees, including AST interning and reconstruction. This split supports a different next experiment for each workload. For TLB, measure which *individual source assertions* are normalized repeatedly with identical options and substitutions. Reuse only a transformation with a proven scope and lifetime. For cf_live, count constructed applications per internalized assertion against the native first push, distinguishing hash-cons hits from genuinely new terms. Changing the entire solver's assertion order based on shared conjuncts made both Apple streams slower.

I tried moving empty-substitution checks out of the per-node loop, bypassing inactive callbacks, and reading a node's shape and substitution-membership hint together. All 300 matched-search measurements reproduced the answers and the four cumulative search counters. On ticket 290 the 4-to-24-check instruction slope in millions was **196.5** in the control, **195.1** with the per-root check and **53.0** in native Z3. On toy consensus it was **1,679.1**, **1,675.5** and **470.4** respectively. Skipping entire quantifier-free or narrow-application subtrees had a larger semantic effect on resource and search counts. On raftok the wide-application shortcut moved complete verification from **25.04 to 37.46 billion** instructions. The current solver keeps its ordinary traversal. [Repeated replays](perf/ivy_apple/holdouts/nov22b-traversal-shortcuts-replay.csv.gz) · [Whole verifications](perf/ivy_apple/holdouts/nov22b-traversal-shortcuts-whole.csv.gz) · [Substitution and shape prototypes](perf/ivy_apple/holdouts/nov22b-rewriter-shape.tar.gz) · [Subtree prototypes](perf/ivy_apple/holdouts/nov22b-syntax-trials.tar.gz)

Larger candidate-model tables and longer-lived substitution caches also changed subsequent search. Increasing the table work budget to 32k reduced abstract Raft from 31.94 to 26.02 billion instructions while raising raftok from 24.78 to 29.15 billion. Retaining as few as 64 previous substitution-cache entries raised raftok from 24.90 to 63.03 billion. Weak identity-only reuse left it at 24.94 billion. The retained defaults use the smaller table and bound cached state to its established lifetime. [684 complete trials, including sibling seed controls](perf/ivy_apple/holdouts/nov22b-cross-check-model-variants.csv.gz) · [Substitution replays](perf/ivy_apple/holdouts/nov22b-substitution-state-replay.csv.gz) · [Experiment patches](perf/ivy_apple/holdouts/nov22b-cross-check-model-variants.tar.gz)

Two controls tested whether cheaper core collection would rescue the TLB replay result in production. Mining only after an earlier occurrence of that goal took at least 1 ms still used **112.8 billion** instructions versus **107.8 billion** in its control. Batching named premises in groups of four by their membership in the previous three bases used **121.1 billion**. The larger grouped policies reached 128 billion. The saved rows include each run's exit and transcript check, including timed-out numeric trials. [Threshold and grouping measurements](perf/ivy_apple/holdouts/nov22b-core-admission-and-groups.csv.gz) · [Threshold prototype](perf/ivy_apple/holdouts/nov22b-core-reuse-cost.patch.gz) · [Group prototype](perf/ivy_apple/holdouts/nov22b-core-reuse-groups.patch.gz)

## Choosing public action names with less temporary storage

The counter that eliminated 136,961,124 failed and successful suffix probes assigns private variables during fragment analysis. A second name allocator runs when composing Ivy actions. If both actions write `x`, the intermediate value normally has the public spelling `__m_x`. An input may already use that exact name, and choosing the next available spelling affects printed formulas. When a single intermediate is needed, the implementation reserves only source constants beginning with its candidate spelling. The first matching source name triggers allocation of the set. Repeated occurrences cost one entry per distinct spelling. Multiple intermediates share the existing public naming convention and record issued names because their bases can overlap. A disjoint update skips building its unused temporary conjunction, while preserving the annotation callback and active tracing. GoIvy commit `48ab9a5dd` contains the implementation and collision regression.

Thirteen alternating benchmark pairs at 500 iterations per case compared the previous and retained frontend on CPU 47. The 128-read-symbol disjoint fixture went from **16.93 to 14.30 µs**, **12,531 to 10,099 B/op** and **63 to 60 allocations/op**. The overlapping fixture went from **68.66 to 35.98 µs**, **45,562 to 23,097 B/op** and **216 to 198 allocations/op**. Run `go test . -tags=xtrace_off -run '^$' -bench '^BenchmarkComposeUpdatesReadOnlySymbols$' -benchtime=500x -benchmem` in GoIvy. [52 benchmark observations](perf/ivy_apple/holdouts/nov22b-source-name-release-micro.csv.gz) · [GoIvy source and regression](https://github.com/glycerine/ivy/commit/48ab9a5dd)

| Complete verification | g3 instructions before → after, billions | Linked Z3 instructions before → after, billions | Go heap allocation saved, g3 / linked Z3 |
|---|---:|---:|---:|
| Apple dramc_nb2 | 166.24 → 161.83 | 173.07 → 168.05 | 350.0 / 350.6 MB |
| TLB | 107.99 → 106.63 | 71.03 → 69.82 | 109.4 / 103.1 MB |
| Apple cf_live | 59.25 → 58.78 | 60.41 → 59.86 | 45.0 / 45.3 MB |
| Apple rfn2 | 17.30 → 17.06 | 52.68 → 52.43 | 30.1 / 30.1 MB |
| Abstract Raft | 32.15 → 32.18 | 24.86 → 24.84 | — |
| raftok | 24.91 → 24.87 | 19.93 → 19.92 | — |

The complete runs used default GC and three rounds per backend and revision on ten inputs. Go heap allocations are differences between median cumulative allocated bytes in two separate instrumented pairs per backend, rounded to decimal MB. They exclude C++ heap allocations. The TLB input produced 18,951 successful checks and both expected failures. The release reproduced all 87 standalone proof transcripts and exit statuses from the pre-change corpus on both engines, including all 90 reported failing obligations. In one pair per input, g3 used fewer instructions on 51 and less wall time on 54. The GoIvy composition, collision and renaming tests passed with g3, linked Z3 and active tracing. The full native g3 package suite passed. [120 complete verifications](perf/ivy_apple/holdouts/nov22b-source-name-release-whole.csv.gz) · [32 allocation runs](perf/ivy_apple/holdouts/nov22b-source-name-release-heap.csv.gz) · [174 corpus runs](perf/ivy_apple/holdouts/nov22b-source-name-release-corpus.csv.gz)

I also tested skipping only the unused conjunction, reserving only relevant names, a separate symbol-tree walker, and filtering local Skolems. On dramc_nb2 the standalone walker used 165.82 billion g3 instructions against 165.88 in its control. Collecting only prefixed symbols used 162.34 billion. A sorted-slice variant stored every matching occurrence until choosing a name. It passed the 87-spec comparison, but source repetition makes the temporary slice grow even if every matching occurrence has the same spelling. The retained allocator deduplicates on insertion. [Variant process runs](perf/ivy_apple/holdouts/nov22b-source-name-variants.csv.gz) · [Microbenchmarks](perf/ivy_apple/holdouts/nov22b-source-name-micro.csv.gz) · [Heap probes](perf/ivy_apple/holdouts/nov22b-source-name-variants-heap.csv.gz) · [Reservation prototypes](perf/ivy_apple/holdouts/nov22b-sparse-names.tar.gz) · [Local-symbol prototypes](perf/ivy_apple/holdouts/nov22b-local-symbols.tar.gz) · [Destination-set prototypes](perf/ivy_apple/holdouts/nov22b-name-dest.tar.gz) · [Sorted prototype](perf/ivy_apple/holdouts/nov22b-single-name-sorted.tar.gz) · [Sorted whole-process control](perf/ivy_apple/holdouts/nov22b-source-name-sorted-whole.csv.gz) · [Sorted allocation control](perf/ivy_apple/holdouts/nov22b-source-name-sorted-heap.csv.gz) · [Sorted corpus](perf/ivy_apple/holdouts/nov22b-source-name-sorted-corpus.csv.gz)

On the retained frontend five additional dramc_nb2 pairs at default GC gave 162.04 / 168.16 billion instructions and 212.6 / 204.0 MiB peak RSS for g3 / linked Z3. The independently calibrated larger-heap pair used GOGC 420 / 600. Its medians were 128.51 / 118.73 billion instructions, **522.7 / 498.3 MiB** peak and 19.51 / 20.16 s. The memory targets were selected using three observations at each of three nearby settings on the sorted-slice build. The final measured peaks differed by 24.4 MiB. The default panel used CPU 46 and the larger-heap panel CPU 45, on a shared host. g3 won three and five of the five wall-time comparisons respectively. [20 release observations](perf/ivy_apple/holdouts/nov22b-source-name-release-memory.csv.gz) · [18 GC calibration runs](perf/ivy_apple/holdouts/nov22b-source-name-rss-calibration.csv.gz) · [28 timed runs on the calibration build](perf/ivy_apple/holdouts/nov22b-source-name-sorted-memory.csv.gz)

To measure the effect of GC without changing CPU between settings, I interleaved both regimes and both engines on the same CPU for each round. CPU 14 used the earlier study's settings of 415 / 565 for the large heaps. CPU 15 used 385 / 565, reducing g3's setting after its 522.7 MiB median in the calibration follow-up. All 32 complete verifications produced byte-identical output and 2,479 passes. Each cell gives g3 / linked Z3. [32 interleaved measurements](perf/ivy_apple/holdouts/nov22b-source-name-interleaved-heaps.csv.gz) · [Reproducible benchmark](perf/ivy_apple/holdouts/dramc_memory_pairs.py)

| CPU and rounds | GOGC | Wall s | Billion instructions | Peak MiB | g3 wall wins |
|---|---:|---:|---:|---:|---:|
| 14, five | 100 / 100 | 21.26 / 22.81 | 162.12 / 168.03 | 216.8 / 197.5 | 4 / 5 |
| 14, five | 415 / 565 | 17.18 / 16.84 | 128.66 / 119.54 | 470.8 / 448.0 | 0 / 5 |
| 15, three | 100 / 100 | 21.26 / 22.76 | 161.73 / 167.72 | 214.1 / 188.8 | 3 / 3 |
| 15, three | 385 / 565 | 17.15 / 16.83 | 129.41 / 119.60 | 473.7 / 438.0 | 0 / 3 |

The median paired g3/Z3 wall ratios were 1.018 and 1.023 in the larger-heap trials on these two CPUs. The medians of peak memory differed by 23 and 36 MiB. The previous eleven-pair comparison before the public-name change remains the closest measured RSS match at 476.6 / 479.3 MiB. On all three final-release CPUs with a large-heap trial, the linked process used fewer instructions. Wall-time ordering varied by CPU.

### Next performance experiments in priority order

Among the 87 standalone inputs, TLB had the largest absolute process instruction gap at 36.75 billion. Hybrid reliable broadcast was second at 7.69 billion. Four fresh pairs per input on CPU 16 measured **14.00 / 6.27 billion** instructions for hybrid reliable broadcast (g3 / linked Z3), **5.00 / 2.74 billion** for alternating bit protocol and **3.73 / 1.55 billion** for Flash 3. Each transcript and exit status matched the previous corpus. [24 additional proof runs](perf/ivy_apple/holdouts/nov22b-source-name-release-additional-specs.csv.gz)

1. **Measure redundant normalization on TLB first pushes.** Those 58 pushes still cost 14.47 billion more instructions in g3. Count calls and total work by source assertion, active substitution environment and preprocessing options, then measure how many results an exact worker-local reuse policy could serve. On cf_live, prioritize AST construction in internalization, which accounted for 8.52 billion sampled instructions before shortening the schedule. The full-stream common-scope experiment shows why the acceptance test must include all subsequent proof search. [First-push profiles](#which-first-push-work-remains-worth-pursuing) · [Common-scope controls](#can-consecutive-verification-batches-keep-their-common-premises)
2. **Reduce repeated rewrites inside model evaluation, internalization and quantifiers.** On toy consensus, the rewriter accounts for about 195 million excess instructions per matched check when called from internalization, 154 million from model evaluation and 148 million from quantifiers. Compare the actual operands and bindings at these call sites before enlarging a cache. The measured table-budget and substitution-cache changes improved one Raft input while making the other slower. Proof premises available during an existing refutation are also candidates for reuse. Charging a separate solver to extract them made the complete TLB verification slower despite 13,109 cache hits. [Matched-search profiles](#where-each-additional-matched-check-spends-instructions) · [Complete core experiment](#reusing-a-proof-core-across-related-verification-batches)
3. **Remove high-volume frontend allocations that both backends pay for.** The private-name search and action composition give two measured examples. Rank additional sites by cumulative bytes and complete-process instructions, then repeat dramc_nb2 at default GC and comparable measured RSS. Timing, instructions and RSS answer different questions here. [Fresh-name measurements](#fresh-variable-names-in-the-ivy-fragment-checker) · [GC costs](#why-the-larger-heap-changes-the-apple-comparison)

## Rewriting on TLB’s 58 first pushes

In five rounds on the November 23 checkout, the complete TLB verification retired **106.98 billion instructions in g3 and 69.80 billion in linked Z3**. Both produced 18,951 successful checks and the two expected failures. g3 was already ahead on Apple rfn2, cf_live and dramc_nb2, the finite-domain Raft wrapper and the numeric example. Linked Z3 was ahead on abstract Raft, raftok, TLB, hybrid broadcast, flash2, ticket and toy consensus. Each of the twelve workloads used one pinned CPU, one Go processor, default GC and the same GoIvy frontend at `01aa2c6fa`, with g3 at `d563ecbea`. All 120 verdict transcripts agreed with the saved controls. [Complete runs](perf/ivy_apple/incremental_cost/nov23-current-baseline.csv.gz)

Across TLB’s 58 first physical pushes, g3 made 211,113 top-level theory-rewriter calls and took 3,227,992 traversal steps. Native Z3 took 4,126,690 traversal steps. g3’s shorter verification schedule accounts for much of the difference in macro finding and assertion reduction. Value propagation gives a direct comparison on the same number of calls and steps. g3 spent **3.27×** the instructions in those rewrites. Pattern inference processed the same 30,786 rows and changed 27,656 of them in both engines. g3 spent **4.32×** the instructions there. [Native calls and steps](perf/ivy_apple/incremental_cost/nov23-first-push-native-rows.csv.gz) · [Go calls and steps](perf/ivy_apple/incremental_cost/nov23-first-push-rewrite-recurrence.csv.gz) · [Solver-thread counter readings](perf/ivy_apple/incremental_cost/nov23-first-push-rewrite-instructions.csv.gz)

| Enclosing phase | g3 / Z3 rewrite calls | g3 / Z3 traversal steps | g3 / Z3 billion instructions inside rewrites |
|---|---:|---:|---:|
| Initial assertion setup | 58 / 58 | 432,658 / 432,658 | 1.36 / 0.38 |
| Value propagation | 87,961 / 87,961 | 1,120,822 / 1,120,822 | 2.74 / 0.84 |
| Macro finding | 30,761 / 61,547 | 393,374 / 786,832 | 0.96 / 0.58 |
| NNF and CNF | 30,761 / 30,761 | 381,486 / 381,480 | 0.93 / 0.28 |
| Assertion reduction | 30,786 / 61,572 | 392,374 / 898,284 | 1.09 / 0.69 |
| Pattern inference | 30,786 / 30,786 | — | 2.59 / 0.60 |
| Simplification after BV sharing | 30,786 / 30,786 | 507,278 / 506,614 | 1.50 / 0.47 |

The three-run medians of all the measured rewrite intervals sum within each run to **11.21 billion g3 instructions versus 3.84 billion native**. This accounts for about half of the previously measured 14.47-billion first-push gap. The intervals cover the call to the theory or pattern rewriter inside each phase. Setup outside that call, constructing the engine, other simplification passes, internalization, propagation and work on other threads fall outside those measurements. The counter reads subtract the cost of an empty bracket. The table omits pattern traversal steps because the pattern entry point does not report them. [First-push totals](#preprocessing-on-the-first-physical-push) · [Counter driver](perf/ivy_apple/incremental_cost/first_push_rewrite_rows.py)

Within one push, 55,195 calls repeated a previous theory rewrite with the same input arena owner, slot, generation, effective options and substitution mapping. They consumed **667,734 steps**, or 20.7% of the Go total. Every earlier and later result for these repeated calls was identical to its input. All repeated steps occurred with an empty substitution. Repetition explains 373,247 steps during NNF and CNF and 291,634 during macro finding, but only **568 of 1,120,822** during value propagation. The latter phase retains a 1.90-billion-instruction gap even when almost every root is new to that rewrite environment. [Two observations per verification on eight inputs](perf/ivy_apple/incremental_cost/nov23-first-push-rewrite-recurrence.csv.gz)

Four prototype builds remembered identity results during a single first push. On TLB the medians were **107.12 billion** instructions for the control, **105.58** for a map and **106.86 / 106.44 / 105.83** for direct-mapped tables of 256 / 1,024 / 4,096 entries. The largest direct table used 32 KiB during each active first push. The map saved 1.4% of complete TLB instructions. A separate group of five pattern-collector prototypes tested small binder bitsets, compact traversal keys, lazy argument slices and bounded scratch reuse. The combined pooled variant gave 106.90 billion against 107.09 for its control. All 464 complete runs in the two experiments matched the reference outcomes. These results favor reducing the cost of each traversal and avoiding entire verification scopes. The production build retains its existing cache policy. [Identity-cache trials](perf/ivy_apple/incremental_cost/nov23-identity-process.csv.gz) · [Four prototype diffs](perf/ivy_apple/incremental_cost/nov23-identity-variants.tar.gz) · [Pattern trials](perf/ivy_apple/incremental_cost/nov23-pattern-process.csv.gz) · [Five pattern prototypes](perf/ivy_apple/incremental_cost/nov23-pattern-variants.tar.gz)

To repeat the observation, build a reference checker, run the chosen verification once and keep its stdout, stderr and exit status. From `pkg`, run the following for each engine (`g3` and `native`) and mode (`rows` and `instructions`), choosing a separate output directory each time. The native build uses the GoIvy archive with one instrumented object and checks its original object against the in-tree build. The driver and all four combinations were exercised on the complete TLB input. The prototype patches apply at `d563ecbea` from the g3 repository root. [Observation driver](perf/ivy_apple/incremental_cost/first_push_rewrite_rows.py) · [Recurrence summary](perf/ivy_apple/incremental_cost/summarize_first_push_rewrites.py) · [Native counter](../src/test/perf/ivy_asserted_rows.cpp)

```sh
python3 perf/ivy_apple/incremental_cost/first_push_rewrite_rows.py \
  --goivy /path/to/ivy/goivy --spec /path/to/examples/liveness/tlb.ivy \
  --stdout /tmp/reference/stdout --stderr /tmp/reference/stderr --expected-exit 1 \
  --abc /path/to/abc --cpu 2 --engine g3 --mode rows --pairs 2 --output /tmp/g3-tlb-rows
python3 perf/ivy_apple/incremental_cost/summarize_first_push_rewrites.py \
  /tmp/g3-tlb-rows/1/observations.gz
```

### How many first pushes can a stronger Boolean solver eliminate?

For every ground goal that the existing unit-propagation proof could not settle, I saved its Boolean clauses and goal, then solved that projection to completion in native Z3. Each live premise index had its own identifier. New ground atoms in a goal received distinct Boolean variables, with repeated occurrences sharing one variable. UNSAT therefore proves the original goal impossible. SAT leaves the original theory question open. This measures the maximum additional coverage available from the *same clauses*, independently of how fast g3 could search them. [Per-goal outcomes for three premise constructions](perf/ivy_apple/goal_instances/nov23-projected-goals.csv.gz)

| Input | Goals left by unit propagation | Additional Boolean refutations | Batches with residual goals | Batches where Boolean search discharged every residual goal |
|---|---:|---:|---:|---:|
| TLB | 2,654 | 379 | 58 | 0 |
| Apple cf_live | 104 | 18 | 12 | 0 |
| Apple dramc_nb2 | 101 | 36 | 19 | 1 |
| Flash | 1,546 | 0 | 28 | 0 |
| Hybrid reliable broadcast | 271 | 21 | 34 | 0 |
| Ticket | 93 | 3 | 12 | 0 |
| Abstract Raft | 156 | 0 | 13 | 0 |
| raftok | 143 | 0 | 14 | 0 |

On TLB the Boolean solver discharged the first residual goal in 15 batches. Each of the 58 batches still had a later residual goal that required reasoning outside the projection. Both real counterexamples remained satisfiable. On Flash, all 1,546 projections were satisfiable. Investing in faster SAT search over these particular clauses would leave every Flash miss unresolved. Matching a goal directly against a quantified premise can supply the missing relationship without generating a larger clause set for every sibling. [TLB captured clauses and original answers](perf/ivy_apple/goal_instances/nov23-tlb-projection.events.gz) · [Independent replay driver](perf/ivy_apple/goal_instances/ground_projection_replay.py)

I also ran 800 complete g3 verifications across three experiments, rotating build order on a pinned logical CPU for each input. Each retained its reference proof output and exit status. Adding application-congruence clauses moved median TLB instructions from 106.67 to 105.20 billion. Using four ground constants per sort moved it to 108.08 billion. Matching flat quantifier triggers across the entire base, even at four instances per pattern, raised TLB from 106.59 to 178.64 billion and dramc_nb2 from 162.29 to 168.64 billion. Consulting an already-prepared unit index before later pushes saved 1.81 billion TLB instructions out of 107.29. These measurements favor deriving instances from the pending goal and retaining a concrete scope when later goals can reuse its search state. [800 complete runs](perf/ivy_apple/goal_instances/nov23-ground-proof-trials.csv.gz) · [Prototype patches](perf/ivy_apple/goal_instances/nov23-ground-proof-prototypes.tar.gz)

From `pkg`, the first command below replays all 2,654 stored TLB goals. The second rebuilds the observer that captured them, using g3 `94e5b2998` and GoIvy `01aa2c6fa` with its local g3 module replacement. The archive also contains each timed build. For example, select `congruence/eu` to build the application-congruence experiment. [Build driver](perf/ivy_apple/goal_instances/ground_proof_checker.py)

```sh
python3 perf/ivy_apple/goal_instances/ground_projection_replay.py \
  --capture perf/ivy_apple/goal_instances/nov23-tlb-projection.events.gz \
  --solver /path/to/ivy/goivy/z3vendor/native_lib/z3 --output /tmp/tlb-boolean.csv
python3 perf/ivy_apple/goal_instances/ground_proof_checker.py \
  --variant projection/control --goivy /path/to/ivy/goivy --abc /path/to/abc \
  --output /tmp/tlb-projection-checker
```

### Would solving sibling goals together amortize the first push?

With a common base, UNSAT for a disjunction of counterexample conditions proves every condition in the group impossible. A satisfiable or inconclusive group must be checked individually or split again. I tested this both on archived solver inputs and inside the full verifier, where the search state and the order of later checks also matter. Four repetitions of each full-checker configuration on nine inputs produced 324 transcripts and exit statuses matching the control. The experiment used the checker preceding the direct universal-premise change. [Whole-checker observations](perf/ivy_apple/holdouts/nov23-sibling-checkers.csv.gz)

| Verification | g3, separate | g3, up to 8 siblings | g3, up to 64 siblings | Linked Z3, separate | Linked Z3, up to 64 siblings |
|---|---:|---:|---:|---:|---:|
| TLB | 106.92 | 95.91 | 134.97 | 69.78 | 51.45 |
| Apple cf_live | 58.87 | 146.74 | 66.03 | 59.83 | 63.22 |
| Apple dramc_nb2 | 162.02 | 197.75 | 155.41 | 168.08 | 164.31 |
| Flash | 9.15 | 12.12 | 33.13 | 4.54 | 5.11 |
| Hybrid reliable broadcast | 14.00 | 15.29 | 18.05 | 6.27 | 7.57 |

Values are median billions of whole-process instructions. The grouped g3 checker first tries an available unit proof on each candidate. It groups consecutive built-in checks and stops at assumptions or unfamiliar callbacks. The timings include construction of the disjunction, unsuccessful group checks, and any individual fallback. The two TLB counterexamples and all 65 Flash counterexamples still appeared. Grouping eight siblings saved 11.01 billion instructions on TLB but added 87.87 billion on cf_live. Keeping this out of the default verifier avoids a larger regression on an independently measured input. [Frontend experiment and individual-check selections](perf/ivy_apple/holdouts/nov23-sibling-controls.tar.gz)

The shell benchmark covers a different workload because its input contains the physical checks that reached the solver. It can replay all of them or only the checks that required ordinary search. Of 236 measured configurations and repetitions, 210 finished with the expected answers and 26 Raft replays hit the 150-second watchdog. The latter appear as censored runs. To reproduce a TLB selection, unpack the two archives below into one directory and run the benchmark with `--events tlb/events.csv.gz --selection tlb-selected.csv.gz --width 16 --cpu 2 --solver /path/to/z3 --output /tmp/tlb-sibling-replay`. Omitting the selection replays the full physical stream. [Shell benchmark](perf/ivy_apple/holdouts/sibling_goals.py) · [Measurements including cutoffs](perf/ivy_apple/holdouts/nov23-sibling-shell.csv.gz) · [Physical streams](perf/ivy_apple/holdouts/nov22b-physical-streams.tar.gz) · [Selections](perf/ivy_apple/holdouts/nov23-sibling-controls.tar.gz)

## Matching a ground goal against a universal premise

Commit `f0b4ebb1d` adds a solver-local index for universally quantified premises. For example, `∀x. R(f(x),x)` rules out `¬R(f(a),a)` even when the usual ground projection chose a different representative for that sort. The matcher follows the actual declarations, checks binder sorts, and requires each occurrence of a variable to match the same ground term. It indexes consequences of a positive conjunction or a negative disjunction. It can also establish a goal by one sufficient disjunct. A miss continues through the normal proof path. [Implementation](api/internal/order/universal_premises.go) · [Regression tests and benchmark](api/api_universal_premises_test.go)

| Complete verification | Previous g3 | g3 with matching | Linked Z3 |
|---|---:|---:|---:|
| Apple rfn2 | 17.03 | **15.28** | 52.36 |
| Flash | 8.98 | **8.36** | 4.53 |
| Hybrid reliable broadcast | 13.99 | **13.05** | 6.28 |
| TLB | 106.71 | **103.49** | 69.85 |
| Ticket | 4.24 | **4.03** | 2.23 |
| Apple cf_live | 58.86 | **58.31** | 59.89 |
| Apple dramc_nb2 | 162.11 | 162.21 | 167.75 |
| Abstract Raft | 32.06 | 31.89 | 24.85 |
| raftok | 24.82 | 24.88 | 19.92 |
| Numeric queue, including its expected unknown | 109.92 | 110.01 | 211.79 |

Values are median billions of user-mode instructions from five runs per build and input. Each triplet used the same pinned CPU, GoIvy `01aa2c6fa`, default GC, one Go runtime CPU and tracing disabled. Concurrent workloads affected elapsed time. The table includes ten of the twelve inputs. The full panel contains 180 completed processes. The release also reproduced linked Z3’s stdout, stderr and exit status on all 87 standalone proof inputs, including 28,929 passing obligations and 90 expected failures. [Release measurements](perf/ivy_apple/goal_instances/nov23-universal-release-process.csv.gz) · [174 corpus runs](perf/ivy_apple/goal_instances/nov23-universal-release-corpus.csv.gz)

The first version could assemble a proof from several ground facts and immediately bypass any matching check. On the numeric queue it raised instructions from 110.99 to 153.75 billion. On raftok, bypassing just five checks raised the median from 24.90 to 36.72 billion. A concrete check can do work that benefits its siblings. The retained version limits the proof to the universal pattern and one sufficient branch. A match can avoid initializing a cold solver. After incremental search starts, the usual path gets the first match under each set of premises. This preserved the numeric and Raft controls while recovering 10.3% on rfn2 and 6.9% on Flash. [916 paired trial observations](perf/ivy_apple/goal_instances/nov23-universal-experiments.csv.gz) · [Rule and scheduling prototypes](perf/ivy_apple/goal_instances/nov23-universal-experiments.tar.gz)

A separate linked-Z3 checker reasserted the original typed ASTs into a freshly reset native solver for each of 1,784 matched final conditions. All returned UNSAT in both the original native scope and the fresh solver. These include 960 Flash, 679 TLB, 91 hybrid broadcast, 29 Ticket, 17 cf_live, seven rfn2 and one raftok condition. A second execution of the saved replay driver reproduced every ordinal, assertion count and decision. The focused alternating match/miss benchmark measured 377.9 ns/op and zero allocations per operation across nine repetitions. The full Go package suite passed. [Native oracle results](perf/ivy_apple/goal_instances/nov23-universal-native-proofs.csv.gz) · [Microbenchmark measurements](perf/ivy_apple/goal_instances/nov23-universal-siblings.bench.gz)

```sh
go test ./api -run '^$' -bench '^BenchmarkUniversalPremiseSiblings$' -benchmem -benchtime=100000x
python3 perf/ivy_apple/holdouts/ground_premise_replay.py --goivy /path/to/ivy/goivy \
  --examples /path/to/ivy-lang-examples --abc /path/to/abc --cpu 2 --final-conditions \
  --observations perf/ivy_apple/goal_instances/nov23-universal-native-proofs.csv.gz \
  --cases flash2 tlb rfn2 cf_live hybrid_reliable_broadcast_cisa raftok ticket
```

## Witness search after the first incremental push

In both abstract Raft specifications, g3 attempted 22 multi-sort witness proofs with up to ten candidate terms per sort and two rounds. Three proved the goal. The 19 misses cost **4.50 billion instructions on raftok and 5.02 billion on raft_no_assume** before ordinary search began. On raftok, constructing and asserting those 19 ground problems cost 1.32 billion instructions and checking them cost 3.00 billion. All 22 probes happened after the first incremental push. On Apple rfn2 the two wide probes both proved their goals while the first push could still be avoided. Freshly reset linked Z3 independently returned UNSAT on all eight original typed conditions across the three inputs. [Private plan counters](perf/ivy_apple/goal_instances/nov23-private-plan-cost.csv.gz) · [Native proof replay](perf/ivy_apple/goal_instances/nov23-witness-native-proofs.csv.gz)

Commit `67ef8601a` tries one candidate per sort for one round when a high-arity, multi-sort witness search runs in a batch whose incremental solver can no longer defer its first push. A failed proof hands the original assertions to ordinary search. The same choice applies to every input and requires no additional solver state.

| Complete verification | Previous g3, billion instructions | New g3 | Linked Z3 | Previous g3, seconds | New g3 | Linked Z3 |
|---|---:|---:|---:|---:|---:|---:|
| raftok, 420 conditions | 24.74 | **20.76** | 19.92 | 5.47 | **4.73** | 4.58 |
| raft_no_assume, 419 conditions | 31.82 | **27.27** | 24.85 | 6.74 | **5.99** | 5.59 |
| Apple rfn2, 26 conditions | 15.29 | **15.28** | 52.43 | 2.45 | **2.46** | 15.44 |

Each entry is a median of seven runs on the same logical CPU per workload, with GoIvy `01aa2c6fa`, default GC, PGO disabled and one Go runtime CPU. The builds alternated and shared the machine with other experiments. In a separate four-round, ten-input panel, raftok fell from 24.33 to 20.45 billion instructions and raft_no_assume from 31.91 to 27.34 billion. These are the abstract Raft inputs. The finite-domain Raft wrapper measured 23.46 billion for g3 and 214.79 billion for linked Z3. All 183 timed processes reproduced the expected outcomes. All 87 additional standalone inputs matched linked Z3 on stdout, stderr and exit status. [Process observations from both sets](perf/ivy_apple/goal_instances/nov23-witness-release.csv.gz) · [87 paired verifications](perf/ivy_apple/goal_instances/nov23-witness-corpus.csv.gz) · [Paired verification driver](perf/ivy_apple/holdouts/paired_verification.py)

Raftok conditions 54 and 100 can still be proven with a single candidate. On condition 55 both widths miss, and on 373 only the wide search succeeds. The narrower search cut the four median standalone times from 7.82, 14.43, 14.75 and 17.72 ms to 3.77, 5.46, 4.79 and 5.73 ms, respectively. The complete verification includes the cost of ordinary search after the missed proof on 373. Disabling all wide-shaped probes after the first push raised raftok from 24.95 to 33.76 billion instructions. Using only two candidates for one round even before the first push raised rfn2 from 15.29 to 20.30 billion. The saved Go benchmark checks each expected proof or miss. [Benchmark and original queries](api/api_witness_instances_benchmark_test.go) · [All microbenchmark samples](perf/ivy_apple/goal_instances/nov23-witness-width.bench.gz) · [Width, timing and Boolean-only controls](perf/ivy_apple/goal_instances/nov23-witness-experiments.csv.gz) · [Prototype and counter patches against g3 63d7d9581](perf/ivy_apple/goal_instances/nov23-witness-experiments.tar.gz)

On TLB the original private proof path still spent 19.72 billion instructions, including 9.43 billion in auxiliary solver proofs and 4.37 billion building quantified Boolean projections. On hybrid reliable broadcast, auxiliary proofs accounted for 3.13 of 3.96 billion. These are nested solver-thread intervals collected before narrowing the witness search. The TLB whole-process change from narrowing alone was 103.52 to 103.41 billion. Reusing a parsed assertion base or reducing auxiliary solver setup is more relevant to those two inputs. [Per-invocation counters on nine inputs, three repetitions](perf/ivy_apple/goal_instances/nov23-private-stages.csv.gz)

```sh
go test ./api -run '^$' -bench '^Benchmark(RaftWitnessWidth|AppleWitnessInstances)$' -benchmem -benchtime=5x -count=5 -cpu=1
python3 perf/ivy_apple/holdouts/ground_premise_replay.py --goivy /path/to/ivy/goivy \
  --examples /path/to/ivy-lang-examples --abc /path/to/abc --cpu 2 --final-conditions \
  --observations perf/ivy_apple/goal_instances/nov23-witness-native-proofs.csv.gz \
  --cases raftok raft_no_assume rfn2
```

## Propagating an existing premise index before building another projection

Commit `8f05e60c7` changes the order of work when a batch already has ground instances and Boolean units for its current premises. The solver first tries propagating the sibling goal through that index. Reuse requires the same AST manager and the same ordered premise handles, including their generations. A miss continues through the existing proof sequence. [Implementation](api/api_order_abstraction.go) · [Scope, cancellation and manager tests](api/api_ground_premises_test.go)

| Complete verification | Previous g3 | g3 with earlier propagation | Linked Z3 | Previous g3, seconds | New g3, seconds | Linked Z3, seconds |
|---|---:|---:|---:|---:|---:|---:|
| Apple dramc_nb2 | 162.24 | **158.34** | 168.06 | 25.86 | **24.54** | 27.52 |
| TLB | 103.67 | **102.80** | 69.84 | 15.83 | **15.69** | 10.78 |

Instruction columns are median billions of user instructions over seven alternating runs per build on one pinned CPU per input. All used GoIvy `01aa2c6fa`. Five further rounds on ten inputs and one pair on each of 87 standalone specs matched the reference outcomes. The trials also measured two tempting alternatives. Eagerly constructing a missing index took hybrid reliable broadcast from 13.02 to 13.34 billion instructions. Moving the universal matcher up with a prepared index took raftok from 24.72 to 36.24 billion before the separate witness-width change. [192 process observations](perf/ivy_apple/goal_instances/nov23-prepared-units.csv.gz) · [87 corpus pairs](perf/ivy_apple/goal_instances/nov23-prepared-units-corpus.csv.gz) · [280 ordering trials](perf/ivy_apple/goal_instances/nov23-unit-order-trials.csv.gz) · [Ordering patches against 63d7d9581](perf/ivy_apple/goal_instances/nov23-unit-order-trials.tar.gz)

## Walking the fragment graph without serializing formulas

GoIvy `817871f86` stores each source vertex's outgoing arc indices in order. The old walk serialized a formula to construct a dictionary key for each arc, sometimes twice. Parallel arcs to the same destination cannot change which cycle depth-first search returns. If the first arc finds a cycle, the search has already returned. If the visit finishes, later arcs see the destination marked complete. Keeping the original arc indices preserves the first formula, argument and location reported when the graph fails stratification. The cost affects both solver backends. [GoIvy implementation and benchmark](https://github.com/glycerine/ivy/commit/817871f86)

For the 129-vertex benchmark, seven 25-iteration samples gave a median **440.84 to 53.48 µs** with leaf formulas and **1,657.35 to 52.71 µs** with nested formulas. Nested-formula allocation fell from 2.02 MB to 44,376 bytes per traversal. In five complete runs per checker, g3 instruction counts on dramc_nb2 fell from **158.71 to 156.23 billion**, on cf_live from **58.31 to 57.60 billion** and on rfn2 from **15.25 to 14.95 billion**. With the new frontend, linked Z3 used 164.98, 59.18 and 51.99 billion on the same three inputs. Wall times varied under contention. On dramc_nb2, for example, the g3 wall median rose from 32.32 to 47.54 seconds despite the lower instruction count. [90 complete runs](perf/ivy_apple/holdouts/nov23-fragment-cycles.csv.gz) · [Original microbenchmark samples](perf/ivy_apple/holdouts/nov23-fragment-cycles-before.bench.gz) · [Retained microbenchmark samples](perf/ivy_apple/holdouts/nov23-fragment-cycles-after.bench.gz)

The two implementations returned the exact same first cycle on 30,000 seeded directed multigraphs with vertex unification, self-loops and duplicate arcs. The checker also preserved both output streams and exit status on 108 permutations of nine real fragment errors. Enabled tracing on rfn2 produced byte-identical 2,334,030,415-byte output streams with 524,035 trace records and 26 passes. The Paxos `log3` comparison matched the Python reference on all 27,855 normalized lines, including 27,782 trace records. An independent set of 87 standalone specs matched linked Z3 on both streams and exit status. [Graph differential driver](perf/ivy_apple/holdouts/fragment_cycles.py) · [Permuted diagnostics](perf/ivy_apple/holdouts/nov23-fragment-cycle-errors.csv.gz) · [Corpus comparisons](perf/ivy_apple/holdouts/nov23-fragment-cycle-corpus.csv.gz) · [Enabled-trace comparison driver](perf/ivy_apple/compare_frontend_traces.py)

```sh
python3 perf/ivy_apple/holdouts/fragment_cycles.py --goivy /path/to/ivy/goivy
go -C /path/to/ivy/goivy test . -tags=xtrace_off -run '^$' -bench '^BenchmarkFragmentCycleTraversal$' -benchmem -benchtime=25x -count=7 -cpu=1
```

## Parsing the unchanged premises of quantified goals

Commit `4ea7abff8` retains the parsed input nodes and child indices for the permanent premises of a solver batch. A sibling goal starts with the exact declaration and source IDs assigned to those premises. It selects representative constants and builds its own quantifier instances after reading the new goal. This matters when changing the goal changes which constant should instantiate a universal. The saved tuple includes the manager and the ordered premise handles with their generations. The solver charges the original parse work against every goal's limit and checks cancellation on reuse. [Implementation](api/internal/order/quantified_premises.go) · [Budget, changed-base and manager regressions](api/api_quantified_premises_test.go)

| Complete verification | Previous g3 | g3 with parsed premises | Linked Z3 |
|---|---:|---:|---:|
| TLB | 102.70 | **100.25** | 69.78 |
| Hybrid reliable broadcast | 13.06 | **12.89** | 6.27 |
| Apple dramc_nb2 | 155.85 | **155.46** | 165.25 |
| Apple rfn2 | 14.88 | 14.89 | 52.02 |
| Abstract raft_no_assume | 27.52 | 27.43 | 24.85 |
| raftok | 20.85 | 20.79 | 19.93 |
| Flash | 8.46 | 8.51 | 4.54 |

Each number is a five-run median in billions of whole-process user instructions on GoIvy `817871f86`. Across nine inputs a second build compiled every eligible goal both ways inside the real verification. It compared admission and errors as well as all 736 returned clause sequences, variable counts and charged work totals. All agreed. The 87 standalone specs also matched linked Z3 on exit status and both output streams. A full native Go test run passed. [All 135 timed runs](perf/ivy_apple/goal_instances/nov23-quantified-premises.csv.gz) · [Per-projection comparisons](perf/ivy_apple/goal_instances/nov23-quantified-premises-oracle.csv.gz) · [87 corpus pairs](perf/ivy_apple/goal_instances/nov23-quantified-premises-corpus.csv.gz)

In the archived dramc_nb2 condition 71 benchmark, projection alone took **3.683 ms and 2,109,269 B/op** when compiling fresh versus **2.105 ms and 1,072,120 B/op** with the parsed base. These are medians from nine 15-iteration runs with one runtime CPU. Across four whole-verifier rounds on TLB, keeping a second table for new goal IDs used 100.27 billion instructions versus 100.26 billion for cloning the premise maps. Retaining additional grounded applications got down to 99.60 billion, at the cost of another set of term, binding and application tables per active batch. The production change keeps just the parsed base. [Microbenchmark](api/api_quantified_premises_test.go) · [Microbenchmark samples](perf/ivy_apple/goal_instances/nov23-quantified-premises.bench.gz) · [400 prototype runs](perf/ivy_apple/goal_instances/nov23-quantified-premise-trials.csv.gz) · [Prototype diffs](perf/ivy_apple/goal_instances/nov23-quantified-premise-trials.tar.gz)

## Where the general solver still spends more instructions

I measured three pairs of complete verification runs on each of 87 standalone inputs and three named Apple isolates. The 540 processes used g3 `8f05e60c7` or the linked native library, GoIvy `01aa2c6fa`, one Go runtime CPU and disabled tracing. Both backends reproduced the reference exit and both output streams on every run, including the expected counterexamples. The observer opens a hardware counter on the calling OS thread for each batch and subtracts an adjacent empty bracket. The measured intervals cover API operations inside verification, excluding frontend preparation, model post-processing and asynchronous runtime work. This is a different measurement from the whole-process instruction tables. [Per-operation totals in every run](perf/ivy_apple/incremental_cost/nov23-verification-phase-totals.csv.gz) · [Raw rows by batch and condition, with input manifest](perf/ivy_apple/incremental_cost/nov23-verification-phase-rows.tar.gz)

| Complete input | g3, billion instructions in measured calls | Linked Z3 | Excess in g3 |
|---|---:|---:|---:|
| TLB | 81.49 | 51.01 | **30.48** |
| Apple dramc_nb2 | 46.62 | 38.54 | **8.07** |
| Hybrid reliable broadcast | 11.39 | 4.69 | **6.70** |
| Flash | 7.79 | 3.91 | **3.87** |
| Nested ticket | 3.98 | 1.71 | **2.27** |
| Abstract raft_no_assume | 26.27 | 24.06 | **2.21** |

Each column uses unrounded per-run sums before taking the three-run median. On TLB, pushes cost 20.46 billion in g3 and 5.98 billion in Z3. Concrete check calls, including any private attempt inside a check, cost 44.79 and 36.55 billion. Separate g3 attempts before the first push cost 8.99 billion. Some of those attempts eliminate native checks, so their gross cost overstates what could be saved by removing them. A given batch can reach its first physical push on a different condition in each engine. Compare total push costs per batch before attributing a per-condition difference.

On abstract raft_no_assume, logical checks 251, 274 and 275 all reached the concrete backend on both sides. Check 274 took **3.87 versus 0.89 billion** instructions and check 251 took **3.65 versus 1.09 billion**. Each gap is larger than the net excess across the entire input because g3 wins on other checks. On Flash, 3.30 billion of its 3.87-billion excess occurs inside concrete checks. These measurements prioritize TLB first-push rewrites and the matched expensive concrete checks of abstract Raft and Flash. [Reusable counter driver](perf/ivy_apple/incremental_cost/verification_phase_costs.py) · [Adapter instrumentation](perf/ivy_apple/incremental_cost/nov23-verification-phase-counter.patch.gz)

```sh
python3 perf/ivy_apple/incremental_cost/verification_phase_costs.py \
  --goivy /path/to/ivy/goivy --spec /path/to/ivy-lang-examples/examples/liveness/ticket.ivy \
  --abc /path/to/abc --cpu 2 --pairs 3 --output /tmp/ticket-phase-costs
```

## November 23 Makefile scope benchmark

Nine alternating pairs of the original `make bench` and `make cppbench` targets on CPU 47 with `GOMAXPROCS=1` and 10,000 iterations put surviving atoms at **354 ns in g3 and 477 ns in Z3**. Scoped atoms took **13.53 µs and 12.42 µs**. The scoped g3 samples ranged from 13.25 to 13.74 µs and the C++ samples from 12.39 to 12.47 µs. Both implementations use the in-tree benchmark. This measures the timed scope pop on this host with the Makefile's ordinary configuration. For independent-worker throughput and retained heap, use the separately tracked Go worker benchmark. [All 36 observations](perf/scope_restoration/nov23-pinned-make-targets.csv.gz) · [Worker benchmark](smtcore/smt_scope_workers_benchmark_test.go)

## Checking wide finite models with ordered decision diagrams

In commit `787f64620`, candidate models with more than 512 assignments can use a reduced ordered multi-valued decision diagram. Each level represents one quantified variable. Domain values that lead to the same remaining answers share a child. For example, when a table ignores its second argument, the diagram can represent the answer independently of that argument. The procedure supports the Boolean and uninterpreted-sort expressions already admitted by the model-table checker. The decision uses the current formula, function interpretations and domains. It runs on each solver's own goroutine. [Model-table entry point](smtcore/smt_model_table_check.go) · [Decision diagram](smtcore/smt_model_table_diagram.go)

| Complete GoIvy verification | Previous g3, billion instructions | New g3 | Linked Z3 | Previous g3, seconds | New g3 | Linked Z3 |
|---|---:|---:|---:|---:|---:|---:|
| Abstract raft_no_assume | 27.41 | **20.87** | 24.83 | 3.63 | **2.85** | 3.49 |
| raftok | 20.94 | **19.37** | 19.92 | 2.80 | **2.61** | 2.83 |
| Hybrid reliable broadcast | 12.86 | 12.75 | **6.24** | 1.97 | 2.00 | **1.01** |
| TLB | 99.96 | 100.12 | **69.55** | 13.03 | 13.41 | **10.00** |
| Flash | 8.53 | 8.49 | **4.54** | 1.35 | 1.33 | **0.77** |
| Nested ticket | 3.95 | 3.96 | **2.22** | 0.93 | 0.93 | **0.56** |
| Apple dramc_nb2 | 152.92 | **152.93** | 162.07 | 19.45 | **20.41** | 22.83 |
| Apple rfn2 | 14.74 | **14.68** | 51.78 | 2.12 | **2.10** | 13.45 |

These are medians from five alternating complete runs per build per input, at default GC with one runtime CPU and the same GoIvy frontend at `75170dd71`. Each input ran on the same pinned processor for all three builds. The earlier seven-pair prototype panel independently measured abstract raft_no_assume at 27.43 → 20.84 billion versus 24.82 billion, and raftok at 20.92 → 19.41 versus 19.93 billion. On raftok, allowing only proofs that the model satisfied the quantifier took 23.92 billion instructions. Returning usable counterexamples mattered as well. [Both panels, 288 processes](perf/ivy_apple/holdouts/nov23-wide-models.csv.gz) · [192 design trials](perf/ivy_apple/holdouts/nov23-wide-model-trials.csv.gz)

The domain includes the named values and enough anonymous representatives to distinguish the bound variables by equality. A universally true body disposes of the auxiliary question. For a false body, the checker finds the first concrete failing assignment in the original enumeration order and re-evaluates it with the scalar evaluator. An inconclusive result falls back to ordinary model-based quantifier instantiation. The diagram admits at most six binders and 64 representatives per binder, with limits of 4,096 nodes and 100,000 work units. The checker polls cancellation during construction and before reporting a result. [Bounds and witness regressions](smtcore/smt_model_table_diagram_test.go)

A separate verifier build compared every completed diagram decision on both abstract Raft inputs and hybrid broadcast with exhaustive scalar checking. All **1,207** decisions agreed, including the exact first failing tuple whenever one existed. The largest domain product was 648,960 assignments. Another instrumented run submitted the original auxiliary questions to g3's ordinary auxiliary solver and replayed their typed SMT-LIB against native Z3. All 716 certificates agreed, covering 563 questions after deduplication within each input. The ordinary model evaluator also confirmed all 206 concrete counterexamples. On the independent 87-spec corpus, every production exit status and proof transcript matched linked Z3. [Exhaustive comparisons](perf/ivy_apple/holdouts/nov23-wide-model-enumeration.csv.gz) · [Auxiliary results](perf/ivy_apple/holdouts/nov23-wide-model-certificates.csv.gz) · [Witness and certificate counts](perf/ivy_apple/holdouts/nov23-wide-model-audit.csv.gz) · [Typed questions](perf/ivy_apple/holdouts/nov23-wide-model-questions.tar.gz) · [Production corpus](perf/ivy_apple/holdouts/nov23-wide-model-corpus.csv.gz) · [Prototype corpus](perf/ivy_apple/holdouts/nov23-wide-model-candidate-corpus.csv.gz)

For a four-variable congruence formula over 16 values, exhaustive checking took **1,248.24 µs** and the diagram took **23.98 µs**, a 52× speedup for this operation. At six values per variable the medians were 31.66 and 10.10 µs. The diagram allocated 16,496 bytes at width 16 and 7,584 at width six. Scalar enumeration allocated zero bytes in both timed loops. The complete Raft improvements above include setup, fallback and ordinary search. [Retained benchmark](smtcore/smt_model_table_diagram_test.go) · [Nine samples per measurement](perf/ivy_apple/holdouts/nov23-wide-models.bench.gz)

```sh
go test ./smtcore -run 'TestModel(Table|Diagram)' -bench '^BenchmarkModelTableCongruence$' -benchmem -cpu=1 -benchtime=15x -count=9
tar -xzf perf/ivy_apple/holdouts/nov23-wide-model-questions.tar.gz -C /tmp
python3 perf/ivy_apple/holdouts/model_tables_replay.py --questions /tmp/raftok.csv.gz \
  --native ../build/z3 --cpu 2 --output /tmp/raftok-model-replay
```

## Three Raft cores with matched search statistics

I extracted three premise subsets from native unsatisfiable cores of real abstract raft_no_assume checks. Each archived input has its own base and negative goal, reproduced through the normal incremental SMT-LIB shells. On five alternating pairs, the answer, decision count, quantifier instantiations, conflicts and resource count matched across both engines on every run. These controls isolate implementation cost much more closely than two runs that merely prove the same goal. [Inputs](perf/ivy_apple/test_vectors/raft_cores) · [All 30 observations](perf/ivy_apple/holdouts/nov23-matched-raft-cores.csv.gz) · [Replay driver](perf/ivy_apple/holdouts/query_replay.py)

| Original check | Premises | g3, million instructions | Z3, million instructions | Ratio | Decisions | Instances | Conflicts | Resource count |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 251 | 41 | 802.11 | 271.99 | 2.95× | 835 | 2,111 | 186 | 147,026 |
| 274 | 50 | 1,064.92 | 339.29 | 3.14× | 1,114 | 1,185 | 103 | 178,671 |
| 275 | 54 | 1,041.15 | 324.53 | 3.21× | 1,139 | 1,127 | 131 | 184,312 |

I also bracketed operations on the solver thread during the **original complete proofs**, before the wide-model change, using g3 `4ea7abff8` and GoIvy `da98895f8`. The original check 251 in batch 6, condition 21, visited 46,360 g3 queue candidates and 40,820 native candidates. g3 performed 26,163 instance rewrites at **21,417 instructions per rewrite**, versus 12,511 at **6,747** for Z3. It performed 22,001 lemma internalizations at **30,001 instructions each**, versus 11,058 at **6,211**. The inclusive queue cost was **2.192 versus 0.450 billion**. On original check 274, internalization cost **32,879 versus 7,008 instructions per call** across 6,563 and 4,217 calls respectively. The complete-input TLB queue consumed **20.35 versus 27.79 billion** because g3 reached fewer candidates there. The queue totals include nested stages. Separate sampling runs identify hot functions, while the exact instruction counts come from the hardware-counter brackets and whole-process totals. [Stage totals for ten inputs](perf/ivy_apple/incremental_cost/nov23-qi-phase-totals.csv.gz) · [Per-check stage rows](perf/ivy_apple/incremental_cost/nov23-qi-phase-rows.tar.gz) · [Replay function profiles](perf/ivy_apple/holdouts/nov23-matched-raft-profiles.tar.gz) · [Stage driver](perf/ivy_apple/incremental_cost/qi_stage_costs.py) · [Native counter wrapper](../src/test/perf/ivy_qi_stages.cpp)

```sh
go build -o /tmp/g3-raft-replay ./cmd/g3
GOMAXPROCS=1 python3 perf/ivy_apple/holdouts/query_replay.py --suite raft-cores \
  --g3 /tmp/g3-raft-replay --native ../build/z3 --cpu 2 --pairs 5 --output /tmp/raft-core-replay
python3 perf/ivy_apple/incremental_cost/qi_stage_costs.py --goivy /path/to/ivy/goivy \
  --spec /path/to/ivy-lang-examples/examples/liveness/ticket.ivy --abc /path/to/abc \
  --cpu 2 --pairs 3 --output /tmp/ticket-queue-cost
```

One first-push prototype tried to reuse earlier rewrites that had returned the identical input AST. Checking only the paths previously visited predicted 154 incorrect hits. Checking the full syntax, including quantifier patterns, found 117,564 genuine identity results on TLB covering 1,466,128 rewrite steps. Even with matching AST results, **4,042 hits had changed resource charges**. Replaying the old charges would have counted 1,594,864 units against an actual 1,583,692. The scanning prototype reduced the complete TLB median from 100.21 to 98.06 billion instructions. The current implementation retains the ordinary rewrite and its actual cancellation accounting. Reducing the cost of each visit remains applicable to both first-push preprocessing and the three equivalent-search controls. [First-push differential checks](perf/ivy_apple/incremental_cost/nov23-first-push-identity-oracle.csv.gz) · [252 whole-verifier trials](perf/ivy_apple/incremental_cost/nov23-first-push-identity-trials.csv.gz)

## Frontend symbol collection and inactive tracing

GoIvy `da98895f8` checks whether binder tracing is enabled before constructing the exit message's arguments. On dramc_nb2 it eliminated approximately 38.8 MB and 340,000 allocations over the complete run. GoIvy `75170dd71` then changed clause symbol collection to insert each encountered symbol directly into the final insertion-ordered dictionary. This preserves the first position of each structural key and the last representative stored for it. Intermediate per-formula dictionaries had previously been built and copied even when they contained a single symbol. [Trace guards](https://github.com/glycerine/ivy/commit/da98895f8) · [Collector and benchmark](https://github.com/glycerine/ivy/commit/75170dd71)

| Complete verification | Previous g3, billion instructions | New g3 | Previous linked Z3 | New linked Z3 |
|---|---:|---:|---:|---:|
| Apple dramc_nb2 | 155.16 | **152.98** | 164.94 | **162.39** |
| Apple cf_live | 57.51 | **57.20** | 59.05 | **58.70** |
| Apple rfn2 | 14.90 | **14.67** | 52.02 | **51.77** |
| TLB | 100.10 | **99.90** | 69.65 | **69.47** |

Five alternating pairs per backend used g3 `4ea7abff8`; both frontend revisions already had the tracing guards. Direct collection saved roughly **251 MB and 1.33 million allocations** per dramc_nb2 verification with either engine. A 128-distinct-symbol microbenchmark improved from **57.77 to 11.53 µs**, 397 to 13 allocations and 73,968 to 20,720 bytes. With 32 symbols the median improved from 9.11 to 3.05 µs and 105 to nine allocations. Two experiments that also tracked visited AST pointers raised the complete g3 dramc_nb2 instruction count from 154.87 to 189.04 and 200.78 billion. The retained walk uses structural keys at symbol leaves and preserves the original traversal order. [Complete verifier runs](perf/ivy_apple/holdouts/nov23-symbols-and-traces.csv.gz) · [Heap and object counts](perf/ivy_apple/holdouts/nov23-symbols-and-traces-allocations.csv.gz) · [Microbenchmark samples](perf/ivy_apple/holdouts/nov23-clause-symbols.bench.gz) · [Collector design trials](perf/ivy_apple/holdouts/nov23-symbol-collector-trials.csv.gz)

For both changes, enabled tracing on Apple rfn2 preserved the entire 2,334,030,415-byte output. The Paxos Python comparison matched all 27,855 normalized lines, including 27,782 trace records. All 87 standalone specs matched the linked-native reference on exit status and both output streams. [Enabled-trace results](perf/ivy_apple/holdouts/nov23-symbols-and-traces-text.csv.gz) · [Corpus results](perf/ivy_apple/holdouts/nov23-symbols-corpus.csv.gz)

### Constant keys in the verifier

GoIvy commit `d8e2f144a` builds the structural key of a constant directly from its name and sort key. A constructor still captures the sort at creation time. Changing the sort later cannot alter the key already used in a dictionary. The previous constructor passed these same two strings through generic formatting. [GoIvy source and benchmark](https://github.com/glycerine/ivy/commit/d8e2f144a)

On Apple dramc_nb2, the change eliminated **4.81 million allocations and 77 MB per verification** with either backend. The instruction counts below are medians of five alternating, pinned whole-checker pairs with the default Go GC percentage. Separate allocation builds measured four pairs. Both builds used the same g3 solver and the same native Z3 library across the change. [All 140 whole-checker samples](perf/ivy_apple/holdouts/nov23-constant-keys.csv.gz) · [All 80 allocation samples](perf/ivy_apple/holdouts/nov23-constant-key-allocations.csv.gz)

| Complete input | g3 before, billion instructions | g3 after | Linked Z3 before | Linked Z3 after |
|---|---:|---:|---:|---:|
| dramc_nb2 | 153.03 | 148.04 | 162.27 | 156.76 |
| rfn2 | 14.73 | 14.34 | 51.79 | 51.48 |
| cf_live | 57.33 | 56.54 | 58.80 | 58.06 |
| TLB | 99.96 | 99.44 | 69.58 | 68.97 |

The constructor saved two allocations per call for Boolean, uninterpreted and binary-predicate sorts. The enabled rfn2 trace retained all 525,981 lines and 2,334,030,415 bytes. The Python Paxos comparison retained all 27,855 normalized lines, including 27,782 trace records. On 87 further inputs, stdout, stderr and exit status matched the native reference. To run the constructor benchmark in GoIvy, use `go test -tags=xtrace_off -run '^$' -bench '^BenchmarkConstantStructuralKey$' -benchtime=10000x -benchmem -cpu=1 -count=9`. [Benchmark and trace records](perf/ivy_apple/holdouts/nov23-constant-key-controls.tar.gz) · [87 paired corpus results](perf/ivy_apple/holdouts/nov23-constant-key-corpus.csv.gz)

### Additional verification inputs

The next survey tried 418 additional byte-distinct Ivy files. Both backends used GoIvy `75170dd71` and ran on the same pinned CPU per input. In the short screening runs, 79 inputs finished successfully with matching output and at least one passing obligation. Another 42 agreed on a failing verification after at least one passing obligation. There were 289 other matching outcomes, five timeouts and three output mismatches. Those last eight cases are excluded from performance comparisons. The manifest records skipped byte-identical files. Identical source bytes can resolve relative includes differently. [836 process records](perf/ivy_apple/holdouts/nov23-additional-spec-survey.csv.gz) · [Input list, duplicates and batch sizes](perf/ivy_apple/holdouts/nov23-additional-spec-inputs.tar.gz) · [Paired survey driver](perf/ivy_apple/holdouts/spec_survey.py)

### Three long Ivy verifications and replay of the order bottleneck

The longer runs from the extended corpus exposed two order workloads with a larger remaining deficit than Apple `rfn2`. Whole-process counts below are billions of retired user instructions per complete checker invocation. Each pair used the same Ivy frontend, input, CPU and GC percentage. All three pairs matched the linked native backend's exit status and complete output. Contention on the measurement host varied substantially, especially during `ord_live2`. [Raw paired runs and operation-level measurements](perf/ivy_apple/holdouts/nov23-order-and-hermes-measurements.tar.gz)

| Ivy input | Passing obligations | Pairs | g3 | Linked Z3 |
|---|---:|---:|---:|---:|
| `jea/issue73/ord.ivy` | 795 | 4 | 1,081.49 | 497.42 |
| `doc/examples/apple/ord_live2.ivy` | 1,986 | 1 | 4,221.15 | 1,965.26 |
| `jea/hermes_rmw_o3_testing.ivy` | 6,338 | 4 | 1,630.04 | 1,483.71 |

On `issue73/ord.ivy`, the medians of three instrumented runs placed 1,045.41 billion g3 instructions and 484.65 billion native instructions inside verification checks. The first instrumented pair attributed about 400 billion excess check instructions to the 95 siblings in batch 13. On Hermes, the early private proof attempts cost g3 244.01 billion instructions and pushes cost 151.32 billion, versus 90.70 billion for native pushes. The two workloads present different optimization opportunities. These counts precede the frontend constant-key change. [Operation records and Go CPU and allocation profiles](perf/ivy_apple/holdouts/nov23-order-and-hermes-measurements.tar.gz)

For the shell comparison, the input retains one permanent premise followed by the 95 ordered push/assert/check/pop sequences from batch 13. The captured printer introduced different local `let` names across snapshots. Expanding the local bindings confirmed that all 95 base formulas were identical within each of batches 12, 13 and 14. In all three alternating pairs, both shells returned UNSAT 95 times in batch 13. Each reported **16,489 conflicts, 5,672,605 decisions, 26,892,811 propagations and 1,532,393 quantifier instantiations**. Median instructions were **674.98 billion for g3 and 241.66 billion for Z3**, a **2.79×** ratio with identical search counters. The corresponding batch 12 and 14 replays cost 0.28 versus 0.08 billion, and 40.05 versus 12.44 billion. [Shell measurements and sampled stacks](perf/ivy_apple/holdouts/nov23-order-and-hermes-measurements.tar.gz)

The arena slot lookup was the largest flat entry in the first complete g3 CPU profile at 8.12%, followed by the solver expression-table lookup at 5.42% and asserted-disjunction propagation at 3.98%. The isolated batch 13 siblings show that the rewriter, quantifier queue, congruence and internalization all contribute to the gap. Reducing the number of quantifier instances offers a larger ceiling. On condition 66, an offline native UNSAT core replayed in g3 with 231 instances and about 0.57 billion instructions. Asserting the entire split input took 72,896 instances and 56.34 billion. Obtaining that reference core consumes a separate search. [Inputs, including the offline controls](perf/ivy_apple/holdouts/nov23-order-inputs.tar.gz)

Condition 38 gave a prospective example. Matching the goal to a predecessor invariant by formula structure and following its definitions selected five base conjuncts and the goal. The saved input takes about 0.10 billion g3 instructions with six instantiations. The full incremental replay takes 37.91 billion instructions with 73,529 instantiations. Both shells independently prove the selected original assertions UNSAT. A wider screening experiment found 404 such successes among 782 eligible snapshots on this input. The 400 successes that reached the ordinary check in the baseline accounted for 153.36–153.62 billion instructions across three runs. All attempted subset processes, including failures and shell startup, retired 73.25 billion instructions. That comparison excludes selector execution and changes in the subsequent incremental search. The first integrated trial increased the complete verifier count to 1,136.87 billion. The selector remains experimental. [Screening results](perf/ivy_apple/holdouts/nov23-order-and-hermes-measurements.tar.gz)

The saved benchmark alternates the two shells, checks every answer, and reports decisions, conflicts, propagations and instantiations. From `pkg`, run `python3 perf/ivy_apple/holdouts/order_replay.py --list` for the 27 vectors. Supply `--case 13-all`, `--g3 /path/to/g3` and `--z3 ../build/z3` to measure the entire batch. Use `--case 13-38-selected` for the small refutation. The optional `--cpu` and `--perf` arguments enable pinned Linux hardware measurements. [Benchmark driver](perf/ivy_apple/holdouts/order_replay.py)

### Ground enumeration values in finite-model tables

A quantified formula can range over ordinary uninterpreted elements while a function maps those elements to an enumeration. Its default, exceptional rows, or comparisons can contain constructors such as `red` and `blue`. The finite-model checker now assigns a local ID to each canonical ground constructor it encounters. It verifies membership in the actual sort declaration, admits enumerations of up to 64 constructors, and checks the original binder sorts as before. A raw declaration outside the enumeration, an interpreted model placeholder or an unsupported term defers to the ordinary auxiliary solver. The new test checks a concrete counterexample by independently evaluating the instantiated formula in its model. [Implementation](smtcore/smt_model_table.go) · [Tests and paired microbenchmark](smtcore/smt_model_table_enum_test.go)

| Complete verification | g3 before, B instructions | g3 after, B instructions | Pairs |
|---|---:|---:|---:|
| Flash example, with its expected failing diagnostic | 8.38 | **7.06** | 5 |
| `test/flash2.ivy`, all obligations pass | 16.93 | **14.63** | 3 |
| `test/arm_rules3.ivy` | 4.98 | **4.85** | 3 |
| `jea/hermes_rmw_o3.ivy` | **77.71** | 78.94 | 3 |

The two Flash runs used 15.8% and 13.6% fewer instructions. Linked Z3 used 4.48 billion on the first input. Across the seven other five-pair controls the largest median movement was 0.11 billion instructions. On short Hermes the complete process regressed 1.6%. A separate operation-bracketed run reduced the cost of all 3,382 auxiliary calls there from 0.89 to 0.80 billion instructions. The process-level increase falls outside the aggregate cost of those calls. Both backends produced identical output, diagnostics and exit status across all 87 corpus inputs. [Complete measurements, hardware counts and finite-model audit](perf/ivy_apple/holdouts/nov23-ground-enumerations.tar.gz)

The audit independently evaluated **674 concrete falsifying assignments** in their source models. For **5,891 certificates**, it recorded the original auxiliary solver question. Native Z3 returned UNSAT for all **360 byte-distinct questions** across the eleven audited workloads. The Flash inputs contributed 131 and 203 questions, with 14 shared. The repeated unit benchmark measured 12.08 µs for the table and 24.86 µs for the auxiliary solver. The table allocated 4,640 bytes in 38 allocations versus 1,663 bytes in 29 allocations for the warmed auxiliary solver. That allocation cost remains a target for reuse within a solver. Run `go test ./smtcore -run '^$' -bench '^BenchmarkModelTableGroundEnumeration$' -benchmem -cpu=1` to reproduce the comparison. [Native answers, replay inputs and benchmark samples](perf/ivy_apple/holdouts/nov23-ground-enumerations.tar.gz)

### Renaming local Skolem symbols during action composition

GoIvy used to build an insertion-ordered dictionary of every source symbol and an occupied-name set for every destination symbol. Only local Skolems are eligible for this rename, and every candidate destination name contains the original Skolem name. GoIvy now walks both syntax trees directly. It indexes the eligible source constants by their name and sort, and collects only destination names that can collide. For structurally equal constants the walk preserves the first encounter position and the last sort representative. It still makes fresh compound nodes when it renames a clause. [GoIvy implementation, identity regression test and benchmark](https://github.com/glycerine/ivy/commit/463910ad1)

| Complete GoIvy verification | g3 before, billion instructions | g3 after | Linked Z3 before | Linked Z3 after |
|---|---:|---:|---:|---:|
| Apple dramc_nb2 | 148.24 | **142.72** | 156.43 | **150.05** |
| Apple rfn2 | 14.32 | **13.94** | 51.44 | **50.94** |
| Apple cf_live | 56.52 | **55.93** | 58.09 | **57.29** |
| TLB, including the expected diagnostic | 98.96 | **98.12** | 69.13 | **68.03** |

Each column uses the median of five complete runs on the same CPU per input, alternating frontend revisions. The g3 backend was at `dc7a660be` throughout. Four separate allocation pairs on dramc_nb2 measured **452 MB and 1.66 million fewer allocations** per g3 run. A first full run of the earlier prototype on `jea/hermes_rmw_o3_testing.ivy` reduced instructions from **1,578.57 to 1,438.55 billion** with g3 and **1,429.51 to 1,262.04 billion** with linked Z3. All 6,338 obligations passed. That Hermes comparison is one screening pair per backend. [Whole-process counters, allocations, corpus and trace controls](perf/ivy_apple/holdouts/nov23-skolem-renaming.tar.gz)

In nine benchmark samples, renaming a clause with one local Skolem and 96 ordinary symbols fell from **73.23 to 6.67 µs**, **102,792 to 4,544 bytes**, and **529 to 14 allocations** per call. The existing no-local-Skolem benchmark fell from **166.5 to 12.61 ns** and from three allocations to zero. Run `go test -tags=xtrace_off -run '^$' -bench '^BenchmarkRenameDistinctClauses' -benchtime=2000x -benchmem -cpu=1 -count=9` in GoIvy. Both backends matched on stdout, stderr and exit status for 87 further inputs. The enabled rfn2 run preserved the complete 2,334,030,415-byte output stream. Python and Go also agreed on all 27,855 normalized Paxos output lines. [Benchmarks and verification records](perf/ivy_apple/holdouts/nov23-skolem-renaming.tar.gz)

### Reusing finite-model table storage within one solver

The finite-model checker compiles each quantified body against the current candidate model. One checker can do this thousands of times. Its small term, value and function arrays and lookup maps now survive a call. Every exit clears the interpretations, releases the model pointer and discards backing arrays that exceed the retention bounds. A published counterexample owns its bindings. The regression test changes the function interpretation between calls and also interrupts a compilation on an unsupported subformula. [Implementation](smtcore/smt_model_table_check.go) · [Regression and worker-storage benchmarks](smtcore/smt_model_table_enum_test.go)

| Measurement | Before | After |
|---|---:|---:|
| Retired instructions per ground-enumeration check | 51,447 | **34,881** |
| Elapsed time per check | 6.97 µs | **3.98 µs** |
| Bytes allocated per check | 4,640 | **320** |
| Allocations per check | 38 | **15** |
| Flash example, billion process instructions | 6.98 | **6.78** |
| Abstract Raft, billion process instructions | 20.70 | **20.38** |
| Ordinary `raftok`, billion process instructions | 18.86 | **18.54** |
| `test/flash2.ivy`, billion process instructions | 14.56 | **14.31** |

The first four rows use nine alternating pairs of 60,000 checks. The hardware counts include benchmark process startup divided by 60,000. The complete Flash and Raft examples use five alternating pairs each with GoIvy `463910ad1`. The test-directory Flash input has three pairs. On dramc_nb2 the median moved from 142.56 to 142.67 billion instructions. Linked Z3 used 4.46 billion instructions on the Flash example, 24.76 billion on abstract Raft and 19.85 billion on ordinary Raft. A separate prototype stored all argument lists and function rows in two additional buffers. It lowered the microbenchmark to ten allocations and saved about 3% more instructions there. Its complete verification counts were close to those of the simpler implementation. [All measurements and experimental variants](perf/ivy_apple/holdouts/nov23-model-table-storage.tar.gz)

In twelve verification runs the reused and freshly compiled tables agreed on all **53,626 calls**, including the exact ordered values in **4,504 concrete counterexamples**, **47,915 certificates** and **1,207 inconclusive results**. All 87 corpus comparisons matched the native reference on output, diagnostics and exit status. The 1,024-checker benchmark measured **2,608 live bytes per active checker** for this small fixture after collection. It measures scratch storage separately from the solver and arena. Run `go test ./smtcore -run '^$' -bench '^BenchmarkModelTableWorkerScratch$' -benchtime=1x -cpu=1 -count=7` to reproduce the retention measurement. [Audit patch, individual decisions and benchmark samples](perf/ivy_apple/holdouts/nov23-model-table-storage.tar.gz)

### Evaluating the default quantifier-queue cost

Z3 accepts arithmetic expressions for the cost and generation of a quantifier instance. On the default configuration, g3 was walking two parsed ASTs for each pair of calls to compute a sum and return the cost. The queue now evaluates those two expressions directly. It still parses and retains the configured expressions, validates each quantifier and pattern, rounds the operands to float32 and updates the maximum cost for the quantifier. Custom expressions and the warning-and-fallback cases continue through the evaluator. [Queue implementation](smtcore/smt_qi_cost.go) · [Equivalence test and benchmark](smtcore/smt_qi_cost_test.go)

| Measurement | Before | After |
|---|---:|---:|
| Instructions per cost and generation pair | 1,761 | **780** |
| Time per pair | 167.8 ns | **76.2 ns** |
| Apple order batch 14, condition 132, billion instructions | 396.82 | **392.45** |
| Apple order batch 14, condition 104 | 215.09 | **212.90** |
| Apple order batch 13, condition 132 | 94.30 | **92.94** |
| Issue 73 order batch 13, condition 13 | 43.27 | **42.28** |
| Complete TLB verification | 98.27 | **97.30** |

The unit benchmark uses nine alternating pairs of 1.5 million operations and includes process startup amortized across the iterations. Both revisions allocated zero bytes per operation. Run `go test ./smtcore -run '^$' -bench '^BenchmarkSMTQICostDefaults$' -benchtime=1500000x -benchmem -cpu=1`. All nine standalone obligations used three alternating pairs and matched on verdicts, conflicts, decisions, propagations and quantifier instantiations. Across eight complete Ivy verifications with five pairs each, rfn2 was essentially flat at 13.94 versus 13.98 billion instructions. The 2.26× local improvement saved 1.1% of all instructions on the hardest standalone Apple condition. [Per-run counters and alternative implementations](perf/ivy_apple/holdouts/nov23-qi-default-cost.tar.gz)

The complete 87-input corpus matched linked Z3 on output, diagnostics and exit status. The first linked-Z3 attempt on `raft_no_assume_test.ivy` reached 320 checks before its 240-second wall-clock limit on the shared CPU. Repeating the pair on that CPU with a 1,200-second limit completed all 419 checks on both engines. The archive includes the initial attempt and the retry. [Corpus results and tagged focused tests](perf/ivy_apple/holdouts/nov23-qi-default-cost.tar.gz)

### Formatting formulas for silent GoIvy traces

GoIvy commit `704b800e5` checks whether a trace can emit output before building its arguments. Allocation profiles identified calls that formatted formulas during variable lowering, sort translation, proof checking and temporal reduction. One temporal diagnostic also walked the entire list of axioms to print them. Both solver backends use this frontend.

| Complete verification | g3 before, billion instructions | g3 after | Linked Z3 before | Linked Z3 after |
|---|---:|---:|---:|---:|
| Apple rfn2 | 13.89 | **12.63** | 50.95 | **49.64** |
| Apple dramc_nb2 | 142.71 | **139.55** | 150.03 | **146.37** |
| Apple cf_live | 55.67 | **54.03** | 57.18 | **55.65** |
| TLB | 97.95 | **97.09** | 68.05 | **67.06** |

Each figure is the median of five runs on the same CPU for that input, alternating the frontend builds. Four separate allocation pairs measured a drop from 10.318 to 9.970 GB per complete g3 run on dramc_nb2 and from 1.054 to 0.964 GB on rfn2. All 87 paired corpus inputs matched linked Z3 on output, diagnostics and exit status. With tracing enabled, both frontend revisions emitted identical 2,334,030,415-byte rfn2 output. The Paxos control also matched Python across all 27,855 normalized lines. [Process counters, allocations, corpus and trace checks](perf/ivy_apple/holdouts/nov23-trace-argument-guards.tar.gz)

### Declaration IDs in the quantifier pattern matcher

The g3 label hasher allocated exactly enough space for each new maximum declaration ID. Registering consecutive IDs copied all earlier entries at every step. The matcher now uses slice capacity growth and initializes newly exposed entries to the existing unassigned value. A sparse-ID regression checks both the hash and the diagnostic listing. [Label hasher](smtcore/smt_mam_code.go) · [Regression and benchmark](smtcore/smt_mam_label_hash_test.go)

| Consecutive IDs per matcher | Instructions before | Instructions after | Bytes allocated before | Bytes allocated after |
|---|---:|---:|---:|---:|
| 4,096 | 11,257,206 | **622,091** | 8,943,472 | **12,536** |
| 32,768 | 481,996,447 | **4,959,103** | 565,540,720 | **154,360** |

The 32,768-ID case used **97× fewer instructions** and made 20 allocations instead of 32,768. Each instruction count is the median of seven paired slopes. Subtracting a short invocation from a longer invocation on the same CPU removes fixed process startup. Reproduce the allocation comparison with `go test ./smtcore -run '^$' -bench '^BenchmarkSMTMAMLabelHash$' -benchmem -cpu=1`. Real proof inputs visited IDs in a different order. On issue 73, 28,263,367 hash calls triggered just 324 resizes and copied 132,322 bytes in the previous implementation. On Apple batch 14, condition 132, 52 resizes copied 33,144 bytes. The large speedup applies to dense registration in ascending order. [Hardware counts, allocation samples and twelve real-input growth counts](perf/ivy_apple/holdouts/nov23-mam-label-growth.tar.gz)

### Replaying all 132 siblings of the Apple order bottleneck

On `doc/examples/apple/ord_live2.ivy`, both shells proved all 132 obligations in batch 14. Across two alternating pairs, both reported exactly 15,266 conflicts, 12,914,622 decisions, 36,736,635 propagations and 4,479,904 quantifier instantiations. g3 retired **1,862.61 billion instructions** and Z3 **680.91 billion**, a 2.74× gap doing the same search. This is a frozen shell baseline shared with the issue 73 study. The benchmark includes process startup, parsing and every push, check and pop. The Ivy frontend is outside the measurement. [Nineteen replay inputs](perf/ivy_apple/holdouts/nov23-apple-order-inputs.tar.gz) · [Paired counters, core discovery and sampled stacks](perf/ivy_apple/holdouts/nov23-apple-order-replays.tar.gz)

| Standalone proof | g3, billion instructions | Native Z3, billion instructions | Quantifier instances on either engine |
|---|---:|---:|---:|
| Batch 14, condition 132, original | 406.95 | 148.51 | 746,473 |
| Same condition, 190 assertions from an offline native core | 137.84 | 50.58 | 198,152 |
| Same condition, 119 assertions after five further core extractions | 37.75 | 11.75 | 47,580 |
| Batch 14, condition 116, original | 43.90 | 17.04 | 107,497 |
| Same condition, 63 defining premises and the goal | **0.625** | **0.154** | **51** |
| Batch 13, condition 110, original | 2.859 | 0.827 | 8,745 |
| Same condition, 36 defining premises and the goal | **0.178** | **0.055** | **8** |

For condition 132, the first core extraction alone cost native Z3 520.64 billion instructions. The five additional extractions cost another 83.50 billion, before validation. The 119-assertion script gives a reproducible lower-work proof for experiments. On condition 116 the selector needed only the input. It started with the goal's declarations and followed five layers of defining equations, preserving their original assertion order and lexical bindings. Three layers sufficed for batch 13 condition 110. Both engines independently proved the emitted assertions UNSAT. Timing in the table includes solving the selected input. Selector construction, unsuccessful attempts and changes to later incremental search must also be charged in a whole-verifier comparison. [Selector](perf/ivy_apple/holdouts/order_definitions.py)

Three instruction-sampled profiles per engine place **68.75 billion** excess instructions in the rewriter and **58.03 billion** in quantifier handling on condition 132. Together they account for about half the 258.44 billion standalone gap. Internalization adds 26.09 billion and congruence handling adds 20.68 billion. Each sample belongs to its first classified stack frame. The periods, sample totals and leaf counts are in the archive. The evidence favors reducing unnecessary instances on expensive siblings and measuring the cost of each retained instance on exactly matched searches. [Profile tables and per-run counters](perf/ivy_apple/holdouts/nov23-apple-order-replays.tar.gz)

From `pkg`, run `python3 perf/ivy_apple/holdouts/order_replay.py --suite apple --list` to list the inputs. Use `--case 14-all --g3 /path/to/g3 --z3 ../build/z3 --rounds 2` for the complete batch, or `--case 14-116-definitions` for the smaller proof. To reproduce the latter input, pipe `14-116.smt2` from the input archive into `python3 perf/ivy_apple/holdouts/order_definitions.py --depth 5`. [Replay driver](perf/ivy_apple/holdouts/order_replay.py)

### Restoring quantifier matcher labels after backtracking

Each equality merge saved two closures to restore the destination enode's label sets. On Apple batch 14, condition 132, **3,032,452 of 3,200,544 merges** saved the same pair of sets as the preceding merge within a measured span. The matcher now records destinations in order and stores a pair of label sets once per consecutive run. Each ordinary undo callback records its position among the merges. Popping a scope restores both kinds of changes in reverse order, including changes made through the public label accessors. Reset releases the backing arrays. Each matcher owns its history. [Restoration code](smtcore/smt_mam_code.go) · [Rollback tests and merge benchmark](smtcore/smt_mam_merge_trail_test.go)

| Measurement | Before | After |
|---|---:|---:|
| Instructions to push, merge once and pop | 962 | **283** |
| Instructions to push, merge 256 times and pop | 245,635 | **36,749** |
| Allocations per group of 256 merges after warmup | 512 | **0** |
| Active trail bytes per matcher with 256 empty-label merges | 17,160 | **2,344** |
| Active trail bytes per matcher with repeated nonempty labels | 17,160 | **2,368** |
| Condition 104, objects allocated per complete shell invocation | 27,032,244 | **22,387,435** |
| Condition 104, bytes allocated per invocation | 2,196,455,316 | **2,085,313,248** |
| Complete `jea/issue73/ord.ivy`, billion instructions | 825.78 | **816.57** |

The instruction microbenchmarks use eight paired slopes. The storage benchmark keeps 1,024 independent matchers live and measures the heap after collection. After pop, the measured overhead per matcher was 4,872 bytes before and 2,344–2,368 bytes after. After reset, the new trail returned to the measured baseline. The whole issue 73 figures are medians of three alternating pairs with byte-identical output on all 795 checks. Seven standalone proofs completed three pairs each with identical conflicts, decisions, propagations and instantiations. In eight further Ivy comparisons with five pairs each, all verdicts matched. The 87-input corpus matched linked Z3 on stdout, stderr and exit status. [Raw measurements and alternative representations](perf/ivy_apple/holdouts/nov23-mam-merge-trail.tar.gz)

Run `go test ./smtcore -run '^$' -bench '^BenchmarkSMTMAMMergeTrail$' -benchmem -cpu=1` for hot-path cost, or use `-bench '^BenchmarkSMTMAMMergeTrailWorkers$' -benchtime=1x -count=8` for per-worker storage. [Worker benchmark](smtcore/smt_mam_merge_trail_workers_benchmark_test.go)

### Sequence scopes in quantified proofs

Every sequence-theory push saved its axiom queue position in a closure, even when the queue position was zero. That cost also applied to quantified proofs that opened many scopes without processing sequence axioms. Scopes starting at zero now share one stateless undo function. Scopes starting later still capture their own queue position. Both undo functions occupy the same place in the callback order. [Implementation](smtcore/smt_theory_seq_assign.go) · [Nested rollback test and scope benchmark](smtcore/smt_sequence_scope_benchmark_test.go)

| Measurement | Before | After |
|---|---:|---:|
| Instructions per empty sequence-theory push and pop | 2,037 | **1,862** |
| Bytes and allocations per empty scope | 16 B, 1 allocation | **0 B, 0 allocations** |
| Apple batch 13, condition 132, allocations per proof | 10,630,534 | **9,657,802** |
| Apple batch 13, condition 132, MB allocated per proof | 1,077.65 | **1,062.01** |
| Apple batch 14, condition 116, allocations per proof | 4,416,381 | **4,081,873** |

The scope benchmark uses seven alternating pairs of 1.5 million iterations. The proof allocations are medians of four pairs on the compressed-matcher baseline. Six standalone proofs had identical conflicts, decisions, propagations and quantifier instantiations across three pairs each. On an earlier common baseline, the seven complete Ivy cases had matching output and median instruction changes between −0.79% and +0.24%. The measured benefit is the reduction in allocation on heavily backtracking proofs. Run `go test ./smtcore -run '^TestSMTSequenceScopesRestoreAxiomHeads$' -bench '^BenchmarkSMTSequenceEmptyScope$' -benchmem -cpu=1`. [Paired counters and allocations](perf/ivy_apple/holdouts/nov23-sequence-scope-undo.tar.gz)

### Dense variable assignments in nonlinear real arithmetic

Assigning variables in ascending order made g3 allocate and copy its entire value and flag arrays at every new variable. Native Z3's vectors retain spare capacity. On a satisfiable problem with 8,192 independent real variables, g3 performed 16,384 value-array relocations, copying 1,073,610,752 bytes of algebraic cells. It copied another 67,100,672 bytes of flags. The new Go vectors grow geometrically and still expose only the requested variables. Both old and new storage count against the owner budget during relocation. A tight budget can buy exactly the requested size. [Implementation](nlsat/assignment.go) · [Ownership, budget and benchmark controls](nlsat/assignment_growth_test.go)

| Assignment benchmark | Before | After |
|---|---:|---:|
| Instructions to set 1,024 variables and close | 12,881,469 | **2,476,323** |
| Instructions to set 8,192 variables and close | 827,898,265 | **30,400,549** |
| Bytes allocated for 8,192 variables | 599,592,784 | **644,584** |
| Allocations for 8,192 variables | 16,385 | **45** |

| Complete satisfiability check | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 8,192 independent variables, `qfnra-nlsat` | 67.76 | **64.48** | 1.52 |
| Same assertions, explicit CNF and NLSAT pipeline | 48.55 | **44.33** | 0.91 |
| 512 ordered variables, `qfnra-nlsat` | 3.08 | 3.16 | 0.14 |
| 512 coupled variables, `qfnra-nlsat` | 3.61 | 3.68 | 0.16 |

Instruction slopes use eight pairs per microbenchmark. The complete checks use three alternating pairs, including shell startup and shutdown. The local improvement is **27.2×** at 8,192 variables. The complete checks saved **4.8%** and **8.7%** at that size. The smaller ordered and coupled controls used 2–3% more instructions in this sample. All nine generated inputs matched Z3 on verdict, nonlinear propagations, stages and resource steps. The default 8,192-variable check took 8,193 stages and 1,105,921 steps on both engines. Another 42 repository vectors preserved g3's stdout, stderr and exit status. These arithmetic runs do not measure Ivy's quantified order proofs. [Raw hardware counters, shell controls and profiles](perf/nlsat/nov23-assignment-growth.tar.gz)

On the default 8,192-variable input, sampled stacks and the increasing instructions per variable point to repeated searches for reusable arena slots in polynomial and algebraic-number construction. The remaining whole-process gap exceeds 40× with identical nonlinear search counts. That makes the location search a higher priority on this family than further tuning assignment copies. Reproduce the shell input with `python3 perf/nlsat/assignment_growth.py 8192 > /tmp/g3-assignment.smt2`, then run each shell with `-st -smt2 /tmp/g3-assignment.smt2`. Run `go test ./nlsat -run '^TestAssignment' -bench '^BenchmarkNLSATAssignmentGrowth$' -benchmem -cpu=1` for the assignment-only comparison. [Input generator](perf/nlsat/assignment_growth.py)

### Finding reusable polynomial and algebraic-number slots

On the 8,192-variable nonlinear check, g3 inspected **1,678,729,211 polynomial locations** and **268,451,840 monomial locations**. Each allocation started at the beginning of its directory, even when earlier slots were all live. Each owner now remembers the earliest position worth searching. Retiring a slot lowers that position. Allocation still returns the lowest reusable slot and advances its generation in the original order. Univariate managers track descriptor pages separately, and the physical coefficient store tracks its own backings when managers exchange polynomials. [Polynomial slot allocation](math/polynomial/polynomial_store.go) · [Shared coefficient storage](math/polynomial/upolynomial_storage.go)

| Measurement | Before | After |
|---|---:|---:|
| Polynomial directory inspections during the 8,192-variable check | 1,678,729,211 | **278,521** |
| Monomial directory inspections during the same check | 268,451,840 | **32,768** |
| Instructions to find and publish 8,192 polynomial slots in preallocated storage | 235,390,544 | **743,645** |
| Instructions to find and publish 8,192 monomial slots in preallocated storage | 336,267,562 | **967,241** |

| Complete nonlinear check | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 512 ordered variables | 3.13 | **3.00** | 0.14 |
| 512 coupled variables | 3.63 | **3.33** | 0.16 |
| 2,048 independent variables | 10.27 | **8.92** | 0.39 |
| 8,192 independent variables | 65.26 | **42.88** | 1.52 |
| 8,192 independent variables, explicit CNF pipeline | 44.75 | **29.39** | 0.91 |
| 16,384 independent variables | 190.28 | **103.06** | 3.03 |

The final binaries reduced instructions by **34.3% at 8,192 variables** and **45.8% at 16,384** on the default pipeline. These are medians of three alternating whole-shell pairs against g3 `5eea9809f`. The ten generated cases matched native Z3 on verdict, nonlinear propagations, stages and resource steps. Both Go builds gave identical stdout, stderr and exit status on 42 additional repository vectors. The focused tests exercise out-of-order reuse, generation exhaustion, failed allocation cleanup and the return of foreign coefficient storage through both deletion and growth. At 16,384 variables g3 still retired 34.0 times as many instructions as native. In the separately instrumented 8,192-variable run, algebraic cells still inspected 100.76 million locations and the two univariate directories inspected 135.53 million between them. [All counters, ablations and controls](perf/nlsat/nov23-polynomial-location-growth.tar.gz)

Run `go test ./math/polynomial -run '^$' -bench '^BenchmarkPolynomialLocationGrowth$' -benchmem -cpu=1` for the directory measurements. For the complete checks, generate the input with `python3 perf/nlsat/assignment_growth.py 16384` and pass it to each shell with `-st -in -smt2`. [Benchmark and reuse tests](math/polynomial/polynomial_location_growth_test.go) · [Input generator](perf/nlsat/assignment_growth.py)

### Releasing shell function declarations in dictionary order

Resetting a shell with 65,536 Boolean constants took **105.18 billion instructions** in g3. For every declaration, g3 searched the owning reference trail from the beginning. Native Z3 visits the symbol dictionary in table order and releases each declaration directly. The Go dictionary now stores the index of the owning row. Overloaded declarations keep that index in their existing table entry. The common single-declaration record shrank from 24 to 16 bytes on amd64. Scope pop erases the dictionary entry before the trail index can be reused. [Implementation](cmd_context/symbol_dictionary.go) · [Scope reuse and interrupted release test](cmd_context/function_declaration_reset_test.go)

| Complete shell workload | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 128 constant names | 0.251 | 0.249 | 0.103 |
| 8,192 constant names | 2.101 | **0.418** | 0.130 |
| 32,768 constant names | 22.910 | **0.931** | 0.216 |
| 65,536 constant names | 105.182 | **1.606** | 0.330 |
| 16,384 names with two overloaded functions each | 20.773 | **1.076** | 0.261 |
| 8,192 names with pop and redeclaration | 2.100 | **0.416** | 0.118 |
| 8,192-variable nonlinear arithmetic check | 43.162 | **41.167** | 1.520 |

These are medians of three alternating pairs against g3 `72accece6`, including parse, both checks and reset for declaration inputs. The 65,536-name workload uses **65.5× fewer instructions**. The nonlinear workload spends most of its time solving and saves **4.6%** overall. This change affects the shell's declaration lifecycle. The Ivy verification API has a different path. All 27 inputs preserved g3's exit status and diagnostics, allowing for the printed wall-clock flip rate on one stochastic-search trace. Native and Go agreed on every verdict. The complete command-context and CLI package tests passed before and after. Reproduce with `python3 perf/shell/declaration_reset.py 65536 | ./g3 -in -smt2`. [Input generator](perf/shell/declaration_reset.py) · [Paired counters, test results and trace comparison](perf/shell/nov23-function-declaration-reset.tar.gz)

### Constant-time checks of polynomial factorization history

A long-lived polynomial manager kept the frames from earlier factorizations. Verifying one receipt and testing whether an owner was busy scanned this history up to three times. A successful frame allocation already assigns a consecutive invocation number. g3 now uses it to locate the original frame, verifies its pointer and owner, and counts executing and finishing frames on each manager and target. The counters cover nested calls and resumed cleanup. The historical frames remain in their owner's arena accounting. [Receipt and cleanup implementation](math/polynomial/polynomial_factor_core.go) · [Ownership tests and benchmark](math/polynomial/polynomial_factor_history_test.go)

| Historical frames | Instructions for one receipt lookup and both busy checks, before | After |
|---:|---:|---:|
| 128 | 4,330 | **111** |
| 1,024 | 33,899 | **111** |
| 8,192 | 270,443 | **111** |

| Complete nonlinear check | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 2,048 independent variables | 9.133 | **8.568** | 0.386 |
| 8,192 independent variables | 40.790 | **37.083** | 1.520 |
| 8,192 independent variables, explicit CNF pipeline | 26.936 | **25.282** | 0.911 |
| 16,384 independent variables | 99.046 | **78.547** | 3.033 |

The complete 16,384-variable workload saved **20.7%** against `36012b25c`. The microbenchmark measures the slope between two iteration counts, with five pairs per size and sufficient iterations to amortize directory construction. The shell measurements use three pairs per input. All ten generated inputs matched native on verdicts, stages, propagations and resource steps. Another 42 repository vectors preserved the Go output, and the polynomial and NLSAT package suites passed. Reproduce the lookup benchmark with `go test ./math/polynomial -run '^$' -bench '^BenchmarkPolynomialFactorHistory$' -benchmem -cpu=1`. [Paired counters, ablations and test results](perf/nlsat/nov23-factor-frame-lookup.tar.gz)

### Releasing named sorts and datatype declarations

Concrete sort aliases, parser-supplied sorts and datatype definitions had a second quadratic teardown path. Reset visited the sort dictionary in native table order, then scanned the entire sort trail for each name. Each declaration now records its one-based owning row at insertion. The dictionary holds the user-sort flag inline, removing a separate allocation. Parametric constructors retain their independent instantiation cache. The release checks the row's current name and index because a partially completed pop can retire trail rows before erasing dictionary entries. [Implementation](cmd_context/symbol_dictionary.go) · [Partial-pop and constructor ownership test](cmd_context/sort_declaration_reset_test.go)

| Complete shell workload | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 8,192 concrete aliases | 3.386 | **0.411** | 0.131 |
| 32,768 concrete aliases | 41.003 | **0.892** | 0.217 |
| 65,536 concrete aliases | 185.723 | **1.594** | 0.332 |
| 8,192 aliases with pop and redeclaration | 3.395 | **0.422** | 0.118 |
| 8,192 sorts, each with an alias and a constant | 6.748 | **0.813** | 0.208 |
| 2,048 nullary datatypes | 0.575 | **0.426** | 0.154 |
| 8,192 unused parametric definitions | 0.585 | 0.596 | 0.156 |

The largest alias case used **116.5× fewer instructions** across the complete process. The benefit scales with the number of owning trail rows. The parametric control has no such rows, and measured 1.9% higher in this sample. All ten generated inputs agreed with native across three pairs. Another 47 repository vectors preserved complete Go output and exit status, including 12 with a completed native comparison. On one intentional parser-error fixture the native executable terminated before its final check. Both Go builds produced the same diagnostics and completed that fixture. The command-context and CLI suites passed. Reproduce with `python3 perf/shell/declaration_reset.py 65536 --shape alias | ./g3 -in -smt2`. [Benchmark generator](perf/shell/declaration_reset.py) · [Counters and exact test scope](perf/shell/nov23-sort-declaration-reset.tar.gz)

### Invalidating pooled arena objects when their chunk is released

Releasing a chunk scanned the allocator's entire object directory to find its members. With many live chunks, reset visited most directory entries once per chunk. The existing 32-bit parent slot now links members of the same chunk. Size-class reuse keeps that link, and physical chunk release walks it once. The object record occupies the same space. Direct allocations and borrowed raw-pointer entries keep their independent lifetimes. [Allocator implementation](util/memory/small_allocator.go) · [Mixed-class regression and benchmark](util/memory/small_allocator_chunk_members_test.go)

| Objects allocated and reset per iteration | Instructions before | Instructions after |
|---:|---:|---:|
| 128 | 241,841 | 209,479 |
| 4,096 | 6,156,737 | 3,409,660 |
| 32,768 | 182,387,957 | 26,228,256 |
| 65,536 | 670,433,833 | **51,825,082** |

The largest lifecycle used **12.9× fewer instructions**, including all allocations in fifteen size classes. On the complete 8,192-variable nonlinear check, instructions fell from 35.894 billion to **34.613 billion**, a 3.6% saving. The 16,384-variable check fell from 78.124 to **76.983 billion**, a 1.5% saving. All six whole-shell inputs matched native on verdict, propagations, stages and resource steps. Both Go builds preserved the output and exit status of 42 additional repository vectors. The memory, polynomial and NLSAT package suites passed, including a checked-build run of the memory suite.

Run `go test ./util/memory -run '^$' -bench '^BenchmarkSmallAllocatorChunkLifecycle$' -benchmem -cpu=1`. The table uses six paired instruction-count slopes against `d92baa7d8`. The complete solver results use three pairs per input. [Hardware counts and verification records](perf/nlsat/nov23-chunk-membership.tar.gz)

### Exact integer witnesses before nonlinear preprocessing

A satisfiable polynomial system can have an easy integer witness even when the general nonlinear solver goes on to isolate thousands of real roots. On 16,384 independent square inequalities, ordinary g3 previously entered 16,385 NLSAT stages. It now finds a concrete assignment before simplification and validates it against the original goal. The complete shell check took **0.271 s**, versus **13.821 s** for the previous g3 and **0.465 s** for native Z3. [Witness search and certification](nlsat/tactic/integer_candidate.go) · [Bounds and exact comparisons](nlsat/tactic/integer_candidate_bounds.go)

| Complete check | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 16,384 independent variables, ordinary check | 90.199 | **1.420** | 3.136 |
| Same formula, explicit `qfnra-nlsat` | 77.539 | **1.420** | 3.033 |
| 2,048 variables with shuffled positive and negative interval bounds | 29.660 | **0.466** | 1.128 |
| 512 variables in a strict order | 2.974 | **0.094** | 0.152 |
| 512 adjacent products subject to lower bounds | 3.507 | **0.103** | 0.171 |
| 512 Boolean disjunctions over polynomial comparisons | 3.817 | **0.167** | 0.209 |
| 512 fixed values with coupled equations | 0.398 | **0.149** | 0.056 |
| 512-variable contradiction | 0.083 | 0.086 | 0.020 |
| 128 variables, one forced irrational root, explicit `qfnra-nlsat` | 0.607 | 0.600 | 0.033 |

The procedure reads unconditional bounds and propagates order constraints across variables. Handling SMT-LIB unary minus mattered. On the 2,048-variable interval workload the version without signed-literal recognition took 29.516 billion instructions. The final version uses exact integer arithmetic for simple comparisons and the existing model evaluator for more complex expressions, with model completion disabled. A rejected assignment proceeds to the ordinary solver. The search is local to the tactic invocation and capped by input size, depth and estimated intermediate integer width. Proofs, explicit resource limits and tracing use the existing portfolio. The dedicated `nlsat` tactic retains its original contract.

The table gives medians from three alternating pinned runs on the Threadripper 3960X with Go 1.27.3, PGO off, and this fork's native Z3 4.7.1. Counts and elapsed times include parsing, solver construction, model handling, statistics and teardown. Timed alternatives in the previous ordinary portfolio can change how much work it completes on a busy CPU. The explicit-route row avoids that timer and still shows a **54.6×** instruction reduction. The gap on the forced irrational root and the roughly 4% overhead on the small contradiction are useful controls. These are nonlinear arithmetic workloads, measured separately from Ivy verification. [All individual counters, models and controls](perf/nlsat/nov23-certified-integer-models.tar.gz)

Independent validation covered 21 input families, three sizes, and three solver routes. For each returned model, the harness bound **every** variable to its printed exact value and asked native Z3 to solve the original assertions again. Across the 189 checks, native accepted 8,374 variable assignments. All 27 UNSAT controls agreed, and both default solvers returned unknown on three mixed-sort checks. Selecting the nonlinear portfolio explicitly produced validated models on those cases. The seven incremental, resource, core and proof controls preserved output, as did 42 repository vectors. The AST, NLSAT tactic, nonlinear portfolio and CLI suites passed. Four historical native-comparison fixtures still differ on malformed datatype declarations, an out-of-range regular-expression bound, and a truncated OPB input. We reproduced all four with the baseline binary. [Model validation and workload generator](perf/nlsat/integer_model_cases.py) · [Tactic regression tests](nlsat/tactic/integer_candidate_test.go)

Reproduce a large ordinary case with `python3 perf/nlsat/integer_model_cases.py 16384 | ./g3 -in -smt2`. To check models from 21 families, run `python3 perf/nlsat/integer_model_cases.py 53 --verify ./g3 ../build/z3`. Repeat with `--route qfnra` and `--route nlsat-portfolio` to exercise both named strategies.

### Positive integer powers in nonlinear arithmetic

The same polynomial can enter the solver as `(* x x)` or `(^ x 2)`. The integer witness search now recognizes both forms. For a positive integral exponent it bounds the size of the exact result before evaluation. A square of a free constant supplies the same magnitude bounds as multiplication. Variable, fractional, nonpositive and excessive exponents continue through the existing solver. This fork exposes the SMT-LIB power operator under `(set-logic ALL)`. [Bounds and tests](nlsat/tactic/integer_candidate_bounds.go) · [Native operator registration](../src/ast/arith_decl_plugin.cpp)

| Complete nonlinear check | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 8,192 independent square bounds, ordinary check | 43.589 | **2.018** | 1.651 |
| 2,048 independent square bounds, explicit `qfnra-nlsat` | 8.532 | **0.250** | 0.406 |
| 512 sums of neighboring second and fourth powers | 3.737 | **0.310** | 0.237 |
| 512 mixed-sign cubic inequalities | 5.263 | 5.326 | 0.337 |
| 128 variables, one forced irrational square root | 0.663 | 0.661 | 0.052 |

The ordinary 8,192-variable case improved **21.6×** in retired instructions. Its median wall time fell from 12.97 s to 0.60 s, versus 0.43 s for Z3. The explicit 2,048-variable portfolio took 0.112 s versus 0.130 s for Z3. The mixed-sign cubic case still reached NLSAT, which used 513 search stages in both g3 builds. The measurements use three alternating pinned rounds against `471137617`. Independent native replay covered five new input families, three sizes and three solver routes. Z3 accepted all 1,608 returned variable assignments and agreed on all nine contradictions. [Inputs and model-checking driver](perf/nlsat/integer_model_cases.py) · [Raw counts, outputs and model checks](perf/nlsat/nov23-positive-powers.tar.gz)

### Solving repeated independent polynomial systems once

In the 8,192-variable positive-square-root benchmark, each variable must satisfy two assertions, `x*x = 2` and `x > 0`. g3 previously entered a median 8,838 nonlinear search stages. It now solves for one representative and reconstructs the others with the representative's exact algebraic value. The full SMT-LIB check took **0.212 s**, down from **11.412 s**, versus **0.327 s** for native Z3. The three alternating rounds included parsing and teardown on the same pinned CPU. [Benchmark generator and native model check](perf/nlsat/independent_components.py) · [Hardware counts and original inputs](perf/nlsat/nov23-independent-polynomials.tar.gz)

| Complete check | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 8,192 positive square roots, ordinary solver | 81.107 | **1.194** | 2.199 |
| 2,048 positive square roots, ordinary solver | 14.240 | **0.311** | 0.558 |
| 512 variables across seven different radicands, explicit nonlinear portfolio | 3.710 | **0.151** | 0.162 |
| 512 positive cubic roots, explicit portfolio | 5.055 | **0.111** | 0.274 |
| 512 positive roots of a quartic, explicit portfolio | 6.529 | **0.129** | 0.333 |
| 256 variables, two linked by a contradictory strict inequality | 1.049 | **0.084** | 0.065 |
| 512 pairwise distinct radicands, explicit portfolio | 3.649 | 3.597 | 0.146 |

The arithmetic portfolio first attempts its existing certified integer witness. On a miss it scans the original assertions and excludes any variable occurring in a multivariate formula. Two eligible variables can share a model value when their complete sets of unary assertions agree up to renaming. The comparison preserves sort, declaration identity and argument order. The retained subgoal contains original assertions and their original unsat-core dependencies. Each solver builds its own temporary indices. The distinct-radicand control still needed 513 stages and cost about 25 times as many instructions as Z3. [Reduction and model extension](nlsat/tactic/independent_polynomials.go) · [Regression tests](nlsat/tactic/independent_polynomials_test.go)

Across 144 checks at three sizes and three entry points, native Z3 accepted all **6,210 returned values** when bound back into the original assertions and agreed on all 27 UNSAT results. These include opposite signs, repeated assertions in different orders, a shared parameter, mixed constraints and rational exceptions. A separate tracked contradiction returned four named assertions that were independently sufficient for UNSAT. Push/pop produced SAT, UNSAT, SAT in both g3 builds and Z3. All three engines returned UNSAT then UNKNOWN under the two incremental assumption sets. The two g3 builds also produced byte-identical outputs for 42 repository inputs. Reproduce with `python3 perf/nlsat/independent_components.py 49 --verify /path/to/g3 ../build/z3`. [Raw checks and complete timings](perf/nlsat/nov23-independent-polynomials.tar.gz)

### Eliminating equality-defined existential variables in verification conditions

**November 23 frontend correction.** The linked-Z3 executable used in the complete GoIvy comparisons in this section and the subsequent enumeration, Flash and issue73 sections was built at 07:28 UTC. Its build preceded the frontend changes to Skolem renaming, trace formatting and equality-defined goals. The g3 release binaries were built later. Both Flash g3 builds used the same frontend, as did the before-and-after g3 builds for the bounded certificate measurements. The earlier native timings and counters describe the older verifier. Standalone SMT shell replays, proof outcomes and Go API benchmarks have their own inputs and measurements. The paired-build driver records the frontend commit embedded in each executable, its build ID and the local g3 replacement. [Paired build driver](perf/ivy_apple/holdouts/build_verification_pair.py)

Apple `ord_live2.ivy` contains goals of the form `exists x. x = t and P(x)`. Once `x` has been replaced with `t` throughout its scope, no quantifier search is needed to discover that witness. In two reverse-order comparisons on CPU 30, the final g3 implementation reduced the complete 1,986-check verification from **3.542–3.543 to 2.843–2.845 trillion instructions**. Elapsed times were 815 and 900 seconds before the change, 676 and 554 seconds after it, and 714 and 518 seconds with linked Z3. The complete stdout and stderr matched across all six runs. A separate pair on CPU 12 measured 3.539 to 2.839 trillion with the preceding implementation. Linked Z3 used 1.965 trillion in that comparison. [All paired runs, transcripts and equivalence queries](perf/ivy_apple/holdouts/nov23-existential-definitions.tar.gz)

| Workload | g3 before, billion instructions | g3 with elimination | Checks |
|---|---:|---:|---:|
| Complete Apple `ord_live2.ivy`, final verifier | 3,542 | **2,843** | 1,986 |
| Its `sys_live` isolate, final verifier | 2,665 | **1,960** | 1,103 |
| Apple batch 14, ordinary shell with preprocessing enabled at scoped assertions | 1,830 | **907** | 132 |
| Apple condition 132, same shell experiment | 400 | **259** | 1 |
| Apple condition 116, same shell experiment | 43.5 | **17.1** | 1 |
| Issue 73 batch 13, same shell experiment | 656 | 656 | 95 |

The shell results use an experimental call at assertion time. On Apple batch 14, instantiations fell from **4,479,904 to 3,043,156**. The verifier invokes the operation on a final condition after an inconclusive private proof, before its ordinary solver push. All seven shorter verifier comparisons matched their reference outcomes. Apple `rfn2` used 12.61 billion instructions with either build. The original `ord_live.ivy` also showed little change on 11,262 checks in the earlier comparison. [Complete verifier measurement driver](perf/ivy_apple/compare.py) · [Incremental shell inputs and replay driver](perf/ivy_apple/holdouts/order_replay.py)

The rewrite uses the existing equality-elimination routine for its occurs check and capture-avoiding substitution. It can flatten nested conjunctions and consecutive existential binders while preserving binder order. It respects patterns, no-patterns, named inner quantifiers, differing weights, proof generation, tracing and resource limits. Of 36 captured and synthetic formulas, 12 changed. Both solvers proved the disequality between each original and rewritten formula UNSAT. The enabled Ivy traces also matched byte-for-byte on ordinary Raft (62,252,830 bytes) and Apple `rfn2` (2,334,030,415 bytes). Run `python3 perf/ivy_apple/holdouts/verify_existential_definitions.py --g3 /path/to/g3 --z3 ../build/z3` to repeat all 24 independent checks. [Rewriter](qe/existential_definitions.go) · [API and binder tests](api/api_existential_definitions_test.go) · [Equivalence driver](perf/ivy_apple/holdouts/verify_existential_definitions.py)


### Shared parameters in repeated polynomial constraints

A shared parameter can remain unknown throughout preprocessing. With `t*t = 2`, `t > 0`, and 4,096 positive variables constrained by `xᵢ*xᵢ = t`, each solution for one local variable supplies a valid solution for every other one at the same value of `t`. The ordinary g3 solver needed a median **7,458 nonlinear search stages before and two after**. Native Z3 used 4,097. End-to-end elapsed times were **89.131 s, 0.111 s and 0.649 s**, respectively, in three alternating pinned rounds against g3 `10dd215b6`. This benchmark measures nonlinear arithmetic. The Apple verifier measurements above exercise quantified logic. [Exact inputs, outputs and counter samples](perf/nlsat/nov23-shared-parameters.tar.gz)

| Ordinary solver input | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 4,096 positive roots of a shared irrational parameter | 591.892 | **0.643** | 5.890 |
| 1,024 positive roots of the same parameter | 32.834 | **0.199** | 1.481 |
| 512 roots of two different shared expressions, `t+u` and `u-t` | 286.961 | **2.064** | 4.244 |
| 256 roots split by an opposite sign constraint | 12.375 | **0.181** | 0.556 |
| 256 roots split between two parameters | 8.022 | **0.111** | 0.353 |
| 128 roots joined by an order chain | 6.232 | 6.288 | 0.280 |
| 128 different radicands containing a common parameter | 4.844 | 4.816 | 0.205 |

The first traversal counts distinct mixing operations and repeated uses of shared expressions. At most twelve frequent parameters are held fixed during a second traversal. A local variable can share a representative only if its entire set of constraints matches, including the identities and positions of every shared parameter. All original parameter constraints and unrecognized interactions stay in the subgoal. For example, two roots constrained to have opposite signs get distinct representatives. The order-chain and distinct-radicand controls still require a full solve. [Implementation and tests](nlsat/tactic/independent_polynomials.go) · [Benchmark generator](perf/nlsat/independent_components.py)

Native Z3 accepted all **2,844** returned assignments across both solver entry points and agreed on all 12 contradictions in 84 checks. A four-assertion named core independently reproduced UNSAT. Each engine returned SAT, UNSAT, SAT across push/pop and UNSAT, UNKNOWN under the two assumption sets. A separate comparison of 42 repository inputs found identical g3 output. Run `python3 perf/nlsat/independent_components.py 48 --verify /path/to/g3 ../build/z3` for the complete generator check. [Raw model, core, scope and corpus checks](perf/nlsat/nov23-shared-parameters.tar.gz)

### Ordered square roots in nonlinear real arithmetic

If every real variable satisfies `x*x = c` for a nonnegative integer, each variable has at most two possible values. Start with every variable at its smaller root. For a violated `x ≤ y` or `x < y`, every satisfying assignment above the current lower bounds must raise `y`. Each variable can rise at most once. Equality contributes a bound in both directions. g3 checks every original comparison against the final assignment before publishing the exact model. This also handles integer constants, perfect squares and comparisons against the value of a square. The polynomial search remains available for more general terms. [Propagation](nlsat/tactic/square_root_candidate.go) · [Exhaustive sign comparison](nlsat/tactic/square_root_candidate_test.go)

| Complete generated SMT-LIB invocation | g3 before, seconds | g3 after | Native Z3 | Billion instructions before / after / Z3 |
|---|---:|---:|---:|---|
| Contradictory chain of 256 distinct roots | 2.977 | **0.028** | 0.388 | 26.066 / **0.066** / 4.199 |
| 2,048 distinct positive roots | 2.081 | **0.098** | 0.092 | 14.785 / **0.571** / 0.587 |
| 8,192 distinct positive roots | 13.880 | **0.479** | 0.438 | 75.252 / **2.715** / 2.330 |
| 8,192 roots, including output of every value | 12.229 | **0.770** | 0.396 | 86.030 / **5.321** / 2.689 |
| 256 roots with an additional two-variable product | 0.280 | 0.279 | 0.026 | 1.903 / 1.932 / 0.083 |

The table gives the medians of three alternating pinned rounds per row against g3 `aa1300a68` and the in-tree C++ binary. The wall times include startup, parsing and teardown. The check-only and model-output rows ran on different CPUs. An earlier 8,192-variable baseline attempt on CPU 16 reached the 150-second timeout. The two complete comparisons at that size used CPUs 33 and 43. The product control still used 257 nonlinear stages on all three builds. [Inputs, counters and transcripts](perf/nlsat/nov23-signed-square-roots.tar.gz)

Across 102 checks in 17 shapes, native Z3 accepted all 3,576 returned variable assignments when bound back into the original constraints and confirmed all 30 contradictions. The Go unit test also compared propagation with every sign assignment on 1,400 deterministic small systems. A tracked four-assertion core replayed as UNSAT. Both g3 builds produced identical output on 42 repository inputs and matched on push/pop, assumptions, explicit limits and proofs. Run `python3 perf/nlsat/square_root_systems.py 53 --verify /path/to/g3 ../build/z3` from `pkg` to repeat the model check. [Generator and independent check](perf/nlsat/square_root_systems.py) · [Exact numeral construction](ast/ast_arith_algebraic.go)

### Compact exact square roots in models

An exact square root in a model often gets printed or discarded without further arithmetic. The arithmetic plugin can store a signed radicand in four extra bytes at its existing external ID. It materializes the arena-backed algebraic cell when a caller first needs general algebraic arithmetic, decimal approximation or translation. The exact SMT-LIB printer knows the defining polynomial and the root index from the signed radicand alone. For general irrational numerals it borrows the existing cell, as the native printer does, allowing later prints to reuse the computed root index. All storage belongs to the solver’s AST manager. [Plugin storage](ast/ast_arith_algebraic.go) · [Exact printer](ast/ast_smt2_arith_literal.go)

| Ordinary SMT-LIB invocation | g3 before, billion instructions | g3 after | Native Z3 | Median seconds before / after / Z3 |
|---|---:|---:|---:|---|
| Check 8,192 distinct roots | 2.737 | **1.375** | 2.330 | 0.550 / **0.373** / 0.531 |
| Check 16,384 distinct roots | 6.743 | **2.632** | 4.672 | 0.976 / **0.586** / 0.820 |
| Check 8,192 roots and print each once | 5.367 | **2.240** | 2.689 | 0.927 / **0.554** / 0.510 |
| Check 16,384 roots and print each once | 13.435 | **4.348** | 5.385 | 2.094 / **1.130** / 0.999 |
| Print 1,024 distinct roots eight times | 2.394 | **1.074** | 0.617 | 0.698 / **0.356** / 0.133 |
| Print 1,024 rational values five times | 0.721 | 0.722 | 0.595 | 0.256 / 0.335 / 0.123 |

Each row compares three alternating runs of g3 `27998d47d`, the compact representation and native Z3. Wall time varies with host load. For example, the rational control executed essentially the same number of Go instructions in the two builds. Evaluating the original quadratic equalities inside the model still took 5.81 billion instructions against 0.39 billion in Z3. The expensive operation there was general algebraic multiplication. The following experiment reduces that same query to 0.45 billion instructions. Run `python3 perf/nlsat/square_root_systems.py 1024 --values 8` to generate the repeated-print workload. [Inputs, counters and solver output](perf/nlsat/nov23-compact-algebraic-roots.tar.gz)

Z3 accepted all 3,576 rebound values and confirmed the 30 contradictions across the same 102 checks. All 16 decimal-output comparisons matched the preceding Go binary byte for byte at precisions 0, 3, 12 and 40. Native Z3 differed from both Go versions on two precision-zero outputs. Tests cover positive and negative roots, decimal and exact printing, source-slot reuse after translation, and retrying interrupted cell allocation. The 16:05 UTC full Go suite passed. [Ownership and failure tests](ast/ast_arith_square_root_test.go) · [Raw validation results](perf/nlsat/nov23-compact-algebraic-roots.tar.gz)

### Evaluating powers from the defining polynomial

The model evaluator used general algebraic multiplication to rediscover facts already in the model. For an irrational root of `a·Xᵏ + b`, multiplying that exact root by itself `k` times gives the rational `−b/a`. This identity applies to every real root, including negative roots and roots of nonmonic polynomials. g3 checks the degree and every intermediate coefficient before using it. Multiplication additionally checks that every factor has the same AST. The current implementation handles signed-word coefficients and the configured degree bound. Larger coefficients and other polynomials use the general algorithm. [Coefficient check](math/polynomial/algebraic_binomial_power.go) · [Arithmetic rewriter](ast/rewriter/arith_algebraic.go)

| Check and model requests | g3 before, billion instructions | g3 after | Native Z3 | Median seconds before / after / Z3 |
|---|---:|---:|---:|---|
| 256 quadratic equalities, eight requests | 5.577 | **0.452** | 0.393 | 2.054 / **0.289** / 0.171 |
| 256 cubic products, four requests | 19.472 | **3.558** | 1.087 | 2.719 / **0.575** / 0.150 |
| 256 quartic products, four requests | 38.260 | **4.043** | 2.308 | 5.210 / **0.623** / 0.285 |
| 256 explicit quadratic powers, four requests | 1.930 | **0.284** | 0.197 | 0.466 / **0.105** / 0.061 |
| 128 negative cubic powers with rational radicands, four requests | 3.321 | **1.851** | 0.230 | 0.535 / **0.331** / 0.051 |
| 128 products of two different roots, two requests | 0.816 | 0.809 | 0.101 | 0.201 / 0.206 / 0.044 |
| 128 roots of quadratics with a linear term, two requests | 2.115 | 2.115 | 0.123 | 0.466 / 0.483 / 0.047 |

The table uses three alternating runs against g3 `43da9f8dc` and the in-tree Z3 binary. Every timed invocation finished successfully with matching output. The explicit-power generator uses the `ALL` signature and the nonlinear real tactic. This fork's `QF_NRA` signature rejects the explicit power operator. A separate 90-case run bound all 3,660 returned values, including the original roots and the evaluated terms, back into the original problem and checked them with native Z3. Run `python3 perf/nlsat/algebraic_powers.py 17 --degree 3 --denominator 5 --operation inverse --verify /path/to/g3 ../build/z3` to reproduce one check. [Generator and verifier](perf/nlsat/algebraic_powers.py) · [Inputs, hardware counters and outputs](perf/nlsat/nov23-binomial-powers.tar.gz)

### Nonlinear propagation on a clause containing complementary literals

A three-variable check returned `unknown` with the reason `nlsat: pending search-state operation`. During one arithmetic clause visit, the solver propagated the first literal false. The next literal was its complement and was already true. Trying to assign it again violated the search trail invariant. The solver now finishes the satisfied clause immediately. The five-assertion input checks two square roots of the same algebraic parameter and compares their order in both directions. The CLI regression checks two model values, introduces a contradictory scope, then pops and checks again with the explicit nonlinear tactic. [Regression input](cmd/g3/test_vectors/nlsat_complement_propagation.smt2) · [Search trail test](nlsat/search_state_test.go)

Across five sizes, four formula variants and two solver entry points, the original build answered 20 of 40 correctly. The corrected build answered all 40. Native Z3 accepted all **1,248** returned assignments when bound simultaneously into the original assertions and confirmed the ten contradictions. This restores solver completeness on these inputs. Timing the old `unknown` against the new `sat` would compare different outcomes. The archive includes three alternating instruction samples on both the failure and same-answer controls at two sizes. [Models, controls and raw timings](perf/nlsat/nov23-complement-propagation.tar.gz)

### Reusing polynomial solutions across non-strict comparisons

For 2,048 positive variables constrained by `xᵢ² = t` and `t² = 2`, a chain of alternating `≤` and `≥` comparisons used to defeat independent-component reduction. After giving each local variable its representative’s value, both operands of every comparison are identical. The nonlinear solver now checks the shared parameter and one local variable, then reconstructs the rest. Three alternating pinned runs of the complete check took a median **0.090 s** against **68.531 s** for g3 before the change and **0.486 s** for native Z3. The g3 nonlinear search went from 4,588 stages to two. [Implementation](nlsat/tactic/reflexive_comparisons.go) · [Raw measurements and inputs](perf/nlsat/nov23-reflexive-comparisons.tar.gz)

| Ordinary solver input | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 2,048 roots, alternating order | 493.485 | **0.468** | 4.318 |
| 1,024 roots, alternating order | 88.253 | **0.255** | 2.164 |
| 512 comparisons of compound expressions | 59.403 | **0.182** | 1.434 |
| 128 roots and a strict comparison requiring opposite signs | 6.065 | 6.090 | 0.270 |
| 128 roots and an additional unconstrained variable | 6.263 | 6.314 | 0.283 |

The reduction compares exact expression shapes. It removes a top-level equality or non-strict inequality only when the two operands have identical syntax after replacing their variables with the same representative. A second scan checks all remaining occurrences, including those inside Boolean formulas. A strict inequality can require different roots and remains a general solver problem. The nonlinear benchmarks exercise this arithmetic path independently of the quantified Apple workload.

Across 60 checks at three sizes through the default solver and the explicit nonlinear portfolio, native Z3 accepted all **2,364 returned assignments** and confirmed all 18 contradictions. Native Z3 also confirmed the two- and six-assertion cores on tracked inputs. Both Go builds matched byte for byte on 42 repository inputs. Push/pop produced SAT, UNSAT, SAT. All three builds returned UNKNOWN under the explicit resource limit. Generate the large timing input with `python3 perf/nlsat/independent_components.py 2048 --shape comparison-chain`. [Benchmark generator](perf/nlsat/independent_components.py) · [Checks and transcripts](perf/nlsat/nov23-reflexive-comparisons.tar.gz)

### Keeping the selected real root in nonlinear conflicts

Both g3 at `150319763` and the in-tree release Z3 returned UNSAT for `x + y > 0, x² = 2, y² = 2`. The assignment `x = y = √2` satisfies all three constraints. A search trace showed the error arose after choosing `x = −√2`. At that point the two constraints on `y` really do conflict. Their resultant vanishes at **both** roots of `x² − 2`. The solver learned that equality alone explained the conflict and discarded the positive root as well. The same three assertions, in the same order, reproduce the error with the ordinary solver and the nonlinear tactic. [Regression input](cmd/g3/test_vectors/nlsat_disconnected_resultant_roots.smt2) · [Before and after search traces](perf/nlsat/nov23-resultant-cells.tar.gz)

The projection step now retains each nonconstant, nonzero principal subresultant coefficient through the first one that is nonvanishing at the current assignment. When a coefficient vanishes there, its factors participate in constructing the real cell containing the assignment. On the reduced example that adds the missing condition `x < 0` to the explanation. Search can then choose the positive root. The regression also checks both model signs, adds a contradictory negative sign under push, and solves the original goal again after pop. A private C++ build of the same in-tree sources with the analogous change produced the same correction. [Go implementation](nlsat/explain_projection_psc.go) · [Native control patch and raw results](perf/nlsat/nov23-resultant-cells.tar.gz)

For 2,048 deterministic systems of two or three variables, all possible roots are integer multiples of one known positive square root. Enumerating those finite domains with integer arithmetic establishes the answers independently of either solver. Of the 1,234 SAT inputs, the two original engines each called 28 UNSAT. Both corrected engines agreed with all **2,048** expected outcomes. The private native control also accepted all 2,993 returned g3 assignments when rebound to the original assertions. On a separate 500-case sign-system campaign, g3 agreed with all 150 SAT and 350 UNSAT outcomes, including 2,953 rebound variable assignments. The 42 existing shell vectors retained their g3 transcripts. Four same-answer timing controls used three alternating runs each. For example, 256 distinct quadratic pairs took 4.604 versus 4.607 billion g3 instructions before and after. Timing a fast but incorrect UNSAT against a correct SAT would obscure the change in verdict. [Finite-domain generator](perf/nlsat/resultant_cells.py) · [Per-case results](perf/nlsat/nov23-resultant-cells.tar.gz)

### Reusing whole solutions to independent nonlinear systems

A component with several unknowns can reuse another component’s solution when a one-to-one variable renaming preserves all constraints. For example, in `xᵢ² = t, xᵢ > 0, xᵢ·yᵢ = 1, yᵢ > 0`, copying only `xᵢ` would leave its partner unsolved. Copying the pair preserves the product as well. The shared parameter `t` retains its source identity across every component. With 1,024 such pairs and a shared positive `t` satisfying `t² = 2`, the complete solver call fell from **11.006 s to 0.078 s**. The in-tree native Z3 took **0.270 s**. Both timed g3 builds include the correction for disconnected resultant roots. [Implementation](nlsat/tactic/coupled_polynomials.go) · [All three alternating pinned samples per engine](perf/nlsat/nov23-coupled-polynomials.tar.gz)

| Ordinary solver input | g3 before, billion instructions | g3 after | Native Z3 |
|---|---:|---:|---:|
| 1,024 pairs with a shared irrational parameter | 90.317 | **0.393** | 2.296 |
| 256 pairs with a shared irrational parameter | 12.929 | **0.162** | 0.583 |
| 2,048 pairs with a constant right-hand side | 33.643 | **0.658** | 1.601 |
| 1,024 pairs with a constant right-hand side | 16.980 | **0.346** | 0.806 |
| 96 pairs with unique radicands | 1.726 | 1.754 | **0.090** |

The scan joins variables that occur together, including occurrences inside nested Boolean expressions and shared arithmetic subterms. Each component with two through eight local variables offers one candidate bijection in source encounter order. Exact declaration identities, ordered child slots and the complete set of assertion shapes establish the match. Different encounter orders can miss an opportunity. Connected or unmatched systems retain their original assertions. For the 96 unique pairs in the table the search still takes 193 stages. Those numbers leave a separate opportunity in the general nonlinear solver.

The default solver and the explicit nonlinear portfolio agreed with native Z3 on 96 runs spanning 16 shapes. The native solver accepted **5,312** rebound variable assignments and confirmed the 24 contradictions. An independent finite-domain enumeration agreed on 500 additional sign systems, including 2,953 rebound assignments. The named two- and six-assertion cores independently reproduced UNSAT. Push/pop gave SAT, UNSAT, SAT, and the explicit resource limit gave UNKNOWN on all builds. Both Go versions produced identical output on 42 repository inputs. Reproduce the parameter benchmark with `python3 perf/nlsat/coupled_systems.py 1024 --shape parameter`. [Benchmark generator](perf/nlsat/coupled_systems.py) · [Models, cores and measurements](perf/nlsat/nov23-coupled-polynomials.tar.gz)

### Quantified enumeration proofs on complete Ivy specifications

Apple verification conditions include small, named enumeration types alongside functions on uninterpreted sorts. A private refutation solver can represent each enumeration by a fresh sort, one distinct constant per constructor, and an axiom restricting every element to one of those constants. Every assignment in the original problem has a corresponding assignment in this language. An UNSAT answer therefore proves the original obligation. Each source solver owns its private search, retains only matching premises, and scopes each sibling goal with its additional axioms. An inconclusive attempt returns control to the ordinary solver, which owns the source result and any counterexample. This path requires an opt-in verification batch, three or more enumeration sorts, and twenty quantified declarations in the base. [Projection and scoping](api/internal/order/enumeration_premises.go) · [Private search](api/api_enumeration_refutation.go)

| Complete GoIvy input | Checks | Previous g3, billion instructions | g3 with finite-domain search | Linked Z3, earlier frontend |
|---|---:|---:|---:|---:|
| Apple `ord_live2.ivy`, all isolates | 1,986 | 2,841.09 | **2,266.89** | **1,964.62** |
| Apple `ord_live2.ivy`, isolate `sys_live` | 1,103 | 1,957.57 | **1,526.62** | 1,314 in the earlier native run |
| `jea/issue73/ord.ivy`, all isolates | 795 | 814.60 | **390.20** | 497.20 |
| Apple `ord_live.ivy`, isolate `cf_live` | 336 | 53.98 | **45.16** | 58.06 |
| Apple `ord_live.ivy`, isolate `dramc_nb2` | 2,479 | 139.31 | **134.92** | 157.13 |
| Apple `ord_live.ivy`, isolate `rfn2` | 26 | 12.71 | 12.75 | 51.47 |
| Raft `raft_no_assume.ivy` | 419 | 19.56 | 19.56 | 24.78 |

The g3 release used GoIvy at `104078aaa`, including its equality-defined witness simplification. The native executable predates that frontend revision. The first two rows are single complete runs. The remaining rows are medians of three alternating pinned runs per engine. Counts include the verifier process and its children. All completed runs reproduced their reference output, errors, and exit status. The ten holdouts included Flash, TLB and expected counterexamples. Wall-clock samples on this shared host varied with contention. The issue73 proof improvement also appears in the three-sibling Go benchmark on captured source ASTs. Two iterations took 25.04 and 31.09 seconds with the ordinary solver and 3.72 and 3.65 seconds with the batch. Run `go test ./api -run '^$' -bench '^BenchmarkFiniteEnumerationVerification$' -benchtime=1x -cpu=1`. [Benchmark and input](api/api_enumeration_refutation_benchmark_test.go) · [Raw counters and transcripts](perf/ivy_apple/holdouts/nov23-finite-enumeration.tar.gz)

The private solver proved 827 goals on the full Apple run. A separate instrumented run spent 846.97 billion solver-thread instructions on successful private checks and **373.54 billion on 17 unsuccessful attempts**. Translation and setup for the successful checks together took 0.47 billion. Each failed private search was followed by an ordinary check, so the avoidable cost depends on the work that the ordinary solver still has to perform. On `sys_live` batch 7 goal 132, the private search used 22.49 billion instructions before the source check used another 42.77 billion. A standalone incremental replay of the projected input reached exactly 762,595 quantifier instantiations, 206,697 decisions and 879 conflicts in both engines. g3 retired 166.42 billion instructions to Z3’s 52.01 billion on that replay. Instance materialization and term rewriting are candidates for reducing work per instance. The whole-run measurements also justify looking for an early test that avoids unproductive private searches. [Per-goal measurements and sampled stacks](perf/ivy_apple/holdouts/nov23-finite-enumeration.tar.gz)

An independent truth-table oracle decided 192 quantified formulas with 96 SAT and 96 UNSAT outcomes. Both shells agreed on the original and projected formulas. The in-tree native Z3 also independently returned UNSAT for every distinct proof input captured from issue73 (266), Apple `sys_live` (513), `dramc_nb2` (69) and `cf_live` (19). The archive preserves all 867 native replay inputs. With tracing enabled, ordinary Raft and Apple `rfn2` produced byte-identical outputs, including 62,252,830 and 2,334,030,415 trace bytes respectively. Focused release and tagged tests cover counterexamples, cancellation, binder scopes, goal-local declarations and solver reuse. [Truth-table oracle](api/api_enumeration_truth_table_test.go) · [Replay inputs, enabled-trace comparisons and tests](perf/ivy_apple/holdouts/nov23-finite-enumeration.tar.gz)

### Exact propagation for real product equations

On 96 generated triangles of real product equations, g3’s median time fell from **37.31 seconds to 0.042 seconds**. The in-tree native Z3 took **0.903 seconds**. The old Go solver made 27,259–29,297 nonlinear stages. For a triangle such as `x*y=2, y*z=3, z*x=5`, multiplication and division give `x²=10/3`. Each additional nonzero product fixes a neighboring value once an initial variable is chosen. An even cycle checks rational consistency and an odd cycle determines the square of that initial variable. These identities avoid a search over thousands of algebraic cells. The benchmark runs separate processes and includes parsing, model construction when requested, and teardown. [Product equation propagation](nlsat/tactic/product_equations.go) · [Generator](perf/nlsat/product_equations.py)

| Complete shell input | Previous g3, billion instructions | g3 after | Native Z3 | Median seconds before / after / Z3 |
|---|---:|---:|---:|---|
| 96 distinct product triangles | 274.911 | **0.060** | 6.804 | 37.312 / **0.042** / 0.903 |
| Same triangles, print all 288 values | 276.318 | **0.092** | 6.817 | 35.281 / **0.037** / 0.792 |
| 1,024 pairs with different square anchors | 24.074 | **0.353** | 0.886 | 12.634 / **0.292** / 0.447 |
| Same pairs, print all 2,048 values | 26.793 | **0.578** | 0.978 | 12.203 / **0.410** / 0.451 |
| 384 anchored chains of four variables | 13.423 | **0.223** | 0.672 | 2.430 / **0.081** / 0.120 |
| 96 even cycles | 1.093 | 0.065 | 0.065 | 0.228 / 0.043 / 0.038 |
| 96 rational roots with printed values | 1.097 | 0.102 | **0.077** | 0.616 / 0.113 / **0.089** |
| 1,024 pairs sharing an irrational parameter | 0.392 | 0.390 | 2.296 | 0.107 / 0.110 / 0.363 |

Each row uses three alternating pinned rounds against g3 `40be21b73` and the same native binary. Shared-host contention affects elapsed time. On the check-only triangle, the integrated change used approximately 4,570 times fewer instructions than the preceding g3 and 113 times fewer than Z3. This result applies to the nonlinear equations in the generator. The Apple verification results above measure a separate quantified workload. Generate the triangle input with `python3 perf/nlsat/product_equations.py 96 --shape triangle`. [Inputs and all engine samples](perf/nlsat/nov23-product-equations.tar.gz)

The solver validates the proposed signs and magnitudes against every scalar comparison before returning SAT. Nonzero products make any contradictory cycle independently sufficient for UNSAT. Free-scale components can have many solutions. When a suggested scale misses a bound, or a component requires larger coefficients, the original solver handles it. A square equated to a symbolic parameter can need fourth-degree values and bypasses this attempt. Compact exact model values now support a rational radicand, with denominators allocated only when needed. Translation and later arithmetic materialize the defining polynomial through the existing arena. [Exact model numerals](ast/ast_arith_algebraic.go) · [Root isolation](math/polynomial/algebraic_square_root_interval.go)

Across 120 end-to-end checks at three sizes, native Z3 accepted all **6,640** rebound values from 96 SAT results and independently confirmed the 24 UNSAT results. The tests checked another 2,000 finite sign systems exhaustively and 1,200 connected systems with independently prescribed fractional magnitudes. Bounds near the largest 32-bit numerator and denominator, translation followed by source-slot reuse, and exact printing passed. Both Go binaries produced identical output on 42 repository vectors. All three engines agreed on SAT/UNSAT/SAT across push/pop, the named contradictory core, UNKNOWN under the explicit resource limit, and UNSAT/UNKNOWN on the two assumption checks. The SAT witness for that last assumption query remains unavailable through the incremental path in all three engines. [Finite assignment oracles](nlsat/tactic/product_equations_test.go) · [Numeral tests](ast/ast_arith_square_root_fraction_test.go) · [Models, cores and transcripts](perf/nlsat/nov23-product-equations.tar.gz)

### Eliminating functionally determined quantifier arguments

On Apple `ord_live2.ivy`, the premises imply that whenever a four-argument relation holds, three of its arguments are determined by unary functions of the fourth. In captured `sys_live` batch 8 goal 105, eliminating determined binders reduced the number of quantifier instances from **487,095 to 6,172**. Running the original and rewritten questions through the same unmodified g3 shell reduced instructions from **96.71 billion to 2.91 billion**. The in-tree Z3 shell went from **30.26 billion to 0.78 billion** on those same questions. Both returned UNSAT with exactly the same instance counts as each other. The shorter search accounts for the large gain in both engines. g3 still spent more instructions per instance. [Replay inputs, counts and controls](perf/ivy_apple/holdouts/nov23-predicate-dependencies.tar.gz)

The finite enumeration proof now discovers these dependencies in the current solver's universal premises. A direct implication can establish an equality. Two definitions guarded by complementary conditions can also establish it when both entail the same equality. A counterexample to a universal or a witness for an existential must then assign the determined variable its defining term. The projection substitutes that term and keeps the implied relation constraint explicit. Exhaustive checks across 491,520 combinations of finite interpretations, formulas and transformations found 13,958 extra satisfying combinations for controls that omitted the support constraint. Both shells independently matched the expected SAT or UNSAT answer on all 192 generated source and projected inputs. [Implementation](api/internal/order/argument_dependencies.go) · [Regression tests](api/api_enumeration_dependencies_test.go) · [Finite interpretations and answers](perf/ivy_apple/holdouts/nov23-predicate-dependencies.tar.gz)

The retained Go benchmark runs consecutive `sys_live` goals 104–106 against their shared original premise. On the first pinned pair, the batch went from **71.96 s to 5.66 s** and proved all three goals privately, up from two. Process instructions fell from **114.38 to 14.20 billion**, including fixture parsing. On the separate three-goal issue73 benchmark, instructions fell from **17.21 to 5.87 billion** with private search parameters held constant. On the complete 795-check issue73 verifier, the dependency-only implementation used **208.71 billion** instructions in one run, compared with approximately **391 billion** for the preceding g3 and **497 billion** for linked Z3. Both Go benchmark inputs use the public proof-batch entry points. Run `go test ./api -run '^$' -bench '^BenchmarkEnumerationPredicateDependencies$' -benchtime=1x -cpu=1`. [Go benchmarks](api/api_enumeration_refutation_benchmark_test.go) · [Apple input](api/test_vectors/apple_sys_live_goals.smt2.gz) · [Raw controls](perf/ivy_apple/holdouts/nov23-predicate-dependencies.tar.gz)

### Certifying quantified formulas in the private finite-domain solver

For a universal formula over a candidate model, g3 can check its finite function tables directly. When every tuple satisfies the formula, this eliminates an auxiliary SMT search. When a tuple violates the formula, ordinary model-based quantifier instantiation still chooses the witness. That choice affected the cost of later checks. On the complete 1,986-check Apple `ord_live2.ivy` input, an exploratory run used **1.530 trillion instructions** with ordinary model checking, **1.316 trillion** when using tables for certificates and witnesses, and **1.214 trillion** when using tables only for certificates. All three used the argument reduction, private relevancy level 1 and six million private resource steps. All produced identical output. A second certificate-only trial at eight million steps used 1.228 trillion. [Private model checks](smtcore/smt_model_checker.go) · [Compared search policies](perf/ivy_apple/holdouts/nov23-bounded-model-certificates.tar.gz)

The cleaned release build reproduced the full Apple result at **1.213 trillion instructions** and **234.76 s** in its first run with the hardware-thread sibling 99.2% idle. The preceding g3 used approximately **2.27 trillion** and linked Z3 with the earlier frontend **1.96 trillion** in the earlier complete runs. On the 1,103-check `sys_live` isolate the new build used **1.148 trillion** against **1.532 trillion** in the preceding g3 on the same CPU. For `issue73/ord.ivy`, three rotating runs gave median instructions of **160.29 billion** with the new g3, **388.87 billion** with the preceding g3, and **497.22 billion** with linked Z3. The first comparison with an idle sibling took **24.55 s, 58.60 s and 144.69 s**, respectively, for all 795 checks. Longer comparisons use the physical-core measurement driver below. [Counter records and sibling utilization](perf/ivy_apple/holdouts/nov23-bounded-model-certificates.tar.gz) · [Rotating comparison driver](perf/ivy_apple/holdouts/rotating_verification.py)

The saved three-goal issue73 Go benchmark took median times of **2.575 s** before the argument reduction, **0.874 s** after the reduction alone, and **0.328 s** with the private search changes. The respective whole-process counts were 17.21, 5.87 and 2.37 billion. The three Apple goals took **71.96 s, 5.66 s and 6.63 s**. Their instruction counts were **114.36, 14.22 and 12.58 billion**. Shared-core timing for the last two Apple variants varied. The full verifier and the isolated API benchmark exercise different mixes of setup and search. [Benchmark source](api/api_enumeration_refutation_benchmark_test.go) · [All nine samples per fixture](perf/ivy_apple/holdouts/nov23-bounded-model-certificates.tar.gz)

The same g3, preceding g3 and linked Z3 reproduced every expected outcome in 90 complete runs over ten held-out cases. A further 228-spec corpus matched byte for byte across the three implementations, including errors and exit status. The instrumented dependency prototype detected no applicable predicate dependency on those 228 inputs. They measure compatibility. The independent native shell confirmed all 320 distinct successful production proof inputs replayed from issue73 (232), `cf_live` (19) and `dramc_nb2` (69). Release and trace-enabled debug tests cover the resource boundary before compilation and during tuple enumeration, cancellation, releasing model references and reusing the scratch storage. [Budget and model tests](smtcore/smt_model_table_test.go) · [Production proof inputs and outcomes](perf/ivy_apple/holdouts/nov23-bounded-model-certificates.tar.gz)

For the canonical `examples/raft/raft_no_assume_test.ivy` benchmark, three alternating pinned runs of the preceding release completed all **419** obligations in a median **5.079 s and 23.25 billion instructions**. Linked Z3 took **63.929 s and 214.72 billion** on the same CPU with the earlier frontend. The similarly named `raft_no_assume.ivy` runs recorded elsewhere use a different input. [Canonical Raft measurements](perf/ivy_apple/holdouts/nov23-bounded-model-certificates.tar.gz)

### Quantifier width and the first push in Flash

On `test/flash_cubicle.ivy`, g3 used **9.413 billion instructions before and 1.909 billion after** sending narrow quantified problems to the ordinary incremental solver. Linked Z3 used **1.811 billion**. In the earlier phase profile, g3 spent 8.67 billion instructions trying proofs before the first push. It discharged 1,036 of 1,037 such attempts, so merely counting successful proofs made the expensive strategy look attractive. On these premises the private solver repeatedly solved easy goals that the ordinary solver could handle after one shared push. The nine rotating instruction-counter rounds reproduced all 1,073 outcomes. [Phase counts and complete runs](perf/ivy_apple/holdouts/nov23-narrow-enumeration.tar.gz)

| Flash Cubicle measurement | Previous g3 | Current g3 | Linked Z3, earlier frontend |
|---|---:|---:|---:|
| Whole-verifier instructions, median of nine rounds | 9.413 billion | 1.909 billion | 1.811 billion |
| Whole-verifier elapsed time, median of seven separate rounds | 1.651 s | 0.343 s | 0.295 s |
| Three captured sibling goals through the Go proof batch, median of seven rounds | 18.20 ms | 5.76 ms | — |
| Same three goals through ordinary Go incremental checks | 15.92 ms | 15.90 ms | — |

The timing runs pinned each checker to CPU 20 and measured its hardware-thread sibling. A qualifying sample required both at least 95% sibling idle time and at least 95% CPU occupancy by the measured process. The driver used `wait4` for direct wall and CPU timing. On this subsecond case, wrapping the process in perf and reporting CPU time to hundredths of a second failed the occupancy test even on an idle sibling. The archive includes rejected attempts. The Go benchmark performs five iterations per sample and checks three unsatisfiable goals per iteration. The previous batch proved all three privately. The new batch sends them to the ordinary solver. [Rotating comparison driver](perf/ivy_apple/holdouts/rotating_verification.py) · [Saved Go benchmark](api/api_enumeration_refutation_benchmark_test.go) · [Captured inputs and measurements](perf/ivy_apple/holdouts/nov23-narrow-enumeration.tar.gz)

The Flash premises quantify over one or two cache identifiers at a time. Enumerations appear as function results. g3 keeps the finite-domain proof eligible if either the base or the current goal binds an enumeration variable or has at least three variables simultaneously in scope. The decision is local to the current query. A later sibling can still qualify on the same base. Tests exercise changes in both directions, nested binders and satisfiable counterexamples. A separate corpus of 228 specifications matched the previous g3 and linked Z3 on output, errors and exit status. Three rotating rounds on Apple `cf_live`, `rfn2`, Hermes and `test/flash2.ivy` also matched. The latter Flash input still used 14.25 billion instructions to linked Z3’s 6.18 billion. Profiling that input found 175 unsuccessful ground-witness attempts consuming 6.52 billion solver-thread instructions. [Selection and regression cases](api/api_enumeration_refutation_test.go) · [Verification controls](perf/ivy_apple/holdouts/nov23-narrow-enumeration.tar.gz)

### When a separate ground-relation proof saves work

On `test/flash2.ivy`, g3 tried to refute 181 ground relation goals by instantiating a connected subset of their premises in a private solver. Six attempts succeeded. The 175 misses created 8,698 ground instances and retired **6.52 billion solver-thread instructions** before the caller resumed ordinary search. The same method succeeded on all 14 of its attempts in Apple `cf_live`. On that isolate it kept 59–99 non-ground premises per attempt out of 169–504 available. That difference gives g3 a decision it can make before paying for translation or instantiation. [Per-plan profiles](perf/ivy_apple/holdouts/nov23-ground-relation-selection.tar.gz)

For a ground relation goal, g3 now starts a private witness search when the selected subset omits at least half of the non-ground premises. All ground facts already belong to the subset. The existing incremental solver checks the query when the subset would duplicate most of its quantified work. In two initial Flash runs, requiring three same-sort binders still cost about 14.2 billion instructions. Backing off after misses cost 10.1 billion. Comparing selected with omitted non-ground premises brought that to 7.54 billion. The same rule admits the useful Apple subset regardless of how many ground facts accompany it. [Selection code](api/api_witness_premises.go) · [Threshold and counterexample tests](api/api_goal_relation_test.go) · [Alternative rules](perf/ivy_apple/holdouts/nov23-ground-relation-selection.tar.gz)

| Flash 2 measurement | Previous g3 | Current g3 | Linked Z3, earlier frontend |
|---|---:|---:|---:|
| Whole-verifier instructions, median of nine rounds | 14.290 billion | **7.575 billion** | 6.182 billion |
| Whole-verifier elapsed time, median of seven rounds | 2.198 s | **1.204 s** | 0.979 s |
| Three captured successive goals through the Go proof batch, median of seven rounds | 27.45 ms | **8.21 ms** | — |
| Same three goals through ordinary Go incremental checks | 22.07 ms | 22.25 ms | — |

All 1,073 Flash outcomes matched. The direct timing runs used CPU 20, requiring 95% process occupancy and 95% idle time on its hardware-thread sibling. With both admission changes installed, the other Flash specification, `test/flash_cubicle.ivy`, remained at 1.93 billion instructions. The 228-spec corpus reproduced the same output, errors and exit codes across all three engines. Three rotating rounds on `cf_live`, `cf_pio_live`, `cmpl_live`, `rfn2`, Hermes and canonical Raft also matched. Run `go test ./api -run '^$' -bench '^BenchmarkGroundRelationVerification$' -benchtime=5x -cpu=1` to reproduce the API benchmark. [Go benchmark](api/api_witness_instances_benchmark_test.go) · [Complete results and captured inputs](perf/ivy_apple/holdouts/nov23-ground-relation-selection.tar.gz)

### Independent replay of the full Apple proof and timing on issue73

The in-tree C++ Z3 shell returned UNSAT for **all 710 distinct private proof inputs** from the complete `doc/examples/apple/ord_live2.ivy` verification at g3 `d0dc8d971`. Production g3 discharged 711 goals on this path. One pair submitted byte-identical assertions. Each replay ran independently, exited successfully and wrote no diagnostics. This checks the formulas actually sent to the private solver. The source-to-projection comparison also has the finite truth-table tests described above. The archive includes the source-verification transcript, the chronological mapping and all 710 SMT inputs. [Native replay results and input files](perf/ivy_apple/holdouts/nov23-apple-certificate-confirmation.tar.gz)

On all **795** checks of `jea/issue73/ord.ivy`, g3 finished in a median **23.878 s**, compared with **57.313 s** before the functional argument reduction and bounded model certificates, and **136.665 s** for linked Z3. The corresponding median process instruction counts were 161.30, 390.98 and 497.19 billion. Three rotations used CPU 15 and one Go runtime CPU. The two g3 builds shared the current GoIvy frontend. The linked-Z3 build used the earlier frontend. The hardware-thread sibling was at least 99.27% idle throughout every run and each checker occupied at least 99.84% of its CPU. All nine transcripts matched. [Rotations and counters](perf/ivy_apple/holdouts/nov23-apple-certificate-confirmation.tar.gz) · [Comparison driver](perf/ivy_apple/holdouts/rotating_verification.py)

### Flash and Hermes with both backends built from GoIvy 104078aaa

Two independent builds reproduced the compiler build IDs for both g3 and linked Z3. Each executable embeds GoIvy commit `104078aaa` and uses the local g3 module replacement. The g3 implementation is `291ac3bb0`, with documentation changes through `4cad6425b`. This replaces the earlier Flash backend comparisons. Seven timing rounds on CPU 20 required at least 95% process occupancy and 95% idle time on its hardware-thread sibling. Nine separate rounds measured process instructions. Every one of the 1,073 outcomes on each Flash specification matched. The rebuilt native verifier also reproduced stdout, stderr and exit status on all 228 additional specifications. [Reproducible build](perf/ivy_apple/holdouts/build_verification_pair.py) · [Build metadata, timing rounds and corpus](perf/ivy_apple/holdouts/nov23-matched-frontend-flash.tar.gz)

| Complete verification | Previous g3 | Current g3 | Linked Z3 | Billion instructions, previous / current / Z3 |
|---|---:|---:|---:|---|
| `test/flash_cubicle.ivy` | 1.371 s | **0.297 s** | 0.239 s | 9.272 / **1.946** / 1.737 |
| `test/flash2.ivy` | 1.947 s | **1.077 s** | 0.880 s | 14.311 / **7.574** / 6.096 |

On the full `jea/hermes_rmw_o3_testing.ivy`, the first matched pair used **1.440 trillion instructions for released g3 and 1.259 trillion for linked Z3**. Each produced the complete 6,338-check transcript. The native figure of 1.429 trillion from the older frontend overstated the cost of the current native verifier. [Matched Hermes pair](perf/ivy_apple/holdouts/nov23-matched-frontend-flash.tar.gz)

### Hermes refutations from ground equalities

The full Hermes run contains 909 single-goal batches whose final condition simplifies to true. Proving UNSAT in these batches requires finding a contradiction among the premises. Starting a second SMT search proved 624 of them, and no batch had a second goal that could reuse such a proof. In a separate native process for each batch, the ground conjuncts alone were UNSAT in **360** cases and SAT in **549**. Native Z3 also replayed an UNSAT core for each of the 360. Many cores have three or four facts. For example, `a = b`, `f(a) = v` and `f(b) ≠ v` are contradictory because a function takes equal arguments to equal results. [All ground inputs, core replays and trial results](perf/ivy_apple/holdouts/nov23-hermes-ground-search.tar.gz)

A prototype that propagated required equalities between predicate arguments found 264 of the 360 refutations. Recognizing equal branches of conditionals raised this to 292. Extending equality through nested applications and sharing the resulting Boolean atoms found all 360, and declined all 549 satisfiable ground subsets. When the speculative mixed search had an estimated budget of at least 40,000 steps, the prototype sent inconclusive cases to the ordinary incremental solver. One full run used **1.310 trillion instructions**, about 9% below the released g3 and 4% above the rebuilt native control. All 6,338 checks matched. These runs establish instruction savings. The prototype still needs the full source-input validation and resource accounting of the production proof path. [Complete-verifier counters and experimental code](perf/ivy_apple/holdouts/nov23-hermes-ground-search.tar.gz)

The retained API benchmark uses four actual premise-and-goal pairs, parsed once and solved on a fresh incremental solver per iteration. The table gives median time across five rotating runs, four iterations per sample on a shared host. In the fourth case the former private search exhausted its allowance and the ordinary solver then repeated the work. The congruence prototype proved that case about **16× faster**. Run `go test ./api -run '^$' -bench '^BenchmarkHermesPremiseVerification$' -benchtime=4x -cpu=1` to replay the shipping implementation. [Go benchmark and fixture names](api/api_short_premises_test.go) · [All benchmark samples](perf/ivy_apple/holdouts/nov23-hermes-ground-search.tar.gz)

| Original private search | Released g3 | Ground SMT prototype | Congruence prototype |
|---|---:|---:|---:|
| Ground conflict, search 1223 | 38.25 ms | 22.05 ms | **6.48 ms** |
| Nested ground applications, search 1228 | 39.95 ms | 22.02 ms | **6.53 ms** |
| Inconclusive private search, 1226 | 124.54 ms | 85.38 ms | **69.15 ms** |
| Ground conflict after the former limit, search 1474 | 156.13 ms | 34.64 ms | **9.70 ms** |

The ordinary SMT solver also has a separate performance gap. We split 36 queries between ground SAT, shallow ground contradictions and harder ground contradictions, taking 12 from each group. We ran each selection with and without its short universals and switched the hardware counter on around either assertion loading or `check-sat`. Across these 36 inputs, loading the ground facts cost g3 1.76× the native instructions and the check itself cost **3.83×**. With the short universals present the corresponding ratios were 1.60× and **4.12×**. On an 18-input profile, the two solvers reported the same totals for decisions, conflicts, instantiations and resource steps in both configurations. On ground checks, 31% of g3’s instruction samples included theory-rewriter construction and 25% included simplifier cleanup. The stacks overlap because cleanup constructs a fresh rewriter. This identifies a general cost to test alongside the Hermes proof change. [Phase measurement driver](perf/ivy_apple/holdouts/shell_check_phases.py) · [Shell inputs, sampled stacks and search statistics](perf/ivy_apple/holdouts/nov23-hermes-ground-search.tar.gz)

### Ground equality propagation in the Hermes verifier

The integrated ground proof cut a complete 6,338-check Hermes verification from **200.80 s to 182.40 s** on its first quiet, pinned pair. Instructions fell from **1.440 trillion to 1.311 trillion**. The matched linked Z3 control used a median **1.259 trillion** over three complete runs. All three builds use GoIvy `104078aaa`. Every outcome matched, including stdout, stderr and exit status across another 228 specifications. [Counter samples and controls](perf/ivy_apple/holdouts/nov23-ground-congruence.tar.gz)

The proof records the equalities that the asserted premises force, including those under negated implications. Two passes propagate equality through applications of the same declaration. The resulting Boolean clauses refuted all 360 ground selections that independent Z3 processes found contradictory and declined all 549 with ground models. Unit propagation alone accounted for 352 of the 360. Each successful proof validates the original assertion scopes, including omitted premises, under its private resource limit. The dense term IDs and equivalence classes live for one query. [Implementation](api/internal/order/ground_equalities.go) · [Polarity, model, source and limit tests](api/api_ground_equalities_test.go) · [Retained benchmark](api/api_short_premises_test.go)

### Ground instances of universal premises in Hermes

On two complete 6,338-check runs, the integrated g3 verifier used **1.254 and 1.255 trillion instructions**. The preceding ground-only implementation used **1.311 trillion** and the matched linked Z3 median was **1.259 trillion**. This brings the complete g3 process within half a percent of Z3 by instruction count. The first elapsed-time run had 91.2% idle time on the hardware-thread sibling, below the 95% threshold for the timing comparison. All eight shorter verification panels and the 228-specification corpus preserved their reference output, errors and exit codes. [Measurements, original inputs and independent answers](perf/ivy_apple/holdouts/nov23-observed-universals.tar.xz)

Independent Z3 processes found all **909** original premise-and-goal pairs UNSAT. Of their ground subsets, 360 were UNSAT. Adding the short universal premises brought the native total to 656. The Go proof found **568**, including 208 that required a universal. It instantiates the entire positive premise at well-sorted arguments of an application already present in the input. For example, a definition of `q(x)` supplies a fact about `q(a)` when the query mentions that application. Preserving Boolean equations as gates allows unit propagation to use the defining condition. Unit propagation found all 568 conflicts in this corpus. The projection declined every one of the 253 selected subsets with a native model. [Implementation](api/internal/order/observed_ground_instances.go) · [Binder scope, polarity and finite function tables](api/api_observed_instances_test.go)

The new original verification fixture, search 948, took a median **25.47 ms** before these changes, **28.72 ms** with only the ground proof, and **3.51 ms** with observed instances. The ground-after-limit fixture took 224.03, 13.69 and 15.03 ms respectively. Extra instantiation work has a cost on queries already settled by ground congruence. Each measurement used five iterations on a fresh incremental solver, in each of five rotating runs. Run `go test ./api -run '^$' -bench '^BenchmarkHermesPremiseVerification$' -benchtime=5x -cpu=1`. [Go benchmark](api/api_short_premises_test.go) · [Native-verified original input](api/test_vectors/hermes_observed_universal.smt2.gz) · [Per-run data](perf/ivy_apple/holdouts/nov23-observed-universals.tar.xz)

### Chaining ground instances on the complete Hermes verification

The first quiet, same-core rotation of the complete 6,338-check Hermes verification took **166.00 s with g3, 174.61 s with the previous g3, and 173.52 s with linked Z3**. The new g3 used **1.128 trillion instructions** against 1.256 trillion for the previous g3 and 1.259 trillion for Z3. This is a 10.4% instruction saving and a 4.3% elapsed-time saving relative to Z3 on this rotation. All three used the same GoIvy frontend at `104078aaa`, one Go processor and default GC. The qualifying runs on CPU 15 had at least 97% idle time on the sibling thread and at least 99% process CPU occupancy. A second CPU gave 161.51, 169.27 and 170.40 s respectively. On a third, the current g3 had 90.5% CPU occupancy and took 194.71 s. Wall time on this shared host varies enough to justify repeated paired runs. All eight shorter verification panels and the separate 228-specification corpus reproduced their reference outputs, errors and exit codes. [Whole-process counters, timing controls and verdicts](perf/ivy_apple/holdouts/nov23-chained-universals.tar.xz)

A universal definition can introduce the application needed to use another definition. For example, instantiating `q(x) = p(k(x))` at `a` makes `k(a)` available to the rule defining `k`. The verifier now indexes generated ground applications in the same query, including those introduced by larger positive universal premises. It instantiates only complete, well-sorted binder tuples and asserts the whole body. It also propagates forced truth values through equality classes and conditionals. An undecided projection falls through to the ordinary solver. Each query has its own index and resource budget. Four rule passes, at most eight new bindings per rule per pass, and 640 successful bindings in total bound recursive definitions. [Instantiation](api/internal/order/observed_ground_instances.go) · [Equality and conditional propagation](api/internal/order/ground_equalities.go)

| Independent Z3 population | Queries | Previous g3 refutations | Current g3 refutations |
|---|---:|---:|---:|
| Full original Hermes queries, all UNSAT | 909 | 568 | **865** |
| Short selections, UNSAT | 656 | 568 | **652** |
| Short selections, SAT | 253 | 0 | 0 |
| Native-core controls, UNSAT | 266 | — | 222 |
| Native-core controls, SAT | 927 | — | **0** |

The core controls retain a native UNSAT core or remove one premise from it. These closely related satisfiable inputs test whether chaining can accidentally prove a counterexample impossible. An independent two-element function-table test also checks 512 combinations of rule order, interpretation and conclusion. On the 44 undecided original queries, we captured the exact generated ground assertions and ran them separately in both SMT shells. Both found 20 UNSAT and 24 SAT. Stronger ground deductions could settle at most the first 20 with these assertions. The other 24 require additional instances or another source of proof. [Original queries and native answers](perf/ivy_apple/holdouts/nov23-observed-universals.tar.xz) · [Captured instances and core controls](perf/ivy_apple/holdouts/nov23-chained-universals.tar.xz) · [Finite interpretation tests](api/api_observed_instances_test.go)

| Captured verification goal, batch mode | Previous g3 | Current g3 |
|---|---:|---:|
| Ground conflict | 10.95 ms | 14.25 ms |
| Nested ground conflict | 11.03 ms | 14.17 ms |
| Former fallback | 99.28 ms | **11.08 ms** |
| Ground conflict after the old search limit | 15.04 ms | 18.78 ms |
| One short universal | 3.80 ms | 4.46 ms |
| Chained universal, search 1058 | 51.54 ms | **6.82 ms** |
| Larger universal, search 994 | 80.35 ms | **8.98 ms** |

The two new fixtures improve **7.6× and 8.9×**. Looking for instances before resolving an existing ground conflict increases the first fixture's time by 30%. The full workload favors chaining. A cheap ground conflict check before reading all universal bodies is worth testing on both kinds of query. Each cell is a median of five rotated measurements, five iterations per measurement, with a fresh solver per iteration. Run `go test ./api -run '^$' -bench '^BenchmarkHermesPremiseVerification$' -benchtime=5x -cpu=1`. [Retained benchmark](api/api_short_premises_test.go) · [Seven fixtures, three builds and all samples](perf/ivy_apple/holdouts/nov23-chained-universals.tar.xz)

### Complete Apple and Hermes runs on the same frontend

On three qualifying rotations of `jea/hermes_rmw_o3_testing.ivy`, current g3 finished in a median **153.28 s**, the previous g3 in **174.61 s** and linked Z3 in **172.32 s**. Each verified all **6,338** checks with identical output. Median whole-process instruction counts were 1.128, 1.256 and 1.259 trillion respectively. The chained instances saved 12.2% of elapsed time against the previous g3 and 11.1% against Z3. All three binaries used GoIvy `104078aaa`, one Go processor, default GC and CPU 15. Acceptance required 95% idle before the run, 95% idle on the hardware-thread sibling during the run and 95% process CPU occupancy. The archive includes rejected attempts. Ordinary `jea/hermes_rmw_o3.ivy` exercises a different set of 2,575 checks. The earlier corpus comparison measured 75.50 billion instructions for g3 against 50.66 billion for Z3 on that file. [Timing rotations, instrumentation and heap profiles](perf/ivy_apple/holdouts/nov23-complete-apple-hermes.tar.xz)

The complete original Apple specification, `doc/examples/apple/ord_live.ivy`, passed all **11,262 checks across 16 named isolates**. Two runs in opposite backend order used different physical cores. Each reproduced the entire native transcript. The 1.61× and 1.66× speedups apply to the whole verification on those two paired runs. A rotation against the preceding g3 revision on CPU 21 shows that the Apple advantage was already present before the latest Hermes change. [Full Apple transcript, counters and isolate timings](perf/ivy_apple/holdouts/nov23-complete-apple-hermes.tar.xz)

| Complete original Apple run | g3 | Linked Z3 | g3 instructions | Z3 instructions |
|---|---:|---:|---:|---:|
| CPU 14, Z3 first | 107.97 s | 174.09 s | 814.69 billion | 1,060.64 billion |
| CPU 10, g3 first | 227.88 s | 377.34 s | 812.69 billion | 1,062.14 billion |

The core-to-core time difference was large even with instruction counts within 0.3% for each backend. On CPU 14, the `dramc_nb2` interval inside the full run took **17.70 s versus 20.40 s**, `rfn2` took **1.34 s versus 5.23 s**, and `sys_live` took **12.51 s versus 19.46 s**. On the separately versioned `ord_live2.ivy`, the complete 1,103-check `sys_live` isolate took median **226.82 s versus 479.31 s** over three qualifying rotations. The complete 795-check `jea/issue73/ord.ivy` took **22.47 s versus 138.25 s** on the same basis. [All timing samples and eight instrumented isolate traces](perf/ivy_apple/holdouts/nov23-complete-apple-hermes.tar.xz)

The heap profiles give a target shared by both verifiers. Go allocated 92.38 GB in 1.038 billion allocations on the complete Hermes testing run with g3. With the native backend the Go portion allocated 81.73 GB in 1.007 billion allocations. The native library's C++ allocations are outside the Go heap measurements. Sampling traced 25.78 GiB of direct Go allocations in g3's verifier process to GoIvy's recursive AST renaming. The saved profiles include the allocation call trees and hardware instruction samples for attributing work in the frontend. [Heap profiles, per-process GC totals and sampled stacks](perf/ivy_apple/holdouts/nov23-complete-apple-hermes.tar.xz)

### Why proving more goals before the first push slowed down TLB

We captured all 58 first goals that reached incremental solving in the TLB verification and obtained an independently replayed Z3 refutation core for each. Running native Z3 on the exact ground facts produced by g3 distinguished two costs. With the full inputs, only **4 of 58** projections contained enough facts to refute the query, and the bounded Boolean prover finished two. When given just the relevant quantified premises from the independently obtained cores, the same projection generated sufficient ground facts in **31 of 58**. The fixed instantiation budget often went to irrelevant terms in the full inputs. Even on the cores, a stronger Boolean deduction could use more of the generated facts. [Source inputs, cores, exact ground scripts and reference answers](perf/ivy_apple/holdouts/nov23-pending-proof-experiments.tar.xz)

A general implication lets a solver use equality during a Boolean search. For example, the equality `x = y` implies that `P(x)` and `P(y)` have the same truth value, including when other arguments of a predicate already match. Adding this implication and allowing wider Boolean bodies raised refutations on the 58 original queries from **2 to 24**. Seeding more terms and increasing the binding cap raised that to **26**. On the known cores the counts rose to **50 and 52** respectively. Removing one premise at the beginning, middle or end of the 164 captured cores across six workloads yielded 462 controls. Native Z3 found 445 SAT and 17 UNSAT. Every variant declined all 445 SAT controls. [All 462 inputs and four independent proof comparisons](perf/ivy_apple/holdouts/nov23-pending-proof-experiments.tar.xz)

| Complete TLB verification | Early attempts | Early refutations | Process instructions |
|---|---:|---:|---:|
| Released g3 | 0 extra | 0 extra | **95.77 billion** |
| Wider bodies and conditional congruence | 236 | 187 | 110.11 billion |
| Same, with a premise selection pass | 235 | 186 | 109.76 billion |
| More seeds and a larger binding cap | 406 | 363 | 132.33 billion |

Each early success lets the next goal reach the same path. The repeated construction and search cost more than the incremental calls they avoided. Those changes remain experimental. Separate first-push pass omissions reduced TLB from 95.50 to 88.50 billion instructions but increased toy consensus from 3.13 to 4.45 billion. Removing pattern inference alone raised TLB to 471.66 billion. The next candidate needs to reduce repeat work in the ordinary solve or reuse already constructed facts, and should be measured on the complete verification. The archive includes a standalone Go overlay replay and both preprocessing sweeps. [Live verifier counters, test overlay and pass omissions](perf/ivy_apple/holdouts/nov23-pending-proof-experiments.tar.xz)

### Eliminating equality-guarded universals on ordinary Hermes

The complete 2,575-check `jea/hermes_rmw_o3.ivy` took a median **8.97 s with g3, 10.53 s with the previous g3 and 7.16 s with linked Z3**. All five qualifying rotations per engine on CPU 15 produced identical output, diagnostics and exit status. Median user-mode instruction counts fell from **76.72 to 64.19 billion**, against **50.68 billion** for Z3. This cuts elapsed time by 14.8% and removes 48% of the preceding instruction gap. A second CPU gave medians of **8.99, 10.72 and 7.29 s** over three qualifying rotations. The separate 6,338-check Hermes testing workload has its own measurements above. [Timing samples, complete panels and source revisions](perf/ivy_apple/holdouts/nov23-guarded-universals.tar.xz)

A counterexample to `forall x. A(x) => (B(x) => C(x))` must satisfy both guards. If either guard requires `x = k` as a conjunct, the only possible counterexample is at `k`. The solver now instantiates the original body at that ground term, retaining all other conditions. It uses the existing capture-avoiding instantiator so references to outer variables keep their meaning. The optional verification API accepts single-variable universals of any sort and keeps the existing proof, tracing and resource guards. GoIvy calls it before the ordinary incremental push. [g3 implementation](qe/guarded_universals.go) · [GoIvy integration and regression](https://github.com/glycerine/ivy/commit/95a9ad062)

The saved SMT-LIB combined two implication guards that remained nested in GoIvy's live expression. Handling only the outer implication produced **zero substitutions in 959 concrete goals**, despite strong isolated replay results. Following the nested implications changed **17** live goals. Their combined check instructions fell from **13.43 to 1.63 billion**, an **8.3× reduction** measured inside the solver calls. Two independent SMT processes found the exclusive-or of the original and transformed goals UNSAT for all 17 occurrences, comprising six distinct scripts. The complete 228-specification comparison and ten additional verification panels reproduced their reference outcomes. The Go tests also exercise both polarities, scope and shadowing, guards that do not fix a value, annotations and 64 finite interpretations containing both satisfiable and unsatisfiable examples. [Live formulas, independent equivalence queries and counts](perf/ivy_apple/holdouts/nov23-guarded-universals.tar.xz) · [Semantic tests and benchmark](api/api_guarded_universals_test.go)

| Fresh incremental replay, three checks per sample | Original goal | Simplified goal | Speedup |
|---|---:|---:|---:|
| Hermes batch 27, condition 10 | 3,410 ms | 241 ms | **14.1×** |
| Hermes batch 29, condition 10 | 828 ms | 93 ms | **8.9×** |

The table gives the median time per check across seven samples per implementation on one pinned CPU. Each operation loads the parsed base on a fresh solver, optionally simplifies the captured nested goal, pushes and checks UNSAT. The whole verifier shares earlier incremental state. To reproduce, run `go test ./api -run '^$' -bench '^BenchmarkHermesGuardedUniversal$' -benchtime=3x -benchmem -cpu=1`. The remaining ordinary-Hermes gap on CPU 15 is **13.50 billion instructions and 1.81 s**. The largest individual remaining gap in the instrumented live replay is batch 10, condition 70 at **3.05 billion versus 1.36 billion instructions**. The archive also includes all 132 ranked original inputs and independently replayed native cores for follow-up experiments. [Benchmark inputs, samples and rankings](perf/ivy_apple/holdouts/nov23-guarded-universals.tar.xz)

### What a fresh SMT check spends on rewriter construction

A small ground Hermes input constructed **878 theory rewriters in one g3 shell process**. Stack instrumentation counted 250 before the check, 622 during it and six at shutdown. Of the 622, **466** came from cleaning up simplifier tactics and **96** from reinitializing value propagation. The default strategy builds alternatives for eleven logic fragments. Cleanup visits alternatives even when the goal selects the final SMT solver. The C++ tactic solver also cleans up before and after the check, and its tactic execution cleans up a third time. Its own hardware samples show substantial time in these constructors. [Go tactic solver](solver/tactic_solver.go) · [C++ tactic solver](../src/solver/tactic2solver.cpp) · [Call counts and profiles](perf/ivy_apple/holdouts/nov23-rewriter-construction.tar.xz)

Across 36 ground inputs and 36 quantified counterparts, g3 constructed 65,237 theory rewriters averaging about **56,200 user instructions each**. Bit-vector setup accounted for 25%, datalog setup for 15%, arithmetic setup for 13% and the initial registration of declaration plugins for another 13%. Registration repeatedly interned eight fixed names and looked up their family IDs on the same AST manager. Once registration succeeds, those bindings live until that manager is destroyed. g3 now remembers that success on the owning manager. Partial registration can still be retried and a closed owner still returns an error. The retained benchmark fell from **714.2 to 4.41 ns per repeat**, with zero Go allocations in both builds. The whole ground check used **5.0% fewer instructions**. Run `go test ./ast -run '^$' -bench '^BenchmarkRepeatedDeclarationRegistration$' -benchtime=200000x -cpu=1 -benchmem`. [Implementation and boundary tests](ast/ast_reg_decl_plugins.go) · [Benchmark](ast/ast_reg_decl_plugins_test.go)

| Retired user instructions per check, millions | Ground | With quantified premises |
|---|---:|---:|
| Registration trial, preceding g3 | 155.42 | 285.04 |
| Remember completed registrations | **147.64** | **274.40** |
| Branch construction trial, preceding g3 | 156.79 | 286.37 |
| Construct selected branches, retain initial arena terms | **85.03** | **212.16** |
| In-tree C++ Z3 | **40.61** | **68.58** |

Each cell averages two measurements of each of 36 inputs. The two experiments have independent controls. On the branch trial, loading the ground queries fell from 101.23 to 50.01 million instructions, versus 57.26 million in Z3. All 72 query answers matched. On a broader set of 1,655 shell inputs, delaying all eleven branches also changed an exposed floating-point model. Retaining the initial arena terms restored that model. The seeded variant still differed on 14 complete transcripts, including reported statistics, timing in traces and the number of warnings for an invalid quantifier cost expression. The released change is the registration optimization. A future branch optimization needs to preserve parameter descriptions, constructor diagnostics and resource behavior. Its measured opportunity applies to fresh tactic-backed checks. GoIvy's usual incremental route already defers its default portfolio. [Trial patches, complete controls and phase measurements](perf/ivy_apple/holdouts/nov23-rewriter-construction.tar.xz)

### Building GoIvy structural sort keys once

On the 6,338-check Hermes testing workload, g3 allocated **5.40 GB less** and made **72,266,073 fewer Go allocations** after changing how the shared frontend builds symbol keys. The original full Apple run saved **2.39 GB** and **40,290,614 allocations**. The frontend formerly built and discarded a string for each nested function domain and range. It now joins the components of the final key once, at the existing construction point. Custom sort implementations still supply their own structural spelling. [GoIvy implementation, regression tests and benchmark](https://github.com/glycerine/ivy/commit/88aca06ce) · [Heap profiles and counters](perf/ivy_apple/holdouts/nov23-structural-sort-keys.tar.xz)

| Complete workload | g3 instructions before → after, billions | Linked Z3 instructions before → after, billions | g3 Go heap allocations before → after, millions |
|---|---:|---:|---:|
| Hermes testing, 6,338 checks | 1,124.575 → **1,097.835** | 1,259.644 → **1,222.832** | 1,037.281 → **965.015** |
| Apple, 11,262 checks | 813.879 → **797.663** | 1,060.944 → **1,040.973** | 492.285 → **451.994** |
| Ordinary Hermes, 2,575 checks | 63.272 → **62.885** | 50.647 → **50.266** | — |

Each long-run instruction count is the median of three complete alternating measurements per version on a pinned CPU. Ordinary Hermes used five pairs. Both backends use the same frontend change from GoIvy `95a9ad062` to `88aca06ce`. Heap totals come from separate complete processes and use decimal GB. On ordinary Hermes, five quiet samples per engine took **8.439 s before, 8.463 s after and 6.742 s with linked Z3**. The small instruction saving there did not yield a measurable elapsed-time improvement. [Complete paired samples, pinned timing controls and allocation measurements](perf/ivy_apple/holdouts/nov23-structural-sort-keys.tar.xz)

A binary predicate key fell from **458.7 to 264.9 ns** and from seven allocations to two. A nested function key fell from **534.5 to 288.1 ns**, from eight allocations to two. The Boolean case stayed at two allocations. Reproduce these medians in GoIvy with `go test -tags=xtrace_off -run '^$' -bench '^BenchmarkConstantStructuralKey$' -benchtime=100000x -benchmem -cpu=1 -count=7`. All 228 verifier controls matched their reference stdout, stderr and exit status. With tracing enabled, the complete 2,334,030,415-byte rfn2 stdout matched before and after, including all 524,035 XTRACE records. The complete Paxos trace also matched its Python reference. The ordinary release-mode GoIvy suite passed in 58.97 s. A separate full-suite attempt with trace output enabled reached its 35-minute cap inside the long Apple test. [Benchmarks and trace comparisons](perf/ivy_apple/holdouts/nov23-structural-sort-keys.tar.xz)

### Classifying assertions during first-push setup

On 256 asserted clauses, the setup phase of the first push fell from **835.0 to 8.83 µs**. The old basic-arithmetic and sequence policies each scanned the assertions even when their configured backend did not consult those statistics. Ordinary Hermes took both walks in 539 solver setups. A fresh ground shell check could scan the same assertions five times before searching. g3 now reads the formulas when a policy needs the statistics and shares that result until setup returns. A failed setup drops its summary before a retry. The storage belongs to a single solver operation. [Setup implementation](smtcore/smt_asserted_bounds.go) · [Arithmetic backend selection](smtcore/smt_setup_basic_arithmetic.go) · [Retry test](smtcore/smt_setup_string_test.go)

| Isolated setup, 256 clauses | Previous g3 | g3 after change | Bytes allocated before → after |
|---|---:|---:|---:|
| First push, logic selected | 834,955 ns | **8,833 ns** | 214,192 → **192** |
| First check, automatic configuration | 2,073,771 ns | **434,928 ns** | 535,432 → **107,432** |

The benchmark times the production setup call after constructing a fresh solver and asserting its clauses. At 32 clauses the corresponding times were 133.0 → 7.68 µs and 294.7 → 79.8 µs. Commits `3158e1d5f` and `111c1cb1d` also cover finite search limits. In 2,314 observed classifications the search-step charge was zero. Trace, diagnostic and injected allocation-failure paths preserve the original collection points. Reproduce with `go test ./smtcore -run '^$' -bench '^BenchmarkSMTAssertionFeatureSetup$' -cpu=1 -benchmem -benchtime=50x -count=9`. [Benchmark](smtcore/smt_setup_features_benchmark_test.go) · [Run data and commands](perf/ivy_apple/holdouts/nov23-assertion-feature-setup.tar.xz)

| Complete GoIvy process, median billion user instructions | Previous g3 | g3 after change | Linked Z3 |
|---|---:|---:|---:|
| Ordinary Hermes | 62.46 | **61.58** | 50.37 |
| TLB | 93.89 | **92.53** | 65.94 |
| Apple cf_live | 45.14 | **43.65** | 55.06 |
| Full Apple, 11,262 checks | 795.69 | **787.47** | 1,041.15 |
| Apple rfn2 | 12.31 | **12.03** | 49.30 |

The full Apple row uses three runs per engine and the other rows use four, pinned to the same CPU with the same GoIvy frontend. On 36 standalone ground queries, the mean check fell from 148.96 to **135.19 million instructions**. On 36 quantified queries it fell from 274.21 to **255.31 million**. In-tree Z3 used 40.61 and 68.58 million respectively. These counters exclude shell parsing. All 228 verifier controls matched reference stdout, stderr and exit status. Of 720 explicit resource-limit comparisons, 717 finished with matching responses and resource counts. Three unbounded arithmetic queries reached the wall cap in both builds. An added contradiction made all three decide UNSAT. The shell comparison covered 1,799 inputs, including a datalog example that returned the same timeout response when invoked with its intended timeout option. [Complete measurements and controls](perf/ivy_apple/holdouts/nov23-assertion-feature-setup.tar.xz)

### Reusing the private AST manager between verification goals

Five quiet, pinned runs of ordinary Hermes took **8.467 s before and 8.089 s after** letting a verification batch reuse its private AST manager. Linked Z3 took **6.833 s** on the same core. The 397 short-premise attempts in an instrumented baseline cost 9.75 billion instructions. Of these, 1.25 billion went into preparing their private contexts and 5.54 billion into the actual child solver checks. Only two consecutive attempts had the same selected premise prefix and budget. A batch can amortize the cost of an arena even when its goals need different premises. [Instrumentation and complete timings](perf/ivy_apple/holdouts/nov23-batch-private-proof-contexts.tar.xz)

Commit `308e67d2b` gives each eligible batch one private manager for up to 16 checks. Each check creates a fresh solver kernel, translates and asserts its own terms, and detaches its cancellation link on return. Releasing the batch also releases its manager. Custom arena owners, debug tracing, injected allocation failures and runtime memory limits use the original per-check lifecycle. The tests alternate contradictory and consistent inputs, exhaust a child step limit, retry after cancellation and rotate the manager with live outer assertions. [Context lifetime](api/api_proof_plan_context.go) · [Production check](api/api_goal_instances_check.go) · [Boundary tests](api/api_proof_plan_context_test.go)

| Complete verifier | Previous g3, billion instructions | g3 with private manager reuse | Linked Z3 |
|---|---:|---:|---:|
| Ordinary Hermes, quiet CPU | 63.02 | **60.18** | 50.26 |
| Hybrid reliable broadcast | 12.00 | **11.05** | 5.90 |
| Apple cf_live | 43.63 | **42.52** | 55.06 |
| Original Apple, all 11,262 checks | 788.78 | **772.75** | 1,041.13 |
| Hermes testing, 6,338 checks | 1,093.05 | **1,085.02** | 1,221.43 |

All builds used GoIvy `88aca06ce`. The ordinary Hermes runs required at least 95% process CPU occupancy and 95% idle time on its sibling hardware thread. The other rows use four complete runs per engine for the two isolates and three for the two long workloads. In the independent compatibility corpus all 228 runs matched reference stdout, stderr and exit status, including 91 expected nonzero exits. With tracing enabled, the complete 2,334,030,415-byte rfn2 stdout matched and the Paxos output matched its Python reference. [Raw runs, corpus and trace checks](perf/ivy_apple/holdouts/nov23-batch-private-proof-contexts.tar.xz)

In nine 100-iteration benchmark samples, creating a new context per check took **2.349 ms, 1,221,829 B and 3,081 allocations**. Reusing the manager took **0.571 ms, 322,626 B and 1,208 allocations**. A second experiment alternated the two modes in 15 pairs. Each sample covered exactly four 16-check rotations. The medians were 4.533 → **1.521 ms**, 1,221,838 → **315,288 B** and 3,081 → **1,193 allocations**. The two timing experiments ran under different host loads. The archive includes the 4-, 16- and 64-check lifetime trials and the experiment that retained translated terms. Run `go test ./api -run '^$' -bench '^BenchmarkPrivateProofPlanContexts$' -cpu=1 -benchtime=64x -benchmem`. [Retained benchmark](api/api_proof_plan_context_benchmark_test.go) · [Samples and alternatives](perf/ivy_apple/holdouts/nov23-batch-private-proof-contexts.tar.xz)

### Small formulas that contain no expandable macro

In 181,662 calls from 82 verification and shell workloads, g3 could determine the existing macro expander's result and exact step count for **149,762** calls by examining at most 32 AST nodes. An instrumented build still performed every original rewrite. All 149,762 returned the original handle and charged the predicted number of steps, including 59,766 calls under an explicit resource limit. The production probe tracks shared terms separately in each quantifier scope. A macro application, conditional or exhausted visit allowance goes through the general expander. A probe that succeeds reserves exactly the original step charge. If the charge would cross the limit, the original traversal takes its normal cancellation path. [Implementation](ast/macros/macro_expansion_identity.go) · [Tests for binders, shared nodes and step boundaries](ast/macros/macro_expansion_identity_test.go) · [Per-workload audit](perf/ivy_apple/holdouts/nov23-macro-expansion-misses.tar.xz)

| Macro expansion, 21 paired samples of 500 calls | Previous g3, ns | g3 at 3ae6121be, ns | Go allocations before → after |
|---|---:|---:|---:|
| Unchanged leaf | 1,076 | **100** | 7 → 0 |
| Unchanged chain | 3,179 | **245** | 16 → 0 |
| Unchanged shared subterm | 3,519 | **321** | 19 → 0 |
| Macro substitution | 32,751 | 33,798 | 95 → 95 |
| Constant conditional | 28,877 | 28,411 | 97 → 97 |

On 36 standalone quantified queries measured twice, the mean check fell from **257.55 to 242.99 million user instructions**. In-tree Z3 used 68.58 million. On 36 ground queries the means were 134.72, 135.38 and 40.61 million respectively. These counters bracket only the check. Across four complete runs per engine, TLB fell from **91.26 to 88.90 billion instructions**, against 65.84 billion with linked Z3. Hybrid reliable broadcast fell from 11.07 to 10.88 billion, against 5.90 billion. Ordinary Hermes moved from 58.15 to 57.94 billion, against 50.61 billion. The table shows why this change helps the quantified workload much more than workloads that actually substitute macros. Reproduce with `go test ./ast/macros -run '^TestMacroExpansion' -bench '^BenchmarkMacroExpansion$' -benchtime=500x -benchmem -cpu=1`. [Benchmark](ast/macros/macro_expansion_benchmark_test.go) · [Individual samples, shell phases and full proofs](perf/ivy_apple/holdouts/nov23-macro-expansion-misses.tar.xz)

All 228 verifier controls matched both reference streams and exit status, including 91 expected failures. The 1,799-input shell comparison preserved normalized responses. The packaged datalog timeout produced the same timeout response on both builds. The 720-pair resource-limit sweep had 717 matching completed responses and three unbounded cases that reached the wall cap in both g3 builds. Adding a contradiction made those three decide UNSAT. The complete rfn2 trace matched on all 2,334,030,415 bytes. Paxos matched its Python reference across 27,855 output lines. [Controls and trace checks](perf/ivy_apple/holdouts/nov23-macro-expansion-misses.tar.xz)

### Rerunning the original scope-restoration make targets

Thirteen alternating pairs on CPU 15, with 10,000 pops per sample, gave these medians on the Threadripper 3960X. All runs restored 128 atoms and 64 clauses per operation. g3 reported zero allocations per operation in both cases. [Measurements](perf/scope_restoration/nov23-final-make-targets.csv) · [Make-target runner](perf/scope_restoration/make_targets.py)

| Timed pop | g3, make bench | In-tree Z3, make cppbench | g3 / Z3 time |
|---|---:|---:|---:|
| Atoms surviving the pop | **376.7 ns** | 490.6 ns | 0.77 |
| Atoms introduced inside the popped scope | 13.822 µs | **13.195 µs** | 1.05 |

These results compare the two requested make targets. The native fixture has its delayed activity queue enabled. The Go target times the default fixture, and the separate Go benchmark `BenchmarkSMTScopeRestorationWithQueue` enables the matching queue and brackets only the pop with a monotonic clock. [Go benchmark](smtcore/smt_hotpath_benchmark_test.go) · [Native benchmark](../src/test/smt_scope_restoration.cpp) · [Per-worker benchmark](smtcore/smt_scope_workers_benchmark_test.go)

### The scope benchmark with the same activity queue

With the delayed activity queue enabled in both solvers, the 128-atom scoped pop took **18.878 µs in Go and 12.641 µs in C++**. When the atoms survived the pop, g3 took **281.8 ns** and Z3 took **480.1 ns**. Each median covers 13 alternating pairs of 10,000 pops pinned to CPU 15. We required at least 95% process CPU occupancy and at least 95% sibling idle time during each run. One native attempt failed admission and was repeated. The Go measurement uses the fixture’s `pop-ns/op` metric. Reproduce with `go test ./smtcore -run '^$' -bench '^BenchmarkSMTScopeRestorationWithQueue$' -cpu=1 -benchtime=10000x` and `../build/test-z3 smt_scope_restoration 10000 Both`. [Individual timings and admission readings](perf/scope_restoration/pop_phases/nov24-queue-elapsed.csv)

| Instructions per queue-enabled pop | g3 | In-tree Z3 | g3 minus Z3 |
|---|---:|---:|---:|
| Full pop, scoped atoms | 202,770 | 144,725 | +58,045 |
| Undo interleaved trail entries | 57,361 | 19,657 | +37,704 |
| Pop relevance scope | 19,471 | 8,036 | +11,435 |
| Restore learned clauses | 106,688 | 89,217 | +17,471 |
| Full pop, surviving atoms | 2,312 | 3,695 | −1,383 |

Undo accounts for **65% of the scoped instruction gap**. The three named subphases add to more than the whole-pop difference because the remaining work costs more in the native fixture. Each row is a separate hardware-counter experiment on CPU 45, with nine alternating pairs, 1,500 observed intervals after eight warmup pops and an empty interval subtracted in each pair. Counter running time equaled enabled time in all 126 readings. The fixture checks that scoped atoms are actually retired and republished, watch clauses keep their original identities and surviving atoms keep their relevance. [Raw thread counters](perf/scope_restoration/pop_phases/nov24-queue-instructions.csv) · [Measurement driver](perf/scope_restoration/pop_phases/measure.py)

### Quantifier bodies and arena symbol records

Changing the body of a 64-binder quantifier fell from **34.66 to 3.82 µs**, a **9.1×** speedup. g3 had been decoding and encoding each retained name, sort and pattern even though the record layout and contents were identical. Commit `1aa5419d9` copies each retained array between two bounded arena spans. Commit `3f69953fd` also validates a symbol record once when the factory actually needs to encode or decode its fields. The arena still owns the records, including both copies when two quantifiers survive. [Quantifier update](ast/ast_core_update.go) · [Symbol encoding and bounds tests](ast/ast_core_payload_test.go)

| Operation and lifetime | Binders | Previous g3 | Changed g3 | In-tree Z3 |
|---|---:|---:|---:|---:|
| Body update, fresh result, block-copy experiment | 1 | 901 ns | 467 ns | 44 ns |
| Body update, fresh result, block-copy experiment | 8 | 4,613 ns | 764 ns | 67 ns |
| Body update, fresh result, block-copy experiment | 64 | 34,664 ns | 3,822 ns | 411 ns |
| Construct from names and sorts, existing result, symbol experiment | 1 | 721 ns | 416 ns | 45 ns |
| Construct from names and sorts, existing result, symbol experiment | 64 | 17,356 ns | 11,084 ns | 322 ns |
| Body update, existing result, symbol experiment | 64 | 12,315 ns | 6,389 ns | 316 ns |

Each experiment has its own paired baseline. The first used 13 alternating rounds and the second used 15, with 20,000 iterations per cell. Small updates varied. In the symbol experiment, fresh two-binder updates took 561 → 647 ns and four-binder updates took 683 → 772 ns. Reproduce with `go test ./ast -run '^$' -bench '^BenchmarkASTQuantifier(BodyUpdate|Construction)$' -cpu=1 -benchtime=20000x -benchmem`. The matching native fixture accepts `construct` for construction and defaults to body update. [Go benchmark](ast/ast_quantifier_update_benchmark_test.go) · [C++ benchmark](../src/test/perf/quantifier_body_update.cpp) · [Every measurement and prototype patch](perf/ivy_apple/holdouts/nov24-quantifier-arena-records.tar.xz)

The large-binder microbenchmark overstates the opportunity on ordinary proofs. Across the whole Apple input, **152,535 of 174,593** updates that changed the body had at most two binders. Another 356,917 updates returned their original quantifier. In ordinary Hermes the corresponding short-update fraction was **25,859 of 29,510**. Three complete runs per engine put full Apple at **769.86 → 767.67 billion instructions** after block copying, versus **1,041.51 billion** with linked Z3. Full Hermes testing was **1,084.25 → 1,083.07 billion**, versus **1,221.01 billion**. Ordinary Hermes used 59.93 → 59.46 billion in seven quiet rounds, but elapsed time rose from 8.335 to 8.509 s. The separate symbol experiment measured 7.837 → 7.843 s against 6.785 s native on ordinary Hermes. Each release preserved all 228 verifier outcomes and reference streams. All 1,799 shell controls matched after normalizing volatile statistics. The bare datalog fixture reached the wall cap in both builds and produced the expected timeout response with its packaged option. Both rfn2 and Paxos trace comparisons passed. [Shape census, complete process counters and controls](perf/ivy_apple/holdouts/nov24-quantifier-arena-records.tar.xz)

Two alternatives remain prototypes. Embedding two binders added 64 bytes per quantifier record and cut a retained two-binder update from 1,159 to 513 ns. Four complete pairs measured 59.60 → 59.37 billion instructions on ordinary Hermes, 89.10 → 88.87 billion on TLB and 42.21 → 42.20 billion on Apple cf_live. Looking up a duplicate before allocating its binder arrays cut a retained 64-binder construction from 14.77 to 5.10 µs. Despite 272,687 duplicate factory calls in 289,393 on full Hermes testing, the lower-level lookup yielded small changes on full runs. On three complete TLB pairs it saved 0.22 billion instructions out of 89.27 billion. These measurements favor removing repeated solver work over enlarging every quantifier. [Prototype widths, duplicate rates and complete comparisons](perf/ivy_apple/holdouts/nov24-quantifier-arena-records.tar.xz)

### Why grouping dense Boolean IDs only partly speeds up scope undo

The Boolean IDs retire in reverse creation order. The delayed activity queue stores variables in priority order. In a two-iteration audit, all 250 proposed contiguous removals failed the delayed queue's suffix check. Repeated scans inspected 16,500 candidate variables. In nine paired samples, the unbounded scan raised undo from **57,417 to 347,555 instructions per scoped pop**. Limiting the scan to eight variables still cost 116,424, against 57,485 in that run's control. A valid dense ID interval gives no guarantee about where its members sit in the heap. [Audit and scoped phase counters](perf/scope_restoration/pop_phases/nov24-boolean-retirement-trials.tar.xz)

Keeping the actual deletion order while releasing surviving arena references together reduced undo to **46,777 instructions**, against **19,657** in C++. The whole pop fell from 203,439 to 190,457 instructions, versus 144,725 in C++. The fastest instruction-count prototype bypassed an error return after a heap preflight. It remains experimental. A simpler single-reference change preserved the general deletion routine and measured 54,161 instructions in undo. Each phase used nine paired 1,500-pop samples with empty brackets subtracted. The existing variable-level counter and queue-matched benchmark can distinguish future heap improvements from moving work into the next variable allocation. [Variable measurement driver](perf/scope_restoration/pop_phases/variables.py) · [All alternatives and observations](perf/scope_restoration/pop_phases/nov24-boolean-retirement-trials.tar.xz)
