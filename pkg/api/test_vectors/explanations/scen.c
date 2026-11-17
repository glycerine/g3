// generate_explanations scenarios (configurations of the g3 fp tests),
// through Z3_fixedpoint_query. usage: scen <scenario> [verbose]
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "z3.h"

static Z3_context c;
static Z3_fixedpoint fp;
static Z3_params prm;
static Z3_sort B;

static void err_handler(Z3_context ctx, Z3_error_code e) { printf("error %d: %s\n", e, Z3_get_error_msg(ctx, e)); }
static void setb(const char* k, int v) { Z3_params_set_bool(c, prm, Z3_mk_string_symbol(c, k), v); }
static void sets(const char* k, const char* v) { Z3_params_set_symbol(c, prm, Z3_mk_string_symbol(c, k), Z3_mk_string_symbol(c, v)); }
static Z3_func_decl decl(const char* n, unsigned arity, Z3_sort* dom) {
  Z3_func_decl d = Z3_mk_func_decl(c, Z3_mk_string_symbol(c, n), arity, dom, B);
  Z3_fixedpoint_register_relation(c, fp, d);
  return d;
}
static void repr(Z3_func_decl d, const char* k) { Z3_symbol s = Z3_mk_string_symbol(c, k); Z3_fixedpoint_set_predicate_representation(c, fp, d, 1, &s); }
static void rule(Z3_ast body, const char* name) { Z3_fixedpoint_add_rule(c, fp, body, Z3_mk_string_symbol(c, name)); }
static Z3_ast forall1(Z3_sort s, Z3_ast body) { Z3_symbol x = Z3_mk_string_symbol(c, "x"); return Z3_mk_forall(c, 0, 0, 0, 1, &s, &x, body); }
static Z3_ast forall2(Z3_sort s, Z3_ast body) { Z3_sort ss[2] = {s, s}; Z3_symbol xs[2] = {Z3_mk_string_symbol(c, "x"), Z3_mk_string_symbol(c, "y")}; return Z3_mk_forall(c, 0, 0, 0, 2, ss, xs, body); }
static Z3_ast and2(Z3_ast a, Z3_ast b) { Z3_ast t[2] = {a, b}; return Z3_mk_and(c, 2, t); }
static Z3_ast app0(Z3_func_decl f) { return Z3_mk_app(c, f, 0, 0); }
static Z3_ast app1(Z3_func_decl f, Z3_ast a) { return Z3_mk_app(c, f, 1, &a); }
static Z3_ast app2(Z3_func_decl f, Z3_ast a, Z3_ast b) { Z3_ast t[2] = {a, b}; return Z3_mk_app(c, f, 2, t); }
static void fact1(Z3_func_decl f, unsigned v) { Z3_fixedpoint_add_fact(c, fp, f, 1, &v); }
static void fact0(Z3_func_decl f) { Z3_fixedpoint_add_fact(c, fp, f, 0, 0); }
static void fact2(Z3_func_decl f, unsigned a, unsigned b) { unsigned v[2] = {a, b}; Z3_fixedpoint_add_fact(c, fp, f, 2, v); }
static void query(Z3_ast q) {
  Z3_lbool r = Z3_fixedpoint_query(c, fp, q);
  printf("result %d\n", r);
  if (r != Z3_L_UNDEF) printf("%s\n", Z3_ast_to_string(c, Z3_fixedpoint_get_answer(c, fp)));
  else printf("reason %s\n", Z3_fixedpoint_get_reason_unknown(c, fp));
}

int main(int argc, char** argv) {
  int s = argc > 1 ? atoi(argv[1]) : 0;
  if (argc > 2) Z3_global_param_set("verbose", argv[2]);
  c = Z3_mk_context(Z3_mk_config());
  Z3_set_error_handler(c, err_handler);
  fp = Z3_mk_fixedpoint(c); Z3_fixedpoint_inc_ref(c, fp);
  prm = Z3_mk_params(c); Z3_params_inc_ref(c, prm);
  B = Z3_mk_bool_sort(c);
  Z3_ast T = Z3_mk_true(c), F = Z3_mk_false(c);
  Z3_ast x = Z3_mk_bound(c, 0, B), y = Z3_mk_bound(c, 1, B);
  sets("engine", "datalog");
  setb("datalog.generate_explanations", 1);
  printf("== scenario %d\n", s);
  if (s >= 1 && s <= 3) {
    // explicit table representation of a negated predicate, relation level, check_relation hashtable
    setb("datalog.explanations_on_relation_level", 1);
    sets("datalog.check_relation", "hashtable");
    setb("datalog.unbound_compressor", 0);
    Z3_fixedpoint_set_params(c, fp, prm);
    Z3_func_decl p = decl("p", 1, &B), q = decl("q", 1, &B);
    Z3_func_decl n = s == 3 ? decl("n", 0, 0) : decl("n", 1, &B);
    repr(n, s == 2 ? "tr_sparse" : "tr_hashtable");
    fact1(p, 1); fact1(p, 0);
    if (s != 3) fact1(n, 1);
    Z3_ast na = s == 3 ? app0(n) : app1(n, x);
    rule(forall1(B, Z3_mk_implies(c, and2(app1(p, x), Z3_mk_not(c, na)), app1(q, x))), "negproof");
    query(app1(q, s == 3 ? T : F));
  } else if (s == 4) {
    // fact level, magic sets, hashtable default table, explicit sparse predicate
    setb("datalog.magic_sets_for_queries", 1);
    sets("datalog.default_table", "hashtable");
    Z3_fixedpoint_set_params(c, fp, prm);
    Z3_func_decl seed = decl("seed", 1, &B), pred = decl("pred", 1, &B);
    repr(pred, "tr_sparse");
    fact1(seed, 1);
    rule(forall1(B, Z3_mk_implies(c, app1(seed, x), app1(pred, x))), "step");
    query(app1(pred, T));
  } else if (s >= 5 && s <= 8) {
    // nullary p, u, q: q :- p, not u; magic (s 6, 8); u derived (s 7, 8)
    int magic = s == 6 || s == 8, derived = s >= 7;
    setb("datalog.magic_sets_for_queries", magic);
    Z3_fixedpoint_set_params(c, fp, prm);
    Z3_func_decl p = decl("p", 0, 0), q = decl("q", 0, 0), u = decl("u", 0, 0);
    fact0(p);
    if (derived) rule(Z3_mk_implies(c, app0(p), app0(u)), "producer");
    rule(Z3_mk_implies(c, and2(app0(p), Z3_mk_not(c, app0(u))), app0(q)), "consumer");
    query(app0(q));
    if (!derived) { fact0(u); query(app0(q)); }
  } else if (s == 9 || s == 10) {
    // relation level, magic, check_relation check, checked default table
    setb("datalog.explanations_on_relation_level", 1);
    setb("datalog.magic_sets_for_queries", s == 9);
    sets("datalog.check_relation", "check");
    if (s == 9) {
      sets("datalog.default_table", "hashtable");
      setb("datalog.default_table_checked", 1);
      sets("datalog.default_table_checker", "sparse");
      setb("datalog.unbound_compressor", 0);
    }
    Z3_fixedpoint_set_params(c, fp, prm);
    if (s == 9) {
      Z3_sort BB[2] = {B, B};
      Z3_func_decl seed = decl("seed", 1, &B), edge = decl("edge", 2, BB), reach = decl("reach", 1, &B), goal = decl("goal", 1, &B);
      repr(seed, "tr_sparse"); repr(edge, "tr_hashtable");
      fact1(seed, 0);
      fact2(edge, 0, 1);
      rule(forall1(B, Z3_mk_implies(c, app1(seed, x), app1(reach, x))), "base");
      rule(forall2(B, Z3_mk_implies(c, and2(app1(reach, x), app2(edge, x, y)), app1(reach, y))), "recursive");
      rule(forall1(B, Z3_mk_implies(c, app1(reach, x), app1(goal, x))), "goalrule");
      query(app1(goal, T));
    } else {
      Z3_func_decl p = decl("p", 0, 0);
      fact0(p);
      query(app0(p));
    }
  } else if (s >= 11 && s <= 22) {
    // P(b) fact P(true) (not for 22), R :- P (forward), P :- R (backward); query R(true)
    int k = s - 11;
    const char* mode[12] = {"enabled", "magic", "relation-level", "checked", "relation-checked", "relation-checked-sparse",
                            "relation-checked-explicit", "checked-default", "checked-default-reverse", "relation-checked-default", "checked-default-lazy", "empty"};
    printf("mode %s\n", mode[k]);
    if (k == 1) setb("datalog.magic_sets_for_queries", 1);
    if (k == 3 || k == 5) sets("datalog.check_relation", "sparse");
    if (k == 4 || k == 6) sets("datalog.check_relation", "hashtable");
    if (k == 2 || k == 4 || k == 5 || k == 6 || k == 9) setb("datalog.explanations_on_relation_level", 1);
    if (k == 7 || k == 9) { setb("datalog.default_table_checked", 1); sets("datalog.default_table_checker", "hashtable"); }
    if (k == 8) { sets("datalog.default_table", "hashtable"); setb("datalog.default_table_checked", 1); sets("datalog.default_table_checker", "sparse"); }
    if (k == 10) { sets("datalog.default_table", "lazy_sparse"); setb("datalog.default_table_checked", 1); sets("datalog.default_table_checker", "hashtable"); }
    Z3_fixedpoint_set_params(c, fp, prm);
    Z3_func_decl P = decl("P", 1, &B), R = decl("R", 1, &B);
    if (k == 6) repr(R, "tr_sparse");
    if (k != 11) fact1(P, 1);
    rule(forall1(B, Z3_mk_implies(c, app1(P, x), app1(R, x))), "forward");
    rule(forall1(B, Z3_mk_implies(c, app1(R, x), app1(P, x))), "backward");
    query(app1(R, T));
  } else if (s == 23 || s == 24) {
    // relation level with doc: check_relation doc (24) or default_relation doc (23); bit-vector columns
    setb("datalog.explanations_on_relation_level", 1);
    if (s == 24) sets("datalog.check_relation", "doc"); else sets("datalog.default_relation", "doc");
    Z3_fixedpoint_set_params(c, fp, prm);
    Z3_sort bv = Z3_mk_bv_sort(c, 3);
    Z3_func_decl p = decl("pl", 1, &bv), q = decl("qd", 1, &bv), r = decl("rr", 1, &bv), bs = decl("bs", 1, &B);
    Z3_ast v = Z3_mk_unsigned_int(c, 5, bv), xb = Z3_mk_bound(c, 0, bv);
    fact1(p, 5); fact1(r, 5); fact1(bs, 1);
    Z3_ast body[4] = {app1(p, xb), app1(r, xb), app1(bs, T), Z3_mk_eq(c, xb, v)};
    rule(forall1(bv, Z3_mk_implies(c, Z3_mk_and(c, 4, body), app1(q, xb))), "joinproject");
    query(app1(q, v));
  }
  return 0;
}
