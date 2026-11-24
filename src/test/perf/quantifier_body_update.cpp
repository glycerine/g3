// Counterpart to BenchmarkASTQuantifierBodyUpdate.
#include "ast/ast.h"
#include <chrono>
#include <iomanip>
#include <iostream>
#include <string>
#include <vector>

namespace {
void run(unsigned binders, bool retained, unsigned iterations) {
    ast_manager m;
    sort* boolean = m.mk_bool_sort();
    std::vector<sort*> sorts(binders, boolean);
    std::vector<symbol> names;
    for (unsigned i = 0; i < binders; ++i)
        names.emplace_back(("x" + std::to_string(i)).c_str());
    var* body = m.mk_var(0, boolean);
    quantifier_ref original(m.mk_forall(binders, sorts.data(), names.data(), body), m);
    expr_ref next_body(m.mk_not(body), m);
    quantifier_ref existing(m);
    if (retained)
        existing = m.update_quantifier(original, next_body);

    auto start = std::chrono::steady_clock::now();
    for (unsigned i = 0; i < iterations; ++i) {
        quantifier_ref result(m.update_quantifier(original, next_body), m);
        VERIFY(result.get() != original.get() && (!retained || result.get() == existing.get()));
    }
    auto elapsed = std::chrono::steady_clock::now() - start;
    double ns = std::chrono::duration<double, std::nano>(elapsed).count() / iterations;
    std::cout << binders << ',' << (retained ? "Retained" : "Fresh") << ',' << iterations
              << ',' << std::fixed << std::setprecision(2) << ns << '\n';
}
}

int main() {
    memory::initialize(0);
    std::cout << "binders,lifetime,iterations,ns_per_op\n";
    for (unsigned n : {1u, 2u, 4u, 8u, 64u})
        for (bool retained : {false, true})
            run(n, retained, 20000);
}
