// Counterpart to BenchmarkASTCanonicalApplication and BenchmarkASTApplicationReuse.
#include "ast/ast.h"
#include <chrono>
#include <iomanip>
#include <iostream>

namespace {
using clock_type = std::chrono::steady_clock;
constexpr unsigned width = 256;
constexpr unsigned iterations = 200000;

void print_result(char const* test, unsigned arity, unsigned retained, clock_type::time_point begin) {
    double ns = std::chrono::duration<double, std::nano>(clock_type::now() - begin).count();
    std::cout << test << ',' << arity << ',' << retained << ',' << iterations << ','
              << std::fixed << std::setprecision(2) << ns / iterations << '\n';
}

void run(unsigned arity, unsigned retained, bool single) {
    ast_manager m;
    sort* boolean = m.mk_bool_sort();
    sort* domain[4] = {boolean, boolean, boolean, boolean};
    expr_ref_vector variables(m), roots(m);
    unsigned count = single ? arity : width + arity;
    for (unsigned i = 0; i < count; ++i)
        variables.push_back(m.mk_var(i, boolean));
    func_decl_ref decl(m.mk_func_decl(symbol("canonical-app"), arity, domain, boolean), m);
    expr* args[4];
    for (unsigned i = 0; i < arity; ++i)
        args[i] = variables.get(single ? i : width + i);
    if (single) {
        app_ref want(m.mk_app(decl.get(), arity, args), m);
        auto start = clock_type::now();
        for (unsigned i = 0; i < iterations; ++i)
            VERIFY(m.mk_app(decl.get(), arity, args) == want.get());
        print_result("single", arity, 1, start);
        return;
    }
    for (unsigned i = 0; i < retained; ++i) {
        args[0] = variables.get(i);
        roots.push_back(m.mk_app(decl.get(), arity, args));
    }
    auto start = clock_type::now();
    for (unsigned i = 0; i < iterations; ++i) {
        unsigned index = (i * 197u) & (width - 1);
        args[0] = variables.get(index);
        app_ref result(m.mk_app(decl.get(), arity, args), m);
        VERIFY(index >= retained || result.get() == roots.get(index));
    }
    print_result("rotating", arity, retained, start);
}
}
int main() {
    memory::initialize(0);
    std::cout << "workload,arity,resident,iterations,ns_per_op\n";
    for (unsigned arity : {1u, 2u, 3u, 4u}) {
        run(arity, 1, true);
        for (unsigned n : {0u, 192u, 256u})
            run(arity, n, false);
    }
    return 0;
}
