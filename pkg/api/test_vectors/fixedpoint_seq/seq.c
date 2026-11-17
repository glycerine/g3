// Z3_fixedpoint_* call sequences read from a file. usage: seq <file>
// Lines (tokens separated by blanks; expressions are s-expressions):
//   sort S bv W | sort S fin N | sort S int | sort S bool
//   vars x S y S ...         bound variables, index = position
//   rel R S...               Z3_mk_func_decl + Z3_fixedpoint_register_relation
//   param K V                bool (true/false), unsigned (digits) or symbol
//   setparams                Z3_fixedpoint_set_params (also mid-session)
//   repr R KIND              Z3_fixedpoint_set_predicate_representation
//   fact R v...              Z3_fixedpoint_add_fact
//   rule NAME E              Z3_fixedpoint_add_rule of (forall vars E) when vars exist
//   query E                  Z3_fixedpoint_query, then the answer or reason
//   qrel R...                Z3_fixedpoint_query_relations, then the answer
//   push | pop
//   cover L R E              Z3_fixedpoint_add_cover
//   delta L R                Z3_fixedpoint_get_cover_delta
//   levels R                 Z3_fixedpoint_get_num_levels
// Expressions: (and E...) (or E...) (not E) (=> E E) (= E E) (R E...),
// a variable name, true, false, or V:S (Z3_mk_unsigned_int of sort S).
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include "z3.h"

static Z3_context c;
static Z3_fixedpoint fp;
static Z3_params prm;
#define MAXN 64
static char sname[MAXN][32]; static Z3_sort sorts[MAXN]; static int nsorts;
static char vname[MAXN][32]; static Z3_sort vsort[MAXN]; static int nvars;
static char rname[MAXN][32]; static Z3_func_decl rels[MAXN]; static int nrels;

static void err_handler(Z3_context ctx, Z3_error_code e) { printf("error: %s\n", Z3_get_error_msg(ctx, e)); }
static Z3_sort find_sort(const char* n) { for (int i = 0; i < nsorts; i++) if (!strcmp(sname[i], n)) return sorts[i]; fprintf(stderr, "no sort %s\n", n); exit(2); }
static Z3_func_decl find_rel(const char* n) { for (int i = 0; i < nrels; i++) if (!strcmp(rname[i], n)) return rels[i]; fprintf(stderr, "no rel %s\n", n); exit(2); }
static int find_var(const char* n) { for (int i = 0; i < nvars; i++) if (!strcmp(vname[i], n)) return i; return -1; }

static const char* p;
static void skip(void) { while (*p == ' ' || *p == '\t') p++; }
static void token(char* out) { skip(); int k = 0; while (*p && *p != ' ' && *p != '\t' && *p != '(' && *p != ')' && *p != '\n') out[k++] = *p++; out[k] = 0; }
static Z3_ast expr(void) {
  skip();
  if (*p == '(') {
    p++;
    char op[64]; token(op);
    Z3_ast args[MAXN]; int n = 0;
    skip();
    while (*p != ')') { args[n++] = expr(); skip(); }
    p++;
    if (!strcmp(op, "and")) return Z3_mk_and(c, n, args);
    if (!strcmp(op, "or")) return Z3_mk_or(c, n, args);
    if (!strcmp(op, "not")) return Z3_mk_not(c, args[0]);
    if (!strcmp(op, "=>")) return Z3_mk_implies(c, args[0], args[1]);
    if (!strcmp(op, "=")) return Z3_mk_eq(c, args[0], args[1]);
    return Z3_mk_app(c, find_rel(op), n, args);
  }
  char t[64]; token(t);
  if (!strcmp(t, "true")) return Z3_mk_true(c);
  if (!strcmp(t, "false")) return Z3_mk_false(c);
  char* colon = strchr(t, ':');
  if (colon) { *colon = 0; return Z3_mk_unsigned_int(c, (unsigned)strtoul(t, 0, 10), find_sort(colon + 1)); }
  int v = find_var(t);
  if (v >= 0) return Z3_mk_bound(c, v, vsort[v]);
  return Z3_mk_app(c, find_rel(t), 0, 0);
}
static void answer(void) { Z3_ast a = Z3_fixedpoint_get_answer(c, fp); printf("%s\n", a ? Z3_ast_to_string(c, a) : "null"); }
static void result(Z3_lbool r) {
  printf("result %d\n", r);
  if (r != Z3_L_UNDEF) answer();
  else printf("reason %s\n", Z3_fixedpoint_get_reason_unknown(c, fp));
}

int main(int argc, char** argv) {
  setvbuf(stdout, 0, _IOLBF, 0);
  FILE* f = fopen(argv[1], "r");
  if (!f) return 2;
  c = Z3_mk_context(Z3_mk_config());
  Z3_set_error_handler(c, err_handler);
  fp = Z3_mk_fixedpoint(c); Z3_fixedpoint_inc_ref(c, fp);
  prm = Z3_mk_params(c); Z3_params_inc_ref(c, prm);
  char line[4096];
  while (fgets(line, sizeof line, f)) {
    p = line;
    char cmd[64]; token(cmd);
    if (!cmd[0] || cmd[0] == ';') continue;
    printf("> %s", line);
    if (!strcmp(cmd, "sort")) {
      char n[32], k[32], a[32]; token(n); token(k); token(a);
      Z3_sort s;
      if (!strcmp(k, "bv")) s = Z3_mk_bv_sort(c, atoi(a));
      else if (!strcmp(k, "fin")) s = Z3_mk_finite_domain_sort(c, Z3_mk_string_symbol(c, n), strtoull(a, 0, 10));
      else if (!strcmp(k, "int")) s = Z3_mk_int_sort(c);
      else s = Z3_mk_bool_sort(c);
      strcpy(sname[nsorts], n); sorts[nsorts++] = s;
    } else if (!strcmp(cmd, "vars")) {
      char n[32], s[32];
      for (;;) { token(n); if (!n[0]) break; token(s); strcpy(vname[nvars], n); vsort[nvars++] = find_sort(s); }
    } else if (!strcmp(cmd, "rel")) {
      char n[32], s[32]; Z3_sort dom[MAXN]; int k = 0;
      token(n);
      for (;;) { token(s); if (!s[0]) break; dom[k++] = find_sort(s); }
      Z3_func_decl d = Z3_mk_func_decl(c, Z3_mk_string_symbol(c, n), k, dom, Z3_mk_bool_sort(c));
      Z3_fixedpoint_register_relation(c, fp, d);
      strcpy(rname[nrels], n); rels[nrels++] = d;
    } else if (!strcmp(cmd, "param")) {
      char k[64], v[64]; token(k); token(v);
      Z3_symbol ks = Z3_mk_string_symbol(c, k);
      if (!strcmp(v, "true") || !strcmp(v, "false")) Z3_params_set_bool(c, prm, ks, !strcmp(v, "true"));
      else if (isdigit((unsigned char)v[0])) Z3_params_set_uint(c, prm, ks, (unsigned)strtoul(v, 0, 10));
      else Z3_params_set_symbol(c, prm, ks, Z3_mk_string_symbol(c, v));
    } else if (!strcmp(cmd, "setparams")) {
      Z3_fixedpoint_set_params(c, fp, prm);
    } else if (!strcmp(cmd, "repr")) {
      char n[32], k[32]; token(n); token(k);
      Z3_symbol s = Z3_mk_string_symbol(c, k);
      Z3_fixedpoint_set_predicate_representation(c, fp, find_rel(n), 1, &s);
    } else if (!strcmp(cmd, "fact")) {
      char n[32], v[32]; unsigned vals[MAXN]; int k = 0;
      token(n);
      for (;;) { token(v); if (!v[0]) break; vals[k++] = (unsigned)strtoul(v, 0, 10); }
      Z3_fixedpoint_add_fact(c, fp, find_rel(n), k, vals);
    } else if (!strcmp(cmd, "rule")) {
      char n[32]; token(n);
      Z3_ast body = expr();
      if (nvars) {
        Z3_symbol names[MAXN];
        for (int i = 0; i < nvars; i++) names[i] = Z3_mk_string_symbol(c, vname[i]);
        body = Z3_mk_forall(c, 0, 0, 0, nvars, vsort, names, body);
      }
      Z3_fixedpoint_add_rule(c, fp, body, Z3_mk_string_symbol(c, n));
    } else if (!strcmp(cmd, "query")) {
      Z3_ast q = expr();
      result(Z3_fixedpoint_query(c, fp, q));
    } else if (!strcmp(cmd, "qrel")) {
      char n[32]; Z3_func_decl ds[MAXN]; int k = 0;
      for (;;) { token(n); if (!n[0]) break; ds[k++] = find_rel(n); }
      result(Z3_fixedpoint_query_relations(c, fp, k, ds));
    } else if (!strcmp(cmd, "push")) {
      Z3_fixedpoint_push(c, fp);
    } else if (!strcmp(cmd, "pop")) {
      Z3_fixedpoint_pop(c, fp);
    } else if (!strcmp(cmd, "cover")) {
      char l[32], n[32]; token(l); token(n);
      Z3_ast e = expr();
      Z3_fixedpoint_add_cover(c, fp, atoi(l), find_rel(n), e);
    } else if (!strcmp(cmd, "delta")) {
      char l[32], n[32]; token(l); token(n);
      Z3_ast d = Z3_fixedpoint_get_cover_delta(c, fp, atoi(l), find_rel(n));
      printf("%s\n", d ? Z3_ast_to_string(c, d) : "null");
    } else if (!strcmp(cmd, "levels")) {
      char n[32]; token(n);
      printf("levels %u\n", Z3_fixedpoint_get_num_levels(c, fp, find_rel(n)));
    } else {
      fprintf(stderr, "bad command %s\n", cmd); return 2;
    }
    fflush(stdout);
  }
  return 0;
}
