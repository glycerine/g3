// Counterpart to BenchmarkSMTEnodeCreation. Keep the input DAG and argument
// roots alive while repeatedly constructing and deleting a single application.
#include "ast/reg_decl_plugins.h"
#include "smt/smt_enode.h"
#include "util/memory_manager.h"
#include <chrono>
#include <cstdlib>
#include <iostream>

namespace {
bool measure(unsigned arity, unsigned iterations) {
    ast_manager m;
    reg_decl_plugins(m);
    sort_ref sort(m.mk_uninterpreted_sort(symbol("U")), m);
    smt::app2enode_t index;
    region roots, applications;
    app_ref_vector constants(m);
    expr* args[16];
    ::sort* domain[16];
    for (unsigned i = 0; i < 16; ++i) {
        app* value = m.mk_const(symbol(i), sort.get());
        constants.push_back(value);
        args[i] = value;
        domain[i] = sort.get();
        smt::enode* node = smt::enode::mk(m, roots, index, value, 0, false, false, 0, true, true);
        index.setx(value->get_id(), node, nullptr);
    }
    func_decl_ref decl(m.mk_func_decl(symbol("f"), arity, domain, sort.get()), m);
    app_ref term(m.mk_app(decl.get(), arity, args), m);
    auto start = std::chrono::steady_clock::now();
    for (unsigned i = 0; i < iterations; ++i) {
        applications.push_scope();
        smt::enode* node = smt::enode::mk(m, applications, index, term.get(), 0, false, false, 0, true, true);
        if (node->get_num_args() != arity || node->get_owner()->get_decl() != decl.get()) return false;
        node->del_eh(m, true);
        applications.pop_scope();
    }
    auto elapsed = std::chrono::duration<double, std::nano>(std::chrono::steady_clock::now()-start).count();
    for (unsigned i = 0; i < 16; ++i) {
        smt::enode* node = index[constants.get(i)->get_id()];
        if (node->get_num_parents() != 0) return false;
        node->del_eh(m, true);
    }
    std::cout << arity << ',' << iterations << ',' << elapsed/iterations << ','
              << sizeof(smt::enode) << ',' << smt::enode::get_enode_size(arity)
              << ',' << sizeof(smt::eq_justification) << '\n';
    return true;
}
}

int main(int argc, char** argv) {
    if (argc != 3) return 1;
    char* end;
    unsigned long arity = std::strtoul(argv[1], &end, 10);
    if (*end || arity > 16) return 1;
    unsigned long iterations = std::strtoul(argv[2], &end, 10);
    if (*end || iterations == 0 || iterations > 1000000000) return 1;
    memory::initialize(0);
    std::cout << "arity,iterations,ns_per_op,header_bytes,node_bytes,justification_bytes\n";
    return measure(static_cast<unsigned>(arity), static_cast<unsigned>(iterations)) ? 0 : 1;
}
