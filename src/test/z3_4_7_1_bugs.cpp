/*++
Module Name:

    z3_4_7_1_bugs.cpp

Abstract:

    Regression tests for wrong answers in z3 4.7.1 that the Go port (g3)
    found. Each test builds the problem with the internal API and checks the
    CORRECT answer, so it fails on an unpatched 4.7.1 tree.

    z3_4_7_1_dom_simplify_root    dom-simplify re-simplifies the dominator
                                  tree root under its own assumption
                                  (pkg/AUDIT.md FUZZ_B-F6).
    z3_4_7_1_elim_term_ite_pop    elim_term_ite's rewriter cache survives pop
                                  (pkg/AUDIT.md smtcore_01-F1).
    z3_4_7_1_macro_finder_off     a macro found while smt.macro_finder was on
                                  is not expanded after it is turned off
                                  (pkg/AUDIT.md smtcore_01-F2).
    z3_4_7_1_pb_preprocess_index  pb-preprocess records m_ge positions as
                                  goal indices (pkg/AUDIT.md FUZZ_C-F2).
    z3_4_7_1_pb_preprocess_resolve pb-preprocess resolve: min_index is not set
                                  for the first complement, and a unit clause
                                  is deleted when no complement is found.
    z3_4_7_1_dt_recognizer        with smt.relevancy=1 a recognizer assigned
                                  false before it is relevant finds an empty
                                  recognizer table and is turned into a
                                  conflict (pkg/AUDIT.md smtcore_10-F3,
                                  upstream issue #4623).
    z3_4_7_1_dom_simplify_oscillation dom-simplify's simplify_goal loops
                                  forever when its forward and backward
                                  passes keep changing the goal.
    z3_4_7_1_update_field_constructor ((_ update-field fst) (mkp i red) j)
                                  is rewritten to (mkp i red): the rewriter
                                  looks up the update-field declaration as
                                  if it were the accessor.
    z3_4_7_1_pb2bv_numeral_lhs    pb2bv encodes (= 3 x) and (<= 3 x) as the
                                  unit x when the numeral is on the left
                                  (pkg/AUDIT.md tactic_arith_03-F1).
    z3_4_7_1_eq2bv_stale_cache    eq2bv reuses its rewriter cache across the
                                  goals of one run (pkg/AUDIT.md
                                  tactic_arith_01-F3).
    z3_4_7_1_purify_trig_unsafe   purify-arith decides which sin/cos
                                  arguments are safe to replace after the
                                  formulas that mention them were already
                                  purified (pkg/AUDIT.md tactic_arith_02-F2).
    z3_4_7_1_ufbv_demod_unbound_var the UFBV demodulator leaves a variable
                                  of an identical non-ground subterm unbound
                                  and later binds it to a ground term
                                  (pkg/AUDIT.md tactic_sls_ufbv_01-F1).
    z3_4_7_1_recover01_inconsistent recover-01 keeps indexing the goal after
                                  an update collapsed it to false
                                  (pkg/AUDIT.md tactic_arith_03-F9).
    z3_4_7_1_distribute_forall_proof distribute-forall gives no proof for
                                  its rewrite when proofs are on
                                  (pkg/AUDIT.md tactic_core_01-F5).
    z3_4_7_1_pb2bv_fractional     the pb2bv rewriter accepts fractional
                                  coefficients and bounds and builds a wrong
                                  circuit (pkg/AUDIT.md ast_rewriter_04-F3).
    z3_4_7_1_lia2card_empty_eq    lia2card loops forever on an equation
                                  between numerals such as (= 2 3)
                                  (pkg/AUDIT.md tactic_arith_02, Needs
                                  verification).
    z3_4_7_1_pdr_nullary_cache    PDR's model_search cache keys nodes by
                                  state only, so two nullary predicates
                                  (state `true`) share an entry and an
                                  unreachable query is answered sat
                                  (pkg/AUDIT.md muz_pdr_01).
    z3_4_7_1_duality_unhoist_repeated_child Duality's UnhoistLoop
                                  rewrites an edge once per occurrence of
                                  the unhoisted predicate and fails with a
                                  wrong number of arguments
                                  (pkg/AUDIT.md duality_03-F2).
    z3_4_7_1_lazy_sparse_nullary_union a union into an empty nullary
                                  register creates a 0-column sparse table
                                  through the lazy_sparse plugin and crashes
                                  (pkg/AUDIT.md muz_rel_04-F2).
    z3_4_7_1_qbmc_eval_level      QBMC proof reconstruction reads rule
                                  variables at the rule ordinal and expects
                                  facts only at level 0; it aborts in
                                  VERIFY(unify_rules) (pkg/AUDIT.md
                                  muz_small_01-F8, upstream 784b613b5).
    z3_4_7_1_ddnf_extract_width   DDNF compile_eq looks an extract equality
                                  up by the constant's width and segfaults
                                  (pkg/AUDIT.md muz_small_01-F9).
    z3_4_7_1_min_cut_shared_node  min_cut::augment_path adds the residual
                                  reverse arc to the sink and never
                                  terminates when augmenting paths share a
                                  node (pkg/AUDIT.md util_misc_01-F1).
    z3_4_7_1_qi_weakening_proof   quantifier instantiation builds its
                                  weakening proof with the modus-ponens
                                  premises swapped (pkg/AUDIT.md
                                  muz_transforms_01-F6).
    z3_4_7_1_duality_quantify_arrays_requery with xform.quantify_arrays
                                  a Duality query leaves the transformed
                                  rules in the context, so later queries
                                  answer wrongly (pkg/AUDIT.md muz_fp_03).
    z3_4_7_1_duality_failed_query_requery a Duality query after one that
                                  threw dereferences the null old solver
                                  and segfaults.
    z3_4_7_1_duality_underapprox_flag_literals with use_underapprox,
                                  FixCurrentState fixes the @under flags and
                                  the next ExpansionChoices check throws
                                  (pkg/AUDIT.md duality_02-F2).
    z3_4_7_1_duality_underapprox_stale_proof with use_underapprox,
                                  DerivationTree::Build interpolates the proof
                                  of the leaf-assumption check
                                  (pkg/AUDIT.md duality_01-F6).
    z3_4_7_1_duality_certificate_eliminated_var a certificate prints the
                                  model default for a rule variable qe-lite
                                  eliminated (pkg/AUDIT.md duality_01-F4).
    z3_4_7_1_implicant_nested_array_eq once a nested array index makes
                                  model_implicant::eval_array_eq mark the
                                  equality true, a later differing index
                                  cannot make it false (pkg/AUDIT.md
                                  model_01-F5).
    z3_4_7_1_pdr_proof_nullary_root the PDR proof trace cache is keyed by
                                  state only, so the query!0 root takes the
                                  proof of another node with state `true`
                                  (pkg/AUDIT.md muz_pdr_02-F2).
    z3_4_7_1_pdr_proof_duplicate_leaf the PDR proof trace caches a repeated,
                                  childless obligation with its unresolved
                                  rule, so the proof no longer concludes the
                                  query (pkg/AUDIT.md muz_pdr_02-F2).
    z3_4_7_1_pdr_boogie_without_proof_trace print_boogie_certificate without
                                  generate_proof_trace: the rules carry no
                                  proofs and the PDR proof trace segfaults
                                  (pkg/AUDIT.md muz_pdr_02-F2).
    z3_4_7_1_pdr_proof_merged_tails the PDR proof trace resolves a child whose
                                  tail remove_duplicate_tails merged away
                                  against a stale unifier (pkg/AUDIT.md
                                  muz_pdr_02-F2).
    z3_4_7_1_rewriter_stale_stacks an evaluation that exceeds max_steps leaves
                                  the rewriter's frame and result stacks
                                  behind and the next evaluation returns a
                                  stale result (pkg/AUDIT.md smtcore_14-F3).
    z3_4_7_1_qe_dl_other_value_neq the qe datalog plugin's "other value"
                                  branch substitutes true for negatively
                                  occurring equalities, so exists x. x != c
                                  over a finite sort becomes false
                                  (pkg/AUDIT.md qe_01-F4).
    z3_4_7_1_sieve_negation_inner_cols sieve_relation_plugin's negation
                                  filter inverts its inner-column test, so
                                  an equality between inner columns is
                                  dropped and the target is emptied
                                  (pkg/AUDIT.md muz_rel_06-F5).
    z3_4_7_1_check_table_nullary_empty with default_table_checked, an empty
                                  check_table with no columns is built by
                                  the sparse plugin and z3 segfaults.
    z3_4_7_1_bitvector_table_nullary with default_table=bitvector, a
                                  bitvector_table with no columns reserves
                                  no bits and add_fact segfaults.
    z3_4_7_1_case_split6_phase    with smt.case_split=6, context::decide
                                  reads an uninitialized phase and ignores
                                  smt.phase_selection (pkg/AUDIT.md
                                  smtcore_02-F9).
    z3_4_7_1_pb_simplex_nested    with smt.pb.enable_simplex, a pb atom's
                                  bool var is both its 0-1 column and its
                                  row's slack column, so a pb constraint
                                  over another pb atom bounds that atom's
                                  weighted sum by [0,1]: sat nested
                                  constraints are answered unsat, or z3
                                  segfaults (follow-up to pkg/AUDIT.md
                                  smtcore_12-F4).
    z3_4_7_1_pb_simplex_pop       with smt.pb.enable_simplex, a bool var
                                  freed by pop keeps its empty pb watch
                                  list; reused as a pb atom, its
                                  assignment bounds its own slack column as
                                  a 0-1 literal and z3 answers unsat on a
                                  sat problem (follow-up to pkg/AUDIT.md
                                  smtcore_12-F4).
    z3_4_7_1_pb_eq_no_args        a pb equality whose arguments are all
                                  fixed at internalization keeps no
                                  arguments and a negative bound;
                                  normalize leaves it undecided, nothing
                                  ever propagates it, and z3 answers sat
                                  on an unsat problem.
    z3_4_7_1_pb_assert_ge_uninit  theory_pb::assert_ge (the sorting-network
                                  entry point of tst_theory_pb) builds a
                                  theory_pb that is never attached to the
                                  context; its first clause reads the null
                                  manager and z3 segfaults.
    z3_4_7_1_solver_updt_params   smt_solver::updt_params re-reads
                                  smt_params from the new parameters alone,
                                  so Z3_get_implied_equalities' produce_models
                                  turns a solver-level smt.mbqi=false back on
                                  (pkg/AUDIT.md smtcore_03-F10 follow-up).
    z3_4_7_1_get_file_names_posix datalog::get_file_names is
                                  NOT_IMPLEMENTED_YET outside _WINDOWS, so
                                  z3 -dl DIRECTORY (the WPA benchmark
                                  loader) prints NOT IMPLEMENTED YET! and
                                  hangs (pkg/AUDIT.md cmd_context_02-F11).
    z3_4_7_1_wpa_warning_number   the WPA loader's "does not belong to sort"
                                  warning formats its uint64_t with MSVC's
                                  %I64u, which glibc prints as a 64-wide %u
                                  of a 32-bit argument (pkg/AUDIT.md
                                  cmd_context_02-F11).

    Run: ./test-z3 z3_4_7_1_dom_simplify_root   (etc.)
--*/

#include "ast/reg_decl_plugins.h"
#include "ast/ast_pp.h"
#include "ast/arith_decl_plugin.h"
#include "ast/pb_decl_plugin.h"
#include "ast/datatype_decl_plugin.h"
#include "smt/smt_kernel.h"
#include "smt/smt_context.h"
#include "smt/theory_pb.h"
#include "smt/params/smt_params.h"
#include "tactic/tactic.h"
#include "tactic/goal.h"
#include "tactic/core/dom_simplify_tactic.h"
#include "tactic/core/pb_preprocess_tactic.h"
#include "api/z3.h"
#include "cmd_context/cmd_context.h"
#include "parsers/smt2/smt2parser.h"
#include "tactic/portfolio/smt_strategic_solver.h"
#include "smt/smt_solver.h"
#include <sstream>
#include "cmd_context/cmd_context.h"
#include "muz/fp/dl_cmds.h"
#include "parsers/smt2/smt2parser.h"
#include "tactic/portfolio/smt_strategic_solver.h"
#include "util/min_cut.h"
#include "ast/array_decl_plugin.h"
#include "muz/base/dl_context.h"
#include "muz/fp/dl_register_engine.h"
#include "muz/transforms/dl_mk_quantifier_instantiation.h"
#include "ast/scoped_proof.h"
#include <sstream>
#include "nlsat/nlsat_assignment.h"
#include "nlsat/nlsat_solver.h"
#include "nlsat/nlsat_explain.h"
#include "util/rlimit.h"
#include "opt/opt_context.h"
#include "opt/opt_parse.h"
#include <fstream>
#include <cstdio>
#include "api/z3_replayer.h"
#include "model/model_implicant.h"
#include "model/model.h"
#include "model/model_evaluator.h"
#include "qe/qe.h"
#include "ast/dl_decl_plugin.h"
#include "muz/rel/dl_sieve_relation.h"
#include "muz/rel/rel_context.h"
#include "ast/rewriter/expr_safe_replace.h"
#include "ast/rewriter/th_rewriter.h"
#include "muz/base/dl_util.h"
#include "muz/fp/datalog_parser.h"
#include "util/warning.h"
void register_z3_replayer_cmds(z3_replayer & in);
#include <iostream>
#ifndef _WINDOWS
#include <csignal>
#include <unistd.h>
#include <cstdlib>
#include <sys/stat.h>
#endif

namespace {

    char const* lbool_name(lbool r) {
        switch (r) {
        case l_true: return "sat";
        case l_false: return "unsat";
        default: return "unknown";
        }
    }

    void expect(char const* test, char const* what, lbool expected, lbool actual) {
        std::cout << test << ": " << what << ": expected " << lbool_name(expected)
                  << ", got " << lbool_name(actual) << "\n";
        ENSURE(expected == actual);
    }

    app* bool_const(ast_manager& m, char const* name) {
        return m.mk_const(symbol(name), m.mk_bool_sort());
    }

    // Apply tactic t to a goal holding fmls, then decide the result with the
    // smt kernel. The tactic preserves satisfiability, so this is the answer
    // (check-sat-using (then t smt)) must give.
    lbool check_after_tactic(ast_manager& m, tactic* t, expr_ref_vector const& fmls) {
        tactic_ref tr(t);
        goal_ref g = alloc(goal, m, false, true, false);
        for (expr* f : fmls) g->assert_expr(f);
        goal_ref_buffer result;
        model_converter_ref mc;
        proof_converter_ref pc;
        expr_dependency_ref core(m);
        exec(*tr, g, result, mc, pc, core);
        bool all_sat = true;
        for (unsigned ri = 0; ri < result.size(); ++ri) {
            goal* r = result[ri];
            std::cout << "  result goal:";
            for (unsigned i = 0; i < r->size(); ++i) std::cout << " " << mk_pp(r->form(i), m);
            std::cout << "\n";
            smt_params fp;
            smt::kernel k(m, fp);
            for (unsigned i = 0; i < r->size(); ++i) k.assert_expr(r->form(i));
            lbool res = r->inconsistent() ? l_false : k.check();
            if (res == l_undef) return l_undef;
            if (res == l_true) return l_true;
            all_sat = false;
        }
        return all_sat ? l_true : l_false;
    }

    lbool check_plain(ast_manager& m, expr_ref_vector const& fmls) {
        smt_params fp;
        smt::kernel k(m, fp);
        for (expr* f : fmls) k.assert_expr(f);
        return k.check();
    }
}

// (assert (= true (ite p1 p0 p1))) is satisfied by p0 = p1 = true, but
// dom-simplify rewrote it to (= true false).
void tst_z3_4_7_1_dom_simplify_root() {
    ast_manager m;
    reg_decl_plugins(m);
    expr_ref p0(bool_const(m, "p0"), m), p1(bool_const(m, "p1"), m);
    expr_ref_vector fmls(m);
    fmls.push_back(m.mk_eq(m.mk_true(), m.mk_ite(p1, p0, p1)));
    expect("dom_simplify_root", "plain check", l_true, check_plain(m, fmls));
    expect("dom_simplify_root", "after dom-simplify", l_true,
           check_after_tactic(m, mk_dom_simplify_tactic(m), fmls));
}

// QF_LRA, after (pop) the cached elim_term_ite name for (ite c a b) has no
// definition any more, so x + 2*(ite c a b) < 1 with a = b = 1, x = 0 is
// reported sat.
void tst_z3_4_7_1_elim_term_ite_pop() {
    ast_manager m;
    reg_decl_plugins(m);
    arith_util a(m);
    sort* real = a.mk_real();
    expr_ref av(m.mk_const(symbol("a"), real), m);
    expr_ref bv(m.mk_const(symbol("b"), real), m);
    expr_ref xv(m.mk_const(symbol("x"), real), m);
    expr_ref cv(bool_const(m, "c"), m);
    expr_ref two(a.mk_numeral(rational(2), false), m);
    expr_ref sum(a.mk_add(xv, a.mk_mul(two, m.mk_ite(cv, av, bv))), m);

    smt_params fp;
    smt::kernel k(m, fp);
    VERIFY(k.set_logic(symbol("QF_LRA")));

    k.push();
    k.assert_expr(a.mk_gt(sum, a.mk_numeral(rational(0), false)));
    expect("elim_term_ite_pop", "first check", l_true, k.check());
    k.pop(1);

    k.push();
    k.assert_expr(m.mk_eq(av, a.mk_numeral(rational(1), false)));
    k.assert_expr(m.mk_eq(bv, a.mk_numeral(rational(1), false)));
    k.assert_expr(m.mk_eq(xv, a.mk_numeral(rational(0), false)));
    k.assert_expr(a.mk_lt(sum, a.mk_numeral(rational(1), false)));
    expect("elim_term_ite_pop", "check after pop", l_false, k.check());
    k.pop(1);
}

// (forall x. f(x) = x + 1) is turned into a macro while macro_finder is on.
// After macro_finder is turned off, (= (f 1) 5) is no longer expanded and
// the quantifier is gone, so z3 reports sat.
void tst_z3_4_7_1_macro_finder_off() {
    ast_manager m;
    reg_decl_plugins(m);
    arith_util a(m);
    sort* int_s = a.mk_int();
    func_decl_ref f(m.mk_func_decl(symbol("f"), int_s, int_s), m);
    symbol xn("x");
    expr_ref x(m.mk_var(0, int_s), m);
    expr_ref body(m.mk_eq(m.mk_app(f, x.get()), a.mk_add(x, a.mk_numeral(rational(1), true))), m);
    expr_ref q(m.mk_forall(1, &int_s, &xn, body), m);

    smt_params fp;
    fp.m_macro_finder = true;
    smt::kernel k(m, fp);

    k.push();
    k.assert_expr(q);
    expect("macro_finder_off", "first check", l_true, k.check());

    params_ref p;
    p.set_bool("macro_finder", false);
    // As smt_solver::updt_params does for (set-option :smt.macro_finder false).
    fp.updt_params(p);
    k.updt_params(p);
    ENSURE(!fp.m_macro_finder);

    expr_ref one(a.mk_numeral(rational(1), true), m);
    k.assert_expr(m.mk_eq(m.mk_app(f, one.get()), a.mk_numeral(rational(5), true)));
    expect("macro_finder_off", "check after macro_finder off", l_false, k.check());
    k.pop(1);
}

// Goal: p1, x > 0, p0, (not p1). m_ge = [0, 2, 3], but classify_vars records
// the positions 0, 1, 2, so the occurrence of (not p1) is attributed to goal
// formula 2 (p0) and the contradiction is resolved away.
void tst_z3_4_7_1_pb_preprocess_index() {
    ast_manager m;
    reg_decl_plugins(m);
    arith_util a(m);
    expr_ref p0(bool_const(m, "p0"), m), p1(bool_const(m, "p1"), m);
    expr_ref x(m.mk_const(symbol("x"), a.mk_int()), m);
    expr_ref_vector fmls(m);
    fmls.push_back(p1);
    fmls.push_back(a.mk_gt(x, a.mk_numeral(rational(0), true)));
    fmls.push_back(p0);
    fmls.push_back(m.mk_not(p1));
    expect("pb_preprocess_index", "plain check", l_false, check_plain(m, fmls));
    expect("pb_preprocess_index", "after pb-preprocess", l_false,
           check_after_tactic(m, mk_pb_preprocess_tactic(m), fmls));
}

void tst_z3_4_7_1_pb_preprocess_resolve() {
    ast_manager m;
    reg_decl_plugins(m);
    pb_util pb(m);
    expr_ref p0(bool_const(m, "p0"), m), p1(bool_const(m, "p1"), m);
    expr_ref p2(bool_const(m, "p2"), m), p3(bool_const(m, "p3"), m);
    {
        // (or (not p0) (not p1)), p1, p0: resolving p1 picks min_index 0
        // although the complement (not p1) is argument 1.
        expr_ref_vector fmls(m);
        fmls.push_back(m.mk_or(m.mk_not(p0), m.mk_not(p1)));
        fmls.push_back(p1);
        fmls.push_back(p0);
        expect("pb_preprocess_resolve", "min_index plain check", l_false, check_plain(m, fmls));
        expect("pb_preprocess_resolve", "min_index after pb-preprocess", l_false,
               check_after_tactic(m, mk_pb_preprocess_tactic(m), fmls));
    }
    {
        // (or (not p1) p2), ((_ pbge 3 1 1 1) (not p0) (not p3) p2), p3:
        // eliminating p0 splits formula 1 into p2 and (not p3); the stale
        // position then points at p2, no complement of p3 is found, and the
        // unit clause p3 is deleted anyway.
        expr_ref_vector fmls(m);
        fmls.push_back(m.mk_or(m.mk_not(p1), p2));
        rational cs[3] = { rational(1), rational(1), rational(1) };
        expr* args[3] = { m.mk_not(p0), m.mk_not(p3), p2 };
        fmls.push_back(pb.mk_ge(3, cs, args, rational(3)));
        fmls.push_back(p3);
        expect("pb_preprocess_resolve", "no-complement plain check", l_false, check_plain(m, fmls));
        expect("pb_preprocess_resolve", "no-complement after pb-preprocess", l_false,
               check_after_tactic(m, mk_pb_preprocess_tactic(m), fmls));
    }
}

// (declare-datatypes () ((L nil (cons (hd Int) (tl L)))))
// (assert (or (not b) (is-cons l))) with smt.relevancy=1: b = false is a
// model, but assign_eh reaches propagate_recognizer for (is-cons l) = false
// before relevant_eh has registered the recognizer, the loop sees no
// recognizers, and the empty conflict makes z3 report unsat.
void tst_z3_4_7_1_dt_recognizer() {
    ast_manager m;
    reg_decl_plugins(m);
    arith_util a(m);
    datatype_util dtutil(m);
    datatype_decl_plugin & dt = *(static_cast<datatype_decl_plugin*>(m.get_plugin(m.get_family_id("datatype"))));
    accessor_decl* hd_tl[2] = {
        mk_accessor_decl(m, symbol("hd"), type_ref(a.mk_int())),
        mk_accessor_decl(m, symbol("tl"), type_ref(0))
    };
    constructor_decl* constrs[2] = {
        mk_constructor_decl(symbol("nil"), symbol("is-nil"), 0, nullptr),
        mk_constructor_decl(symbol("cons"), symbol("is-cons"), 2, hd_tl)
    };
    datatype_decl * decl = mk_datatype_decl(dtutil, symbol("L"), 0, nullptr, 2, constrs);
    sort_ref_vector new_sorts(m);
    VERIFY(dt.mk_datatypes(1, &decl, 0, nullptr, new_sorts));
    sort* list = new_sorts.get(0);
    func_decl* cons = (*dtutil.get_datatype_constructors(list))[1];
    expr_ref l(m.mk_const(symbol("l"), list), m);
    expr_ref b(bool_const(m, "b"), m);
    expr_ref is_cons(m.mk_app(dtutil.get_constructor_recognizer(cons), l.get()), m);
    expr_ref fml(m.mk_or(m.mk_not(b), is_cons), m);

    for (unsigned relevancy = 0; relevancy <= 2; ++relevancy) {
        smt_params fp;
        fp.m_relevancy_lvl = relevancy;
        smt::kernel k(m, fp);
        k.assert_expr(fml);
        std::string what = "check with relevancy " + std::to_string(relevancy);
        expect("dt_recognizer", what.c_str(), l_true, k.check());
    }
}

#ifndef _WINDOWS
namespace {
    void dom_simplify_oscillation_timeout(int) {
        static char const msg[] =
            "dom_simplify_oscillation: dom-simplify did not terminate within 20 seconds\n";
        ssize_t n = write(2, msg, sizeof(msg) - 1);
        (void)n;
        _exit(1);
    }
}
#endif

// (not p3), (not (and p1 p1 p3)), (not true): simplify_goal's forward and
// backward passes keep rewriting the goal, and 4.7.1 loops while anything
// changed. Upstream 0ef8ebe89f caps the loop at 10 rounds. A watchdog turns
// the hang into a failure.
void tst_z3_4_7_1_dom_simplify_oscillation() {
#ifndef _WINDOWS
    signal(SIGALRM, dom_simplify_oscillation_timeout);
    alarm(20);
#endif
    ast_manager m;
    reg_decl_plugins(m);
    expr_ref p1(bool_const(m, "p1"), m), p3(bool_const(m, "p3"), m);
    expr_ref_vector fmls(m);
    fmls.push_back(m.mk_not(p3));
    expr* and_args[3] = { p1, p1, p3 };
    fmls.push_back(m.mk_not(m.mk_and(3, and_args)));
    fmls.push_back(m.mk_not(m.mk_true()));
    expect("dom_simplify_oscillation", "plain check", l_false, check_plain(m, fmls));
    std::cout.flush(); // the watchdog leaves with _exit
    expect("dom_simplify_oscillation", "after dom-simplify", l_false,
           check_after_tactic(m, mk_dom_simplify_tactic(m), fmls));
#ifndef _WINDOWS
    alarm(0);
#endif
}

// ((_ update-field fst) (mkp i red) j) must rewrite to (mkp j red).
// datatype_rewriter::mk_app_core passes the update-field declaration to
// get_accessor_constructor, gets no matching constructor and returns
// (mkp i red) unchanged, so the script below is answered sat (fixed
// upstream in f989e4eb38, "fix #2276").
void tst_z3_4_7_1_update_field_constructor() {
    char const* script =
        "(declare-datatypes () ((C red green blue)))\n"
        "(declare-datatypes () ((P (mkp (fst Int) (snd C)))))\n"
        "(declare-const p0 P)\n"
        "(declare-const i Int)\n"
        "(declare-const j Int)\n"
        "(assert (= ((_ update-field fst) (mkp i red) j) p0))\n"
        "(assert (not (= (fst p0) j)))\n"
        "(check-sat)\n"
        "(simplify ((_ update-field fst) (mkp i red) j))\n";
    Z3_config cfg = Z3_mk_config();
    Z3_context ctx = Z3_mk_context(cfg);
    Z3_del_config(cfg);
    std::string out = Z3_eval_smtlib2_string(ctx, script);
    Z3_del_context(ctx);
    std::string expected = "unsat\n(mkp j red)\n";
    std::cout << "update_field_constructor: expected\n" << expected << "got\n" << out;
    ENSURE(out == expected);
}

namespace {
    // Run an SMT-LIB2 script as the z3 shell does (with the Datalog/Horn
    // commands installed) and compare its whole output, errors included.
    void expect_script_output(char const* test, char const* script, char const* expected) {
        std::ostringstream out;
        {
            cmd_context ctx;
            ctx.set_solver_factory(mk_smt_strategic_solver_factory());
            install_dl_cmds(ctx);
            ctx.set_regular_stream(out);
            ctx.set_diagnostic_stream(out);
            std::istringstream in(script);
            parse_smt2_commands(ctx, in);
        }
        std::cout << test << ": expected\n" << expected << "got\n" << out.str();
        ENSURE(out.str() == expected);
    }
}

// PDR with two nullary body predicates: G :- B, A. A is reachable, B is not
// (D only takes even values, so D(3) never holds), so the answer is unsat.
// model_search keyed its node cache by state alone, and the state of every
// nullary predicate is `true`, so B's obligation was pre-closed as a
// duplicate of A's and the query was answered sat (with :print-answer the
// trace extraction then crashes). pkg/AUDIT.md muz_pdr_01.
void tst_z3_4_7_1_pdr_nullary_cache() {
    char const* script =
        "(set-option :fixedpoint.engine pdr)\n"
        "(set-option :fixedpoint.xform.slice false)\n"
        "(set-option :fixedpoint.xform.inline_linear false)\n"
        "(set-option :fixedpoint.xform.inline_eager false)\n"
        "(declare-rel A ())\n"
        "(declare-rel B ())\n"
        "(declare-rel C (Int))\n"
        "(declare-rel D (Int))\n"
        "(declare-rel G ())\n"
        "(declare-var x Int)\n"
        "(rule (C 0))\n"
        "(rule (=> (and (C x) (< x 3)) (C (+ x 1))))\n"
        "(rule (=> (C 3) A))\n"
        "(rule (D 0))\n"
        "(rule (=> (and (D x) (< x 3)) (D (+ x 2))))\n"
        "(rule (=> (D 3) B))\n"
        "(rule (=> (and B A) G))\n"
        "(query G)\n";
    expect_script_output("pdr_nullary_cache", script, "unsat\n");
}

// Duality unhoists the loop predicate inv, which occurs twice in the body
// of the err rule. Parent->Incoming lists that edge once per child
// position, so RPFP::UnhoistLoop rewrote it twice; the second pass added
// the new parameters to the already extended literal again and the query
// failed with "Wrong number of arguments (4) passed to function inv!2".
// err is reachable (k = -1 gives inv(0, -2) and 0 > -1), so the answer is
// sat. pkg/AUDIT.md duality_03-F2.
void tst_z3_4_7_1_duality_unhoist_repeated_child() {
    char const* script =
        "(set-option :fixedpoint.engine duality)\n"
        "(declare-rel inv (Int Int))\n"
        "(declare-rel err ())\n"
        "(declare-var x Int)\n"
        "(declare-var n Int)\n"
        "(declare-var k Int)\n"
        "(declare-var y Int)\n"
        "(declare-var m Int)\n"
        "(rule (=> (and (= x 0) (= n (* 2 k))) (inv x n)))\n"
        "(rule (=> (and (inv x n) (< x n)) (inv (+ x 1) n)))\n"
        "(rule (=> (and (inv x n) (inv y m) (> x (+ n 1))) err))\n"
        "(query err)\n";
    expect_script_output("duality_unhoist_repeated_child", script, "sat\n");
}

// With the lazy_sparse table, s(x) and p(x, y) are joined into the nullary
// z. The union into z's still-empty register creates the target with the
// source's plugin: lazy_table_plugin::mk_empty passes the 0-column
// signature to sparse_table_plugin::mk_empty, which cannot handle it and
// crashes in column_layout::make_byte_aligned_end. p(3, 5) and s(3) hold, so
// z is derivable. pkg/AUDIT.md muz_rel_04-F2.
void tst_z3_4_7_1_lazy_sparse_nullary_union() {
    char const* script =
        "(set-option :fixedpoint.engine datalog)\n"
        "(set-option :fixedpoint.datalog.default_table lazy_sparse)\n"
        "(define-sort D () (_ BitVec 3))\n"
        "(declare-rel q (D D D))\n"
        "(declare-rel p (D D))\n"
        "(declare-rel s (D))\n"
        "(declare-rel z ())\n"
        "(declare-var x D)\n"
        "(declare-var y D)\n"
        "(declare-var w D)\n"
        "(rule (q #b010 #b001 #b100))\n"
        "(rule (q #b011 #b001 #b101))\n"
        "(rule (q #b011 #b011 #b101))\n"
        "(rule (=> (and (q x #b001 y) (bvult x y)) (p x y)))\n"
        "(rule (=> (and (q x w y) (q w x y)) (s x)))\n"
        "(rule (=> (and (p x y) (s x)) z))\n"
        "(query z :print-answer true)\n";
    expect_script_output("lazy_sparse_nullary_union", script, "sat\ntrue\n");
}

// QBMC proof reconstruction (qlinear::get_model) read the rule variables at
// the rule ordinal instead of the trace level, and assumed that a fact is
// only selected at level 0. On this recursive program the step it builds
// does not unify with the previous one and z3 aborts in
// VERIFY(unifier.unify_rules(...)). Fixed upstream in 784b613b5 (#11540),
// which also puts the hyper-resolution positions in the (1, 0) order.
// pkg/AUDIT.md muz_small_01-F8.
void tst_z3_4_7_1_qbmc_eval_level() {
    char const* script =
        "(set-option :fixedpoint.engine qbmc)\n"
        "(set-option :fixedpoint.xform.inline_linear false)\n"
        "(set-option :fixedpoint.xform.inline_eager false)\n"
        "(set-option :fixedpoint.xform.slice false)\n"
        "(set-option :fixedpoint.print_answer true)\n"
        "(declare-rel p (Int))\n"
        "(declare-rel q ())\n"
        "(declare-var x Int)\n"
        "(declare-var y Int)\n"
        "(rule (p 0))\n"
        "(rule (=> (and (p x) (= y (+ x 1))) (p y)))\n"
        "(rule (=> (and (p x) (> x 2)) q))\n"
        "(query q)\n";
    std::ostringstream out;
    {
        cmd_context ctx;
        ctx.set_solver_factory(mk_smt_strategic_solver_factory());
        install_dl_cmds(ctx);
        ctx.set_regular_stream(out);
        ctx.set_diagnostic_stream(out);
        std::istringstream in(script);
        parse_smt2_commands(ctx, in);
    }
    // The derivation depends on the model, so check its shape only: sat,
    // and a proof that ends in the fact (p 0).
    std::cout << "qbmc_eval_level: got\n" << out.str();
    ENSURE(out.str().compare(0, 4, "sat\n") == 0);
    ENSURE(out.str().find("(asserted (p 0))") != std::string::npos);
}

// DDNF compile_eq looked the tbv of (= ((_ extract 0 0) x) #b1) up in the
// DDNF of the constant's width (1). process_eq stored it in the DDNF of x's
// width (4); no 1-bit DDNF exists and z3 segfaulted. pkg/AUDIT.md
// muz_small_01-F9. Still present upstream.
void tst_z3_4_7_1_ddnf_extract_width() {
    char const* script =
        "(set-option :fixedpoint.engine ddnf)\n"
        "(declare-rel p ((_ BitVec 4)))\n"
        "(declare-rel q ())\n"
        "(declare-var x (_ BitVec 4))\n"
        "(rule (=> (= x #x5) (p x)))\n"
        "(rule (=> (and (p x) (= ((_ extract 0 0) x) #b1)) q))\n"
        "(query q)\n";
    // The ddnf engine dumps the compiled rules to std::cout.
    std::ostringstream out;
    std::streambuf* saved = std::cout.rdbuf(out.rdbuf());
    {
        cmd_context ctx;
        ctx.set_solver_factory(mk_smt_strategic_solver_factory());
        install_dl_cmds(ctx);
        ctx.set_regular_stream(out);
        ctx.set_diagnostic_stream(out);
        std::istringstream in(script);
        parse_smt2_commands(ctx, in);
    }
    std::cout.rdbuf(saved);
    std::string expected =
        "(declare-rel q ())\n"
        "(declare-rel p ((_ BitVec 2)))\n"
        "(declare-var A (_ BitVec 2))\n"
        "(rule (=> (= A #b01) (p A)))\n"
        "(rule (=> (and (p A) (or (= A #b01) (= A #b10))) q))\n"
        "(query q)\n"
        "unknown\n";
    std::cout << "ddnf_extract_width: expected\n" << expected << "got\n" << out.str();
    ENSURE(out.str() == expected);
}

#ifndef _WINDOWS
namespace {
    void min_cut_shared_node_timeout(int) {
        static char const msg[] =
            "min_cut_shared_node: compute_min_cut did not terminate within 20 seconds\n";
        ssize_t n = write(2, msg, sizeof(msg) - 1);
        (void)n;
        _exit(1);
    }
}
#endif

// Source 0, sink 1, and the diamond 0->2, 0->3, 2->4, 3->4, 4->1: both
// augmenting paths share node 4. min_cut::augment_path added the missing
// residual reverse arc as k -> 1 (the sink) instead of k -> l, which keeps
// refilling capacity into the sink, so compute_min_cut never terminated.
// The cut is the single edge 4->1, reported as node 1. Used by spacer with
// fixedpoint.spacer.minimize_unsat_core=true. Fixed upstream in 8150cd6e8
// (#11401). pkg/AUDIT.md util_misc_01-F1.
void tst_z3_4_7_1_min_cut_shared_node() {
#ifndef _WINDOWS
    signal(SIGALRM, min_cut_shared_node_timeout);
    alarm(20);
#endif
    min_cut mc;
    unsigned a = mc.new_node(), b = mc.new_node(), c = mc.new_node();
    mc.add_edge(0, a);
    mc.add_edge(0, b);
    mc.add_edge(a, c);
    mc.add_edge(b, c);
    mc.add_edge(c, 1);
    unsigned_vector cut;
    mc.compute_min_cut(cut);
#ifndef _WINDOWS
    alarm(0);
#endif
    std::cout << "min_cut_shared_node: cut:";
    for (unsigned n : cut) std::cout << " " << n;
    std::cout << "\n";
    ENSURE(cut.size() == 1 && cut[0] == 1);
}

// With proof traces on, mk_quantifier_instantiation::instantiate_rule proves
// the instantiated rule from the original rule's proof p1 as a weakening. It
// built that proof without enabling proof mode, so when the manager's proof
// mode is off (as in a fixedpoint query) the rule got no proof and the rule
// inliner later dereferenced it. With proof mode on it passed the premises
// in the wrong order: mk_modus_ponens(def-axiom(fact(p1) => fml), p1) takes
// its conclusion from fact(p1), here a forall, through to_app(...)->get_arg(1).
// The proof must be mk_modus_ponens(p1, def-axiom(fact(p1) => fml)),
// concluding the new rule. pkg/AUDIT.md muz_transforms_01-F6. Still present
// upstream.
void tst_z3_4_7_1_qi_weakening_proof() {
    for (unsigned proofs_on = 0; proofs_on < 2; ++proofs_on) {
        ast_manager m;
        reg_decl_plugins(m);
        array_util au(m);
        datalog::register_engine re;
        smt_params fp;
        params_ref p;
        p.set_bool("generate_proof_trace", true);
        p.set_bool("xform.instantiate_quantifiers", true);
        datalog::context ctx(m, re, fp, p);
        sort* b = m.mk_bool_sort();
        sort* arr = au.mk_array_sort(b, b);
        func_decl_ref pd(m.mk_func_decl(symbol("p"), 0, static_cast<sort* const*>(nullptr), b), m);
        ctx.register_predicate(pd, false);
        // p :- (select a true) = false, forall y. (select a y) = false
        expr_ref a(m.mk_var(0, arr), m);
        expr_ref y(m.mk_var(0, b), m);
        expr_ref a1(m.mk_var(1, arr), m);
        expr* sel_y_args[2] = { a1, y };
        expr_ref body(m.mk_eq(au.mk_select(2, sel_y_args), m.mk_false()), m);
        symbol yn("y");
        expr_ref all(m.mk_forall(1, &b, &yn, body), m);
        expr* sel_t_args[2] = { a, m.mk_true() };
        expr_ref tail(m.mk_and(m.mk_eq(au.mk_select(2, sel_t_args), m.mk_false()), all), m);
        expr_ref rule_fml(m.mk_implies(tail, m.mk_const(pd)), m);
        ctx.add_rule(rule_fml, symbol::null);
        datalog::rule_set& rules = ctx.get_rules();
        ENSURE(rules.get_num_rules() == 1 && rules.get_rule(0)->get_proof());
        proof* p1 = rules.get_rule(0)->get_proof();
        std::cout << "qi_weakening_proof: proof mode " << (proofs_on ? "on" : "off")
                  << ", source proof fact " << mk_pp(m.get_fact(p1), m) << "\n";
        scoped_proof_mode _spm(m, proofs_on ? PGM_ENABLED : PGM_DISABLED);
        datalog::mk_quantifier_instantiation qi(ctx, 0);
        scoped_ptr<datalog::rule_set> result = qi(rules);
        ENSURE(result && result->get_num_rules() > 0);
        for (unsigned i = 0; i < result->get_num_rules(); ++i) {
            datalog::rule* r = result->get_rule(i);
            proof* pr = r->get_proof();
            std::cout << "  instantiated rule has a proof: " << (pr != nullptr) << "\n";
            ENSURE(pr);
            expr_ref fml(m);
            ctx.get_rule_manager().to_formula(*r, fml);
            ENSURE(m.get_num_parents(pr) == 2);
            bool p1_first = m.get_parent(pr, 0) == p1;
            std::cout << "  first premise is the source proof: " << p1_first << "\n";
            ENSURE(p1_first);
            bool concludes_rule = m.get_fact(pr) == fml.get();
            std::cout << "  proof concludes the rule " << mk_pp(fml, m) << ": " << concludes_rule << "\n";
            ENSURE(concludes_rule);
        }
    }
}

namespace {
    // Run an SMT-LIB2 script through the API and compare its output.
    void expect_script(char const* test, char const* script, char const* expected) {
        Z3_config cfg = Z3_mk_config();
        Z3_context ctx = Z3_mk_context(cfg);
        Z3_del_config(cfg);
        std::string out = Z3_eval_smtlib2_string(ctx, script);
        Z3_del_context(ctx);
        std::cout << test << ": expected\n" << expected << "got\n" << out;
        ENSURE(out == expected);
    }
}

// x in [0,1], so (= 3 x) and (<= 3 x) are false and both checks are unsat.
// pb2bv_tactic::imp::convert handles a numeral on the left in its own
// branch, which turns (= 3 x) into the unit x and, with SASSERT compiled
// out, (<= 3 x) into x as well.
void tst_z3_4_7_1_pb2bv_numeral_lhs() {
    char const* script =
        "(declare-const x Int)\n"
        "(declare-const b Bool)\n"
        "(assert (<= 0 x))\n"
        "(assert (<= x 1))\n"
        "(assert (not b))\n"
        "(push)\n"
        "(assert (or b (= 3 x)))\n"
        "(check-sat-using (then pb2bv smt))\n"
        "(pop)\n"
        "(push)\n"
        "(assert (or b (<= 3 x)))\n"
        "(check-sat-using (then pb2bv smt))\n"
        "(pop)\n";
    expect_script("pb2bv_numeral_lhs", script, "unsat\nunsat\n");
}

// split-clause makes two goals that share (= x 3). eq2bv rewrites it to
// (= z!0 #b11) in the first goal; its member rewriter m_rw keeps the cache,
// so the second goal gets the stale z!0 next to its own z!1 bounds and the
// unsat problem is answered sat.
void tst_z3_4_7_1_eq2bv_stale_cache() {
    char const* script =
        "(declare-const x Int)\n"
        "(declare-const p Bool)\n"
        "(declare-const b Bool)\n"
        "(declare-const c Bool)\n"
        "(assert (or p (not p)))\n"
        "(assert (<= x 2))\n"
        "(assert (or (= x 3) b))\n"
        "(assert (or (= x 3) c))\n"
        "(assert (not b))\n"
        "(check-sat-using (then split-clause eq2bv smt))\n";
    expect_script("eq2bv_stale_cache", script, "unsat\n");
}

// x/y = 1, y = 10, sin x = 1/2 is unsat (sin 10 != 1/2). purify-arith runs
// find_unsafe lazily, on the goal whose first formula was already purified
// to (= k!0 1.0) etc., so it no longer sees x under the division, replaces
// (sin x) by a fresh basis and z3 4.7.1 answers sat with an x that violates
// x/y = 1. Any answer but sat is acceptable; after (apply purify-arith) the
// (sin x) must stay.
void tst_z3_4_7_1_purify_trig_unsafe() {
    char const* script =
        "(declare-const x Real)\n"
        "(declare-const y Real)\n"
        "(assert (= (/ x y) 1.0))\n"
        "(assert (= y 10.0))\n"
        "(assert (= (sin x) 0.5))\n"
        "(check-sat-using qfufnra)\n";
    Z3_config cfg = Z3_mk_config();
    Z3_context ctx = Z3_mk_context(cfg);
    Z3_del_config(cfg);
    std::string out = Z3_eval_smtlib2_string(ctx, script);
    std::cout << "purify_trig_unsafe: check-sat-using qfufnra: " << out;
    ENSURE(out != "sat\n");
    std::string goal = Z3_eval_smtlib2_string(ctx, "(apply purify-arith)\n");
    Z3_del_context(ctx);
    std::cout << "purify_trig_unsafe: purify-arith:\n" << goal;
    ENSURE(goal.find("(sin x)") != std::string::npos);
}

// The demodulator f(x, g(x)) = x + 1 must not rewrite f(c, g(y)) under
// forall y: g(x) and g(y) are the same AST, match_args_aux_proc leaves x
// unbound, and the pair (x, c) then binds x := c, which drops the
// quantifier. With y := d the second assertion contradicts the third, so
// the problem is unsat; z3 4.7.1 answers sat (fixed upstream in b76ed6a47,
// "proper fix to #6476").
void tst_z3_4_7_1_ufbv_demod_unbound_var() {
    char const* script =
        "(set-logic UFBV)\n"
        "(declare-fun f ((_ BitVec 8) (_ BitVec 8)) (_ BitVec 8))\n"
        "(declare-fun g ((_ BitVec 8)) (_ BitVec 8))\n"
        "(declare-const c (_ BitVec 8))\n"
        "(declare-const d (_ BitVec 8))\n"
        "(assert (forall ((x (_ BitVec 8))) (= (f x (g x)) (bvadd x #x01))))\n"
        "(assert (forall ((y (_ BitVec 8))) (bvule (f c (g y)) #x05)))\n"
        "(assert (= (f c (g d)) #x07))\n"
        "(assert (= (f d (g c)) #x09))\n"
        "(check-sat)\n";
    // Run it as the z3 shell does (its own cmd_context and ast_manager): the
    // demodulator's choices depend on AST ids, and the API context's
    // manager does not reproduce the bad match.
    std::stringstream out;
    {
        cmd_context ctx;
        ctx.set_solver_factory(mk_smt_strategic_solver_factory());
        ctx.set_interpolating_solver_factory(mk_smt_solver_factory());
        ctx.set_regular_stream(out);
        ctx.set_diagnostic_stream(out);
        std::istringstream in(script);
        VERIFY(parse_smt2_commands(ctx, in));
    }
    std::cout << "ufbv_demod_unbound_var: expected\nunsat\ngot\n" << out.str();
    ENSURE(out.str() == "unsat\n");
}

// recover-01 rewrites (or p (= x 0)) to false; goal::update then shrinks the
// goal to the single formula false, but the rewrite loop keeps reading
// form(idx) up to the old size and z3 4.7.1 crashes (fixed upstream in
// c230d89a3, "fix #5294").
void tst_z3_4_7_1_recover01_inconsistent() {
    char const* script =
        "(declare-const x Int)\n"
        "(declare-const p Bool)\n"
        "(declare-const b Bool)\n"
        "(assert (= p (not p)))\n"
        "(assert b)\n"
        "(assert (> x 0))\n"
        "(assert (or p (= x 0)))\n"
        "(assert (or (not p) (= x 3)))\n"
        "(apply recover-01)\n";
    std::cout << "recover01_inconsistent: running (apply recover-01)\n";
    std::cout.flush(); // z3 4.7.1 dies here
    expect_script("recover01_inconsistent", script,
                  "(goals\n(goal\n  false\n  :precision precise :depth 1)\n)\n");
}

namespace {
    // Run an SMT-LIB2 script as the z3 shell does, with its own cmd_context
    // and ast_manager, so options such as :produce-proofs can still be set.
    void expect_shell_script(char const* test, char const* script, char const* expected) {
        std::stringstream out;
        {
            cmd_context ctx;
            ctx.set_solver_factory(mk_smt_strategic_solver_factory());
            ctx.set_interpolating_solver_factory(mk_smt_solver_factory());
            ctx.set_regular_stream(out);
            ctx.set_diagnostic_stream(out);
            std::istringstream in(script);
            parse_smt2_commands(ctx, in);
        }
        std::cout << test << ": expected\n" << expected << "got\n" << out.str();
        ENSURE(out.str() == expected);
    }
}

// With proofs on, distribute_forall_tactic::rw_cfg::reduce_quantifier sets
// result but not result_pr, and z3 4.7.1 crashes building the goal's
// proof (fixed upstream in d530d1314, "fix #3276").
void tst_z3_4_7_1_distribute_forall_proof() {
    char const* script =
        "(set-option :produce-proofs true)\n"
        "(declare-fun p (Int) Bool)\n"
        "(declare-fun q (Int) Bool)\n"
        "(assert (forall ((x Int) (y Int)) (and (p x) (q y))))\n"
        "(apply distribute-forall)\n";
    std::cout << "distribute_forall_proof: running (apply distribute-forall)\n";
    std::cout.flush(); // z3 4.7.1 dies here
    expect_shell_script("distribute_forall_proof", script,
                  "(goals\n(goal\n  (forall ((x Int)) (p x))\n  (forall ((y Int)) (q y))\n"
                  "  :precision precise :depth 1)\n)\n");
}

// 0.5a + 0.5b <= 0.7 forbids a = b = true. pb2bv_rewriter::imp's is_pb
// accepts the fractional coefficients and bound, and mk_le_ge builds a
// bit-vector circuit from them that is always true, so card2bv turns the
// unsat problem into a sat one (fixed upstream in 10768bd00, "fix #3727",
// and f7a7b9e1f, "fix #4108").
void tst_z3_4_7_1_pb2bv_fractional() {
    char const* script =
        "(declare-const a Bool)\n"
        "(declare-const b Bool)\n"
        "(assert (<= (+ (ite a 0.5 0.0) (ite b 0.5 0.0)) 0.7))\n"
        "(assert a)\n"
        "(assert b)\n"
        "(check-sat-using (then card2bv smt))\n";
    expect_script("pb2bv_fractional", script, "unsat\n");
}

#ifndef _WINDOWS
namespace {
    void lia2card_empty_eq_timeout(int) {
        static char const msg[] =
            "lia2card_empty_eq: lia2card did not terminate within 20 seconds\n";
        ssize_t n = write(2, msg, sizeof(msg) - 1);
        (void)n;
        _exit(1);
    }
}
#endif

// lia2card rewrites (= 2 3) to (and false true). goal::update flattens that
// to false but then pushes the unflattened (and false true) as a new
// formula, so the goal stays consistent and grows by one formula that the
// rewrite loop visits, rewrites and pushes again, forever. A watchdog turns
// the hang into a failure.
void tst_z3_4_7_1_lia2card_empty_eq() {
#ifndef _WINDOWS
    signal(SIGALRM, lia2card_empty_eq_timeout);
    alarm(20);
#endif
    std::cout << "lia2card_empty_eq: running (apply lia2card)\n";
    std::cout.flush(); // the watchdog leaves with _exit
    expect_script("lia2card_empty_eq", "(assert (= 2 3))\n(apply lia2card)\n",
                  "(goals\n(goal\n  false\n  :precision precise :depth 1)\n)\n");
#ifndef _WINDOWS
    alarm(0);
#endif
}

namespace {
    void nlsat_set_value(nlsat::solver& s, nlsat::assignment& as, nlsat::var x, int num, int den = 1) {
        anum_manager& am = s.am();
        unsynch_mpq_manager& qm = s.qm();
        scoped_mpq q(qm);
        qm.set(q, num, den);
        scoped_anum v(am);
        am.set(v, q);
        as.set(x, v);
    }

    nlsat::literal nlsat_lit(nlsat::solver& s, nlsat::atom::kind k, polynomial_ref const& p) {
        nlsat::poly* ps[1] = { p.get() };
        bool is_even[1] = { false };
        return s.mk_ineq_literal(k, 1, ps, is_even);
    }

    // Number of literals in ls that are not true in the solver's current
    // assignment.
    unsigned nlsat_num_not_true(nlsat::solver& s, unsigned n, nlsat::literal const* ls) {
        unsigned r = 0;
        for (unsigned i = 0; i < n; ++i)
            if (s.value(ls[i]) != l_true)
                ++r;
        return r;
    }

    // Signed projection of lits onto x. Every literal of the projection must
    // hold in the current model, which lits satisfy.
    void nlsat_signed_project_check(char const* test, nlsat::solver& s, nlsat::var x,
                                    nlsat::scoped_literal_vector const& lits,
                                    nlsat::scoped_literal_vector& result) {
        ENSURE(nlsat_num_not_true(s, lits.size(), lits.c_ptr()) == 0);
        nlsat::explain& ex = s.get_explain();
        ex.set_signed_project(true);
        ex.project(x, lits.size(), lits.c_ptr(), result);
        s.display(std::cout << test << ": ", lits.size(), lits.c_ptr());
        s.display(std::cout << "\n  ==> ", result.size(), result.c_ptr());
        unsigned bad = nlsat_num_not_true(s, result.size(), result.c_ptr());
        std::cout << "\n  projection literals false in the model: " << bad << "\n";
        ENSURE(bad == 0);
    }
}

// x0 = 2 and x1 above (plus infinity) or below (minus infinity) every root
// of p = (x0 - 5)*x1 + 1 and q = (x0 - 5)*x1 + 3 (resp. x0*x1 - 9 and
// x0*x1 - 10), all of them negative in the model.
// explain::project_plus_infinity and project_minus_infinity store sign(p)
// in an unsigned, so s > 0 always holds and they add x0 - 5 > 0 for the
// plus case and x0 < 0 for the minus case, both false at x0 = 2 (fixed upstream in 363d7aad2, "fix bug reported in
// #1637"). In nlqsat this makes
// (forall a. exists x. a*x < -1 and (a - 2)*x > 3) loop forever.
void tst_z3_4_7_1_nlsat_signed_infinity() {
    params_ref ps;
    reslimit rlim;
    nlsat::solver s(rlim, ps, false);
    nlsat::pmanager& pm = s.pm();
    nlsat::var x0 = s.mk_var(false);
    nlsat::var x1 = s.mk_var(false);
    polynomial_ref _x0(pm), _x1(pm), p(pm), q(pm);
    _x0 = pm.mk_polynomial(x0);
    _x1 = pm.mk_polynomial(x1);
    {
        nlsat::assignment as(s.am());
        nlsat_set_value(s, as, x0, 2);
        nlsat_set_value(s, as, x1, 6);
        s.set_rvalues(as);
        p = ((_x0 + (-5)) * _x1) + 1;
        q = ((_x0 + (-5)) * _x1) + 3;
        nlsat::scoped_literal_vector lits(s), result(s);
        lits.push_back(nlsat_lit(s, nlsat::atom::LT, p));
        lits.push_back(nlsat_lit(s, nlsat::atom::LT, q));
        nlsat_signed_project_check("nlsat_signed_infinity plus", s, x1, lits, result);
    }
    {
        nlsat::assignment as(s.am());
        nlsat_set_value(s, as, x0, 2);
        nlsat_set_value(s, as, x1, 4);
        s.set_rvalues(as);
        p = (_x0 * _x1) + (-9);
        q = (_x0 * _x1) + (-10);
        nlsat::scoped_literal_vector lits(s), result(s);
        lits.push_back(nlsat_lit(s, nlsat::atom::LT, p));
        lits.push_back(nlsat_lit(s, nlsat::atom::LT, q));
        nlsat_signed_project_check("nlsat_signed_infinity minus", s, x1, lits, result);
    }
}

// (x0)(x1) > 0 at x0 = x1 = 50000. evaluator::eval_ineq multiplies the
// values eval_sign returns, which are the raw numerators of the factor
// values, in a 32-bit int: 50000 * 50000 wraps to a negative number and the
// atom evaluates to false (fixed upstream in f79dccccf and 6071797ba, "fix
// #2238", which normalize eval_sign to -1, 0, 1).
void tst_z3_4_7_1_nlsat_eval_sign_overflow() {
    params_ref ps;
    reslimit rlim;
    nlsat::solver s(rlim, ps, false);
    nlsat::pmanager& pm = s.pm();
    nlsat::var x0 = s.mk_var(false);
    nlsat::var x1 = s.mk_var(false);
    polynomial_ref _x0(pm), _x1(pm);
    _x0 = pm.mk_polynomial(x0);
    _x1 = pm.mk_polynomial(x1);
    nlsat::poly* factors[2] = { _x0.get(), _x1.get() };
    bool is_even[2] = { false, false };
    nlsat::scoped_literal_vector lits(s);
    lits.push_back(s.mk_ineq_literal(nlsat::atom::GT, 2, factors, is_even));
    lits.push_back(s.mk_ineq_literal(nlsat::atom::LT, 2, factors, is_even));
    nlsat::assignment as(s.am());
    nlsat_set_value(s, as, x0, 50000);
    nlsat_set_value(s, as, x1, 50000);
    s.set_rvalues(as);
    lbool gt = s.value(lits[0]);
    lbool lt = s.value(lits[1]);
    s.display(std::cout << "nlsat_eval_sign_overflow: ", lits[0]);
    std::cout << " := " << gt << ", ";
    s.display(std::cout, lits[1]);
    std::cout << " := " << lt << " at x0 = x1 = 50000\n";
    ENSURE(gt == l_true);
    ENSURE(lt == l_false);
}

// x0 = 2, x1 = 1/2: eq = x0*x1 - 1 = 0, q1 = x1 + x0 > 0, q2 = x1^2 - 3 < 0.
// Signed projection onto x1 substitutes x1 = 1/x0 into q1 and q2.
// explain::solve_eq never resets its table qs of powers of A = x0, so for
// q2 (degree 2, after q1 of degree 1) it reads the powers 1, x0 of q1's
// block: it computes the constant -2 instead of 1 - 3*x0^2 and drops the
// condition 3*x0^2 - 1 > 0. The projection then holds at x0 = 1/5, where
// x1 = 5 is forced and q2 = 22 > 0 (fixed upstream in 19e2f8c9d, "fix
// #1694").
void tst_z3_4_7_1_nlsat_solve_eq_powers() {
    params_ref ps;
    reslimit rlim;
    nlsat::solver s(rlim, ps, false);
    nlsat::pmanager& pm = s.pm();
    nlsat::var x0 = s.mk_var(false);
    nlsat::var x1 = s.mk_var(false);
    polynomial_ref _x0(pm), _x1(pm), eq(pm), q1(pm), q2(pm);
    _x0 = pm.mk_polynomial(x0);
    _x1 = pm.mk_polynomial(x1);
    eq = (_x0 * _x1) + (-1);
    q1 = _x1 + _x0;
    q2 = (_x1 * _x1) + (-3);
    nlsat::assignment as(s.am());
    nlsat_set_value(s, as, x0, 2);
    nlsat_set_value(s, as, x1, 1, 2);
    s.set_rvalues(as);
    nlsat::scoped_literal_vector lits(s), result(s);
    lits.push_back(nlsat_lit(s, nlsat::atom::EQ, eq));
    lits.push_back(nlsat_lit(s, nlsat::atom::GT, q1));
    lits.push_back(nlsat_lit(s, nlsat::atom::LT, q2));
    nlsat_signed_project_check("nlsat_solve_eq_powers", s, x1, lits, result);
    // At x0 = 1/5 the literals have no solution in x1, so some literal of
    // the projection must be false there.
    nlsat::assignment as2(s.am());
    nlsat_set_value(s, as2, x0, 1, 5);
    s.set_rvalues(as2);
    unsigned bad = nlsat_num_not_true(s, result.size(), result.c_ptr());
    std::cout << "  projection literals false at x0 = 1/5: " << bad << "\n";
    ENSURE(bad > 0);
}

// Core {x1 < 0, 3*x1 + 2 = 0, x1 - x0 > 0} at x0 = 0. simplify rewrites
// x1 < 0 with the equation to the constant -2 < 0, but tests s == -1
// where sign(-2) returns -2: it builds -2 > 0, which is false, and drops
// the rest of the core as if the assumptions alone implied the conflict
// (normalize has the same s == -1 test). The lemma
// not(x1 < 0) or not(3*x1 + 2 = 0) or not(x1 - x0 > 0) or <explanation>
// is then false at x0 = -1, x1 = -2/3 (fixed upstream in 0f20175bd, "fix
// #2556, sign of of inequality is not restricted to -1, 0, 1").
void tst_z3_4_7_1_nlsat_simplify_core_sign() {
    params_ref ps;
    reslimit rlim;
    nlsat::solver s(rlim, ps, false);
    nlsat::pmanager& pm = s.pm();
    nlsat::var x0 = s.mk_var(false);
    nlsat::var x1 = s.mk_var(false);
    polynomial_ref _x0(pm), _x1(pm), eq(pm), d(pm);
    _x0 = pm.mk_polynomial(x0);
    _x1 = pm.mk_polynomial(x1);
    eq = (3 * _x1) + 2;
    d = _x1 - _x0;
    nlsat::scoped_literal_vector core(s), result(s);
    core.push_back(nlsat_lit(s, nlsat::atom::LT, _x1));
    core.push_back(nlsat_lit(s, nlsat::atom::EQ, eq));
    core.push_back(nlsat_lit(s, nlsat::atom::GT, d));
    nlsat::assignment as(s.am());
    nlsat_set_value(s, as, x0, 0);
    s.set_rvalues(as);
    nlsat::explain& ex = s.get_explain();
    ex.set_simplify_cores(true);
    ex(core.size(), core.c_ptr(), result);
    s.display(std::cout << "nlsat_simplify_core_sign: core ", core.size(), core.c_ptr());
    s.display(std::cout << "\n  explanation ", result.size(), result.c_ptr());
    // The lemma is (not core) or result; it must hold everywhere.
    nlsat::assignment as2(s.am());
    nlsat_set_value(s, as2, x0, -1);
    nlsat_set_value(s, as2, x1, -2, 3);
    s.set_rvalues(as2);
    ENSURE(nlsat_num_not_true(s, core.size(), core.c_ptr()) == 0);
    unsigned true_lits = result.size() - nlsat_num_not_true(s, result.size(), result.c_ptr());
    std::cout << "\n  explanation literals true at x0 = -1, x1 = -2/3: " << true_lits << "\n";
    ENSURE(true_lits > 0);
}

namespace {
    // Run an SMT-LIB script and return its output.
    std::string eval_smtlib2(char const* script) {
        Z3_config cfg = Z3_mk_config();
        Z3_context ctx = Z3_mk_context(cfg);
        Z3_del_config(cfg);
        std::string out = Z3_eval_smtlib2_string(ctx, script);
        Z3_del_context(ctx);
        return out;
    }

    // out must start with expected and, after that, contain no "false":
    // the scripts end with get-value of their assertions.
    void expect_valid_answer(char const* test, std::string const& out, char const* expected) {
        std::cout << test << ":\n" << out;
        ENSURE(out.compare(0, strlen(expected), expected) == 0);
        ENSURE(out.find("false") == std::string::npos);
    }
}

// x := y^2 is inlined. In x - y^2 > 0 the factor vanishes; substitute_var
// treats it as an empty product, mk_ineq_literal makes 1 > 0 or 1 < 0 of
// it, and update_clauses then calls mk_clause with no literals, which
// segfaults. In (or (> (- x (* y y)) 1) (> (* z z y) 1)) the first factor
// becomes the constant -1: the atom -1 > 0 over a constant polynomial is
// kept, and the model violates (> (* z z y) 1). Both are fixed upstream by
// the 2024 rewrite of the inline_vars simplifier (29e724f78, 8fe357f1f).
void tst_z3_4_7_1_nlsat_inline_vars() {
    expect_valid_answer("nlsat_inline_vars zero factor", eval_smtlib2(
        "(declare-const x Real)\n"
        "(declare-const y Real)\n"
        "(assert (= x (* y y)))\n"
        "(assert (> (- x (* y y)) 0))\n"
        "(check-sat-using (then simplify purify-arith (using-params nlsat :inline_vars true :reorder false)))\n"),
        "unsat\n");
    expect_valid_answer("nlsat_inline_vars constant atom", eval_smtlib2(
        "(declare-const x Real)\n"
        "(declare-const y Real)\n"
        "(declare-const z Real)\n"
        "(assert (= x (* y y)))\n"
        "(assert (or (> (- x (* y y)) 1) (> (* z z y) 1)))\n"
        "(check-sat-using (then simplify purify-arith (using-params nlsat :inline_vars true :reorder false)))\n"
        "(get-value ((= x (* y y)) (or (> (- x (* y y)) 1) (> (* z z y) 1))))\n"),
        "sat\n");
}

// With inline_vars the equation x*y + z^3 = 1 is solved for x, then check()
// still reorders the variables; search_check calls fix_patch, which writes
// x's value through the pre-reorder index into the reordered assignment, so
// the model violates both remaining assertions.
void tst_z3_4_7_1_nlsat_inline_reorder() {
    expect_valid_answer("nlsat_inline_reorder", eval_smtlib2(
        "(declare-const x Real)\n"
        "(declare-const y Real)\n"
        "(declare-const z Real)\n"
        "(assert (> y 0.0))\n"
        "(assert (= (+ (* x y) (* z z z)) 1.0))\n"
        "(assert (> (* x z) 2.0))\n"
        "(check-sat-using (using-params qfnra-nlsat :inline_vars true))\n"
        "(get-value ((> y 0.0) (= (+ (* x y) (* z z z)) 1.0) (> (* x z) 2.0)))\n"),
        "sat\n");
}

#ifndef _WINDOWS
namespace {
    void qe_project_real_timeout(int) {
        static char const msg[] =
            "qe_project_real_direction: qe-sat did not terminate within 20 seconds\n";
        ssize_t n = write(2, msg, sizeof(msg) - 1);
        (void)n;
        _exit(1);
    }
}
#endif

// x >= a + y, x < b + y, x < c + y: one non-strict lower bound and two
// strict upper bounds. arith_plugin::project_real chooses the lower family
// (1 + 0 < 0 + 2) from raw counts and encodes the branch for it, but
// assign and subst choose the family with get_bound_sizes, which doubles
// the non-strict counts of a real (2 + 0 < 0 + 2 is false), and read the
// branch as an upper bound. qe-sat with projection_mode false then never
// terminates. A watchdog turns the hang into a failure.
void tst_z3_4_7_1_qe_project_real_direction() {
#ifndef _WINDOWS
    signal(SIGALRM, qe_project_real_timeout);
    alarm(20);
#endif
    std::string out = eval_smtlib2(
        "(declare-const a Real)\n"
        "(declare-const b Real)\n"
        "(declare-const c Real)\n"
        "(assert (forall ((y Real)) (exists ((x Real)) (and (>= x (+ a y)) (< x (+ b y)) (< x (+ c y))))))\n"
        "(check-sat-using (using-params qe-sat :projection_mode false))\n");
#ifndef _WINDOWS
    alarm(0);
#endif
    std::cout << "qe_project_real_direction: " << out;
    ENSURE(out == "sat\n");
}

// exists x. (mod (+ x a) k) = r and bounds, with a = 0 and b = 10, is
// satisfiable for k = 0 (mod by 0 is uninterpreted), for k = -3, r = 0
// (x = 3) and for k = -3, r = 1 (x = 1). bounds_proc takes (mod t 0) and
// (mod t -3) as divisibility constraints with divisors 0 and -3, div_z
// folds them into an lcm of 0 or -3, and qe returns false.
void tst_z3_4_7_1_qe_mod_divisor() {
    char const* bodies[3] = {
        "(and (= (mod (+ x a) 0) 0) (< x b))",
        "(and (= (mod (+ x a) (- 3)) 0) (< x b) (> x a))",
        "(and (= (mod (+ x a) (- 3)) 1) (< x b) (> x a))"
    };
    for (char const* body : bodies) {
        std::string script =
            std::string("(declare-const a Int)\n(declare-const b Int)\n(assert (exists ((x Int)) ") + body + "))\n"
            "(assert (= a 0))\n(assert (= b 10))\n(check-sat-using (then qe smt))\n";
        std::string out = eval_smtlib2(script.c_str());
        std::cout << "qe_mod_divisor " << body << ": " << out;
        ENSURE(out == "sat\n");
    }
}

// exists x. is-nil(x) and (x = y or x = z), with y and z both cons, is
// unsat. datatype_plugin::subst_rec replaces is-nil(x) by true and x by y
// (resp. z) in the equality branches but drops is-nil(y), so qe returns
// true and z3 answers sat.
void tst_z3_4_7_1_qe_dt_eq_recognizer() {
    std::string out = eval_smtlib2(
        "(declare-datatypes ((Lst 0)) (((cons (hd Int) (tl Lst)) (nil))))\n"
        "(declare-const y Lst)\n"
        "(declare-const z Lst)\n"
        "(assert (exists ((x Lst)) (and ((_ is nil) x) (or (= x y) (= x z)))))\n"
        "(assert ((_ is cons) y))\n"
        "(assert ((_ is cons) z))\n"
        "(check-sat-using (then qe smt))\n");
    std::cout << "qe_dt_eq_recognizer: " << out;
    ENSURE(out == "unsat\n");
}

// Two quantified formulas in one goal for qe-light. fm::init resets
// m_bvar2expr but not m_expr2bvar (it resets m_expr2var twice), so in the
// second formula the literal a maps to the stale bvar of the first run,
// which now names c: the clause (or b a) becomes (or b c). With a true and
// b, c false the goal is satisfiable (z <= -2, x >= 5, g z), but z3
// answers unsat.
void tst_z3_4_7_1_qe_lite_fm_bvar() {
    std::string out = eval_smtlib2(
        "(declare-const a Bool)\n"
        "(declare-const b Bool)\n"
        "(declare-const c Bool)\n"
        "(declare-fun g (Int) Bool)\n"
        "(assert (exists ((x Int) (z Int)) (and (or a (<= (+ x z) 3)) (or b (>= (+ x z) 5)) (g z))))\n"
        "(assert (exists ((x Int) (z Int)) (and (or c (<= (+ x z) 3)) (or b (>= x 5)) (or a (<= x 1)) (g z))))\n"
        "(assert a)\n"
        "(assert (not b))\n"
        "(assert (not c))\n"
        "(check-sat-using (then qe-light smt))\n");
    std::cout << "qe_lite_fm_bvar: " << out;
    ENSURE(out == "sat\n");
}

// pb_util::mk_lt normalizes into m_coeffs and calls mk_ge(n, m_coeffs, ...),
// whose normalize resets m_coeffs before it reads the coefficients. So
// 2*p0 + 3*p1 < 3, satisfied by p0 = true, p1 = false, becomes a pbge with
// zero coefficients and a positive bound, which is false. maxres with
// opt.maxres.add_upper_bound_block uses mk_lt to block the upper bound and
// stops at a non-optimal cost (19 instead of 3 on the AUDIT.md input).
void tst_z3_4_7_1_pb_mk_lt_coeffs() {
    ast_manager m;
    reg_decl_plugins(m);
    pb_util pb(m);
    expr_ref p0(bool_const(m, "p0"), m), p1(bool_const(m, "p1"), m);
    rational cs[2] = { rational(2), rational(3) };
    expr* args[2] = { p0, p1 };
    expr_ref lt(pb.mk_lt(2, cs, args, rational(3)), m);
    std::cout << "pb_mk_lt_coeffs: " << mk_pp(lt, m) << "\n";
    expr_ref_vector fmls(m);
    fmls.push_back(lt);
    fmls.push_back(p0);
    fmls.push_back(m.mk_not(p1));
    expect("pb_mk_lt_coeffs", "2*p0 + 3*p1 < 3 with p0, not p1", l_true, check_plain(m, fmls));
}

// opb::parse tries parse_token("min:") before parse_token("max:"), and
// parse_token consumes the matching prefix "m" even when the token does not
// match, so "max:" is read as "ax:" and parse_id exits with
// (error line 1 "unexpected char: a" expected "x") and code 3.
void tst_z3_4_7_1_opb_max_objective() {
    ast_manager m;
    reg_decl_plugins(m);
    opt::context opt(m);
    unsigned_vector handles;
    std::istringstream in("* test\nmax: +2 x1 +3 x2 ;\n+1 x1 +1 x2 <= 1 ;\n");
    parse_opb(opt, in, handles);
    ENSURE(handles.size() == 1);
    lbool r = opt.optimize();
    expr_ref lo = opt.get_lower(handles[0]);
    expr_ref hi = opt.get_upper(handles[0]);
    std::cout << "opb_max_objective: " << lbool_name(r) << " " << lo << " " << hi << "\n";
    ENSURE(r == l_true);
    ENSURE(lo == hi);
}

// MK_NARY_PROBE starts with r = child(1) and then folds from child 1 again,
// so (+ 1 2) is 4 and (* 2 3) is 12.
void tst_z3_4_7_1_probe_nary_fold() {
    std::string out = eval_smtlib2(
        "(check-sat-using (if (= (+ 1 2) 4) (fail-if 1) smt))\n"
        "(check-sat-using (if (= (* 2 3) 12) (fail-if 1) smt))\n"
        "(check-sat-using (if (= (+ 1 2) 3) (fail-if 1) smt))\n"
        "(check-sat-using (if (= (+ 1 2 4) 7) (fail-if 1) smt))\n");
    std::cout << "probe_nary_fold:\n" << out;
    ENSURE(out == "sat\nsat\nunknown\nunknown\n");
}

// Log Z3_mk_pble, Z3_mk_pbge and Z3_mk_pbeq and replay the log. The
// generated log_Z3_mk_pble writes the int coefficients with U and Au, but
// exec_Z3_mk_pble reads them with get_int_array, so every replay prints
// "[z3 exception]: expecting int_array at position 3 but got uint_array"
// (fixed upstream in fed977b49, "fix #1782", which logs I and Ai and
// replays 'i' arrays).
void tst_z3_4_7_1_pb_log_replay() {
    char const* log_name = "z3_4_7_1_pb_log_replay.log";
    ENSURE(Z3_open_log(log_name));
    {
        Z3_config cfg = Z3_mk_config();
        Z3_context ctx = Z3_mk_context(cfg);
        Z3_del_config(cfg);
        Z3_ast args[2] = {
            Z3_mk_const(ctx, Z3_mk_string_symbol(ctx, "a"), Z3_mk_bool_sort(ctx)),
            Z3_mk_const(ctx, Z3_mk_string_symbol(ctx, "b"), Z3_mk_bool_sort(ctx))
        };
        int coeffs[2] = { 1, -2 };
        Z3_mk_pble(ctx, 2, args, coeffs, 1);
        Z3_mk_pbge(ctx, 2, args, coeffs, -1);
        Z3_mk_pbeq(ctx, 2, args, coeffs, 1);
        Z3_del_context(ctx);
    }
    Z3_close_log();
    std::ostringstream out;
    {
        std::ifstream in(log_name);
        z3_replayer r(in);
        register_z3_replayer_cmds(r);
        std::streambuf* saved = std::cout.rdbuf(out.rdbuf());
        try {
            r.parse();
        }
        catch (z3_exception & ex) {
            out << "replay error: " << ex.msg() << "\n";
        }
        std::cout.rdbuf(saved);
    }
    std::remove(log_name);
    std::cout << "pb_log_replay: replay output:\n" << out.str();
    ENSURE(out.str().empty());
}

// sexpr::display prints an atom with display_atom and then, missing a
// return, walks the atom as a composite. (get-user-tactics) displays each
// declared tactic's sexpr, so it segfaults whenever a tactic is a plain
// symbol (fixed upstream in 202ed79a2, "#5445").
void tst_z3_4_7_1_sexpr_display_atom() {
    std::string out = eval_smtlib2(
        "(declare-tactic t1 simplify)\n"
        "(get-user-tactics)\n");
    std::cout << "sexpr_display_atom: " << out;
    ENSURE(out == "((declare-tactic t1 simplify))\n");
}

// With fixedpoint.xform.quantify_arrays the Duality query applies the
// default transformation to the context's own rule set and never restores
// it (pdr and spacer save old_rules and call replace_rules). The query
// rule for q1 and the coi filter for it stay behind, so the rule for q2 is
// gone and the third query no longer sees the rule for q1: z3 answers
// "sat unsat unsat" where q1 is reachable. With xform.scale the stale
// query!0 output is chosen for the q2 query, which then answers sat.
// pkg/AUDIT.md muz_fp_03 (duality_preprocess.go quantify_arrays note).
void tst_z3_4_7_1_duality_quantify_arrays_requery() {
    char const* script =
        "(set-option :fixedpoint.engine duality)\n"
        "(set-option :fixedpoint.xform.inline_eager false)\n"
        "(set-option :fixedpoint.xform.inline_linear false)\n"
        "(set-option :fixedpoint.xform.slice false)\n"
        "(set-option :fixedpoint.xform.subsumption_checker false)\n"
        "(set-option :fixedpoint.xform.coalesce_rules false)\n"
        "(set-option :fixedpoint.xform.quantify_arrays true)\n"
        "(declare-rel p (Real))\n"
        "(declare-rel q1 ())\n"
        "(declare-rel q2 ())\n"
        "(rule (p 1.0))\n"
        "(rule (=> (p 1.0) q1))\n"
        "(rule (=> (p 2.0) q2))\n"
        "(query q1)\n"
        "(query q2)\n"
        "(query q1)\n";
    expect_script_output("duality_quantify_arrays_requery", script, "sat\nunsat\nsat\n");
}

// A Duality query that throws (here iZ3 refuses the arithmetic rewrite
// proof of the err query) has already replaced the saved duality_data with
// a fresh one whose old_rs is null. The next query dereferences that null
// solver to swap back its counterexample and segfaults. ok is reachable
// (s = 1 gives p(1, 1)), so the second query must answer sat.
// pkg/AUDIT.md duality_01-F3 (follow-up: the Duality scale pipeline test).
void tst_z3_4_7_1_duality_failed_query_requery() {
    char const* script =
        "(set-option :fixedpoint.engine duality)\n"
        "(declare-rel p (Real Real))\n"
        "(declare-rel q2 (Real))\n"
        "(declare-rel err ())\n"
        "(declare-rel ok ())\n"
        "(declare-var s Real)\n"
        "(declare-var v Real)\n"
        "(rule (=> (not (<= s 0.0)) (p s s)))\n"
        "(rule (=> (and (p v s) (not (<= s 0.0)) (= v (* 2.0 s))) (q2 s)))\n"
        "(rule (=> (and (q2 s) (not (<= s 0.0))) err))\n"
        "(rule (=> (and (p s s) (not (<= s 0.0))) ok))\n"
        "(query err)\n"
        "(query ok)\n";
    expect_script_output("duality_failed_query_requery", script,
                         "(error \"query failed: theory not supported by interpolation or bad proof\")\nunknown\nsat\n");
}

// model_implicant::eval_array_eq on arrays of arrays: at index 1 the values
// are (store K0 0 1) and (store (store K0 0 1) 3 0), different terms for
// equal arrays, so the nested call marks e true; at index 2 the values are
// (store K0 0 5) and (store K0 0 6), which differ. The marks are bits and
// set_false cannot clear a true mark, so a != b came out as (= a b) true
// and minimize_literals dropped the disequality. pkg/AUDIT.md model_01-F5.
void tst_z3_4_7_1_implicant_nested_array_eq() {
    ast_manager m;
    reg_decl_plugins(m);
    arith_util a(m);
    array_util au(m);
    sort_ref I(a.mk_int(), m);
    sort_ref AI(au.mk_array_sort(I, I), m);
    sort_ref AAI(au.mk_array_sort(I, AI), m);
    expr_ref n0(a.mk_int(0), m), n1(a.mk_int(1), m), n2(a.mk_int(2), m);
    expr_ref n3(a.mk_int(3), m), n5(a.mk_int(5), m), n6(a.mk_int(6), m);
    expr_ref K0(au.mk_const_array(AI, n0), m);
    expr_ref KK(au.mk_const_array(AAI, K0), m);
    auto store = [&](expr* base, expr* index, expr* value) {
        expr* args[3] = { base, index, value };
        return expr_ref(au.mk_store(3, args), m);
    };
    expr_ref X1 = store(K0, n0, n1);
    expr_ref X2 = store(X1, n3, n0);
    expr_ref A = store(store(KK, n2, store(K0, n0, n5)), n1, X1);
    expr_ref B = store(store(KK, n2, store(K0, n0, n6)), n1, X2);
    func_decl_ref ad(m.mk_const_decl(symbol("implicant_a"), AAI), m);
    func_decl_ref bd(m.mk_const_decl(symbol("implicant_b"), AAI), m);
    model_ref mdl = alloc(model, m);
    mdl->register_decl(ad, A);
    mdl->register_decl(bd, B);
    expr_ref eq(m.mk_eq(m.mk_const(ad), m.mk_const(bd)), m);
    expr_ref neq(m.mk_not(eq), m);
    model_implicant mi(m);
    ptr_vector<expr> fmls;
    fmls.push_back(neq);
    expr_ref_vector lits = mi.minimize_literals(fmls, mdl);
    std::cout << "implicant literals:";
    for (expr* l : lits) std::cout << " " << mk_pp(l, m);
    std::cout << "\n";
    ENSURE(lits.size() == 1 && lits.get(0) == neq.get());
}

// With fixedpoint.duality.use_underapprox, RPFP::FixCurrentState runs
// UnderapproxFormula on the edge dual with an empty dont_cares set, so the
// (or (not @underK) U_K) disjuncts fix the model polarity of the @under
// flags into the edge constraints. The next ExpansionChoices check assumes
// the opposite polarity, is unsat, and Duality throws the char const*
// "error in ExpansionChoices", which escapes and aborts z3. Reach(2) is
// derivable (0, 1, 2), so the answer is sat. pkg/AUDIT.md duality_02-F2.
void tst_z3_4_7_1_duality_underapprox_flag_literals() {
    char const* script =
        "(set-option :fixedpoint.engine duality)\n"
        "(set-option :fixedpoint.duality.use_underapprox true)\n"
        "(declare-rel Reach (Int))\n"
        "(declare-rel Goal ())\n"
        "(declare-var x Int)\n"
        "(rule (=> (or (= x 0) (and (> x 0) (Reach (- x 1)))) (Reach x)))\n"
        "(rule (=> (and (Reach x) (= x 2)) Goal))\n"
        "(query Goal)\n";
    expect_script_output("duality_underapprox_flag_literals", script, "sat\n");
}

// With fixedpoint.duality.use_underapprox, DerivationTree::Build first runs
// CheckWithUnderapprox, a check under the @under leaf assumptions. When it
// is unsat, tree->Solve calls interpolate_tree, which only re-checks when
// the solver has no proof, so it interpolates the proof derived under those
// assumptions. Its leaves include the assumption literals, which are not
// constraints of the interpolation problem, and iZ3 fails with "unknown
// assertion". Reach(2) is derivable, so the answer is sat.
// pkg/AUDIT.md duality_01-F6.
void tst_z3_4_7_1_duality_underapprox_stale_proof() {
    char const* script =
        "(set-option :fixedpoint.engine duality)\n"
        "(set-option :fixedpoint.duality.use_underapprox true)\n"
        "(declare-rel Reach (Int))\n"
        "(declare-rel Goal ())\n"
        "(declare-var x Int)\n"
        "(rule (=> (= x 0) (Reach x)))\n"
        "(rule (=> (and (> x 0) (Reach (- x 1))) (Reach x)))\n"
        "(rule (=> (and (Reach x) (= x 2)) Goal))\n"
        "(query Goal)\n";
    expect_script_output("duality_underapprox_stale_proof", script, "sat\n");
}

// print_proof prints each rule variable as tree->Eval(&edge, skolem). qe-lite
// eliminated x from the edge of rule!2 (x1 = x + 3), so x is unconstrained
// in the counterexample model and z3 prints its default, certifying q0(5)
// with (= x 0). The step needs x = 2. pkg/AUDIT.md duality_01-F4.
void tst_z3_4_7_1_duality_certificate_eliminated_var() {
    char const* script =
        "(set-option :fixedpoint.engine duality)\n"
        "(declare-rel q0 (Int))\n"
        "(declare-rel err ())\n"
        "(declare-var x Int)\n"
        "(declare-var x1 Int)\n"
        "(rule (=> (= x 2) (q0 x)))\n"
        "(rule (=> (and (q0 x) (= x1 (+ x 3))) (q0 x1)))\n"
        "(rule (=> (and (q0 x) (> x 4)) err))\n"
        "(query err :print-certificate true)\n";
    char const* expected =
        "sat\n"
        "(derivation\n"
        "(step s!4 (q0 2)\n"
        " rule!1  (subst\n"
        "    (= x 2)\n"
        "  )\n"
        "  (labels  )\n"
        "  (ref   ))\n"
        "(step s!3 (q0 5)\n"
        " rule!2  (subst\n"
        "    (= x 2)\n"
        "    (= x1 5)\n"
        "  )\n"
        "  (labels  )\n"
        "  (ref  s!4  ))\n"
        "(step s!2 (err)\n"
        " rule!3  (subst\n"
        "    (= x 5)\n"
        "  )\n"
        "  (labels  )\n"
        "  (ref  s!3  ))\n"
        "(step s!1 (@Fail!0)\n"
        " rule!4  (subst\n"
        "  )\n"
        "  (labels  )\n"
        "  (ref  s!2  ))\n"
        ")\n"
        "(model \n"
        "\"@under2 -> false\n"
        "@under4 -> false\n"
        "@b_2 -> true\n"
        "@b_3 -> true\n"
        "@b_1 -> true\n"
        "@under3 -> false\n"
        "@b_4 -> true\n"
        "\")\n"
        "\n";
    expect_script_output("duality_certificate_eliminated_var", script, expected);
}

// PDR proof trace for Goal :- P(x), with P(x) a fact. The query!0 root and
// the nullary Goal node both have the state `true`, and the P node too, so
// model_search::get_proof_trace, whose cache is keyed by the state alone,
// gave the root the proof of the first cached node: z3 printed a proof of
// (forall x (P x)) instead of one of query!0. pkg/AUDIT.md muz_pdr_02-F2.
void tst_z3_4_7_1_pdr_proof_nullary_root() {
    char const* script =
        "(set-option :fixedpoint.engine pdr)\n"
        "(set-option :fixedpoint.generate_proof_trace true)\n"
        "(set-option :fixedpoint.xform.inline_eager false)\n"
        "(set-option :fixedpoint.xform.inline_linear false)\n"
        "(set-option :fixedpoint.xform.subsumption_checker false)\n"
        "(declare-rel P (Int))\n"
        "(declare-rel Goal ())\n"
        "(declare-var x Int)\n"
        "(rule (P x))\n"
        "(rule (=> (P x) Goal))\n"
        "(query Goal :print-certificate true)\n";
    expect_script_output("pdr_proof_nullary_root", script,
        "sat\n"
        "(let ((a!1 (forall ((x Int)) (=> (P x) Goal)))\n"
        "      (a!2 (forall ((A Int)) (=> (P A) Goal)))\n"
        "      (a!4 (forall ((x Int)) (P x)))\n"
        "      (a!5 (forall ((A Int)) (P A))))\n"
        "(let ((a!3 (mp (asserted a!1) (symm (der (= a!2 a!1)) (= a!1 a!2)) a!2))\n"
        "      (a!6 (mp (asserted a!4) (symm (der (= a!5 a!4)) (= a!4 a!5)) a!5)))\n"
        "  ((_ hyper-res 1 0)\n"
        "    (asserted (=> Goal query!0))\n"
        "    (hyper-res a!3 a!6 Goal)\n"
        "    query!0)))\n");
}

// PDR proof trace where p(true) is needed by both q(true) and r(true). The
// search expands one p obligation and closes the repeated one without
// children. get_proof_trace reached the childless twin first and cached it
// with its unresolved rule, so the proof of goal kept the premise (a true)
// and concluded (=> (a true) goal), not the query. pkg/AUDIT.md
// muz_pdr_02-F2.
void tst_z3_4_7_1_pdr_proof_duplicate_leaf() {
    char const* script =
        "(set-option :fixedpoint.engine pdr)\n"
        "(set-option :fixedpoint.generate_proof_trace true)\n"
        "(set-option :fixedpoint.xform.slice false)\n"
        "(set-option :fixedpoint.xform.inline_linear false)\n"
        "(set-option :fixedpoint.xform.inline_eager false)\n"
        "(declare-rel a (Bool))\n"
        "(declare-rel p (Bool))\n"
        "(declare-rel q (Bool))\n"
        "(declare-rel r (Bool))\n"
        "(declare-rel goal ())\n"
        "(declare-var v Bool)\n"
        "(rule (=> (= v true) (a v)))\n"
        "(rule (=> (a v) (p v)))\n"
        "(rule (=> (p v) (q v)))\n"
        "(rule (=> (p v) (r v)))\n"
        "(rule (=> (and (q true) (r true)) goal))\n"
        "(query goal :print-certificate true)\n";
    expect_script_output("pdr_proof_duplicate_leaf", script,
        "sat\n"
        "(let ((a!1 (asserted (=> (and (q true) (r true)) goal)))\n"
        "      (a!2 (= (=> (and (q true) (r true)) goal)\n"
        "              (=> (and (r true) (q true)) goal)))\n"
        "      (a!4 (forall ((v Bool)) (=> (p v) (r v))))\n"
        "      (a!5 (forall ((A Bool)) (=> (p A) (r A))))\n"
        "      (a!7 (forall ((v Bool)) (=> (a v) (p v))))\n"
        "      (a!8 (forall ((A Bool)) (=> (a A) (p A))))\n"
        "      (a!10 (forall ((v Bool)) (=> (= v true) (a v))))\n"
        "      (a!11 (forall ((A Bool)) (=> (= A true) (a A))))\n"
        "      (a!14 (forall ((v Bool)) (=> (p v) (q v))))\n"
        "      (a!15 (forall ((A Bool)) (=> (p A) (q A)))))\n"
        "(let ((a!3 (mp a!1 (rewrite a!2) (=> (and (r true) (q true)) goal)))\n"
        "      (a!6 (mp (asserted a!4) (symm (der (= a!5 a!4)) (= a!4 a!5)) a!5))\n"
        "      (a!9 (mp (asserted a!7) (symm (der (= a!8 a!7)) (= a!7 a!8)) a!8))\n"
        "      (a!12 (mp (asserted a!10) (symm (der (= a!11 a!10)) (= a!10 a!11)) a!11))\n"
        "      (a!16 (mp (asserted a!14) (symm (der (= a!15 a!14)) (= a!14 a!15)) a!15)))\n"
        "(let ((a!13 (mp a!12 (rewrite (= a!11 (a true))) (a true))))\n"
        "(let ((a!17 ((_ hyper-res 1 0 2 0)\n"
        "              a!3\n"
        "              (hyper-res a!6 (hyper-res a!9 a!13 (p true)) (r true))\n"
        "              (hyper-res a!16 (hyper-res a!9 a!13 (p true)) (q true))\n"
        "              goal)))\n"
        "  ((_ hyper-res 1 0) (asserted (=> goal query!0)) a!17 query!0)))))\n");
}

// fixedpoint.print_boogie_certificate turns the PDR proof trace on without
// fixedpoint.generate_proof_trace, so the rules carry no proofs.
// get_proof_trace only has SASSERT(p1), and in a release build the null
// premise reaches the Boogie printer, which segfaults. The rule's formula is
// a sound premise (it was asserted). pkg/AUDIT.md muz_pdr_02-F2.
void tst_z3_4_7_1_pdr_boogie_without_proof_trace() {
    char const* script =
        "(set-option :fixedpoint.engine pdr)\n"
        "(set-option :fixedpoint.print_boogie_certificate true)\n"
        "(declare-rel inv (Int))\n"
        "(declare-rel err ())\n"
        "(declare-var x Int)\n"
        "(rule (=> (= x 0) (inv x)))\n"
        "(rule (=> (and (inv x) (< x 3)) (inv (+ x 1))))\n"
        "(rule (=> (and (inv x) (= x 2)) err))\n"
        "(query err :print-certificate true)\n";
    expect_script_output("pdr_boogie_without_proof_trace", script,
        "sat\n"
        "(derivation\n"
        "(step\n s!0 (inv 0)\n null\n (subst)\n (labels)\n (ref)\n)\n"
        "(step\n s!1 (inv 1)\n null\n (subst\n  (= B 0)\n  (= A 1))\n (labels)\n (ref s!0)\n)\n"
        "(step\n s!2 (inv 2)\n null\n (subst\n  (= B 0)\n  (= A 1))\n (labels)\n (ref s!1)\n)\n"
        "(step\n s!3 query!0\n null\n (subst\n  (= B 0)\n  (= A 1))\n (labels)\n (ref s!2)\n)\n"
        ")\n\n");
}

// PDR proof trace for G :- P(x), Q(x), Q(0), R(y). Once P(0) is resolved,
// Q(x) becomes Q(0) and remove_duplicate_tails merges it with the other
// Q(0) tail, so the rule has one tail fewer than the node has children.
// get_proof_trace always unifies tail 0 with the next child; for the merged
// child that fails and z3 went on with the stale unifier, building a
// hyper-resolution over a garbled rule. The merged child's premise must be
// left out. pkg/AUDIT.md muz_pdr_02-F2.
void tst_z3_4_7_1_pdr_proof_merged_tails() {
    char const* script =
        "(set-option :fixedpoint.engine pdr)\n"
        "(set-option :fixedpoint.generate_proof_trace true)\n"
        "(set-option :fixedpoint.xform.inline_eager false)\n"
        "(set-option :fixedpoint.xform.inline_linear false)\n"
        "(set-option :fixedpoint.xform.subsumption_checker false)\n"
        "(declare-rel P (Int))\n"
        "(declare-rel Q (Int))\n"
        "(declare-rel R (Int))\n"
        "(declare-rel G ())\n"
        "(declare-var x Int)\n"
        "(declare-var y Int)\n"
        "(rule (P 0))\n"
        "(rule (Q 0))\n"
        "(rule (R y))\n"
        "(rule (=> (and (P x) (Q x) (Q 0) (R y)) G))\n"
        "(query G :print-certificate true)\n";
    expect_script_output("pdr_proof_merged_tails", script,
        "sat\n"
        "(let ((a!1 (forall ((x Int) (y Int)) (=> (and (P x) (Q x) (Q 0) (R y)) G)))\n"
        "      (a!2 (forall ((A Int) (B Int)) (=> (and (R B) (P A) (Q A) (Q 0)) G)))\n"
        "      (a!4 (forall ((y Int)) (R y)))\n"
        "      (a!5 (forall ((A Int)) (R A))))\n"
        "(let ((a!3 (mp (asserted a!1) (symm (der (= a!2 a!1)) (= a!1 a!2)) a!2))\n"
        "      (a!6 (mp (asserted a!4) (symm (der (= a!5 a!4)) (= a!4 a!5)) a!5)))\n"
        "  ((_ hyper-res 1 0)\n"
        "    (asserted (=> G query!0))\n"
        "    (hyper-res a!3 a!6 (asserted (P 0)) (asserted (Q 0)) G)\n"
        "    query!0)))\n");
}

// A model evaluation that exceeds max_steps throws rewriter_exception and
// leaves the frame and result stacks of the interrupted traversal behind.
// The next evaluation's main_loop resumes them and mixes the stale results
// with the new ones: evaluating (> x 0) applies + to a Bool and throws
// "Sort mismatch". proto_model::eval catches the exception and goes on
// evaluating, so MBQI later reports such a sort mismatch or segfaults
// (pkg/AUDIT.md smtcore_14-F3). Upstream rewriter_tpl::operator() clears
// the stale stacks first (Z3Prover/z3 18bb90f93, b013df9a9).
void tst_z3_4_7_1_rewriter_stale_stacks() {
    ast_manager m;
    reg_decl_plugins(m);
    arith_util a(m);
    app_ref x(m.mk_const(symbol("x"), a.mk_int()), m);
    model_ref md = alloc(model, m);
    md->register_decl(x->get_decl(), a.mk_numeral(rational(1), true));
    expr_ref deep(x, m);
    for (unsigned i = 0; i < 20; ++i)
        deep = a.mk_add(x, a.mk_mul(x, deep));
    params_ref p;
    p.set_uint("max_steps", 40);
    model_evaluator ev(*md, p);
    bool thrown = false;
    try {
        expr_ref r(m);
        ev(deep, r);
    }
    catch (model_evaluator_exception &) {
        thrown = true;
    }
    std::cout << "rewriter_stale_stacks: first evaluation threw: " << thrown << "\n";
    ENSURE(thrown);
    expr_ref gt(a.mk_gt(x, a.mk_int(0)), m);
    expr_ref r(m);
    try {
        ev(gt, r);
    }
    catch (z3_exception & ex) {
        std::cout << "rewriter_stale_stacks: second evaluation threw: " << ex.msg() << "\n";
        ENSURE(false);
    }
    std::cout << "rewriter_stale_stacks: " << mk_pp(gt, m) << " -> " << mk_pp(r, m) << "\n";
    ENSURE(m.is_true(r));
}

// exists x:S. x != c over a finite-domain (datalog) sort of size 4 is true.
// The domain is large (4 >= one atom), so the dl plugin has one branch, the
// "other value" branch, where x differs from c. assign_large_domain asserts
// not (= x c), but subst_large_domain substitutes true for the negatively
// occurring atom (= x c), so z3 eliminates the quantifier to false.
void tst_z3_4_7_1_qe_dl_other_value_neq() {
    ast_manager m;
    reg_decl_plugins(m);
    datalog::dl_decl_util dl(m);
    sort_ref s(dl.mk_sort(symbol("S"), 4), m);
    app_ref c(m.mk_const(symbol("c"), s), m);
    expr_ref body(m.mk_not(m.mk_eq(m.mk_var(0, s), c)), m);
    sort* ss[1] = { s };
    symbol names[1] = { symbol("x") };
    expr_ref fml(m.mk_exists(1, ss, names, body), m);
    smt_params fp;
    qe::expr_quant_elim qe(m, fp);
    expr_ref result(m);
    qe(m.mk_true(), fml, result);
    std::cout << "qe_dl_other_value_neq: " << mk_pp(fml, m) << " -> " << mk_pp(result, m) << "\n";
    ENSURE(m.is_true(result));
}

// exists x:S. x != y and x != z over a finite-domain sort of size 2 is
// (= y z), and exists x:S. x != 0 and x != 1 is false. is_small_domain
// tests domain_size < #atoms, so with 2 atoms in a 2-element domain the dl
// plugin uses the large-domain mode, whose "other value" branch assumes a
// value that differs from every atom; none exists, and z3 eliminates both
// to true. pkg/AUDIT.md qe_01-F4.
void tst_z3_4_7_1_qe_dl_domain_boundary() {
    ast_manager m;
    reg_decl_plugins(m);
    datalog::dl_decl_util dl(m);
    sort_ref s(dl.mk_sort(symbol("S"), 2), m);
    app_ref y(m.mk_const(symbol("y"), s), m);
    app_ref z(m.mk_const(symbol("z"), s), m);
    expr_ref x(m.mk_var(0, s), m);
    sort* ss[1] = { s };
    symbol names[1] = { symbol("x") };
    smt_params fp;
    expr_ref body(m.mk_and(m.mk_not(m.mk_eq(x, y)), m.mk_not(m.mk_eq(x, z))), m);
    expr_ref fml(m.mk_exists(1, ss, names, body), m);
    expr_ref result(m);
    {
        qe::expr_quant_elim qe(m, fp);
        qe(m.mk_true(), fml, result);
    }
    std::cout << "qe_dl_domain_boundary: " << mk_pp(fml, m) << " -> " << mk_pp(result, m) << "\n";
    // Brute force over the domain: result must hold exactly when y = z.
    for (unsigned yv = 0; yv < 2; ++yv) {
        for (unsigned zv = 0; zv < 2; ++zv) {
            expr_safe_replace rep(m);
            rep.insert(y, dl.mk_numeral(yv, s));
            rep.insert(z, dl.mk_numeral(zv, s));
            expr_ref inst(m);
            rep(result, inst);
            th_rewriter rw(m);
            rw(inst);
            ENSURE(yv == zv ? m.is_true(inst) : m.is_false(inst));
        }
    }
    expr_ref zero(dl.mk_numeral(0, s), m), one(dl.mk_numeral(1, s), m);
    expr_ref body2(m.mk_and(m.mk_not(m.mk_eq(x, zero)), m.mk_not(m.mk_eq(x, one))), m);
    expr_ref fml2(m.mk_exists(1, ss, names, body2), m);
    {
        qe::expr_quant_elim qe(m, fp);
        qe(m.mk_true(), fml2, result);
    }
    std::cout << "qe_dl_domain_boundary: " << mk_pp(fml2, m) << " -> " << mk_pp(result, m) << "\n";
    ENSURE(m.is_false(result));
}

// sieve_relation_plugin::mk_filter_by_negation_fn computes its
// "column is inner" flags as !is_inner_col, so it handles each equality
// in the wrong case: an equality between two inner columns is dropped (the
// inner relations are then subtracted on zero columns, which empties the
// target whenever the negated relation is nonempty), and an equality on a
// sieved (full) column of the negated relation, which always holds, turns
// the whole filter into the identity. It also maps the equality's loop
// index, not its column, to the inner column. pkg/AUDIT.md muz_rel_06-F5.
void tst_z3_4_7_1_sieve_negation_inner_cols() {
    ast_manager m;
    reg_decl_plugins(m);
    datalog::register_engine re;
    smt_params fp;
    datalog::context ctx(m, re, fp);
    datalog::relation_manager & rmgr = ctx.get_rel_context()->get_rmanager();
    datalog::dl_decl_util & dl = ctx.get_decl_util();
    datalog::sieve_relation_plugin & sieve = datalog::sieve_relation_plugin::get_plugin(rmgr);
    sort_ref s(dl.mk_sort(symbol("S"), 4), m);
    datalog::relation_signature sig;
    sig.push_back(s);
    sig.push_back(s);
    datalog::relation_signature inner_sig;
    inner_sig.push_back(s);
    // Column 0 is sieved (full), column 1 is the inner column.
    bool inner_cols[2] = { false, true };
    auto mk_rel = [&](unsigned n, unsigned const* vals) {
        datalog::sieve_relation * r = sieve.mk_from_inner(sig, inner_cols,
            rmgr.mk_empty_relation(inner_sig, null_family_id));
        for (unsigned i = 0; i < n; ++i) {
            datalog::relation_fact f(m);
            f.push_back(dl.mk_numeral(0, s));
            f.push_back(dl.mk_numeral(vals[i], s));
            r->add_fact(f);
        }
        return r;
    };
    auto has = [&](datalog::relation_base const& r, unsigned v) {
        datalog::relation_fact f(m);
        f.push_back(dl.mk_numeral(2, s));
        f.push_back(dl.mk_numeral(v, s));
        return r.contains_fact(f);
    };
    unsigned tgt_vals[2] = { 0, 1 };
    unsigned neg_vals[1] = { 1 };
    // Both ends inner: t(_, y) minus n(_, y) on column 1 keeps only y = 0.
    {
        datalog::scoped_rel<datalog::relation_base> t = mk_rel(2, tgt_vals);
        datalog::scoped_rel<datalog::relation_base> n = mk_rel(1, neg_vals);
        unsigned cols[1] = { 1 };
        scoped_ptr<datalog::relation_intersection_filter_fn> fn =
            rmgr.mk_filter_by_negation_fn(*t, *n, 1, cols, cols);
        ENSURE(fn);
        (*fn)(*t, *n);
        std::cout << "sieve_negation_inner_cols: inner/inner: y=0 kept " << has(*t, 0)
                  << ", y=1 kept " << has(*t, 1) << "\n";
        ENSURE(has(*t, 0) && !has(*t, 1));
    }
    // Negated column full (sieved): the equality always holds, so a nonempty
    // n removes every row of t.
    {
        datalog::scoped_rel<datalog::relation_base> t = mk_rel(2, tgt_vals);
        datalog::scoped_rel<datalog::relation_base> n = mk_rel(1, neg_vals);
        unsigned t_cols[1] = { 1 };
        unsigned n_cols[1] = { 0 };
        scoped_ptr<datalog::relation_intersection_filter_fn> fn =
            rmgr.mk_filter_by_negation_fn(*t, *n, 1, t_cols, n_cols);
        ENSURE(fn);
        (*fn)(*t, *n);
        std::cout << "sieve_negation_inner_cols: inner/full: t empty " << t->empty() << "\n";
        ENSURE(t->empty());
    }
}

// With datalog.default_table_checked, the selection q(#b100) is projected
// to a check_table with no columns, and the union into g's still-empty
// register creates the target with the source's plugin:
// check_table_plugin::mk_empty passes the 0-column signature to the sparse
// plugin (the checked default table or the checker), which cannot handle
// it and crashes in column_layout::make_byte_aligned_end. n(#b100) holds
// and path(#b100, #b100) does not, so g is derivable. Found while fixing
// the pkg/AUDIT.md muz_rel findings (no finding ID of its own).
void tst_z3_4_7_1_check_table_nullary_empty() {
    char const* tables[2][2] = { { "sparse", "hashtable" }, { "hashtable", "sparse" } };
    for (unsigned i = 0; i < 2; ++i) {
        std::string script =
            std::string("(set-option :fixedpoint.engine datalog)\n"
                        "(set-option :fixedpoint.datalog.default_table ") + tables[i][0] + ")\n"
            "(set-option :fixedpoint.datalog.default_table_checked true)\n"
            "(set-option :fixedpoint.datalog.default_table_checker " + tables[i][1] + ")\n"
            "(define-sort s () (_ BitVec 3))\n"
            "(declare-rel e (s s))\n"
            "(declare-rel path (s s))\n"
            "(declare-rel n (s))\n"
            "(declare-rel q (s))\n"
            "(declare-rel g ())\n"
            "(declare-var x s)\n"
            "(declare-var y s)\n"
            "(rule (e #b001 #b010))\n"
            "(rule (e #b100 #b101))\n"
            "(rule (=> (e x y) (n x)))\n"
            "(rule (=> (e x y) (path x y)))\n"
            "(rule (=> (and (n x) (not (path x x))) (q x)))\n"
            "(rule (=> (q #b100) g))\n"
            "(query g)\n";
        std::cout << "check_table_nullary_empty: default_table " << tables[i][0]
                  << ", checker " << tables[i][1] << "\n";
        expect_script_output("check_table_nullary_empty", script.c_str(), "sat\n");
    }
}

// With datalog.default_table=bitvector, the bitvector_table constructor
// reserved its bit vector inside the column loop, so the table of the
// nullary g reserved no bits and add_fact (fact2offset = 0) wrote past the
// empty bit vector and crashed. q(#b100) holds (n(#b100) and no path from
// #b100 back to itself), so g is derivable. Fixed upstream in 8dac9b7b94,
// which reserves 1 << shift bits after the loop. No pkg/AUDIT.md finding.
void tst_z3_4_7_1_bitvector_table_nullary() {
    char const* script =
        "(set-option :fixedpoint.engine datalog)\n"
        "(set-option :fixedpoint.datalog.default_table bitvector)\n"
        "(define-sort s () (_ BitVec 3))\n"
        "(declare-rel e (s s))\n"
        "(declare-rel path (s s))\n"
        "(declare-rel n (s))\n"
        "(declare-rel q (s))\n"
        "(declare-rel g ())\n"
        "(declare-var x s)\n"
        "(declare-var y s)\n"
        "(declare-var z s)\n"
        "(rule (e #b001 #b010))\n"
        "(rule (e #b010 #b011))\n"
        "(rule (e #b011 #b001))\n"
        "(rule (e #b100 #b101))\n"
        "(rule (=> (e x y) (n x)))\n"
        "(rule (=> (e x y) (path x y)))\n"
        "(rule (=> (and (path x y) (e y z)) (path x z)))\n"
        "(rule (=> (and (n x) (not (path x x))) (q x)))\n"
        "(rule (=> (q #b100) g))\n"
        "(query g)\n";
    expect_script_output("bitvector_table_nullary", script, "sat\n");
}

// With smt.case_split=6, context::decide declared `lbool phase;` without a
// value and theory_aware_branching_queue::next_case_split returns without
// writing it, so the phase was indeterminate; the release build decided
// negative every time, whatever smt.phase_selection said. With the phase
// initialized to l_undef (as upstream does), phase_selection=1 (always
// true) makes every decision positive: the first decision sets c to true,
// which satisfies (or a b c), so a and b stay irrelevant and are false in
// the model; with relevancy off, all three are decided and true.
// pkg/AUDIT.md smtcore_02-F9.
void tst_z3_4_7_1_case_split6_phase() {
    char const* relevancy[2] = { "2", "0" };
    char const* expected[2] = {
        "sat\n((a false)\n (b false)\n (c true))\n",
        "sat\n((a true)\n (b true)\n (c true))\n" };
    for (unsigned i = 0; i < 2; ++i) {
        std::string script =
            std::string("(set-option :auto_config false)\n"
                        "(set-option :smt.case_split 6)\n"
                        "(set-option :smt.phase_selection 1)\n"
                        "(set-option :smt.relevancy ") + relevancy[i] + ")\n"
            "(set-option :produce-models true)\n"
            "(declare-const a Bool)\n"
            "(declare-const b Bool)\n"
            "(declare-const c Bool)\n"
            "(assert (or a b c))\n"
            "(check-sat-using smt)\n"
            "(get-value (a b c))\n";
        std::cout << "case_split6_phase: relevancy " << relevancy[i] << "\n";
        expect_script_output("case_split6_phase", script.c_str(), expected[i]);
    }
}

// theory_pb::internalize_atom with smt.pb.enable_simplex uses an atom's bool
// var both as its 0-1 column (in rows whose argument it is) and as the slack
// column of its own row (the "TBD" comment there). In
// (pbge 3: 2*I + b + d) with I = (pbge 2: ~b + ~d + a), the outer row puts
// [0,1] bounds on I's column, which is I's slack ~b + ~d + a, so once I is
// true (slack >= 2) the simplex is infeasible and z3 answers unsat; a = b =
// true, d = false satisfies both. With a disjunction over the inner atom's
// arguments z3 segfaults instead. Follow-up to pkg/AUDIT.md smtcore_12-F4;
// upstream removed pb.enable_simplex.
void tst_z3_4_7_1_pb_simplex_nested() {
    char const* header =
        "(set-option :auto_config false)\n"
        "(set-option :smt.pb.enable_simplex true)\n"
        "(declare-const a Bool)\n"
        "(declare-const b Bool)\n"
        "(declare-const c Bool)\n"
        "(declare-const d Bool)\n";
    std::string unsat_script = std::string(header) +
        "(assert ((_ pbge 3 2 1 1) ((_ pbge 2 1 1 1) (not b) (not d) a) b d))\n"
        "(check-sat)\n";
    expect_script_output("pb_simplex_nested", unsat_script.c_str(), "sat\n");
    std::string crash_script = std::string(header) +
        "(assert ((_ pbge 3 2 1 1) ((_ pbge 2 1 1 1) a b c) d (not a)))\n"
        "(assert (or (not d) (and a b c)))\n"
        "(check-sat)\n";
    expect_script_output("pb_simplex_nested", crash_script.c_str(), "sat\n");
}

// With smt.pb.enable_simplex, theory_pb::assign_eh bounds the 0-1 column of
// a bool var whenever the var has a (possibly empty) literal watch list.
// pop_scope_eh empties the watch lists of the popped constraints but keeps
// the lists. In the scope below, v5 (bool var 5) is an argument; after the
// pop, var 5 is reused for the atom (pbeq 0: 2*v0), whose slack column is
// also 5. Assigning the atom true puts the literal lower bound 1 on column
// 5, its slack upper bound 0 then conflicts, and z3 answers unsat; v0 =
// v3 = false, v1 = v2 = v5 = true is a model. Follow-up to pkg/AUDIT.md
// smtcore_12-F4; upstream removed pb.enable_simplex.
void tst_z3_4_7_1_pb_simplex_pop() {
    char const* script =
        "(set-option :auto_config false)\n"
        "(set-option :smt.pb.enable_simplex true)\n"
        "(declare-const v0 Bool)\n"
        "(declare-const v1 Bool)\n"
        "(declare-const v2 Bool)\n"
        "(declare-const v3 Bool)\n"
        "(declare-const v4 Bool)\n"
        "(declare-const v5 Bool)\n"
        "(push 1)\n"
        "(assert ((_ pble 1 2 3 2 1 4) v3 v0 v2 v4 (not v5)))\n"
        "(check-sat)\n"
        "(pop 1)\n"
        "(assert (not ((_ pbge 3 3 5 5 4) (not v2) v3 (not v1) (not v5))))\n"
        "(assert ((_ pbeq 1 1 2) v2 v0))\n"
        "(check-sat)\n";
    expect_script_output("pb_simplex_pop", script, "sat\nsat\n");
}

// smt_solver::updt_params(p) passed p alone to smt_params::updt_params and
// context::updt_params, so every smt setting not in p was re-read from the
// global defaults. Z3_get_implied_equalities calls updt_params with just
// produce_models=true, which silently re-enabled mbqi on a solver created
// with mbqi=false: the quantified problem below then answered sat instead of
// unknown (and a quantifier internalized under mbqi=false crashed the model
// checker on the next check). Upstream applies the solver's merged
// parameters (solver::get_params()) instead.
void tst_z3_4_7_1_solver_updt_params() {
    Z3_config cfg = Z3_mk_config();
    Z3_context ctx = Z3_mk_context(cfg);
    Z3_del_config(cfg);
    Z3_solver s = Z3_mk_simple_solver(ctx);
    Z3_solver_inc_ref(ctx, s);
    Z3_params p = Z3_mk_params(ctx);
    Z3_params_inc_ref(ctx, p);
    Z3_params_set_bool(ctx, p, Z3_mk_string_symbol(ctx, "mbqi"), false);
    Z3_solver_set_params(ctx, s, p);
    Z3_params_dec_ref(ctx, p);
    Z3_ast q = Z3_mk_const(ctx, Z3_mk_string_symbol(ctx, "q"), Z3_mk_bool_sort(ctx));
    unsigned id = 0;
    Z3_lbool ie = Z3_get_implied_equalities(ctx, s, 1, &q, &id);
    Z3_solver_from_string(ctx, s,
        "(declare-fun f (Int) Int)(declare-const c Int)"
        "(assert (forall ((x Int)) (! (> (f x) 0) :pattern ((f x)))))"
        "(assert (> (f c) 1))");
    Z3_lbool r = Z3_solver_check(ctx, s);
    std::cout << "solver_updt_params: implied equalities " << ie << ", check " << r << " (expected 0: mbqi stays off)\n";
    Z3_solver_dec_ref(ctx, s);
    Z3_del_context(ctx);
    ENSURE(ie == Z3_L_TRUE);
    ENSURE(r == Z3_L_UNDEF);
}

// theory_pb::internalize_atom drops the arguments that are already assigned
// and subtracts the coefficients of the true ones from the bound. Below,
// a and c are true at base level when (pbeq 1: a + c) is internalized, so
// the equality keeps no arguments and bound -1. pb_rewriter_util::normalize
// only rejects an equality whose bound exceeds the sum of its coefficients,
// so it returns l_undef; a constraint with no arguments is never watched
// and never propagated, and z3 answers sat (model_validate reports the
// model invalid). The problem is unsat: a + c = 2. pb_rewriter::mk_app_core
// folds such an equality to false; upstream axiomatized pb equalities as
// two inequalities in bd96eaff47 ("axiomatize pb-eq").
void tst_z3_4_7_1_pb_eq_no_args() {
    char const* script =
        "(declare-const a Bool)\n"
        "(declare-const b Bool)\n"
        "(declare-const c Bool)\n"
        "(assert (or a b))\n"
        "(assert (not b))\n"
        "(assert ((_ pbeq 1 1 1) a c))\n"
        "(assert (or c b))\n"
        "(check-sat)\n";
    expect_script_output("pb_eq_no_args", script, "unsat\n");
}

// theory_pb::assert_ge, the test entry point to the pb sorting network,
// builds a local theory_pb and never calls init on it, so its manager is
// null. The first clause the network adds calls th.justify, whose
// proofs_enabled() dereferences that manager, and z3 segfaults: on all
// four literals false and k = 1, tst_theory_pb crashes right after its
// fuzz phase. Upstream constructs the theory from the context since
// becf423c77 ("remove level of indirection for context and ast_manager
// in smt_theory"). Mirrors the N = 4 rounds of tst_theory_pb.
void tst_z3_4_7_1_pb_assert_ge_uninit() {
    ast_manager m;
    smt_params params;
    params.m_model = true;
    reg_decl_plugins(m);
    unsigned const N = 4;
    for (unsigned i = 0; i < (1u << N); ++i) {
        smt::literal_vector lits(N, smt::false_literal);
        unsigned k = 0;
        for (unsigned j = 0; j < N; ++j) {
            if (i & (1u << j)) {
                lits[j] = smt::true_literal;
                ++k;
            }
        }
        {
            smt::context ctx(m, params);
            ctx.push();
            smt::literal l = smt::theory_pb::assert_ge(ctx, k + 1, lits.size(), lits.c_ptr());
            if (l != smt::false_literal) {
                ctx.assign(l, nullptr, false);
                ENSURE(l_false == ctx.check());
            }
            ctx.pop(1);
        }
        {
            smt::context ctx(m, params);
            ctx.push();
            smt::literal l = smt::theory_pb::assert_ge(ctx, k, lits.size(), lits.c_ptr());
            ENSURE(l != smt::false_literal);
            ctx.assign(l, nullptr, false);
            ENSURE(l_true == ctx.check());
            ctx.pop(1);
        }
    }
    std::cout << "pb_assert_ge_uninit: PASS\n";
}

// mk_to_fp's "1 Real -> 1 FP" branch compares domain[1], one past the end
// of the arity-1 domain, with m_real_sort (undefined behavior; the release
// build rejects (_ to_fp eb sb) of a Real with "sort mismatch, expected one
// argument of Real sort"). Accepting the input is no fix: fpa2bv_converter
// has no one-Real to_fp and crashes on it. Upstream removed the branch and
// "(Real)" from the supported-combinations message, so the declaration
// falls through to that message. pkg/AUDIT.md ast_05-F2.
void tst_z3_4_7_1_to_fp_one_real() {
    char const* script =
        "(declare-const f (_ FloatingPoint 3 3))\n"
        "(declare-const r Real)\n"
        "(assert (= f ((_ to_fp 3 3) r)))\n";
    std::ostringstream out;
    {
        cmd_context ctx;
        ctx.set_solver_factory(mk_smt_strategic_solver_factory());
        ctx.set_regular_stream(out);
        ctx.set_diagnostic_stream(out);
        std::istringstream in(script);
        parse_smt2_commands(ctx, in);
    }
    std::string expected =
        "(error \"line 3 column 29: Unexpected argument combination for (_ to_fp eb sb). "
        "Supported argument combinations are: ((_ BitVec 1) (_ BitVec eb) (_ BitVec sb-1)), "
        "(_ BitVec (eb+sb)), (RoundingMode (_ BitVec (eb+sb))), "
        "(RoundingMode (_ FloatingPoint eb' sb')), (RoundingMode Int Real), "
        "(RoundingMode Real Int), (RoundingMode Int), and (RoundingMode Real).\")\n";
    std::cout << "to_fp_one_real: got\n" << out.str();
    ENSURE(out.str() == expected);
}

#ifndef _WINDOWS
namespace {
    void get_file_names_posix_timeout(int) {
        static char const msg[] =
            "get_file_names_posix: get_file_names did not return within 20 seconds\n";
        ssize_t n = write(2, msg, sizeof(msg) - 1);
        (void)n;
        _exit(1);
    }

    void get_file_names_touch(std::string const& path) {
        std::ofstream out(path.c_str());
        ENSURE(out.good());
    }
}
#endif

// datalog::get_file_names lists the files with a given extension, as the WPA
// directory loader (wpa_parser_impl::parse_directory_core) needs. In 4.7.1
// the POSIX branch is NOT_IMPLEMENTED_YET(): z3 prints NOT IMPLEMENTED YET!
// and reaches UNREACHABLE(), which hangs a release build. The expected list
// follows the Windows code: the files of a directory whose names end with
// ".map", then each subdirectory not starting with '.', recursively; names
// are visited in sorted order so the result does not depend on readdir.
void tst_z3_4_7_1_get_file_names_posix() {
#ifndef _WINDOWS
    signal(SIGALRM, get_file_names_posix_timeout);
    alarm(20);
    char const* tmp = getenv("TMPDIR");
    std::string tmpl = std::string(tmp && *tmp ? tmp : "/tmp") + "/z3_get_file_names_XXXXXX";
    std::vector<char> buf(tmpl.begin(), tmpl.end());
    buf.push_back(0);
    ENSURE(mkdtemp(buf.data()) != nullptr);
    std::string root(buf.data());
    char const* dirs[] = { "/sub", "/sub/deeper", "/a0", "/.hid", "/d.map" };
    for (char const* d : dirs) {
        ENSURE(mkdir((root + d).c_str(), 0700) == 0);
    }
    char const* files[] = { "/b.map", "/a.map", "/c.rel", "/map", "/xmap", "/e.mapx",
                            "/.h.map", "/sub/z.map", "/sub/deeper/y.map", "/a0/w.map",
                            "/.hid/q.map" };
    for (char const* f : files) {
        get_file_names_touch(root + f);
    }
    datalog::string_vector res;
    datalog::get_file_names(root, "map", true, res);
    std::cout.flush(); // the watchdog leaves with _exit
    char const* expected[] = { "/.h.map", "/a.map", "/b.map", "/a0/w.map", "/sub/z.map",
                               "/sub/deeper/y.map" };
    unsigned const n = sizeof(expected) / sizeof(expected[0]);
    bool ok = res.size() == n;
    for (unsigned i = 0; ok && i < n; ++i) {
        ok = res[i] == root + expected[i];
    }
    datalog::string_vector flat;
    datalog::get_file_names(root + "/", "rel", false, flat);
    ok = ok && flat.size() == 1 && flat[0] == root + "/c.rel";
    alarm(0);
    if (!ok) {
        std::cerr << "get_file_names_posix: FAIL\n";
        for (std::string const& r : res) {
            std::cerr << "  " << r << "\n";
        }
        for (std::string const& r : flat) {
            std::cerr << "  rel: " << r << "\n";
        }
        ENSURE(false);
    }
    std::string cleanup = "rm -rf '" + root + "'";
    ENSURE(system(cleanup.c_str()) == 0);
    std::cout << "get_file_names_posix: PASS\n";
#endif
}

#ifndef _WINDOWS
namespace {
    // A fresh directory under $TMPDIR (or /tmp) holding the given files.
    std::string wpa_test_directory(char const* const* files, unsigned n) {
        char const* tmp = getenv("TMPDIR");
        std::string tmpl = std::string(tmp && *tmp ? tmp : "/tmp") + "/z3_wpa_XXXXXX";
        std::vector<char> buf(tmpl.begin(), tmpl.end());
        buf.push_back(0);
        ENSURE(mkdtemp(buf.data()) != nullptr);
        std::string root(buf.data());
        for (unsigned i = 0; i + 1 < n; i += 2) {
            std::ofstream out((root + "/" + files[i]).c_str());
            out << files[i + 1];
            ENSURE(out.good());
        }
        return root;
    }

    // wpa_parser::parse_directory on root; the warnings go to warnings.
    bool wpa_parse_directory(std::string const& root, std::ostream& warnings) {
        ast_manager m;
        smt_params s_params;
        reg_decl_plugins(m);
        datalog::register_engine re;
        params_ref params;
        params.set_sym("engine", symbol("datalog"));
        datalog::context ctx(m, re, s_params, params);
        set_warning_stream(&warnings);
        scoped_ptr<datalog::wpa_parser> parser = datalog::wpa_parser::create(ctx, m);
        bool ok = parser->parse_directory(root.c_str());
        set_warning_stream(nullptr);
        return ok;
    }
}
#endif

// A tuple number missing from the sort's map file is skipped with
// warning_msg("symbol number %I64u on line %d ..."). %I64u is MSVC's
// spelling of a 64-bit unsigned; glibc reads it as the I flag, field width
// 64 and %u, so the number is padded to 64 columns and read as a 32-bit
// unsigned int from a uint64_t argument (undefined behavior). The fork
// formats it with PRIu64. Upstream (v5.4.2) still uses %I64u.
void tst_z3_4_7_1_wpa_warning_number() {
#ifndef _WINDOWS
    char const* files[] = { "V.map", "1 a\n", "r.rules", "e(x:V) printtuples\n",
                            "e.rel", "1\n4294967301\n" };
    std::string root = wpa_test_directory(files, 6);
    std::stringstream warnings;
    ENSURE(wpa_parse_directory(root, warnings));
    std::string expected = "WARNING: symbol number 4294967301 on line 2 in file " + root +
        "/e.rel does not belong to sort V\n";
    std::string cleanup = "rm -rf '" + root + "'";
    ENSURE(system(cleanup.c_str()) == 0);
    if (warnings.str() != expected) {
        std::cerr << "wpa_warning_number: got\n" << warnings.str() << "want\n" << expected;
        ENSURE(false);
    }
    std::cout << "wpa_warning_number: PASS\n";
#endif
}
