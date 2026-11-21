// Native companion to BenchmarkSMTAssertedOrWitness in pkg/smtcore.
#include "ast/reg_decl_plugins.h"
#include "smt/smt_context.h"
#include <chrono>
#include <iostream>
#include <string>

namespace {
struct witness_context : smt::context {
    using smt::context::context;
    using smt::context::internalize_assertion;

    void deliver(smt::bool_var v, bool value) {
        m_relevancy_propagator->assign_eh(bool_var2expr(v), value);
    }
};

void run(unsigned width, unsigned index, unsigned iterations) {
    ast_manager m;
    reg_decl_plugins(m);
    params_ref p;
    p.set_bool("auto_config", false);
    p.set_uint("relevancy", 2);
    smt_params options;
    options.updt_params(p);
    witness_context ctx(m, options, p);
    expr_ref_vector atoms(m), children(m);
    for (unsigned i = 0; i < width; ++i) {
        std::string name = "root-witness-" + std::to_string(i);
        expr* atom = m.mk_const(symbol(name.c_str()), m.mk_bool_sort());
        atoms.push_back(atom);
        ctx.internalize(atom, true);
        children.push_back(i % 2 ? m.mk_not(atom) : atom);
    }
    expr_ref root(m.mk_or(children.size(), children.c_ptr()), m);
    ctx.internalize_assertion(root, nullptr, 0);
    VERIFY(ctx.is_relevant(root.get()) && !ctx.b_internalized(root.get()));
    VERIFY(!ctx.is_relevant(children.get(index)));
    bool value = index % 2 == 0;
    smt::bool_var v = ctx.get_bool_var(atoms.get(index));
    ctx.assign(smt::literal(v, !value), smt::b_justification::mk_axiom());
    ctx.deliver(v, value);
    VERIFY(ctx.is_relevant(children.get(index)));

    auto start = std::chrono::steady_clock::now();
    for (unsigned i = 0; i < iterations; ++i)
        ctx.deliver(v, value);
    auto elapsed = std::chrono::steady_clock::now() - start;
    double ns = std::chrono::duration<double, std::nano>(elapsed).count();
    std::cout << width << "," << index << "," << iterations << "," << ns / iterations << "\n";
}
}

int main() {
    memory::initialize(0);
    std::cout << "width,child,iterations,ns_per_op\n";
    for (unsigned width : {4u, 16u, 64u}) {
        run(width, 0, 500000);
        run(width, width - 1, 500000);
    }
}
