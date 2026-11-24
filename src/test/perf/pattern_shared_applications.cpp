// Companion to BenchmarkPatternSharedApplications in pkg/ast/pattern.
#include "ast/reg_decl_plugins.h"
#include "ast/pattern/pattern_inference.h"
#include <chrono>
#include <iostream>
#include <string>
#include <vector>

static void run(unsigned width, unsigned count) {
    ast_manager m;
    reg_decl_plugins(m);
    sort* boolean = m.mk_bool_sort();
    expr_ref x(m.mk_var(0, boolean), m), y(m.mk_var(1, boolean), m);
    std::vector<sort*> sorts(width, boolean);
    std::vector<expr*> args(width);
    for (unsigned i = 0; i < width; ++i) args[i] = i % 2 ? y.get() : x.get();
    app_ref_vector terms(m);
    for (unsigned i = 0; i < 2; ++i) {
        func_decl_ref f(m.mk_func_decl(symbol(900 + i), width, sorts.data(), boolean), m);
        terms.push_back(m.mk_app(f, width, args.data()));
    }
    for (unsigned i = 0; i < width; ++i) args[i] = terms.get(i % 2);
    expr_ref body(m.mk_and(width, args.data()), m);
    symbol names[] = {symbol(0u), symbol(1u)};
    quantifier_ref original(m.mk_forall(2, sorts.data(), names, body), m);
    pattern_inference_params params;
    params.m_pi_use_database = false;
    params.m_pi_block_loop_patterns = false;
    pattern_inference_cfg cfg(m, params);
    {
        expr_ref result(m);
        proof_ref proof(m);
        bool changed = cfg.reduce_quantifier(original, body, nullptr, nullptr, result, proof);
        VERIFY(changed && to_quantifier(result)->get_num_patterns() == 2);
        for (unsigned j = 0; j < 2; ++j)
            VERIFY(to_app(to_quantifier(result)->get_pattern(j))->get_arg(0) == terms.get(j));
    }
    auto start = std::chrono::steady_clock::now();
    for (unsigned i = 0; i < count; ++i) {
        expr_ref result(m);
        proof_ref proof(m);
        bool changed = cfg.reduce_quantifier(original, body, nullptr, nullptr, result, proof);
        VERIFY(changed);
    }
    std::cout << width << ',' << count << ',' << std::chrono::duration<double, std::nano>(std::chrono::steady_clock::now() - start).count() / count << '\n';
}
int main(int argc, char** argv) {
    if (argc != 3) return 1;
    unsigned width = std::stoul(argv[1]), count = std::stoul(argv[2]);
    if (width < 2 || !count) return 1;
    memory::initialize(0);
    run(width, count);
}
