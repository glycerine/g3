// datalog.generate_explanations through Z3_fixedpoint_query (rel_context::query(expr*)).
// usage: expl <mode>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "z3.h"

static Z3_context c;
static Z3_fixedpoint fp;

static void err_handler(Z3_context ctx, Z3_error_code e) {
  printf("error %d: %s\n", e, Z3_get_error_msg(ctx, e));
}

static Z3_ast app2(Z3_func_decl f, Z3_ast a, Z3_ast b) { Z3_ast args[2] = {a, b}; return Z3_mk_app(c, f, 2, args); }
static Z3_ast app1(Z3_func_decl f, Z3_ast a) { return Z3_mk_app(c, f, 1, &a); }

static void rule(Z3_ast body, const char* name, unsigned nv, Z3_sort* sorts) {
  Z3_symbol names[8];
  for (unsigned i = 0; i < nv; i++) { char b[8]; sprintf(b, "v%u", i); names[i] = Z3_mk_string_symbol(c, b); }
  Z3_ast r = nv ? Z3_mk_forall(c, 0, 0, 0, nv, sorts, names, body) : body;
  Z3_fixedpoint_add_rule(c, fp, r, name ? Z3_mk_string_symbol(c, name) : 0);
}

static void query(Z3_ast q) {
  Z3_lbool r = Z3_fixedpoint_query(c, fp, q);
  printf("result %d\n", r);
  if (r != Z3_L_UNDEF) {
    printf("%s\n", Z3_ast_to_string(c, Z3_fixedpoint_get_answer(c, fp)));
  } else {
    printf("reason %s\n", Z3_fixedpoint_get_reason_unknown(c, fp));
  }
}

int main(int argc, char** argv) {
  int mode = argc > 1 ? atoi(argv[1]) : 0;
  if (argc > 2) Z3_global_param_set("verbose", argv[2]);
  Z3_config cfg = Z3_mk_config();
  c = Z3_mk_context(cfg);
  Z3_set_error_handler(c, err_handler);
  fp = Z3_mk_fixedpoint(c); Z3_fixedpoint_inc_ref(c, fp);
  Z3_params p = Z3_mk_params(c); Z3_params_inc_ref(c, p);
  Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "engine"), Z3_mk_string_symbol(c, "datalog"));
  Z3_params_set_bool(c, p, Z3_mk_string_symbol(c, "datalog.generate_explanations"), 1);
  if (mode == 1 || mode == 5) Z3_params_set_bool(c, p, Z3_mk_string_symbol(c, "datalog.explanations_on_relation_level"), 1);
  if (mode == 2) Z3_params_set_bool(c, p, Z3_mk_string_symbol(c, "datalog.magic_sets_for_queries"), 1);
  if (mode == 6) Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "datalog.default_table"), Z3_mk_string_symbol(c, "hashtable"));
  if (mode == 8) Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "datalog.default_relation"), Z3_mk_string_symbol(c, "doc"));
  if (mode == 9) Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "datalog.check_relation"), Z3_mk_string_symbol(c, "sparse"));
  if (mode == 11 || mode == 12) Z3_params_set_bool(c, p, Z3_mk_string_symbol(c, "datalog.explanations_on_relation_level"), 1);
  if (mode == 12) Z3_params_set_bool(c, p, Z3_mk_string_symbol(c, "datalog.magic_sets_for_queries"), 1);
  if (mode == 14) Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "datalog.default_table"), Z3_mk_string_symbol(c, "bitvector"));
  if (mode == 15) Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "datalog.default_table"), Z3_mk_string_symbol(c, "lazy_sparse"));
  if (mode == 16) Z3_params_set_bool(c, p, Z3_mk_string_symbol(c, "xform.inline_eager"), 0);
  Z3_fixedpoint_set_params(c, fp, p);
  Z3_sort S = Z3_mk_finite_domain_sort(c, Z3_mk_string_symbol(c, "S"), 4);
  Z3_sort B = Z3_mk_bool_sort(c);
  Z3_sort SS[2] = {S, S};
  Z3_func_decl e = Z3_mk_func_decl(c, Z3_mk_string_symbol(c, "e"), 2, SS, B);
  Z3_func_decl path = Z3_mk_func_decl(c, Z3_mk_string_symbol(c, "path"), 2, SS, B);
  Z3_func_decl node = Z3_mk_func_decl(c, Z3_mk_string_symbol(c, "node"), 1, SS, B);
  Z3_func_decl bad = Z3_mk_func_decl(c, Z3_mk_string_symbol(c, "bad"), 1, SS, B);
  Z3_fixedpoint_register_relation(c, fp, e);
  Z3_fixedpoint_register_relation(c, fp, path);
  Z3_fixedpoint_register_relation(c, fp, node);
  Z3_fixedpoint_register_relation(c, fp, bad);
  unsigned f01[2] = {0, 1}, f12[2] = {1, 2}, f23[2] = {2, 3};
  Z3_fixedpoint_add_fact(c, fp, e, 2, f01);
  Z3_fixedpoint_add_fact(c, fp, e, 2, f12);
  if (mode != 4) Z3_fixedpoint_add_fact(c, fp, e, 2, f23);
  unsigned b2[1] = {2};
  Z3_fixedpoint_add_fact(c, fp, bad, 1, b2);
  Z3_ast x = Z3_mk_bound(c, 0, S), y = Z3_mk_bound(c, 1, S), z = Z3_mk_bound(c, 2, S);
  Z3_sort S3[3] = {S, S, S};
  rule(Z3_mk_implies(c, app2(e, x, y), app2(path, x, y)), "base", 2, S3);
  Z3_ast t2[2] = {app2(path, x, y), app2(e, y, z)};
  rule(Z3_mk_implies(c, Z3_mk_and(c, 2, t2), app2(path, x, z)), mode == 3 ? 0 : "step", 3, S3);
  if (mode == 3 || mode == 7 || mode == 11) {
    Z3_ast t3[2] = {app2(e, x, y), Z3_mk_not(c, app1(bad, x))};
    rule(Z3_mk_implies(c, Z3_mk_and(c, 2, t3), app1(node, x)), 0, 2, S3);
  }
  if (mode == 4) {
    Z3_ast t4[2] = {app2(path, x, y), Z3_mk_not(c, Z3_mk_eq(c, x, y))};
    rule(Z3_mk_implies(c, Z3_mk_and(c, 2, t4), app1(node, y)), "diff", 2, S3);
  }
  Z3_ast q;
  unsigned u0 = 0, u3 = 3;
  Z3_ast c0 = Z3_mk_numeral(c, "0", S), c3 = Z3_mk_numeral(c, "3", S);
  (void)u0; (void)u3;
  switch (mode) {
  case 3: case 7: case 11: q = app1(node, x); break;
  case 13: q = app2(path, c0, x); break;
  case 4: q = app1(node, x); break;
  case 5: q = app2(path, c0, c3); break;
  default: q = app2(path, x, y); break;
  }
  if (mode == 0 || mode == 1 || mode == 2 || mode == 6 || mode == 5 || mode == 8 || mode == 9 || mode == 12 || mode == 14 || mode == 15 || mode == 16) {
    // the query's free variables are existentially quantified
    Z3_symbol nm[2] = {Z3_mk_string_symbol(c, "a"), Z3_mk_string_symbol(c, "b")};
    if (mode != 5) q = Z3_mk_exists(c, 0, 0, 0, 2, SS, nm, q);
  } else {
    Z3_symbol nm[1] = {Z3_mk_string_symbol(c, "a")};
    q = Z3_mk_exists(c, 0, 0, 0, 1, SS, nm, q);
  }
  query(q);
  if (mode == 0) {
    // second query on the same context
    query(app2(path, c0, c3));
  }
  return 0;
}
