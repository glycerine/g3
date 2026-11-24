// Companion to BenchmarkSMTEnodeScopeBatch in pkg/smtcore.
#include "smt/smt_context.h"
#include "ast/reg_decl_plugins.h"
#include <chrono>
#include <cstdlib>
#include <iostream>

namespace {
struct batch_context : smt::context {
    using smt::context::context;
    using smt::context::push_scope;
    using smt::context::pop_scope;
};

bool measure(unsigned width, unsigned rounds) {
    ast_manager m;
    reg_decl_plugins(m);
    sort_ref s(m.mk_uninterpreted_sort(symbol("U")),m);
    sort* dom[2] = {s.get(),s.get()};
    func_decl_ref f(m.mk_func_decl(symbol("f"),2,dom,s.get()),m);
    params_ref p;
    p.set_bool("auto_config", false);
    p.set_uint("relevancy", 2);
    smt_params params;
    params.updt_params(p);
    batch_context ctx(m,params,p);
    expr_ref_vector roots(m), terms(m);
    for (unsigned i = 0; i < 64; ++i) {
        roots.push_back(m.mk_const(symbol(i),s.get()));
        ctx.internalize(roots.get(i),false);
    }
    for (unsigned i = 0; i < width; ++i) {
        terms.push_back(m.mk_app(f.get(),roots.get(i%64),roots.get(i/64)));
        if (i%16 == 0) ctx.internalize(terms.get(i),false);
    }
    unsigned base = ctx.get_num_e_internalized();
    auto start = std::chrono::steady_clock::now();
    for (unsigned k = 0; k < rounds; ++k) {
        ctx.push_scope();
        for (unsigned i = 0; i < width; ++i)
            if (i%16 != 0) ctx.internalize(terms.get(i),false);
        if (ctx.get_num_e_internalized() != base+width-width/16) return false;
        ctx.pop_scope(1);
        if (ctx.get_num_e_internalized() != base) return false;
    }
    auto ns = std::chrono::duration<double,std::nano>(std::chrono::steady_clock::now()-start).count();
    for (unsigned i = 0; i < width; ++i)
        if (ctx.e_internalized(terms.get(i)) != (i%16 == 0)) return false;
    std::cout << "width,iterations,ns_per_op,enodes_per_op\n" << width << ',' << rounds << ',' << ns/rounds << ',' << width-width/16 << '\n';
    return true;
}
}
int main(int argc, char** argv) {
    if (argc != 3) return 1;
    char* end;
    auto width = std::strtoul(argv[1], &end, 10);
    if (*end || (width != 32 && width != 256 && width != 2048)) return 1;
    auto iterations = std::strtoul(argv[2], &end, 10);
    if (*end || iterations == 0 || iterations > 10000000) return 1;
    memory::initialize(0);
    return measure(width, iterations) ? 0 : 1;
}
