// Instruction and traversal counts for asserted-formula rewrites in one Z3 solver_push.
#include "api/z3.h"
#include "test/perf/thread_counter.h"
#include <cstdint>
#include <cstring>

namespace {
struct row_counts {
    char const* phase;
    char const* kind;
    std::uint64_t calls, steps, unchanged, instructions;
};
row_counts rows[128];
unsigned used;
bool active;
bool measure;
char const* phase;
std::uint64_t empty_read;
}

extern "C" unsigned long long ivy_clock() {
    if (!active || !measure) return 0;
    unsigned long long v[3];
    if (read(event_fd, v, sizeof(v)) != sizeof(v) || v[1] != v[2]) std::abort();
    return v[0];
}

extern "C" char const* ivy_row_phase(char const* next) {
    char const* prev = phase;
    phase = next;
    return prev;
}

extern "C" void ivy_row_note(char const* kind, unsigned steps, bool same, unsigned long long start) {
    if (!active) return;
    auto elapsed = measure ? ivy_clock() - start : 0;
    unsigned i = 0;
    for (; i < used; ++i)
        if (!std::strcmp(rows[i].phase, phase) && !std::strcmp(rows[i].kind, kind)) break;
    if (i == used) {
        if (used == 128) std::abort();
        rows[used++] = {phase, kind, 0, 0, 0, 0};
    }
    auto& r = rows[i];
    ++r.calls;
    r.steps += steps;
    r.unchanged += same;
    if (elapsed > empty_read) r.instructions += elapsed - empty_read;
}

extern "C" void __real_Z3_solver_push(Z3_context, Z3_solver);
extern "C" void __wrap_Z3_solver_push(Z3_context ctx, Z3_solver solver) {
    active = true;
    measure = std::getenv("SCOPE_THREAD_EVENT") != nullptr;
    used = 0;
    phase = "setup";
    empty_read = 0;
    if (measure) {
        open_counter();
        signal_counter(true);
        auto start = ivy_clock();
        empty_read = ivy_clock() - start;
    }
    __real_Z3_solver_push(ctx, solver);
    active = false;
    if (measure) {
        signal_counter(false);
        close(event_fd);
    }
    for (unsigned i = 0; i < used; ++i) {
        auto const& r = rows[i];
        std::fprintf(stderr, "CENSUS-NATIVE %s %s %llu %llu %llu %llu\n",
            r.phase, r.kind, (unsigned long long)r.calls, (unsigned long long)r.steps,
            (unsigned long long)r.unchanged, (unsigned long long)r.instructions);
    }
}
