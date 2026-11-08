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

3. **AST lifetimes of arith rewriter temporaries.** Go's arith rewriter
   returns ref-count-0 handles that a later sweep frees; C++ frees each
   `expr_ref` temporary on scope exit. Recycled AST ids then differ, and
   mon_lt orders sum monomials by id (lia2pb_model_scopes).

4. **get-consequences algorithm.** Go's `get-consequences` does not follow
   `smt_consequences.cpp` (get_consequences: 11 checks vs 5, no eq adapter).

5. **integer_auto_deep_ite.** Checks 1-2: one added equality too many and
   the equality-axiom clause count differs. Check 3: different first
   decision.

6. **DONE: Typed tactic statistics.** `Tactic.CollectStatistics()` returned
   `map[string]float64`; C++ `collect_statistics(statistics&)` keeps uint
   and double values apart. Go prints `5.00` where z3 prints `5` and cannot
   sum duplicate keys the way `statistics::display_smt2` does
   (84 implementers). Now `CollectStatistics(*statistics.Statistics)`,
   as `collect_statistics(statistics&)`: implementers add uint and double
   values, combinators pass the object down, and tactic2solver collects
   into its result and its own statistics. nlsat reports the five counters
   of `solver::collect_statistics`; horn keeps only the engine's.

7. **SAT solver statistics.** Statistics from the SAT path differ widely,
   and the SAT solver's `mk bool var` is not reported
   (qflia_solver_pb_sat_pipeline check 1).

8. **DONE: `(apply ...)` goal display.** Go omitted
   `:precision precise :depth N`: `apply` calls `goal::display(ctx)`, the
   ast_printer_context overload, which prints them.

9. **Clause storage and del-clause.** Go keeps one clause list where C++
   has `m_aux_clauses`/`m_lemmas` with `del_clauses` at pop; `del-clause`
   differs (shared_arithmetic_fixed_integer: z3 1, Go 3). The same storage
   prevents binary lemmas above scope 0, which C++ allows when relevancy
   is below 2.

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

11. **Extra SMT contexts (first golden-trace divergence).** On
    sequence_helper_inventory block 12, z3 creates 17 `smt::context`s
    (count of `creating boolean variable: 0` records with
    `-tr:mk_bool_var`); g3 created 52, now 27.
    - DONE: theory_seq's `seq_expr_solver` kernel. g3 installs theories when
      a solver is created, where z3 registers them at the first check/push
      (`setup_context`); building the kernel in `Initialize` made every
      constructed solver build one. It is now built in `Setup`.
    - OPEN: `mk_lra_tactic` falls back to `mk_qe_tactic` (qe_tactic.cpp,
      the classic quantifier elimination), which creates no contexts. g3
      uses QE2 (qsat-based `mk_qe2_tactic`) there because its port of
      qe_tactic.cpp (qe/quantifier_tactic.go) is partial. QE2's kernels are
      the 10 extra contexts. Fix: complete the qe_tactic port and use it.
    - OPEN: `qe::qsat::~qsat` calls reset(), creating two kernels at
      teardown (z3's last 2 contexts); g3's qsat Close does not.
    Until this is fixed no vector matches z3 record for record, so
    `TestTraceGolden` (cmd/g3, `-tags g3trace`, see the file comment) has
    no default vectors.

12. **Golden trace coverage.** Port more TRACE records into Go at the
    sites z3 traces (add each to `goldenTraceSites` in
    cmd/g3/trace_golden_test.go). Ported so far: assign_core (with the
    justification display), add_eq_detail, decide, mk_bool_var, and seq
    add_axiom and get_automaton. Next candidates: seq final_check_eh
    (theory_seq::display), the arith tags, and the remaining seq records.

13. **DONE: conflict counting.** z3 counts `conflicts` in
    `context::resolve_conflict`; g3 counted when a conflict was set, so a
    conflict found where z3 never resolves one (e.g. while internalizing
    an unsat assertion) counted 1. The count now happens in the resolver.
    It was the only differing statistic in 50 vectors.

14. **OPEN: inner-solver and outer `mk-bool-var`.** In
    seq_negative_symbolic_mixed the outer context makes 81 Boolean
    variables where z3 makes 77. Separately, the automaton's minterm
    queries differ: z3 checks each with a fresh constant `x!N` that is
    bit-blasted and decided; g3 uses `seq-automaton!N` and creates no bit
    variables. Comparing these needs context identity in the traces.
