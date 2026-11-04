# Base unit audit: cost accounting

Scope: `src/muz/base/dl_costs.h` and `dl_costs.cpp`, both marked ported in `port_manifest.json`. This is an audit of this unit, not a claim that every compiler, transform, profiling or cleanup consumer is complete. The separate engine-base audit covers `dl_engine_base.h`. Source consumer searches below cover `src/muz`; Go searches cover `pkg/muz`.

## Costs and recorder

| Source declaration / branch | Go mapping and evidence |
| --- | --- |
| `costs` constructor, `empty`, `reset` | Zero-value `Costs`, `Empty`, `Reset` in `pkg/muz/base/costs.go`. |
| subtraction, both strict assertions; addition | `Subtract` rejects either non-strict operand with `ErrCosts`; `Add` uses uint32 arithmetic. The strict checks model debug assertions as unconditional errors, an intentional API difference from a release C++ build. `TestCostsArithmeticThresholdAndOutput` covers subtraction including equality. |
| threshold and output | `PassesThreshold` uses >= milliseconds, ignoring instructions; `Output` formats `instr: N  time: Nms` and returns writer errors. Same test covers threshold boundary and format. |
| recorder construction/destruction, `start`, `finish` | `NewCostRecorder`, `Start`, `Finish`, `Close` sample monotonic elapsed time and charge the outgoing object once, clear its flag, set incoming flag; closing finishes the last interval. Go validates closed/duplicate-recording objects and clock reversal; source has no equivalent error API. Source `m_running` is set but never read. `TestCostRecorderSwitchesObjectsAndChargesWholeMilliseconds` covers switching, final close and flags. |
| sample rounding | Source converts each absolute sample to whole milliseconds before subtraction. The Go recorder now does the same; `TestCostRecorderTruncatesEachSampleBeforeSubtraction` covers 1.9ms to 2.1ms (one millisecond charged), and the switch test covers 2.5ms to 7ms (five charged). Earlier Go behavior rounded the interval instead. |

`time_type` and instruction counters are source `unsigned`, represented as uint32. The elapsed stopwatch delta is narrowed to uint32, matching the source's narrowing on its normal 32-bit unsigned targets. Go's injected clock rejects negative deltas; the source stopwatch supplies nonnegative elapsed samples. No independent test of overflow or stopwatch failure is claimed.

## Accounted object and ownership

| Source declaration / branch | Go mapping and evidence |
| --- | --- |
| constructor, destructor, parent setter/getter | `NewAccountedObject`, `SetAccountingParentRule`, `ParentRule`, `Close`. Go retains the rule and releases an old parent before publishing replacement, and refuses closing while recording. Source holds a context pointer and decrements the parent via its rule manager. Go rejects nil/self/closed parents and may return cleanup errors; source's context identity assertion is not represented as a separate argument. `TestAccountedObjectPropagatesToRetainedRuleParent` verifies retained lifetime and final release; `rule_pending_test.go` covers pending cleanup. Parent replacement error and cross-manager behavior are not independently proven here. |
| current/processed accessors and `being_recorded` | `CurrentCosts`, `ProcessedCosts`, `BeingRecorded`; Go exposes processed values by copy and rejects access to closed owners. |
| `process_costs` empty early return and nonempty propagation loop | `ProcessCosts` copies and clears current, then adds the delta to this object and each parent. Parent propagation and retained rule lifetime are checked by `TestAccountedObjectPropagatesToRetainedRuleParent` and `TestRuleUnfold...` in `pkg/muz/transforms/rule_unfold_test.go`; empty path and chains longer than one parent are source-reviewed but not isolated by these tests. |
| total, output threshold and profile | `TotalCosts`, `PassesOutputThreshold`, `OutputProfile` sum current and processed, then delegate to `Costs`. `rule_set_display.go` applies the threshold; `rule_display.go` writes profiles. Threshold parameters are passed explicitly rather than obtained from a context. |

## Source consumers

* `dl_instruction.cpp` records every attempted instruction in `instruction_block::perform`, including a termination check, and processes nested blocks; its profile display includes instructions above threshold or currently being recorded. Go `pkg/muz/rel/instruction.go` creates and closes a recorder around the block, starts before termination and performs recursive `ProcessAllCosts` through instruction implementations. Compiler parent assignment is in `pkg/muz/rel/compiler.go` and `compiler_explanation_rule.go`. Source `rel_context.cpp` processes costs on its corresponding lifecycle paths; integration/profile fidelity across every partial relational execution path, nested display and failure branch has not been established by this unit audit.
* `dl_rule.cpp` sets parent when rebuilding rules and conditionally displays profiles; `dl_rule_set.cpp` filters rule display on threshold. Go `rule_unbound.go`, `rule_display.go`, and `rule_set_display.go` implement these respective paths. Their broader source units remain independently subject to audit.
* Source parent-setting transform consumers are unbound compressor (two call sites), interpreted-tail simplifier (two), subsumption checker, filter rules (two), rule inliner, magic sets, similarity compressor, and simple joins (two). Go now sets source-rule parents in both interpreted-tail construction paths, subsumption replacement, and filter definition/replacement; the inliner already uses the shared rule unifier. Focused tests charge transformed rules and verify propagation to their originals, including a shared filter attributed to its first creator. Other attribution and exceptional ownership paths remain open.
* `costs::operator-` has no direct consumer found in `src/muz` outside its definition, and Go `Subtract` is exercised by its unit test. Do not infer consumers outside the searched source tree are absent.

## Limits and verification

This audit maps every declaration and conditional in the two cost files and identifies the source consumer families, but does not certify the independently partial callers, every possible exceptional cleanup continuation, clock wrap, or end-to-end profile attribution for all transforms. Nested profile display and further parent-link failure boundaries remain open. No manifest status is changed by this document.

Focused verification from `pkg`: `go test ./muz/base -run 'Test(Cost|AccountedObject)' -count=1` and `go test ./muz/rel -run 'Test.*Instruction' -count=1`.
