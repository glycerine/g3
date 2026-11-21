// Linux-only observer for the instrumented qi_queue translation unit.
#include <linux/perf_event.h>
#include <unistd.h>
#include <sys/syscall.h>
#include <cstdint>
#include <cstdlib>
#include <cstdio>

namespace {
struct qi_observation {
    int fd = -1;
    unsigned ordinal = 0, id = 0, generation = 0, bindings = 0, phase = 0;
    const char *outcome = "satisfied";
    uint64_t last = 0, empty = 0, instructions[7] = {};
    uint64_t read_count() {
        uint64_t data[3];
        if (read(fd, data, sizeof(data)) != sizeof(data) || data[1] != data[2]) std::abort();
        return data[0];
    }
    qi_observation(unsigned qid, unsigned gen, unsigned n): id(qid), generation(gen), bindings(n) {
        if (!std::getenv("IVY_QI_ACTIVE")) return;
        static thread_local int counter = -1;
        static thread_local unsigned seq = 0;
        if (counter < 0) {
            perf_event_attr a = {};
            a.size = sizeof(a); a.type = PERF_TYPE_HARDWARE; a.config = PERF_COUNT_HW_INSTRUCTIONS;
            a.exclude_kernel = a.exclude_hv = 1;
            a.read_format = PERF_FORMAT_TOTAL_TIME_ENABLED | PERF_FORMAT_TOTAL_TIME_RUNNING;
            counter = syscall(SYS_perf_event_open, &a, 0, -1, -1, 0);
            if (counter < 0) { perror("perf_event_open"); std::abort(); }
        }
        fd=counter; ordinal=++seq;
        uint64_t a=read_count(); last=read_count(); empty=last-a;
    }
    void next(unsigned p) {
        if (fd < 0) return;
        uint64_t now=read_count(), delta=now-last;
        if (delta>empty) instructions[phase]+=delta-empty;
        phase=p; last=now;
    }
    ~qi_observation() {
        if (fd<0) return;
        next(6);
        std::fprintf(stderr,"IVY-QI %u %u %u %u %s %llu",ordinal,id,generation,bindings,outcome,(unsigned long long)empty);
        for(auto n:instructions) std::fprintf(stderr," %llu",(unsigned long long)n);
        std::fputc('\n',stderr);
    }
};
}
