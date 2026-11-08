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

2. **Port `theory_seq::final_check_eh` faithfully.** Logics missing from
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

6. **Typed tactic statistics.** `Tactic.CollectStatistics()` returns
   `map[string]float64`; C++ `collect_statistics(statistics&)` keeps uint
   and double values apart. Go prints `5.00` where z3 prints `5` and cannot
   sum duplicate keys the way `statistics::display_smt2` does
   (84 implementers).

7. **SAT solver statistics.** Statistics from the SAT path differ widely,
   and the SAT solver's `mk bool var` is not reported
   (qflia_solver_pb_sat_pipeline check 1).

8. **`(apply ...)` goal display.** Go omits
   `:precision precise :depth N`.

9. **Clause storage and del-clause.** Go keeps one clause list where C++
   has `m_aux_clauses`/`m_lemmas` with `del_clauses` at pop; `del-clause`
   differs (shared_arithmetic_fixed_integer: z3 1, Go 3). The same storage
   prevents binary lemmas above scope 0, which C++ allows when relevancy
   is below 2.
