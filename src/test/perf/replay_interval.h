// A single translated method in the Linux replay benchmark uses this scope.
// Nested calls count separately. Instructions accrue to the outermost call.
#pragma once
#include "test/perf/thread_counter.h"

namespace replay_measurement {
struct totals {
    unsigned long long instructions = 0, calls = 0, empty = 0, depth = 0;
    ~totals() {
        std::fprintf(stderr, "REPLAY-OP %llu %llu\n", calls, instructions);
        if (calls) close(event_fd);
    }
};
static totals total;

static unsigned long long read_count() {
    unsigned long long data[3];
    if (read(event_fd, data, sizeof(data)) != sizeof(data) || data[1] != data[2]) std::abort();
    return data[0];
}

struct interval {
    unsigned long long start = 0;
    interval() {
        if (!total.calls++) {
            open_counter();
            signal_counter(true);
            auto lo = read_count();
            total.empty = read_count() - lo;
        }
        if (!total.depth++) start = read_count();
    }
    ~interval() {
        if (--total.depth == 0) {
            auto n = read_count() - start;
            if (n > total.empty) total.instructions += n - total.empty;
        }
    }
};
}
