// Linux perf stat FIFO control for the standalone, instrumented scope fixture.
// The driver supplies a temporary copy of the fixture with measurement hooks.
#include <cerrno>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fcntl.h>
#include <unistd.h>
#include "util/memory_manager.h"

bool scope_phase_active = false;
unsigned scope_phase_visits = 0;
static int selected_phase;
#ifdef G3_PHASE_THREAD
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

#else
static int control_fd, reply_fd;
static void open_counter() {
    control_fd = open(std::getenv("SCOPE_CONTROL"), O_WRONLY);
    reply_fd = open(std::getenv("SCOPE_ACK"), O_RDONLY);
    if (control_fd < 0 || reply_fd < 0) std::abort();
}

static void transfer(int fd, char* buffer, size_t bytes, bool send) {
    while (bytes) {
        ssize_t count = send ? write(fd, buffer, bytes) : read(fd, buffer, bytes);
        if (count < 0 && errno == EINTR) continue;
        if (count <= 0) std::abort();
        bytes -= count;
        buffer += count;
    }
}

static void signal_counter(bool begin) {
    char enable[] = "enable\n", disable[] = "disable\n";
    char* command = begin ? enable : disable;
    transfer(control_fd, command, std::strlen(command), true);
    char reply[4] = {};
    // Some versions of perf append a NUL after the newline.
    do { transfer(reply_fd, reply, 1, false); } while (reply[0] == 0);
    transfer(reply_fd, reply + 1, 3, false);
    if (std::memcmp(reply, "ack\n", 4)) std::abort();
}

static void report_counter() {}
#endif

void scope_phase_event(int id, bool begin) {
    if (!scope_phase_active || id != selected_phase) return;
    if (begin) ++scope_phase_visits;
    signal_counter(begin);
}

#include "smt_scope_restoration.cpp"

int main(int argc, char** argv) {
    if (argc != 3) return 2;
    memory::initialize(0);
    selected_phase = std::stoi(std::getenv("SCOPE_PHASE"));
    open_counter();
    std::string name(argv[2]);
    if (name != "ScopedAtoms" && name != "SurvivingAtoms") return 4;
    benchmark_scope_restoration(std::stoul(argv[1]), name == "ScopedAtoms");
    report_counter();
    std::cout << "intervals " << scope_phase_visits << "\n";
}
