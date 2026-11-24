// Companion to BenchmarkSMTAssertedBatchReplacement. Compile this fixture
// with -fno-access-control to call the fork's private production swap.
#include "ast/reg_decl_plugins.h"
#include "smt/asserted_formulas.h"
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <cstring>

static void measure(unsigned width, char const* change, unsigned iterations) {
    ast_manager m;
    reg_decl_plugins(m);
    smt_params options;
    asserted_formulas assertions(m, options, params_ref());
    vector<justified_expr> batches[2];
    for (unsigned version = 0; version < 2; ++version) {
        for (unsigned j = 0; j < width; ++j) {
            bool differs = version && ((!std::strcmp(change, "Last") && j + 1 == width) ||
                                       (!std::strcmp(change, "First") && j == 0));
            expr* term = m.mk_var(differs ? width : j, m.mk_bool_sort());
            batches[version].push_back(justified_expr(m, term, nullptr));
        }
    }
    assertions.swap_asserted_formulas(batches[0]);
    auto start = std::chrono::steady_clock::now();
    for (unsigned i = 0; i < iterations; ++i)
        assertions.swap_asserted_formulas(batches[(i + 1) & 1]);
    auto end = std::chrono::steady_clock::now();
    VERIFY(assertions.get_num_formulas() == width);
    for (unsigned i = 0; i < width; ++i)
        VERIFY(assertions.get_formula(i) == batches[iterations & 1][i].get_fml());
    double ns = std::chrono::duration<double, std::nano>(end - start).count() / iterations;
    std::printf("%s,%u,%u,%.2f\n", change, width, iterations, ns);
}

int main() {
    memory::initialize(0);
    std::puts("position,width,iterations,ns");
    for (auto width : {8u, 128u, 1024u})
        for (auto change : {"Identical", "Last", "First"})
            measure(width, change, 3000);
    return 0;
}
