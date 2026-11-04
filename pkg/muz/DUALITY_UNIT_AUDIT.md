# Duality profiling unit audit

Scope: `src/duality/duality_profiling.cpp` and `.h`, marked ported in `port_manifest.json` before this audit, plus their retained consumers. This is a source and consumer audit, not a claim that the Duality solver or interpolation profiler is complete. Reviewed 2026-11-03; no manifest status is changed here.

## Source surface and ownership

| Source | Go ownership and evidence |
| --- | --- |
| Header guard and four declarations: `timer_start`, `timer_stop`, `print_profile`, `show_time` | `pkg/duality/profiling.go`: `TimerStart`, `TimerStop`, `Print`, `ShowTime`. The header guard has no Go runtime counterpart. |
| Global `top`, `current`, initializer and node constructor | Source-facing Duality backends share a process-lifetime root and current pointer; node children, elapsed time, start and parent live in the profiler tree. Explicitly supplied Go profilers remain independently owned. |
| `timer_start`: look up child by `const char*` identity, initialize name and parent when empty, overwrite start time, descend | `TimerStart` uses string-value keys, preserves an existing child's elapsed time, resets its start and descends. Nil and empty names return an error; all identified retained source sites pass nonempty literals. Source pointer identity can distinguish equal text at different addresses; Go merges equal names under a parent. |
| `timer_stop`: on mismatch climb to matching ancestor, charge it if not root, otherwise do nothing; normal stop charges and ascends | `TimerStop` implements this recovery; `TestProfilerImbalancedStopRecoversLikeSource` covers a lost child and missing name. The `#if 0` fatal mismatch diagnostic is disabled and has no live counterpart. |
| `print_node`, `time_entry`, `ltstr`, `print_profile`: sum root's direct children, print recursively with indentation and padding, aggregate non-root times by name in lexical order, then append interpolation statistics | `printNode` and `Print`; `TestProfilerNestedTotalsAndInterpolation` checks tree, totals and appended interpolation report. Go sorts children lexically while the C++ child map orders pointer keys; totals are lexical in both. `TestProfilerSortedOutputShowTimeAndWriterFailure` covers output order and writer failure. The source's temporary output pointer is replaced by a borrowed writer and errors are propagated. |
| `show_time` and source `current_time`/`output_time` included from `src/interp/iz3profiling.cpp` | `ShowTime` writes time since profiler construction. The included source clock starts its static stopwatch on first use, not necessarily profiler construction; Go uses monotonic elapsed time. The existing manifest's characterization as a process clock is inaccurate: the retained source calls `stopwatch::get_current_seconds`. No source caller of Duality `show_time` was found. |
| `_WINDOWS` compiler-warning pragmas | Compiler-specific only; no Go behavioral branch. |

The default Duality timer tree and interpolation profiler now have process lifetime, and the interpolation owner is shared with the API and SMT-LIB command paths. Explicitly supplied profilers remain independent. `Print` borrows the interpolation profiler and writer; `SetProfileOutput` borrows the optional destination. Cross-backend and cross-client tests cover accumulated samples after context closure, report failure and retry. Timing origin and the other source timer sites below remain separate fidelity questions.

## Retained consumers and open gaps

`src/muz/duality/duality_dl_interface.cpp:341` prints after successful `Solve` when `duality.profile` is set; exceptions bypass the report. `pkg/muz/duality/query_engine.go` similarly prints after successful solve and before result publication, and propagates writer errors. `TestDualityBackendProfileOutputAndWriterRetry` covers disabled/enabled reporting, error and retry. `duality_solver.cpp` has only a commented additional print call. Searches across `src` found no other `print_profile` or `show_time` call.

The timer consumers are in `src/duality/duality_solver.cpp` and `src/duality/duality_rpfp.cpp`. Go wraps many corresponding sites using `startProfile`/`profileScope` and `Engine.Solve` installs the active profiler for the solve duration (`pkg/duality/engine.go`, `profile_scope.go`). `TestDualityOwnedBackendIntegerQueriesAndBorrowedServiceLifetime` checks representative feasibility, derivation and stratified labels, while `pkg/duality/engine_test.go` checks scope restoration on failures. These tests do not establish coverage of all source labels or phases.

Concrete remaining report-fidelity obligations:

- Source `Covering::Contains` and `Candidate` have timers (`duality_solver.cpp:2821-2831`); the corresponding Go methods now use the same labels. `TestCoveringDescendantClosureAndCandidateFiltering` checks accumulated timing, nested scope restoration, error handling and membership behavior. Other source timer scopes remain open below.
- Source `RPFP::GetEdgeFormula` times `ReducedDualEdge`, `getting children`, and `Persisting`, and `AssertEdge` times `solver add` (`duality_rpfp.cpp:919-966`). Go records these labels; `TestRPFPEdgeAssertionsPersistenceAndUnderapproximation` covers uncached and cached construction, cached assertion and the true-edge shortcut. The source `SubtermTruth` and `UnderapproxFormula` call-site timers are also present, with branch, error and allocation-failure scope tests. Reachability of each source site under every engine option remains a separate solver audit.
- Go lends the profiler during `Engine.Solve` and post-solve derivation, certificate and proof output. The latter can call `Localize`; a later-query report test checks the resulting timer. `RedVars` timing is scoped to the source `ReducedDualEdge` call site instead of every invocation. `SolveMain` now stops before quantified-counterexample validation, with a regression checking that validation's `Localize` is a root timer. The fixedpoint `FromClauses` path does not itself call the previously suspected construction timer sites. Other external call paths remain open.
- Source keys child nodes by pointer identity, but Go uses string contents. Distinct addresses containing equal text can yield separate source tree children and merge in Go; the source's text-keyed totals then combine them. Whether retained literal addresses ever differ in a relevant build is compiler-dependent. Sorting by text also deliberately changes sibling order. Go scopes use deferred cleanup on error, while source can lose stops and later recover by climbing, so failure-path timing can differ.

These are diagnostic and lifetime fidelity gaps, not evidence of a changed solver answer. The profiling unit implements its basic public operations, but its former `ported` classification and existing tests did not establish complete source-consumer report equivalence; it is now classified partial. Wider solver branches and interpolation statistics require their own audits.

## Focused verification

From `pkg`, both passed on 2026-11-03:

- `go test ./duality -run 'TestProfiler(NestedTotalsAndInterpolation|ImbalancedStopRecoversLikeSource|SortedOutputShowTimeAndWriterFailure)$' -count=1`
- `go test ./muz/duality -run 'TestDualityBackendProfileOutputAndWriterRetry|TestDualityOwnedBackendIntegerQueriesAndBorrowedServiceLifetime' -count=1`
