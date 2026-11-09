# Fidelity TODO (statistics and behavior vs. C++ Z3 4.7.1)

Found while matching `-st` output against `build/z3` on
`pkg/cmd/g3/test_vectors`. Work these in order.

1. **DONE: Remove the early scope pop after search.** `SMTKernel.search` pops
   the search scopes in a deferred function as soon as a check finishes.
   C++ `context::search` leaves them in place; `pop_to_base_lvl` runs at the
   next assert, push, pop or check. The early pop deletes theory variables
   and pivots once more (qflia_solver_pb_sat_pipeline: 97 pivots vs 96).
   New divergence seen while testing: on pigeonhole PHP(4,3) with
   `smt.max_conflicts=0`, repeated `check-sat-assuming` calls turn unsat on
   check 9 in z3 and check 10 in Go.

2. **DONE (see pkg/SEQ_PORT.md): Port `theory_seq::final_check_eh` faithfully.** Logics missing from
   `setup::setup_logic` (QF_NIA, ALL, ...) use `setup_unknown`, which
   registers theory_seq. Its final check draws from the context random
   generator in `reduce_length_eq` and `branch_variable` even with no
   sequence constraints. Go's sequence final check is a different design
   and does not draw, so later random choices diverge
   (nonlinear_nested_integer_row_aliases, bv_integer_conversion_scopes).

3. **AST lifetimes and id recycling.** z3 frees each `expr_ref`
   temporary on scope exit and recycles its id (LIFO), so later terms take
   different ids, and mon_lt orders sum monomials by id
   (lia2pb_model_scopes: arith-add-rows 3 vs 5, pivots 7 vs 6). Compare the
   AST creation logs with `trace=true trace_file_name=FILE` on both
   binaries (the `[mk-app] #id` lines). Fixed so far:
   - eq2ineq creates the `>=` atom before the `<=` atom, as the reference
     build (GCC, right-to-left argument evaluation) does;
   - the kernel no longer builds `(not atom)` result literals after every
     satisfiable check; get_assignments and the other getters build them
     on the first query. (This exposed a pdr test that reused an
     unowned `(not q)` handle; the test now holds a reference.)
   Tried and reverted: releasing mk_le_ge_eq_core's cancel_monomials
   results (new_arg1, new_arg2) on return. In g3 a hash-consed node with
   reference count 0 stays live until an inc/dec pair, so the release
   deleted nodes that callers still held without owning (pdr failed).
   Doing this needs callers to own their handles first.
   Also fixed (measured by comparing `[mk-*]` AST creation logs on every
   test vector; 73 -> 95 of 659 identical):
   - mk_default_tactic constructs its alternatives in GCC's right-to-left
     argument order (smt, qffplra, qffp, nra, lira, ..., qfbv), so the
     tactic constructors' ASTs (bit2int, bv1-blaster numerals) get z3's
     ids. With that, the two-check-sat-using example below matches.
   - context::init creates mk_true_proof() under proofs, after the context
     members (asserted_formulas, quantifier_manager's qi cost parser).
   - lackr: a single argument equality is the lhs itself (no 1-ary and),
     and fun2terms_map/app_set iterate in z3's hash-table order
     (ackermannization/obj_hashtable.go), which picks f!N and the lemma
     order.
   - cmd_context resolves `=` to the basic plugin's OP_EQ declaration
     instead of calling mk_eq, so a parsed Boolean equality stays `=`
     (g3 built iff). All-stats exact 583 -> 598; AST logs 98 identical.
   - asserted_formulas releases m_formulas first to last (vector shrink
     and destructor), each justified_expr formula before proof; g3
     released last to first, proof first (add_bounds_fractional_real
     freed its goal formulas in reverse). AST logs: 99 identical.
   - mk_simplify_tactic is clean(simplify_tactic): the cleaner tactical
     destroys the rewriter (and its cache) right after each apply. The
     default strategy, mk_default_tactic and the sls factory built a bare
     simplifier, whose cache kept the simplified formulas alive (AST logs
     101 -> 103 identical).
   2026-11-08 (AST logs 103 -> 123 identical):
   - elim-uncnstr's m_fresh_vars and m_cache_domain are app_ref_vectors:
     destroyed in reverse member order, each releasing forward (g3 used
     one LIFO frame). This fixed the get-model (> x 2) lead.
   - model printing wraps a sort shown with an indent in mk_indent
     (cmd_context::display(out, s, indent)).
   - the kernel's captured literal-result snapshot (a Go addition; z3
     queries the live context) is released on pop, so popped atoms die.
   - smt2 process_app builds the body before the "(" fname header (GCC
     right-to-left argument evaluation).
   - z3 writes smt-context records ([attach-enode], [assign], [push],
     [pop], [begin-check], [conflict], [resolve-lit], [resolve-process])
     to the same stream; g3 now does too, and creates context::init's
     true/false enodes eagerly. Compare whole logs with
     /var/tmp/g3-job-487c9ee8/fullcmp_log.py (pointer values masked):
     122 of 658 identical. Not ported yet: [new-match], [instance] and
     [end-of-instance] (see item 19).
   Later the same day (full logs 122 -> 238 identical, stdout identical
   to z3 630 -> 640 of 660):
   - display paths release their format_ref documents (cmd_context
     display, model_v2, sat model converter); GCC right-to-left order in
     mk_seq1/3/4/5, smt2 process_app, bv mk_eq_concat and mk_leq_core;
     bounded_int2bv and bv2int expr_ref scopes; act_cache releases in
     chashtable order (ast/ast_activity_chashtable.go);
     is_model_available() probes before each get_model.
   - bugs found on the way: trace=true broke arithmetic coercion
     (nested trace acquisitions); pb2bv in QF_LIA failed to build a model
     (func_interp arity); model function definitions now use pp_funs
     formats; get-proof's display_smt2 indent quirk; unknown-sort error
     context/position; NRA numerals are Real (get_sort_names side effect).
   Tools: /var/tmp/g3-job-487c9ee8/fullcmp_log.py (whole log),
   firstdiff.py (buckets first differences), outcmp.py/outlist.py
   (stdout vs z3). build_dbg/ is an untracked z3 build that logs
   [del] #id records (Z3_TRACE_DEL=1) from a patched copy of ast.cpp;
   pair it with a temporary Go hook in ast_core_delete.go.
   Later still: full logs 238 -> 268 identical. Updated quantifiers log
   [mk-quant]; real diff logic's zero is Int; mk_le_ge_eq_core's
   expr_ref temporaries are released (cancel_monomials, elim_to_real,
   is_neg_poly/div_polynomial reassignment) and moved monomials sorted
   by mon_lt; fpa mk_is_nan order and convert_bv2fp's extract shape.
   Rebuild the [del]-logging g3 with /var/tmp/g3-job-487c9ee8/build_g3del.sh.
   DONE in fpa: the bv2fpa model conversion released a group of terms in a
   different order: its obj_maps are now ast.ObjMap (see below).
   Next buckets: '[mk-app] vs [mk-var]' (fixedpoint, see item 20),
   not/iff/= id differences (bit-blaster and SAT ownership), and
   '[mk-quant]' ordering.
   The earlier example, `(check-sat-using smt)(declare-const z
   Int)...(check-sat-using smt)` with x+y=7 and x>=0, now has identical
   AST logs: the apparent destruction-order difference came from the
   tactic construction order above, not from smt::context teardown.
   DONE this round (full logs identical 268 -> 300): preprocessing proof
   lifetimes (rewriter main_loop, nnf_cnf r1/pr1, per-formula simplify_fmls
   locals), ackermannization (ackr locals, ~ackr_info member order),
   theory_bv (Close frees bit atoms newest first; expand_diseq's diff is a
   loop local, not a shared temporary), cmd_context restore_assertions
   (assertions forward, then names), array-ext refused at internalization.
   Tools: $CLAUDE_JOB_DIR/tmp/delcmp.sh TEST [rebuild] prints the first real
   difference with both deletion logs; mkbt.sh TEST LINE gives g3's creation
   stack for a log line; classify.py buckets all differing vectors.
   DONE (300 -> 315): goals store forms/proofs/dependencies in a port of
   parray_manager (tactic/goal_parray.go), so a goal copy such as
   or_else's `goal orig(*in)` keeps values replaced afterwards alive until
   it dies; theory_bv's bit-blaster keeps flat and/or gates (only
   bit_blaster_rewriter disables them in this fork); pb2bv builds monomial
   ites right to left and ~imp releases bits in obj_map order; the
   bit-blast rewriter keeps m_in1/m_in2/m_out until the next operation.
   DONE (315 -> 332): smt kernel teardown follows ~asserted_formulas member
   order (macros, m_formulas, defined names, substitutions, rewriter);
   get-value evaluates each term with its own model_evaluator (model::eval);
   distribute-forall pins tmp_q per iteration.
   Pattern to look for next: a z3 expr_ref/quantifier_ref local that g3
   leaves unpinned (a node nothing ever references is never freed, so it
   outlives the source's temporary), and member vectors that z3 resets at
   the start of the next operation while g3 releases them at the end.
   Remaining big buckets: `not`/`and`/`=` id offsets in bit-blasting
   vectors (theory_bv's shared t.temporary list approximates per-function
   expr_ref_vector locals), Real/Int numeral ids (arith), query!0 (item 20),
   [new-match] (item 19).
   DONE: pops and smt::context::flush (Close unwinds through popTo) undo
   mk_bool_var and mk_enode entries newest first across both stacks, as
   z3's single m_trail_stack does. Related: z3's relevancy propagator keeps m_relevant_exprs as an
   expr_ref_vector, so its terms live until it unmarks them on pop. g3's
   propagator holds no references; the ported unmark_relevant_exprs
   TRACE (datatype_update_equality_scopes) shows terms already deleted
   when g3 unmarks them, so g3 frees them earlier than z3 does.
   DONE (332 -> 345 identical full logs; run-to-run nondeterminism over all
   660 vectors: 6 files -> 0): Go maps that z3 iterates as obj_map /
   core_hashtable now use ast.ObjMap (ast/ast_obj_map.go: the
   core_hashtable layout, insert_if_not_there with a custom hash/eq, Num
   values). Converted: fpa2bv/bv2fpa caches, ast_translation cache,
   expr2var (nlsat mk_model walks variables in mk_inv order), bit-blaster
   const2bits (fixed a nondeterministic lackr model) and its m_keys/m_values
   forward release, reduce-args (decl2arg2funcs, arg2func with
   arg2func_hash_proc), the model finder's instantiation_set
   (m_elems/m_inv), proto_model m_aux_decls/found_aux_fs, model_core
   m_interp/m_finterp layouts (teardown, copy, translate, cleanup),
   aig max_sharing m_saved and expr2aig, grobner m_var2weight.
   distribute-forall's cache is an act_cache. Two lifetime fixes surfaced
   once teardown followed table order: proto_model::mk_model moves the
   func_interps into the final model, and model_generator's m_asts pins
   stay alive until the next reset (released forward).
   Check for nondeterminism with /var/tmp/g3-job-487c9ee8/ndscan.sh (three
   runs per vector, compares trace logs).
   Also DONE (345 -> 360): array already_diseq builds m_is_diseq_tmp per
   select pair; MBQI check(q) pushes before assert_neg_q_m, restricts each
   skolem as it is made, pops before its skolems die, and its universe /
   inst-set restrictions hold their equalities unpinned (ptr_buffer, so the
   rewriter does not cache them) walking declarations last to first; a
   rejected candidate proto model lives until the next mk_proto_model;
   qi_queue m_instances release forward; pp_attribute builds right to
   left; theory_str's len-0 axiom uses ctx.mk_eq_atom in a gate context.
   Also DONE (360 -> 385): expr_substitution and goal2sat (dep2asm,
   assumption order) follow obj_map order; NNF caches are act_caches; the
   ackr model converter and the const-case multiplier (raw ite per bit)
   follow the source; search() drops the previous model at start
   (scoped_mk_model); theory_str creates its nested seq_expr_solver at
   setup, as theory_str::init does.
   Open leads: qsat runs one more round than z3 in
   basic_graph_mixed_model_refusal (two extra contexts at the end);
   datatype_cycle_proof's occurs-check th-lemma has an extra antecedent
   (z3 lists one equality proof, g3 two); the model finder's auf_solver
   key2node maps (pair-hashed) are still handle-sorted.
   DONE since: m_A_f_is layout (pair_hash) drives collect_partial_funcs and
   the partial func_decl_set; m_ng_decls is a func_decl_set (pre-order
   fill); the hint solver's sets, backtrackable residue/satisfied sets and
   m_fs follow obj_map order (set_interp walked a random Go map); qe subst
   and nlarith caches release in m_trail order; substitution_tree,
   bv-slice and muz/base (rule-manager predicates, rule-set outputs,
   parked predicates) release deterministically.
   DONE: the relation manager walks m_relations' layout (reset,
   restrict); interpreted filters decode by variable index; the muz/fp,
   pdr, spacer, tab, transforms and duality map loops are deterministic
   (handle order, insertion order or source layout).
   DONE: FuncDeclSet is an obj_hashtable (for_each_expr post-order fill),
   func_decl_dependencies refs and reset follow table order; duality's
   remaining maps, the MPS option setter and debug expressions are
   deterministic. The mapscan list is now empty (symbol-keyed and
   duality loops use name/number order rather than z3's symbol-hash
   dictionaries).
   Open: g3's single generated model stands for both m_proto_model and
   m_model (scoped_mk_model drops both at search start and builds m_model
   from the last proto at search end); releasing the stale model after
   the next build instead made 15 logs worse, so the split needs care.
   Still open in this area: model universes (m_usort2universe is an
   obj_map; g3 keeps insertion order), cmd_context m_object_refs (a symbol
   dictionary; only debug commands use it), and the remaining mapscan list
   ($CLAUDE_JOB_DIR/tmp/mapscan/out.txt): muz/rel, duality, muz/fp,
   qe (bool_atom_index, nlarith weights, branch cache), tactic/core
   pb_preprocess, bv_slice, substitution_tree. Those loops are
   deterministic in practice only if the map has one entry; each needs its
   z3 container checked (obj_map -> ast.ObjMap, vector -> insdict).
   DONE: pb-preprocess is re-ported (tactic/core/pb_preprocess_simplify.go,
   tactic/pb_preproc_model_converter.go): m_vars in table order with
   next_resolvent, one classification per round, z3's resolve and
   subsumption, the m_r rewriter. Recorded deviations from 4.7.1 bugs that
   can be unsound: positions are goal indices (the source stores m_ge
   indices), and subsumption rebuilds args2/coeffs2 per candidate.

4. **DONE: get-consequences algorithm.** smt_solver::get_consequences_core
   is now context::get_consequences (smt_consequences.cpp: antecedent sets,
   forced disequalities in obj_map order, bounded_search rounds with
   inc_limits, delete_unfixed, extract_fixed_eqs), on a search split into
   begin/boundedSearch/end with resolve_conflict extracted. bounded_search no
   longer builds the model (search does, as scoped_mk_model). The
   get_consequences vector matches z3, including the consequence order.

5. **DONE: integer_auto_deep_ite.** Now matches z3 exactly. The last
   difference was arith-assert-diseq (54 vs 4): push_new_th_diseqs walks
   the source parent list (m_parents), which keeps only parents that stay
   congruence roots through merges; g3's parent list keeps every parent,
   so it re-sent one disequality for each congruent copy of an equality.
   Enodes now also keep that source list (sourceParents).

6. **DONE: Typed tactic statistics.** `Tactic.CollectStatistics()` returned
   `map[string]float64`; C++ `collect_statistics(statistics&)` keeps uint
   and double values apart. Go prints `5.00` where z3 prints `5` and cannot
   sum duplicate keys the way `statistics::display_smt2` does
   (84 implementers). Now `CollectStatistics(*statistics.Statistics)`,
   as `collect_statistics(statistics&)`: implementers add uint and double
   values, combinators pass the object down, and tactic2solver collects
   into its result and its own statistics. nlsat reports the five counters
   of `solver::collect_statistics`; horn keeps only the engine's.

7. **DONE: SAT solver statistics.** qflia_solver_pb_sat_pipeline now
   matches z3, including the SAT `mk bool var`, since tactic statistics
   are typed and summed (item 6).

8. **DONE: `(apply ...)` goal display.** Go omitted
   `:precision precise :depth N`: `apply` calls `goal::display(ctx)`, the
   ast_printer_context overload, which prints them.

9. **DONE: Clause storage and del-clause.** `del-clause` and `mk-clause`
   count as the source does (binary clauses that live only in the watch
   lists are not counted when deleted; a two-literal clause stored as a
   full clause counts only in `mk-clause`). Binary lemmas now follow
   use_binary_clause_opt: a lemma is refused only at relevancy level 2 or
   more and may be stored above scope 0 (base level 0, atoms internalized
   at level 0), and its watches persist across pops as in the source. Go
   still keeps one clause list where C++ has `m_aux_clauses`/`m_lemmas`;
   the pop and lemma-preservation rules applied to it give the source's
   deletions and counts.

10. **RESOLVED (user confirmed): Hung test: parsers/smt2
    TestSMT2GeometricIntegerOptimizationCrossesBooleanBranches/maximize.**
    Documented by `pkg/parsers/smt2/log.hung.red` (timed out after 10
    minutes in `opt.(*ArithmeticSolver).optimizeArithmetic`, rechecking
    `CheckSat` forever). Cause: with search scopes kept after a check, Go's
    geometric optimizer built its `mk_ge` bound atom before `push()`, whose
    `pop_to_base_lvl` deleted it, so the bound asserted nothing. Fixed in
    aa4ab5c8c by following `optsmt::geometric_opt` (assert the
    `update_lower` blocker; build `mk_ge` only after the push). The log's
    stack (CheckSat at arithmetic_solver.go:510) is from the intermediate
    working tree before that commit. At aa4ab5c8c the test passes in 0.02s.
    Run it only with a short `-timeout` (e.g. 60s) until this has stayed
    fixed through a full `make`.

11. **DONE: Extra SMT contexts (first golden-trace divergence).** On
    sequence_helper_inventory block 12, z3 creates 17 `smt::context`s
    (count of `creating boolean variable: 0` records with
    `-tr:mk_bool_var`); g3 created 52, now 27.
    - DONE: theory_seq's `seq_expr_solver` kernel. g3 installs theories when
      a solver is created, where z3 registers them at the first check/push
      (`setup_context`); building the kernel in `Initialize` made every
      constructed solver build one. It is now built in `Setup`.
    - DONE: `mk_lra_tactic` falls back to `mk_qe_tactic` (qe_tactic.cpp);
      g3 used QE2 (qsat-based `mk_qe2_tactic`), whose kernels were 10
      extra contexts and added `mk-bool-var` 4 under logic ALL (27
      vectors). It now uses qe.QuantifierTactic. That port of
      qe_tactic.cpp leaves a quantifier in place when no plugin
      eliminates it, as the source does; the gaps in the expr_quant_elim
      plugins it uses are item 18.
    - DONE: `qe::qsat::~qsat` calls reset(), creating two kernels at
      teardown; QSATTactic.Close now does too. Block 12 creates 17
      contexts on both sides.
    - DONE (found by the record comparison on block 12):
      - context::mark_as_relevant propagates relevancy right after
        marking (smt_context.h); g3 only marked.
      - add_eq propagates relevancy from root to root (is_relevant(r1) ->
        mark_as_relevant(r2) and vice versa) before the merge.
      - m_atom_propagation_queue: assign_core queues a relevant atom when
        it is assigned, relevant_eh queues an assigned atom when it
        becomes relevant, propagate_atoms drains it in that order, and it
        is reset by pop_scope and resolve_conflict. g3 processed newly
        relevant atoms immediately, ahead of the queued ones.
      - set_merge_tf uses push_eq (queued), not an immediate add_eq.
      - internalize_theory_atom sets the enode flag for an atom the theory
        gave an enode (theory_seq's str.in.re), also in a gate context.
      - assert_default justifies asserted literals with a
        justification_proof_wrapper even without proofs.
      - family ids: cmd_context registers its plugins in init_manager_core
        order (arith, bv, array, datatype, seq, pb, fpa, datalog_relation)
        after pdecl_manager has created the datatype family, so seq is 9
        and bv 7 as in z3.
      All-stats exact went from 570 to 578 of 658; verdicts unchanged.
    - DONE: the last block-12 difference was propagate_th_eqs and
      propagate_th_diseqs, which stop at an inconsistency and reset their
      queues; g3 kept delivering theory equalities after a bv merge_eh
      conflict, and theory_arith's adapter created atoms z3 never made.
      Block 12 now matches z3 at every golden site and is the default
      golden vector (test_vectors/seq_regex_length_disequality.smt2).
      All-stats exact: 580 of 658.

12. **Golden trace coverage.** 423 of 660 test vectors now match the z3
    trace build at every golden site (assign_core, decide, add_eq_detail,
    mk_bool_var, the seq records, and the conflict records with ids
    masked); they are TestTraceGolden's default vectors
    (cmd/g3/test_vectors/golden_trace_vectors.txt). Fixes found by the
    failing vectors so far (423 -> 482 matching):
    - model_checker::assert_neg_q_m makes its Skolems with
      mk_fresh_const(nullptr, s), named k!N; g3 named them mbqi!N.
    - when setup_unknown(st) picks a profile without theory_seq, z3 never
      runs theory_seq::init and so never builds the seq_expr_solver
      kernel; g3's eager seq placeholder built it anyway (one extra SMT
      context in every check-sat-using smt under logic ALL).
    - instantiate_default_as_array_axiom makes mk_default(arr) before the
      epsilon arguments and f(epsilon...) (the other default axioms build
      the value first), which fixes the ids and the equality's argument
      order (485 matching; AST logs 101 identical).
    - conflict_resolution keeps justification marks and the processed-
      equality set for a whole resolve (until finalize_resolve's
      unmark_justifications(0)), so a justification or equality already
      explained adds no antecedents again; g3's expander cleaned up after
      every expansion and re-reported them (489 matching).
    - Boolean class propagation is now a port of propagate_bool_var_enode
      (walk from the assigned node's successor, only if it is the root,
      the root is irrelevant, or the root has another value; mp_iff
      justifications) and of propagate_bool_enode_assignment(_core),
      which add_eq runs before merging the class rings. g3 scanned the
      merged class from the root (495 matching).
    - simplify_aux_clause_literals sorts an auxiliary clause's literals
      by index before simplifying (std::sort), which fixes the order and
      the watched literals; g3 kept the given order. All-stats exact
      598 -> 606 of 659. (mk_gate_clause's proof is built before the
      sort, so a gate clause's proof fact lists the literals in gate
      order.)
    - (fixed) mk_clause wraps the clause's justification in a
      unit_resolution_justification over the removed false literals and
      builds the proof lazily; g3 built the unit-resolution proof eagerly
      under proofs and kept a bare proof wrapper.
    - theory_seq's assumption(n1, n2) has lit = null_literal; g3's
      equality assumptions left the zero SMTLiteral, which is the true
      literal, so every seq equality dependency added a spurious `true`
      antecedent to its propagations and conflicts.
    - theory_bv kept each bit vector's bits alive through an assembled
      mkbv term, an AST z3 never makes (it shifted later ids); it now
      references the bit expressions directly (518 matching).
    Former lead (bv_fixed_cache_scopes): at level 0 z3 propagates
    (= ((_ bit2bool 0) x) ((_ bit2bool 0) y)) with a justification whose
    antecedents are the two bit literals; g3 assigns it with a proof
    wrapper (no antecedents). Cause: mk_clause wraps the clause's
    justification (here the proof wrapper of mk_gate_clause) in a
    unit_resolution_justification over the removed false literals; with
    a proof g3 instead builds the unit-resolution proof eagerly and keeps
    the bare proof wrapper (smt_internalizer.go addClause...).
    Many remaining equality-order differences come from AST ids
    (mk_eq_atom orders by id), i.e. item 3.
    The remaining vectors show where g3 still diverges; run them with
    G3_GOLDEN_VECTORS=<name>. About half still differ in the number of
    SMT contexts (g3 more in most): compare the context-creating call
    sites, as done for the two fixes above.
    Port more TRACE records into Go at the
    sites z3 traces (add each to `goldenTraceSites` in
    cmd/g3/trace_golden_test.go). Ported so far: assign_core (with the
    justification display), add_eq_detail, decide, mk_bool_var, mk_enode,
    propagate_atoms, propagate_relevancy (propagate, relevant_eh and
    unmark_relevant_exprs), propagate_relevancy_to_args, and seq add_axiom,
    deque_axiom and get_automaton.
    Fixed trace formats: theory_bv prints booleans as 1/0 (ostream bool)
    and internalize_term/internalize_atom use mk_bounded_pp.
    The `-tags g3trace` smtcore run passes completely: a test writer without
    an owner no longer panics, and the theory_arith path of the equality
    adapter (smt_arith_eq_adapter_source.go) now emits mk_axioms's TRACE
    records (lines 86-226) in the source order.
    Remaining: the LRA path of the adapter (smt_arith_eq_adapter.go) is a
    different algorithm (it derives the bounds by internalizing the
    equality); z3's theory_lra uses the same arith_eq_adapter::mk_axioms.
    Switching LRA to the source path changes LRA behavior, so it needs its
    own stats comparison.
    Ported: the conflict tag in conflict_resolution (initialize_resolve
    403/411, process_antecedent 314/335, resolve 479/487/573,
    finalize_resolve 432/439/462, resolve 499 processing consequent and
    507 clause detail): all of conflict_resolution's conflict-tag
    records. On block 12 all 148 match z3 apart from AST ids (indices,
    levels and variables agree). These records print AST ids; they are in goldenIDMaskedSites
    (compared with `#<digits>` masked) and the conflict tag is a default
    golden tag, so TestTraceGolden checks them on block 12. They already show a difference
    on block 12: in the core conflict for (not (= x "abcc")) (an
    eq_propagation justification), z3's antecedents start with the
    "abcc" equality and include one more level-0 literal; g3's e-graph
    explanation visits the equalities in another order. Cause: g3's
    addEquality refused to merge two classes recorded as disequal and set
    its own SMTDisequalityConflictJustification. z3's add_eq has no such
    check: it merges, and reinsert_parents_into_cg_table assigns the
    false equality atom true with an eq_propagation_justification, which
    is the conflict. The check, the justification type and the tests of
    that mechanism are removed; the conflict records on block 12 now
    match z3 except for AST ids. (The disequality table still answers
    the checker's is_diseq queries; z3 answers those from the congruence
    table.)
    Next candidates: seq final_check_eh
    (theory_seq::display), the arith tags, and the remaining seq records
    (add_accept2step, propagate_*, solve_eq, simplify_eq, reduce_eq).

13. **DONE: conflict counting.** z3 counts `conflicts` in
    `context::resolve_conflict`; g3 counted when a conflict was set, so a
    conflict found where z3 never resolves one (e.g. while internalizing
    an unsat assertion) counted 1. The count now happens in the resolver.
    It was the only differing statistic in 50 vectors.

14. **Inner-solver and outer `mk-bool-var`.**
    - DONE: the outer context of seq_negative_symbolic_mixed now matches
      z3's statistics exactly (fixed by the relevancy and atom-queue work
      in item 11).
    - DONE: the seq_expr_solver kernel had no theory setup in g3 (only the
      lazily installed arith and seq plugins). kernel(m, fp) runs
      setup_context at its first push, and with no logic that reaches
      setup_unknown(), which installs every theory; g3 now installs the
      retained setup with an empty logic, so the bit-vector theory blasts
      the minterm constants as in z3. The minterm constant is named "x"
      (sym_expr_boolean_algebra::is_sat), not "seq-automaton".
    - DONE: the minterm queries match (x!0..x!16 as in z3). The label
      algebra is sym_expr_boolean_algebra (mk_and, mk_not, is_sat), and
      labels are refcounted like sym_expr: they own m_t/m_s and are freed
      where z3 drops the last move or ref_t (generate_min_terms locals,
      min_terms resets, mk_product's ab and pruned mvs, re2aut/seq2aut
      scoped_ptr intermediates, the compressed eautomaton's m_delta_inv
      then m_delta destruction).
    - DONE: the seq_expr_solver kernel builds a proto model on FC_DONE
      (m_fparams.m_model), and a kernel keeps its last model until the
      next mk_proto_model (model_generator::reset) instead of freeing it
      when a check begins. theory_bv's pop releases bit2bool atoms last
      created first.
    - DONE: seq_negative_symbolic_mixed's AST log is identical to z3's
      (all 24052 lines; it diverged at line 80). The last gaps were:
      context::is_diseq holds m_is_diseq_tmp's (= n1 n2) until the sort
      changes or flush; a pop undoes mk_enode and mk_bool_var newest first
      across both stacks (one m_trail_stack in the source); and the seq
      port evaluated nested AST-building arguments left to right where GCC
      goes right to left (add_axiom(~mk_eq(..), mk_seq_eq(..)) and 27
      other sites, found with a go/ast scan for calls with two or more
      mk* arguments; the other packages' error-returning APIs already
      sequence their constructions).
      A deletion-logging z3 (z3del: ast_manager::delete_node prints
      [del] #id, act_cache::insert prints [act], and Z3_DEL_BT/Z3_MK_BT_N
      print a backtrace) is rebuilt from a private copy of src and
      build_trace, outside the repository.
    - DONE: rewriteStringInRegex is a port of mk_str_in_regexp: it uses
      the compressed re2automaton result, both is_sequence overloads (the
      expression walk visits right operands and last characters first),
      add_next, plain mk_and/mk_or, and u_map frontiers that iterate in
      core_hashtable order (it walked a Go map before, so its AST order was
      nondeterministic).

15. **DONE: quantifier statistics.** `context::collect_statistics` adds
    `m_qmanager->collect_statistics` (qi_queue: quant instantiations, lazy
    and missed instantiations, min/max missed cost); g3 never reported
    them. Theories are now also collected in registration order, not map
    order. All-statistics exact 515 -> 541.

16. **DONE: atom queue decided at assignment.** `assign_core` queues a
    literal for atom propagation only if it is already an atom (and the
    relevancy test passes) when it is assigned. An equality that
    `internalize_eq` assigns while creating its enode (both sides already
    in one class) is assigned before `set_eq_flag`, so it is never
    queued. g3 tested at propagation time and also queued non-atoms;
    it now records the decision with the assignment. All-statistics exact
    541 -> 555.

17. **DONE: proof mode uses the plain SMT solver in the SMT-LIB front
    end.** `read_smtlib2_commands` installs
    `set_interpolating_solver_factory(mk_smt_solver_factory())`, and
    `produce_interpolants()` is a synonym of `produce_proofs()`, so with
    proofs `mk_solver` builds a plain SMT solver, not the strategic one.
    g3 always built the strategic solver. All-statistics exact 555 -> 567;
    one cmd/g3 test now expects z3's `unknown` answers.

18. **OPEN: expr_quant_elim output order (not a correctness gap).**
    Checked: errArithmeticBranchUnsupported (qe/quantifier_tactic.go) is
    reachable only without a QE context (resolveCooper with |b| > 3 and
    no qeConfig); the qe tactic always sets one, so integer resolution
    with large coefficients succeeds. Example:
    `(assert (exists ((x Int)) (and (<= (* 5 x) y) (>= (* 7 x) z))))
    (apply qe)` gives an equivalent formula on both sides, but the
    disjuncts come in a different order (z3: cases z = 0, 2, 4, 3; g3:
    3, 0, 4, 2) and z3 keeps both (+ (* (- 7) y) (* 5 z)) and
    (+ (* 5 z) (* (- 7) y)). For an upper bound above 1, z3's mk_big_or
    uses mk_big_or_symbolic_blast (a bounded bit-vector variable that the
    QE context enumerates with blast_or); compare g3's symbolic
    expansion in qe/arithmetic_qe_resolve_cooper.go with
    i_solver_context::blast_or.
    DONE (578fcc0ae): the example also showed a printer bug. Goal forms
    print with ast_smt2_pp indent 2 (mk_indent), so nested-let
    continuation lines are indented like z3's.
    FINDING: z3's blast_or is m_qe.eliminate_exists(1, &z_bv, fml, ...),
    the full qe.cpp quant_elim search with the bv plugin. Its branch order
    is model-guided (search-tree assign from the SAT model), which gives
    cases 0, 2, 4, 3. g3 calls eliminateQuantifierBitVectorSearch
    (qe/quantifier_tactic_arith_search.go), its own scalar search and not a
    port of quant_elim_plugin. Matching the order means porting qe.cpp's
    search tree (quant_elim_plugin, search_tree, model-based branch
    selection) and routing blast_or through it. That is a large port, like
    item 19.

19. **OPEN: E-matching is not a port of z3's MAM.** smtcore/smt_mam.go is a
   direct congruence-aware matcher over context enodes, not mam.cpp's
   code trees, inverted path indexes and backtracking interpreter. Match
   discovery order, and so instance order and the quantifier statistics,
   can differ from z3. Port mam.cpp (compiler, code trees, interpreter,
   inverted path index) for the instantiated pattern shapes.
   DONE meanwhile: the trace stream has [new-match] (add_instance; MBQI
   and quick-check instances list no used enodes, e-matching lists each
   top-level enode then nested application matches in discovery order),
   [instance] (with the quant-inst proof id under proofs) and
   [end-of-instance]. Full logs 351 -> 360 identical. Known gap: for a
   multi-pattern z3 can reach the second atom without listing it
   (lazy_multipattern_scopes), which needs the interpreter itself.

20. **DONE (e36db5524): fixedpoint rules are flushed lazily.** AddRule queues; the
   query flush binds and builds rules; other readers build the Go eager view on
   first use. Was: dl_context::add_rule
   only queues the formula (m_rule_fmls); flush_add_rules runs mk_rule
   (variable binding, rule-set insertion) at query time or get_rules().
   g3's PropositionalEngine binds and adds each rule immediately, so
   variable/rule ASTs are created earlier (38 vectors' first AST-log
   difference) and invalid-rule errors surface at the rule command, not
   at the query. Port the queue and the flush points.
   DONE: BackgroundAssertions (read on the query path) no longer builds the
   eager rule view, so a query binds each rule once, in its flush.
   OPEN (24 vectors' first difference): `(query R)` is
   dl_context::rel_query, and engine_base::query(num_rels, rels) (every
   engine but rel_context) wraps R as an expression, so mk_query builds the
   `query!N` rule. g3's QueryPredicate dispatches predicate-direct paths
   (queryPDR etc.); routing PDR through queryRelationExpression still made
   no query!0 and hung on fixedpoint_pdr_arithmetic_inductive, so the PDR
   expression path itself needs porting first.
   DONE for PDR/QPDR (muz/fp/pdr_relation_query.go): `(query R)` now runs
   mk_query in place: it registers query!N, adds `R(...) => query!N`, solves
   query!N, and removes the rule again (dl_interface::query's
   replace_rules(old_rules)). Answer/Proof/Model resolve R to query!N.
   Follow-ups that are also done:
   - get_trace never keeps the root head.
   - rel_query leaves datalog::context's m_mc null until a context::query
     runs, so relation queries apply no Horn model conversions.
     Certificates/models now show raw sliced names (P!slice!1) as z3 does.
   - add_cover no longer refuses inlined predicates.
   - add_child_property owns its mk_assumptions vector.
   Still OPEN:
   - Other engines' predicate-direct paths (spacer, bmc, tab, clp, datalog).
   - DONE: engine_base's `exists x. R(x)` is built before the rule flush
     (dl_interface::query's get_rules()). mk_query hoists it, with names,
     and wraps the rule in its forall. PDR logs now match through the
     query rule. Next first differences are in the transforms (scale's
     extra Real nodes).
   - DONE: bind_variables::abstract ported, with first-encounter indices.
   - DONE: inline_linear ported over m_head_index/m_tail_index.
   - DONE: m_inlined_rules, m_pinned and the plugin teardown now follow
     the source lifetimes. The old closeRuleBuckets ranged over a Go map
     (nondeterministic).
   - DONE: pred_transformer::init_rule's per-rule `init = mk_or(init,
     fml)`, and initialize's rewrite order (transition first).
   - DONE (fixedpoint_pdr_variable_trace log now IDENTICAL):
     - rule::deallocate order; m_inlined_rules released forward.
     - With a null m_mc, no inliner model rules are recorded.
     - The shared context th_rewriter (ctx.get_rewriter()) and the rule
       manager's qe_lite (m_qe) live with the context.
     - pm.mk_and in init_rule/init_rules; m_initial_state is one slot.
     - ensure_level adds solver levels unconditionally.
     - lform is held while asserting; reachable-cache locals are released.
     - The rewriter keeps its proof in m_pr; init_rule gets a per-rule
       rewriter; PDR's eq2ineq applies before the first assert.
   - DONE: Spacer relation queries and query_from_lvl use mk_query. Spacer
     enables proofs from update_rules through solve (scoped_restore_proof).
     virtual_solver asserts (=> pred true). Spacer init_rule gets a per-rule
     rewriter.
   - NEXT leads:
     - DONE: spacer init_rules locals; initialize's rw replaces the
       transition/initial slots; frames start empty; mk_pob normalizes
       twice; mk_extend_lit; virtual_solver ctor.
     - DONE (cfb8bddbe, 649589c5f): fixedpoint_spacer_core line 162. The
       factories own their virtual solvers (three per transformer, torn
       down per factory); ~itp_solver does not pop; a rule-less output gets
       no transformer; set_query only records; pop_to_base_lvl drops the
       literal snapshot; pop_scope_core releases justifications after the
       trail and the theories. Now line 168: only the AST id differs (#47
       vs #32), from the smt::context destructor's release order.
     - DONE (6f05521e5): mk_subsumption_checker port (plugin-lifetime
       m_total_relations and m_ref_holder, rule_subsumption_index, one
       extension mc per transform_rules pass). fp_spacer_magic first diff
       moved 39 -> 154.
       RESOLVED (9f66116f8): fix_unbound_vars(res, true) is restored. The
       duality "bridge" test was asserting more than z3 can do. In z3,
       duality's m_ctx.get_rules() runs check_rules on the transformed
       set, so quantify_arrays programs are rejected ("recursive predicate
       ... occurs nested in the body of a rule", then unknown; z3 segfaults
       on any later query). g3 now does the same.
     - OPEN (user decision): rule_transformer plugin order. The ported
       libstdc++ std::sort does match z3: fixedpoint_pdr_bit_blast matches
       to line 61 with it and to line 44 with Go's sort.SliceStable.
       It is the only vector of 660 whose trace differs between the two;
       stdout and stats are the same. Waiting on whether an STL exception
       is approved.
     - OPEN: fp_spacer_generalizers_inv is nondeterministic. The same
       binary gives different traces across runs (1 of 4).
     - DONE (3228654b5): mk_loop_counter makes its 0/1 numerals per rule;
       fp_spacer_karr is identical.
     - DONE (785336e91): substitution_tree walked m_roots in Go map order;
       it now uses func_decl id order. That fixed the
       fp_spacer_generalizers_inv nondeterminism. A 3-run determinism sweep
       over all 660 vectors is clean.
     - DONE (399d8379d): the transformed rules outlive the query and the
       Spacer context (m_spacer_rules); fp_spacer_magic is identical.
     - DONE (c83d78b68): mk_array_instantiation's std::cout banner and
       rule-set displays, mk_rule of (=> (and tail) head), done_selects in
       obj_map order; fp_spacer_array_instantiation is identical on stdout
       and trace.
     - DONE (c09ab9a97, 155c24eab): mk_quantifier_abstraction and
       mk_quantifier_instantiation go through rm.mk_rule (HNF splits rules
       as z3 does), with the mk_horn_rule fix_unbound gate, the source's
       local lifetimes, and pattern-before-app order. First differences:
       qa 47 -> 68, qinst 54 -> 65 (free-id order).
       Note: z3's quantify_arrays over-approximates. Goal :- P(a), not
       a[true] with P(a) :- a[true] is sat in z3, and g3 now agrees.
     - OPEN: fp_spacer_array_blast line 56. g3's mk_array_blast is
       restructured: it rewrites each conjunct instead of the whole body,
       ackermanize recurses instead of using a todo stack, defs are in
       insertion order instead of obj_map order, and it builds the rule
       directly instead of rm.mk_rule. Also, GCC evaluates
       mk_implies(mk_and(eqs), mk_eq(v1, v2)) right to left, so the eq
       comes first. Port blast/ackermanize/insert_def faithfully.
     - DONE (3a734e85a): mk_array_eq_rewrite uses a basic_union_find
       expr_equiv_class and rm.mk_rule; fp_spacer_array_eq_rewrite's
       first difference moved 68 -> 85.
     - OPEN: fixedpoint_spacer_coalesce_unfold line 69 (free-id order).
       In z3, ~mk_interp_tail_simplifier frees pve's last head (P x) and
       then its tails in creation order (#44 #45 #46 #47). Keeping
       rule_substitution's m_head/m_tail alive until the next get_result
       or Close, per the member order, gave 44 46 47 45 and cost 3
       full-log matches (401 -> 398), so it was reverted. Trace which
       member holds the head's last reference (outer m_tail? res?) before
       retrying.
     - DONE: mk_bit_blast lifetimes and rm.mk_rule (3a53c066c); spacer
       init_rules case 1 builds (or dummy ext) before (not dummy)
       (526ad095d); extract_horn applies ensure_app to the head and every
       body element before hoisting (9f0bbd88d). First differences:
       spacer_bit_blast 70 -> 129, spacer_scale 86 -> 109,
       spacer_array_blast 67 -> 75.
     - OPEN: fp_spacer_quantifier_abstraction line 68. z3 keeps
       (p #35 #36) alive until ~hnf (context teardown): it sits in
       hnf::m_body of the last mk_horn call, which reset() does not clear.
       g3's HornNormalizer.bodyRefs has the same persistence, so the
       difference is which formulas reach mk_horn (is_horn), or how
       eliminate_quantifier_body edits m_body in place.
     - DONE (c91f91afb): mk_query makes mk_asserted(rule_expr) up front
       under proof tracing; spacer_nonlinear_inlining 91 -> 110.
     - DECISION NEEDED: spacer_nonlinear_inlining line 110. In z3,
       try_to_inline_rule proves the resolvent after rule_unifier::apply has
       already run fix_unbound_vars and the interpreted-tail simplifier. So
       resolve_rule's hyper-resolve conclusion is the simplified,
       renumbered rule, while s1/s2 are the unifier's substitutions. Doing
       the same in g3 makes nonlinear_inlining IDENTICAL (full logs
       401 -> 402). But g3's slice proof restoration then builds invalid
       side conditions, and four TestRule*ProofComposition tests fail
       (they check validity with a solver). The variant is saved in the
       job tmp dir as rule_inliner_z3proof.go. Reverted pending a decision:
       z3's proof shape, or valid restored proofs. Earlier note:
       The inliner's
       hyper-resolution proof: z3 builds the conclusion (=> #25 (P x))
       from the resolved rule after its body (and #82 #79), but g3 builds
       a quantified (=> (and #25 #82 #79) head) first. Compare
       mk_rule_inliner/rule_unifier's mk_rule_rewrite/hyper-res proof
       construction.
     - DONE (063aad730): spacer init_rule's fml/tmp lifetimes;
       recursive_trace 109 -> 160, linear_inv 129 -> 182.
     - DONE (3e5c55603, d46f3a2b6): mk_quantifier_instantiation's
       m_var2cnst/m_cnst2var are plugin members, and ground_free_vars
       releases the fresh aux constant on reassignment;
       quantifier_instantiation 65 -> 256.
     - DONE (26af4862c): mk_horn_rule's m_body/m_head terms live until the
       next rule; array_blast 75 -> 82.
     - OPEN: fp_spacer_mutual_inv line 117. z3 frees qe_lite's input fml
       (and ...) inside th_rewriter before DER's intermediate; g3 frees the
       intermediate first (equalityDERCall.close releases c.refs LIFO). A
       plain forward release (m_new_exprs.reset()) cost 2 full-log matches,
       so it was reverted. c.refs mixes m_new_exprs with temporaries that
       z3 frees earlier, so split them before retrying.
     - DONE (01e46cbaa): qe_lite find_definitions keeps ts per iteration;
       mutual_inv 103 -> 117, arith_inv 120 -> 172,
       generalizers_inv 116 -> 181.
     - NOTE: /mnt/oldrog filled up (other sessions' /mnt/oldrog/tmp). Tests
       now run with GOCACHE/GOTMPDIR/TMPDIR under
       /var/tmp/g3-job-487c9ee8 (in its env file).
     - DONE (arithmetic group): the pve le/ge difference terms, and
       itail/simp_res lifetimes. pdr_nonlinear_trace and model_generalizer
       logs are identical; the convex/multicore/utvpi first differences
       moved from about line 70 to 110.
     - DONE: libstdc++ std::sort for rule_transformer plugins, with
       apply_default_transformation's registration order (no-op stand-ins
       for disabled plugins). The other transformer pipelines (rel_context,
       bmc, tab, ...) still register in g3's own order; check them the same
       way.
     - DONE: plugin-owned m_unifier; substitution releases forward.
       mk_scale numerals; mk_bit_blast m_g_vars/m_f_vars.
     - DONE (5397eb7e2): the Farkas B set includes the proxy definitions
       (or (not proxy) lit), elim_proxies replaces proxies with true, and
       the negation uses the simplifying mk_not. Earlier analysis:
       (Farkas core): safe_assumptions::elim_proxies replaces every
       proxy with true (asserted-true proof) and drops lemmas that become
       true; consequences are neither proxy-eliminated nor simplified. g3
       instead substitutes proxy -> formula, because its farkas learner
       returns the proxied B-literals themselves where z3's sees through
       the (=> proxy atom) implications. Port farkas_learner::get_lemmas
       first, then elim_proxies (tried: fixedpoint_pdr_initial_trace's
       first difference moved from line 353 to 662, but
       TestPropSolverExtractsFarkasTheoryCore lost its Farkas core).
       Second try: publishing the empty Farkas core as z3 would (no
       subset fallback) made 8 PDR arithmetic vectors time out. So z3's
       learner really returns non-proxy lemmas. Trace
       farkas_learner::get_lemmas on fixedpoint_pdr_arithmetic_invariant
       (-tr:farkas_learner) and compare g3's GetLemmas output.
     - fixedpoint_pdr_scale line 187 is inside theory_arith internalization
       (bound atoms before def-axioms): theory_arith work, out of scope here.
     - mk-var id reuse (initial_trace, recursive_trace,
       repeated_disequalities).
     - fixedpoint_pdr_scale: extra Real nodes in mk_scale.
     - fixedpoint_pdr_bit_blast: mk_bit_blast order.
   - OLD NOTE (superseded): ids drifted from the inliner
     teardown. z3's m_inlined_rules = [P(0,1), Q(1,1), R(2), Goal()]. Each
     rule frees its accounting-parent chain, i.e. the resolvents before
     interp-tail simplification. z3 frees Q's chain first (#57 Q(1,1),
     #58, #56); g3 frees an R-chain node (#61) first. Compare the parent
     chains that expandDefinition/resolve/normalizeResolvedRule build with
     try_to_inline_rule (apply, fix_unbound_vars, transform_rule).
   - After an expression query, z3's rel_query concatenates the older m_mc
     chain; g3 applies only the current conversions.
   - Fresh-name counter: the second sliced query names P!slice!30 in z3,
     P!slice!29 in g3.
   - The sliced property prints (not (>= x 1)) in g3 vs (<= x 0) in z3.
   - AddCover's "sliced predicate columns" refusal is g3's own (z3 checks
     only xform.slice).

22. **OPEN: opt::context::optimize flow (item 12 cluster).** A full sweep
    puts the earliest remaining trace differences in optimization:
    unbounded_real/lexicographic_interval (32), weighted_soft (35),
    strict_lexicographic (37), box (39), pareto (39), bounded_integer
    (40), lexicographic_integer (49), pareto_soft (30). z3's optimize()
    always runs normalize(): to_fmls builds mk_objective_fn apps
    (maximize!!N / minimize!!N / maxsat!!N) over the hard constraints,
    simplify_fmls runs the simplify/propagate-values/solve-eqs tactic, and
    from_fmls follows. Then comes the initial check ([begin-check] 0).
    PROGRESS (9a1f116be): normalize now runs regardless of rlimit/timeout
    (the UINT_MAX default timeout had disabled it), and the objective
    functions are named maximize!!N. Remaining: init_solver before
    normalize (z3's attach-enode lines come first), the initial
    check_sat (the [begin-check] 0), and normalizing on the first
    pareto/box call.
    PLAN: this is a restructure, not a local fix. cmd_context's
    checkOptimize -> runNormalizedOptimization -> checkOptimizeLex builds the
    opt.ArithmeticSolver (whose SMT kernel creation gives the attach-enode
    lines) only after normalization, and ArithmeticSolver.Check goes
    straight to optimizeOne per objective. z3 runs clear_state,
    init_solver, import_scoped_state, normalize, internalize,
    update_solver, assert(m_hard_constraints), check_sat(0) and
    model_updated, then m_optsmt.setup / update_lower, and only then
    execute(...) / execute_lex / execute_box / execute_pareto.
    Steps: (1) create the backend before EncodeOptimizationFormulas;
    (2) assert the simplified hard formulas, then make an initial Check
    with no assumptions, keeping its model as the first incumbent (this
    also short-circuits unsat/unknown as z3 does); (3) feed the
    objectives to the existing per-objective search.
    TRIED: adding only step (2), an initial CheckSat at the start of
    ArithmeticSolver.Check, cost 6 stdout matches (647 -> 641) without
    moving the early trace lines (the init_solver order still differs).
    It was reverted. Do (1) through (3) together, and check which
    optimization outputs depend on the extra search state.
    Second try: CheckHard() in checkOptimizeLex after the hard
    assertions, before the objectives (z3's order). The "AST source
    precondition" errors are gone, and only bit_vector_optimization
    changes. But that exposes a LATENT MaxSMT BUG: for maximize x with
    (bvule x #x9), the model is x=9 (cost 6 over the 8/4/2/1 bit soft
    terms), while the engine reports cost 8 (objective 7) as optimal.
    The reported soft cost disagrees with the final model, so investigate
    the maxres/bit-vector objective bound bookkeeping (addBitVectorObjective,
    optimizeSoft, finalizeSoftBounds) before landing the initial check.
    FIXED (dc2748bc8): maxres publishOptimum now follows found_optimum and
    reports the best model's cost instead of the overshooting core lower
    bound. It reproduced on HEAD with smt.random_seed=5. With that fixed,
    the initial hard check landed in the lex path (see git log).
    Remaining: init_solver before normalize, and the pareto/box first-call
    normalize.
    DONE (1e33f366c, ae95bd25c, and the internalize commit): on the lex
    path the backend is created before normalize, lia2card makes its
    numerals per used variable, and objectives are internalized before the
    initial check. First differences: unbounded_real 30 -> 39,
    bit_vector 13 -> 41, bounded_integer 38 -> 51, lexicographic_integer
    47 -> 65.
    BLOCKED ON theory_arith (out of scope): unbounded objectives. In z3,
    geometric_lex's maximize_objective returns the blocker false for an
    unbounded objective, so the re-check is unsat and the loop ends
    (unbounded_real_optimization line 39: one Real numeral, then
    begin-check). g3's optimizeArithmetic instead runs an "unbounded
    recovery": it asserts objective >= model value + 1 and re-checks,
    because g3's arithmetic theory can report a locally unbounded tableau
    before a Boolean branch fixes the objective. Remove the recovery once
    theory_arith's maximize is faithful.
    TRIED: dropping the priority != pareto/box gate (normalize on the
    first pareto/box call) cost 2 stdout matches (647 -> 645), and the
    cmd_context/cmd/g3 tests ran past 10 minutes (a likely hang in the
    pareto/box paths with simplified hard formulas). Reverted. Those
    paths need the decoded objectives and the preprocessor model converter
    first (checkOptimizePareto/checkOptimizeBox read c.objectives and
    c.assertions directly).
    g3's runNormalizedOptimization only simplifies behind its own
    useSimplifier gate (skipped for pareto/box/rlimit/timeout) and never
    builds the objective-function apps. Port opt_context.cpp's optimize /
    normalize / to_fmls / from_fmls in order. Other early clusters from
    the sweep: bit-vector optimization (13-14, mk-var vs extract), the
    horn tactic/solver query rule (31-33: z3 builds (and ...) before
    query!0; DONE for horn_solver_finite in 537b08dec, which is now
    identical; horn_tactic_finite still differs at 34, where z3 frees the
    (not Q) temporary and wraps the query rule in a forall), and
    sat_dimacs/pb2bv string printing (35, 46).

21. **Recorded deviations, kept deliberately.** cmd/g3 tests encode
   these: bvsmul_noovfl numeral folding uses signed semantics (the source
   compares the unsigned product with 2^(sz-1)); nlqsat answers goals the
   source rejects as 'not NRA' (is_pure_proc is not ported; the test keeps
   the C++ unknowns as referenceWant). Ask before changing either.
   Also kept: symbolic_automata::mk_product tests is_final_state without
   the epsilon closure, so ("" | a) & ("" | b) would be empty; g3's
   product includes the closure (string_regex_automaton_product_test).
