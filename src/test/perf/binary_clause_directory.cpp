// Companion to BenchmarkSMTClauseDirectoryMemory/Binary. The Boolean atoms
// and watch-list directory already exist when the memory measurement starts.
#include "ast/reg_decl_plugins.h"
#include "smt/smt_context.h"
#include <cstdlib>
#include <iostream>
#include <memory>
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
    size_t literal_watches() {
        size_t total = 0;
        for (auto& list : m_watches)
            if (list.begin_clause()) total += list.end_literals() - list.begin_literals();
        return total;
    }
};

struct worker {
    ast_manager manager;
    params_ref params;
    smt_params smt;
    expr_ref_vector atoms;
    smt::literal literals[96];
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

bool measure(unsigned count) {
    std::vector<std::unique_ptr<worker>> workers;
    for (unsigned i = 0; i < count; ++i) workers.emplace_back(new worker);
    size_t small0 = 0, watch0 = 0, occurrences0 = 0;
    for (auto const& w : workers) {
        small0 += w->manager.get_allocator().get_allocation_size();
        watch0 += w->context->watch_bytes();
        occurrences0 += w->context->literal_watches();
    }
    size_t malloc0 = live_malloc_bytes();
    constexpr unsigned width = 1024;
    for (auto const& w : workers) {
        for (unsigned j = 0; j < width; ++j) {
            smt::literal pair[] = {w->literals[j % 32], ~w->literals[32 + j / 32]};
            if (w->context->mk_clause(2, pair, nullptr, smt::CLS_AUX) != nullptr) return false;
        }
    }
    size_t malloc1 = live_malloc_bytes(), small1 = 0, watch1 = 0, occurrences1 = 0;
    for (auto const& w : workers) {
        small1 += w->manager.get_allocator().get_allocation_size();
        watch1 += w->context->watch_bytes();
        occurrences1 += w->context->literal_watches();
    }
    if (occurrences1 - occurrences0 != 2 * width * count || small0 != small1) return false;
    std::cout << "workers,clauses_each,watch_bytes,manager_bytes,malloc_bytes\n"
              << count << ',' << width << ',' << watch1-watch0 << ',' << small1-small0 << ',' << malloc1-malloc0 << '\n';
    return true;
}
}
int main(int argc, char** argv) {
    if (argc != 2) return 1;
    char* end;
    auto n = std::strtoul(argv[1], &end, 10);
    if (*end || n < 1 || n > 128) return 1;
    memory::initialize(0);
    return measure(static_cast<unsigned>(n)) ? 0 : 1;
}
