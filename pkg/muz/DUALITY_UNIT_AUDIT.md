# Duality profiling unit audit

Scope: `src/duality/duality_profiling.cpp` and `.h`, marked ported in `port_manifest.json` before this audit, plus their retained consumers. This is a source and consumer audit, not a claim that the Duality solver or interpolation profiler is complete. Reviewed 2026-11-03; no manifest status is changed here.

## Source surface and ownership

| Source | Go ownership and evidence |
| --- | --- |
| Header guard and four declarations: `timer_start`, `timer_stop`, `print_profile`, `show_time` | `pkg/duality/profiling.go`: `TimerStart`, `TimerStop`, `Print`, `ShowTime`. The header guard has no Go runtime counterpart. |
| Global `top`, `current`, initializer and node constructor | `NewProfiler` owns a root and current pointer; node children, elapsed time, start and parent live in the profiler tree. Unlike the process-global source, separate Go profiler instances do not share accumulated data. |
| `timer_start`: look up child by `const char*` identity, initialize name and parent when empty, overwrite start time, descend | `TimerStart` uses string-value keys, preserves an existing child's elapsed time, resets its start and descends. Nil and empty names return an error; all identified retained source sites pass nonempty literals. Source pointer identity can distinguish equal text at different addresses; Go merges equal names under a parent. |
| `timer_stop`: on mismatch climb to matching ancestor, charge it if not root, otherwise do nothing; normal stop charges and ascends | `TimerStop` implements this recovery; `TestProfilerImbalancedStopRecoversLikeSource` covers a lost child and missing name. The `#if 0` fatal mismatch diagnostic is disabled and has no live counterpart. |
| `print_node`, `time_entry`, `ltstr`, `print_profile`: sum root's direct children, print recursively with indentation and padding, aggregate non-root times by name in lexical order, then append interpolation statistics | `printNode` and `Print`; `TestProfilerNestedTotalsAndInterpolation` checks tree, totals and appended interpolation report. Go sorts children lexically while the C++ child map orders pointer keys; totals are lexical in both. `TestProfilerSortedOutputShowTimeAndWriterFailure` covers output order and writer failure. The source's temporary output pointer is replaced by a borrowed writer and errors are propagated. |
| `show_time` and source `current_time`/`output_time` included from `src/interp/iz3profiling.cpp` | `ShowTime` writes time since profiler construction. The included source clock starts its static stopwatch on first use, not necessarily profiler construction; Go uses monotonic elapsed time. The existing manifest's characterization as a process clock is inaccurate: the retained source calls `stopwatch::get_current_seconds`. No source caller of Duality `show_time` was found. |
| `_WINDOWS` compiler-warning pragmas | Compiler-specific only; no Go behavioral branch. |

The interpolation profiler is a separate owned object (`pkg/interp/profiling.go`) passed to `NewProfiler`, or created when nil. The backend creates the two together in `pkg/muz/duality/backend.go`; `NewQueryEngine` also creates a pair when no graph profiler is provided. `Print` borrows the interpolation profiler, not its writer. `SetProfileOutput` borrows the optional report destination; query defaults to stdout. The source's global timer tree and interpolation data accumulate across calls; Go accumulation is per profiler/backend lifetime. This is an observable cross-instance and timing-origin adaptation, not exact process-global behavior.

## Retained consumers and open gaps

`src/muz/duality/duality_dl_interface.cpp:341` prints after successful `Solve` when `duality.profile` is set; exceptions bypass the report. `pkg/muz/duality/query_engine.go` similarly prints after successful solve and before result publication, and propagates writer errors. `TestDualityBackendProfileOutputAndWriterRetry` covers disabled/enabled reporting, error and retry. `duality_solver.cpp` has only a commented additional print call. Searches across `src` found no other `print_profile` or `show_time` call.

The timer consumers are in `src/duality/duality_solver.cpp` and `src/duality/duality_rpfp.cpp`. Go wraps many corresponding sites using `startProfile`/`profileScope` and `Engine.Solve` installs the active profiler for the solve duration (`pkg/duality/engine.go`, `profile_scope.go`). `TestDualityOwnedBackendIntegerQueriesAndBorrowedServiceLifetime` checks representative feasibility, derivation and stratified labels, while `pkg/duality/engine_test.go` checks scope restoration on failures. These tests do not establish coverage of all source labels or phases.

Concrete remaining report-fidelity obligations:

- Source `Covering::Contains` and `Candidate` have timers (`duality_solver.cpp:2821-2831`); the corresponding Go methods now use the same labels. `TestCoveringDescendantClosureAndCandidateFiltering` checks accumulated timing, nested scope restoration, error handling and membership behavior. Other source timer scopes remain open below.
- Source `RPFP::GetEdgeFormula` times `ReducedDualEdge`, `getting children`, and `Persisting`, and `AssertEdge` times `solver add` (`duality_rpfp.cpp:919-966`). Corresponding graph work exists in `pkg/duality/rpfp_assert_edge.go` and `rpfp_localize.go`, but these labels have no Go timer scopes. Source `SubtermTruth` and `UnderapproxFormula` timer sites likewise have no matching scopes in `pkg/duality/subterm_truth.go` and `implicant.go`. Reachability of each source site under every engine option remains a separate solver audit; do not interpret absence in one test as dead code.
- Go's context only lends the profiler during `Engine.Solve`; source timer calls outside that scope still accumulate globally. In particular, source `SetEdgeMaps`, `Localize`, and `RedVars` are instrumented in the graph construction routines, while the Go wrappers silently do nothing when `activeProfiler` is nil. The active consumer must be traced per construction path before claiming equivalent report coverage.
- Source keys child nodes by pointer identity, but Go uses string contents. Distinct addresses containing equal text can yield separate source tree children and merge in Go; the source's text-keyed totals then combine them. Whether retained literal addresses ever differ in a relevant build is compiler-dependent. Sorting by text also deliberately changes sibling order. Go scopes use deferred cleanup on error, while source can lose stops and later recover by climbing, so failure-path timing can differ.

These are diagnostic and lifetime fidelity gaps, not evidence of a changed solver answer. The profiling unit implements its basic public operations, but its former `ported` classification and existing tests did not establish complete source-consumer report equivalence; it is now classified partial. Wider solver branches and interpolation statistics require their own audits.

## Focused verification

From `pkg`, both passed on 2026-11-03:

- `go test ./duality -run 'TestProfiler(NestedTotalsAndInterpolation|ImbalancedStopRecoversLikeSource|SortedOutputShowTimeAndWriterFailure)$' -count=1`
- `go test ./muz/duality -run 'TestDualityBackendProfileOutputAndWriterRetry|TestDualityOwnedBackendIntegerQueriesAndBorrowedServiceLifetime' -count=1`
