// usage: wide <bits> <relation>: facts 5 and 7 of p(BitVec bits), query p
#include <stdio.h>
#include <stdlib.h>
#include "z3.h"
int main(int argc, char** argv) {
  unsigned bits = argc > 1 ? atoi(argv[1]) : 160;
  const char* relation = argc > 2 ? argv[2] : "doc";
  Z3_config cfg = Z3_mk_config();
  Z3_context c = Z3_mk_context(cfg);
  Z3_del_config(cfg);
  Z3_fixedpoint fp = Z3_mk_fixedpoint(c);
  Z3_fixedpoint_inc_ref(c, fp);
  Z3_params p = Z3_mk_params(c);
  Z3_params_inc_ref(c, p);
  Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "engine"), Z3_mk_string_symbol(c, "datalog"));
  Z3_params_set_symbol(c, p, Z3_mk_string_symbol(c, "datalog.default_relation"), Z3_mk_string_symbol(c, relation));
  Z3_fixedpoint_set_params(c, fp, p);
  Z3_sort bv = Z3_mk_bv_sort(c, bits);
  Z3_func_decl d = Z3_mk_func_decl(c, Z3_mk_string_symbol(c, "p"), 1, &bv, Z3_mk_bool_sort(c));
  Z3_fixedpoint_register_relation(c, fp, d);
  unsigned five = 5, seven = 7;
  Z3_fixedpoint_add_fact(c, fp, d, 1, &five);
  Z3_fixedpoint_add_fact(c, fp, d, 1, &seven);
  Z3_lbool r = Z3_fixedpoint_query_relations(c, fp, 1, &d);
  printf("result %d\n%s\n", r, r == Z3_L_UNDEF ? Z3_fixedpoint_get_reason_unknown(c, fp) : Z3_ast_to_string(c, Z3_fixedpoint_get_answer(c, fp)));
  Z3_params_dec_ref(c, p);
  Z3_fixedpoint_dec_ref(c, fp);
  Z3_del_context(c);
  return 0;
}
