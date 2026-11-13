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
   RESOLVED by 1bbcecbac (a learned unit is kept for reassertion only
   under delay_forced_restart, m_units_to_reassert; g3 had kept every
   learned unit, so later limited checks settled PHP early). Regression
   vector: cmd/g3/test_vectors/php43_max_conflicts_repeated_assuming.smt2
   (stdout and trace=true log identical to the fork).

2. **DONE (see pkg/SEQ_PORT.md): Port `theory_seq::final_check_eh` faithfully.** Logics missing from
   `setup::setup_logic` (QF_NIA, ALL, ...) use `setup_unknown`, which
   registers theory_seq. Its final check draws from the context random
   generator in `reduce_length_eq` and `branch_variable` even with no
   sequence constraints. Go's sequence final check is a different design
   and does not draw, so later random choices diverge
   (nonlinear_nested_integer_row_aliases, bv_integer_conversion_scopes).

3. **RESOLVED for the test corpus: AST lifetimes and id recycling.** STATUS 2026-11-13 (24393536c): every test vector is trace-identical to the fork (fullcmp2 802 identical / 0 differ, 11 traced timeouts; with Z3_TRACE_NO_PP 813 identical / 0 differ / 0 timeouts), including the vectors named here, so this item is resolved for the test corpus; the notes below are history. z3 frees each `expr_ref`
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

12. **Golden trace coverage.** STATUS 2026-11-13: TestTraceGolden had
    silently failed since c3c0016bd shifted smt_context.cpp lines (it runs
    only without -short, so make never ran it). It now identifies sites by
    tag + function + file and ignores line numbers (no ported function has
    an unported same-tag site, checked over all vectors), and g3's emitted
    lines were corrected (assign_core 294, add_eq_detail 498, decide
    1830/1886, propagate_atoms 1381, relevant_eh 1615, smt_relevancy
    467/468). Golden list: 781 of 818 vectors (was 519).
    STATUS 2026-11-13 (later): 812 of 818. Fixed: process_antecedent
    traces the true literal that a unit_resolution_justification lists
    for a removed false disjunct (smt_conflict_resolution.cpp
    process_antecedent; g3 returned before the TRACE); pdr's
    extract_theory_core constructs farkas_learner, whose constructor
    builds an unused smt::kernel over its own proof manager
    (pdr_farkas_learner.cpp:382), so g3 writes that context's
    mk_bool_var record (smtcore.TraceDetachedContextSetup);
    theory_lemma_justification does not override get_from_theory, so its
    FromTheory is null_theory_id (pb_strengthening_proof). bv_blast_*,
    bv_conflict_restore_relevancy, flush_bound_axioms_introsort_* and
    fp_to_ubv_uf_model only took longer than the old 2m limit (q3063: z3
    ~25 min, g3 ~50 min traced; g3's traced runs are about twice as slow
    as z3's). These 8 are in test_vectors/golden_trace_slow_vectors.txt,
    compared only with G3_GOLDEN_SLOW=1 (limit 90m); the default list has
    804 vectors and runs in about 9 minutes with a 10m per-run limit.
    Still not golden: sequence_helper_inventory, z3_bug_automaton_product,
    z3_bug_qe_nested_store, recursive_mutual_boolean_model (documented
    z3 4.7.1 bug fixes); quant_seq_shared_boolean_scopes (the z3 TRACE build
    reaches UNREACHABLE in theory_seq::assign_eh for the Boolean atom
    (seq.nth y 0), which is __builtin_unreachable without Z3DEBUG, and
    loops forever there; its 139 golden records equal g3's first 139);
    qfuzz_cheap_eq_after_instance (traced runs of both do not finish in 30
    minutes; the first 554727 golden records agree).
    Older notes follow. 423 of 660 test vectors now match the z3
    trace build at every golden site (assign_core, decide, add_eq_detail,
    mk_bool_var, the seq records, and the conflict records with ids
    masked); they are TestTraceGolden's default vectors
    (cmd/g3/test_vectors/golden_trace_vectors.txt). Fixes found by the
    failing vectors so far (423 -> 482 matching):
    - 2026-11-10 later batch (full logs 555 -> 570 of 659; stdout 650/662):
      model finder qinfos built at register_quantifier; scoped_mk_model
      drops m_proto_model (factory trail) at search start/end; ~theory_seq
      undoes its trail then destroys members in reverse order; theory_seq
      m_rewrite(e) is in place; iz3: local th_rewriter per z3_simplify,
      _tree wrapper local, constructor/member order, ~iz3base order;
      theory_str: check_concat_len_in_eqc/infer_len_concat in new_eq_check,
      check_length_const_string's tail get_len_value, mk_int_var INT_SORT,
      ctx.mk_eq_atom in simplify_parent and shared premises; learned units
      kept only under delay_forced_restart; nlqsat hoist is nlqsat::hoist
      and its guard is "is_true"; conflict_resolution is context-owned and
      m_lemma_proof is replaced (not reset) per conflict; TacticResult
      releases converters before goals; tactic new_curr/new_pr locals die
      with their scope; array mk_interface_eqs uses mk_eq_atom.
      Recorded: items 26-29 below.
    - 2026-11-10 order/lifetime batch (full logs 517 -> 532 of 659): shared_occs,
      value_set m_values and m_var2num_occs walk hash-table order (ObjMap
      layouts); elim_term_ite m_new_defs and vector::reset release front to
      back; defined_names' ite definition builds MK_EQ before MK_NOT;
      opt::context::pop releases m_optsmt/m_hard_constraints; theory_bv
      releases its bit expressions after init_bits (m_bits keeps literals),
      honors rewriter.flat/elim_and, and MK_AC_BINARY interleaves
      get_arg_bits with blasting; bv2int/int2bv statement order and in-place
      rewrite; lackr::eager simplifies in place; simplify_fmls always builds
      result_pr; pattern inference builds quant_intro before the pull; model
      finder offsets use one arith_rewriter mk_add/mk_sub step (MkAdd2/MkSub2);
      insert_rec_fun builds lhs/eq before the macro case. Later the same day
      (532 -> 545): ~qi_queue dies in flush; mk_bits paths drop bit owners;
      bit-blaster locals live until init_bits ends; unit_subsumption,
      pb2bv new_exprs/new_deps release order; qsat statement temporaries.
      ASK BEFORE CHANGING: datatype_cycle_proof differs because g3's
      occurs_check_explain adds the app=C(...) edge that z3 4.7 omits (the
      conflict is otherwise unjustified). OPEN: proof-producing runs build
      conflict proofs recursively (clause proof, then literal proofs); z3's
      conflict_resolution::get_proof uses a LIFO todo stack, so antecedent
      proofs come first (max_bv_sharing_proofs line 236). Also (545 -> 547):
      m_nl_new_exprs.reset() front to back; simplify keeps its proof when
      proofs are enabled. OPEN: simplify :print-proofs prints with the format
      printer, where z3 uses ast_smt_pp::display_expr_smt2 (no format nodes;
      simplify_command line 166). OPEN: reset_assertions_global line 141:
      asserted_formulas' bit2int m_bit0 dies before another context's
      qi_queue variables in z3, after them in g3. Open: model finder's
      mk_add_core numeral order (recursive_mutual_and_arguments line 917);
      pretty-printer format ids (bv_integer_conversion_scopes line 92987).
    - 2026-11-10 seq lifetime batch (full logs 513 -> 517 of 659; stdout
      649/662): simplify's th_solver comes from the strategic factory;
      seq_rewriter keeps m_es/m_lhs/m_rhs and reduce_eq edits ref-holding
      vectors; mk_eq_core uses the faithful reduce_eq (reimplementation
      deleted); fixed_length/propagate_length_coherence reassign expr_refs;
      bv bit owners drop before the context unwinds; seq_factory owns
      m_trail and dies with m_proto_model (released after flush);
      sym_expr::accept returns an owned ref; cg_conflict_eh is called from
      add_eq. sequence_helper_inventory first diff 50 -> 21236,
      sequence_word_equations_relations 112 -> 105702. Then re2automaton
      builds products/complements only with a solver (m_sa) and the
      unported RegexByteStringCandidates/RegexAcceptedLengths helpers are
      gone; sequence_helper_inventory now first differs at 21357, where the
      product automaton's final states differ. ASK BEFORE CHANGING: g3's
      product (seq_rewriter_automaton.go) deliberately uses closureIsFinal
      instead of z3 4.7's is_final_state, because the source treats
      ("" | a) & ("" | b) as empty; matching the trace means adopting that
      bug.
    - 2026-11-09 lifetime batch (full logs 489 -> 496 of 661; stdout 648,
      all-stats 624): qsat operator()'s defs live until the tactic call
      returns; tactic2solver m_assertions shrink front to back. OPEN leads:
      lra_propagation_mode_updates (theory_lra keeps x=2 after pop, g3
      picks x=3; item 16); weighted_soft_optimization (z3's optimize()
      normalizes with simplify_fmls before its first check, g3's weighted
      soft path checks hard constraints first without the simplifier;
      item 22); the nlqsat vectors (the reference z3 answers unknown and
      stops early).
    - 2026-11-09 ordering batch (full logs 480 -> 489 of 661): utvpi_tester
      m_coeff_map and cofactor get_best occs iterate as obj_maps (table
      order); theory_lra keeps theory::mk_eq_atom (id order, no numeral
      swap); bit_blaster mk_udiv_urem updates p in place from the top bit
      down; context::restart builds a candidate proto model when the
      search fails with a non-false status.
    - 2026-11-09 expr_ref batch (full logs 462 -> 480 of 661): z3 frees
      an expr_ref local on scope exit or reassignment, while a g3 handle
      with reference count 0 lives until some inc/dec pair touches it.
      Ported as owned locals: poly_rewriter mk_sub's minus_one,
      bv2real/bv2int mk_bv_mul/add/sub/eq s1/t1 (and mk_sbv as a raw
      numeral), pb2bv mk_bv's dualized nargs, bit_blaster mk_multiplier's
      last-step mk_xor3 into out. GCC right-to-left argument order:
      bv2real mk_mul/mk_add/mk_sub/mk_eq, th_rewriter pull_ite (else branch
      first). Asserted-formula proofs follow each pass's rule (assert_expr
      vs simplify_fmls vs propagate_values/nnf_cnf). OPEN leads:
      lra_backend_models_scopes (theory_arith picks x=3,y=2 where z3 picks
      x=4,y=1; item 16); pb_native_watches_proof prints (not
      rejected_watch) inline because one fewer holder leaves its
      reference count at 2 (z3 binds it as $x38); card2bv_weighted_models
      line 148 (mk_bit2bool runs four times after begin-check, z3 once).
    - 2026-11-09 MAM/teardown batch (full logs 450 -> 462 of 661):
      the MAM port (item 19); context::flush order (undo_trail_stack(0)
      releases enodes and bool vars, then m_qmanager = nullptr frees the
      plugin, qi_queue instances and m_subst, then clauses; the proto
      model is a later member); ~asserted_formulas destroys its
      simplify_fmls members (pattern inference) after the context;
      expr_pattern_match m_precompiled, vector<justified_expr> batches
      and cached_var_subst m_refs all release front to back (formula
      before proof); cached_var_subst::reset keeps var_subst's reducer
      cache; macro_finder is_arith_macro holds/releases new_body/new_q
      and uses an Int zero coerced by mk_app; arith_util::mk_add returns
      a single app argument as is; setup_auto_config's logic list (no
      UFLRA/QF_FP case) decides whether setup_unknown(st) builds the
      theory_seq kernel.
      OPEN lead: the model finder's quantifier_analyzer runs in z3 at
      register_quantifier (internalize time) and builds its x<=t bounds
      with macro_util::mk_add (arith_rewriter folding, e.g. 5+1 -> 6);
      g3 builds the AUF graph lazily at model-check time with a raw add
      then normalizeGround (all_quantified_arithmetic_after_push line 45).
    - 2026-11-09 batch (full trace=true logs identical 427 -> 445 of 661):
      add_eq eq-propagates only marked parents; theories pop in
      registration order; search pops flush the asserted rewriter cache;
      internalize_term applies the sort constraint twice; enum2bv,
      purify-arith trig, theory_arith mk_polynomial_ge and macro_util
      mk_sub argument order and lifetimes; tactic2solver member release
      order; symmetry-reduce obj_map/u_map iteration order; blast-term-ite
      stray mk_app; cofactor-term-ite cofactor_rw port; qe-lite
      reduce_quantifier result threading; maximize_ac_sharing compares
      the AST kind (so every associative app is shared after init).
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

14. **DONE: Inner-solver and outer `mk-bool-var`.**
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

18. **DONE (2026-11-10): expr_quant_elim output order matches z3.**
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
    UPDATE (2026-11-10): the search tree is now ported
    (qe/expr_quantifier_tree*.go: quant_elim_plugin
    add_constraint/elim_var/find_min_weight/constrain_assignment, the
    search_tree with guarded leaves), and the bit-vector blast runs through
    it (eliminateQuantifierScalarSearch with qeScalarBitVector). The
    example still differs (g3 cases 3, 0, 4, 2; z3 0, 2, 4, 3), and the
    order is model-guided: z3's first inner model gives z_bv = 0, g3's
    gives 3. NEXT: compare the inner smt::kernel's params and first model
    for the blasted formula (bv2int/mod over the bit-vector), the same
    model-choice class as item 25. z3 also keeps both orientations of
    (+ (* (- 7) y) (* 5 z)), which points to an uncached rewrite of one
    copy.
    z3 -tr:qe (z3del build): the first inner model has z!2 = #b11, the
    same as g3's. constrain_assignment then sets "z!2 := 4" (four branches)
    and the re-checks add children for z = 2, 1, 3, 0, in that order. So
    the order comes from constrain_assignment's re-check sequence and leaf
    collection, not from the first model. NEXT: log g3's branch values in
    qe/expr_quantifier_branch_limit.go and qe/expr_quantifier_tree_advance.go
    and diff them against that sequence.
    DONE (diff): g3 adds the z branches 3, 1, 0, 2, while z3 adds 2, 1, 3, 0.
    The search loop (final_check, update_current, pop, solve_vars,
    constrain_assignment, which returns early for bit-vector variables)
    matches the source, and nothing is asserted between the two checks.
    z3's kernel answers the re-check after pop with a new model
    (z!2 = #b10), where g3's returns the same model (#b11). The remaining
    gap is in the inner smt::kernel's model on a re-check without new
    assertions (phase/restart state of the bv2int/mod formula), not in the
    qe port.
    A top-level check-sat twice on the same formula agrees between z3 and
    g3 (zb #b11 both times; y/z change identically). The qe inner kernel
    is different: z3's first inner model has z = -4, where top level gives
    -7. NEXT: compare quant_elim_new's smt::kernel (m_fparams from the qe
    tactic, its assumption literal, the qe plugins' extra assertions) with
    g3's inner SMTSolver configuration in qe/quantifier_tactic_arith_search.go.
    DONE (5b510dc22): the inner kernel now gets the flets (bit2int,
    enum_const_mod, int2bv2int, array canonize) via
    SMTSolver.ApplyQuantElimSettings. The first case now matches z3 (g3
    0, 3, 4, 2 vs z3 0, 2, 4, 3). PENDING: the flet's m_relevancy_lvl = 0.
    With relevancy 0 passed as a kernel parameter, searchExprQEScalarTree's
    re-check after CHOOSE_VAR (expr_quantifier_bool_search.go, the
    refreshed == nil branch) gets a sat status whose result has no model
    (SMTSolver.AcquireModel returns nil), so the search errors with
    ErrASTDomain. Find why a sat re-check without new assertions publishes
    no model when relevancy is 0 (a kernel fast path?), then apply
    relevancy 0.
    DONE (b97881aba): this was an SMT-core bug. preserveLearnedClauses
    numbered the slots it kept by lemma clause objects only, but the pop's
    compaction also keeps binary lemmas, so reinit read a neighbour's
    record. The kernel now runs with relevancy 0. Branch order now: g3
    z = 3, 0, 2, 1; z3 2, 1, 3, 0. z3's three inner checks in the first
    final_check/pop round move z!2 from #b11 to #b10, while g3's stay at
    #b11. NEXT: dump each inner check's model in both (z3 -tr:qe prints
    final_check models; add a g3 print in searchExprQEScalarTree) and find
    where the kernels' re-check behaviour splits: restart/phase state,
    theory_lra's bv2int variables, or the asserted assumption literal.
    DUMPED (g3 print in searchExprQEScalarTree): the first inner model is
    the same in both (z!2 = #b11, z = -4, y = -2). z3's re-check without new
    assertions (final_check's m_solver.check() after CHOOSE_VAR) returns
    the #b10 model (z = -12, y = -8). g3's re-check returns #b11 three more
    times, then #b00, then #b10. A top-level double check-sat of the same
    formula agrees between the two, so the split is in how a re-check
    restarts search when the kernel holds the qe assignment/assumption state
    (decision phase and literal ordering after pop_to_base_lvl). NEXT:
    trace the second check's decisions with z3 -tr:decide on the qe run.
    DONE (-tr:decide diff, g3trace build): two qe-port gaps, not kernel
    ones. (1) quant_elim_plugin::check pushes a solver scope before
    asserting the formula and pops it at the end; g3 asserted at base
    level. (2) final_check runs `while (can_propagate_assignment)
    propagate_assignment;` and update_current(true) on one model, then a
    check; when update_current(false) moves to another node it continues
    with that model. g3 went back to the outer check after each
    NEED_PROPAGATION step and after a node change, which added extra
    checks. With both fixed the case order matches z3 (0, 2, 4, 3), and
    the decision trace has the same length (1434 lines), first differing at
    line 1220 (p151 vs p150, bool-var numbering). Left: the two outer atoms
    print as (+ (* (- 7) y) (* 5 z)) where z3 keeps (+ (* 5 z) (* (- 7) y))
    (an uncached rewrite of one copy in z3), and the p151/p150 numbering.
    DONE: the orientation was a port-order bug. mk_resolve builds a*s + b*t,
    its <= 0 bound and the slack bound before the |a| < |b| swap; g3's
    resolveCooper swapped first. With that fixed, the example's output is
    byte-identical to z3 (test vector qe_cooper_symbolic_blast.smt2).

19. **DONE (2026-11-09, 46531ec4a): MAM ported; old matcher deleted.**
   smtcore/smt_mam_{code,compiler,interpreter,impl}.go port mam.cpp's
   label hasher, instruction set, compiler, backtracking interpreter,
   inverted path index and mam_impl. Patterns are added by assign_eh from
   propagate_atoms (context::assign_quantifier), as in default_qm_plugin.
   Full logs 450 -> 451 identical (quantified_ematching_relevancy);
   lazy_multipattern_scopes now diverges at line 616 (was 475). That
   difference is a lifetime one (item 3): z3's qi_queue m_instances
   shrink at pop frees (not q) with its or-lemma, while g3 still holds
   (not q) until the schedule's Close. History of the item follows.
   WAS: E-matching is not a port of z3's MAM. smtcore/smt_mam.go is a
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
   PLAN (2026-11-09; started): port src/smt/mam.cpp (4037 lines) into
   smtcore/smt_mam_*.go, replacing smt_mam.go's matcher, in phases:
   a. label_hasher and the enode lbl hash (set_lbl_hash, m_lbls/m_plbls
      updates in add_eq and internalization); instruction types (initn,
      compare, check, filter, pcheck, get_enode, choose, cont, bind,
      get_cgr, yield, is_cgr) and code_tree / code_tree_manager.
   b. compiler: linearize patterns into code trees, sharing prefixes
      (insert, is_compatible, multi-pattern joints).
   c. interpreter: execute with backtracking, the bindings/registers,
      init_continue/next candidates, on_match -> add_instance with the
      used-enode list in discovery order.
   d. mam_impl: add_pattern, add_node/relevant, propagate (match_new
      patterns, to_match, inverted path index path_tree for add_eq),
      rematch, scope trail (mam_value_trail, mk_tree_trail).
   e. Switch the quantifier manager to it, delete smt_mam.go's matcher,
      and compare [new-match] traces (quantified_ematching_relevancy,
      lazy_multipattern_scopes and the quant_* vectors).

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
     - DONE (user approved the STL exception, 2026-11-09): rule_transformer
       plugin order keeps the libstdc++ std::sort port
       (muz/base/rule_transformer_sort.go): fixedpoint_pdr_bit_blast matches
       to line 61 with it and to line 44 with Go's sort.SliceStable.
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
       (bound atoms before def-axioms): theory_arith work (now approved, 2026-11-09).
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

25. **DONE (2026-11-10): LRA model values
    (lra_backend_models_scopes x=3,y=2 vs z3's x=4,y=1;
    lra_propagation_mode_updates).** z3 uses theory_lra (setup_r_arith),
    and so does g3. Both register y as lar column 0 and make the same two
    tableau-rows iterations with the same RNG draw (seed 0, first draw 38).
    The difference is the order of cells in the term's row:
    - lar_term::m_coeffs is a std::unordered_map<unsigned, mpq>, and
      libstdc++ iterates it newest-first for these small keys.
    - z3's row for (+ x y) therefore lists x before y, so
      find_beneficial_column_in_row_tableau_rows keeps x and rejects y on
      the even draw. x enters, giving x=4, y=1.
    - g3's LARTerm iterates insertion order (insdict), so y enters, giving
      x=3, y=2.
    Matching z3 means emulating libstdc++ unordered_map iteration order
    (bucket layout, node insertion at the bucket head, rehash policy).
    That is porting STL internals, which AGENTS.md forbids without
    explicit approval, as with the rule_transformer sort exception.
    DECISION NEEDED from the user: approve a libstdc++-order emulation for
    lar_term (and any other unordered_map/unordered_set the lp code
    iterates), or accept this as a recorded deviation.
    APPROVED and DONE: pkg/util/lp/std_unordered_map.go ports GCC 13
    libstdc++ std::unordered_map<unsigned, V> (identity hash, mod bucket,
    _Prime_rehash_policy, insert at bucket head, rehash, erase, clear) as
    StdUnorderedRationalMap, and LARTerm uses it. Positive control:
    src/test/stl_unordered_map_order_control.cpp, whose output is
    pkg/util/lp/test_vectors/libstdcxx_unordered_map_order.txt (prime table,
    3000 random toggles, 5000 sparse keys); TestStdUnorderedMapMatchesLibstdcxx
    replays it line for line. lra_backend_models_scopes is now identical.
    The port exposed a separate gap: mk_clause sorted only CLS_AUX literals,
    while simplify_aux_lemma_literals also sorts CLS_AUX_LEMMA (and treats a
    literal true at base level as making the lemma true); now ported.
    Full logs 498 -> 504 identical; lra_scoped_rational_witness_b 249 -> 1075,
    lra_propagation_mode_updates 80 -> 100.
    NOTED (source behaviour, reproduced): after a pop, lar_solver restores
    m_status to the outer OPTIMAL but keeps the inner scope's column values,
    so final_check_eh skips make_feasible. With (set-logic QF_LRA), z3 4.7
    reports x=2, y=3 for x>=0, x<=4, y<=2, x+y>=5 after push/(x<=2)/pop,
    which violates y<=2; g3 now gives the same model
    (TestSMTLRAFinalCheckKernelModelConflictAndRecovery pins it).

22. **RESOLVED: opt::context::optimize flow (item 12 cluster).** STATUS 2026-11-13 (24393536c): every test vector is trace-identical to the fork (fullcmp2 802 identical / 0 differ, 11 traced timeouts; with Z3_TRACE_NO_PP 813 identical / 0 differ / 0 timeouts), including the vectors named here, so this item is resolved for the test corpus; the notes below are history.
   PROGRESS (2026-11-09, cfd7faeae..3228858d1): update_solver installs
   sat/sat_solver's inc_sat_solver behind opt.maxSMTBackend, and every
   MaxSMT engine, blocker and commit goes through solver(). Fixes in this
   batch:
   - Hard constraints are asserted after update_solver, so they reach only
     the SAT solver.
   - optsmt::setup's base-level push/pop runs after each sat initial check.
   - m_sat_solver survives until the next update_solver.
   - MaxSMT objectives run unscoped (execute_lex).
   - A lone objective is committed.
   - maxres commit_assignment re-asserts m_defs/m_asms.
   - max_resolve/cs_max_resolve build per iteration in source order.
   - init leaves m_csmodel null.
   - probe_bv walks the normalized hard constraints.
   - is_maxsat ids print orig_term.
   - lia2card args, conds and m_rw lifetimes follow the source.
   - simplify_fmls drops to_fmls's fmls before from_fmls.
   - m_simplify and m_model_converter persist until the next normalize,
     and propagate_values is not clean()-wrapped.
   - from_fmls keeps objectives it does not see.
   weighted_soft_optimization is now identical (full logs 496 -> 497).
   Then (a6b3b4977..ec8ba3c47) bit_vector_optimization moved from line 42
   to 1182 of 1294:
   - from_fmls mk_atomic/purify runs per objective, and fm joins
     m_model_converter.
   - The purifier alone owns the post-from_fmls terms.
   - CheckHard keeps m_model, and maxsmt starts from it.
   - check_sat_hill_climb and get_cores are faithful.
   - m_opt_solver persists until the next init_solver.
   - new_soft is an obj_map iterated in table order.
   - remove_core/split_core order matches.
   Arithmetic lex (75c7bc2bf..92e37fc50): simplify_fmls's first simplify
   now gets m_params (elim_to_real on), update_lower builds m_lower_fmls,
   theory_lra mk_ge makes the numeral first, and optsmt m_lower,
   update_lower_lex and execute_lex's update_lower are in.
   strict_lexicographic_interval moved 42 -> 278. Leads there:
   (1) line 279: the smt2 printer builds format nodes for trace_assign's
       display_verbose in a different order (flat/mk_group);
   (2) after commit, g3 assigns the committed bound right away
       ([assign] #43), while z3 defers assert_expr to
       m_asserted_formulas, so its second lex (push, begin-check 1, pop)
       shows no assignment;
   (3) DONE (c6fdb63f0): geometric_lex re-commits earlier objectives, using
       the captured model value. CORRECTION to that commit message: z3's
       second lex shows a single [begin-check] 1 and then a pop, so its
       first check is unsat. The likely cause: z3 commits m_lower = 5 - eps,
       which theory_lra::mk_ge reads as non-strict, giving x >= 5. Its
       geometric_lex still returns l_true, and the reported x interval /
       y value come from m_lower/m_upper and update_lower. Committing
       optLower in g3 gives "unsat", because g3 treats the failed first
       check as the result. A faithful port needs geometric_lex's
       return-l_true path plus reporting from optsmt m_lower/m_upper, which
       replaces the ArithmeticInterval special case.
   DONE (2026-11-10): geometric_lex's expr_ref bound now outlives the next
   check_sat (released when maximize_objective overwrites it, via
   SMTSolver.MaximizeOptimizationObjectiveInto, or at exit), which fixed the
   numeral id recycling at line 278; the first difference is now 287, lead (2).
   ROOT CAUSE of lead (2), confirmed with z3 -tr:assert_expr_bug and gdb:
   z3's commit asserts (>= x 5.0) (from m_lower = 5 - eps), and
   asserted_formulas::assert_expr rewrites it to false with the
   propagate_values substitution ((not (>= x 5.0)) is a unit fact). So
   asserted_formulas is inconsistent, push internalizes nothing, the second
   lex's first check is unsat, and geometric_lex returns l_true with
   m_upper = m_lower. g3 committed (>= x (/ 9.0 2.0)), the model value.
   DONE: commit_assignment now asserts mk_ge(i, m_lower[i]) (optLower), and
   a later lex objective whose first check is unsat returns l_true and reports
   m_lower[i]. strict_lexicographic_interval's full log is identical to z3's
   (497 -> 498), and bounded_real_strict_optimization moved 75 -> 138.
   Remaining: ArithmeticInterval still builds the reported interval itself,
   not from m_lower/m_upper.
   DONE (2026-11-10): optsmt m_upper is tracked (optUpper). geometric_lex's
   l_true exit sets m_upper = m_lower and resets every later m_lower to
   -infinity (finishGeometricLex), and ArithmeticInterval is
   display_assignment's get_lower_as_num != get_upper_as_num, i.e.
   m_lower[i] != m_upper[i] after execute_lex's update_lower. Output is
   unchanged on every optimization vector.
   Box (a5d39adb7, ba2664f22, 4853fd923): the first box call runs
   init_solver/normalize, and execute_box runs on the optimize() solver
   (optsmt::box, then a scoped execute per MaxSMT objective). elim01's
   cleanup keeps m_rewriter. box_optimization moved 38 -> 79.
   Pareto: DELIBERATE DEVIATION (ac072393b). Running normalize on the first
   pareto call (it works once filterParetoModel applies
   m_model_converter) reproduces z3 4.7's behaviour: gia_pareto never
   terminates once 0-1 objectives are converted to MaxSAT sums (z3 times
   out on a 2-variable x+y=1 frontier; see the pareto cmd_context tests).
   Pareto therefore keeps the unnormalized path, and its traces stay
   divergent by design.
   bit_vector_optimization line 1182 (2026-11-10, delcmp): the free-list
   order of #8 = (or #41 #60 #49) and #26 = (not #8) is swapped. In the
   release run just before the divergence, z3 frees #26 with #108 =
   (or #37 #26) and #8 later, while g3 frees #8 early and #26 later, so
   some holder keeps #8 in z3 but (not #8) in g3 (likely bit_blaster
   m_values). NEXT: Z3_TRACE_DEL=1 Z3_DEL_BT=8 on the z3del build, 21st
   free of #8, to name the holder.
   NARROWED (2026-11-10): the holder is the simplify tactic's th_rewriter
   act_cache, released in simplify_tactic::cleanup (cleaner_tactical, in
   inc_sat_solver::internalize_goal); g3's SimplifyTactic.Cleanup is the
   same path. With [actdec] logging of act_cache::dec_refs and del_unused in
   both (z3del src/ast/act_cache.cpp, temporary), the first difference is
   much earlier: g3's cleanup releases entries #58 -> #58 and #63 -> #63
   (#58 = (and #39 #47), #63 = (or #47 #39), under #57 = (=> a!11 #63)) that
   z3's cache never held. rewriter_core::must_cache needs ref_count > 1, so
   g3 holds an extra reference on those subterms while simplify runs (an
   item 3 lifetime gap upstream of the tactic). NEXT: log act_cache inserts
   with the key's ref count in both and find the extra holder.
   FOUND/FIXED (bb86ecd37 and the next commit): (1) process_unsat's core
   block fml = mk_not(mk_and(core)) is a local; g3 gave the conjunction and
   block to the engine owner, so the and had ref count 2 and was cached.
   max_resolve/cs_max_resolve locals now follow source lifetimes, and
   update_model evaluates into m_csmodel. (2) The model evaluator's AND/OR
   short-circuit (added with recursive definitions, 3e96cf2e9) is a
   deliberate deviation: without it z3 4.7 and g3 both hang on
   recursive_mutual_boolean_model's get-value. It skipped later children,
   so their cache entries were missing. It is now enabled only once a
   recursive function has been defined (ASTManager
   NoteRecursiveFunctionDefinitions). Full logs 504 -> 507 identical.
   NEXT LEAD (after those fixes the act_cache logs are identical): the line
   1182 free-order swap of #8/#26 now comes from model teardown
   (~model_core in ~inc_sat_solver at opt::context::update_solver). With
   [mdec] logging of ~model_core's m_interp walk (z3del
   src/model/model_core.cpp, temporary) and of g3's retireConstant, z3
   destroys 14 more model entries there (soft!9/soft!10, x and the
   (= #b1 ((_ extract i i) x))!k proxies, in several models). g3 never builds
   those intermediate models (copies, or models made by model-converter
   application in inc_sat_solver::get_model / opt's model updates), or frees
   them on another path. Find which z3 models these are (Z3_DEL_BT on a value
   id, or a ~model_core stack) and port their creation and lifetime.
   FOUND/PORTED: they come from opt::context::verify_model (called from
   maxres::update_assignment), which g3 lacked, and from elim01's
   bool2int_model_converter (z3 4.7), which g3 had replaced with a
   filter+extension pair. opt/verify_model.go ports verify_model (md->copy,
   fix_model, eval of m_term, compare with m_adjust_value(upper)), skipped for
   maxsmt_solver_base::init's initial model. cmd_context registers m_term and
   m_adjust_value for converted objectives (SetObjectiveVerification).
   tactic/bool2int_model_converter.go ports the converter, and elim01 now
   installs it unconditionally, as z3 does. The model-count mismatch is gone.
   The next difference is the entry order inside a 7-entry model
   (obj_map layout), and line 1183 is unchanged. Corpus metrics unchanged
   (stdout 649, full logs 507); make passes.
   NARROWED: pairing [mfree] blocks (g3 build_g3md.sh adds the marker), the
   first differing model is block 8: same 7 entries, different table order.
   z3 frees it in bit_blaster_model_converter, inside
   inc_sat_solver::extract_model. inc_sat_solver's m_map walk ([xm] logging
   in both) is identical at all 19 extract_model calls. Because the model holds
   x (a bit-vector value), it is the output of an earlier converter in m_mc0,
   and that converter inserts constants in a different order in g3. NEXT:
   log register_decl order per model (or a creation stack for that model) and
   compare with g3's m_mc0 chain (inc_sat_solver's internalize_goal
   converters).
   FOUND/FIXED: registration order was identical ([mreg] logging per model
   address); the layout differed because model_core::register_decl uses
   m_interp.insert_if_not_there2, whose grow check runs before the lookup,
   so re-registering an existing x in a 7-entry, capacity-8 table rehashes to
   16. g3's RegisterConstant/RegisterFunction touched the layout only for new
   keys; both now call ObjMap.InsertIfNotThere on every registration. Model
   teardown blocks now match through block 88 (was 7). NEXT for line 1182:
   three bit_blaster_rewriters are destroyed in a row at update_solver. Their
   cfg vectors (values/out/in2/in1) are identical in both tools, but g3 frees
   #8 after the first rewriter's Close and z3 after the third. So a different
   rewriter's cache (or result stack) holds #8. Compare each rewriter's
   act_cache contents at destruction.
   FOUND/FIXED: the caches were empty, so the holder was the SAT solver's
   model. ~inc_sat_solver destroys members in reverse declaration order:
   m_mc0, ..., m_subgoals, m_preprocess, m_bb_rewriter, then m_model, then
   m_map. g3's Solver.Close released the model first. It now closes the
   converters, then the encoder's preprocess/blaster part
   (closeBeforeModel), then the model, then the atom map. bit_vector_optimization
   is now identical; full logs 509 -> 510; make and make tags pass.
   Sweep (items 3/12): tactic exec cleans up only after success or a
   tactic_exception; other exceptions propagate uncleaned, and
   tactic2solver::check_sat_core cleans up once. g3's Exec always cleaned,
   so a qsat-bearing default tactic reset its kernels twice. Full logs
   510 -> 513 (pending_default_portfolio_option_error,
   basic_graph_mixed_model_refusal, utvpi_mixed_model_refusal).
   Lead at box line 80: z3's pb_rewriter keeps m_args (an expr_ref_vector
   member reset at the next non-constant rewrite), and that inc/dec frees a
   raw (not y!2), which g3 keeps. TRIED porting m_args: stdout went
   648 -> 649 (pb_simplex_proof), but pb_native_watches_scopes and
   pb_simplex_scopes stopped being identical and card2bv_weighted_models
   fell 148 -> 40, so it was reverted. Some g3 caller or rewriter
   lifetime differs; z3 frees card2bv's (not p) during the apply. Find
   that holder before re-landing m_args.
   CORRECTION/DONE (2026-11-10): in this tree pb_rewriter::m_args is a
   ptr_vector (no references). The holder is mk_app_core's local
   pb_ast_rewriter_util, whose m_refs keeps each negate() result
   (m.mk_not) until mk_app_core returns. g3's negate made raw nots that were
   never released; now ported (PseudoBooleanRewriter.negations). Full logs
   507 -> 508 (bounded_integer_optimization), box 79 -> 95,
   lexicographic_integer 70 -> 78, no regressions.
   box line 95 (2026-11-10): maxres add_soft's fml = (iff soft!k e) is a
   local, so only inc_sat_solver::m_fmls keeps it, until inc_sat_solver::pop
   in execute_box's scoped_push. g3's freshProxy gave it to the engine owner;
   now a local, and the iff frees match z3. The next difference is an extra
   arithmetic numeral that g3's model evaluation creates in execute_box. g3
   runs filterResultModel (fix_model: model converter plus m_fm) when the
   check finishes, for every box snapshot and for lex. z3 keeps
   m_model/m_box_models raw and runs fix_model in context::get_model
   each time a model is requested. NEXT (item 12 flow): move fix_model to
   model retrieval in cmd_context. This needs m_model_converter and m_fm
   to live with the opt::context (they do for the converter; s.filter is
   closed with the ArithmeticSolver).
   DONE (2026-11-10): runOptimize's optimizer keeps raw models
   (SetRawModels), and ArithmeticSolver.FixModel is fix_model. cmd_context
   applies it on every model retrieval (Model, modelAtIndex/get_box_model,
   results, model completion, OptimizationModel) while the published
   check-sat result is the raw one; the retained optimizer keeps the
   converter and m_fm alive. box_optimization's full log is now identical
   (508 -> 509); make and make tags pass.
   Next lead: at line 1182, in the old inc_sat_solver's bit_blaster
   m_values teardown, z3 frees (iff ..) #37/#26 before (or ..) #8. g3's
   delete worklist matches z3's, so a refcount (an extra or missing holder
   of #8) differs.
   NEXT: pareto_bit_vector (12), pareto_soft (29), box (38), pareto (38),
   bit_vector (42), strict_lexicographic (42). The pareto and box paths
   still skip normalize/update_solver (see PLAN below).
   Noted: g3's rlimit accounting differs from z3's. z3's OPB front end
   finishes at any budget, while g3 reports unknown below about 450
   (cmd/g3 OPB tests pin checkpoints).
   ROOT CAUSE for the propositional MaxSAT vectors (weighted_soft_optimization,
   pareto_soft, box, ...; 2026-11-09): opt::context::update_solver replaces the
   opt_solver with mk_inc_sat_solver when probe_bv holds (only Bool/PB/BV
   symbols, maxres-family engine, not pareto, no proofs), so every later
   check_sat, core and model comes from inc_sat_solver (whose
   internalize_goal runs the simplify tactic, hence z3's (not a) (not b)
   (or ...) before [push] 0). g3 computes that eligibility
   (cmd_context/optimization_sat_eligibility.go) but only uses it as a flag;
   opt.ArithmeticSolver.backend is a concrete *smtcore.SMTSolver. NEXT: put
   the backend behind the solver interface (check_sat with assumptions,
   get_unsat_core, get_model, push/pop, assert_expr) and construct
   sat/sat_solver's inc_sat_solver for the eligible case, copying the
   opt_solver's assertions as update_solver does. A full sweep
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
    OPEN (theory_arith now approved, 2026-11-09): unbounded objectives. In z3,
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
    DONE (d59ca8f62, 71d51408f and the follow-up): one-shot trace/print
    formats were never released, while z3's format_ref frees them after
    pp(). PrintFormatTransient owns and releases an unreferenced root,
    and every one-shot call site uses it. sat_dimacs is now identical;
    full logs 402 -> 406.
    Later sweep fixes (full logs 406 -> 408, stdout 647):
    - horn_tactic get_formula_kind locals (tmp, body) die on return, and
      f/q/queries are expr_refs; mk_query binds the expression query rule
      under forall over its free variables (horn_tactic_finite identical).
    - ctx-solver-simplify was a recursive reimplementation with fresh
      constants; now a port of reduce/simplify_bool/mk_fresh with the
      k!0xbeef101(id) proxies, the clean() wrapper (cleanup resets the
      kernel), and m_num_steps never incremented (ctx_solver_simplify
      identical).
    - QF_FPLRA has no mk_tactic_for_logic case and uses mk_default_tactic;
      mk_qffp_tactic's first simplify loses simp_p to the closing
      updt_params(p).

    More sweep fixes (full logs 408 -> 419): ufbv's smt stage is
    mk_smt_tactic_using(false, main_p) (no pattern database); factor
    tactic numeral/product order and per-helper to_expr; gcd rounding
    releases new_arg1; propagate_values push_result's raw new_pr;
    reduce_args mk_mc lifetimes; lackr vector release order; datalog
    mk_coi_filter honors the null model converter of rel_query.
    2026-11-09: the unbounded recovery is gone (geometric_lex asserts the
    false blocker); normalize's fmls are released after from_fmls; the
    objective terms/hard constraints and optsmt's rewritten objectives live
    in the context until the next import_scoped_state; to_expr is
    faithful; lia2card's is_pb builds no difference term.
    unbounded_real_optimization and unbounded_lexicographic_interval are
    identical. Lead: bounded_integer_optimization line 57: z3 frees
    (pble x!1 x!2) and the pb normalize's (not x!i) when simplify_fmls
    returns, so is_maxsat's get_pb_sum makes new negations; g3's final
    simplified formulas still hold them until after from_fmls (compare the
    simplify_fmls output formulas).
    Leads left:
    - integer_division_scopes: theory_arith mk_idiv_mod_axioms builds
      (- 0 k) before (< k 0) (GCC right-to-left mk_ite arguments);
      theory_arith may now be touched (approved 2026-11-09).
    - fixedpoint_relation_representations: rel_context::query builds
      m_answer eagerly (table_base::to_formula), which keeps a variable
      alive across later queries; Go's datalog answers are lazy and
      backend-specific.

23. **DONE (2026-11-09): fpa2bv_converter ported, reimplementation deleted.**
    Result: all 24 floating-point vectors match z3's trace=true log line for
    line, and the t_fpa/t_fpa_detail traces match too. Commits a276179d1
    (switch-over), be25ef062, e6d489931, bbbcd0d2b, a4f899500. Fixes found
    along the way:
    - BV_RM_VAL encoding was swapped in the converter port (away=0,
      even=1); z3 has even=0, away=1, positive=2, negative=3. The model's
      bv2rm decoder used z3's order, so a rounding-mode variable could get
      an excluded value as its model (wrong answer; regression vector
      qffp_rounding_mode_model_values).
    - theory_fpa hooks ported (theory_fpa_hooks.go): convert_atom/term/
      conversion_term, mk_side_conditions, internalize_atom, new_eq_eh and
      new_diseq_eh (replacing the eager InternalizeEquality bridge),
      assign_eh, relevant_eh with rm unwrap, wrap's th_rw, m_th_rw with
      arith_lhs=true, assert_cnstr's mark_as_relevant.
    - Core: reinsert_parents_into_cg_table only eq-propagates marked
      parents; theories pop in registration order; internalize_uninterpreted
      plus internalize_term apply the sort constraint twice.
    Original notes:
    fpa2bv_converter was a reimplementation, not a port.
    ast/fpa (about 5500 lines of Go) restructures fpa2bv_converter.cpp
    (4130 lines). Example: mk_numeral(sort, mpf) builds
    fp(bv_sgn, bvadd(e, bias), bv_sig) from the mpf's sign, significand
    and unbiased exponent. Go packs the IEEE bits into one numeral and
    extracts the fields, and Bias concatenates 0 with all ones instead of
    making the numeral 2^(ebits-1)-1. This is the first divergence in
    qffplra_solver (line 63) and likely in the other fpa vectors
    (fpa_canonical_model_scopes, qffpbv_tactic). It needs a function-by-
    function port, starting with mk_numeral/mk_bias/mk_unbias/mk_fp/
    split_fp and the rounding core, plus an AST-level reader for an fp
    numeral's sign, significand and exponent that makes no packed
    numeral.
    Plan (started 2026-11-09):
    a. Port fpa2bv_converter.cpp into ast/fpa as methods named after the
       source (mkAdd, unpack, round, addCore, ...), with expr_ref locals as
       released-in-reverse ref frames and raw nested terms, and m_simp as
       the real bool_rewriter (injected: ast/rewriter imports ast/fpa).
       Add ast readers for mpf numeral fields and the mpf operations the
       converter calls (fm().set/round/powers2), without making AST nodes.
    b. Port fpa2bv_rewriter.cpp's reduce_app/reduce_var/reduce_quantifier
       dispatch onto the source-named methods.
    c. theory_fpa's fpa2bv_converter_wrapped overrides (mk_const,
       mk_rm_const, mk_uninterpreted_function), the tactic, bv2fpa and the
       model converter switch to the ported maps (m_const2bv,
       m_rm_const2bv, m_uf2bvuf, m_min_max_ufs).
    d. Delete the reimplementation; check qffplra_solver,
       fpa_canonical_model_scopes, qffpbv_tactic and the other fpa vectors
       against z3 traces.

24. **DONE (2026-11-09): theory_str propagation order (string_value now
    matches z3 line for line).** Causes: theory_str set a theory var from
    apply_sort_cnstr (the source keeps theory's empty body, so a String
    variable gets its var at set_up_axioms and its equality arrives later
    through attach_th_var's replace branch); context::mk_eq_atom was not
    ported (theory override: arith_util::mk_eq ordering for arithmetic
    theories, id-ordered mk_eq otherwise); context::get_assignments'
    (not e) vector was not built in ctx_dep_analysis,
    classify_ast_by_type_in_positive_context and propagate_length.
    Remaining in the other string vectors (generic_string_*_free_models,
    qfs_legacy_string_free_models, line 251): the format manager's node
    ids inside a justification display differ (#16 vs #24); g3's other
    theory_str MkEq sites (about 120) should be checked against the
    source's ctx.mk_eq_atom uses as traces expose them.

21. **Recorded deviations, kept deliberately.** cmd/g3 tests encode
   these: bvsmul_noovfl numeral folding uses signed semantics (the source
   compares the unsigned product with 2^(sz-1)); nlqsat answers goals the
   source rejects as 'not NRA' (is_pure_proc is not ported; the test keeps
   the C++ unknowns as referenceWant). Ask before changing either.
   Also kept: symbolic_automata::mk_product tests is_final_state without
   the epsilon closure, so ("" | a) & ("" | b) would be empty; g3's
   product includes the closure (string_regex_automaton_product_test).

26. **Noted (2026-11-10): source LRA wrong model after pop, reproduced
    faithfully.** Under QF_LRA, `(assert (< (+ x y) 2.0))`, then push,
    `x >= 1`, `y >= 1` (unsat), then pop: the next check-sat is sat with
    the model x = y = 1. Both z3 4.7.1 release and g3 print that model:
    lar_solver::pop keeps the conflict's column values, and final_check
    skips make_feasible because the popped status is OPTIMAL. In a debug
    build (g3debug tag; a Z3DEBUG source build would do the same),
    get_model's update_delta SASSERT(l <= u) fires.
    TestSMTSolverSetupUsesLinearRealArithmetic turns model building off
    for that check under the debug tags. Note that build_dbg/ is an
    NDEBUG build, so it does not show the assertion. Ask before fixing.
    Similarly, a bit-vector over m_bv_blast_max_size stays unattached and
    takes mk_extra_fresh_value with a finite sort. Release builds return
    a value; debug builds fail SASSERT(s->is_infinite())
    (TestSMTBitVectorBlastLimitGivesUnknown turns models off under the
    debug tags).

27. **Recorded deviation (2026-11-10): relevancy repair after option
    updates.** smt_solver::updt_params re-reads every smt param into the
    smt_params that the context shares, so a later `set-option` turns
    m_relevancy_lvl back to 2 after setup chose 0. Base-level atoms
    internalized while relevancy was off never get marked relevant, and
    z3 4.7.1 then omits them from models. In
    lra_propagation_mode_updates, the final model is empty, which still
    matches stdout. relevancy_update_scopes shows an assertion-violating
    model. g3 marks the cached assertions relevant again before each
    check (SMTKernel.repairRelevancy, commit 2a8d0fa63);
    TestSMTLibRunnerRelevancyUpdatesKeepModelsValidWithKnownCPPDefect
    encodes that. As a result, the
    lra_propagation_mode_updates trace builds one extra model numeral.
    Ask before removing the repair.
    Checked (2026-11-11): with the repair disabled (the `if
    k.repairRelevancy && k.relevancy.enabled` block in the check path,
    smtcore/smt_kernel.go), lra_propagation_mode_updates and
    arithmetic_eq_options both match z3's full trace exactly. They are the
    only vectors in the comparison set that the repair affects.
    DECIDED (2026-11-11): keep the repair; see "z3 4.7.1 bugs fixed in g3".

28. **Mostly done (2026-11-11).** quant_seq_symbolic_int_prefix now
    matches z3 for the whole trace g3 writes in 30s (578684 lines; z3
    writes 15.3M lines in that time, since g3 writes the trace one byte
    per syscall). The fixes:
    - f68224d31: process_justifications bounds each m_todo_js pass by its
      size before the pass, so the 28478 resolve order matches.
    - cd38ee773: theory_seq's get_num_value, lower_bound, upper_bound and
      get_length hold the arithmetic value in an expr_ref (61428).
    - 0ec94b600: the reinit selection walks m_clauses_to_reinit buckets,
      including re-pushed older lemmas (95570).
    Remaining: the trace writer's per-byte writes (a shared file-level
    buffer), after which longer comparisons are practical.
    History: **Partly done (220c9c04a): the extra compress case is
    removed.** quant_seq_symbolic_int_prefix's stdout matches and its
    trace matches to line 28478. Still open: the merge-order divergence
    at 28478 (below). Note: with trace=true, g3 writes the trace file one
    byte per syscall; this vector's trace then exceeds fullcmp's 30s
    limit. A per-stream bufio broke ordering, because borrowed streams
    share the fd. Buffering needs a shared file-level writer.
    Original note: **automaton compress has a case z3 lacks.**
    pkg/math/automata/automaton_compress_source.go has a final case
    (`out_degree(src) == 1 && (final(src) || !final(dst))`: copy dst's
    out-moves to src) that z3 4.7.1's automaton::compress does not have.
    Because of it, `(re.+ (re.range "0" "9"))` compresses to
    "0 -> 1, 1 -> 1" instead of z3's "0 -> 1, 1 -> 0 (epsilon)".
    Removing the case makes quant_seq_symbolic_int_prefix's trace match
    up to line 7159 (from 411), but the first check then returns unknown
    instead of sat: a later theory_seq divergence (a decision on
    (= (str.len (int.to.str n)) k) in z3 against a propagated
    disequality in g3). Fix that divergence before removing the case.
    TestAutomatonEpsilonClosureFinalLoopsAndSequence would also need to
    stop requiring an epsilon-free result.
    Progress (2026-11-10, 016885d4c): the divergence was is_string missing
    seq.unit byte numerals (so solve_itos never reduced (int.to.str n) !=
    "0" to n != 0) plus arith_util::mk_add(1 arg). With those fixed and
    the compress case removed, the first check is sat and the trace
    matches to line 28478. There, a 4-literal conflict lists
    (not #258) first in z3 and last in g3, and the later check-sat
    (line 6 of stdout) then runs past 60s. Still open: find that
    conflict's source (not seq setConflict/propagateLit, whose order
    matches). Found: it is an eq_conflict_justification (two interpreted
    roots merged). Its get_antecedents, the LIFO m_todo_eqs drain and
    eq2literals all match the source, so the different antecedent order
    must come from the enode proof tree (m_trans edge direction chosen
    in add_eq/merge, e.g. class-size or interpreted-root swaps). Next
    step: compare merge directions on this vector (trace the
    [merge]/add_eq records). Done: -tr:add_eq "merging" records agree
    for the first 335 merges; merge 336 differs (z3 merges #297 into #316
    and then #50 with #298; g3 merges #298 into #316 and then #50 with
    #297). #297 = (= #278 #192) and #298 = (= #72 #161) are both Boolean
    equality atoms, so the order in which the two equality propagations
    reach add_eq differs (likely a propagation-queue or theory eq-
    propagation order in the seq/arith interface). The full trace still
    matches there because format-node ids overlap and these merges
    leave no [assign] record.

29. **DONE (2026-11-10): fixedpoint_relation_representations.** The
    (iff var true) is table_base::to_formula in rel_context::query's
    eager m_answer; g3 now builds it when a finite-table query succeeds.
    Still lazy: query clones (expression queries), which
    TestExpressionAnswerAllocationFailureKeepsPublishedQuery relies on,
    and the Karr/udoc/interval backends. Original note: z3 builds
    `(iff var true)` for each query of a Bool relation (after the query
    application `(Sparse x)`); g3 does not. The source is neither
    ensure_app, fix_unbound_vars nor mk_query (gdb shows no hits there);
    the next candidates are the rel_context query path and relation
    plugins.

30. **RESOLVED (opened 2026-11-10): opt pareto pipeline order.** STATUS 2026-11-13 (24393536c): every test vector is trace-identical to the fork (fullcmp2 802 identical / 0 differ, 11 traced timeouts; with Z3_TRACE_NO_PP 813 identical / 0 differ / 0 timeouts), including the vectors named here, so this item is resolved for the test corpus; the notes below are history. In
    pareto_bit_vector_optimization, z3's opt::context::optimize runs
    init_solver first: the opt_solver's smt::kernel builds the qi_queue
    cost-parser variables, then the objectives are purified into fresh
    maximize!!N constants, and only then are the objectives bit-blasted.
    g3 bit-blasts the objectives before it builds the solver, and never
    creates the maximize!! constants. Port opt::context::optimize/normalize
    in source order.
    Progress (2026-11-11, 4e21f16f3, which replaces the reverted
    029b6fe84):
    - The first pareto check runs init_solver and normalize, and
      purifies against from_fmls' hard constraints.
    - The pareto state holds normalize's model converter for every
      frontier point and applies it as operator()(md, 0); the
      extension converter's unindexed form was a no-op.
    - Soft costs include from_fmls' adjust offset.
    - The pareto vectors' logs advance to 224/78/32 (from 12/38/29), and
      stdout still matches.
    Open:
    - Fixed (2026-11-11): the second-check unsat after a single soft
      objective under pareto (or box). optimize() runs a lone objective
      as execute(obj, true, false) whatever the priority; its m_pareto1
      toggle is cleared by the next clear_state before it is read, so z3
      answers sat on every check. Not specific to negative weights: a
      single maximize objective did the same, and box too. Two further
      fixes the negative-weight case exposed: maxsmt_solver_base::init
      evaluates the initial model without completion (maxres then runs
      its assumption check), and simplify_fmls' fmls.reset() releases
      to_fmls's objective applications before from_fmls's mk_not.
      Vectors: pareto_single_negative_soft,
      pareto_single_objective_negative_softs, box_single_objective_rechecks.
    - Fixed (cdbaef450): pareto now takes over init_solver's opt_solver
      (no second kernel), and pareto_bit_vector reaches line 2124.
    - Fixed (2026-11-11): a fresh pareto check now runs optimize()'s
      initial check_sat, optsmt::setup's empty scoped push, and then
      gia_pareto's own check_sat. pareto_bit_vector reaches 4396 and
      pareto_soft 58.
    - pareto_optimization (x, y in [0,3]) stops at line 80 because the
      MaxSAT-conversion fallback applies to it, yet z3 terminates on it.
      The fallback cannot predict which inputs z3 finishes on (z3 hangs
      on the 0-1 case with (= (+ x y) 1) but not on this one). Decision
      needed: either drop the fallback, which is faithful and makes
      TestOptimizationArithmeticEngineSetupSkipsParetoContinuation
      hang as z3 does, or keep it.
    - TestParetoCanceledImprovementDropsBackendAndAllowsFreshSearch is
      brittle: a resource limit hit while building the candidate model
      returns an error instead of unknown (budgets 5000-12000 fail).
    - Recorded ask-first deviation: z3 4.7.1 does not terminate on 0-1
      arithmetic pareto objectives once normalize makes them MaxSAT, so g3
      keeps the un-normalized path there.
    - Fixed (2026-11-11): pareto_soft_optimization and
      pareto_bit_vector_optimization logs are identical. mk_dominates and
      mk_not_dominated_by assert while their comparison vectors are alive
      (the asserted rewriter caches the conjuncts); set_pareto(nullptr)
      keeps the opt_solver until the next init_solver; a fresh pareto check
      after a finished one runs optsmt::setup's push/pop again; optimize()'s
      initial-check model (m_model) lives until yield; a continued
      execute_pareto skips normalize and objective conversion; the purifier
      releases m_objectives (m_terms then m_term) before
      m_hard_constraints, front to back; m_objective_refs keeps every
      maximize!!N declaration for the context's lifetime (the bv1 ite decl
      id then lands above the extract decl ids, so propagate_values orients
      (= extract ite) as z3 does).
    - pareto_optimization is identical once the MaxSAT-conversion fallback
      (paretoObjectiveBecomesMaxSAT) is removed; g3 then hangs on the 0-1
      case with (= (+ x y) 1) exactly as z3 4.7.1 does (checked with both
      binaries). The removal is the ask-first decision above.

31. **RESOLVED (started 2026-11-11): PDR/spacer query pipeline lifetimes (about 40
    STATUS 2026-11-13 (24393536c): every test vector is trace-identical to the fork (fullcmp2 802 identical / 0 differ, 11 traced timeouts; with Z3_TRACE_NO_PP 813 identical / 0 differ / 0 timeouts), including the vectors named here, so this item is resolved for the test corpus; the notes below are history.
    fixedpoint_* and fp_spacer_* vectors).** Plan: pkg/PDR_PORT_PLAN.md.
    Update (2026-11-11, 15a376729): every spacer vector now matches the
    source trace; the full logs are 639 identical / 21 differ, stdout
    651/662. Fixed in this stretch (each its own commit): compute_implicant
    literal lifetime (K2); Farkas plugins negate with ast_util mk_not;
    prop_solver::maxsmt soft-vector reset, `saved`, and scoped_mk_proxy
    before check_sat; theory_arith keeps m_random across updt_params;
    eq_conflict_justification prefetch order (K1); create_children's phi1
    consumed by qe_project while Phi lives on; unsat_core_learner's free
    reduce_hypotheses and original-root traversal; get_ground_sat_answer's
    standalone kernel, raw one-literal and, and datalog m_last_answer cache;
    rule_dependencies + rule_stratifier ported over obj_map table orders
    (RuleSet.SourceStrata) for the inliner's cycle breaking and
    mutual-inlining order; kernel teardown in context::flush order (trail
    before clauses, conflict resolver after the flush, clause atoms first
    to last); bit-vector expansion with OP_BIT1 after the extract and the
    OP_DT_IS recognizer (also in PDR); lemma body, array generalizer,
    expansion vector and model_evaluator_util lifetimes; updt_params no
    longer flushes the asserted rewriter (set_eliminate_and no-op when
    unchanged); FlattenAndOwned for is_invariant's conj;
    get_origin_summary's summary[i] = v release.
    Remaining PDR lead (pdr_recursive_trace 9291, pdr_initial_trace 12934):
    `(not #515)` is cached by g3's MaximizeBVSharing rewriter at the
    asserted pass because its refcount is higher than z3's then; z3 frees
    it when pdr expand_node's `cube` vector dies. The G3_RC_ID history
    ($W/build_g3rc.sh) shows the extra holders come from the proof-mode
    TheoryRewriter roots/caches during PropSolver.check (makeSafeAssumptions
    and the asserted preprocessing). Compare with z3's
    prop_solver::check_safe_assumptions and asserted_formulas lifetimes.
    Spacer plan (2026-11-11): pkg/SPACER_PORT_PLAN.md covers the 15 differing
    spacer vectors (query flow Q1-Q4, transformer T1-T6/I1/R1, solve S1-S2,
    kernel K1-K4) with a porting order and per-step checks.
    Done so far:
    - D1, the member rule_substitution: 7b8a4cca5.
    - D2, nested simplifiers on the context rewriter: 3f1cc9e78.
    - D3, the coalesce/unfold block and inliner teardown order: ced248afc.
    - The init_rule locals: d008dfdf3.
    - The mk_slice update_rule locals: eff9414ba.
    - The context::init_rules obj_map order: f884bc751.
    - D11, reduce_disequalities: 945143dae.
    - Named pdr_proxy constants: 58bfef495.
    - Every context scope pushes/pops asserted_formulas' m_bv_sharing:
      f2a3352ec.
    - pdr is_invariant: raw mk_not and pm.mk_and: 23f3f72da.
    - spacer virtual_solver resets m_proof first; elim_aux_assertions
      pins only the proofs it creates: c00abc6d8.
    - create_children implicant via m_brwr.mk_and; array utils use the
      raw mk_and and own no-proof th_rewriters: ae62458ad.
    - simplify_bounds under scoped_no_proof; the adhoc normalizer
      reduces before it builds: b04ac59ca.
    - One unit_subsumption_tactic per simplify_formulas (spacer and
      pdr); prop_solver, pred_transformer and context teardown order:
      ad647a088 (and the premise fix 4d90cdee5).
    - Diff logic adds bound axioms only under m_arith_add_binary_bounds:
      27c6dd86a.
    - PDR farkas: constr::get, local extract rewriter, get_lemmas'
      simplify_lemmas: 86866a840, 41268536e. mk_assumptions tmp1:
      efa1d642d.
    - A registered theory_arith sees setup-time arith params (pdr's
      eager_eq_axioms=false): d2df0e13b.
    - Conflict proofs: lazy literal2expr (88c649042), m_new_proofs pins
      (f5eaedde8); def-axiom (not true) for false_literal (36cbbbd2e).
    The survey shows 32 of 55 identical, and the full logs are at 612/659.
    Progress (2026-11-11): the bv-sharing lead is fixed. mk_safe's conjs
    slot is overwritten by the proxy, dropping the expanded literal's
    reference (985bb9536). create_children renames o_cube =
    mk_and(child_states[i]) once into a caller-owned n_cube, instead of
    renaming each literal through SymbolMux.ConvertFormula, which keeps
    every result in m.references for the mux's lifetime (13c655f04).
    pdr_sliced_compound_head and pdr_arithmetic_invariant now match;
    pdr_initial_trace is at 12926 and pdr_recursive_trace at 9291. Next:
    (1) other ConvertFormula callers have the same mux-lifetime retention
    and should move to caller-owned refs where the source uses a local
    expr_ref; (2) at pdr_initial_trace 12926, (= #38 #29) from
    qe-lite's abstraction outlives z3's copy. (3) pdr_background 548:
    `query!0 #25` (mk_query's head application) is held by g3's query rule
    in e.set until removeQueryRule after solve; z3 has freed it before the
    first true-axiom. z3's dl_interface::query calls replace_rules(old_rules)
    before solve, but check_reset's m_old_rules also keeps the post-mk_query
    rules, so z3's rule head is likely a different term (mk_rule's variable
    normalization) and the original app dies with mk_query's locals.
    Compare g3's makeQueryHead/newRuleWithHeadPolicy with
    rule_manager::mk_query/mk_rule. Tools: $W/build_g3rc.sh
    (G3_RC_ID=n logs every inc/dec of that main-manager node with a
    short stack into the trace).
    Open lead: fixedpoint_pdr_initial_trace (also recursive_trace and
    sliced_compound_head), where (<= #53 #29) stays in g3's
    MaximizeBVSharing rewriter cache until kernel close, but z3 frees it
    at the asserted_formulas pop. The likely cause is a different refcount
    at the bv-sharing rewrite, which made the node cacheable. Checked
    (2026-11-11): the asserted pass sequence matches z3. Under gdb, z3's
    maximize_bv_sharing rewriter never inserts #246 or #249 into its
    act_cache; z3 caches #246 only in the main rewriter, which
    flush_cache drops at the pop. g3's bv-sharing rewrite of root #250 =
    (or #249 #246) sees #246 with refcount 5 and caches it. Next step:
    list #246's holders in g3 at that point. Second open
    lead: fixedpoint_spacer_datatype_state line 2470, where g3's model
    evaluator (ArithRewriter.cancelMonomials -> numeralForTerm) creates an
    Int numeral that z3 still holds alive. Find the holder z3 keeps. Remaining:
    - bucket A: quantifier_abstraction (the hnf m_body holder),
      array_blast and array_eq_rewrite (rewriter cache), bit_blast,
      mutual_inv, nonlinear_inlining, quantifier_instantiation,
      pdr_background and pdr_integer (D6, cross-query retention);
    - bucket C (D10, spacer asserted(true));
    - further PDR solve-phase differences. g3's
    muz/fp queryRelationMkQuery is a reimplementation of
    engine_base::query -> pdr::dl_interface::query ->
    apply_default_transformation. Term lifetimes differ as a result. In
    fixedpoint_pdr_bit_blast, `(p x)` and `(= x x)` survive in z3 until
    ~mk_interp_tail_simplifier, when the rule transformer is destroyed
    at the end of apply_default_transformation; g3 frees them before it
    builds `query!0!bv!2`. Port the transformer ownership structure.

32. **DONE (2026-11-11, d998ec55f): m_clauses_to_reinit order.** The pop
    now restores learned clauses by (bucket level, push order), and
    bv_integer_conversion_scopes matches z3's full trace (182940 lines).
    Stdout is unchanged at 651/662. Correction to d998ec55f's message:
    qe_cooper_symbolic_blast (line 53) and quant_seq_symbolic_int_prefix
    (line 28478) still differ; they only left fullcmp's list because they
    time out at 30s. History:
    **Pretty-printer format ids (2026-11-10).** Fixed: get-consequences
    prints its vector as one ast_smt2_pp document (82381f483), and
    theory_str tester names release their mk_ismt2_pp document
    (7e04eec53). Three generic/legacy string vectors now match. Still
    open: bv_integer_conversion_scopes line 92987, where z3 reuses a live
    format string (#238) that g3 has already freed and rebuilds. Track
    format-manager deletions to find the earlier lifetime difference.
    Re-analysis (2026-11-11): this is not a format lifetime issue. The
    deletion sequences match. The clause printed by "[assign] #159
    clause" (line 92660) lists its last two literals in a different
    order. z3 has (not (let ((a!1 (+ ... (* -1 (mod x 16))))) (<= a!1 0)))
    before (not (>= (ite ((_ bit2bool 3) ..) 8 0) 8)); g3 has the
    reverse. This is an arithmetic (bv2int/int2bv bound) propagation
    clause, so its antecedent collection order is the next thing to
    check (likely theory_arith).
    Root cause found (2026-11-11), in the SMT core rather than
    theory_arith:
    - The clause comes from theory_arith assign_bound_literal
      (mk_clause CLS_AUX_LEMMA). Both sides sort it by literal index, so
      the order follows bool-var numbering.
    - The bool vars of atoms #61, #209 and #218 are created in a
      different order. In z3 all three are re-internalized by
      reinit_clauses in pop_scope_core: 61, 209, 218. In g3
      (restoreLearnedClauses) the lemma [62 191 137 228 218 61] is
      re-initialized before [56 61 209 137 159 218 81]. So 218 comes
      first, and 61/209 are created later by the arith eq adapter.
    - The cause: z3 keeps m_clauses_to_reinit[level] buckets. mk_clause
      puts a lemma in the bucket of its atoms' max intern level
      (smt_internalizer.cpp:1139). reinit_clauses walks the buckets in
      ascending level order and re-pushes kept clauses into the current
      level's bucket (smt_context.cpp:2201-2300). g3's
      preserveLearnedClauses/restoreLearnedClauses instead walk s.clauses
      in creation order.
    - Fix: port m_clauses_to_reinit (bucket on creation, ascending walk,
      re-push on keep) and drive restoreLearnedClauses from it. This
      touches every log's search, so plan it and run the full comparison
      and stdout checks.

33. **DONE (2026-11-10): rewriteAdd and model finder analysis order.**
    g3's add rewrite has mk_nflat_add_core's BR_FAILED exits (7a6d9b7d7).
    The line-917 difference came from the model finder.
    populate_inst_sets now walks one qinfo order (8c6134371). The
    quantifier_analyzer traversal is the source worklist, with m_ftodo
    and m_ttodo as LIFO stacks; visit_term shares m_pos_cache, so a
    Boolean term's positive visit is skipped (d9c4bd2d0).
    cleanup_func_interp uses the source todo stack and trail (c49a25293).
    A copied theory_seq builds its kernel at register_plugin
    (ef71015dc). The first diff in recursive_mutual_and_arguments moved
    from line 916 to line 1135.

34. **DONE (2026-11-10, e1f607765): persistent auf_solver.** In z3,
    model_finder::m_auf_solver lives with the model finder. process_auf
    resets it at its start: flush_nodes deletes the nodes in m_nodes
    order, each root deleting its set in table order. set_model resets
    the eval cache. The eval cache (m_eval_cache[2] plus the
    m_eval_cache_range trail) memoizes model evaluations.
    get_uvar_inst_set reads the node root set, or
    qinfo->get_macro_based_inst_set for macro-satisfied quantifiers.
    ~model_finder runs reset() (the quantifier_infos) and then destroys
    its members in reverse order. ~auf_solver runs flush_nodes and then
    reset_eval_cache.
    g3 builds a transient graph per FixModel, copies each variable's
    set into per-quantifier f.sets, re-evaluates without a cache, and
    releases sets per quantifier. As a result, recursive_mutual_and_arguments
    line 1136 recycled ids in a different order, because the sets and
    eval results died in a different order. g3 now follows the source
    lifecycle, and recursive_mutual_and_arguments is identical.

35. **RESOLVED (opened 2026-11-11): qe tactic top-level flow.** STATUS 2026-11-13 (24393536c): every test vector is trace-identical to the fork (fullcmp2 802 identical / 0 differ, 11 traced timeouts; with Z3_TRACE_NO_PP 813 identical / 0 differ / 0 timeouts), including the vectors named here, so this item is resolved for the test corpus; the notes below are history. qe_cooper_symbolic_blast
    (now no longer timing out) differs at line 53: z3's qe_tactic runs
    expr_quant_elim::elim -> quant_elim_new::eliminate_exists_bind ->
    eliminate_block, which builds the plugins (mk_arith_plugin's
    arith_qe_util numerals come first) and then the quant_elim_plugin
    search. g3's rewriteQuantifierScalar
    (qe/quantifier_tactic_bool_recursive.go) runs its own cascade
    (eliminateQuantifierBoolUnits, ...ArithmeticEquations,
    ...ArithmeticClosed, ...ArithmeticSearch, ...), so
    ConjunctionEnum.ExtractEqualities runs before any plugin exists. The
    ported search tree (qe/expr_quantifier_tree*.go) is already there;
    a faithful fix routes the tactic through eliminate_block with the
    plugin set and deletes the cascade. Large port.
    Plan (2026-11-11): (1) a generic eliminate_block(vars, fml) built from
    eliminateQuantifierScalarSearch (qe/quantifier_tactic_arith_search.go)
    that registers bool, bv, arith, array, datatype and dl plugins in
    pop_context's order on one SolverContext, with all of the block's
    variables; (2) quant_elim_new::eliminate_exists (per variable unless
    eliminate_variables_as_block) and eliminate_exists_bind/forall_bind;
    (3) expr_quant_elim::elim's todo walk calling it, with m_plugins reuse
    (push_context/pop_context keep one quant_elim_plugin); (4) delete the
    cascade in rewriteQuantifierScalar and the eliminateQuantifier*
    strategy entry points. Check every qe_* vector stays identical.

36. **DONE (2026-11-11, ee08ebb36): theory_str mk_eq_atom audit.** 107
    sites now use MkEqAtom; the source's raw m.mk_eq sites (concat length
    axiom, value/length option testers) keep MkEq. No vector changed;
    the orientation tests were updated. Original note: z3's theory_str
    builds equalities with ctx.mk_eq_atom (461 sites; raw m.mk_eq only
    at lines 1038, 10734, 11069-11110 and 11714). g3's theory_str still
    calls m.MkEq at about 115 sites, so String equalities can keep the
    wrong orientation (mk_eq_atom puts the lower id on the left; Int
    sides go through arith_util::mk_eq). ed791f2de fixed the
    automaton-path sites. Convert the rest site by site against the
    source. Related: attachBooleanEnode sets mergeTruthValue on an
    existing enode without a trail (smtcore/smt_internalizer.go), which
    context::mk_enode never does.

37. **RESOLVED (started 2026-11-11): remaining full-log differences outside
    STATUS 2026-11-13 (24393536c): every test vector is trace-identical to the fork (fullcmp2 802 identical / 0 differ, 11 traced timeouts; with Z3_TRACE_NO_PP 813 identical / 0 differ / 0 timeouts), including the vectors named here, so this item is resolved for the test corpus; the notes below are history.
    PDR/spacer.** Full logs are 620 identical / 40 differ (stdout 651/662
    unchanged). Fixed this session, each now matching z3's whole trace:
    quant_seq_regex_backjump, quant_seq_bounded_regex_backjump
    (ed791f2de), qfufbv_ackr_incsat_lazy/_eager (b16d97a91),
    sat_pb_bv_pipeline (79db10e61), array_canonical_constructor_reads
    (e3f1bbe02), bv_integer_conversion_scopes (d998ec55f). Moved further:
    card2bv_weighted_models 148 -> 39844 (45f4be0c2, QF_FD takes
    setup_unknown), bv_fixed_cache_scopes 1834 -> 4191 (c06c89c5c, gate
    and root clause proof wrappers). Open leads:
    - card2bv_weighted_models 39844: at the final kernel close, z3's
      descending pop sweep frees the bit2bool atoms (#107/#106, ...)
      in place. g3 frees them after the sweep, so the bool-var and enode
      trail order (or an extra holder) differs.
    - DONE (fd36cddc0): bv_native_operator_models. simplify_clauses
      skipped binary clauses in z3 but deleted/counted them in g3.
    - bv_fixed_cache_scopes 4218 (fd36cddc0, 434ecc52f moved it from
      4191): at the user pop g3 frees the inner scope's assertion terms
      ((f x), (f y), the distinct, and its `rewrite` proof #121 from
      asserted_formulas::assert_expr's th_rewriter). z3 keeps them until
      the next assert_expr, where th_rewriter_cfg::reduce_app's
      `result_pr = nullptr` (called from process_const with m_pr) drops
      #121. So some rewriter member (m_pr, or a proof result stack entry)
      still holds the last proof after main_loop. Find it with gdb and
      model it in ast/rewriter's engine (r.pr).
    - DONE (2026-11-13): qe_rec_free_boolean (z3 segfaulted) and
      pb_strengthening_proof (z3 aborted at context destruction) are now
      comparable: the fork carries the qsat backport (upstream 1ce0d7512 /
      8ad939a10) and a theory_pb heap-justification fix, and g3 ports both.
    - lra_propagation_mode_updates and arithmetic_eq_options: item 27.
    - nla2bv_real_models 1711: in bv2real_util::align_divisors
      (mk_le -> align_divisors -> mk_bv_mul(rational, t)), z3 frees a
      transient bv numeral (#76) right after building the bvmul. g3 keeps
      it, so a later numeral gets a fresh id. Compare
      tactic/arith/bv2real_util.go mulAdopting/alignLocals with
      mk_bv_mul's s1/t1 locals and mk_extend's numeral case.
    - nlqsat_*, datatype_cycle_proof: recorded ask-first deviations.

## Status 2026-11-11 (late): full-log 647 identical / 13 differ; make tags green

Newly identical: fixedpoint_pdr_initial_trace (get_rule/update_models in the
caller's proof mode, asserted_formulas proof handling without proofs,
diff-logic/utvpi internalize_eq_eh, get_trace ownership),
card2bv_weighted_models (theory_bv mk_bits keeps only literals),
sequence_word_equations_relations (scoped_vector::pop_scope front to back).
Moved: fixedpoint_pdr_recursive_trace -> 22942, fixedpoint_finite_relations
97 -> 308. Open leads:
- fixedpoint_finite_relations 308: record_transformed_rules ->
  rule_set::reset releases rules front to back (rule_ref_vector::reset);
  g3's RuleSet.resetContentsUnchecked (muz/base/rule_set.go) goes back to
  front. Touches every rule set: re-check all fixedpoint/spacer/pdr vectors.
- fixedpoint_finite_relations 342 (after b3cd9aaee): z3's `(query R)`
  command calls context::rel_query, which never sets m_mc, so
  m_context.get_model_converter() is null for the whole session and no
  datalog transform (rule inliner hsmc, coi filter, slice, scale, ...)
  captures anything. g3's datalog path always captures inliner/cone
  conversions (TakeModelRules) and uses them in datalogModel ->
  expandDatalogModel, so it holds the deleted rules' heads (e.g. Small #40)
  until the next query. ASK BEFORE CHANGING: matching z3 means not capturing
  on the rel_query path, which drops inlined predicates from g3's datalog
  models (horn tactic / context::query paths keep a converter).
- DONE: fixedpoint_pdr_recursive_trace is identical (prop_solver member
  order, smt_context_manager kernel/predicate order, m_bv_sharing position).
- qe_cooper_symbolic_blast (item 35): a work-in-progress port of
  expr_quant_elim / quant_elim_new / eliminate_block that replaces the
  quantifier_tactic cascade is saved (NOT applied, its ./qe tests fail) at
  /var/tmp/g3-job-487c9ee8/patches/qe_cooper_eliminate_block_wip.patch;
  it moved the first diff 53 -> 92. Next: quant_elim_plugin::pop's
  solve_vars over all plugins (arith_qe_util::extract_equalities, then
  array conj_enum::extract_equalities) instead of solveExprQEBoolUnits.
- datatype_cycle_proof: ee39d9490 changed this recorded ask-first deviation
  (occurs_check_explain no longer adds the extra closing pair); the vector is
  now identical. Revert with `git revert ee39d9490` if that was unwanted.

## z3 4.7.1 bugs fixed in g3 (decided 2026-11-11)

Two upstream z3 fixes were backported into the fork's src/ AND ported into
g3 (2026-11-11), so g3 and the fork agree again and those vectors are back
in identity testing:

- **Relevancy** (upstream 5dfe4a4b4): the smt context keeps its own
  m_relevancy_lvl, 2 until the first setup_context, then the configured
  level; later setups only lower it. z3 4.7.1 let a later
  set-option :smt.relevancy 2 re-enable filtering over assertions
  internalized without relevancy, giving models that violate them
  (relevancy_update_scopes printed p = false). g3's former repairRelevancy
  workaround is gone. Fork: src/smt/smt_context.{h,cpp},
  smt_internalizer.cpp. g3: SMTKernel.setupContextRelevancy and the array
  theory's register-time relevancy (theory_array::init). Note: the case-split
  queue warning still reads the parameter, because the queue is created in
  the context constructor.
- **Pareto hang** (upstream e18702330, fix #1699): opt_context::mk_cmp
  evaluates objectives with model completion. Without it an objective the
  model leaves undefined (a 0-1 variable introduced by preprocessing) stayed
  symbolic, the dominance bound was trivially true, and gia_pareto looped
  forever (x, y in [0,3] with x + y = 1 or 3). Fork: src/opt/opt_context.cpp.
  g3: cmd_context mk_cmp/pareto evaluation; the non-z3 MaxSAT-avoidance
  fallback was removed.

The deliberate deviations below remain. Their vectors are listed in
pkg/cmd/g3/test_vectors/z3_bug_exclusions.txt and left out of z3 identity
testing (main_test.go carries a commented-out entry pointing there):

- **Regex intersection product ignores epsilon closure** (was part of
   item 21).
   - Reproduction: pkg/cmd/g3/test_vectors/z3_bug_automaton_product.smt2:
       (declare-const x String)
       (assert (str.in.re x (re.inter (re.union (str.to.re "") (str.to.re "a"))
                                      (re.union (str.to.re "") (str.to.re "b")))))
       (check-sat)
     z3 4.7.1 answers unsat; the correct answer is sat with x = "" (g3).
   - Cause in z3: symbolic_automata::mk_product
     (src/math/automata/symbolic_automata_def.h) decides final product
     states with is_final_state, which does not follow epsilon moves. The
     automata for ("" | a) and ("" | b) reach their final states from the
     initial state only through epsilon transitions, so no product state
     is final and the intersection is empty.
   - g3 fix: sequenceAutomaton.closureIsFinal
     (pkg/ast/rewriter/seq_rewriter_automaton.go) uses the epsilon closure.
     Test: string_regex_automaton_product_test.
   - Excluded vector: sequence_helper_inventory (its trace first differs at
     line 21357, where the product automaton's final states differ).
   - Upstream: no fix to borrow; src/math/automata was deleted by
     fcd3a70c9 (2025-08-07), and z3 4.17.0 answers sat with x = "".

- **Model evaluation of recursive predicates never terminates** (z3 4.7.1;
  upstream z3 4.17.0 answers at once through its recfun theory, a redesign
  rather than a small fix, so nothing to backport).
   - Reproduction: pkg/cmd/g3/test_vectors/recursive_mutual_boolean_model.smt2
     (define-funs-rec even/odd, then get-value of (even 0)): z3 4.7.1 never
     finishes; g3 prints ((even 0) true) ...
   - Cause in z3: rewriter_tpl::constant_fold (src/ast/rewriter/rewriter_def.h)
     folds only ite before visiting arguments, so the model evaluator rewrites
     both or/and arguments of (or (<= n 0) (odd (+ -1 n))) and the recursion
     never reaches a reduced base case.
   - g3 fix: Traversal.EnableModelBooleanShortCircuit
     (pkg/ast/rewriter/traversal.go), enabled once a recursive function is
     defined. Excluded vector: recursive_mutual_boolean_model; comparable
     companion: recursive_mutual_safe_get_value.

- **qe array solve_store ignores coinciding store indices** (found while
  porting item 35; STILL PRESENT in upstream z3 4.17.0 at 30c74fcce, so a
  candidate for an upstream PR).
   - Reproduction: pkg/cmd/g3/test_vectors/z3_bug_qe_nested_store.smt2:
       (declare-const b (Array Int Int)) (declare-const i Int) (declare-const j Int)
       (declare-const v1 Int) (declare-const v2 Int)
       (assert (exists ((a (Array Int Int))) (= b (store (store a i v1) j v2))))
       (assert (= i j)) (assert (= v1 0)) (assert (= v2 1))
       (check-sat-using (then qe smt))
     Both z3 4.7.1 and z3 4.17.0 answer unsat; plain (check-sat) says sat,
     which is correct (the outer store overwrites v1).
   - Cause in z3: qe_array_plugin::solve_store (src/qe/qe_array_plugin.cpp)
     turns store(store(A, j, u), i, v) = t into t[i] = v and
     store(t, i, v)[j] = u. When i = j the second literal is v = u. The
     correct condition is t[i] = v and (i = j or t[j] = u), generalized to
     every inner write and multidimensional indices.
   - g3 fix: ArrayPlugin.TryStoreEquality (pkg/qe/qe_array_plugin_store.go,
     commit a15af8166) guards each inner write's equation with
     (or (= index outer-index) ...). Test: TestArrayPluginNestedStoreElimination.
   - Affected: nested-store qe outputs differ from z3 (quantifier_tactic
     array equation tests); no comparison vector besides the reproduction.

- **opt from_fmls stale orig_term** (found 2026-11-12; fixed upstream, so
  a backport candidate for the fork like the relevancy and pareto fixes).
   - Reproduction: pkg/cmd/g3/test_vectors/z3_bug_opt_stale_objective_term.smt2
     (box priority; x <= 7, a => x < 3, b => x > 1; maximize x, maximize y,
     assert-soft a :weight 2 :id q, assert-soft b :weight -3 :id q):
     z3 4.7.1 prints (q (- 1)); the optimum (a true, b false, cost 0,
     offset -3) is (q (- 3)), printed by g3 and z3 4.17.0.
   - Cause in z3: context::from_fmls (src/opt/opt_context.cpp) declares
     expr_ref orig_term outside its loop and is_maxsat does not reset it,
     so q's obj.m_term is the preceding (maximize y) term. maxres's
     update_assignment calls verify_model(q's index, ...), which evaluates
     y (0) against the adjusted cost and rejects every improving model
     (-tr:opt shows "Out of bounds: y 0 != -3"); found_optimum then
     recomputes the cost from the stale initial model (cost 2, shown -1).
     Upstream moved orig_term into the loop and is_maxsat sets
     orig_term = nullptr.
   - g3: only objectives converted from maximize/minimize get a
     verification term (cmd_context addOptimizerObjective,
     verifyConverted). Excluded vector: the reproduction. The trace=true
     log is identical; only get-objectives differs.

- **bvsmul_noovfl numeral folding** (recorded as item 21; fixed upstream:
  z3 4.17.0 folds (bvsmul_noovfl #xa #x3) to true).
   - Cause in z3: bv_rewriter::mk_bvsmul_no_overflow
     (src/ast/rewriter/bv_rewriter.cpp) multiplies the numerals as unsigned
     values, so signed -6 * 3 = -18 (an underflow, no overflow) folds to
     false.
   - g3: BVRewriter.RewriteMultiplyBound
     (pkg/ast/rewriter/bv_rewriter_mul_bounds.go) folds the signed product.
   - Vector bv_native_operator_models stays in identity testing (its trace
     is identical); its stdout differs at that one get-value line, and the
     C++ comparison in main_test.go patches the fork's line.

## Open leads (2026-11-11, late)

- RESOLVED (2026-11-12): recursive_mutual_boolean_model. z3 4.7.1 does not
  terminate: get-value of (even 0) evaluates the preprocessed body
  (or (<= n 0) (odd (+ -1 n))), and rewriter_tpl::constant_fold folds only
  ite early, so every and/or argument is rewritten and the dead branch unfolds
  forever. g3's model short-circuit is kept as a bug fix (see "z3 4.7.1 bugs
  fixed in g3"); the vector is excluded and recursive_mutual_safe_get_value
  (same script, get-value without even/odd) is trace-identical. Also found:
  with model_evaluator.max_steps the fork's get-value prints a null value and
  segfaults in shared_occs::process (parse_get_value displays model::eval's
  failed result) - a second z3 4.7.1 bug, not reproduced by g3.
- RESOLVED (2026-11-12, by the pareto/box setup port): s2 is trace- and
  stdout-identical; s4 (x unbounded below, infinite frontier) times out in
  both and g3's 30 s trace is an exact prefix of the fork's. Original lead:
  pareto with a soft and an arithmetic objective together (e.g. assert-soft a
  plus (minimize x)) diverges early in the trace while stdout matches: the
  fork creates a named bound atom |-3 <= (* (- 1) x)| and an attach-enode
  that g3 never makes (opt_solver / theory_arith bounds inside gia_pareto).
  Reproductions in /var/tmp/g3-wt-paretoneg.work (s2.smt2, s4.smt2).

## nlqsat init_expr2var port (2026-11-12)

init_expr2var is ported; with the nlsat explain fixes (normalize in
mk_quadratic_root, simplify's lower-stage pass, signed projection's zero
start), the nlqsat mbp/mk_model lifetimes, tseitin scope-ordered locals (which
also fixed a real soundness bug: ite_extra clauses lacked the chain context),
and the unknown-tactic error column, every nlqsat/nra/qe/nlsat/qfnra vector
and probe matches the fork. Two deliberate differences remain:
- nlsat reorder: g3 renames a root atom's variable on reorder; the fork does
  not, and on restore_order deletes lemmas whose old root variable is below
  the renamed polynomial's max variable. g3 applies the fork's deletion test
  and additionally deletes a lemma whose renamed root is ill-formed, so it
  never keeps a lemma the fork has invalidated (traces match on all vectors).
- nlqe (dead code in z3 4.7.1, TBD_TACTIC): the C++ loop adds only level-1
  projections to the answer, so a clause projected at a deeper universal
  level over free variables is dropped and the answer becomes true. g3's
  projectQE also adds such clauses (universal level, clause level 0), as the
  source comment "is_forall() && clevel.max() == 0" suggests. Not reachable
  from z3 4.7.1 ("unknown tactic nlqe"), so no identity vector is affected.
DONE (2026-11-12): tseitin get_lit's sign case uses a plain MkNot
(mk_lit's m.mk_not); inv keeps the simplifying mkNot. No vector exposed it
(tseitin's own rewriter folds true/false first). Lead: tseitin's imp lives
as long as nlqsat's m_nftactic (g3 releases it per Apply); no vector or
combinator probe ((then tseitin-cnf simplify ...)) shows it.
FIXED (2026-11-12): tseitin-cnf failed on any goal containing and
("operator not supported"): the or_else fallback's simplifier was built
with a params value freed when the factory returned, so elim_and never
applied. It is now using_params(simplify(p), simp_p) as in z3. Vector
tseitin_cnf_and_fallback.

## Optimization leads after the pareto/box setup port (2026-11-12)

Pareto now runs optimize()'s internalize, optsmt::setup, update_lower and
update_bound; box uses optsmt::geometric_opt for the default engine. Open:
- (fixed) box_same_term_three_objectives (QF_IDL, random initial
  activity) is now trace-identical: setup_unknown(st) registers only the
  selected profile's plugins (here theory_i_arith), so
  setupUnknownStaticPlugins drops the unselected eager placeholders (their
  final checks, e.g. theory_seq's random draws, and final_check rotation
  slots) and takes the profile order (quantified profile: datatypes before
  bv). Kernels whose plugins were copied from a set-up context keep the
  eager set (context::copy_plugins order is not modelled yet). QF_AUFBV,
  QF_ABV and QF_UFBV under auto_config register bv before arrays.
- Done (box_soft_minmax_push_pop): optsmt::m_models is opt::context
  state (cmd_context optimizationOptsmtModels), never cleared by
  optsmt::reset; execute_box hands out get_model(j), null entries
  included. The same vector needed: opt pop runs reset_maxsmts on the
  retained optimizer; ~maxres releases m_csmodel, m_trail, m_mus, m_defs,
  m_asms, m_B, then maxsmt_solver_base::m_trail (separate owners, add_soft /
  cs block / upper block formulas are locals); from_fmls pushes purify
  definitions at the objective's position among the hard formulas
  (HardPosition), which fixes the asserted_formulas substitution layout;
  lia_rewriter_cfg::args (PBSum) releases front to back.
- Done (lex_unexecuted_soft_offset_interval, ..._continuation):
  update_bound sets maxsmt m_upper/m_lower from the model's soft cost;
  a lex run that stops at an unbounded objective displays the later MaxSMT
  objectives as (interval adjust(m_lower) adjust(m_upper)).
- Done (box_farkas_engine, box_symba_engine, box_farkas_real_unknown_bounds,
  lex_symba_unknown_bounds, lex_farkas_unknown_bounds): farkas_opt /
  update_upper and symba_opt ported (symba's locals outlive geometric_opt);
  get-objectives after an undecided box/lex run prints display_assignment's
  get_lower_as_num/get_upper_as_num (optsmt m_lower/m_upper, maxsmt bounds),
  also for later l_undef box continuations.
- DONE (2026-11-12) optimization leads: non-last lex objectives commit
  first, then update_lower's eval supplies the value (no early
  captureArithmeticModel eval); setup_QF_IDL switches an unused arithmetic
  plugin to theory_i_arith (useIntegerExt), and MakeObjectiveLowerBound
  follows opt_solver::mk_ge per theory; opt pop releases only optsmt and
  hard constraints (purified objectives live until import_scoped_state);
  an unknown run with display bounds clears valueReady. New vectors
  lex_minimize_negated_then_more, lex_unbounded_real_idl_profile,
  box_soft_push_pop_objective_lifetimes, box_symba_soft_pop_unknown_display.
  The case-split-after-push lead no longer reproduces.
- RESOLVED (2026-11-12): a z3 4.7.1 bug (stale orig_term in from_fmls);
  see "z3 4.7.1 bugs fixed in g3". Original report: box with a
  negative-weight soft group (l2d plus
  (set-option :opt.priority box): x<=7, a=>x<3, b=>x>1, c or x>6,
  soft c w4 id p, maximize x, maximize y, soft a w2 id q, soft b w-3 id q,
  soft (not c) w1.5 id s; check/get-objectives, push, (not a), check,
  pop, check): trace identical, but get-objectives prints (q (- 3))
  where z3 prints (q (- 1)) after the 1st and 3rd check-sat. Likely
  maxsmt get_lower/get_upper combining m_lower/m_upper (update_bound) with
  m_msolver bounds and m_adjust_value (offset -3) after box's scoped
  execute. -3 is the true optimum, so this may be z3 behaviour to mimic.
- An unbounded pareto frontier (assert-soft a plus (minimize x) with x
  unbounded) enumerates forever in both z3 and g3; g3's trace matches z3's
  prefix up to the timeout.
- DONE (2026-11-12) theory registration leads: setupPluginSet follows z3's
  rule (plugins registered before setup in registration order, then the
  profile's missing plugins in profile order). CFG_BASIC registers every
  theory; copied plugins keep the source's order (recordCopiedPlugins);
  opt_solver::ensure_pb registers pb before setup. setup_auto_config's
  QF_FP/QF_FPBV/QF_BVFP/UFLRA go through setup_unknown(st) (two stdout bugs:
  QF_FP with Int lacked arithmetic; ground UFLRA). sortmax walks new_soft in
  obj_map order and its clauses are unheld locals; cs_max_resolve's closing
  disjunction is the fml local. New vectors sortmax_soft_hash_order,
  maxres_single_correction_set, setup_unknown_qf_fp_with_int,
  setup_unknown_uflra_ground, setup_basic_qf_fp_with_int.
- Leads (pre-existing): pure QF_FP fp.add builds ite arguments in the
  opposite order from trace line 505 (/var/tmp/g3-wt-setupleads.work/r/
  fp2.smt2); SMT2 errors for unknown sorts/constants/functions under a
  restrictive logic differ (column later than z3's, z3 quotes 'Array', z3
  4.7.1 spells "indentifier"); MBQI scripts differ in free-list id reuse
  (UFNIA line 71, AUFLIA line 562; r/mq*.smt2).
## fp ite order, SMT2 error text, MBQI lifetimes (2026-11-12)

DONE: bv_rewriter mk_extract/mk_bit2bool build ite arguments else-first
(GCC right-to-left evaluation); bit_blaster mk_le releases locals in scope
order; propagate_values ~imp order. SMT2 parser errors match the fork in
text and column (unknown sort/constant/function, indexed identifiers with
z3's "indentifier" spelling, sort mismatches, wrong arity, array messages,
set-logic after init). MBQI: qe_lite reduce_quantifier scoped locals, proto
model released when replaced, qi_queue instance lifetimes, relevancy
propagator owns its references (pop inside popTo), quantifier pop first,
macro_util arith macro candidate locals. New vectors fpa_add_ite_argument_order,
fpa_to_sbv_propagate_values_cleanup, mbqi_auflia_arith_macro_lifetimes,
parser_error_* (5). Now stdout-identical: closest_theory_variables_proof,
pb_native_watches_proof, pb_simplex_proof,
pending_reset_assertions_native_sort_init.
Leads:
- Model printing of internal sorts: pdecl_manager::pp uses m_sort2info, so a
  BitVec sort the user never wrote (range of fp.to_ubv/fp.to_sbv in
  get-model) prints as (_ bv 4) in z3 but (_ BitVec 4) in g3 (line wrapping
  differs too). Needs the sort_info save/pp port (pdecl.cpp:732-1045).
- With no logic set, z3 predeclares the parametric List datatype
  (pm().mk_plist_decl(), cmd_context.cpp:725); g3 says unknown sort 'List'.
- mq_UFNIA line 2,295,926: after a base-level conflict and [pop] 1 2, BCP
  assigns (not #1036) and #1037 in the opposite order (watch list or clause
  literal order).
- mq3_UFLIA line 603,652: antecedent literals of a clause in reverse order
  during conflict resolution (MBQI or dyn_ack clause literal order).
- mq_AUFLIRA line 65,185: two theory-propagated bound atoms on the same
  variable and bound assigned in the opposite order ((not #268), #269,
  justification -1); check var_occs / implied-bound iteration.
- mq_ALL / mq_NONE / mq_AUFLIA ~1.95M lines: not investigated.
- fp.to_real variant: trace diverges at line 13,372 (arith * / + id order).
- qe call arenas (liteCall / vector arenas) still hold refs until a call
  ends; other qe paths likely have the qe_lite kind of lifetime difference.

## Tactic sweep fixes (2026-11-12)

Every registered tactic was run as (apply T) and (check-sat-using T) on
small Bool/LIA, LRA/UF and BV goals, comparing stdout with the fork.
FIXED:
- ufbv / bv: "arena: closed". The ufbv-rewriter adapter took ownership of
  the caller's input goal (TakeGoal), so repeat's result cleanup closed it.
  It now borrows the goal (AddGoal) and drops the duplicate reference
  (Goal.ReleaseDuplicateReference). Vector ufbv_tactic_arith_goal
  (stdout-identical).
- qfnra: g3 hand-built the nlsat route (different purify, depth 16 vs 25)
  and added fail-if-undecided after the first two routes, so an undecided
  nlsat result fell through to smt (sat where z3 says unknown). It now uses
  try_for(qfnra-nlsat) exactly as mk_qfnra_tactic. Vector
  qfnra_uf_nlsat_undecided; all 34 QF_NRA vectors stay trace-identical.
- subpaving: the simplify stages lacked using_params (as tseitin-cnf did),
  so arith_lhs/elim_and never applied.
- collect-statistics keys use smt2_printer's func_decl form
  ((declare-fun + (Int Int) Int)). Vector collect_statistics_decl_keys.
- nlsat: z3's "eliminate distinct operator (use tactic ...)" message.
Remaining sweep differences: horn / horn-simplify error text and behaviour
(worker running); sine-filter (the fork dumps core); qfbv-sls (timing).
FIXED (2026-11-12): ufbv_tactic_arith_goal trace. Cause: the ufbv rewriter
(demodulator) walked Go maps (random order) where ufbv_rewriter.cpp walks
obj_hashtable/obj_map in table order (m_processed, m_fwd_idx and each
quantifier_set for the output; reschedule_processed, reschedule_demodulators,
rewrite1's first match, destructor frees). Now ast.ObjMap throughout
(NewObjMapCapacity(1) for quantifier_set, EraseHash). Note: in the 4.7.1
registry the name ufbv-rewriter maps to mk_quasi_macros_tactic, so only
`ufbv` exercises the demodulator. Leads: rewrite_cache.go / match_subst*.go
use Go maps for lookups only (re-check if ever iterated); other packages may
iterate Go maps where C++ walks hashtables (audit worker running over
smtcore/ast/tactic/sat/nlsat/model/opt/cmd_context/parsers; muz, duality,
interp and qe still to audit: 505 range-over-map sites in total).

## horn tactic errors (2026-11-12)

DONE: horn / horn-simplify on unsupported goals match the fork in text,
trace and engine choice: rule_properties source messages surface as
default_exception text (HornTacticException; check-sat-using prints
"tactic failed: ...", apply keeps line/column), Collect walks post-order
into obj_maps (z3 reports m_uninterp_funs.begin()), horn rules are named
<null>, horn-simplify selects the engine and runs check_rules, datalog rule
checks run right after flush_add_rules, get_formula_kind's flatten_or args
are held (FlattenOrHeld). Vectors horn_tactic_nested_recursive_error,
horn_tactic_uninterpreted_error, horn_tactic_datalog_uninterpreted_error.
Leads:
- DONE: process exit code for a script with an error: z3 exits 1, g3 exited 103
  (ErrorParser), e.g. (assert x).
- checkDatalogEarlyRuleProperties skips check_infinite_sorts (kept for the
  integer-relation extension); z3's DATALOG check_rules includes it, so a
  declare-sort relation under the datalog engine may differ.
- Other datalog flush callers (propositional_engine.go flushes,
  pdr_relation_query.go) may still run property checks later than z3; audit
  against fixedpoint (query ...) traces.
- ASTManager.FlattenOr leaves its fresh negations unowned; callers in
  der_rewrite, pdr/manager, spacer/inductive_property and qe/lite_indices
  may differ in id reuse (FlattenOrHeld is the model).

## Command sweep (2026-11-12)

get-info/get-option/echo/statistics, datatypes and define-sort, simplify
options, push/pop/reset/exit, BV/FP/string get-value and proofs were compared
with the fork: all identical except runtime statistics (memory, allocs) and
:reason-unknown after a search on the incremental smt solver. FIXED: the
kernel's unknown starts as "unknown" (context::m_unknown) and smt_solver
reports last_failure_as_string() for every status, not only l_undef. Vector
reason_unknown_after_search.

## Tactics on quantified goals (2026-11-12)

FIXED: pb recognizers (pseudoBooleanIs) answer false for non-apps as
pb_decl_plugin's is_app && is_xx does (pb-preprocess failed on any
quantifier); goal2nlsat skips the or-split and treats non-app atoms as
Boolean variables (process_bvar), so nlsat gives the purify-arith error and
the qfnra routes return the goal unchanged. Vectors quantified_goal_*
(pb_preprocess, qfnra_nlsat, qfnra, nlsat).
Leads: other recognizers may return ErrASTDomain on quantifier/variable
inputs where C++ checks is_app (GetFamilyID/GetDeclKind on a quantifier fail
through declIdentity); pb_preprocess parse (tactic/core/pb_preprocess_terms.go)
calls GetFamilyID after the PB check without an is_app guard.

## Go map iteration audit, core packages (2026-11-12)

168 range-over-map sites in smtcore, ast, tactic (minus ufbv), sat, nlsat,
model, opt, cmd_context, parsers, api, util/params, math/automata,
math/hilbert: 111 order-insensitive, 36 fixed to z3 container order, 21
deterministic but unfaithful (leads). Two real run-to-run nondeterminism
bugs fixed: bv_size_reduction (signed bound maps drove fresh constants and
substitutions) and SLS (Randomize/Reset/UpdateAll over entry points). Other
fixes: ASTExprMap pairs/reset (expr_map order; api_qe, Z3_ast_map),
flatten leftovers, bv_bounds is_sat/rewrite, quick_checker candidate sets,
arith_eq_adapter display order, implied_equalities sort/value order,
sat dep2asm/asm2fml order, recover_01/degree_shift/eq2bv table order,
bounded_int2bv extend_model, api ast_map, automata determinization. New
ast helpers: ObjMap.Finalize, pair-keyed ObjMap helpers.
Leads (21 sites): theory_str (~15, sorted by handle instead of table order;
only generic_string_z3str3_free_models uses z3str3 and matches today);
smt_implied_equalities is not a faithful port (model loop vs per-value
representatives, array fallback, filter_basic/reduce_value missing,
union-find root choice); tactic_sine TRACE("sine") and re-push order;
smt_deterministic_order.go helpers and macro_manager publishForbidden sort
by slot/handle instead of obj_hashtable order; cmd_context
clearDebugExpressions releases in name order; quick-checker vector-of-
hashtables copy layout. Not yet audited: muz (~220 sites), duality, interp,
qe, util/lp.

## Differential fuzzing (2026-11-12)

Random Bool/LIA/BV/UF and Real/array/UF scripts (fuzz generators in the job
tmp dir), comparing answers and full stdout (with get-model) with the fork.
- FIXED: array model entry order. theory_array_base select_set is a
  ptr_hashtable (sel_hash = composite hash of the select index roots; insert
  replaces an equal entry); mk_value and propagate_selects_to_store_parents
  walk it in table order (smtArraySelectSet). The array else value comes from
  simple_factory::get_some_value = *m_values.begin() of an obj_hashtable:
  numeralValueSet keeps an ObjMap (ObjMap.FirstKeys). About 1 in 4 random
  array scripts printed different models before. Vectors
  array_model_entry_order, array_model_some_value_order.
- Lead: other proto_model factories still pick values in insertion order
  where C++ walks an obj_hashtable begin(): user_sort_factory.go,
  datalog_factory.go, structured.go, array_factory.go, datatype_factory.go.
- FIXED: model building failed with "build state model values: g3: AST
  source precondition" (unknown where z3 says sat; ~1 in 200 random
  LIA+BV+UF scripts, e.g. (and (not (= x y)) (not (= x 3)) (not (= y 3)))
  with (= v (ite c u #x3))). Theory ModelValue required the class root to
  own its theory variable; C++ mk_value just takes n->get_th_var(), which
  after a merge belongs to another member (SMTTheoryBase.HasVariable; bv,
  array, lra, diff-logic, dl, utvpi, fpa, str). Also: theory_fpa applies
  fpa2bv updt_local_params (rewriter.hi_fp_unspecified=false, so the
  fp.to_ubv unspecified UF parts are built); theory_bv internalizes concat /
  extract by copying m_bits literals; arith_rewriter mk_abs_core builds its
  ite arguments right to left; pdecl_manager sort_info / pp ported
  (cmd_context/sort_info.go), so unreferenced sorts print as (_ bv 4).
  Vectors fp_to_ubv_uf_model, fp_to_ubv_sort_print, bv_uf_ite_argument_model,
  lia_bv_ite_distinct_model, sort_info_model_printing.
  Leads: fuzz seed (gen.py 1126; job tmp m_gen_1126.smt2) model values still
  differ - trace diverges at line 308 in freed-id reuse after bvsmod/bvshl
  bit-blasting (bit-blaster temporaries dec_ref order); fp_to_ubv_uf_model
  trace diverges at line 989,210 (theory_bv propagate_bits / assign_bit
  order); get-model function order (BV-range UF g printed after a Seq-range
  UF h); sort_info approximations (datatype field sorts saved at parse
  time, complete_model instantiate, only get-model/WriteExpression/WriteSort
  use the cmd_context pp_env).
- Lead: predeclared List datatype without a logic
  (needs lazy pdecl psorts; g3's sort parameters are real uninterpreted sorts
  and allocate ids, so predeclaring at init would shift ids).

## qe/qsat BV goals, qe-sat, seq fuzz failures (2026-11-12)

FIXED (qe): (apply qe) on a BV+UF exists expanded the disjunction in a
different order - SolverNNFAdapter kept private ContainsApp caches that
outlived quant_elim_plugin::reset (now m_s.contains(i)); m_subfml lives
until the plugin's next check (persistent quantElimPlugin.subfml); qsat
project/project_qe/add_assumption locals in their own call scope; get_nnf
on a quantifier keeps the formula (qe-sat on array foralls failed with
ErrASTDomain); qe-sat check_success reports TACTIC_CANCELED_MSG. Vectors
qe_bv_uf_exists_expansion, qe_sat_array_forall_index,
qe_sat_real_int_nested_exists_canceled.
FIXED (seq): solve_unit_eq(vec) dropped the mk_concat reference before
add_solution reset the solution-map cache ("invalid AST owner" on
(str.prefixof "a" (str.substr t 0 x))); seq_rewriter mk_seq_extract ported
faithfully (constant position past the end, 32-bit is_unsigned limits and
wraparound, unit loop, zero-unit mk_concat exception; simplify catches
ASTException as z3_exception). gen3 string/datatype fuzz seeds 1-1600 now
match. Vectors seq_prefix_substr_variable_length,
seq_extract_constant_position_bounds, seq_contains_extract_past_end.
Leads:
- qsat is ~6x and qe-sat ~11x slower than z3 on the BV+UF goal with the
  same search statistics (flat profile; 122 MB vs 4.7 MB reported memory -
  check retention); rlimit count differs (2705746 vs 8175552).
- DONE (2026-11-12): qe-sat construction order and lifetimes (members built
  in source order and kept across Apply, reset() order, simplify_rewriter_star
  with one persistent simplify_solver_context and rewriter_core cache stack,
  iterative qe nnf with forward trail release, solver_context plugins per
  project call, quant_elim_plugin reset/destructor order, atom sets in
  obj_hashtable order, check_pred m_refs forward, get_assignments after
  unsat/unknown). Both qe_sat_* vectors trace-identical. Leads:
  QESATTactic.ResetStatistics clears snapshots (source kernel's is empty);
  ArithmeticPlugin.Close iterates Go maps (obj_pair_map in C++);
  SolverContext.MkAtom non-ref path keeps normalizedAtoms;
  BoolAtomIndex assumes a fresh table (capacity carried by atom_set::reset).
- other qsat helpers (maximizeCore, getCore) keep temporaries in the
  round-wide call scope; exprQEBranchVariables.Close order vs obj_map.
- tactic catch sites that only accept rewriter exceptions where C++ catches
  z3_exception (audit IsRewriterException uses).

## Go map iteration audit, part 2 (2026-11-12)

316 sites in muz/*, util/lp, duality, interp, ackermannization (portcheck,
cmd/partial-list unreachable): 273 order-insensitive, 14 fixed to the C++
order (dl_compiler var_indexes u_map, variable_intersection, neg_intersection,
bound_relation uint_set, spacer slice/scale/quantifier-abstraction/karr
model converters, iz3translate AstSet, ackr_info m_t2c), 19 made
deterministic in the closest order (no ported C++ order), 1 lead.
DONE (2026-11-12, user-approved STL exception 3): lu_extend add_last_rows_to_B
iterates std::unordered_set<unsigned> in libstdc++ order
(util/lp/std_unordered_set.go, positive control
src/test/stl_unordered_set_order_control.cpp, vector
lra_lu_extend_add_last_rows with smt.arith.solver 6 + lp.simplex_strategy 2).
Leads: lar_conflict_certificate debug
"left side =" in unordered_map order; mk_slice converter order;
array-instantiation/scale/magic predicate registration use AST id order;
ackr Info.Close reverse order vs ~ackr_info table order; iz3hash hash_set
order; with fixedpoint.xform.karr on Int predicates z3 4.7.1 errors "Rule
contains infinite sorts" but g3 runs on.

## Datalog wrong answer across queries (2026-11-12)

FIXED (correctness): a second datalog query could return wrong tuples
(dlq: nopath answered (3,7) instead of {(4,1),(7,7),(4,4)} after a q1 query).
g3 evaluated interpreted filter conditions with RewriteWithBindings on the
context's shared rewriter, which kept the last fact's bindings; the next
query's interpreted-tail simplifier then rewrote (= a c) to (= a #x2). Now
RelationManager.GroundCondition does what should_remove does: var_subst
(context m_var_subst, std_order) then the plain rewriter on the ground term.
Vectors datalog_query_sequence_{negation,no_print,three,repeat}
(stdout-identical).
DONE (2026-11-12): traces now identical too. compile_rule's interpreted
tail is one mk_and, renamed to columns with var_subst, emitted as a single
filter_interpreted / filter_interpreted_and_project (prepareInterpreted +
filterInterpreted); facts are removed only when the ground condition is
false (should_remove), default_table_filter_not_equal_fn ported; simple
joins' planner pins die when the transform returns; rel_context m_code lives
until the next saturate.
Leads: ground interpreted tail with nothing bound (C++ filters a void
register); explanation-product filter on mixed conjunctions is split per
conjunct (finite_product_relation_plugin::filter_interpreted_fn iterates
tuples) and errors on a mixed non-conjunction; m_ectx is closed per round
instead of reset at the next saturate; simple_joins structure differs from
join_planner; audit other state left on ctx.get_rewriter()
(SetSubstitution).

## Re-fuzz after the fixes (2026-11-12)

1200 fresh random scripts (gen.py/gen2.py with get-model, gen3.py
strings/datatypes): 2 differ. gen.py seed 3183: one BV model value differs
(likely the bit-blaster temporary dec_ref / freed-id reuse lead, as seed
1126). gen3.py seed 3097: z3 answers sat in 8 s, g3 runs past 30 s; g3's
trace is an exact prefix of z3's (4.6M of 36M lines), so the search is the
same and g3 is only slower (constant-factor performance, see the qsat note).

## BV model values / bit-blaster lifetimes (2026-11-12)

FIXED: random LIA+BV+UF scripts printing different BV model values.
Bit-blaster expr_ref_vector groups now die in reverse declaration order
(smod/sdiv/srem/ext rotate; overflow helpers); theory_bv keeps get_arg_bits
locals alive until after init_bits and hands them off in C++ destruction
order (MK_UNARY/MK_BINARY/MK_AC_BINARY, sub, reductions); BV atom argument
bits release arg2 then arg1, each front to back; restore_relevancy after a
conflict marks and propagates after all lemma atoms are rebuilt
(context::mark_as_relevant); internalize_num calls fixed_var_eh directly;
bv_bit_prop prints 0/1; TRACE("relevancy") in assign_core. Fuzz seeds
5000-6200 now all match. Vectors bv_blast_temporaries_model_values,
bv_blast_temporaries_smod_order, bv_internalize_arg_bits_lifetime,
bv_conflict_restore_relevancy, bv_atom_arg_bits_release_order (their full
trace=true runs exceed the 30 s fullcmp timeout).
Leads: func-decl delete order (decl ids 2147483xxx) differs; mk_neg /
mk_adder / mk_subtracter release cin vs not_a in the other order; C++
relevancy mark_as_relevant sets every ring member relevant without the
already-relevant check (duplicate atom-queue entries); dead generic AC case
in operationBits; trace=true is 4-5x slower than z3 (format objects for
model/[assign] pretty printing dominate).

## proto_model factories and model completion (2026-11-12)

FIXED: user_sort, datalog, struct (array/datatype) factories take values in
obj_hashtable begin() order (ObjMap + FirstKeys); complete_model uses
m().get_some_value without a some_value_proc (model value 0 for an
uninterpreted sort); completion walks m_psort_decls / m_func_decls in
dictionary<> table order (cmd_context/symbol_dictionary.go, string_hash
layout, builtin psort decls by logic, scoped restore); the model printer
builds the cardinality constraint as a format (model_smt2_pp.cpp:102-132).
Ten factory_* vectors.
Leads: parser error texts ("more than one constant with the same sort ...
disumbiguate", query "Invalid function name. Expecting sort list startig
with '('", unknown relation column 9 vs 7); engine trace diffs on probes
(datalog mk-var order, bmc hyper-res arguments 1 0 vs 0 1, pdr trace line
19934, tab/clp/bmc/duality on sort errors); (_ FiniteDomain n) is not an
SMT2 sort in 4.7.1 (datalog factory unit-tested only); complete_model
instantiation of parametric user sorts with Bool args unprobed;
:global-declarations toggling and failed datatype rollback in the
dictionary; "universe for" comment is a simplified string.

## Status (2026-11-12, end of day)

fullcmp2 over all test vectors: 732 trace-identical; the only trace
differences are qe_rec_free_boolean and pb_strengthening_proof (the fork
crashes); stdout differences: bv_native_operator_models (documented z3 bug)
and qe_rec_free_boolean; 8 timeouts are trace=true runs past 30 s (new BV
and fp vectors, mbqi_auflia_arith_macro_lifetimes,
qe_bv_uf_exists_expansion) whose traces matched as far as they ran.
Largest open item: constant-factor speed (6-11x slower than z3 on identical
searches; trace=true 4-5x slower).

## Fixedpoint leads (2026-11-12, late)

FIXED: parser error texts (ambiguous constant "disumbiguate", faithful
parse_func_decl_ref with all its messages and the find-before-next column,
cmd_context::find_func_decl / func_decls::find, query argument messages,
parse_sort(context) errors at every sort site, indexed identifier messages);
bmc linear/qlinear hyper-res positions (0,1) and the slice converter's
tail-0 unification; tab relation query via engine_base::query + mk_query;
query params persistence (params_ref copy-on-write semantics); karr: the
inner datalog context's check_rules makes every karr run error "Rule
contains infinite sorts" as in z3 4.7.1. Vectors fp_parser_ambiguous_constant,
fp_query_func_decl_ref_errors, fp_query_builtin_macro_refs,
parser_error_sort_contexts, fp_bmc_linear_hyper_res_positions,
fp_pdr_karr_infinite_sorts, fp_pdr_karr_bv_loop_counter,
fp_query_params_persist, fp_tab_relation_query (stdout-identical; traces
of the bmc/karr/tab ones still differ, see leads).
DECIDED (user, 2026-11-12): keep g3's datalog Int/Real column support
(bound/interval/external-interval backends) as a deliberate deviation;
z3 4.7.1's DATALOG check_rules rejects any Int/Real relation column with
"Rule contains infinite sorts" and answers unknown. Do not add
check_infinite_sorts to checkDatalogEarlyRuleProperties.
Leads: datalog statistics (:dl.unary_singleton / :dl.union accumulate across
queries in z3; :max-memory/:memory/:num-allocs/:rlimit-count missing from
:print-statistics); tab trace from line 61 (and #1 temporary, eager answer
proof); clp/bmc construct the engine before flushing rules; duality @Fail!0
vs implication order; pdr/spacer probe divergences (~824, ~13274); datalog
inliner after an earlier query (m_preds_with_facts / output predicates
across scoped_query); karr loop-counter mk-var/app order; qlinear
eval_q(md, sub[j], i) audit; slice proof converter positions; dead karr
fixpoint machinery and unsliced-proof retry.

## List, model order, qe cleanup (2026-11-12, late)

FIXED: predeclared parametric List datatype without a logic (init_list;
decl ids only, so expression ids do not shift); cmd_context pp_env
pp_fdecl/pp_fdecl_ref ((as nil (List Int)), overloads with signatures);
declare-sort/define-sort redeclaration errors and columns; get-model
function order (sort_fun_decls walks the dependency func_decl_set in table
order); bit-blaster cin release order in mk_neg/mk_subtracter; relevancy
mark_as_relevant sets every ring member; QESATTactic.ResetStatistics no-op;
qe ArithmeticPlugin m_trail and obj_pair_map bounds cache with the source
destructor order; qe_tactic::cleanup rebuilds expr_quant_elim; MakeBound no
longer holds the strict-Int difference term (it was cached by the rewriter);
clearDebugExpressions releases in m_object_refs table order. Ten vectors
(list_predeclared_*, parametric_datatype_constructor_as_print,
overloaded_decl_as_print, model_func_decl_dependency_order,
qe_tactic_cleanup_lifetimes, qe_int_strict_bound_rewriter_cache).
Leads: qe Real exists elimination (trace line 303: z3 builds (* c (* -1 b))
and (* c a) after the division and frees it; get_lt_bound/mk_div vs
normalizeRest/addStrict); other qe/arith ref-held temporaries that make the
rewriter cache terms z3 does not; format pretty-printer holds refs on
intermediate formats (decl delete order, not visible in trace=true);
pp_env m_func_decl2alias; parse_psort_name looks up global psorts before
datatype parameters; user parametric datatype formals use the parameter
name instead of symbol(i); (declare-sort Int 0) column 19 vs 20; theory_str
obj_map sites unaudited (std::map<expr*> sites are pointer-ordered, not
portable); z3 4.7.1 crashes on define-sort L () (List Real) after a pop that
removed (List Int).

## Performance round 1 (2026-11-12)

No behaviour change (fullcmp2 identical, one fewer timeout). Measured:
g7_qsat 14.4 -> 11.7 s (z3 2.1 s); slow_3097 85.7 -> 56.2 s (z3 8.1 s),
peak memory 412 -> 216 MB; trace=true runs ~33% faster. Changes: idgen free
list grows geometrically (it was copied on every node registration); seq
theory reference holders use node refcounts and pooled local holders;
batched seq shape queries (SequenceExprShape, IsSequenceExpr, IncRefEach);
decl/sort family+kind cached on the node; trace acquisition frames reused
and a small-slice format flatten cache; numeral conversions without string
round trips (SetSmallRat, ArithmeticNumeralBigRat; mpq_set trace path
kept); cached family ids; two-word arena reads/writes; fewer allocations in
clause creation and literal capture.
Next opportunities: astNodeRecord layout (nodeAt/validNode cache misses
6-16%); batch query APIs for seq canonize/expand, BV addFixedBitEquality
(37% of g7), MkBoolVar/NodeHeader callers; open-addressed AST hash-cons
table (tableFind/Insert/Erase ~5%); dense arrays for relevancy
positive/negativeWatch and seqEvalCache; small-value fast path for model BV
numerals (bitVectorNumeralDeclCore 12% of g7); fold numeric-manager
checkReady/checkCleanup flags into one busy word; dense format flatten cache
and SMT2PPExprTree reuse in trace mode.

## qe Real/Int elimination and qsat scopes (2026-11-12, late)

FIXED (correctness): Int Cooper resolution ignored the x |-> d*x + z
substitution (x_subst in mk_resolve), so (apply qe) on e.g.
exists x. x mod 2 = 0 & 3x <= b & 5x >= a produced mod 3 residues instead
of z3's (correct) mod 6. Also: normalizeRest runs C++'s simplify(mk_div)
(Real elimination trace), mk_bound final simplify and argument order,
Cooper port follows assign_nested_divs/assign_divs/mk_div_equivs (shared
div_z per bounds processor, shared mark), per-iteration s/t locals, MBP ts
map in obj_map order with per-row lifetimes, qsat maximize_model scope and
mk_not(fml) without an extra reference, qe_rec rewritten to the C++ frames.
Vectors qe_real_strict_bound_div_simplify, qe_real_disequality_bounds,
qe_int_nested_mod_residues, qe_rec_real_disjunction,
qe_rec_forall_exists_real, qe_rec_nested_alternation.
Leads: asserted_formulas::assert_expr frees an unowned input right after
rewriting (g3's smtAssertionScope keeps it to scope end; qsat forall/exists
line 122, qe_rec Int line 400); asserted rewriter ActivityCache release
order at flushAssertedCache (forall/exists Int qe probe line 118); bv2int /
mod rewrite order after begin-check (Int Cooper probe line 3020, mod 3 probe
line 396451); MBP linearize/maximize evals held for the whole call; Real
qe2 probe diverges at line 788.

## Fixedpoint engine traces (2026-11-12, late)

FIXED: fp_bmc_linear_hyper_res_positions, fp_pdr_karr_bv_loop_counter,
fp_pdr_karr_infinite_sorts and fp_tab_relation_query are trace-identical.
Non-datalog query clones copy rule objects (no re-normalization), mk_query
builds the query rule in place, bmc/clp/tab engines own a kernel created at
ensure_engine before rules flush, linear bmc compile follows group order,
loop counter starts at get_max_rule_var+1, tab subsumption index lives until
the next setup and the answer proof is built lazily, duality @Fail!0 order
and clause lifetimes, datalog execution statistics accumulate across
queries and :max-memory/:memory/:num-allocs/:rlimit-count are printed.
Vectors fp_clp_int_engine_kernel, fp_bmc_int_two_queries.
Leads: clp rewrites the context's own rules (mk_query into m_rule_set,
apply_default_transformation replaces it) - g3 keeps the originals (probes
clp1, clp_unsat ~105); tab free order with multiple queries (tab4 #42 vs
#37) and in get_proof (tab2 ~126); duality persistent simplifier cache vs a
th_rewriter per simplify() (dual1 ~132); datalog first query runs one extra
project_rename/union; pdr ~171439 and spacer ~194541 id-reuse divergences;
get-info :all-statistics lacks :time; qlinear eval_q audit; slice proof
converter positions; dead karr fixpoint code and unsliced-proof retry.

## Performance round 2 (2026-11-12, late)

No behaviour change (fullcmp2 763 identical, two fewer timeouts). Measured:
slow_3097 56.2 -> 47.4 s, g7_qsat 11.7 -> 11.5 s (z3 8.1 s / 2.1 s),
trace=true runs 10-13% faster. Changes: app records cache decl
family/kind/range (appIdentity) for ~30 recognizers; batched seq queries
(SequenceConcatOrEmpty, SequenceFlattenConcatAppend, SequenceUnitOrString);
slot-indexed seqEvalCache; trace formatting reads all arguments in one
payload lookup with a slot-indexed flatten cache; node hashing/equality
read payloads in one lookup; open-addressed AST hash-cons table (teardown
order dictionary kept). Tried and dropped: 48->32-byte node record (no
gain). Not done (would change behaviour or too risky): BV numeral fast path
(slot generation side effects are visible), single busy word for numeric
manager checks.
Next: theory_seq local holders' IncRef/DecRef pairs (~18% of slow_3097;
batched reverse release under one lock, fewer redundant holds in canonize /
expand / simplifyEq); batched bit2bool + equality literal creation in
addFixedBitEquality (~35% of g7); trace mode: flattened format node
hash-consing (~40%), prepareTraceAcquisition frame init (~3%), let-binding
pretty printer (~22%); relevancy watch maps by slot; hash-cons teardown
dictionary as a position-indexed slice.

## Fixedpoint round 3 (2026-11-12, late)

FIXED: query clones share the context's rule manager (z3 has one per
context: last head/body, label rewriter cache and pinned predicates persist
across queries); clp writes the transformed program back into the context
(mk_query + apply_default_transformation), so repeated clp queries now give
z3's answers (e.g. query u then query p is unsat/unsat in both); clp query
predicate is the first output predicate in hash-table order; tab unifier
members live until the next unify; tab certificates use mk_pp; iz3
InterpolationError surfaces as z3's "query failed: theory not supported by
interpolation or bad proof"; empty sparse-table results clear the register
(no extra project_rename/union); query command reports :time; spacer walks
rules in tag-table hash order. Vectors fp_clp_* (7), fp_tab_requery_rule_manager.
Leads: tab3 line 121 (smt solver pop order after the tautology check, ids
#66/#70 swapped - smtcore); tab_proof line 75 (proof-mode rule flush frees an
and term later); dual1 line 110 (theory rewriter proof cache keeps proof
terms in z3 - pkg/ast/rewriter); spacer_multi line 34332 (and conjunct order
around mbp/der); pdr_multi line 388642 (g3 frees a lemma z3 keeps, likely the
lemma-to-level map / m_levels); rlimit-count differs (51 vs 47, 80 vs 79:
missing rewriter or rule-transformer canceled() checks); coverage check for
karr datalog machinery (fp/datalog_karr_*) and errRetryUnslicedProof
before deleting; qlinear eval_q kept as g3's corrected version (the
source path crashes z3 4.7.1: "Failed to verify: unifier.unify_rules").

## smtcore/qe ordering leads (2026-11-12, late)

FIXED: qe BoolAtomIndex borrowed atoms instead of taking references (C++
atom_set is obj_hashtable<app> of raw pointers); the extra refs made
must_cache true and changed the asserted rewriter cache contents/release
order. Bit2Int.rewrite is a faithful for_each_ast traversal (last argument
reduced first; patterns, no-patterns, body; separate visited mark).
arith_plugin::subst's get_cache re-adds the branch's variables (duplicate
add_var, confirmed with gdb), so z3 eliminates them twice (replayBranch).
arith_project_plugin's pinned literals released front to back before
tids/mbo/eval. Vectors qe_int_forall_exists_atom_cache,
qe_int_mod2_cooper_check_sat, qe_int_mod3_bit2int_check_sat,
qe_int_mod3_bit2int_apply, qsat_exists_forall_real_pinned,
qsat_exists_forall_int_pinned.
DONE (2026-11-12): asserted_formulas::assert_expr input lifetime ported
(the rewriter result overwrites `expr_ref r(e)`, so an unowned input dies
right after rewriting); pdr SMTContext.Assert now holds `fml = m_pushed ? e
: mk_implies(m_pred, e)` across the assert as C++ does (resumable owner
release), spacer internalizeAssertions holds its implication. Vector
qe_rec_int_forall_implies restored (trace-identical).
Leads: audit other callers that pass fresh unowned expressions to
SMTKernel/SMTSolver.Assert (spacer predicate_reach/rules/transformer,
context_invariant, qe, tactics, opt) against C++ expr_ref holders; a debug
check that the root is alive at rewriter publish would find the next one;
spacer VirtualSolver.Assert pushed path / AssertTracked pass raw e as C++
does (check the caller chain); PropSolver.AddLevelFormula and spacer
internalizeAssertions drop the ref record without a resumable receipt when
DelRef leaves pending work.
Leads: (apply qe)(check-sat) mod 2 probe diverges at line 7571425 (free-list
order from qe tactic cleanup: quantElimPlugin.Close -> SolverContext.Close ->
ArithmeticPlugin.Close vs C++ member destruction order); MBP
linearize/maximize temporaries in the shared hold list.

## Performance round 3 (2026-11-12, late)

No behaviour change (fullcmp2 776 identical; a stricter base-vs-new
comparison of full traces, stdout and exit codes over all vectors: 0
mismatches). Measured: slow_3097 47.5 -> 40.0 s (z3 8.1 s); g7_qsat
unchanged (~11.8 s, z3 2.1 s); trace=true runs 37-45% faster. Changes:
ASTManager.DecRefEach/DecRefEachReverse (same DecRef sequence under one
lock, pending deletions resumed between elements) for theory_seq holders;
lendVec/takeVec move references instead of copying; appendConcat flattens
and references in one call (SequenceFlattenConcatAppendHeld); format
flattening returns unchanged nodes and short-cuts marked choice nodes
(both off under the app_ground debug tag); NumericTextStream writes whole
runs; slot-indexed relevancy handler/watch tables.
Next: batched seq queries for ReduceEqVec (~14% of slow_3097) and
canonizeInto/expand (~20%); nodeAt misses (14%) and manager lock per tiny
query (~3.5%); a format-node lookup that does not allocate a temporary
(audit format-manager generations); g7 model numerals (BitVectorNumeral
~14%, ArithmeticNumeral ~20%) constrained by visible slot generations;
numeric-manager checkReady/checkCleanup (~5%); defer overhead in
mkAppCoreArguments.

## Search divergences behind array model differences (2026-11-12, late)

FIXED: boundedSearch ran PropagateCore again after FinalCheckTheories
returned Continue (C++ bounded_search just loops back, so propagate runs
once); the extra round let arith propagate_bounds derive a unit lemma z3
never derives. add_store/add_map/add_const/add_as_array use the two-argument
set_prop_upward(v, d) (marks the bases of the class's existing stores, not
v itself; partly undoes d2d54d342), removing extra axiom2b instances. Array
trace text and order matched to the fork (-tr:array headers identical).
Vectors final_check_propagate_model, final_check_propagate_bounds,
array_store_class_upward. gen2 fuzz seeds 9000-11000 all match (9187 crashes
z3 itself).
Leads: -tr:array term printing uses mk_pp where C++ uses mk_bounded_pp /
mk_ll_pp (trace output only); theory_array set_prop_upward's
!m_array_delay_exp_axiom branch (instantiate_axiom2b_for /
instantiate_axiom_map_for) unported (DelayExpAxiom defaults to true);
pkg/smtcore/theory_bv_conversion.go is not gofmt-clean (pre-existing).

## Assert caller ownership audit (2026-11-12, late)

DONE: debug-build check in assertFormulaCore (g3debug) that assert_expr's
input is still alive after rewriting, naming the caller (forwarders, the
C++ raw-temporary sites - unit_subsumption, implied_equalities, qsat
prepareKernels, qe satWeaken/satCoreExtrapolate - and fixtures exempt).
Fixed callers that needed the C++ expr_ref: spacer AddReachFact (formula
reused after the assert), PropSolver.AssertAtLevel (two contexts),
derivation CreateNextChild (mk_and summaries), bmc nonlinear
checkNonlinearLevel (q_at_level), VirtualSolver.AssertTracked (new_t),
duality validateBackgroundProof. Resumable releases for pdr
AddLevelFormula and spacer (spacerAssertOwner). Vectors
fp_spacer_level_guard_rewrite, fp_pdr_level_formula,
fp_bmc_nonlinear_two_preds.
Leads: bmc nonlinear probe (p(x) & r(y) & x+y=3 -> q) id reuse differs at
trace line 186 (#83/#87/#99) and a spacer reach-fact probe at 85488;
asserted_formulas.cpp:163 reads has_quantifiers(e) after the rewrite frees
a raw-temporary input (C++ use-after-free at those sites; g3 computes it
before); spacer VirtualSolver does not reproduce solver_na2as::m_assumptions
(AssertTracked appears unreached); exemption lists name functions as
strings; gofmt flags theory_bv_conversion.go, propositional_engine.go,
tab/search.go, tab/unfold_clause.go.

## Performance round 4 (2026-11-12, late)

No behaviour change (strict base-vs-new comparison over all vectors: 0
mismatches; fullcmp2 783 identical). slow_3097 40.5 -> 35.5 s (z3 8.1 s);
g7_qsat unchanged (~12 s, z3 2.1 s); trace=true runs 5-9% faster.
Changes: first two app arguments inlined in the app record (inlineArgs;
debug builds read the checked way); SequenceEqEnds and IncRefThenDecRef
batch ReduceEqVec's queries and ref pairs; IncRefSequenceConcatOrEmpty in
canonizeInto; seq sort parameter cached on the sort record; enodes store
decl family/kind (arith shared checks); pooled reduce_eq vectors; defers
moved off hot paths; compile-time trace frame size.
Next: node record first-touch misses (nodeAt 15%, app record 8%); seq
hold/release pairs (~8%); MkSequenceLength rebuilds its decl per call
(caching would change slot generations); isVar sort checks (5%); g7 is
mostly real hash-consing (addFixedBitEquality 35%, BV model numerals 14%);
trace-mode format lookup without a temporary node (~4-5%, generations only
compared for equality); symbol manager sync.Mutex (~3.5% of trace time,
needs a concurrency decision).
FIXED (2026-11-13): debug-tagged builds were nondeterministic on
qe_rec_real_disjunction (qsat PredicateAbstraction.ValidateDefinitions
iterated pred2lit, a Go map; now obj_map order) and generally differed from
release because debug-only checks built/freed terms, cached numerals and
wrote trace records. New ASTManager.IsolatedCheck runs such checks with the
trace muted and restores the id generators, free-list order and fresh-name
counter (and drops newly cached numerals) afterwards; used by qe validation
checks, theory_lra validate_*, the inc_sat extract_model DEBUG_CODE (on a
ScratchCopy of the model). Sweep over all vectors (debug x5 vs release):
773 identical, 12 fpa vectors differ only in trace because C++ Z3DEBUG
fpa2bv mk_const uses the fpa2bv_sgn_/exp_/sig_ three-constant encoding
(ported faithfully; kept), 2 fail C++ SASSERTs in debug as z3 does
(definition_coercions, solve_eqs_proof).
Leads: re-sweep the 10 vectors skipped for time; other debug-only checks
not reached by vectors should go through IsolatedCheck; release-path
fidelity: PredicateAbstraction.Reset releases keys in insertion order (C++
dec_keys walks obj_map order), MBP force-elimination shares one scratch
evaluator (C++ builds one per model::eval), release validateAssumptions
keeps values to call end; ScratchCopy on a bare ModelCore.

## Quantifier fuzzing (2026-11-12/13)

New fuzzer gen4.py (random forall axioms over f/g/p with and without
patterns plus ground facts, full stdout with get-model).
FIXED: crash (nil deref in MAM isEq/collectParents): undo_add_eq cleared
the congruence pointer of parents with congruence closure disabled (arith +
and *), which are never re-added; z3 never touches m_cg for them.
push_app_ite builds the original f(args) only for proofs; MBQI candidate
terms (get_term_from_ctx) follow m_root2value obj_map order kept across
models; MBQI simple projection uses the fresh witness constant k itself
(insert(k, 0), set_else(k)) instead of k's numeral - a deliberate g3
deviation removed, so e.g. forall x y. y = x is unknown as in z3 4.7.1 (g3
said unsat, which is mathematically right and what upstream z3 says);
reweighted patterned quantifiers replace the original by ref; the instance
checker finds congruent terms through the congruence table (FindApp);
rejected parser patterns are released in z3's stack compaction order;
complete_cex dies after the aux pop. Vectors qfuzz_* (6).
Leads: k!N projection names swapped on several seeds (389, 416, 515, 1065,
1161, 1202, 1245): auf node creation / union order (get_uvar, get_A_f_i;
g3 runs addMonotoneExceptions up front, z3 inside mk_mono_proj); q_80:
theory_arith propagate_cheap_eq finds an equality at level 0 after a
restart where z3 finds it at level 1 (m_to_check / m_var_offset2row_id after
restart and MBQI instance insertion); q_130: arith bound-axiom clause
creation order (mk_bound_axioms / flush_bound_axioms); MBQI-heavy cases are
5-20x slower than z3 (q_1070, q_1109 > 90 s).

## Quantifier fuzzing round 2 (2026-11-13)

FIXED: model_finder::process_auf order (per quantifier: its variable nodes,
then each qinfo's process_auf; add_mono_exceptions inside mk_mono_proj;
x_eq_y skips the second insert_avoid for one node) - fixes swapped k!N
projection names; hint_solver returns non-candidates first; m_ng_decls only
from the analyzer's process_app; context::propagate checks can_propagate()
after m_qmanager->propagate (no extra loop re-running arith
propagate_bounds); k-diseq pending constraints no longer skip the model
check / force restarts (g3 hung where z3 takes 0.01 s) and Restart marks
relevant with propagation; cache_generation / m_cached_generation ported
(enodes recreated after backtracking keep their generation);
is_var_and_ground / is_x_gle_t_atom temporaries released as in C++. Vectors
qfuzz_auf_node_creation_order, qfuzz_cheap_eq_after_instance,
qfuzz_k_diseq_restart_relevancy, qfuzz_hint_candidates_last,
qfuzz_analyzer_ng_decls, qfuzz_cached_generation_reinit.
DONE (user-approved STL exception, 2026-11-13): libstdc++ (GCC 13) std::sort
ported for theory_arith flush_bound_axioms (pkg/smtcore/libstdcxx_sort.go;
compare_atoms compares k only, so ties take introsort order with > 16
atoms). Positive control src/test/stl_sort_order_control.cpp ->
pkg/smtcore/test_vectors/libstdcxx_sort_order.txt (1206 lines incl. McIlroy
inputs reaching the heap fallback); negative controls (stable sort, no heap
fallback, threshold 15) fail. Vectors flush_bound_axioms_introsort_{q2119,
q3063,q3110,old_q515} (stdout-identical; their full traces exceed the
fullcmp2 timeout). Fuzz seeds 2000-2400 / 3000-3400: 0 differences.
DONE (approved extension): theory_lra's compare_bounds std::sort
(theory_lra.cpp:1633-1634) uses the same port; libstdcxx_sort.go is now
index-based (libstdcxxSortData Len/Less/Swap) with typed wrappers for
atoms and LRA bounds; TestLibstdcxxSortLRABoundsMatchesControl added; on 200
random QF_LRA probes the stable sort diverged from the fork's trace in 190
(4 with different models) and the port matches all 200. Vectors
lra_flush_bound_axioms_introsort_s150 / _s30. Note: 4.7.1 reaches
theory_lra only via setup_r_arith (QF_LRA / QF_UFLRA), not
smt.arith.solver=6. Leads: QF_UFLRA unprobed; other std::sort call sites in
the C++ source not surveyed for tie order (each needs its own approval).
Lead: q_2119 / old_q_515 traces diverge deep (lines ~97.5M / ~37.8M): the
fork assigns a core equality literal (justification -1) before conflict
resolution where g3 goes straight to resolve-lit (eq_propagation /
eq_root_propagation order).
Leads: q_3157 ~8x slower than z3 (same answer); cache_generation reinit set
uses preserveLearnedClauses / learnedUnits instead of z3's
m_clauses_to_reinit buckets; classifyAUF reimplements the analyzer's
m_is_auf; a permanent activity_profile trace site would help (most z3 -tr
tags create pretty-print ASTs and change z3's own behaviour).

## Seq/core search divergences, fuzz round 2 (2026-11-13)

FIXED: theory_seq branch_variable v0 is reassigned like expr_ref (frees the
old value); branch_binary_variable's ys.size()-xs.size() is an unsigned
difference converted to int (1-2 = -1, not 4294967295); dynamic Ackermann
propagate_eh runs once per resolved conflict in bounded_search (not inside
the propagate loop); restart / conflict-limit checks only when
!inconsistent() after resolve_conflict; congruence reinsertion walks
r1->m_parents in source order interleaved with true-eq assignment (first
inserted stays the root); undo reinserts over sourceParents; arith_eq_adapter
uses one relevancy handler marking eq, le, ge (AddPairTargetsDependency).
e_12200 (timed out) now trace-identical in 0.05 s; vector
seq_binary_eq_negative_length_offset. e_12045 / e_12302 / e_12018 match z3
for 212-293M trace lines but are 4-6x slower than z3.
Leads: qfuzz_cached_generation_reinit diverges at line 178 in preprocessing
(z3 builds (* -1 (f x)) with the ite outside; g3 (* -1 (if c (f x) a)));
make sourceParents the only congruence parent list (drop the separate
parents list and fallbacks; remove popped enodes from r1.sourceParents);
dynAck.propagate's progress result is now unused; with trace=true,
qfuzz_cheap_eq_after_instance writes a very large trace (z3 too) and times
out in fullcmp2 (stdout identical, 1.8 s untraced).

## Congruence parents and MBQI follow-ups (2026-11-13)

DONE: enodes keep one parent list (sourceParents = z3 m_parents): merge uses
remove_parents_from_cg_table / reinsert_parents_into_cg_table, undo erases
the r2 tail and reinserts from r1; SMTEnode.delete removes one occurrence
per argument (del_eh), which left f(x, x) stale before; true-equality
parents are left alone on undo; dynAck.propagate returns only an error.
proto_model reregister_decl moves the old interpretation to f_aux and
inserts the new one without destroying the old (ModelCore.ReplaceFunction);
simplify_clauses' "nothing to simplify" check counts only non-binary
clauses (binary clauses live in watch lists). The (* -1 (f x)) ite lead was
already gone. Vectors qfuzz_reregister_keeps_cached_interp,
qfuzz_simplify_clauses_binary_only. gen4 seeds 4000-4400: all traced runs
identical.
Leads: equality trail and enode creation are separate trails (C++
interleaves them on one trail; undo copes via the r2 tail and
deleteAfterEqualityUndo); -tr:delete_node_bug differs on seed 4034 (format
nodes; not visible in trace=true); ReregisterFunction is not
transactional if ReplaceFunction failed after f_aux was registered.

## Status (2026-11-13): qsat/qe and theory_pb crash fixes backported

The fork's src/qe/qsat.cpp, src/qe/qe.cpp and src/smt/theory_pb.cpp carry
the backported fixes (see the z3 4.7.1 bug notes above), and g3 ports them.
build/z3, build_dbg/z3 and build_trace/z3 were rebuilt with them; the
pre-backport binaries are saved in
/var/tmp/g3-job-487c9ee8/fork-before-qsat-pb-backport/.
fullcmp2: 801 trace-identical, 0 differ; the only stdout difference is
bv_native_operator_models (documented z3 bug); 11 timeouts are slow
trace=true runs. qe_rec_real_disjunction's expected output changed: the old
fork answer reduced the satisfiable existential to false (unsound); the fixed
elim leaves it unchanged.
The same fixes (plus the relevancy and pareto backports of c3c0016bd) were
applied, uncommitted, to ivy's vendored z3 (goivy/z3vendor/z3), with
FORK_BUGFIXES.md, fork_bugfixes/test_vectors and src/test/fork_bugfixes.cpp.

## Fuzz round 14 (2026-11-13)

gen.py 5000-5300, gen2.py 5000-5300, gen3.py 13000-13300, gen4.py
5000-5250 against the rebuilt fork. Four quantifier seeds differed only
under the 10 s limit (identical at 30 s). Two real differences:
- gen2 5298 (no set-logic -> real difference logic): z3 prints
  `(smt.diff_logic: non-diff logic expression (+ (- (/ 1.0 4.0)) r))` and
  answers unknown; g3 answered sat. theory_diff_logic's mk_var(app*) flags
  any arith-family app (is_interpreted) even when it already has a theory
  var; g3's ensureVariable only flagged unattached nodes, never printed the
  verbose message (its mk_pp format terms are in the trace), and
  ApplySortConstraint created variables although C++ apply_sort_cnstr is
  empty for diff logic. DONE: faithful port of the theory_diff_logic
  internalization (smtcore/theory_diff_logic_internalize.go: init, mk_term,
  mk_num, both mk_var, is_offset, internalize_atom/term,
  found_non_diff_logic_expr with its IF_VERBOSE(0) mk_pp; mk_value is Int
  for Int terms; get_name "difference-logic"). Vector
  diff_logic_non_diff_offset_term. fullcmp2: 802 identical / 0 differ.
  Remaining notes: C++ mk_term -> mk_var(n) -> internalize(n) recurses
  forever on an undecomposable arith term that is not yet internalized
  (e.g. (f (to_real x)) with smt.arith.solver 1); g3 detects the re-entry
  and returns an error. The mixed variant keeps g3's separate Real 0.0
  enode (C++ dereferences a null enode there). g3 uses the mixed theory
  for both idl and rdl in AS_DIFF_LOGIC mode (unchanged).
- gen4 5077 (MBQI): different models (z3 k!2065 projection, a = 7736; g3
  a = 23). Untraced, g3 does far less work (11129 vs 68183 instantiations)
  but trace=true runs agree for 95M+ lines: z3's trace stream itself
  changes the search, because [assign]/justification display uses
  literal::display -> mk_pp, whose format ASTs shift later AST ids. g3
  copies that faithfully, so trace=true cannot see untraced divergences.
  NEW DIAGNOSTIC: Z3_TRACE_NO_PP=1 (fork src/smt/smt_literal.cpp, g3
  smtcore/smt_context_trace.go traceNoPP) prints literals as #id; then a
  traced run follows the untraced search (z3: identical stdout and -st).
  With it, 5077 diverges at log line 945118: in conflict 6374 the
  conflicting mp_iff_justification has its enodes swapped (z3 node1 is
  #8689's enode, g3 #8643's), so resolution order differs. Both creation
  sites (propagate_bool_var_enode, propagate_bool_enode_assignment_core)
  and set_conflict match the C++. FIXED: my first diagnosis (z3 not
  finding #8689 at mk_enode) was wrong; z3 does find it, and the
  congruence only looks queued last because propagate_atoms runs before
  propagate_eqs. Real causes: (1) add_diseq (smt_context.cpp:1117-1124)
  sets a conflict for an already-equal pair only when the root has no
  theory var or its theory does not use diseqs; g3 always set it and
  stopped before the #8861=#8689 merge. (2) Lemma GC
  (del_inactive_lemmas2 :2743, init_search :3383-3388): per-search reset
  of the GC counter and threshold, GC from the base scope's lemma limit,
  can_delete/is_justifying with the reinit stack, threshold always
  old_clause_activity (unsigned (i-start)/real_sz is 0), decay only kept
  clauses. Also ported: one interleaved trail for bool var / enode /
  add_eq undo (undo_add_eq and undo_mk_enode exactly, workarounds
  removed), and pop resets the eq / th_eq / th_diseq queues as
  pop_scope_core does. 5077 is now trace-identical (43,966,487 lines with
  Z3_TRACE_NO_PP) and stdout-identical. Not added as a vector: g3 takes
  31 s untraced (z3 4.8 s). Remaining: lemma_gc_half variant does not
  reorder stored lemmas after its sort; diseq undo has its own trail.
Fuzz round 15 (30 s limit): gen.py 6000-6300, gen2.py 6000-6300, gen3.py
14000-14300, gen4.py 6000-6200: no differences.
fullcmp2 with Z3_TRACE_NO_PP=1 (untraced search, smaller logs, no timeouts)
at 0580ae369: 810 identical, 3 differ: flush_bound_axioms_introsort_old_q515
(line 204370), flush_bound_axioms_introsort_q2119 (414899),
quant_seq_symbolic_int_prefix (431345); stdout: bv_native_operator_models
only. After the interleaved-trail / add_diseq / lemma GC change: 812
identical, 1 differs: quant_seq_symbolic_int_prefix (line 431345); the
two flush_bound_axioms_introsort divergences are gone.
FIXED quant_seq_symbolic_int_prefix: smt::context teardown order. The atom
held by m_is_diseq_tmp is released in flush() right after
del_clauses/del_justifications (smt_context.cpp:2893-2898); ~context
destroys members in reverse declaration order (dyn_ack, cg_table, then
theories); th_rewriter::imp destroys m_cfg before the rewriter caches;
~ast_manager prints undef with the released proof sort. With
Z3_TRACE_NO_PP: 813 identical / 0 differ / 0 timeouts. Remaining at exit
only: g3's cmd_context restoreDeclarations deletes 4 user func_decls that
z3 never deletes (cmd_context::reset calls reset_func_decls first;
cmd_context.cpp:1244-1258); no log or stdout effect. Possible follow-ups:
~plugin_manager deletes theories front to back (g3 closes back to front);
m_fingerprints is destroyed after the theories.

## Fuzz round 16 (2026-11-13, after the interleaved-trail fix)

gen.py/gen2.py 7000-7300, gen3.py 15000-15300, gen4.py 7000-7250: no
output or model differences. Three quantifier seeds exceed the 30 s fuzz
limit in g3 only (performance, not fidelity):
- q_7028: z3 14.9 s, g3 > 300 s untraced; with Z3_TRACE_NO_PP the
  trace=true logs are identical (185,246,315 lines).
- q_7056: z3 5.4 s, g3 114 s, same output.
- q_7248: z3 5.6 s, g3 34 s, same output.
Inputs kept in the job fuzz dir r16; candidates for the MBQI/e-matching
performance work (see the performance rounds above).
q_7028 profile (G3_CPUPROFILE, or /usr/lib/linux-tools/6.14.0-37-generic/perf):
52% in mkBoundAxioms' threshold comparisons (big.Rat.Cmp cross-multiplies
and allocates). smtRatCmp compares numerators when denominators are equal,
and atoms cache a small-integer threshold for the occs scan: >300 s -> 107 s
(z3 14.9 s), output identical. Integer numerals are now read with
SetInt64 (no GCD): 104 s. q_7056 114 s -> 27 s, q_7248 34 s (z3 5-6 s).
Remaining: about 55% of q_7028 is the rewriter under MBQI's auxiliary
counterexample check (mk_eq_core steps over large model-function ite terms),
spread over per-AST-access overhead (ASTManager lock, validNode, nodeAt,
GetKind/GetFamilyID); that is PERF.md P5 (batched reads), an architectural
change, not a local fix.

## Fuzz round 17 (2026-11-13) and the ASTManager lock question

gen.py/gen2.py 8000-8300, gen3.py 16000-16300, gen4.py 8000-8250: one
difference, q_8169 (MBQI, LIA+UF): z3 b = 0, g3 b = 5013. With
Z3_TRACE_NO_PP the logs agree to the learned lemma at line 507130, where
g3 keeps #3427 ((>= #3401 #2955), justified by a from-theory -1
justification with antecedent (not #3121)) that z3's minimize_lemma
removes; z3 then pops 11 scopes, g3 6. FIXED: minimize_lemma's
m_lvl_set is a level_approx_set (approx_set_tpl<unsigned>, a 32-bit mask
of level & 31; smt_conflict_resolution.h:34, approx_set.h), so levels
congruent mod 32 alias; g3 used an exact level set and rejected an
antecedent z3 followed. q_8169 trace and stdout now identical (not added
as a vector: g3 11 s, z3 3 s; unit test TestMinimizeLemmaApproxLevelSet).
ASTManager locking: the user chose "no mutex" (manager confined to one
goroutine, like z3's ast_manager); that is already the case
(astOwnerConfined no-op Lock/Unlock). The remaining per-access cost is the
checks in lock() (Self, trace/format re-entrancy flags, Closed,
sourceDTStarted) and validNode/nodeAt handle validation, about 10-15% of
the q_7028 profile; reducing them means hoisting validation out of hot
loops (P5 follow-up).

## Fuzz round 18 (2026-11-13)

gen.py/gen2.py 9000-9300, gen3.py 17000-17300, gen4.py 9000-9250:
- gen2 9187 `(assert (distinct 0.0 s 4.0)) (assert b)`, no set-logic: the
  fork segfaults (release and debug) in smt::enode::mk <- context::mk_enode
  <- internalize_eq <- internalize_distinct inside smt_tactic's context;
  g3 answers unknown (difference logic gives up). Consistent with the
  diff-logic port's note that C++ dereferences a null enode for the real 0
  numeral; a z3 4.7.1 crash, not a g3 divergence (z3 crash list).
- q_9009: z3 unknown in 10 s; g3 > 600 s untraced. Not a divergence: with
  Z3_TRACE_NO_PP g3's log is an exact prefix of z3's. 91% of g3's time was
  model-value construction: ArithmeticNumeralRat turned large values into a
  decimal string and SetString parsed it back with a multiply-add per digit
  (quadratic). RationalService.SetBigRat now sets numerator/denominator from
  big.Int limbs (same normalized value; unit test against SetString):
  69 s, output identical. Remaining: about 30% is numeral copies digit by
  digit through checked accessors (IntegerManager.copyFromOwner...,
  Cell/Digit/writeDigit) where z3's mpz copy is a memcpy; a bulk-copy fast
  path needs care with the allocation-receipt bookkeeping (follow-up).

## Fuzz round 19 (2026-11-13, after 907076aff)

gen.py/gen2.py 10000-10300, gen3.py 18000-18300, gen4.py 10000-10250: no
output or model differences. q_10065 exceeds the 30 s fuzz limit only
(z3 17 s, g3 54 s, identical output).
