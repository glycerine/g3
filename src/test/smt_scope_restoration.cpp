/*++
Companion to pkg/smtcore/smt_hotpath_benchmark_test.go in the Go port:
BenchmarkSMTScopeRestoration/{SurvivingAtoms,ScopedAtoms}.

Configure: cmake -S . -B build/cpp-bench -DCMAKE_BUILD_TYPE=Release
Build: cmake --build build/cpp-bench --target test-z3 --parallel 8
Run:   ./build/cpp-bench/test-z3 smt_scope_restoration [iterations] [SurvivingAtoms|ScopedAtoms|Both]

Only pop_scope(1) is timed. Construction, validation, and cleanup are excluded.
Whole-process CPU profiles include that excluded work; filter by
smt::context::pop_scope/pop_scope_core when profiling the measured operation.
Native C++ keeps learned clause objects and reinitializes their literals where
necessary. Do not mimic Go's reconstruction implementation here: the purpose
is to compare the two implementations of the same logical backtrack.
--*/

#include "smt/smt_context.h"
#include "ast/reg_decl_plugins.h"
#include "util/z3_exception.h"
#include <chrono>
#include <cerrno>
#include <cstdlib>
#include <iomanip>
#include <iostream>
#include <limits>
#include <string>

namespace {
constexpr unsigned num_atoms = 128;
constexpr unsigned num_clauses = num_atoms / 2;

struct restoration_context : smt::context {
    using smt::context::context;
    using smt::context::push_scope;
    using smt::context::pop_scope;

    void begin_search() {
        VERIFY(m_scope_lvl == 1 && m_base_lvl == 1 && !m_searching);
        m_searching = true;
    }
    void end_search() { m_searching = false; }
    smt::clause_vector const& get_lemmas() const { return m_lemmas; }
};

void benchmark_scope_restoration(unsigned iterations, bool scoped) {
    ast_manager m;
    reg_decl_plugins(m);
    expr_ref_vector atoms(m);
    for (unsigned i = 0; i < num_atoms; ++i) {
        std::string name = "bench-restore-" + std::to_string(i);
        atoms.push_back(m.mk_const(symbol(name.c_str()), m.mk_bool_sort()));
    }
    params_ref p;
    p.set_bool("auto_config", false);
    p.set_uint("relevancy", 2);
    smt_params params;
    params.updt_params(p);
    restoration_context ctx(m, params, p);
    unsigned const base_vars = ctx.get_num_bool_vars();
    smt::literal literals[num_atoms];
    smt::clause* clauses[num_clauses];
    auto internalize = [&]() {
        for (unsigned i = 0; i < num_atoms; ++i) {
            ctx.internalize(atoms.get(i), true);
            ctx.mark_as_relevant(atoms.get(i));
            literals[i] = ctx.get_literal(atoms.get(i));
        }
    };
    using clock = std::chrono::steady_clock;
    clock::duration elapsed = clock::duration::zero();
    for (unsigned round = 0; round < iterations; ++round) {
        ctx.push(); // user scope owns the restored learned clauses
        if (!scoped)
            internalize();
        ctx.begin_search();
        ctx.push_scope(); // search decision scope, not another user push
        if (scoped)
            internalize();
        for (unsigned j = 0; j < num_clauses; ++j) {
            smt::literal ls[] = {
                literals[2*j], ~literals[2*j+1], literals[(2*j+2) % num_atoms]
            };
            clauses[j] = ctx.mk_clause(3, ls, nullptr, smt::CLS_LEARNED);
            VERIFY(clauses[j]);
            VERIFY(clauses[j]->reinternalize_atoms() == scoped);
        }
        VERIFY(ctx.get_scope_level() == 2 && ctx.get_lemmas().size() == num_clauses);
        VERIFY(ctx.get_intern_level(ctx.get_bool_var(atoms.get(0))) == (scoped ? 2u : 1u));

        auto start = clock::now();
        ctx.pop_scope(1);
        elapsed += clock::now() - start;

        VERIFY(ctx.get_scope_level() == 1 && ctx.get_lemmas().size() == num_clauses);
        VERIFY(!ctx.inconsistent());
        for (expr* atom : atoms) {
            VERIFY(ctx.b_internalized(atom));
            VERIFY(ctx.get_intern_level(ctx.get_bool_var(atom)) == 1);
            VERIFY(ctx.is_relevant(atom) == !scoped);
        }
        for (unsigned j = 0; j < num_clauses; ++j) {
            // Surviving clauses keep their objects; scoped atoms must now have
            // literals bound to the variables internalized at the search base.
            VERIFY(ctx.get_lemmas()[j] == clauses[j]);
            VERIFY(clauses[j]->get_num_literals() == 3);
            smt::literal expected[] = {
                ctx.get_literal(atoms.get(2*j)), ~ctx.get_literal(atoms.get(2*j+1)),
                ctx.get_literal(atoms.get((2*j+2) % num_atoms))
            };
            for (smt::literal lit : expected) {
                bool found = false;
                for (unsigned k = 0; k < clauses[j]->get_num_literals(); ++k)
                    found |= clauses[j]->get_literal(k) == lit;
                VERIFY(found);
            }
        }
        ctx.end_search();
        ctx.pop(1); // discard the user scope and all its restored clauses
        VERIFY(ctx.get_lemmas().empty());
        VERIFY(ctx.get_num_bool_vars() == base_vars);
        VERIFY(ctx.get_scope_level() == 0);
    }
    double ns = std::chrono::duration<double, std::nano>(elapsed).count();
    std::cout << "BenchmarkSMTScopeRestoration/"
              << (scoped ? "ScopedAtoms" : "SurvivingAtoms") << '\t'
              << iterations << '\t' << std::fixed << std::setprecision(2)
              << ns / iterations << " ns/op\t"
              << num_atoms << " atoms/op\t" << num_clauses << " clauses/op\n";
}
}

void tst_smt_scope_restoration(char** argv, int argc, int& i) {
    unsigned iterations = 10000;
    std::string selected = "Both";
    if (i + 1 < argc) {
        char* end = nullptr;
        errno = 0;
        unsigned long value = std::strtoul(argv[++i], &end, 10);
        if (errno || !end || *end || !value || value > std::numeric_limits<unsigned>::max())
            throw default_exception("smt_scope_restoration expects a positive iteration count");
        iterations = static_cast<unsigned>(value);
    }
    if (i + 1 < argc)
        selected = argv[++i];
    if (selected != "Both" && selected != "SurvivingAtoms" && selected != "ScopedAtoms")
        throw default_exception("smt_scope_restoration case must be SurvivingAtoms, ScopedAtoms, or Both");
    if (selected != "ScopedAtoms")
        benchmark_scope_restoration(iterations, false);
    if (selected != "SurvivingAtoms")
        benchmark_scope_restoration(iterations, true);
}
