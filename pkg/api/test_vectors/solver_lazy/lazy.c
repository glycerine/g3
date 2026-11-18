// Z3_mk_solver builds no solver: init_solver does, on first use, after the
// caller's own terms (the AST ids goivy's checks depend on).
// usage: lazy <mode>
//   0: mk_solver, parse, set_params, push/assert/check/pop, check
//   1: mk_solver, get_param_descrs (built and dropped), parse, assert,
//      check, reset, assert, check
//   2: mk_solver_for_logic and mk_simple_solver, each parse/assert, check
//      s, check t, num_scopes; t is m_last_obj, so its dec_ref leaves it
//      alive (with its smt context) until the context dies
//   3: parse a text with an unused declaration twice (the parse's
//      cmd_context frees it each time)
//   4: mk_solver_for_logic(UFLIA), parse/assert, check (the tactic
//      solver's simplify/arith rewriter path)
#include <stdio.h>
#include <stdlib.h>
#include "z3.h"

static Z3_context c;
static const char* base;
static const char* goal;
// Each parse declares only what it uses (an unused declaration is freed
// when the parse ends; that is not what these cases test).
static Z3_ast parse(const char* body) {
  char buf[1024];
  const char* decls = body == goal ? "(declare-fun f (Int) Int) (declare-fun a () Int) " : "(declare-fun f (Int) Int) ";
  snprintf(buf, sizeof buf, "%s%s", decls, body);
  return Z3_parse_smtlib2_string(c, buf, 0, 0, 0, 0, 0, 0);
}
static const char* base_text = "(assert (forall ((x Int)) (! (= (f x) (+ x 1)) :pattern ((f x)))))";
static const char* goal_text = "(assert (= (f a) a))";

int main(int argc, char** argv) {
  int mode = argc > 1 ? atoi(argv[1]) : 0;
  base = base_text;
  goal = goal_text;
  c = Z3_mk_context(Z3_mk_config());
  if (mode == 0) {
    Z3_solver s = Z3_mk_solver(c);
    Z3_solver_inc_ref(c, s);
    Z3_ast b = parse(base);
    Z3_params p = Z3_mk_params(c);
    Z3_params_inc_ref(c, p);
    Z3_params_set_bool(c, p, Z3_mk_string_symbol(c, "smt.macro_finder"), 1);
    Z3_solver_set_params(c, s, p);
    Z3_params_dec_ref(c, p);
    Z3_solver_assert(c, s, b);
    Z3_solver_push(c, s);
    Z3_solver_assert(c, s, parse(goal));
    printf("%d\n", Z3_solver_check(c, s));
    Z3_solver_pop(c, s, 1);
    printf("%d\n", Z3_solver_check(c, s));
    Z3_solver_dec_ref(c, s);
  } else if (mode == 1) {
    Z3_solver s = Z3_mk_solver(c);
    Z3_solver_inc_ref(c, s);
    Z3_param_descrs d = Z3_solver_get_param_descrs(c, s);
    Z3_param_descrs_inc_ref(c, d);
    Z3_param_descrs_dec_ref(c, d);
    Z3_solver_assert(c, s, parse(base));
    printf("%d\n", Z3_solver_check(c, s));
    Z3_solver_reset(c, s);
    Z3_solver_assert(c, s, parse(goal));
    printf("%d\n", Z3_solver_check(c, s));
    Z3_solver_dec_ref(c, s);
  } else if (mode == 3) {
    const char* text = "(declare-fun f (Int) Int) (declare-fun a () Int) (assert (forall ((x Int)) (= (f x) (+ x 1))))";
    Z3_parse_smtlib2_string(c, text, 0, 0, 0, 0, 0, 0);
    Z3_parse_smtlib2_string(c, text, 0, 0, 0, 0, 0, 0);
    printf("ok\n");
  } else if (mode == 4) {
    Z3_solver s = Z3_mk_solver_for_logic(c, Z3_mk_string_symbol(c, "UFLIA"));
    Z3_solver_inc_ref(c, s);
    Z3_solver_assert(c, s, parse(base));
    printf("%d\n", Z3_solver_check(c, s));
    Z3_solver_dec_ref(c, s);
  } else {
    Z3_solver s = Z3_mk_solver_for_logic(c, Z3_mk_string_symbol(c, "UFLIA"));
    Z3_solver_inc_ref(c, s);
    Z3_solver t = Z3_mk_simple_solver(c);
    Z3_solver_inc_ref(c, t);
    Z3_solver_assert(c, s, parse(base));
    Z3_solver_assert(c, t, parse(goal));
    // Sequenced: GCC evaluates printf's arguments right to left.
    int rs = Z3_solver_check(c, s);
    int rt = Z3_solver_check(c, t);
    printf("%d %d\n", rs, rt);
    printf("%u %u\n", Z3_solver_get_num_scopes(c, s), Z3_solver_get_num_scopes(c, t));
    Z3_solver_dec_ref(c, t);
    Z3_solver_dec_ref(c, s);
  }
  return 0;
}
