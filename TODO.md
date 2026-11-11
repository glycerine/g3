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

22. **OPEN: opt::context::optimize flow (item 12 cluster).**
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

28. **Open (2026-11-10): automaton compress has a case z3 lacks.**
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

30. **Open (2026-11-10): opt pareto pipeline order.** In
    pareto_bit_vector_optimization, z3's opt::context::optimize runs
    init_solver first: the opt_solver's smt::kernel builds the qi_queue
    cost-parser variables, then the objectives are purified into fresh
    maximize!!N constants, and only then are the objectives bit-blasted.
    g3 bit-blasts the objectives before it builds the solver, and never
    creates the maximize!! constants. Port opt::context::optimize/normalize
    in source order.

31. **Open (2026-11-10): PDR/spacer query pipeline lifetimes (about 40
    fixedpoint_* and fp_spacer_* vectors).** g3's
    muz/fp queryRelationMkQuery is a reimplementation of
    engine_base::query -> pdr::dl_interface::query ->
    apply_default_transformation. Term lifetimes differ as a result. In
    fixedpoint_pdr_bit_blast, `(p x)` and `(= x x)` survive in z3 until
    ~mk_interp_tail_simplifier, when the rule transformer is destroyed
    at the end of apply_default_transformation; g3 frees them before it
    builds `query!0!bv!2`. Port the transformer ownership structure.

32. **Pretty-printer format ids (2026-11-10).** Fixed: get-consequences
    prints its vector as one ast_smt2_pp document (82381f483), and
    theory_str tester names release their mk_ismt2_pp document
    (7e04eec53). Three generic/legacy string vectors now match. Still
    open: bv_integer_conversion_scopes line 92987, where z3 reuses a live
    format string (#238) that g3 has already freed and rebuilds. Track
    format-manager deletions to find the earlier lifetime difference.
