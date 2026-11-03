# Fixedpoint engine registration audit

Audited on 2026-11-03 after the engine-base closure and `f07739846`. This document
supports completion of [dl_register_engine.h](../../src/muz/fp/dl_register_engine.h)
and [dl_register_engine.cpp](../../src/muz/fp/dl_register_engine.cpp) only. Every
declaration, branch, field, constructor and ownership transfer was checked against
actual callers. Independent review found no missing factory behavior. No new
production adapter or test was needed: the existing concrete consumers cover the
complete factory surface.

The previous manifest description was stale: Duality is available, all ten engine
names have concrete dispatch, and QBMC is no longer an unavailable branch. Wider
proof/search completeness is a derived-engine obligation, not a missing factory
case. [ENGINE_BASE_AUDIT.md](ENGINE_BASE_AUDIT.md) records the separate completed
base contract.

## Complete declaration and branch map

| Source | Contract and Go mapping |
| --- | --- |
| Header 19–22, 36 | Include guard and context dependency become Go packages/imports. No platform conditional or STL implementation is present. |
| Header 26–27 | `register_engine` contains only a borrowed `context* m_ctx`. Go combines context association and concrete dispatch in `PropositionalEngine`; its manager and parameter services are borrowed. There is no separately owned factory object. |
| Header 29; C++ 30 | Constructor initializes `m_ctx` to null. Live source contexts bind it before use. Go constructors take the association directly and reject invalid inputs rather than exposing an unbound factory phase. |
| Header 31 | `set_context` assigns the borrowed pointer without allocation or ownership transfer. `NewPropositionalEngineWithServices` establishes the equivalent association; separate public fixedpoint objects own separate command/engine state. |
| Header 30; C++ 32–58 | `mk_engine` dispatches every source kind as detailed below. Go uses [fp/register_engine.go](fp/register_engine.go), [fp/engine_kind.go](fp/engine_kind.go) and the concrete query/property entry points. |
| C++ 52–57 | `LAST_ENGINE` and out-of-range enum values reach `UNREACHABLE` and return null. They are not live valid-engine branches. Go exposes no selectable sentinel and reports an error for an invalid explicit name; rejected selection preserves the current choice. |

| Source factory case | Actual Go constructor/consumer path | Existing direct evidence |
| --- | --- | --- |
| C++ 34–36: PDR and QPDR allocate the same `pdr::dl_interface` | `pdrContext` / `newConfiguredPDRContext` in [fp/pdr_engine.go](fp/pdr_engine.go); `e.backend` preserves the PDR/QPDR distinction. | `TestPDRAndQPDRSwitchInvalidatesResult`; configured expression and predicate dispatch tests. |
| C++ 37–38: Spacer | `spacerContext` in [fp/spacer_rule_lifecycle.go](fp/spacer_rule_lifecycle.go) and actual query construction in [fp/spacer_engine.go](fp/spacer_engine.go). | Configured expression/predicate dispatch; `TestSpacerExpressionConstructionFailureWithoutPrequeryOwner`. |
| C++ 39–40: Datalog | The concrete rule/relation owners created by `NewPropositionalEngine`; `queryDatalogPredicates` and concrete relation dispatch in [fp/propositional_engine.go](fp/propositional_engine.go). | Configured dispatch, independent API state and engine round-trip invalidation tests. |
| C++ 41–43: BMC and QBMC allocate the same `bmc` class | Shared `queryBMCPredicate` path in [fp/bmc_engine.go](fp/bmc_engine.go), with selected backend mode carried into compilation/search. | `TestBMCEngineRuleTraceAndSnapshot`, `TestQBMCEngineRuleTraceAndSnapshot`; default model tests query both concrete modes. |
| C++ 44–45: TAB | `queryTab` in [fp/tab_engine.go](fp/tab_engine.go), its persistent session owner and per-query `tabBackendAttempt`. | `TestTabEngineQueryProofAndSnapshots`; default model lifetime test. |
| C++ 46–47: CLP | `queryCLPPredicate` in [fp/clp_engine.go](fp/clp_engine.go), with concrete solver/engine owned by `clpBackendAttempt`. | `TestCLPEngineResultSnapshotAndAnswer`; default model lifetime test. |
| C++ 48–49: Duality | `queryDuality` in [fp/duality_engine.go](fp/duality_engine.go) creates `duality.NewBackend` with the actual manager/services and stores the persistent backend in its dispatcher owner. | `TestDualityFixedpointDispatchProofModelScopesAndSwitching`; independent API parameter/engine selection. |
| C++ 50–51: DDNF | [fp/ddnf_engine.go](fp/ddnf_engine.go) owns the DDNF registry/trail and invokes the concrete preprocessor/compiler. Source execution remains disabled, so successful queries return unknown. | `TestDDNFQueryDumpsCompiledRulesAndReturnsUnknown`; default model lifetime test includes successful unknown snapshots. |

Automatic engine classification and configured-name fallback are implemented in
[fp/auto_engine.go](fp/auto_engine.go) and the configured-selection path. Those
algorithms originate in `dl_context.cpp`, not this factory switch. Their tests
verify that the factory is reached with the selected kind, without claiming that
this audit completes the surrounding context implementation.

## Ownership and live callers

The source factory owns neither the bound context nor the returned engine.
`context::context` binds the pointer at `dl_context.cpp:234`.
`context::ensure_engine` at lines 922–932 assigns the newly allocated engine
directly into the owning `m_engine` scoped pointer, then updates its parameters.
There are no temporary owners, callbacks, caches or later fallible steps inside
`mk_engine` itself. Constructor internals and the enclosing context's parameter
update/lifecycle remain separate source units.

Go's fused dispatcher is allocated once for its caller; concrete backends are
created when their query/property path needs them. Persistent fields, query
clones and concrete attempt owners hold those backends. Selection does not create
an extra owner that could be lost. `selectBackend` invalidates stale result state
when switching and uses existing concrete cleanup paths; repeating a selection
does not create another factory instance. This mapping is not a claim that every
derived solver's construction/cleanup algorithm has been audited here.

| Actual source caller | Live Go association and owner |
| --- | --- |
| `src/muz/fp/dl_cmds.cpp:36–76`: command wrapper owns a factory and scoped context | [../cmd_context/fixedpoint.go](../cmd_context/fixedpoint.go) `fixedpointContext` constructs `NewPropositionalEngineWithServices` and publishes `c.fixedpoint` only after success. The command context owns it. |
| `src/api/api_datalog.cpp:38–53`: fixedpoint API object owns factory/context members | [../api/api_fixedpoint_object.go](../api/api_fixedpoint_object.go) `NewFixedpoint` gives each object its own parser/command context, whose fixedpoint dispatcher is created through the preceding path. Independent state and local parameter tests exercise two objects sharing one AST manager. |
| `src/muz/fp/horn_tactic.cpp:35–47`: tactic implementation owns factory/context | [fp/horn_tactic.go](fp/horn_tactic.go) creates a concrete dispatcher per application and owns it through deferred cleanup. Both finite Datalog and infinite-domain PDR tactic consumers are tested. |
| `src/shell/datalog_frontend.cpp:121–132`: stack factory precedes stack context | Public Go frontends use the command-context owner above. The source factory has no shell-specific branch; completion of the separate shell frontend is not implied. |
| `src/muz/transforms/dl_mk_karr_invariants.cpp:46–59`: inner context reuses the source factory | The source inner context rebinds the borrowed factory pointer. [transforms/karr_invariants.go](transforms/karr_invariants.go) uses concrete typed Karr relation evaluation and rule owners, avoiding an otherwise unused mutable factory layer. The transformation algorithm remains a separate audit. |
| `src/muz/ddnf/ddnf.cpp:494–505`: inner context reuses the source factory | The source inner context likewise rebinds it. Go uses the DDNF registry/preprocessor/compiler directly. No independent factory state or ownership transfer is missing; DDNF algorithm completion is not claimed. |

## Exact verification evidence

- [fp/engine_result_routing_test.go](fp/engine_result_routing_test.go):
  `TestRetainedEngineKindSelectionAndDiagnostics`,
  `TestEngineSwitchDoesNotRepublishPriorBackendResult`,
  `TestPDRAndQPDRSwitchInvalidatesResult`,
  `TestDDNFQueryDumpsCompiledRulesAndReturnsUnknown`.
- [fp/configured_predicate_dispatch_test.go](fp/configured_predicate_dispatch_test.go),
  [fp/configured_expression_dispatch_test.go](fp/configured_expression_dispatch_test.go),
  [fp/configured_engine_fallback_test.go](fp/configured_engine_fallback_test.go):
  `TestConfiguredDirectPredicateQueryDispatch`,
  `TestConfiguredExpressionQueryDispatchBeforeFirstQuery`,
  `TestConfiguredUnknownEngineFallsBackToSourceAutoSelection`. These verify real
  selected queries, cached choice, local/module precedence and invalid explicit
  selection without mutating the prior choice.
- [fp/bmc_engine_test.go](fp/bmc_engine_test.go),
  [fp/tab_engine_test.go](fp/tab_engine_test.go),
  [fp/clp_engine_test.go](fp/clp_engine_test.go),
  [fp/duality_engine_test.go](fp/duality_engine_test.go): the named branch tests in
  the table exercise actual queries and result owners, not enum names alone.
- [fp/engine_default_model_test.go](fp/engine_default_model_test.go)
  `TestEngineDefaultModelStatusAndIndependentOwners` and
  [../api/api_fixedpoint_default_model_test.go](../api/api_fixedpoint_default_model_test.go)
  `TestAPIFixedpointLastDefaultModel` cover real inherited-engine queries, repeat
  use, separate exports and survival after dispatcher closure.
- [fp/spacer_expression_construction_failure_test.go](fp/spacer_expression_construction_failure_test.go)
  `TestSpacerExpressionConstructionFailureWithoutPrequeryOwner` exercises actual
  construction failure and retry before any pre-query backend owner exists.
- [../api/api_fixedpoint_object_isolation_test.go](../api/api_fixedpoint_object_isolation_test.go)
  `TestAPIFixedpointObjectsHaveIndependentState` and
  [../api/api_fixedpoint_object_parameters_test.go](../api/api_fixedpoint_object_parameters_test.go)
  `TestAPIFixedpointParametersAreOwnedAndLocal` verify the public context association.
- [fp/horn_tactic_infinite_test.go](fp/horn_tactic_infinite_test.go)
  `TestHornTacticSelectsPDRForInfiniteDomainGoal` and
  [fp/horn_tactic_datalog_proof_test.go](fp/horn_tactic_datalog_proof_test.go)
  `TestHornTacticDatalogUsesSourceDefaultProof` verify the tactic factory consumer.

The selected native branch/configuration/ownership gate used
`GOCACHE=/tmp/g3-go-build`, `-count=1`, `-timeout=90s` and the named tests above:
`muz/fp` passed in 11.517s and `api` in 0.056s. The separate public-isolation and
tactic-consumer gate passed in 0.011s / 0.023s. No broad suite was needed for this
read-only source-unit audit.

## Bounded Go adaptations

- One concrete typed dispatcher replaces a C++ virtual factory plus context
  pointer. Managers/services are bound at construction; no unused generic factory
  or mutable `set_context` adapter is introduced.
- Backend creation is deferred to actual query/property use. Invalid explicit
  names and invalid/closed Go owners return errors instead of entering source
  undefined/unreachable enum or null-context paths.
- Explicit Go engine switching is supported with snapshot invalidation and
  concrete cleanup. Source configured selection remains cached; its automatic
  classification belongs to `dl_context.cpp`.
- This completion covers only the two registration files. It does not promote
  the context, command/API wrappers, Karr/DDNF inner algorithms, or any derived
  engine implementation, and does not assert full proof/model/search parity.
