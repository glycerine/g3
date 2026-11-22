// Keep one completely independent context and initialized default solver per worker.
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <vector>
#include "z3.h"

struct worker {
    Z3_context context;
    Z3_solver solver;
};

int main(int argc, char** argv) {
    if (argc != 2) return 2;
    char* end = nullptr;
    long count = std::strtol(argv[1], &end, 10);
    if (*end || count < 1 || count > 1024) return 2;
    std::vector<worker> workers;
    workers.reserve(count);
    auto before = Z3_get_estimated_alloc_size();
    auto start = std::chrono::steady_clock::now();
    for (long i = 0; i < count; ++i) {
        Z3_config config = Z3_mk_config();
        Z3_context ctx = Z3_mk_context_rc(config);
        Z3_del_config(config);
        Z3_ast truth = Z3_mk_true(ctx);
        Z3_inc_ref(ctx, truth);
        Z3_solver solver = Z3_mk_solver(ctx);
        Z3_solver_inc_ref(ctx, solver);
        Z3_solver_assert(ctx, solver, truth);
        Z3_dec_ref(ctx, truth);
        workers.push_back({ctx, solver});
    }
    auto elapsed = std::chrono::steady_clock::now() - start;
    double ns = std::chrono::duration<double, std::nano>(elapsed).count();
    double retained = static_cast<double>(Z3_get_estimated_alloc_size()) - static_cast<double>(before);
    std::printf("Workers%ld 1 %.2f ns/op %.2f live-B/worker\n", count, ns, retained / count);
    for (auto it = workers.rbegin(); it != workers.rend(); ++it) {
        Z3_solver_dec_ref(it->context, it->solver);
        Z3_del_context(it->context);
    }
    return 0;
}
