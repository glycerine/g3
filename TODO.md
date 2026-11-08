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
   Next: model and value printing (format_ns `string`/`compose`/`cr`
   ASTs, labels.smt2 and others), and the remaining buckets of first
   differences (records 33-40) in the per-vector comparison.
   The earlier example, `(check-sat-using smt)(declare-const z
   Int)...(check-sat-using smt)` with x+y=7 and x>=0, now has identical
   AST logs: the apparent destruction-order difference came from the
   tactic construction order above, not from smt::context teardown.
   OPEN: in principle smt::context::flush still differs (z3 undoes one
   interleaved trail of mk_bool_var/mk_enode entries; g3 unwinds enodes,
   then Boolean variables), which matters only when a term's last
   reference is one of the two. Related: z3's relevancy propagator keeps m_relevant_exprs as an
   expr_ref_vector, so its terms live until it unmarks them on pop. g3's
   propagator holds no references; the ported unmark_relevant_exprs
   TRACE (datatype_update_equality_scopes) shows terms already deleted
   when g3 unmarks them, so g3 frees them earlier than z3 does.

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
    - OPEN: the minterm queries themselves differ. g3's
      sequenceAutomatonBuilder (ast/rewriter/seq_rewriter_automaton.go)
      is not a port of re2automaton/symbolic_automata (mk_complement,
      mk_minterms, determinization): on seq_negative_symbolic_mixed z3
      checks x!2, x!4, x!10, x!12, x!14, x!16 and g3 checks x!0, x!2,
      x!4-x!7, x!11, x!13, x!15, x!17. Port symbolic_automata_def.h for
      the retained callers.

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
