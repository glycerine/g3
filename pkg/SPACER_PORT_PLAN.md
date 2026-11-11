# Spacer query-pipeline and solve-phase lifetimes: porting plan

Written 2026-11-11 from a read-only study of src/muz/spacer, src/muz/transforms
(C++) and pkg/muz (Go), plus trace comparisons of the 26 `*spacer*` vectors
(`build/z3` vs HEAD `6145273a5`). Companion to PDR_PORT_PLAN.md (item 31);
its step 6 hand-offs (buckets C and D) are absorbed here. Nothing in the tree
was changed while writing this.

Tools (all under `W=/var/tmp/g3-job-487c9ee8`, see TODO item 31 and 37):

- `bash $W/tdiff.sh <vector> <g3-binary> [N]`: first N lines of the
  pointer-masked full-trace diff; leaves `$W/t.z` and `$W/t.g`.
- `python3 $W/fullcmp_log.py <g3-binary> <dir> > $W/fl_curN.txt`: all vectors.
- `bash $W/build_g3rcdel.sh` → `$W/g3rcdel`: with `Z3_TRACE_DEL=1` writes
  `[del] #id` lines into the trace, and with `G3_RC_ID=<id>` also one
  `[rc+/-] #id rc=N <stack>` line for every refcount change of that main-
  manager node. This is the tool that found every holder below. Strip
  `^\[rc` lines before diffing against z3's log.
- `$W/z3del/build_trace/z3` with `Z3_TRACE_DEL=1`, `Z3_DEL_BT=<id>`
  (deleter stacks on stderr, one `---- del #id` block per deletion, take the
  n-th), `Z3_MK_ID=<id> Z3_MK_ID_OCC=<n>` (creator stack). Symbolize with
  `addr2line -i -f -C -e $W/z3del/build_trace/z3`.
- Node ids are reused: always count occurrences of `[mk-*] #id` / `[del] #id`
  up to the divergence line in *each* log to pick the right life.

## 1. Where the 15 differing spacer vectors stop (HEAD 6145273a5)

Full logs: 624 of 660 comparable vectors identical; 15 of the 26 spacer
vectors differ. First-difference line / total:

| vector | first diff | cause (section) |
|---|---|---|
| fp_spacer_quantifier_abstraction | 67/292 | T1 hnf retention |
| fp_spacer_array_blast | 81/200 | T2 context rewriter cache |
| fp_spacer_array_eq_rewrite | 86/359 | T3 instantiate_rule local order |
| fixedpoint_spacer_nonlinear_inlining | 109/264 | T4 inliner resolvent shape |
| fp_spacer_mutual_inv | 116/2332 | T5 qe_lite in-place rewrite |
| fp_spacer_bit_blast | 128/1121 | I1 init_rule conj pins + R1 m_pr |
| fp_spacer_quantifier_instantiation | 274/1051 | T6 instantiate_quantifier local |
| fixedpoint_spacer_unfold_rules | 432/479 | S1 mk_reach_fact flatten/mk_and |
| fixedpoint_spacer_recursive_trace | 460/20644 | K1 eq-chain proof order |
| fp_spacer_scale | 1466/1517 | S1 |
| fp_spacer_linear_inv | 2270/10945 | K2 conflict-proof retention |
| fp_spacer_mincut_inv | 2270/10945 | K2 (same prefix as linear_inv) |
| fixedpoint_spacer_datatype_state | 2469/31435 | K3 model-evaluator numeral |
| fp_spacer_generalizers_inv | 2673/5156 | S2 elim_proxies |
| fixedpoint_spacer_bv_state | 6983/193519 | K4 bv atoms at pop |

Identical already: spacer_core, spacer_inliner_resolution, spacer_slice_columns,
spacer_coalesce_unfold, spacer_acyclic_inlining, fp_spacer_array_instantiation,
fp_spacer_flexible_trace, fp_spacer_karr, fp_spacer_magic,
fp_spacer_skip_propagate, fp_spacer_arith_inv. stdout matches z3 on all 26.
The PDR plan's bucket C (`asserted(true)` from init_reach_facts) is resolved:
both sides now emit the same `asserted #1` lines.

The first seven are transformer-phase (before the first kernel check) and
are the same kind of problem the PDR plan fixed: a g3 local released where
z3 has a longer-lived holder, or the reverse. The rest are solve-phase.

## 2. Query flow: spacer::dl_interface::query vs querySpacerAtLevel

z3 (spacer_dl_interface.cpp:84): `ensure_opened; m_refs.reset;
m_pred2slice.reset; rules0 = get_rules() [flush]; rule_set old_rules(rules0)
[copy, before the query rule]; rm.mk_query(query, rules0) [adds query!N and
its rule to the context's rules]; bg_assertion = get_background_assertion();
check_reset() [ends with m_old_rules.replace_rules(new_rules): the previous
query's m_old_rules, query rule included, die here]; apply_default_
transformation; slice; coalesce/unfold; m_spacer_rules.replace_rules(rules);
record_transformed_rules; reopen; m_ctx.replace_rules(old_rules) [the query
rule leaves the persistent program]; scoped_restore_proof; set_proof_converter;
set_model_converter; set_query; set_axioms; update_rules(m_spacer_rules);
solve.`

g3 (muz/fp/spacer_engine.go:629 `querySpacerAtLevel`, reached from
`querySpacerRelation` → `queryRelationMkQuery` → `makeSpacerRelationQuery`):

- **Q1.** `makeSpacerRelationQuery` (muz/fp/spacer_relation_query.go) builds
  the query rule as a free-standing `base.Rule` and never adds it to `e.set`;
  `preprocessHornRulesWithStages` adds it to the transformation input. PDR's
  path (`queryRelationMkQuery` + `addQueryRuleNow`/`removeQueryRule`,
  muz/fp/pdr_relation_query.go) puts it in `e.set` like z3 and removes it after
  solve. Consequences: z3's `m_old_rules` (check_reset snapshot) contains the
  query rule and keeps its head app and body alive until the next query;
  g3's `snapshotHornRules(e.set)` does not. Also the rule's accounting and
  `mk_asserted` proof lifetimes differ.
- **Q2.** `check_reset` timing. g3 replaces `e.spacerRules` (m_old_rules)
  after `UpdateRules` (`previousSnapshot.Close()` at spacer_engine.go ~770);
  z3 replaces it right after mk_query, before the transformations. PDR got
  this in 6145273a5 (`checkPDRSourceRules` now does the replace and
  `closePDRForReplacement` keeps m_old_rules across a context rebuild). Do the
  same in `shouldResetSpacerRules`/`querySpacerAtLevel`.
- **Q3.** Background assertion: z3 builds a fresh `mk_and(m_background)`
  per query when there are two or more assertions and holds it to function
  end (PDR plan D8). g3 passes `e.background`. Check `AssertBackground` on a
  two-assertion vector before deciding.
- **Q4.** `record_transformed_rules()` keeps a second copy of the transformed
  rule set in the datalog context (`m_transformed_rule_set`) until the next
  query. Rule objects are shared by refcount with `m_spacer_rules`, so AST
  lifetimes do not change; no action unless a trace shows otherwise.
- The rest (proof-mode restore, `init_lemma_generalizers` before
  `init_rules`, obj_map order in `init_rules`, one `spacer::context` per
  interface rebuilt by `updt_params`) already matches.

Mux conversions: spacer's `Manager.FormulaNewToOld/OldToNew` already return
caller-owned `ast.ASTRef` (muz/spacer/manager.go:249, sym_mux.go:387), which
is what `sym_mux::conv_formula` does (a local `rewriter_tpl<conv_rewriter_cfg>`
writing into the caller's `expr_ref`; nothing retained). The retention bug is
PDR-only: `muz/pdr/symbol_mux.go` appends every result to `m.references`
(released at mux close). Five PDR call sites still use the retaining form:
predicate_rules.go:818/933/993, context_bounded.go:58, validate_model.go:58.
Convert them to `FormulaNewToOldRef` with the z3 local's scope (z3:
`pred_transformer::mk_assumptions` tmp1/tmp2 pushed into `result`;
`get_formulas`/`get_propagation_formula` locals; `is_reachable`'s locals;
`validate` locals) and then delete `FormulaNewToOld`/`FormulaOldToNew`'s
retaining forms so no caller can regress.

## 3. Transformer-phase findings (T1–T6, I1, R1)

Each entry: the g3 node, who creates and frees it on each side, and the fix.

- **T1 (fp_spacer_quantifier_abstraction @67).** `#37 = (p x (select A x))`
  is created twice; the second life is made in
  `mk_quantifier_abstraction::mk_head` and in z3 dies only in `hnf::~hnf`
  at `datalog::context::~context`. That hnf is `rule_manager::m_hnf`: its
  `operator()` calls `reset()` first, so the previous rule's `m_body`,
  `m_head`, `m_defs`, `m_todo`, `m_fresh_vars` and proofs survive until the
  next `mk_rule` that goes through hnf, or destruction. g3
  (`QuantifierAbstractionPlugin.assign` → rc 1→0) frees it when the plugin's
  rule assignment ends. g3's `RuleManager.hornTemporaries`
  (muz/base/rule_formula.go:251,373) is the right place but holds fewer
  things than hnf's imp. Port hnf's retained member set exactly (read
  src/ast/normal_forms/hnf.cpp `imp::reset`), including the final
  `result`/`ps` vectors that z3 copies out but whose members stay referenced
  by `m_body`.
- **T2 (fp_spacer_array_blast @81).** `#42 = (and #46 #44)` is created by
  the `bool_rewriter` inside `mk_array_blast::ackermanize`'s `th_rewriter
  m_rewriter` (a member constructed at plugin registration) and in z3 dies in
  `act_cache::~act_cache ← rewriter_core::~rewriter_core ← datalog::context::
  ~context`: the last holder is a rewriter cache owned by the datalog context,
  i.e. `ctx.get_rewriter()` (PDR plan D2), which the interp-tail simplifier
  used on a formula containing #42 and which z3 never flushes between
  transformations or queries (only `context::reset`). g3 frees it in
  `ArrayBlastPlugin.blast.func3`. Check (a) that `ArrayBlastPlugin`'s own
  rewriter is constructed at registration and closed with the plugin
  (dl_mk_array_blast.cpp:34, `m_rewriter(m, m_params)`), and (b) that g3's
  engine-level context rewriter (the `contextRewriter` argument of
  `NewArrayBlastPlugin` and the simplifiers) is never `Reset` or `Close`d
  during the engine's life except where `dl_context.cpp` calls
  `m_rewriter.reset()`. Confirm the holder with `G3_RC_ID=42` on the
  g3rcdel build after (a).
- **T3 (fp_spacer_array_eq_rewrite @86).** `#52 = (= #37 #37)` is created
  and freed inside `mk_array_eq_rewrite::instantiate_rule` on both sides;
  z3 frees `#54 #53 #51 #50` before `#52`, g3 frees `#52` third. Match the
  declaration order of instantiate_rule's `expr_ref` locals (reverse
  destruction) in `ArrayEqRewritePlugin.transformRule`'s deferred releases
  (muz/transforms/array_eq_rewrite.go).
- **T4 (fixedpoint_spacer_nonlinear_inlining @109, shape).** With
  `generate_proof_trace`, z3's `rule_unifier::apply` builds the resolvent
  directly: `apply(tgt.get_head())`, `apply(tgt, tail_index)`, `apply(src)`,
  `remove_duplicate_tails`, `res = m_rm.mk(new_head, tail, tail_neg, name,
  m_normalize)`, `set_accounting_parent_object`, then `fix_unbound_vars(res,
  true)` and `m_interp_simplifier.transform_rule(res)`; the proof is added
  afterwards by `datalog::resolve_rule(m_rm, tgt, src, tail_index, s1, s2,
  *res)` (dl_mk_rule_inliner.cpp, `try_to_inline_rule`). g3's
  `RuleInlinerPlugin.resolve` → `RuleUnifier.resolveRaw` → `Rule.ToFormula`
  (`and #25 #82 #79`) → `mk_rule` from a formula, whose th_rewriter under
  proofs emits the extra `iff`/`rewrite`/`mp` steps and a different body.
  Port `rule_unifier::apply` as above (a `RuleManager.Mk(head, tail, neg,
  name, normalize)` that does not round-trip through a formula) and
  `resolve_rule` for the proof; delete the ToFormula path. Verify on
  nonlinear_inlining, spacer_acyclic_inlining, spacer_inliner_resolution and
  fp_spacer_mutual_inv (all inliner-heavy; the last three must stay
  identical).
- **T5 (fp_spacer_mutual_inv @116).** In `qe_lite::impl::operator()
  (uint_set, bool, expr_ref_vector&)` with one non-ground conjunct, z3 copies
  it to `fml`, runs the single-formula `operator()` (der, fm, then
  `m_rewriter(fml)` in place: `th_rewriter::operator()(expr_ref&)` assigns
  the result over the input, freeing `#78` there), then `fmls[index] = fml`.
  g3 (`Lite.eliminateConjunctions`, pkg/qe/lite_indices.go:43-55) keeps the
  pre-rewrite formula pinned in the DER call until `drain`, so `#84` dies
  before `#78`. Release the input at the in-place assignment.
- **T6 (fp_spacer_quantifier_instantiation @274).** The instantiated
  quantifier `#68 (k!17 ...)` (19th life) is created by `var_subst` in
  `mk_quantifier_instantiation::yield_binding` and in z3 dies inside
  `instantiate_quantifier` (a local); g3 keeps it to the end of
  `QuantifierInstantiationPlugin.transformRule` (temporaries of
  `ExprSafeReplace.Apply`, `patternedQuantifier`, `InferenceRewriter`).
  Match `instantiate_quantifier`/`instantiate_rule`'s locals
  (dl_mk_quantifier_instantiation.cpp).
- **I1 (fp_spacer_bit_blast @128).** `#68 = (= rep true)` is an element of
  `pred_transformer::init_rule`'s local `conj` (created in `init_atom`);
  z3 does `fml = mk_and(conj); conj.reset();` and after that the only holder
  is the rewrite proof (see R1). g3's `spacerRuleBuilder.initializeAtom`
  pins every conj element (`b.pin`, predicate_rules.go:49,60) and
  `normalizeRule` pins again (rc=2), both until `PredicateTransformer.Close`.
  Make `conj` a local of the per-rule normalization released right after the
  `mk_and`, as z3 does; keep only what z3 keeps (`m_rule2transition`,
  `m_rule2vars`, `m_rule2tag`, `transitions`).
- **R1 (cross-cutting; also TODO item 37's bv_fixed_cache_scopes).**
  `rewriter_tpl::operator()(t, result)` is `operator()(t, result, m_pr)`
  (rewriter.h:350): with proofs enabled, the final proof of a two-argument
  rewrite is stored in the member `m_pr` and stays there until the next
  traversal's first `process_const`, whose `th_rewriter_cfg::reduce_app`
  begins with `result_pr = nullptr`, or until the next `main_loop` end
  overwrites it. That is the deleter seen for #68 here and for #121 in
  bv_fixed_cache_scopes (`reduce_app ← process_const ← visit ← process_app`
  of the next `assert_expr`). g3's `rewriterEngine.Rewrite` does pass `r.pr`
  and `theoryRewriterConfiguration.ReduceApp` resets the proof ref at entry
  (th_rewriter.go:958), so the mechanism exists; what differs is which
  rewriters are two-argument callers. Audit the g3 callers that correspond
  to z3 `rw(fml)` / `rw(t, result)` sites (init_rule, asserted_formulas::
  assert_expr's `m_rewriter(e, r, pr)` is three-argument and does *not*
  retain) and make each use `Rewrite` (member proof) or `RewriteWithProof`
  (caller proof) exactly as z3 does. Check bv_fixed_cache_scopes 4218 and
  fp_spacer_bit_blast 128 together.

## 4. Solve-phase findings (S1–S2, K1–K4)

- **S1 (fixedpoint_spacer_unfold_rules @432, fp_spacer_scale @1467).**
  `context::mk_reach_fact` (spacer_context.cpp:3090) builds `path_cons` =
  [`pt.get_transition(r)`, each child's `formula_n2o(kid->get(), i)`], then
  `res = m_pm.mk_and(path_cons)` (spacer manager `mk_and`: ast_util's
  0→true, 1→the element, else `m.mk_and`, no `true` kept, flat), optionally
  `compute_implicant_literals` → `res = m_pm.mk_and(lits)`, then
  `qe_project`. g3's `PredicateTransformer.MakeReachFact`
  (predicate_reach_projection.go:123) uses `MkAndArray(path)`
  (`astUtilMkAnd`), which produced `(and X true)` and a nested
  `(and #54 (and #206 #92))` where z3 has a flat 5-ary `and`; the implicant
  literal order then differs. Use the manager's `mk_and` on the flattened
  path (and check `flatten_and` is applied where z3 applies it).
- **S2 (fp_spacer_generalizers_inv @2673).** After a pop, g3 runs
  `ItpSolver.eliminateProxies` (itp_solver.go:471) again on a second formula
  (`#170`), emitting `iff #170 #170`/`refl`; z3 starts the next check. z3's
  `elim_proxies(core)` does one `rep(f)` on `f = mk_and(core)` then
  `flatten_and`; it is called from `get_itp_core` only when
  `result == l_false && m_core && m.proofs_enabled() && !m_subset_based_core`
  (spacer_prop_solver.cpp:229). Compare core contents at this point (the
  farkas learner output) and make `eliminateProxies` rewrite the single
  conjunction, not each element.
- **K1 (fixedpoint_spacer_recursive_trace @460).** The kernel's conflict
  proof builds `unit-resolution #127 …` before `unit-resolution #107 …` in
  z3 and after it in g3, inside a `trans*(symm(…), …)` equality-chain proof.
  Same family as 434ecc52f (LIFO todo order): check
  `smtConflictProofBuilder.prefetchEquality`/`equalityProof` against
  `conflict_resolution::visit_eq_justications` + `get_proof(n1, n2)` for an
  EQUATION step whose literal proof is a clause (`unit-resolution`), and the
  symmetric-branch order. Use `-tr:conflict` on both binaries.
- **K2 (fp_spacer_linear_inv, fp_spacer_mincut_inv @2271).** Right after
  a conflict, g3 creates `(<= #28 #197)`/`not` and immediately frees them
  together with `#221 (not #239)`, `#239 (<= #197 #28)`, `#197 (Int)` and the
  proofs `#244/#228 (unit-resolution …)`; z3 keeps #239/#221/#197 alive.
  `mk_conflict_proof` ends with `m_new_proofs.reset()` on both sides, so the
  holder is elsewhere: the lemma clause's proof, or theory_arith's bound
  atoms. Run `G3_RC_ID=239` (g3rcdel) and `Z3_DEL_BT=239` to get both
  release points.
- **K3 (fixedpoint_spacer_datatype_state @2469).** g3's model evaluator
  (`ArithRewriter.cancelMonomials → numeralForTerm`) creates an Int numeral
  that z3 still has alive (z3's `model_evaluator` keeps its rewriter cache
  across evaluations within one evaluator; g3 may reset per evaluation).
  Compare `model_evaluator::operator()` cache lifetime with
  `model.ModelEvaluator.Evaluate`.
- **K4 (fixedpoint_spacer_bv_state @6983; del-diff at 12135).** At a kernel
  `[pop] 1 1`, z3 frees a batch of theory_bv atoms (`bit2bool`), an equality
  and `or` clauses that g3 keeps. SMT-core/theory_bv pop lifetime, not
  spacer; same area as card2bv_weighted_models in TODO item 37 (bit2bool
  atoms freed in sweep order). Investigate in smtcore after the spacer items.

## 5. Porting order, with the check for each step

Run `$W/tdiff.sh <vector> <bin>` per step and `fullcmp_log.py` + `outcmp.py`
+ `go test ./muz/... ./qe/ ./smtcore/` before each commit; `make` and `make
tags` hourly. Commit with explicit pathspecs, no amend, no Co-Authored-By.

1. **Query flow (Q1, Q2, Q3).** [Q2 done 0848932bf; Q1 was already in
   place via querySpacerRelation; Q3/Q4 untouched pending a trace.] Route spacer through `queryRelationMkQuery`
   like PDR (query rule in `e.set`, removed after solve, retained owner
   closed), move the m_old_rules replace into the spacer check_reset, keep
   m_old_rules across a context rebuild. Check: every identical spacer vector
   stays identical; fp_spacer_quantifier_abstraction and
   fixedpoint_spacer_nonlinear_inlining may move.
2. **PDR mux call sites** (section 2): five sites to `…Ref`, delete the
   retaining forms. [Done for check_inductive (the only hot site). The
   others are unreached by source consumers: Premises only under the debug
   assertInvariants, validate_model only under validate=true,
   PropagationFormula and CheckBounded only from tests. Left as is.] Check: fixedpoint_pdr_initial_trace (12926),
   pdr_recursive_trace (9291), pdr_utvpi_fragment (3465),
   pdr_general_arithmetic_fragment (754).
3. **T1 hnf retention.** Check: fp_spacer_quantifier_abstraction. [Done.
   The holders were rule_manager::m_head/m_body (final head and body apps
   kept until the next mk_horn_rule, now RuleManager.hornHead/hornBody)
   and rule_manager::m_rwr, the label_rewriter member whose rewriter cache
   keeps every app it rewrote until ~rule_manager (now RuleManager.labels;
   g3 built one per mk_rule call). The quantifier hoister also built the
   iff/ite `or` pair left to right; the source evaluates mk_and's
   arguments right to left. quantifier_abstraction 67 -> 77;
   fp_spacer_array_blast (T2) became identical: its last holder was this
   cache, not the context rewriter's.]
4. **T2 context rewriter cache lifetime** (and the array_blast member
   rewriter). Check: fp_spacer_array_blast, fp_spacer_bit_blast,
   fp_spacer_array_eq_rewrite.
5. **T3, T5, T6 local orders.** Check: array_eq_rewrite, mutual_inv,
   quantifier_instantiation.
6. **T4 rule_unifier::apply.** Largest transformer item. Check: the four
   inliner vectors named above. [Also fp_spacer_quantifier_abstraction,
   now at 112: its first difference is the order in which the inliner's
   retained inlined rules die at transformer teardown. z3 fills
   m_inlined_rules in do_eager_inlining's stratum order (transform_rule per
   candidate predicate) and rule_set::reset releases them front to back;
   g3's inliner (accumulateDefinitions/breakInliningCycles) is a
   reimplementation with its own order. Port do_eager_inlining with the
   stratifier as part of this step.]
7. **I1 + R1.** Check: fp_spacer_bit_blast and bv_fixed_cache_scopes
   together. [I1 done (7d973f4a5). bit_blast then needed the solve-phase
   items below and is identical; bv_fixed_cache_scopes (R1) untouched.]
8. **S1 mk_reach_fact.** Check: unfold_rules, fp_spacer_scale. [Done,
   with the rest of the solve-phase lifetimes it uncovered: extend_initial's
   app_ref v; qe_project reassigning the caller's expr_ref at its first
   flattening (ProjectOwned); qe_lite's tmp, its eq::der
   apply_substitution frame, and the qe_lite object living to the end of
   each qe_project round; and model_evaluator_util as one evaluator per
   expand_node (muz/spacer/model_evaluator_util.go). unfold_rules, scale,
   bit_blast and array_eq_rewrite are identical.]
9. **S2 elim_proxies.** Check: fp_spacer_generalizers_inv.
10. **K1, K2, K3** in smtcore/model; **K4** last. Each has its own vector.

Expected outcome after steps 1–9: the seven transformer-phase vectors and
the two S vectors identical; the K vectors advance to kernel-level
divergences that belong with TODO items 37 and 31's smtcore leads.
