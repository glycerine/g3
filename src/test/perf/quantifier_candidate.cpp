// Same candidate body and bindings as BenchmarkSMTQuantifierCandidate.
// Time checker::is_sat only. The Go benchmark also measures queue dispatch.
#include "ast/ast.h"
#include "ast/reg_decl_plugins.h"
#include "smt/smt_context.h"
#include "smt/smt_checker.h"
#include <chrono>
#include <iomanip>
#include <iostream>
#include <string>

namespace {
void run(unsigned width, unsigned iterations) {
    ast_manager m;
    reg_decl_plugins(m);
    params_ref p;
    p.set_bool("auto_config", false);
    p.set_uint("relevancy", 2);
    smt_params options;
    options.updt_params(p);
    smt::context ctx(m, options, p);
    smt::checker checker(ctx);
    sort_ref s(m.mk_uninterpreted_sort(symbol("candidate-sort")), m);
    func_decl_ref f(m.mk_func_decl(symbol("candidate-function"), s, s), m);
    expr_ref x(m.mk_var(0, s), m);
    expr_ref fx(m.mk_app(f.get(), x.get()), m);
    expr_ref_vector equalities(m), atoms(m);
    ptr_vector<smt::enode> bindings;
    for (unsigned i = 0; i <= width; ++i) {
        auto name = "candidate-" + std::to_string(i);
        expr_ref a(m.mk_const(symbol(name.c_str()), s), m);
        atoms.push_back(a);
        ctx.internalize(a, false);
        ctx.mark_as_relevant(a.get());
        bindings.push_back(ctx.get_enode(a));
        if (i < width) {
            expr_ref fa(m.mk_app(f.get(), a.get()), m);
            ctx.internalize(fa, false);
            ctx.mark_as_relevant(fa.get());
            equalities.push_back(m.mk_eq(fx, fa));
        }
    }
    expr_ref body(m.mk_or(equalities.size(), equalities.c_ptr()), m);
    for (unsigned i : {0u, width - 1, width}) {
        smt::enode* binding = bindings.get(i);
        VERIFY(checker.is_sat(body, 1, &binding) == (i < width));
    }
    for (unsigned i : {0u, width - 1}) {
        smt::enode* binding = bindings.get(i);
        auto start = std::chrono::steady_clock::now();
        for (unsigned j = 0; j < iterations; ++j)
            VERIFY(checker.is_sat(body, 1, &binding));
        auto elapsed = std::chrono::steady_clock::now() - start;
        double ns = std::chrono::duration<double, std::nano>(elapsed).count();
        std::cout << width << ',' << (i ? "Last" : "First") << ',' << iterations << ','
                  << std::fixed << std::setprecision(2) << (ns / iterations) << '\n';
    }
}
}

int main() {
    memory::initialize(0);
    std::cout << "width,position,iterations,ns_per_op\n";
    run(4, 100000);
    run(16, 100000);
    return 0;
}
