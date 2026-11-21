// Compiled in the private instrumented qi_queue translation unit.
#include "util/qi_operation_measurement.h"
#include <cstdio>
#include <unordered_set>
#include <vector>

namespace qi_profile {
thread_local unsigned phase = 0;
thread_local std::uint64_t counts[9][11] = {};
struct pass_stats {
    char const* name;
    unsigned ordinal;
    pass_stats(char const* name, unsigned ordinal): name(name), ordinal(ordinal) {}
    unsigned visits = 0, distinct = 0;
    std::unordered_set<void const*> live;
};
static thread_local std::vector<pass_stats> passes;
static thread_local unsigned pass_ordinal = 0;

window::~window() {
    if (phase && !previous) {
        for (unsigned p = 1; p <= 8; ++p) {
            std::fprintf(stderr, "IVY-OPS %u", p);
            for (auto n : counts[p]) std::fprintf(stderr, " %llu", static_cast<unsigned long long>(n));
            std::fputc('\n', stderr);
        }
    }
    phase = previous;
}

pass::pass(char const* name) : enabled(phase != 0) {
    if (enabled) passes.emplace_back(name, ++pass_ordinal);
}
pass::~pass() {
    if (enabled) {
        auto const& p = passes.back();
        std::fprintf(stderr, "IVY-PASS %u %s %u %u\n", p.ordinal, p.name, p.visits, p.distinct);
        passes.pop_back();
    }
}
void visit(void const* n) {
    if (!passes.empty()) {
        auto& p = passes.back();
        ++p.visits;
        if (p.live.insert(n).second) ++p.distinct;
    }
}
void retire(void const* n) {
    for (auto& p : passes) p.live.erase(n);
}
}
