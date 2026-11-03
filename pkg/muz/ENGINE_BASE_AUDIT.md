# Fixedpoint engine base header audit

Audited on 2026-11-03 through `f07739846`. This is the closure evidence for
[`src/muz/base/dl_engine_base.h`](../../src/muz/base/dl_engine_base.h) alone.
Every enum value, field, method, default, conditional and owned local was checked
against the live Go dispatcher and its callers. The independent source reviewer
approved header-only completion after the fixes below. Derived engine algorithms
and the enclosing context/rule units keep their separate completion status.

## Complete source mapping

Source line numbers refer to `dl_engine_base.h`. Go paths below are relative to
`pkg/muz/`; method names identify the implementation independently of line shifts.
Evidence labels refer to the focused test catalogue below.

| Source lines and contract | Live Go implementation and branch evidence |
| --- | --- |
| 19–24, 120: include guard and dependencies | Go packages/imports replace the guard and includes. There is no platform-specific conditional or STL implementation to reproduce. |
| 26–38: ten `DL_ENGINE` values and `LAST_ENGINE` sentinel | [base/engine_kind.go](base/engine_kind.go) preserves all values and `EngineKindCount`. The production selection domain in [fp/engine_kind.go](fp/engine_kind.go) has all ten names; `SelectEngine` in [fp/register_engine.go](fp/register_engine.go) selects each concrete engine and rejects unknown names. The sentinel is not a selectable engine. E1. |
| 40–45: borrowed `ast_manager&`, stored name, constructor, empty virtual destructor | `NewPropositionalEngine` / `NewPropositionalEngineWithServices` in [fp/propositional_engine.go](fp/propositional_engine.go) associate the borrowed manager, backend name and parameter services. Concrete `Close` releases engine resources without closing the caller's manager. The source base destructor has no additional algorithm. E1–E3 exercise manager and exported-result lifetimes. |
| 47, 51: pure `get_answer` and `query(expr*)` | `Answer` / `ExpressionAnswer`, `Query` / `QueryExpression`, and concrete backend dispatch in [fp/propositional_engine.go](fp/propositional_engine.go) and [fp/query_expression.go](fp/query_expression.go). These satisfy the interface contract; they do not establish completion of each solver implementation. E1, E2, E6. |
| 48–50: unsupported `get_ground_sat_answer` default | `GroundSatAnswer` and `ExpressionGroundSatAnswer` admit the Spacer override; other engine kinds reject the operation. [fp/pdr_engine.go](fp/pdr_engine.go), [fp/query_expression.go](fp/query_expression.go). E6. |
| 52–53: relation count other than one returns `l_undef` | `QueryPredicates` checks non-Datalog cardinality before inspecting individual declarations. Datalog has a separate source override for multiple relations. E1, E4, E6. |
| 54–63: owning query/argument/sort vectors; ascending variable indices | `makeRelationQueryExpression` in [fp/bmc_engine.go](fp/bmc_engine.go) constructs `R(var(0), …)` with the declared domain sorts and an owner reserved before each constructor. `queryRelationExpression` keeps the result owned through the live expression query. E4. |
| 64–70: reverse binder sorts/names, build application, bind only nonnullary queries, dispatch expression query | The dense relation wrapper and `QueryExpression` perform equivalent existential normalization; [fp/query_head.go](fp/query_head.go) uses descending de Bruijn arguments and reversed head domains. Nullary relations have no variable/binder work. Actual BMC/QBMC/CLP/Duality callers and Spacer's separate relation wrapper are covered, including mixed sorts and ordering. E4, E5. |
| 72–74: unsupported `query_from_lvl` default | `QueryExpressionFromLevel` and `QueryPredicateFromLevel` admit only the Spacer override and reject other backends. E6. |
| 76, 78: empty `reset_statistics` and `collect_statistics` defaults | `ResetStatistics` / `CollectStatistics` in [fp/propositional_engine.go](fp/propositional_engine.go) dispatch concrete overrides and perform no backend statistics work in inherited-default branches. Go validity checks are additional API guards. E6. |
| 77: empty `display_profile` default | No live source call dispatches this generic default: `context::display_profile` calls `m_rel->display_profile` directly. [fp/datalog_profile.go](fp/datalog_profile.go) implements that concrete consumer. An unused generic profile adapter is unnecessary. E6. |
| 79–81: unsupported `get_num_levels` default | `NumLevels` in [fp/pdr_engine.go](fp/pdr_engine.go) selects the configured engine, dispatches PDR/QPDR or Spacer, and rejects unavailable engines. E6. |
| 82–84: unsupported `get_reachable` default | `Reachable` in [fp/pdr_engine.go](fp/pdr_engine.go) requires the Spacer override and a valid result owner. E6. |
| 85–90: unsupported `get_cover_delta` / `add_cover` defaults | `CoverDelta` / `AddCover` in [fp/pdr_engine.go](fp/pdr_engine.go) dispatch the PDR/QPDR and Spacer overrides; other engines reject. Existing slicing/property guards remain. E6. |
| 91–93: unsupported `add_invariant` default | `AddInvariant` delegates to `AddCover(-1, …)`. Unsupported engines still reject; the existing PDR invariant operation is a richer Go API choice, recorded below. E6. |
| 94–96: unsupported `display_certificate` default | `WriteCertificate` now rejects the inherited Datalog operation. All concrete certificate branches remain separate. Datalog's source path is `rel_context` → `rel_context_base` → `engine_base`, with no certificate override. DDNF rejection is its own concrete override behavior, not evidence for this default: its source display method calls its unreachable `get_answer`. E7. |
| 97–99: allocate and return a fresh owning empty model | `Model` returns `model.NewModel` for BMC/QBMC/TAB/CLP/DDNF after Go validity/snapshot checks, independently of a valid result's True/False/Undef status. Each export is independent. Richer Datalog/PDR/Spacer/Duality model paths are preserved. E2. |
| 100–102: unsupported `get_rules_along_trace` default | `RulesAlongTrace` in [fp/pdr_engine.go](fp/pdr_engine.go) rejects unavailable backends, dispatches Spacer, and preserves the existing BMC/QBMC trace extension. E1, E6. |
| 103–105: return owning `asserted(true)` proof | The default `Proof` branch reserves an empty AST reference before `MkProofAsserted`, adopts any nonnull constructor result even with an error, and uses retryable reference cleanup. Disabled proof mode still gives the source null proof. Concrete PDR/Duality/Spacer overrides remain unchanged. E3. |
| 106: empty `updt_params` default | `ParametersUpdated` performs Go snapshot invalidation and concrete engine refresh. No inherited source algorithm is missing. Parameter-dependent derived behavior is audited in its own unit. E1. |
| 107: empty `cancel` default | No live generic `engine_base::cancel` caller was found in the source context/API/command paths. Concrete resource-limit/cancellation mechanisms remain separate; no unused adapter was added. |
| 108: empty `cleanup` default | There is no inherited cleanup algorithm. Live query-resource cleanup and concrete owner destruction are implemented by the dispatcher/backend owners; this row does not claim a whole-context lifecycle audit. E2–E5. |
| 113–117: pure engine factory and context association | `NewPropositionalEngineWithServices`, `SelectEngine`, and the concrete factory branches associate the actual manager/parameter context and select all engines. Source `register_engine::mk_engine` groups PDR/QPDR and BMC/QBMC; Go preserves those concrete backend relationships. E1. |

## Owned locals and failure branches

The header contains owned construction only in the relation wrapper, default
model and default proof. Their actual Go handoffs were audited through callers,
including the shared synthetic query head required by those callers.

| Source owner or handoff | Go ownership evidence |
| --- | --- |
| `expr_ref_vector args` | Empty AST roots are reserved before each `MkVar`; nonnull results are adopted before propagating constructor errors. The cold 512-column campaign requires interruption and completion; the nullary campaign requires completion and may use existing arena pages. Both retry every ordinal and compare live AST counts. E4. |
| `sort_ref_vector sorts`, `svector<symbol> names` | Go slices and symbol values replace source containers. Domain sorts stay reachable through the registered declaration and then the argument/query ASTs. Dense variable and reversed-head tests verify the binder mapping without porting STL internals. E4, E5. |
| `expr_ref q` and expression-query handoff | The final query root remains owned through `QueryExpression`. Temporary argument roots are released only while the query owns its children. Cleanup refusal returns a `queryReferenceCleanupFailure` from [fp/query_reference_cleanup.go](fp/query_reference_cleanup.go), preserving the remaining roots for retry. E4 includes actual native `ErrASTPending` refusal and recovery. |
| Fresh synthetic declaration/head in live consumers | [fp/query_head.go](fp/query_head.go) owns variables, fresh declaration and application before each fallible step, returning an owning head with a borrowed declaration. [fp/query_expression.go](fp/query_expression.go), [fp/duality_preprocess.go](fp/duality_preprocess.go), and [fp/spacer_relation_query.go](fp/spacer_relation_query.go) keep that owner until registration/rule ownership is established. E5. |
| Spacer's unpublished query rule | [fp/spacer_relation_query_cleanup.go](fp/spacer_relation_query_cleanup.go) keeps the rule, temporary roots and any nested construction receipt until cleanup succeeds. Failure publishes neither declaration nor rule. The native finalizer regression verifies the actual rule remains valid through repeated refusal, then closes cleanly. E5. |
| `model_ref(alloc(model, m))` | Distinct empty model exports survive later queries and engine closure. Construction-failure tests require no partial result and balanced live nodes/references and budget, followed by a successful retry. Public API `LastModel` exercises the same implementation. E2. |
| `proof_ref(m.mk_asserted(m.mk_true()), m)` | The default result reference is reserved before fresh proof construction. The cold physical-reference-allocation regression fails at the actual publication frontier and checks live nodes, references, infos and parameters, then retries twice with an asserted-true semantic oracle. E3. |

## Actual source consumers

| Source consumer | Go consumer |
| --- | --- |
| `dl_context.cpp:234,924–925`: bind factory/context, instantiate engine, update parameters | `NewPropositionalEngineWithServices`, `SelectEngine`, `ParametersUpdated`. |
| `dl_context.cpp:887,910,938`: expression, from-level and relation queries | `QueryExpression`, `QueryExpressionFromLevel`, `QueryPredicate(s)`; command-context and API query entry points use these paths. |
| `dl_context.cpp:914–919,946–961`: model, proof, answer, ground answer and trace exports | Dispatcher exports, expression-clone exports and [../api/api_fixedpoint_default_model_test.go](../api/api_fixedpoint_default_model_test.go)'s actual `LastModel` caller. |
| `dl_context.cpp:545–565`: levels, cover, reachable and invariant operations | [fp/pdr_engine.go](fp/pdr_engine.go) and configured-property dispatch tests. |
| `dl_context.cpp:746,769`: parameter update and cleanup | `ParametersUpdated` and concrete query/backend ownership paths; inherited hooks themselves contain no additional work. |
| `dl_context.cpp:986–988`, `dl_cmds.cpp:269–277,364–368`: certificate dispatch after SAT/UNSAT status | [../cmd_context/fixedpoint.go](../cmd_context/fixedpoint.go) `WriteFixedpointCertificate` and [../parsers/smt2/commands.go](../parsers/smt2/commands.go). Unsupported Datalog errors occur before the command's trailing certificate newline. |
| `dl_context.cpp:1005,1011,1017`: concrete relation profile and generic statistics | `DatalogProfile`, `ResetStatistics`, `CollectStatistics`. |
| `horn_tactic.cpp:271,284`: obtain proof/model from completed query | [fp/horn_tactic.go](fp/horn_tactic.go) and the source-default-proof regression in E3. |

The small `base.EngineBase` adapter has no production consumers. Its fake-core
unit tests are not used as evidence that the live fixedpoint dispatch is complete.

## Focused test catalogue

| Evidence | Exact files and tests |
| --- | --- |
| E1: selection, defaults and valid snapshots | [fp/engine_result_routing_test.go](fp/engine_result_routing_test.go): `TestRetainedEngineKindSelectionAndDiagnostics`, `TestDatalogResultSnapshotRoutesAndInvalidates`, `TestPDRAndSpacerPreQueryResultGuards`, `TestPDRAndSpacerResultSnapshotsInvalidateAfterParameterRevision`, `TestEngineSwitchDoesNotRepublishPriorBackendResult`; [fp/configured_predicate_dispatch_test.go](fp/configured_predicate_dispatch_test.go): `TestConfiguredDirectPredicateQueryDispatch`. |
| E2: default model and public ownership | [fp/engine_default_model_test.go](fp/engine_default_model_test.go): `TestEngineDefaultModelStatusAndIndependentOwners`, `TestEngineDefaultModelConstructionFailureAndRetry`; [../api/api_fixedpoint_default_model_test.go](../api/api_fixedpoint_default_model_test.go): `TestAPIFixedpointLastDefaultModel`. |
| E3: default proof and tactic consumer | [fp/engine_default_proof_test.go](fp/engine_default_proof_test.go): `TestEngineDefaultProofReferenceAllocationFailureAndRetry`; [fp/horn_tactic_datalog_proof_test.go](fp/horn_tactic_datalog_proof_test.go): `TestHornTacticDatalogUsesSourceDefaultProof`; E1's Datalog snapshot test checks the proof-disabled null result. |
| E4: generic relation construction and consumers | [fp/relation_query_ownership_test.go](fp/relation_query_ownership_test.go): `TestRelationQueryColdConstructionFailuresAndRetry`, `TestRelationQueryOwnedExpressionAcrossLiveBackends`; [fp/relation_query_cleanup_test.go](fp/relation_query_cleanup_test.go): `TestRelationQueryExpressionSurvivesNativeCleanupRefusal`; [fp/bmc_relation_query_test.go](fp/bmc_relation_query_test.go): `TestBMCRelationQueryUsesSourceVariableOrder`; [fp/clp_query_cardinality_test.go](fp/clp_query_cardinality_test.go): `TestCLPRelationQueryChecksCardinalityBeforeDeclarations`. |
| E5: shared head and Spacer's separate owner | [fp/query_head_ownership_test.go](fp/query_head_ownership_test.go): `TestQueryHeadAndSpacerRelationColdConstructionOwnership`; [fp/spacer_relation_query_cleanup_test.go](fp/spacer_relation_query_cleanup_test.go): `TestSpacerRelationQueryFinalizerPreservesUnpublishedRule`; [fp/spacer_relation_query_test.go](fp/spacer_relation_query_test.go): `TestSpacerRelationQueryReversesMixedSortHead`, `TestSpacerRelationQueryProofFollowsTraceSetting`. |
| E6: remaining live interface branches | [fp/configured_property_dispatch_test.go](fp/configured_property_dispatch_test.go): `TestConfiguredPrequeryPropertyDispatch`, `TestConfiguredPropertyDispatchPreservesSlicingGuard`; [fp/configured_predicate_dispatch_test.go](fp/configured_predicate_dispatch_test.go): `TestConfiguredMultiplePredicateUnsupportedGuard`, `TestConfiguredPredicateQueryFromLevelDispatch`; [fp/propositional_engine_test.go](fp/propositional_engine_test.go): `TestSpacerEngineUndefinedAnswerAndGroundAvailability`; [fp/datalog_profile_test.go](fp/datalog_profile_test.go): `TestDatalogProfileCountsRulesRelationsTuplesAndCompiler`; [fp/pdr_expression_statistics_test.go](fp/pdr_expression_statistics_test.go) and [fp/spacer_expression_statistics_test.go](fp/spacer_expression_statistics_test.go): `TestPDRExpressionStatisticsSurviveCloneDisposal`, `TestSpacerExpressionStatisticsSurviveCloneDisposal`. |
| E7: unsupported certificate and public propagation | [fp/engine_default_certificate_test.go](fp/engine_default_certificate_test.go): `TestDatalogDefaultCertificateRefusesWithoutChangingResult`; [../parsers/smt2/datalog_certificate_test.go](../parsers/smt2/datalog_certificate_test.go): `TestSMT2DatalogCertificateReportsUnsupported`. Both new tests failed against the old implementation: direct calls returned nil error, and public SAT/UNSAT queries printed an extra blank line. Existing PDR, BMC, QBMC, TAB, CLP and Duality certificate tests also passed after the correction. |

Final native gates used `GOCACHE=/tmp/g3-go-build`, `-count=1`, selected test names
and `-timeout=90s`: default/wrapper/interface tests passed in `muz/fp` (0.797s)
and `api` (0.052s); E6's remaining interface tests passed (0.609s); certificate
and concrete-backend coexistence checks passed in `muz/fp` (0.040s) and
`parsers/smt2` (0.355s). Root's independent certificate gate passed in 0.059s /
0.757s. These are focused closure checks, not a claim that a new full suite ran.
The preceding unchanged query-owner production passed the full native fixedpoint
package in 66.216s before the final certificate correction.

## Repairs established by the audit

| Commit | Source contract repaired |
| --- | --- |
| `4fa14bcf5` | Fresh independent empty models for inherited BMC/QBMC/TAB/CLP/DDNF model exports, including public `LastModel`. |
| `812d374ec` | Ownership throughout generic relation-query construction and expression dispatch. |
| `9b33b7515` | Owning result reservation before default asserted-true proof construction. |
| `6edd84f7d` | Owning synthetic query-head handoff and Spacer's retryable unpublished query-rule scope. |
| `f07739846` | Source-default unsupported Datalog certificate rejection through direct and SMT-LIB dispatch. |

## Explicit adaptations and exclusions

- Go returns typed errors instead of reproducing C++ exception wording. The
  SMT-LIB parser wraps the message and location in `ScannerError`; the public
  regression verifies that representation and the exact status-only output.
- Existing open/result-snapshot guards reject stale, pre-query and closed
  exports more strictly than the bare source virtual defaults. Within a valid
  snapshot, the inherited empty-model operation is not restricted to UNSAT.
- Multiple-relation rejection preserves source `l_undef` while exposing Go
  unknown-reason metadata. Dense existential queries normalize through the
  Go rule/query representation rather than constructing a redundant binder AST.
- Existing richer Datalog models, PDR invariant insertion and BMC/QBMC trace
  exports remain explicit Go API extensions. They are not evidence that source
  defaults themselves contain those algorithms.
- Source no-op hooks without live generic consumers do not justify new unused
  adapters. Go containers replace C++ containers; no STL internals are ported.
- This audit does not complete `dl_context`, `dl_rule`, `dl_register_engine`,
  `rel_context`, or any PDR/Spacer/BMC/TAB/CLP/Duality/DDNF source unit. In
  particular, sparse query-variable compaction and context pre-install/result
  lifecycle remain separate audits; dense header wrappers do not enter sparse
  compaction. Closure of this abstract header does not certify every derived
  proof, model, search or cleanup algorithm.
