// Build against this fork's libz3. The Go companion is
// pkg/api/api_solver_startup_benchmark_test.go.
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include "z3.h"

int main(int argc, char** argv) {
    if (argc != 3) return 2;
    const bool configured = std::strcmp(argv[1], "MacroFinder") == 0;
    if (!configured && std::strcmp(argv[1], "Default") != 0) return 2;
    char* end = nullptr;
    const long count = std::strtol(argv[2], &end, 10);
    if (count <= 0 || *end) return 2;

    Z3_config cfg = Z3_mk_config();
    Z3_context ctx = Z3_mk_context_rc(cfg);
    Z3_del_config(cfg);
    Z3_ast truth = Z3_mk_true(ctx);
    Z3_inc_ref(ctx, truth);
    Z3_params options = nullptr;
    if (configured) {
        options = Z3_mk_params(ctx);
        Z3_params_inc_ref(ctx, options);
        Z3_params_set_bool(ctx, options, Z3_mk_string_symbol(ctx, "smt.macro_finder"), true);
    }
    auto start = std::chrono::steady_clock::now();
    for (long i = 0; i < count; ++i) {
        Z3_solver solver = Z3_mk_solver(ctx);
        Z3_solver_inc_ref(ctx, solver);
        if (options) Z3_solver_set_params(ctx, solver, options);
        Z3_solver_assert(ctx, solver, truth);
        Z3_solver_dec_ref(ctx, solver);
    }
    auto elapsed = std::chrono::steady_clock::now() - start;
    double ns = std::chrono::duration<double, std::nano>(elapsed).count();
    std::printf("BenchmarkStrategicSolverFirstAssertion/%s %ld %.2f ns/op\n", argv[1], count, ns / count);
    if (options) Z3_params_dec_ref(ctx, options);
    Z3_dec_ref(ctx, truth);
    Z3_del_context(ctx);
    return 0;
}
