# PDR/Spacer query-pipeline lifetimes: porting plan (TODO item 31)

Written 2026-11-10 from a read-only study of src/muz (C++) and pkg/muz (Go),
plus trace comparisons of the 55 `fixedpoint_*`/`fp_spacer_*` vectors
(`build/z3` vs a build of HEAD `c806da09c`). Nothing in the tree was changed.

## 1. Diagnosis of the named symptom (fixedpoint_pdr_bit_blast)

Trace ids, both binaries (z3del with `Z3_TRACE_DEL=1`, g3bt2 with
`Z3_TRACE_DEL=1 G3_DEL_BT=<id>`):

```
[mk-app] #32 p #23            <- (p #b101)
[mk-app] #33 = #23 #23        <- (= #b101 #b101)
... bit blast builds p!bv!1 / query!0!bv!2 rules (#34..#48), identical in both ...
[del] #44 #43 #42 #41 #35 #34 #48 #47      identical in both
z3:  [mk-app] #47 query!0!bv!2 #1 #2 #1    then later  [del] #32  [del] #33
g3:  [del] #32  [del] #33   then            [mk-app] #33 query!0!bv!2 #1 #2 #1
```

Backtraces:

- z3 creates #32/#33 in `mk_interp_tail_simplifier::rule_substitution::apply`
  (`substitution::apply`, inside `propagate_variable_equivalences`) of the
  priority-40000 simplifier. z3 deletes them from
  `~mk_interp_tail_simplifier` ← `~rule_transformer` ← end of
  `apply_default_transformation`. The holder is the plugin member
  `rule_substitution m_rule_subst` whose `substitution m_subst` keeps every
  `apply` result in `m_new_exprs` until the next `reset()`/`reset_cache()`
  or destruction.
- z3 builds `query!0!bv!2 #1 #2 #1` in `rule_unifier::apply` ←
  `mk_rule_inliner::try_to_inline_rule` (the 34970 inliner, after bit blast
  at 35000), while #32/#33 are still pinned by the 40000 simplifier.
- g3 deletes #32 and #33 from `RuleSet.Close` ← `RuleTransformer.Apply`
  (the pre-bit-blast rule set) through `Rule.Close` →
  `AccountedObject.Close` → parent `Rule.Close`. The rule set is the *last*
  holder in g3 because `InterpTailSimplifierPlugin.propagate`
  (muz/transforms/interp_tail_simplifier.go:206) creates a fresh
  `substitution.New` per rule and closes it at return, so the apply-cache
  reference that outlives the rule in z3 never exists in g3.

Both sides retain the accounting parent rule (C++ `accounted_object` inc_refs
the parent; Go `SetAccountingParentRule` retains it). That is not a deviation.

So the root cause is one missing *member* object, not the overall shape of the
Go transformer; `RuleTransformer.Apply` (muz/base/rule_transformer.go) is
already a faithful port of `rule_transformer::operator()` (copy, per-plugin
replace, close the previous set, `replace_rules` at the end, plugins closed in
sorted order by `Close`).

## 2. Source-order map: C++ chain → Go

| # | C++ (src/muz) | Go (pkg/muz) | Notes |
|---|---|---|---|
| 1 | `context::rel_query` (base/dl_context.cpp:935) → `engine_base::query(num_rels, rels)` (base/dl_engine_base.h:52): args, reversed sorts/names, `mk_app`, `mk_exists` | `fp.(*PropositionalEngine).QueryPredicate` → `queryPDRRelation`/`querySpacerRelation` → `queryRelationMkQuery` (fp/pdr_relation_query.go:46); `makeRelationQueryExpression` (fp/bmc_engine.go:90) | q lives to the end of the query on both sides |
| 2 | `pdr::dl_interface::query` (pdr/pdr_dl_interface.cpp:74) / `spacer::dl_interface::query` (spacer/spacer_dl_interface.cpp:84): `ensure_opened`, `m_refs.reset`, `m_pred2slice.reset` | no equivalent members (`pdrPredicates`/`pdrSlices` maps are reset in `clearPDRPreprocessing`) | |
| 3 | `rules0 = m_ctx.get_rules()` → `flush_add_rules` (`mk_rule` per queued formula under `scoped_proof_mode`, `check_rules`) | `flushPendingRules` (fp/raw_rules.go:139) | same place in the order |
| 4 | `rule_set old_rules(rules0)` (copy, before the query rule) | `sourceSnapshot = snapshotHornRules(e.set)` taken *after* preprocessing and *including* the query rule (fp/pdr_engine.go) | deviation D6 |
| 5 | `rule_manager::mk_query` (base/dl_rule.cpp:264): `bind_variables(query,false)`, `quantifier_hoister::pull_quantifier`, free vars + compaction, `flatten_body`, `mk_fresh_head_predicate("query")`, `register_predicate(qpred,false)`, `set_output_predicate`, optional `filter_model_converter`, head/impl/forall, `mk_asserted` under `scoped_proof_mode`, `mk_rule(rule_expr, pr, rules)`; locals die at return | `hoistQueryExistentialsNamed`, `compactQueryVariables` (fp/query_variables.go), `makeQueryHead` (fp/query_head.go), `RegisterPredicate`, `MkImplies`, `MkQuantifier`, `addQueryRuleNow` (binds, asserted proof, `addFormulaWithProof` into `e.set`), `closeASTRefs(roots)` | shape matches; the query rule is removed again by `removeQueryRule` after solve (D6) |
| 6 | `bg_assertion = get_background_assertion()` (fresh `mk_and` when >1 assertion; lives to function end) | `e.background` ref passed to `context.SetBackground` | check D8 |
| 7 | `check_reset()`: compare `m_ctx.get_rules()` (now with the query rule) to `m_old_rules`; `m_context->reset()` on a fresh rule; `m_old_rules.replace_rules(new_rules)` (holds every rule, incl. query rule, until the next query) | `checkPDRSourceRules` (fp/pdr_rule_lifecycle.go) + `reset` decision in `queryPDRWithSliceProof`; `pdrSourceRules` snapshot replaced after `Initialize` | D6, D7 |
| 8 | `apply_default_transformation(m_ctx)` (transforms/dl_transforms.cpp): `flet` bind_vars=false, `ensure_closed`, register plugins (constructed here), `transform_rules(transf)`, `~rule_transformer` deallocs plugins in sorted order | `preprocessHornRulesWithStages` (fp/spacer_engine.go:31): builds `prepared` = copy of `e.set` (+ output predicate), `Finalize`, registers plugins (several constructed lazily), `transformer.Apply(prepared)`, reads model steps, `transformer.Close()` | D1–D5 |
| 9 | slice block: `rule_transformer transformer; mk_slice* slice`; `transform_rules`; copy `slice->get_predicates()` into `m_pred2slice`/`m_refs` (held until next query); transformer destroyed at block end | `slicer` transformer + `NewRuleSlicePlugin`; `PredicateSlices()` maps; `slicer.Close()` | mostly equivalent; `m_refs` pins decls until next query (part of D6) |
| 10 | unfold block: `transf1(coalesce)`, `transf2(unfold)` constructed together (mk_unfold constructs a `rule_unifier` → nested `mk_interp_tail_simplifier`), coalesce once, unfold N times, destroyed `transf2` then `transf1` at block end | `coalescer` built/applied/closed, then one `unfolder` per round built/applied/closed | D3 (construction time / destruction order) |
| 11 | `rules = m_ctx.get_rules()`; empty output preds → `set_unsat`; `query_pred = rules.get_output_predicate()` | `prepared.DerivedPredicate(declaration)` | |
| 12 | `m_pdr_rules.replace_rules(rules); m_pdr_rules.close(); m_ctx.record_transformed_rules(); m_ctx.reopen(); m_ctx.replace_rules(old_rules)` | `snapshot = snapshotHornRules(prepared)` → `e.pdrRules`; `prepared` itself closed by `preprocessed.Close()` after solve | D6: z3 keeps the transformed rules in two places (`m_pdr_rules`, `m_transformed_rule_set`) until the next query |
| 13 | `scoped_restore_proof _sc(m)`; `set_proof_converter`, `set_model_converter`, `set_query`, `set_axioms(bg)`, `update_rules(m_pdr_rules)` (`init_core_generalizers` may `toggle_proof_mode(PGM_ENABLED)`, then `init_rules`) | `context.SetAuxiliaryPredicates`, `SetBackground`, `Initialize(prepared)` (pdr/context.go; proof toggles at context.go:473) | D9 |
| 14 | empty rules → `set_unsat` + verbose model; else `m_context->solve()` | `solvePDRProgram` | |
| 15 | function exit: `_sc` restores proof mode, `bg_assertion`, `query_pred`, `old_rules` released (after solve) | deferred: `preprocessed.Close()` (frees transformed rules), snapshots, `parameters.Close()`, `removeQueryRule`, `closeASTRefs(queryRoots)` | D6 |

`dl_interface` itself (one per datalog context, constructed by
`ensure_engine` before `engine_base::query`) owns `m_context` (a single
`pdr::context`, rebuilt only by `updt_params`), `m_pdr_rules`, `m_old_rules`,
`m_refs`, `m_pred2slice`. Go builds a new `pdr.Context` per query in
`newConfiguredPDRContext` (fp/pdr_engine.go:786) and then decides whether to
keep the old one (D7).

## 3. Ownership and lifetime of each transformer and what it holds

Plugins are `alloc`'d at `register_plugin` time (constructors run before any
transform) and `dealloc`'d by `~rule_transformer` → `reset()` in the sorted
order (priority descending; `std::sort` order for ties, which g3 already
reproduces with `stdSortPlugins`). Within a plugin, members die in reverse
declaration order.

| Plugin (priority) | C++ AST-holding members (declaration order) | When C++ releases | Go counterpart and difference |
|---|---|---|---|
| `mk_coi_filter` (45000, 34990, 34960) | `vector<app*> m_new_tail` (raw) | nothing pinned | `ConePlugin` (coi_plugin.go): equivalent |
| `mk_interp_tail_simplifier` (40000, 34980, 34950; also nested in rule_unifier, mk_bit_blast::impl, mk_array_blast) | `th_rewriter& m_simp` = `ctx.get_rewriter()` (shared, context-owned); `rule_substitution m_rule_subst` = {`substitution m_subst` (apply cache `m_apply_cache`, pinned results `m_new_exprs`), `unifier m_unif`, `app_ref m_head`, `app_ref_vector m_tail`, `svector<bool> m_neg`, `rule* m_rule`}; `ptr_vector<expr> m_todo`; `obj_hashtable<expr> m_leqs`; `app_ref_vector m_tail`; `expr_ref_vector m_itail_members`; `expr_ref_vector m_conj`; `normalizer_cfg* m_cfg`; `normalizer_rw* m_rw` (own rewriter_tpl) | `rule_substitution::reset(r)` at the start of every `propagate_variable_equivalences` call (`m_subst.reset()` → `reset_cache()` frees the previous rule's apply results); `unify` calls `reset_cache()` after its two applies; `get_result`'s applies (new head, new tails) stay in `m_new_exprs` and in `m_head`/`m_tail` until the next `reset(r)` or `~mk_interp_tail_simplifier`. `m_tail`, `m_itail_members`, `m_conj` keep the last rule's tails until the next `reset()`/destruction. | `InterpTailSimplifierPlugin`: `propagate` makes `substitution.New` + `NewUnifier` per rule and closes both at return (**D1**); `heldSimplified` models `simp_res`/`itail` locals only. `NewInterpTailSimplifierPlugin` (non-shared) creates its own `TheoryRewriter` where C++ always uses the context rewriter (**D2**). |
| `mk_rule_inliner` (35000, 34970, 34930 ...) | `th_rewriter& m_simp` (context's); `rule_ref_vector m_pinned`; `rule_set m_inlined_rules`; `horn_subsume_model_converter* m_mc` (owned by the context converter chain); `rule_unifier m_unifier` = {`mk_interp_tail_simplifier m_interp_simplifier`, `substitution m_subst`, `unifier m_unif`}; `substitution_tree m_head_index`, `m_tail_index`; `substitution m_subst`; two `visitor`s | all at `~mk_rule_inliner`, reverse order: visitors, `m_subst`, `m_tail_index`, `m_head_index`, `m_unifier` (`m_unif`, `m_subst`, nested simplifier), `m_inlined_rules`, `m_pinned`. `rule_unifier::unify_rules` does `m_subst.reset()` per attempt, so the last unification's apply results survive to destruction (z3 trace: `[del] #34 #48` after the simplifier's `#32 #33`). | `RuleInlinerPlugin`: `unifier *RuleUnifier` (one `Substitution` per plugin, matches) and `simplifier` are created lazily on first use (C++ constructs them at registration) (**D3**); `Close` order: linear indexes, unifier, inlinedRules, pinned, modelRules, simplifier. C++ order: indexes/subst, unifier (its subst, then its simplifier), inlined rules, pinned. Model rules are taken out before Close (`TakeModelRules`), fine. |
| `mk_subsumption_checker` (35005, 34975, ...) | `rule_ref_vector m_ref_holder`; `func_decl_set m_total_relations`; `obj_map<func_decl, rule*> m_total_relation_defining_rules`; `obj_map<func_decl, obj_hashtable<app>*> m_ground_unconditional_rule_heads` | destructor | `SubsumptionCheckerPlugin`: `refHolder`, `totalRelations`, `totalDefining`; equivalent |
| `mk_quantifier_instantiation` (37000, always registered) | `expr_safe_replace m_var2cnst`, `m_cnst2var`; `ptr_vector`s; `m_funs` | destructor; `operator()` returns null when the param is off | g3 registers `noopTransformPlugin` when off (**D5**, harmless unless the constructor creates ASTs) |
| `mk_quantifier_abstraction` (38000, optional) | `func_decl_ref_vector m_refs`; `m_new2old`, `m_old2new`; `qa_model_converter* m_mc` | destructor | `QuantifierAbstractionPlugin`: `oldToNew` map; declarations pinned by the rule manager |
| `mk_array_instantiation` (34999), `mk_array_eq_rewrite` (34998) | pointers to src/dst rule sets, counters | nothing pinned | equivalent |
| `mk_bit_blast` (35000, always registered) | `impl`: `params_ref`; `mk_interp_tail_simplifier m_simplifier` (nested, uses the context rewriter); `bit_blaster_rewriter m_blaster`; `expand_mkbv m_rewriter` whose cfg holds `expr_ref_vector m_args, m_f_vars, m_g_vars`, `func_decl_ref_vector m_old_funcs, m_new_funcs`, `m_pred2blast` | destructor (`dealloc(m_impl)`): `m_rewriter` cfg vectors, `m_blaster`, `m_simplifier` in that order; `operator()` returns null when `xform.bit_blast` is off | `BitBlastPlugin`: `simplifier` built with `NewInterpTailSimplifierPlugin` (own rewriter, **D2**); `expansion` keeps `args/fVars/gVars` refs; when off g3 registers a noop (**D5**) |
| `mk_karr_invariants` (36010) | `context m_inner_ctx` (a whole datalog context); `obj_map<func_decl, expr*> m_fun2inv`; `ast_ref_vector m_pinned` | destructor; returns null when `xform.karr` is off | `KarrInvariantsPlugin`; noop when off (**D5**) |
| `mk_scale` (36030) | `expr_ref_vector m_trail`; `app_ref_vector m_eqs`; `obj_map<expr,expr*> m_cache`; `scale_model_converter* m_mc` | destructor; null when off | `ScalePlugin` (pendingPins etc.); noop when off |
| `mk_array_blast` (35999, unless quantify_arrays) | `params_ref`; `th_rewriter m_rewriter` (own); `mk_interp_tail_simplifier m_simplifier` (context rewriter); `defs_t m_defs` | destructor; null when off | `ArrayBlastPlugin`: own `rewriter` (right) but `simplifier` via non-shared ctor (**D2**) |
| `mk_magic_symbolic` (36020, optional) | none beyond the context | | equivalent |
| `mk_slice` (separate transformer) | `expr_ref_vector m_solved_vars`; `obj_map<func_decl,func_decl*> m_predicates`; `ast_ref_vector m_pinned`; `slice_proof_converter* m_pc`; `slice_model_converter* m_mc` | destructor at the end of the slice block; the interface then pins the predicate pairs in `m_refs` until the next query | `RuleSlicePlugin`; `PredicateSlices()` maps, `TakeProofConverter`; no `m_refs` analogue (part of D6) |
| `mk_coalesce` / `mk_unfold` (separate transformers) | `mk_unfold` owns a `rule_unifier m_unify` (nested simplifier + substitution) | both transformers are constructed before either runs and destroyed together at the block end (unfold first) | `RuleUnfoldPlugin` wraps a `RuleInlinerPlugin` created per round (**D3**) |

## 4. Concrete Go deviations

- **D1 (root cause of the named symptom).** `InterpTailSimplifierPlugin.propagate` builds and closes a `substitution.Substitution` and `Unifier` per rule. C++ keeps one `rule_substitution` per plugin instance; `reset(r)` per rule; the last rule's `get_result` apply results (and `m_head`, `m_tail`) survive until plugin destruction. Also the per-plugin `m_tail`, `m_itail_members`, `m_conj` vectors are reused across rules in C++ (their contents are released at the *next* `reset()`), while Go's locals die per rule; this only matters for ids that are free at the next creation, so handle it in the same step.
- **D2.** `NewInterpTailSimplifierPlugin` (used by `BitBlastPlugin` and `ArrayBlastPlugin`) allocates its own `TheoryRewriter`; every C++ `mk_interp_tail_simplifier`, nested or not, uses `ctx.get_rewriter()` (`th_rewriter& m_simp`). Only the `normalizer_rw` is per instance.
- **D3.** Construction time and destruction order of nested objects: C++ constructs `rule_unifier` (with its nested simplifier) inside `mk_rule_inliner`/`mk_unfold` constructors at registration; Go creates `unifier` and `simplifier` lazily. C++ destroys an inliner's members in reverse declaration order (indexes and local subst, then the unifier's subst and nested simplifier, then `m_inlined_rules`, then `m_pinned`); Go's `Close` closes the unifier before `inlinedRules` but the nested simplifier last. The unfold block constructs coalesce and unfold transformers together and destroys unfold first; Go builds and closes one transformer per stage.
- **D4.** `RuleTransformer.Apply` closes the superseded rule set after `finalizeTransformResult(next)`; C++ `dealloc(new_rules)` happens after `close()` of the new set and before `ensure_closed()`. Equivalent in AST terms; no change needed.
- **D5.** Disabled stages (`bit_blast`, `karr`, `scale`, `array_blast`, `instantiate_quantifiers`) are `noopTransformPlugin`s in Go, while C++ constructs the real plugins and lets `operator()` return null. Only matters if a constructor creates ASTs (`mk_karr_invariants::m_inner_ctx` is the one to check with `Z3_MK_BT_N` on a karr-off vector).
- **D6.** Cross-query retention. z3 keeps alive until the *next* query: `m_old_rules` (all rules incl. the synthetic query rule, from `check_reset`), `m_pdr_rules`/`m_spacer_rules` and `m_transformed_rule_set` (the transformed program), `m_refs` (sliced predicate pairs), and the `pdr::context`'s own rule copies. Go frees the transformed `prepared` set (`preprocessed.Close`) and the query rule (`removeQueryRule`) at the end of the query, and takes `sourceSnapshot` after preprocessing instead of copying `old_rules` before `mk_query`. Within one query the release order at function exit is: proof mode restored, `bg_assertion`, `query_pred`, `old_rules` (all after `solve`).
- **D7.** One `pdr::context` per `dl_interface` (reset by `check_reset`), versus `newConfiguredPDRContext` per query in Go, built *before* preprocessing. Only visible if context construction creates ASTs or solver state before the transforms run.
- **D8.** `get_background_assertion()` builds `mk_and(m_background)` per query (a new app when there are two or more background assertions) and holds it until function exit; verify what `e.background` does on multi-assertion vectors.
- **D9.** `scoped_restore_proof` around `update_rules`/`solve`: C++ `pdr::context::update_rules` runs `init_core_generalizers` (which toggles `PGM_ENABLED` when the classifier says the program is not propositional) *before* `init_rules`, so `pred_transformer::init_rule`'s `th_rewriter` emits `refl`/`iff` proofs; those are the `[mk-app] iff #1 #1` / `refl` lines missing from g3 in the sliced-trace bucket. The proof mode is restored only when `dl_interface::query` returns.
- **D10 (not pipeline, listed for completeness).** Spacer `pred_transformer::mk_assumptions` → `add_reach_fact` → `init_reach_facts` creates `asserted(true)` proofs in z3 (`[mk-app] #N asserted #1`) that g3's spacer context never creates. Nine logs stop there.
- **D11 (not pipeline).** PDR solve phase: z3 creates a unary `and` (`[mk-app] #183 and #55`) after a `(or ... (not query!0!5))` propagation line; g3 does not. Eight logs stop there.

## 5. Survey of the 55 vectors (full trace logs, pointers masked)

17 identical today: pdr_inductive_reachability, pdr_linear_trace,
pdr_missing_tail, pdr_model_generalizer, pdr_nonlinear_trace,
pdr_repeated_disequalities, pdr_variable_trace, propositional,
relation_representations, spacer_core, spacer_inliner_resolution,
spacer_slice_columns, fp_spacer_array_instantiation, fp_spacer_flexible_trace,
fp_spacer_karr, fp_spacer_magic, fp_spacer_skip_propagate. stdout matches z3
on all 55.

38 differ, by first-difference bucket (line of first difference in brackets):

- **A. Transformer-phase lifetimes (this plan, steps 1–4):**
  fixedpoint_pdr_bit_blast [60], fp_spacer_quantifier_abstraction [67],
  fixedpoint_spacer_coalesce_unfold [68], fixedpoint_pdr_sliced_compound_head
  [77], fp_spacer_array_blast [81], fp_spacer_array_eq_rewrite [86],
  fixedpoint_spacer_nonlinear_inlining [109; note the `and` arity differs, so
  part of it is an inliner shape difference], fp_spacer_mutual_inv [116],
  fp_spacer_bit_blast [128], fixedpoint_spacer_acyclic_inlining [201],
  fp_spacer_quantifier_instantiation [274], fixedpoint_pdr_background [540,
  second query], fixedpoint_pdr_integer [548, second query].
- **B. PDR init_rule under proof mode (step 5):** fixedpoint_pdr_sliced_trace
  [116], fixedpoint_pdr_sliced_compound_tail [136],
  fixedpoint_pdr_untransformed_nonlinear_trace [169], and probably
  fixedpoint_pdr_general_arithmetic_fragment [115: `Inv_0_0` vs `Inv_0_n`
  creation order in pred_transformer init].
- **C. Spacer init_reach_facts `asserted(true)` (D10, separate item):**
  fixedpoint_spacer_recursive_trace, fp_spacer_arith_inv,
  fixedpoint_spacer_datatype_state, fp_spacer_generalizers_inv,
  fp_spacer_linear_inv, fp_spacer_mincut_inv, fixedpoint_spacer_unfold_rules,
  fixedpoint_spacer_bv_state, fp_spacer_scale.
- **D. PDR solve phase (D11, separate item):** fixedpoint_pdr_scale,
  fixedpoint_pdr_untransformed_linear_trace, fixedpoint_pdr_convex_both,
  fixedpoint_pdr_convex_closure, fixedpoint_pdr_convex_interior,
  fixedpoint_pdr_arithmetic_inductive, fixedpoint_pdr_multicore,
  fixedpoint_pdr_arithmetic_invariant, fixedpoint_pdr_utvpi_fragment,
  fixedpoint_pdr_recursive_trace, fixedpoint_pdr_initial_trace.
- **E. Not PDR/Spacer:** fixedpoint_finite_relations uses
  `fixedpoint.engine datalog` (belongs with the datalog items).

## 6. Porting order, with the check for each step

Run the per-vector check with `$W/tdiff.sh <vector> <g3 binary>`
(`W=/var/tmp/g3-job-487c9ee8`); run the bucket check with the survey
script in section 7. Commit after each step that leaves its bucket no worse.

1. **`rule_substitution` as a plugin member (D1).** In
   muz/transforms/interp_tail_simplifier.go give `InterpTailSimplifierPlugin`
   a `ruleSubstitution` field = {`subst *substitution.Substitution`,
   `unifier *substitution.Unifier`, `head ast.ASTRef`, `tails []ast.ASTRef`,
   `neg []bool`, `rule *base.Rule`}, created in both constructors (C++
   constructs it in the plugin constructor). `propagate` starts with
   `reset(r)`: `subst.Reset()` (which frees the previous rule's apply results),
   `Reserve(1, vars)`. Keep `tryUnify`'s `ResetCache` after the two applies.
   Port `get_result` so the new head and new tails are produced by
   `subst.ApplyExpression` into the member `head`/`tails` refs (previous
   contents released there, as `m_tail.reset()` does) and are *not* released at
   the end of `propagate`. `Close` releases in C++ reverse member order:
   `neg`, `tails`, `head`, `unifier`, `subst` (whose `Close` runs
   `ResetCache` then `Reset`). Also hoist `m_tail`/`m_itail_members`/`m_conj`
   into plugin fields reused across rules if the next check still shows an
   early free.
   Check: `tdiff.sh fixedpoint_pdr_bit_blast` is empty; with
   `Z3_TRACE_DEL=1` on a g3del-style build, `[del] #32 [del] #33` move after
   `[mk-app] #47 query!0!bv!2 #1 #2 #1`. Expect fp_spacer_bit_blast and
   fp_spacer_mutual_inv to move or clear.
2. **Context rewriter for nested simplifiers (D2).** `NewBitBlastPlugin` and
   `NewArrayBlastPlugin` take the context rewriter and build their simplifier
   with `NewInterpTailSimplifierPluginShared` (fp/spacer_engine.go already has
   `e.contextRewriter()`); keep `ArrayBlastPlugin.rewriter` as its own
   `th_rewriter`. Check: fp_spacer_array_blast, fp_spacer_bit_blast,
   fixedpoint_pdr_bit_blast stay/become identical.
3. **Construction time and teardown order (D3, D5).** Construct
   `RuleInlinerPlugin.unifier` and `.simplifier` in the constructor (as
   `rule_unifier` is a member), and order `Close` as C++ destroys: linear
   indexes and local subst, unifier (its subst, then its nested simplifier),
   `inlinedRules`, pinned. Build `RuleUnfoldPlugin`'s resolver in its
   constructor; in `preprocessHornRulesWithStages` construct the coalesce and
   unfold transformers together before applying either and close unfold then
   coalesce. Replace `noopTransformPlugin` with the real plugin only where its
   C++ constructor is shown (by `Z3_MK_BT_N`) to create ASTs; otherwise leave
   the noops. Check: fixedpoint_spacer_coalesce_unfold,
   fp_spacer_quantifier_abstraction, fp_spacer_quantifier_instantiation,
   fixedpoint_pdr_sliced_compound_head, fp_spacer_array_eq_rewrite,
   fixedpoint_spacer_acyclic_inlining.
4. **Cross-query and end-of-query retention (D6, D7, D8).** In
   `queryRelationMkQuery`/`queryPDRWithSliceProof`/`querySpacer`: take the
   `old_rules` copy before adding the query rule; keep the query rule alive in
   the `pdrSourceRules`/spacer equivalent of `m_old_rules` instead of freeing it
   in `removeQueryRule` (it leaves the context rule set, it does not die); keep
   the transformed set (`prepared`) as the engine's `m_pdr_rules` and a
   `m_transformed_rule_set` copy until the next query replaces them; release
   `bg_assertion`, `query_pred`, `old_rules` after solve in that order; stop
   constructing a fresh `pdr.Context` before preprocessing when one exists
   (construct once, `Reset` on fresh rules, rebuild only on parameter change);
   build the background conjunction per query. Check: fixedpoint_pdr_background,
   fixedpoint_pdr_integer (second-query ids), fixedpoint_spacer_acyclic_inlining
   (end-of-query ids); rerun the whole bucket A list.
5. **PDR `update_rules` proof mode and init order (D9).** In pdr/context.go
   run the core-generalizer classification (toggle `ASTProofEnabled` when not
   propositional) before `init_rules`, keep proofs enabled through `solve`, and
   restore the previous mode when the query returns (`scoped_restore_proof` in
   the fp caller, not inside the context). Check that `pred_transformer`
   creates `Inv_0_0`-style o-index constants before `Inv_0_n`. Check:
   fixedpoint_pdr_sliced_trace, fixedpoint_pdr_sliced_compound_tail,
   fixedpoint_pdr_untransformed_nonlinear_trace,
   fixedpoint_pdr_general_arithmetic_fragment.
6. **Hand-offs (new TODO items, not this one).** Bucket C: spacer
   `init_reach_facts`/`mk_assumptions` `asserted(true)` proofs (nine logs,
   start with fp_spacer_arith_inv line 172, `Z3_MK_BT_N=130`). Bucket D: PDR
   solve-phase unary `and` (start with fixedpoint_pdr_arithmetic_inductive line
   467). Bucket E: datalog engine (fixedpoint_finite_relations).

After step 4 the expected state is bucket A identical (13 logs), after step 5
bucket B (4 logs); buckets C and D stay for their own items.

## 7. Tooling used (all read-only)

- `$W/tdiff.sh <vector> <g3 binary> [lines]`: first lines of the masked
  trace diff (z3 `build/z3` vs g3).
- `$W/z3del/build_trace/z3` with `Z3_TRACE_DEL=1` adds `[del] #id` lines;
  `Z3_DEL_BT=<id>` prints a backtrace at that id's deletion;
  `Z3_MK_BT_N=<n>` prints a backtrace at the n-th `[mk-app]`. Symbolize with
  `addr2line -e $W/z3del/build_trace/z3 -f -C -i` on the `(+0x...)` offsets.
- `$W/g3bt2` (built 22:38 today from HEAD, hook from `$W/build_g3bt2.sh`):
  `Z3_TRACE_DEL=1` for `[del]` lines, `G3_DEL_BT=<id>` for a Go stack at that
  id's deletion (count occurrences: an id is recycled, so pick the n-th `=== del
  #id` block). Rebuild with `$W/build_g3bt2.sh` after code changes.
- Bucket survey (python, run with `-I`; z3 and g3 paths at the top):

```python
import subprocess,glob,os,re,concurrent.futures as cf
Z='/mnt/oldrog/home/jaten/go/src/github.com/glycerine/g3/build/z3'; G=os.environ['G3']
tmp=os.environ['TMPD']; os.makedirs(tmp,exist_ok=True)
files=[f for f in sorted(glob.glob('/mnt/oldrog/home/jaten/go/src/github.com/glycerine/g3/pkg/cmd/g3/test_vectors/f*.smt2')) if os.path.basename(f).startswith(('fixedpoint_','fp_spacer_'))]
ptr=re.compile(r'0x[0-9a-f]+')
def log(b,f,t):
    out=os.path.join(tmp,os.path.basename(f)+'.'+t)
    try: subprocess.run([b,'trace=true','trace_file_name='+out,f],capture_output=True,timeout=60)
    except subprocess.TimeoutExpired: return None
    return ptr.sub('PTR',open(out).read()).split('\n')
def job(f):
    a,b=log(Z,f,'z'),log(G,f,'g')
    if a is None or b is None: return f,None
    n=next((i for i,(x,y) in enumerate(zip(a,b)) if x!=y),min(len(a),len(b)))
    return f,(a==b,n,a[n:n+2],b[n:n+2])
with cf.ThreadPoolExecutor(8) as ex:
    for f,r in ex.map(job,files):
        print(os.path.basename(f),'IDENTICAL' if r and r[0] else r and ('@%d z3=%s g3=%s'%(r[1],r[2],r[3])))
```

Run as `G3=$W/g3curNN TMPD=$W/survey python3 -I survey.py`.
