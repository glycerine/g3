> Q: For Ivy, Ken McMillan uses a fork of the Z3 4.7.1 theorem prover with his extensions,
> and I wonder what features have been added to Z3 since then so I can warn my users
> (I'm making a port of that fork to Go) about what will not be 
> available from g3 (g3 is the Go port of Z3).

# A: What Z3 has added since 4.7.1 (and g3 therefore lacks)

g3 is a Go port of Ken McMillan's Ivy fork of Z3 4.7.1. Z3 4.7.1 was released in mid-2018, just before Z3's 4.8.0 overhaul. Upstream Z3 is now at 5.4.0, so g3 is missing about eight years of upstream work. This document lists what g3 does not provide, grouped by how likely you are to notice.

It is based on Z3's `RELEASE_NOTES.md` (current master) plus a diff of the public C API headers between the `z3-4.7.1` tag and master.

## 1. Changes to the input language

These are the most likely to show up as parse errors or different answers when you move a script between g3 and current Z3.

- **Lambdas** (4.8.0): `lambda` binders, `Z3_mk_lambda`, `Z3_mk_lambda_const`.
- **Recursive functions handled natively** (4.8.3): `define-fun-rec` and `define-funs-rec` with a dedicated solver, plus `Z3_mk_rec_func_decl` and `Z3_add_rec_def`. Unfolding was reworked again in 4.8.8, 5.3.0 and 5.4.0.
- **Unicode strings and a Char theory** (4.8.11): strings became 18-bit Unicode, with `Char` and `str.to_code`/`str.from_code`. String literal escapes behave differently from 4.7.1. Later additions:
  - `str.<` and `str.<=`
  - `str.replace_all`, `str.replace_re`, `str.replace_re_all`
  - `seq.nth`, `seq.last_indexof`
  - `re.diff`, `re.allchar`, `re.^`
  - regex xor (5.0)
- **Sequence higher-order operations** (4.9.0): `seq.map`, `seq.mapi`, `seq.foldl`, `seq.foldli`. Also datatypes nested through `Seq` (4.8.17) and `seq.power` (5.3.0).
- **Polymorphism** (4.12.3): `(declare-type-var A)` and `Z3_mk_type_variable`. Polymorphic datatypes followed in 4.15.4.
- **Bit-vector/integer conversions**:
  - the standard SMT-LIB bit-vector overflow predicates (4.12.2)
  - `ubv_to_int`, `sbv_to_int`, `int_to_bv` (4.14.1)
- **Entirely new theories**:
  - FiniteSet sort with `set.size`, `set.map`, `set.range` and filter (5.0.0)
  - QF_FF finite fields (5.3.0)
  - Native `Complex` arithmetic (5.3.1)
  - Special relations: `Z3_mk_linear_order`, `Z3_mk_partial_order`, `Z3_mk_tree_order`, `Z3_mk_piecewise_linear_order`, `Z3_mk_transitive_closure`
- **New commands**: `set-simplifier` (4.12.2) and `set-initial-value` (4.13.1).
- **Behavior changes to watch for**:
  - 5.3.0 rejects backslashes in quoted symbols.
  - 5.4.0 makes `(** Int Int)` return an Int. Before that it returned a Real, which is what g3 still does.
  - 5.4.0 also defines what `**` does with a zero or negative Real base.
  - 4.8.7 replaced the `model_compress` option with `model.compact`.

## 2. Features 4.7.1 has that later Z3 removed

This is why g3 exists. **Z3 4.8.0 removed the interpolation API, the Duality CHC engine and the PDR engine.** McMillan's Ivy fork depends on them, so current upstream Z3 cannot simply replace g3 in an Ivy workflow.

Current Z3 offers *model-based* interpolation for quantifier-free UF plus arithmetic instead (since 4.8.8). It is not the same as the proof-based interpolants Ivy relies on.

## 3. Solver engines g3 does not have

You will mostly notice these as speed differences and as `unknown` results, not as errors.

- **Arithmetic**:
  - A new LP/LIA solver (4.8.0) that became the default (4.8.10).
  - A rewritten NIA core (4.8.8).
  - Stronger LIA cuts that use Diophantine equation solving (4.15.0).
  - New NLSAT projection methods: sample-cell (4.13.1) and levelwise (4.15.5).
  - Multivariate polynomial factorization (5.0).
  - Many QF_LRA and Gröbner performance improvements.
- **Bit-vectors**: int-blasting with `smt.bv.solver=2` (4.12.5), and `smt.bv.reduce_size`.
- **Search**:
  - Parallel cube-and-conquer (4.8.0), replaced by a shared search tree for `smt.threads>1` (4.15.4), which later gained shared backbones.
  - SLS modulo theories, i.e. local search (4.14.0).
  - The new CDCL(T) core (`sat.euf=true`).
- **Quantifiers**:
  - `qel` term-graph quantifier elimination, and model-based projection for arrays and algebraic datatypes (4.12.3).
  - A higher-order quantifier solver and higher-order matching (5.x).
  - Improved MBQI with recursive functions.
- **Horn clauses (Spacer)**: a substantial overhaul in 4.8.0, and Global Guidance in 4.11.2.
- **Regex**: Antimirov derivatives (4.8.x), and a monadic regex solver that is on by default from 5.1.
- **Optimization**:
  - RC2 with totalizers and LNS anytime MaxSAT.
  - Adding constraints incrementally during optimization.
  - Optimization over QF_NRA, and Pareto fronts over real arithmetic (5.3–5.4).

## 4. APIs with no g3 equivalent

There are more than 180 new C API functions. The main groups:

- **User propagator** (4.8.9 onward): the `Z3_solver_propagate_*` family, covering fixed, eq/diseq, created, decide, final, on_binding and consequence callbacks.
- **Proofs and logging**: `Z3_solver_register_on_clause` (clause logging), proof trimming and checking, `Z3_set_proof_check_callback`, and newer Lean and F* proof reconstruction.
- **Solver introspection**:
  - `Z3_solver_cube`, `Z3_solver_get_units`, `Z3_solver_get_non_units`, `Z3_solver_get_levels`, `Z3_solver_get_trail`
  - E-graph access: `Z3_solver_congruence_root`, `Z3_solver_congruence_next`, `Z3_solver_congruence_explain`
  - `Z3_solver_solve_for`, `Z3_solver_to_dimacs_string`
- **Simplifiers**: the `Z3_simplifier_*` family and `Z3_solver_add_simplifier`.
- **Parser and terms**: incremental parsing (`Z3_mk_parser_context` and related functions), `Z3_substitute_funs`, `Z3_get_depth`, `Z3_is_ground`.
- **Optimization**: `Z3_optimize_assert_and_track`, `Z3_optimize_get_unsat_core`, `Z3_optimize_register_model_eh`, `Z3_optimize_set_initial_value`.
- **Other**: an expanded real-closed-field API, `Z3_qe_model_project_with_witness`, `Z3_enable_concurrent_dec_ref`.
- **Removed upstream**: `Z3_apply_result_convert_model`, `Z3_fixedpoint_push`/`Z3_fixedpoint_pop` and `Z3_get_error_msg_ex` exist in 4.7.1 but are gone now. `Z3_bool` was dropped in 4.11.

**If you need modern features from Go:** Z3 has shipped official Go bindings since 4.16.0. They use cgo. If you can accept cgo and do not need Ivy's interpolation extensions, they are a supported alternative. g3's distinguishing features are that it is pure Go and that it keeps the Ivy fork's interpolation support.

## 5. Soundness fixes

Z3 4.7.1 predates years of fuzzing-driven bug fixes. Upstream release notes list many fixed cases of unsound `sat`/`unsat` results, in nonlinear arithmetic, floating point, sequences, arrays, quantifier elimination and recursive functions. A faithful port inherits whichever of those bugs existed in 4.7.1.

## Limitations of this list

The release notes for the 4.8.x series are brief, so sections 1–3 are not exhaustive for that period. The API list in section 4 comes from a header diff and is complete for the C API. `Z3_ast_vector_*` and `Z3_ast_map_*` were left out because they only looked new: 4.7.1 kept them in a separate header.

## Sources

- [Z3 RELEASE_NOTES.md](https://github.com/Z3Prover/z3/blob/master/RELEASE_NOTES.md)
- [Z3 4.7.1 tag](https://github.com/Z3Prover/z3/tree/z3-4.7.1)
