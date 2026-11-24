// Linux instruction counts for the first-push pass comparison.
#pragma once
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <ctime>
#include <linux/perf_event.h>
#include <sys/syscall.h>
#include <unistd.h>

namespace {
class pass_instruction_span {
    char const* m_name;
    int m_fd, m_tid;
    std::uint64_t m_start, m_empty, m_start_ns;

    static std::uint64_t clock_ns() {
        timespec t;
        if (clock_gettime(CLOCK_MONOTONIC, &t))
            std::abort();
        return std::uint64_t(t.tv_sec) * 1000000000 + t.tv_nsec;
    }

    std::uint64_t counter() const {
        std::uint64_t values[3];
        if (read(m_fd, values, sizeof(values)) != sizeof(values) || values[1] != values[2])
            std::abort();
        return values[0];
    }

public:
    explicit pass_instruction_span(char const* name): m_name(name), m_tid(syscall(SYS_gettid)) {
        perf_event_attr attr{};
        attr.size = sizeof(attr);
        attr.type = PERF_TYPE_HARDWARE;
        attr.config = PERF_COUNT_HW_INSTRUCTIONS;
        attr.exclude_kernel = attr.exclude_hv = 1;
        attr.read_format = PERF_FORMAT_TOTAL_TIME_ENABLED | PERF_FORMAT_TOTAL_TIME_RUNNING;
        m_fd = syscall(SYS_perf_event_open, &attr, 0, -1, -1, 0);
        if (m_fd < 0)
            std::abort();
        auto a = counter();
        m_empty = counter() - a;
        m_start_ns = clock_ns();
        m_start = counter();
    }

    ~pass_instruction_span() {
        auto end = counter();
        auto end_ns = clock_ns();
        close(m_fd);
        std::fprintf(stderr, "ASSERTED-PASS %s %d %llu %llu %llu %llu\n", m_name, m_tid,
                     static_cast<unsigned long long>(m_start_ns), static_cast<unsigned long long>(end_ns),
                     static_cast<unsigned long long>(end - m_start), static_cast<unsigned long long>(m_empty));
    }
};
}
