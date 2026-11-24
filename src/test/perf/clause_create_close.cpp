// Companion to BenchmarkSMTClauseCreateClose. requested_bytes is the change
// in this AST manager's small-object allocator while the clause is live.
#include "ast/reg_decl_plugins.h"
#include "smt/smt_clause.h"
#include "smt/smt_justification.h"
#include <chrono>
#include <cstdlib>
#include <iostream>
#include <string>
#include <vector>

namespace {
struct test_proof : smt::justification {
    proof* mk_proof(smt::conflict_resolution&) override { return nullptr; }
};

struct count_deletions : smt::clause_del_eh {
    unsigned long count = 0;
    void operator()(ast_manager&, smt::clause*) override { ++count; }
};

bool measure(unsigned width, std::string const& mode, unsigned long iterations) {
    ast_manager manager;
    reg_decl_plugins(manager);
    expr_ref_vector atoms(manager);
    atoms.push_back(manager.mk_true());
    std::vector<smt::literal> literals;
    for (unsigned i = 0; i < width; ++i) {
        atoms.push_back(manager.mk_const(symbol(i), manager.mk_bool_sort()));
        literals.emplace_back(i + 1, i % 2 != 0);
    }

    std::vector<expr*> expressions;
    for (unsigned i = 0; i < atoms.size(); ++i) expressions.push_back(atoms.get(i));

    bool save = mode == "Saved";
    auto kind = mode == "Learned" || save ? smt::CLS_LEARNED : smt::CLS_AUX;
    test_proof proof_object;
    auto* reason = mode == "Proof" ? &proof_object : nullptr;
    count_deletions handler;
    auto* on_delete = mode == "Handler" ? &handler : nullptr;
    auto create = [&]() {
        return smt::clause::mk(manager, width, literals.data(), kind, reason, on_delete, save, expressions.data());
    };
    auto const base = manager.get_allocator().get_allocation_size();
    auto* trial = create();
    auto const bytes = manager.get_allocator().get_allocation_size() - base;
    if (trial->get_num_literals() != width || trial->get_kind() != kind ||
        trial->get_justification() != reason || trial->get_del_eh() != on_delete ||
        trial->reinternalize_atoms() != save) return false;
    for (unsigned i = 0; i < width; ++i) {
        if (trial->get_literal(i) != literals[i]) return false;
        if (save && (trial->get_atom(i) != atoms.get(i+1) ||
                     trial->get_atom_sign(i) != literals[i].sign())) return false;
    }
    if (trial->is_lemma() && trial->get_activity() != 1) return false;
    trial->deallocate(manager);
    auto const start = std::chrono::steady_clock::now();
    for (unsigned long i = 0; i < iterations; ++i) create()->deallocate(manager);
    auto const ns = std::chrono::duration<double, std::nano>(std::chrono::steady_clock::now() - start).count();
    if (manager.get_allocator().get_allocation_size() != base ||
        (on_delete && handler.count != iterations+1)) return false;
    std::cout << "width,mode,iterations,requested_bytes,ns_per_op\n"
              << width << ',' << mode << ',' << iterations << ',' << bytes << ',' << ns/iterations << '\n';
    return true;
}
}

int main(int argc, char** argv) {
    if (argc != 4) return 1;
    char* end;
    unsigned long width = std::strtoul(argv[1], &end, 10);
    if (*end || (width != 3 && width != 8 && width != 65)) return 1;
    std::string mode(argv[2]);
    if (mode != "Bare" && mode != "Learned" && mode != "Saved" && mode != "Proof" && mode != "Handler") return 1;
    unsigned long iterations = std::strtoul(argv[3], &end, 10);
    if (*end || iterations == 0 || iterations > 100000000) return 1;
    memory::initialize(0);
    return measure(static_cast<unsigned>(width), mode, iterations) ? 0 : 1;
}
