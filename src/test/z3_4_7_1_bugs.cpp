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
#include "api/z3.h"
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
