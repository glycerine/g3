// Companion to BenchmarkSMTClauseDirectoryMemory. The Boolean atoms
// and watch-list directory already exist when the memory measurement starts.
#include "ast/reg_decl_plugins.h"
#include "smt/smt_context.h"
#include <cstdint>
#include <cstdlib>
#include <iostream>
#include <memory>
#include <string>
#include <vector>
#ifdef __GLIBC__
#include <malloc.h>
#endif

namespace {
struct measured_context : smt::context {
    using smt::context::context;
    size_t watch_bytes() {
        size_t total = 0;
        for (auto& list : m_watches) {
            auto* first = reinterpret_cast<char const*>(list.begin_clause());
            if (first == nullptr) continue;
            auto* last = reinterpret_cast<char const*>(list.end_literals());
            // watch_list::expand puts the offsets in front of the data.
#ifdef _AMD64_
            total += 4 * sizeof(unsigned);
#else
            total += 3 * sizeof(unsigned);
#endif
            total += static_cast<size_t>(last - first);
        }
        return total;
    }
    size_t clause_watches() {
        size_t total = 0;
        for (auto& list : m_watches)
            if (list.begin_clause()) total += list.end_clause() - list.begin_clause();
        return total;
    }

    size_t directory_bytes() const {
        size_t total = 0;
        for (auto* clauses : {&m_lemmas, &m_aux_clauses})
            if (clauses->capacity())
                total += 2 * sizeof(unsigned) + clauses->capacity() * sizeof(smt::clause*);
        return total;
    }

    size_t learned() const { return m_lemmas.size(); }
    size_t auxiliary() const { return m_aux_clauses.size(); }

    size_t literal_watches() {
        size_t total = 0;
        for (auto& list : m_watches)
            if (list.begin_clause()) total += list.end_literals() - list.begin_literals();
        return total;
    }
};

struct borrowed_justification : smt::justification {
    proof* mk_proof(smt::conflict_resolution&) override { return nullptr; }
};

struct worker {
    ast_manager manager;
    params_ref params;
    smt_params smt;
    expr_ref_vector atoms;
    smt::literal literals[96];
    borrowed_justification reason;
    std::unique_ptr<measured_context> context;
    worker(): atoms(manager) {
        reg_decl_plugins(manager);
        params.set_bool("auto_config", false);
        smt.updt_params(params);
        smt.m_binary_clause_opt = true;
        context.reset(new measured_context(manager, smt, params));
        for (unsigned i = 0; i < 96; ++i) {
            atoms.push_back(manager.mk_const(symbol(i), manager.mk_bool_sort()));
            context->internalize(atoms.get(i), true);
            literals[i] = context->get_literal(atoms.get(i));
        }
    }
};

size_t live_malloc_bytes() {
#ifdef __GLIBC__
    auto const m = mallinfo2();
    return m.uordblks + m.hblkhd;
#else
    return 0;
#endif
}

enum class shape { binary, auxiliary, learned, justified, mixed };

bool parse_shape(std::string const& name, shape& which) {
    if (name == "Binary") which = shape::binary;
    else if (name == "Auxiliary") which = shape::auxiliary;
    else if (name == "Learned") which = shape::learned;
    else if (name == "Justified") which = shape::justified;
    else if (name == "Mixed") which = shape::mixed;
    else return false;
    return true;
}

shape clause_shape(shape population, unsigned index) {
    if (population != shape::mixed) return population;
    switch (index % 8) {
    case 0: return shape::learned;
    case 1: case 2: return shape::justified;
    case 3: case 4: return shape::binary;
    default: return shape::auxiliary;
    }
}

struct totals {
    size_t small = 0, watches = 0, directory = 0, literals = 0, clauses = 0;
    size_t learned = 0, auxiliary = 0;
    explicit totals(std::vector<std::unique_ptr<worker>> const& workers) {
        for (auto const& w : workers) {
            small += w->manager.get_allocator().get_allocation_size();
            watches += w->context->watch_bytes();
            directory += w->context->directory_bytes();
            literals += w->context->literal_watches();
            clauses += w->context->clause_watches();
            learned += w->context->learned();
            auxiliary += w->context->auxiliary();
        }
    }
};

bool measure(unsigned count, shape population, char const* name) {
    std::vector<std::unique_ptr<worker>> workers;
    for (unsigned i = 0; i < count; ++i) workers.emplace_back(new worker);
    totals before(workers);
    auto const malloc0 = static_cast<int64_t>(live_malloc_bytes());
    constexpr unsigned width = 1024;
    size_t binary = 0, learned = 0;
    for (auto const& w : workers) {
        for (unsigned j = 0; j < width; ++j) {
            smt::literal lits[] = {w->literals[j % 32], ~w->literals[32 + j / 32],
                                   w->literals[64 + (j % 32 + j / 32) % 32]};
            auto mode = clause_shape(population, j);
            if (mode == shape::binary) {
                if (w->context->mk_clause(2, lits, nullptr, smt::CLS_AUX)) return false;
                ++binary;
                continue;
            }
            auto* reason = mode == shape::justified ? &w->reason : nullptr;
            auto kind = mode == shape::learned ? smt::CLS_LEARNED : smt::CLS_AUX;
            if (kind == smt::CLS_LEARNED) ++learned;
            smt::literal expected[] = {lits[0], lits[1], lits[2]};
            auto* c = w->context->mk_clause(3, lits, reason, kind);
            if (!c || c->get_num_literals() != 3 || c->get_kind() != kind ||
                c->get_justification() != reason) return false;
            for (auto l : expected)
                if (c->get_literal(0) != l && c->get_literal(1) != l && c->get_literal(2) != l)
                    return false;
        }
    }
    auto const malloc_delta = static_cast<int64_t>(live_malloc_bytes()) - malloc0;
    totals after(workers);
    auto const concrete = width * count - binary;
    if (after.literals - before.literals != 2 * binary || after.clauses - before.clauses != 2 * concrete ||
        after.learned - before.learned != learned || after.auxiliary - before.auxiliary != concrete - learned)
        return false;
    auto const small = after.small - before.small, watch = after.watches - before.watches;
    if (!concrete && small) return false;
    if (name) {
        std::cout << "workers,clauses_each,mode,watch_bytes,manager_bytes,directory_bytes,malloc_bytes\n"
                  << count << ',' << width << ',' << name << ',' << watch << ',' << small << ','
                  << after.directory - before.directory << ',' << malloc_delta << '\n';
    }
    else {
        std::cout << "workers,clauses_each,watch_bytes,manager_bytes,malloc_bytes\n"
                  << count << ',' << width << ',' << watch << ',' << small << ',' << malloc_delta << '\n';
    }
    return true;
}
}
int main(int argc, char** argv) {
    if (argc != 2 && argc != 3) return 1;
    char* end;
    auto n = std::strtoul(argv[1], &end, 10);
    if (*end || n < 1 || n > 128) return 1;
    auto which = shape::binary;
    if (argc == 3 && !parse_shape(argv[2], which)) return 1;
    memory::initialize(0);
    return measure(static_cast<unsigned>(n), which, argc == 3 ? argv[2] : nullptr) ? 0 : 1;
}
