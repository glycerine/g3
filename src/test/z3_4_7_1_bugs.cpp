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

    Run: ./test-z3 z3_4_7_1_dom_simplify_root   (etc.)
--*/

#include "ast/reg_decl_plugins.h"
#include "ast/ast_pp.h"
#include "ast/arith_decl_plugin.h"
#include "ast/pb_decl_plugin.h"
#include "ast/datatype_decl_plugin.h"
#include "smt/smt_kernel.h"
#include "smt/params/smt_params.h"
#include "tactic/tactic.h"
#include "tactic/goal.h"
#include "tactic/core/dom_simplify_tactic.h"
#include "tactic/core/pb_preprocess_tactic.h"
#include <iostream>
#ifndef _WINDOWS
#include <csignal>
#include <unistd.h>
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
