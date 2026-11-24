// Counterpart to BenchmarkSMTQuantifierInstanceBatches.
#include "ast/ast.h"
#include "smt/cached_var_subst.h"
#include <chrono>
#include <iomanip>
#include <iostream>

namespace {
void run(unsigned width, unsigned iterations) {
    ast_manager m;
    sort* boolean = m.mk_bool_sort();
    func_decl_ref relation(m.mk_func_decl(symbol("relation"), boolean, boolean, boolean), m);
    auto body = [&](expr* x, expr* y) {
        app* xy = m.mk_app(relation, x, y);
        app* yx = m.mk_app(relation, y, x);
        app* shared = m.mk_ite(xy, yx, x);
        return m.mk_ite(yx, m.mk_app(relation, shared, x), m.mk_app(relation, y, shared));
    };
    sort* sorts[] = { boolean, boolean };
    symbol names[] = { symbol(1u), symbol(2u) };
    quantifier_ref q(m.mk_forall(2, sorts, names, body(m.mk_var(0, boolean), m.mk_var(1, boolean))), m);
    expr_ref_vector atoms(m);
    region storage;
    smt::app2enode_t lookup;
    ptr_vector<smt::enode> nodes;
    for (unsigned i = 0; i < width; ++i) {
        app* a = m.mk_const(symbol(i + 10), boolean);
        atoms.push_back(a);
        nodes.push_back(smt::enode::mk(m, storage, lookup, a, 0, false, false, 0, true, false));
    }
    cached_var_subst cache(m);
    expr_ref result(m);
    unsigned batch = width * width;
    auto begin = std::chrono::steady_clock::now();
    for (unsigned i = 0; i < iterations; ++i) {
        unsigned k = i % batch;
        if (k == 0 && i != 0)
            cache.reset();
        smt::enode* bindings[] = { nodes[k / width], nodes[k % width] };
        cache(q, 2, bindings, result);
        VERIFY(result.get() != nullptr);
    }
    auto elapsed = std::chrono::steady_clock::now() - begin;
    unsigned last = (iterations - 1) % batch;
    VERIFY(result.get() == body(atoms.get(last % width), atoms.get(last / width)));
    for (auto* n : nodes)
        n->del_eh(m, false);
    double ns = std::chrono::duration<double, std::nano>(elapsed).count() / iterations;
    std::cout << batch << ',' << iterations << ',' << std::fixed << std::setprecision(2) << ns << '\n';
}
}

int main() {
    memory::initialize(0);
    std::cout << "batch,iterations,ns_per_op\n";
    for (unsigned width : {4u, 8u, 16u, 32u})
        run(width, 65536);
    return 0;
}
