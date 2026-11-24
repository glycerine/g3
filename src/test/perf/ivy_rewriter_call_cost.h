// Linux perf observer for a single rewriter operation in one preprocessing pass.
// This header is included only in the private benchmark objects.
#pragma once

#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <ctime>
#include <linux/perf_event.h>
#include <sys/syscall.h>
#include <unistd.h>

namespace ivy_rewriter_cost {

struct counts {
    int fd = -1;
    uint64_t calls = 0, samples = 0, sum = 0, offset = 0;
};

extern thread_local counts* active;

inline uint64_t instructions(counts const& s) {
    uint64_t v[3];
    if (read(s.fd, v, sizeof(v)) != sizeof(v) || v[1] != v[2])
        std::abort();
    return v[0];
}

inline uint64_t monotonic_ns() {
    timespec t;
    if (clock_gettime(CLOCK_MONOTONIC, &t))
        std::abort();
    return uint64_t(t.tv_sec) * 1000000000 + t.tv_nsec;
}

[[gnu::noinline]] inline uint64_t enter() {
    auto* s = active;
    if (!s || (++s->calls + s->offset) % 67)
        return 0;
    ++s->samples;
    return instructions(*s);
}

[[gnu::noinline]] inline void leave(uint64_t start) {
    if (start)
        active->sum += instructions(*active) - start;
}

struct sample {
    uint64_t start = enter();
    ~sample() { leave(start); }
};

[[gnu::noinline]] inline void calibrate() { sample empty; }

class pass {
    counts s;
    char const* name = nullptr;
    int tid = 0;
    uint64_t start = 0, empty = 0;

public:
    explicit pass(char const* n) {
        if (active || (strcmp(n, "propagate-values") && strcmp(n, "pattern-inference") &&
                       strcmp(n, "maximizing-bv-sharing")))
            return;
        perf_event_attr a{};
        a.type = PERF_TYPE_HARDWARE;
        a.size = sizeof(a);
        a.config = PERF_COUNT_HW_INSTRUCTIONS;
        a.exclude_kernel = a.exclude_hv = 1;
        a.read_format = PERF_FORMAT_TOTAL_TIME_ENABLED | PERF_FORMAT_TOTAL_TIME_RUNNING;
        s.fd = syscall(SYS_perf_event_open, &a, 0, -1, -1, 0);
        if (s.fd < 0)
            std::abort();
        name = n;
        tid = syscall(SYS_gettid);
        active = &s;
        for (int i = 0; i < 9; ++i) {
            s.calls = 66;
            calibrate();
        }
        empty = s.sum / s.samples;
        s.calls = s.samples = s.sum = 0;
        if (auto off = std::getenv("VISIT_OFFSET"))
            s.offset = std::strtoul(off, nullptr, 10);
        start = monotonic_ns();
    }

    ~pass() {
        if (!name)
            return;
        auto stop = monotonic_ns();
        active = nullptr;
        close(s.fd);
        std::fprintf(stderr, "VISIT-COST %s %d %llu %llu %llu %llu %llu %llu\n",
                     name, tid, (unsigned long long)start, (unsigned long long)stop,
                     (unsigned long long)s.calls, (unsigned long long)s.samples,
                     (unsigned long long)s.sum, (unsigned long long)empty);
    }
};
}
