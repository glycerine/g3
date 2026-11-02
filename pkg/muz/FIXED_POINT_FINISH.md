# Fixedpoint port completion checklist

This tracks completion of the whole `src/muz/` port in `pkg/muz/`, including live dependencies outside that directory. A working engine or green test is not a whole-source completion claim. Source algorithms, proofs, models, incremental behavior, diagnostics and explicit ownership must all be accounted for.

Last updated: 2026-11-02. Work is active. No engine is declared fully audited by this document.

## How to follow progress

Each completed item needs source paths, implementation/test evidence and a commit in the progress log. Unchecked means incomplete or unproven. Preserve existing encodings, safeguards and test coverage. Exclude dead source only after checking live consumers. Keep commits small and frequent; use focused native tests. The user runs checked/debug tests separately. Run the full native suite sparingly, roughly hourly.

## 1. Inventory and baseline

- [x] Enumerate every current partial or unclassified muz unit below from `port_manifest.json`.
- [x] Establish a native full-suite baseline: `go -C pkg test ./... -short -timeout=0` exited 0; log `/tmp/g3-native-port-audit.log`. This run began before the latest changes; those received focused checks separately.
- [ ] Reconcile stale manifest descriptions with current code, especially Duality; preserve partial status until independently justified.
- [ ] Inspect every source conditional, function and live consumer for each partial unit; turn each remaining obligation into an explicit implementation or audit item.
- [ ] Verify previously marked ported units against their actual consumers and proof/ownership requirements; status labels alone are insufficient.
- [ ] Classify remaining CMake/README entries through their actual Go build/documentation replacements.

## 2. Shared fixedpoint infrastructure

- [ ] Close context/rule ownership, scope rollback, query transformation and engine lifecycle audits (`base/dl_context`, `dl_rule`, `dl_engine_base`).
- [ ] Verify parameter defaults, aliases, updates and engine selection against `fixedpoint_params.hpp` and engine registration.
- [ ] Close SMT-LIB/Datalog command parsing, error reporting, query state and exported result/proof/model contracts.
- [ ] Test cancellation, resource refusal and partial construction/destruction at actual allocation boundaries; preserve errors through cleanup.

## 3. PDR — first engine closure

- [ ] Audit remaining `pdr_util.cpp` methods against live consumers; implement needed normalization/diagnostic paths and document genuinely unused helpers.
  - [x] Verify focused evidence for model-satisfied disequality reduction and source swap-removal/reprocessing order.
  - [x] Verify difference/UTVPI classification, backend selection and rejection of out-of-fragment proof lemmas.
  - [x] Verify exact arithmetic normalization, owning publication, cancellation retry and disposal under a refused allocation budget.
  - [ ] Close interrupted-decrement ownership coverage in `arith_normalize.go`, including any unconsumed roots and explicit cleanup continuation.
  - [ ] Confirm the full caller graph for `ceil_log2` and cube formatting; exclude only genuinely unused helpers or dormant self-tests.
  - [ ] Reconcile method-level audit evidence and mark the unit complete only after the remaining checks are satisfied.
- [ ] Confirm method and ownership evidence for all PDR units, including arithmetic fragment selection and proof-derived lemmas.
- [ ] Verify recursive/nonlinear SAT and UNSAT, assumptions/background, repeated queries, witnesses, proofs and original-rule traces at the public boundary.
- [ ] Close all PDR source obligations before declaring the engine complete.

## 4. Spacer — second engine closure

- [ ] Close `spacer_context` reach-fact selection, predecessor tags, obligation search and ownership/exception audits.
- [ ] Close `spacer_util` model evaluation, QE/MBP, array auxiliaries, appended datatype literals and source diagnostics.
- [ ] Finish slice-proof conversion for every live proof shape. Sparse/trailing/vacuous/mixed-sort binders and reverse binary resolution now have evidence, but general multi-premise mixed orientations remain unproven.
- [ ] Verify coalesce/unfold/slice composition preserves original asserted leaves, substitutions, resolution positions and checker-valid proofs.
- [ ] Close exported model, answer, counterexample and trace behavior through `spacer_dl_interface`.
- [ ] Verify exceptional cleanup and parameter restoration in projection and preprocessing.
- [ ] Close all Spacer source obligations before declaring the engine complete.

## 5. Duality — engine and external source dependency closure

- [ ] Reconcile the stale inventory for `src/duality/` and `src/muz/duality/` with current implementation and tests.
  - [x] Correct the stale muz Duality interface descriptions; keep both units partial.
  - [ ] Reconcile the external `src/duality/` descriptions and method-level audit records.
- [ ] Audit RPFP graph/vocabulary ownership, nodes/edges, clone/delete, assertions, dual formulas and incremental solver scopes.
- [ ] Audit lazy-annotation expansion, heuristic ordering, covering, backtracking, restarts and constrained counterexample completion.
- [ ] Audit interpolation learning, generalization, proof cores, underapproximation, Gaussian elimination and integer/array projection.
- [ ] Preserve independent cleanup/resource errors through recovery and restart paths; extend actual-path failure campaigns where coverage is missing.
- [ ] Close fixedpoint registration, preprocessing/model conversion, quantified witness reconstruction, proofs and certificate export.
- [ ] Test recursive, mutually recursive and nonlinear SAT/UNSAT plus original-clause proof/certificate validity.
- [ ] Close wrapper numerical ownership, diagnostics, profiling and all live conditional branches.
- [ ] Close all Duality source obligations before declaring the engine complete.

## 6. Remaining muz components — required for whole-port completion

- [ ] Finish BMC engine construction, bounded/quantified search, source solver selection, proofs, interfaces and ownership audits.
- [ ] Finish CLP and TAB contexts and their public engine contracts.
- [ ] Finish relational engine/context and all partial table/relation plugins listed below, including joins, projections, complements, conversions and lifecycle paths.
- [ ] Audit explanations, similarity compression, relation-manager dispatch, external relations and checking wrappers against actual consumers.
- [ ] Audit remaining transforms, dataflow and DDNF consumers, including interactions with engine-specific preprocessing.

## 7. Acceptance gates

- [ ] Every live source unit is implemented or has an explicitly justified Go/platform replacement; no unproven partial production unit remains.
- [ ] Every implemented algorithm has meaningful focused tests; add independent semantic oracles where source tests are insufficient.
- [ ] Public fixedpoint/API/SMT-LIB integration tests cover SAT, UNSAT, unknown and exception/resource distinctions for each applicable engine.
- [ ] Proofs are independently checked and their side conditions discharged; model/answer formulas are checked against original rules.
- [ ] Incremental queries, resets, engine switches, background assumptions and cancellation/resource recovery have regression coverage.
- [ ] Final source/ownership audit and manifest agree with implementation and documented deviations.
- [ ] Final native suite passes on the integrated final state; record the exact command and revision. Incorporate the user's tagged-suite findings before final completion.

## Remaining unit ledger

These are inventory obligations, not claims that the whole unit is missing. For each row: inspect code, identify live gaps, implement them, verify semantics and ownership, then update the manifest and this ledger. Paired headers share the audit but require their own interface review.

### README

- [ ] `src/muz/README` (unported)

### base

- [ ] `src/muz/base/CMakeLists.txt` (unported)
- [ ] `src/muz/base/dl_context.cpp` (partial), `src/muz/base/dl_context.h` (partial)
- [ ] `src/muz/base/dl_engine_base.h` (partial)
- [ ] `src/muz/base/dl_rule.cpp` (partial), `src/muz/base/dl_rule.h` (partial)
- [ ] `src/muz/base/fixedpoint_params.hpp` (partial)

### bmc

- [ ] `src/muz/bmc/CMakeLists.txt` (unported)
- [ ] `src/muz/bmc/dl_bmc_engine.cpp` (partial), `src/muz/bmc/dl_bmc_engine.h` (partial)

### clp

- [ ] `src/muz/clp/CMakeLists.txt` (unported)
- [ ] `src/muz/clp/clp_context.cpp` (partial), `src/muz/clp/clp_context.h` (partial)

### dataflow

- [ ] `src/muz/dataflow/CMakeLists.txt` (unported)

### ddnf

- [ ] `src/muz/ddnf/CMakeLists.txt` (unported)

### duality

- [ ] `src/muz/duality/CMakeLists.txt` (unported)
- [ ] `src/muz/duality/duality_dl_interface.cpp` (partial), `src/muz/duality/duality_dl_interface.h` (partial)

### fp

- [ ] `src/muz/fp/CMakeLists.txt` (unported)
- [ ] `src/muz/fp/datalog_parser.cpp` (partial), `src/muz/fp/datalog_parser.h` (partial)
- [ ] `src/muz/fp/dl_cmds.cpp` (partial), `src/muz/fp/dl_cmds.h` (partial)
- [ ] `src/muz/fp/dl_register_engine.cpp` (partial), `src/muz/fp/dl_register_engine.h` (partial)

### pdr

- [ ] `src/muz/pdr/pdr_util.cpp` (partial)

### rel

- [ ] `src/muz/rel/CMakeLists.txt` (unported)
- [ ] `src/muz/rel/check_relation.cpp` (partial), `src/muz/rel/check_relation.h` (partial)
- [ ] `src/muz/rel/dl_base.cpp` (partial), `src/muz/rel/dl_base.h` (partial)
- [ ] `src/muz/rel/dl_bound_relation.cpp` (partial), `src/muz/rel/dl_bound_relation.h` (partial)
- [ ] `src/muz/rel/dl_check_table.cpp` (partial), `src/muz/rel/dl_check_table.h` (partial)
- [ ] `src/muz/rel/dl_external_relation.cpp` (partial), `src/muz/rel/dl_external_relation.h` (partial)
- [ ] `src/muz/rel/dl_finite_product_relation.cpp` (partial), `src/muz/rel/dl_finite_product_relation.h` (partial)
- [ ] `src/muz/rel/dl_interval_relation.cpp` (partial), `src/muz/rel/dl_interval_relation.h` (partial)
- [ ] `src/muz/rel/dl_lazy_table.cpp` (partial), `src/muz/rel/dl_lazy_table.h` (partial)
- [ ] `src/muz/rel/dl_mk_explanations.cpp` (partial), `src/muz/rel/dl_mk_explanations.h` (partial)
- [ ] `src/muz/rel/dl_mk_similarity_compressor.cpp` (partial), `src/muz/rel/dl_mk_similarity_compressor.h` (partial)
- [ ] `src/muz/rel/dl_product_relation.cpp` (partial), `src/muz/rel/dl_product_relation.h` (partial)
- [ ] `src/muz/rel/dl_relation_manager.h` (partial)
- [ ] `src/muz/rel/dl_sieve_relation.cpp` (partial), `src/muz/rel/dl_sieve_relation.h` (partial)
- [ ] `src/muz/rel/dl_sparse_table.cpp` (partial), `src/muz/rel/dl_sparse_table.h` (partial)
- [ ] `src/muz/rel/dl_vector_relation.h` (partial)
- [ ] `src/muz/rel/rel_context.cpp` (partial), `src/muz/rel/rel_context.h` (partial)
- [ ] `src/muz/rel/udoc_relation.cpp` (partial), `src/muz/rel/udoc_relation.h` (partial)

### spacer

- [ ] `src/muz/spacer/spacer_context.cpp` (partial), `src/muz/spacer/spacer_context.h` (partial)
- [ ] `src/muz/spacer/spacer_dl_interface.cpp` (partial), `src/muz/spacer/spacer_dl_interface.h` (partial)
- [ ] `src/muz/spacer/spacer_util.cpp` (partial), `src/muz/spacer/spacer_util.h` (partial)

### tab

- [ ] `src/muz/tab/CMakeLists.txt` (unported)
- [ ] `src/muz/tab/tab_context.cpp` (partial), `src/muz/tab/tab_context.h` (partial)

### transforms

- [ ] `src/muz/transforms/CMakeLists.txt` (unported)
- [ ] `src/muz/transforms/dl_mk_slice.cpp` (partial)

## Progress log

- 2026-11-02 — Created the complete component/unit checklist from the current inventory. Recent proof work includes sparse/trailing binder remapping and reverse binary resolution; all remain part of a partial slice-converter audit. Native PDR, Spacer, fixedpoint, Duality and transforms checks have passed in recent focused runs; the full native baseline exited 0. Current next action: reconcile stale Duality inventory entries, then continue concrete source-gap closure in the priority order above.

- 2026-11-02 — Corrected the muz Duality interface inventory descriptions: registration, query execution and proof/certificate export are implemented; full source and ownership closure remains partial. Focused public integration verification is recorded with the commit.
  Validation: `go -C pkg test ./parsers/smt2 -run '^TestSMT2Duality(MutualRecursionIntegerUnderapproximation|SafetyCertificate|ReachableCertificate)$' -count=1` and `go -C pkg test ./portcheck -short` both passed. Commit title: `Create fixedpoint completion checklist and reconcile Duality status`.

- 2026-11-02 — PDR utility checklist decomposed into verified algorithm evidence and explicit remaining ownership/consumer obligations. The prior checklist/inventory commit is `763135367`. Current evidence covers disequality ordering, fragment classification/backend gates, exact normalization, cancellation/retry and no-allocation temporary disposal; interrupted decrement closure remains unchecked. Source targets: `src/muz/pdr/pdr_util.cpp`, `pkg/muz/pdr/pdr_util.go`, `fragment.go`, `arith_normalize.go` and their focused tests.
  Validation: `go -C pkg test ./muz/pdr -run 'Test(PDRDisequality|PDRDifference|PDRFragment|PDRContextSelects|PropSolverRejects|NormalizeArithmetic|ArithmeticNormalization|PDRArithmeticNormalization)' -short -count=1` passed. Commit title: `Record verified PDR utility evidence and remaining obligations`.
