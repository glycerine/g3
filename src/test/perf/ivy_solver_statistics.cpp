// Link-time observer for the single-threaded GoIvy classic_z3 benchmark.
// Build separately and link with --wrap=Z3_solver_check.
#include <cstdio>
#include "z3.h"

extern "C" Z3_lbool __real_Z3_solver_check(Z3_context, Z3_solver);

static void snapshot(const char* side, unsigned number, Z3_context ctx, Z3_solver solver) {
    Z3_stats stats = Z3_solver_get_statistics(ctx, solver);
    Z3_stats_inc_ref(ctx, stats);
    std::fprintf(stderr, "NATIVE-STATS %u %s", number, side);
    for (unsigned i = 0; i < Z3_stats_size(ctx, stats); ++i) {
        if (Z3_stats_is_uint(ctx, stats, i))
            std::fprintf(stderr, "|%s=%u", Z3_stats_get_key(ctx, stats, i), Z3_stats_get_uint_value(ctx, stats, i));
    }
    std::fputc('\n', stderr);
    Z3_stats_dec_ref(ctx, stats);
}

extern "C" Z3_lbool __wrap_Z3_solver_check(Z3_context ctx, Z3_solver solver) {
    static unsigned sequence;
    unsigned number = ++sequence;
    snapshot("before", number, ctx, solver);
    Z3_lbool result = __real_Z3_solver_check(ctx, solver);
    snapshot("after", number, ctx, solver);
    return result;
}
