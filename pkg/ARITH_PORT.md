# Porting `theory_arith` faithfully

Goal: replace Go's replay-based arithmetic (`smtcore/theory_arith_linear.go` and
friends on top of `math/simplex`) with a mechanical port of the C++
`src/smt/theory_arith*.h`, so behavior follows the source and fixes like B5
(`fixed_var_eh`) become direct ports, not adaptations.

## Why

PERF.md B5 showed that features of the source cannot be added faithfully to
the current design: `fixed_var_eh` depends on the persistent tableau, derived
bounds with explanations, and the bound trail, none of which exist in Go.
Attempts to adapt it each fixed one failure and caused another. Of the 297
`theory_arith<Ext>` member functions, about half are cited or named in Go, but
the engine is missing: `set_bound`, `add_row`, `mk_row`, `internalize_add`,
`internalize_mul`, `update_value`, `restore_assignment`, pivot selection and
gains, `try_to_imply_eq`, and all of `theory_arith_inv.h`.

## Scope

- Instantiations reached by `smt_setup.cpp`: `theory_mi_arith` (inf_ext:
  rational with infinitesimals) and `theory_i_arith` (i_ext). Port each as
  concrete typed Go, per AGENTS.md; no generic numeral layer. `theory_lra` is
  out of scope here.
- Source files: `theory_arith.h` (1,267 lines), `_core.h` (3,545), `_aux.h`
  (2,284), `_eq.h` (370), `_int.h` (1,424), `_nl.h` (2,467), `_inv.h` (232),
  `_pp.h` (550, display only as needed by retained callers).

## Acceptance

For each phase, on all `cmd/g3/test_vectors` and the differential fuzzers:

1. Verdicts and models agree with C++ (`fuzz.py`, `fuzzinc.py` in the audit).
2. **Arithmetic statistics match C++ exactly** (`arith-pivots`,
   `arith-conflicts`, `arith-bound-prop`, `arith-fixed-eqs`,
   `arith-assert-lower/upper`, `arith-add-rows`), and Boolean search
   statistics match. Exact agreement is the evidence that the port follows
   the source; any difference is a bug in the port until explained.
3. `make` stays green. Search must be deterministic (B4).

The old implementation stays selectable until the port passes, so every
phase can be compared side by side.

## Phases

1. **Data structures.** `var_data`, `row`/`column` (compare with existing
   `theory_arith_tableau.go`), `bound`, `atom`, `derived_bound`,
   `justified_derived_bound`, `antecedents`, the scope and trail stacks
   (`m_bound_trail`, `m_unassigned_atoms`, `m_update_trail_stack`), and the
   numerals: `inf_numeral` over the existing `util/numeral` rationals.
2. **Internalization.** `internalize_term_core`, `internalize_add`,
   `internalize_mul`, `mk_row`, `init_row`, `internalize_atom`, `mk_var`.
3. **Assignment and scopes.** `set_bound`, `assert_lower/upper`,
   `update_value`, `save_value`, `restore_assignment`, `push/pop_scope_eh`,
   `restore_bounds`, `del_vars`.
4. **Feasibility and conflicts.** `make_feasible`, `select_pivot`, `pivot`,
   gains, `set_conflict`, explanation (`antecedents`).
5. **Propagation.** `propagate_bounds`, `imply_bound_for_monomial`,
   `assign_bound_literal`, `fixed_var_eh`, `try_to_imply_eq`, offset rows.
6. **Final check.** `assume_eqs`, model generation, then `_int.h` (branch and
   bound, GCD test, Gomory cuts) for `theory_i_arith`.
7. **Nonlinear** (`_nl.h`, Gröbner), only as far as retained logics reach it.
8. **Switch over** in setup, delete the replay design and its tests, and
   rerun PERF.md measurements.

## Audit: what already exists in Go (name-independent)

Checked by location, call graph and statistics, not just function names.

- **Which C++ theory serves which logic (smt_setup.cpp, 4.7.1):** LIA-family
  logics use `theory_i_arith`; AUFLIA/UFLIA/UFNIA/QF_S and some others use
  `theory_mi_arith` (the B5 case); LRA-only logics use `theory_lra`.
  Default `arith.solver=2` (`AS_ARITH`) selects the old `theory_arith`.
- **`theory_lra` is a real port.** `pkg/util/lp` ports `lar_solver`,
  `lp_bound_propagator` and the row bound analyzer, driven by
  `smtcore/theory_lra*.go`. It is not part of this work.
- **`tactic/arith/bound_propagator` is unrelated** to the SMT theory. In C++ it
  is used only by tactics (`propagate_ineqs`), never by `theory_arith`.
- **`math/simplex` ports `src/math/simplex`** (sparse matrix, simplex, the
  `arith_*` helpers for GCD, Gomory, freedom intervals and row conflicts), not
  the `theory_arith` engine.
- **Ported but not wired in:** `smtcore/theory_arith_tableau.go` (`row`,
  `column`, `add_row_entry`, compress) has no non-test callers.
- **Ported in Go and live:** `fixed_var_eh`'s equality part
  (`propagateFixedEquality`, `theory_arith_fixed_bounds.go`), bound axioms,
  Gröbner, GCD test, Gomory cuts, branch and bound, assume-eq,
  `theory_arith_mutation`, objective code, statistics reporting.
- **Missing as a live engine:** no Go function plays the role of `set_bound`,
  `assert_lower/upper`, `update_value`, `restore_assignment`, `pivot`,
  `select_pivot`, `make_feasible`, `add_row`/`mk_row`, `internalize_add/mul`.
  `SMTLinearArithmeticTheory.Propagate` calls `rebuild()`, which rebuilds a
  `math/simplex` solver from all assertions. Its infeasible fallback conflict
  uses every assertion, not an explained row. The `arith pivots` and
  `arith assert lower` statistics are derived from that replay, so they
  cannot match C++ even when verdicts do.

Consequence for the plan: phases 2 to 5 are real new code. The data structures
in phase 1 mostly exist (tableau row/column) and need bound, atom and
var_data types wired to them. `propagateFixedEquality`, the explanation
helpers, Gröbner, GCD, Gomory and branch code should be reused and rewired
to the live tableau rather than rewritten.

## Ground rules for the port (restart)

The first attempt (commits 3e500de53, cd2c3cce4) split the theory into an
"engine" plus a "host" interface with a fake host for tests. That is not the
source structure and was discarded. The port follows the C++ directly:

- One type, `SMTTheoryArith`, is `theory_arith<Ext>`. It embeds
  `SMTTheoryBase` (the C++ base class `theory`) and calls the context through
  `t.state`, exactly where the source calls `get_context()`. No interfaces or
  adapters between the theory and the context.
- Files mirror the C++ headers and their order: `smt_theory_arith.go`
  (theory_arith.h), `smt_theory_arith_core.go`, `_aux.go`, `_eq.go`, `_int.go`,
  `_nl.go`, `_pp.go`. Function names are the C++ names in lowerCamelCase and
  appear in the same order as in the header.
- Already-ported pieces that match the source are reused as they are:
  `theory_arith_tableau.go` (row/column), `theory_arith_bounds.go` (bound,
  atom, eq_bound, derived_bound, antecedents, var data). Nothing new is
  invented for a type the source already has.
- Tests exercise a real `SMTInternalizerState`, not a fake.

## Status

Ported (commits dd0607fda through 3ed202f4c), all of `theory_arith<Ext>`:

- `smt_theory_arith.go` (theory_arith.h), `smt_theory_arith_core.go`,
  `_aux.go`, `_eq.go`, `_int.go` (with the euclidean solver bridge),
  `_nl.go` (intervals, cross nested forms, Gröbner), `_pp.go`
  (collect_statistics) and `_opt.go` (theory_opt members).
- `smt_arith_eq_adapter_source.go` ports `arith_eq_adapter::mk_axioms` for
  the port; the replay theory keeps the previous adapter behavior.
- `opt/arithmetic_solver.go` now rewrites objectives as `optsmt::add` does.

Selection: `G3_THEORY_ARITH=port` replaces the replay theory at
registration (i_ext for integer setups, mi_ext otherwise; theory_inf_arith
is not ported).

Verification:

- Focused tests: `smtcore/smt_theory_arith_test.go` (real context) and
  `cmd/g3/theory_arith_port_test.go` (B5, ground B5, mod/div without
  relevancy, minimize).
- All 659 `cmd/g3/test_vectors`: with the port, every vector where the
  replay theory agrees with C++ also agrees, plus B5, which only the port
  solves. Optimization vectors match C++ output byte for byte.
- Arithmetic statistics on small cases match C++ exactly (uf1, uf4); on B5
  they are close (add rows 7/7, assert lower/upper 6/6, bound prop 2/2, fixed
  eqs 2/2, pivots 5 vs 6, eq adapter 9 vs 8). Remaining differences sit in
  the surrounding search, not in the theory.
- `make` passes with the replay theory still the default.

Known deviations, all documented in the code: `flush_bound_axioms` and
`get_polynomial_info` use deterministic orders where the source's order
depends on std::sort or hash tables; Gröbner dependencies travel as AST
dependencies whose leaves index the bounds; value_trail entries live in the
theory's scope records.

`make` with `G3_THEORY_ARITH=port` (after 0b03b49cd): failures that remain
before the port can become the default (phase 8):

- Setup-routing tests that assert the replay type and its fields:
  `smt_arithmetic_lifecycle_test`, `smt_setup_difference_test` (two),
  `smt_setup_test` QF_LIA automatic setup. They need equivalent checks on
  `SMTTheoryArith`, not code changes.
- `TestOptimizationFrontendResourceLimitedOPBBounds` pins Go step counts
  under `rlimit`; C++ solves the instance outright at those limits. The port
  counts steps as the source does (`get_cancel_flag` increments), so the
  interrupted bound differs.
- `TestParetoCanceledImprovementDropsBackendAndAllowsFreshSearch`: when the
  resource limit fires inside asserted-formula preprocessing, the Go kernel
  returns an error where the source yields `unknown`. Kernel behavior,
  exposed because the port spends steps like the source.

Also needed for phase 8: `theory_inf_arith` (AS_OPTINF) is not ported, and
the sequence/string theories type-assert the replay theory for length
bounds; they must read bounds through `SMTTheoryArith` instead.

## Phase 8: switch-over (done)

`SMTTheoryArith` is the arithmetic theory for every `theory_mi_arith`,
`theory_i_arith` and `theory_inf_arith` setup (theory_lra logics keep
`SMTLRATheory`). The replay theory (`SMTLinearArithmeticTheory` over
`math/simplex`), its 79 files and 105 tests are deleted; the env switch and
the kernel swap are gone. `port_manifest.json` maps the C++ members to the
port (bcb01010a).

Source fixes made on the way, each checked against z3 4.7.1 (a TRACE build,
`build_trace/` from `mk_make.py -t`, was used to diff mk_bool_var,
mk_arith_var, arith_eq_adapter, arith_relevant_eh, decide and pdr traces):

- `poly_rewriter` `mk_mul_core` / `mk_flat_mul_core` / `mk_nflat_mul_core` /
  `mk_mul_app` ported for arithmetic (`ast/rewriter/arith_poly_mul.go`):
  `(* c (* x y))` as in C++, so theory_arith never sees a numeral-first
  three-argument product.
- `bounded_search` continues on FC_CONTINUE; the Go-only "theory made no
  progress" cutoff is removed.
- `theory_seq` length comparisons use `mk_sub` / `mk_simplified_literal`.
- `arith_eq_adapter::mk_axioms` builds `t1_eq_t2` with
  `theory_arith::mk_eq_atom` (`arith_util::mk_eq`); `internalize_eq_eh`
  passes the atom's own enodes; `restart_eh` works on enode pairs.
- `final_check_eh` traces `arith_eq_adapter_info`; `record_conflict` closes
  its local `th_rewriter`.

On the nested mod/div problem behind `TestExprQENestedModuloProjection` the
port creates the same 72 Boolean variables in the same order as z3, with the
same decisions, conflicts, GCD tests and Gomory cuts.

Tests written against the replay theory were changed only where C++ behaves
differently (bare constants have no arithmetic variable until an arithmetic
term mentions them; atoms are `<=`/`>=` normal forms; proof rules, statuses
and shared-context PDR models as z3 4.7.1 produces). `make` passes.

Open, outside theory_arith: the Go sequence theory does not follow
`theory_seq`'s unit decomposition (z3 refutes the first scope of
`sequence_word_equations_relations.smt2` in propagation with
`(= "a" (seq.unit #x61))` atoms; Go branches in final check). The
`str.contains` model in that script is therefore compared as "a valid
model" rather than byte for byte.
