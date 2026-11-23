// Count retired user instructions in qi_queue::instantiate and its stages.
// The benchmark adapter runs one verification at a time on this thread.
#include "api/z3.h"
#include "test/perf/thread_counter.h"
#include <cstring>

static bool active;
static unsigned long long empty, rows[6][2];

extern "C" unsigned long long qi_read() {
    if (!active) return 0;
    unsigned long long values[3];
    if (read(event_fd, values, sizeof(values)) != sizeof(values) || values[1] != values[2])
        std::abort();
    return values[0];
}

extern "C" void qi_note(unsigned long long start, int stage) {
    if (start) {
        auto n = qi_read() - start;
        ++rows[stage][0];
        rows[stage][1] += n > empty ? n - empty : 0;
    }
}

extern "C" Z3_lbool __real_Z3_solver_check(Z3_context, Z3_solver);
extern "C" Z3_lbool __wrap_Z3_solver_check(Z3_context ctx, Z3_solver solver) {
    open_counter();
    signal_counter(true);
    active = true;
    auto start = qi_read();
    empty = qi_read() - start;
    std::memset(rows, 0, sizeof(rows));
    auto result = __real_Z3_solver_check(ctx, solver);
    active = false;
    close(event_fd);
    for (int i = 0; i < 6; ++i)
        if (rows[i][0])
            std::fprintf(stderr, "Q-STAGE %d %llu %llu\n", i, rows[i][0], rows[i][1]);
    return result;
}
