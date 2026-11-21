// A Linux hardware counter for the calling benchmark thread.
#pragma once

#include <cstdio>
#include <cstdlib>
#include <unistd.h>
#include <linux/perf_event.h>
#include <sys/ioctl.h>
#include <sys/syscall.h>
static int event_fd;

static void open_counter() {
    perf_event_attr attr = {};
    attr.type = PERF_TYPE_HARDWARE;
    attr.size = sizeof(attr);
    attr.config = std::strtoul(std::getenv("SCOPE_THREAD_EVENT"), nullptr, 10);
    attr.disabled = attr.exclude_kernel = attr.exclude_hv = 1;
    attr.read_format = PERF_FORMAT_TOTAL_TIME_ENABLED | PERF_FORMAT_TOTAL_TIME_RUNNING;
    event_fd = syscall(__NR_perf_event_open, &attr, 0, -1, -1, 0);
    if (event_fd < 0) { std::perror("perf_event_open"); std::abort(); }
}

static void signal_counter(bool begin) {
    if (ioctl(event_fd, begin ? PERF_EVENT_IOC_ENABLE : PERF_EVENT_IOC_DISABLE, 0) < 0)
        std::abort();
}

static void report_counter() {
    unsigned long long values[3];
    if (read(event_fd, values, sizeof(values)) != sizeof(values)) std::abort();
    close(event_fd);
    std::printf("SCOPE-THREAD %llu %llu %llu\n", values[0], values[1], values[2]);
}

