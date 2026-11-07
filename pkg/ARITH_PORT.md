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

## Status

- Phase 0 (this plan): done. B5 stays open until phase 5.
