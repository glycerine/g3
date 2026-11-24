// Companion to BenchmarkSMTDeepSharedAssertion in pkg/smtcore.
#include "ast/reg_decl_plugins.h"
#include "smt/smt_context.h"
#include <chrono>
#include <iostream>
#include <string>

namespace {
struct benchmark_context : smt::context {
    using smt::context::context;
    using smt::context::push_scope;
    using smt::context::pop_scope;
};

void run(unsigned unrelated, unsigned iterations) {
    ast_manager m;
    reg_decl_plugins(m);
    sort* boolean = m.mk_bool_sort();
    func_decl_ref wrap(m.mk_func_decl(symbol("f"), boolean, boolean), m);
    expr_ref shared(m.mk_const(symbol("a"), boolean), m);
    for (unsigned i = 0; i <= 1024; ++i)
        shared = m.mk_app(wrap, shared.get());
    expr_ref next(m.mk_app(wrap, shared.get()), m);
    params_ref p;
    p.set_bool("auto_config", false);
    p.set_uint("relevancy", 2);
    smt_params params;
    params.updt_params(p);
    benchmark_context ctx(m, params, p);
    ctx.internalize_assertion(shared, nullptr, 0);
    for (unsigned i = 0; i < unrelated; ++i) {
        auto name = "p" + std::to_string(i);
        ctx.internalize(m.mk_const(symbol(name.c_str()), boolean), true);
    }
    unsigned base = ctx.get_num_bool_vars();
    auto begin = std::chrono::steady_clock::now();
    for (unsigned i = 0; i < iterations; ++i) {
        ctx.push_scope();
        ctx.internalize_assertion(next, nullptr, 0);
        ctx.pop_scope(1);
    }
    auto elapsed = std::chrono::steady_clock::now() - begin;
    VERIFY(ctx.get_num_bool_vars() == base && ctx.b_internalized(shared) && !ctx.b_internalized(next));
    std::cout << unrelated << ',' << iterations << ','
              << std::chrono::duration<double, std::nano>(elapsed).count() / iterations << '\n';
}
}
int main(int argc, char** argv) {
    if (argc != 3) return 1;
    unsigned iterations = std::stoul(argv[2]);
    if (!iterations) return 1;
    memory::initialize(0);
    run(std::stoul(argv[1]), iterations);
    return 0;
}
