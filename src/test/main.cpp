#include<iostream>
#include<time.h>
#include<string>
#include<cstring>
#include "util/util.h"
#include "util/trace.h"
#include "util/debug.h"
#include "util/timeit.h"
#include "util/warning.h"
#include "util/memory_manager.h"
#include "util/gparams.h"


//
// Unit tests fail by asserting.
// If they return, we assume the unit test succeeds
// and print "PASS" to indicate success.
// 

#define TST(MODULE) {                                        \
        std::string s("test ");                              \
        s += #MODULE;                                        \
        void tst_##MODULE();                                 \
        if (do_display_usage)                                \
            std::cout << #MODULE << "\n";                    \
        for (int i = 0; i < argc; i++)                       \
            if (test_all || strcmp(argv[i], #MODULE) == 0) { \
                enable_trace(#MODULE);                       \
                enable_debug(#MODULE);                       \
                timeit timeit(true, s.c_str());              \
                tst_##MODULE();                              \
                    std::cout << "PASS" << std::endl;        \
            }                                                \
    }

#define TST_ARGV(MODULE) {                              \
    std::string s("test ");                             \
    s += #MODULE;                                       \
    void tst_##MODULE(char** argv, int argc, int& i);   \
    if (do_display_usage)                               \
        std::cout << #MODULE << "\n";                   \
    for (int i = 0; i < argc; i++)                      \
    if (strcmp(argv[i], #MODULE) == 0) {            \
            enable_trace(#MODULE);                      \
        enable_debug(#MODULE);                      \
        timeit timeit(true, s.c_str());             \
        tst_##MODULE(argv, argc, i);                \
            std::cout << "PASS" << std::endl;           \
    }                                               \
}

void error(const char * msg) {
    std::cerr << "Error: " << msg << "\n";
    std::cerr << "For usage information: test /h\n";
    exit(1);
}

void display_usage() {
    std::cout << "Z3 unit tests [version 1.0]. (C) Copyright 2006 Microsoft Corp.\n";
    std::cout << "Usage: test [options] [module names]\n";
    std::cout << "\nMisc.:\n";
    std::cout << "  /h          prints this message.\n";
    std::cout << "  /v:level    be verbose, where <level> is the verbosity level.\n";
    std::cout << "  /w          enable warning messages.\n";
    std::cout << "  /a          run all unit tests that don't require arguments.\n";
#if defined(Z3DEBUG) || defined(_TRACE)
    std::cout << "\nDebugging support:\n";
#endif
#ifdef _TRACE
    std::cout << "  /tr:tag     enable trace messages tagged with <tag>.\n";
#endif
#ifdef Z3DEBUG
    std::cout << "  /dbg:tag    enable assertions tagged with <tag>.\n";
#endif
}

void parse_cmd_line_args(int argc, char ** argv, bool& do_display_usage, bool& test_all) {
    int i = 1;
    while (i < argc) {
    char * arg = argv[i], *eq_pos = nullptr;

    if (arg[0] == '-' || arg[0] == '/') {
        char * opt_name = arg + 1;
        char * opt_arg  = nullptr;
        char * colon    = strchr(arg, ':');
        if (colon) {
        opt_arg = colon + 1;
        *colon  = 0;
        }
        if (strcmp(opt_name, "h") == 0 ||
                strcmp(opt_name, "?") == 0) {
        display_usage();
                do_display_usage = true;
                return;
        }
        else if (strcmp(opt_name, "v") == 0) {
        if (!opt_arg)
            error("option argument (/v:level) is missing.");
        long lvl = strtol(opt_arg, nullptr, 10);
        set_verbosity_level(lvl);
        }
        else if (strcmp(opt_name, "w") == 0) {
                enable_warning_messages(true);
        }
        else if (strcmp(opt_name, "a") == 0) {
                test_all = true;
        }
#ifdef _TRACE
        else if (strcmp(opt_name, "tr") == 0) {
        if (!opt_arg)
            error("option argument (/tr:tag) is missing.");
        enable_trace(opt_arg);
        }
#endif
#ifdef Z3DEBUG
        else if (strcmp(opt_name, "dbg") == 0) {
        if (!opt_arg)
            error("option argument (/dbg:tag) is missing.");
        enable_debug(opt_arg);
        }
#endif
    }
        else if (arg[0] != '"' && (eq_pos = strchr(arg, '='))) {
            char * key   = arg;
            *eq_pos      = 0;
            char * value = eq_pos+1; 
            try {
                gparams::set(key, value);
            }
            catch (z3_exception& ex) {
                std::cerr << ex.msg() << "\n";
            }
        }            
    i++;
    }
}


int main(int argc, char ** argv) {
    memory::initialize(0);
    bool do_display_usage = false;
    bool test_all = false;
    parse_cmd_line_args(argc, argv, do_display_usage, test_all);
    TST(random);
    TST(vector);
    TST(symbol_table);
    TST(region);
    TST(symbol);
    TST(heap);
    TST(hashtable);
    TST(rational);
    TST(inf_rational);
    TST(ast);
    TST(optional);
    TST(bit_vector);
    TST(fixed_bit_vector);
    TST(tbv);
    TST(doc);
    TST(udoc_relation);
    TST(string_buffer);
    TST(map);
    TST(diff_logic);
    TST(uint_set);
    TST_ARGV(expr_rand);
    TST(list);
    TST(small_object_allocator);
    TST(timeout);
    TST(proof_checker);
    TST(simplifier);
    TST(bit_blaster);
    TST(var_subst);
    TST(simple_parser);
    TST(api);
    TST(old_interval);
    TST(get_implied_equalities);
    TST(arith_simplifier_plugin);
    TST(matcher);
    TST(object_allocator);
    TST(mpz);
    TST(mpq);
    TST(mpf);
    TST(total_order);
    TST(dl_table);
    TST(dl_context);
    TST(dl_util);
    TST(dl_product_relation);
    TST(dl_relation);
    TST(parray);
    TST(stack);
    TST(escaped);
    TST(buffer);
    TST(chashtable);
    TST(ex);
    TST(nlarith_util);
    TST(api_bug);
    TST(arith_rewriter);
    TST(check_assumptions);
    TST(smt_context);
    TST(theory_dl);
    TST(model_retrieval);
    TST(model_based_opt);
    TST(factor_rewriter);
    TST(smt2print_parse);
    TST(substitution);
    TST(polynomial);
    TST(upolynomial);
    TST(algebraic);
    TST(prime_generator);
    TST(permutation);
    TST(nlsat);
    if (test_all) return 0;
    TST_ARGV(smt_scope_restoration);
    TST(z3_4_7_1_dom_simplify_root);
    TST(z3_4_7_1_elim_term_ite_pop);
    TST(z3_4_7_1_macro_finder_off);
    TST(z3_4_7_1_pb_preprocess_index);
    TST(z3_4_7_1_pb_preprocess_resolve);
    TST(z3_4_7_1_dt_recognizer);
    TST(z3_4_7_1_dom_simplify_oscillation);
    TST(z3_4_7_1_update_field_constructor);
    TST(z3_4_7_1_pb2bv_numeral_lhs);
    TST(z3_4_7_1_eq2bv_stale_cache);
    TST(z3_4_7_1_purify_trig_unsafe);
    TST(z3_4_7_1_ufbv_demod_unbound_var);
    TST(z3_4_7_1_recover01_inconsistent);
    TST(z3_4_7_1_distribute_forall_proof);
    TST(z3_4_7_1_pb2bv_fractional);
    TST(z3_4_7_1_lia2card_empty_eq);
    TST(z3_4_7_1_pdr_nullary_cache);
    TST(z3_4_7_1_duality_unhoist_repeated_child);
    TST(z3_4_7_1_lazy_sparse_nullary_union);
    TST(z3_4_7_1_qbmc_eval_level);
    TST(z3_4_7_1_ddnf_extract_width);
    TST(z3_4_7_1_min_cut_shared_node);
    TST(z3_4_7_1_qi_weakening_proof);
    TST(z3_4_7_1_nlsat_signed_infinity);
    TST(z3_4_7_1_nlsat_eval_sign_overflow);
    TST(z3_4_7_1_nlsat_solve_eq_powers);
    TST(z3_4_7_1_nlsat_simplify_core_sign);
    TST(z3_4_7_1_nlsat_inline_vars);
    TST(z3_4_7_1_nlsat_inline_reorder);
    TST(z3_4_7_1_qe_project_real_direction);
    TST(z3_4_7_1_qe_mod_divisor);
    TST(z3_4_7_1_qe_dt_eq_recognizer);
    TST(ext_numeral);
    TST(interval);
    TST(f2n);
    TST(hwf);
    TST(trigo);
    TST(bits);
    TST(mpbq);
    TST(mpfx);
    TST(mpff);
    TST(horn_subsume_model_converter);
    TST(model2expr);
    TST(hilbert_basis);
    TST(heap_trie);
    TST(karr);
    TST(no_overflow);
    // TST(memory);
    TST(datalog_parser);
    TST_ARGV(datalog_parser_file);
    TST(dl_query);
    TST(quant_solve);
    TST(rcf);
    TST(polynorm);
    TST(qe_arith);
    TST(expr_substitution);
    TST(sorting_network);
    TST(theory_pb);
    TST(simplex);
    TST(sat_user_scope);
    TST(pdr);
    TST_ARGV(ddnf);
    TST(ddnf1);
    TST(model_evaluator);
    TST_ARGV(lp);
    TST(get_consequences);
    TST(pb2bv);
    TST_ARGV(cnf_backbones);
    //TST_ARGV(hs);
}

void initialize_mam() {}
void finalize_mam() {}

