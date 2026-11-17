// External relation reference: Z3_fixedpoint_init with reduce/reduce_assign
// callbacks that model each relation term by an emptiness flag and log
// every call. Usage: ext <mode>
#include <stdio.h>
#include <string.h>
#include "z3.h"
Z3_context c; Z3_fixedpoint fp; Z3_sort B, V, I;
static char nonempty[1 << 20];
static int fresh = 0;
static const char* opname(Z3_decl_kind k) {
  switch (k) {
  case Z3_OP_RA_STORE: return "store"; case Z3_OP_RA_EMPTY: return "empty";
  case Z3_OP_RA_IS_EMPTY: return "is_empty"; case Z3_OP_RA_JOIN: return "join";
  case Z3_OP_RA_UNION: return "union"; case Z3_OP_RA_WIDEN: return "widen";
  case Z3_OP_RA_PROJECT: return "project"; case Z3_OP_RA_FILTER: return "filter";
  case Z3_OP_RA_NEGATION_FILTER: return "negation_filter"; case Z3_OP_RA_RENAME: return "rename";
  case Z3_OP_RA_COMPLEMENT: return "complement"; case Z3_OP_RA_SELECT: return "select";
  case Z3_OP_RA_CLONE: return "clone"; default: return "?";
  }
}
static void logdecl(const char* what, Z3_func_decl f, unsigned n, Z3_ast const a[]) {
  Z3_decl_kind k = Z3_get_decl_kind(c, f);
  printf("%s %s", what, opname(k));
  unsigned np = Z3_get_decl_num_parameters(c, f);
  for (unsigned i = 0; i < np; ++i) {
    switch (Z3_get_decl_parameter_kind(c, f, i)) {
    case Z3_PARAMETER_INT: printf(" %d", Z3_get_decl_int_parameter(c, f, i)); break;
    case Z3_PARAMETER_AST: printf(" [%s]", Z3_ast_to_string(c, Z3_get_decl_ast_parameter(c, f, i))); break;
    case Z3_PARAMETER_SORT: printf(" <%s>", Z3_sort_to_string(c, Z3_get_decl_sort_parameter(c, f, i))); break;
    default: printf(" ?"); break;
    }
  }
  printf(" |");
  for (unsigned i = 0; i < n; ++i) printf(" %s", Z3_ast_to_string(c, a[i]));
}
static void assign_cb(void* st, Z3_func_decl f, unsigned n, Z3_ast const a[], unsigned no, Z3_ast const o[]) {
  logdecl("assign", f, n, a);
  printf(" ->");
  for (unsigned i = 0; i < no; ++i) printf(" %s", Z3_ast_to_string(c, o[i]));
  printf("\n");
  unsigned o0 = Z3_get_ast_id(c, o[0]);
  switch (Z3_get_decl_kind(c, f)) {
  case Z3_OP_RA_EMPTY: nonempty[o0] = 0; break;
  case Z3_OP_RA_STORE: nonempty[o0] = 1; break;
  case Z3_OP_RA_CLONE: nonempty[o0] = nonempty[Z3_get_ast_id(c, a[0])]; break;
  case Z3_OP_RA_UNION: case Z3_OP_RA_WIDEN: {
    char was = nonempty[Z3_get_ast_id(c, a[0])], src = nonempty[Z3_get_ast_id(c, a[1])];
    nonempty[o0] = was || src;
    if (no == 2) nonempty[Z3_get_ast_id(c, o[1])] = src && !was;
    break;
  }
  default: break; // filters keep the flag
  }
}
static void app_cb(void* st, Z3_func_decl f, unsigned n, Z3_ast const a[], Z3_ast* r) {
  logdecl("reduce", f, n, a);
  Z3_decl_kind k = Z3_get_decl_kind(c, f);
  if (k == Z3_OP_RA_IS_EMPTY) {
    *r = nonempty[Z3_get_ast_id(c, a[0])] ? Z3_mk_false(c) : Z3_mk_true(c);
  } else if (k == Z3_OP_RA_SELECT) {
    *r = nonempty[Z3_get_ast_id(c, a[0])] ? Z3_mk_true(c) : Z3_mk_false(c);
  } else if (k == Z3_OP_RA_JOIN || k == Z3_OP_RA_PROJECT || k == Z3_OP_RA_RENAME || k == Z3_OP_RA_COMPLEMENT) {
    char name[32]; snprintf(name, sizeof name, "X%d", fresh++);
    *r = Z3_mk_const(c, Z3_mk_string_symbol(c, name), Z3_get_range(c, f));
    char v = k == Z3_OP_RA_COMPLEMENT ? 1 : nonempty[Z3_get_ast_id(c, a[0])];
    if (k == Z3_OP_RA_JOIN) v = v && nonempty[Z3_get_ast_id(c, a[1])];
    nonempty[Z3_get_ast_id(c, *r)] = v;
  } else {
    *r = 0; // fallback: mk_app
  }
  printf(" => %s\n", *r ? Z3_ast_to_string(c, *r) : "null");
}
static Z3_func_decl decl(const char* n, unsigned k, Z3_sort* s) {
  Z3_func_decl d = Z3_mk_func_decl(c, Z3_mk_string_symbol(c, n), k, s, B);
  Z3_fixedpoint_register_relation(c, fp, d);
  return d;
}
static void repr(Z3_func_decl d, int product) {
  Z3_symbol k[2] = {Z3_mk_string_symbol(c, "external_relation"), Z3_mk_string_symbol(c, "interval_relation")};
  Z3_fixedpoint_set_predicate_representation(c, fp, d, product ? 2 : 1, k);
  printf("repr err %d\n", Z3_get_error_code(c));
}
static void rule(Z3_ast body, Z3_ast head, unsigned nv, Z3_sort* s, const char* name) {
  Z3_symbol names[4];
  for (unsigned i = 0; i < nv; ++i) { char b[8]; snprintf(b, sizeof b, "v%u", i); names[i] = Z3_mk_string_symbol(c, b); }
  Z3_ast f = Z3_mk_implies(c, body, head);
  if (nv) f = Z3_mk_forall(c, 0, 0, 0, nv, s, names, f);
  Z3_fixedpoint_add_rule(c, fp, f, Z3_mk_string_symbol(c, name));
  printf("rule %s err %d\n", name, Z3_get_error_code(c));
}
static void query(Z3_func_decl d) {
  Z3_lbool r = Z3_fixedpoint_query_relations(c, fp, 1, &d);
  printf("result %d err %d\n", r, Z3_get_error_code(c));
  if (Z3_get_error_code(c) != Z3_OK) printf("msg %s\n", Z3_get_error_msg(c, Z3_get_error_code(c)));
  if (r == Z3_L_UNDEF && Z3_get_error_code(c) == Z3_OK) printf("reason %s\n", Z3_fixedpoint_get_reason_unknown(c, fp));
  else if (Z3_get_error_code(c) == Z3_OK) printf("answer %s\n", Z3_ast_to_string(c, Z3_fixedpoint_get_answer(c, fp)));
}
static Z3_ast app2(Z3_func_decl d, Z3_ast x, Z3_ast y) { Z3_ast a[2] = {x, y}; return Z3_mk_app(c, d, 2, a); }
static Z3_ast app1(Z3_func_decl d, Z3_ast x) { return Z3_mk_app(c, d, 1, &x); }
static Z3_ast and2(Z3_ast x, Z3_ast y) { Z3_ast a[2] = {x, y}; return Z3_mk_and(c, 2, a); }
static Z3_ast and3(Z3_ast x, Z3_ast y, Z3_ast z) { Z3_ast a[3] = {x, y, z}; return Z3_mk_and(c, 3, a); }
int main(int argc, char** argv) {
  int mode = argc > 1 ? argv[1][0] - '0' : 0;
  if (argc > 2) Z3_global_param_set("verbose", argv[2]);
  c = Z3_mk_context(Z3_mk_config()); fp = Z3_mk_fixedpoint(c); Z3_fixedpoint_inc_ref(c, fp);
  Z3_set_error_handler(c, 0);
  Z3_params p = Z3_mk_params(c); Z3_params_inc_ref(c, p);
  Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "engine"), Z3_mk_string_symbol(c, "datalog"));
  if (mode == 3 || mode == 6 || mode == 7) Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "datalog.default_relation"), Z3_mk_string_symbol(c, "external_relation"));
  Z3_fixedpoint_set_params(c, fp, p);
  Z3_fixedpoint_init(c, fp, 0);
  Z3_fixedpoint_set_reduce_assign_callback(c, fp, assign_cb);
  Z3_fixedpoint_set_reduce_app_callback(c, fp, app_cb);
  B = Z3_mk_bool_sort(c); V = Z3_mk_bv_sort(c, 2); I = Z3_mk_int_sort(c);
  if (mode == 6 || mode == 7) {
    Z3_func_decl Pd = decl("p", 1, &B), O = decl("other", 1, &B);
    Z3_symbol kh = Z3_mk_string_symbol(c, "tr_hashtable"), ks = Z3_mk_string_symbol(c, "tr_sparse");
    Z3_fixedpoint_set_predicate_representation(c, fp, Pd, 1, mode == 6 ? &kh : &ks);
    Z3_fixedpoint_set_predicate_representation(c, fp, O, 1, mode == 6 ? &ks : &kh);
    unsigned one = 1;
    Z3_fixedpoint_add_fact(c, fp, O, 1, &one);
    Z3_fixedpoint_add_fact(c, fp, Pd, 1, &one);
    printf("facts err %d\n", Z3_get_error_code(c));
    Z3_ast x = Z3_mk_bound(c, 0, B); Z3_symbol xs = Z3_mk_string_symbol(c, "x");
    Z3_ast q1 = Z3_mk_exists(c, 0, 0, 0, 1, &B, &xs, app1(Pd, x));
    Z3_ast q2 = Z3_mk_exists(c, 0, 0, 0, 1, &B, &xs, and2(app1(Pd, x), app1(O, x)));
    Z3_ast qs[3] = {q1, q2, app1(Pd, Z3_mk_true(c))};
    for (int i = 0; i < 3; i++) {
      Z3_lbool r = Z3_fixedpoint_query(c, fp, qs[i]);
      printf("result %d err %d\n", r, Z3_get_error_code(c));
      if (Z3_get_error_code(c) == Z3_OK && r != Z3_L_UNDEF) printf("answer %s\n", Z3_ast_to_string(c, Z3_fixedpoint_get_answer(c, fp)));
    }
    return 0;
  }
  if (mode == 5) {
    Z3_sort vv[2] = {V, V};
    Z3_func_decl E = decl("e", 2, vv), Q = decl("q", 1, vv), R = decl("r", 1, vv);
    repr(E, 0); repr(Q, 0); repr(R, 0);
    unsigned f00[2] = {0, 0}, f01[2] = {0, 1}, f22[2] = {2, 2};
    Z3_fixedpoint_add_fact(c, fp, E, 2, f00);
    Z3_fixedpoint_add_fact(c, fp, E, 2, f01);
    Z3_fixedpoint_add_fact(c, fp, E, 2, f22);
    printf("facts err %d\n", Z3_get_error_code(c));
    Z3_ast x = Z3_mk_bound(c, 0, V);
    rule(app2(E, x, x), app1(Q, x), 1, vv, "diag");
    rule(app2(E, Z3_mk_numeral(c, "0", V), x), app1(R, x), 1, vv, "row");
    query(Q); query(R);
    return 0;
  }
  if (mode == 0 || mode == 2 || mode == 3) {
    Z3_sort vv[2] = {V, V}, vvv[3] = {V, V, V};
    Z3_func_decl E = decl("e", 2, vv), P = decl("p", 2, vv), C = decl("c", 1, vv);
    if (mode != 3) { repr(E, 0); repr(P, 0); repr(C, 0); }
    unsigned f01[2] = {0, 1}, f12[2] = {1, 2};
    Z3_fixedpoint_add_fact(c, fp, E, 2, f01);
    Z3_fixedpoint_add_fact(c, fp, E, 2, f12);
    printf("facts err %d\n", Z3_get_error_code(c));
    Z3_ast x = Z3_mk_bound(c, 2, V), y = Z3_mk_bound(c, 1, V), z = Z3_mk_bound(c, 0, V);
    rule(app2(E, x, y), app2(P, x, y), 3, vvv, "base");
    rule(and2(app2(P, x, y), app2(E, y, z)), app2(P, x, z), 3, vvv, "step");
    if (mode == 2) rule(and2(app2(E, x, y), Z3_mk_not(c, app2(P, y, x))), app1(C, x), 3, vvv, "neg");
    if (mode == 2) query(C); else query(P);
    return 0;
  }
  if (mode == 1 || mode == 4) {
    Z3_sort ii[2] = {I, I}, iv[2] = {I, V};
    Z3_func_decl A = decl("a", 1, &I), Bd = decl("b", 1, &V), D = decl("d", 2, ii);
    repr(A, 1); repr(Bd, 0); repr(D, 1);
    unsigned f3 = 3, f5 = 5, d34[2] = {3, 4}, d51[2] = {5, 1};
    Z3_fixedpoint_add_fact(c, fp, A, 1, &f3);
    Z3_fixedpoint_add_fact(c, fp, A, 1, &f5);
    Z3_fixedpoint_add_fact(c, fp, D, 2, d34);
    Z3_fixedpoint_add_fact(c, fp, D, 2, d51);
    printf("facts err %d\n", Z3_get_error_code(c));
    Z3_ast x = Z3_mk_bound(c, 1, I), z = Z3_mk_bound(c, 0, I), xi = Z3_mk_bound(c, 0, I);
    Z3_ast one = Z3_mk_numeral(c, "1", V), two = Z3_mk_numeral(c, "2", V);
    if (mode == 1) {
      rule(and2(app1(A, xi), Z3_mk_gt(c, xi, Z3_mk_int(c, 4, I))), app1(Bd, one), 1, &I, "hi");
      rule(and2(app1(A, xi), Z3_mk_lt(c, xi, Z3_mk_int(c, 3, I))), app1(Bd, two), 1, &I, "lo");
    } else {
      rule(and3(app1(A, x), app2(D, x, z), Z3_mk_gt(c, z, x)), app1(Bd, one), 2, ii, "up");
      rule(and3(app1(A, x), app2(D, x, z), Z3_mk_lt(c, z, x)), app1(Bd, two), 2, ii, "down");
    }
    (void)iv;
    query(Bd); query(A); query(D);
    return 0;
  }
  return 0;
}
