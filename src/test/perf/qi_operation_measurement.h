// Observation-only hooks for the private native build used by operations.py.
#pragma once
#include <cstdint>
#include <cstdlib>

namespace qi_profile {
extern thread_local unsigned phase;
extern thread_local std::uint64_t counts[9][11];

inline void event(unsigned column, unsigned count = 1) {
    if (phase) counts[phase][column] += count;
}

inline unsigned shrink(unsigned from, unsigned to) {
    event(7, from - to);
    return to;
}

struct window {
    unsigned previous = phase;
    window() { if (std::getenv("IVY_QI_ACTIVE")) phase = 8; }
    ~window();
};
struct candidate {
    char const* outcome = nullptr;
    unsigned previous = phase;
    candidate(unsigned, unsigned, unsigned) { if (previous) phase = 1; }
    void next(unsigned n) { if (previous) phase = n + 1; }
    ~candidate() { phase = previous; }
};
struct pass {
    explicit pass(char const* name);
    ~pass();
    bool enabled;
};
void visit(void const* expression);
void retire(void const* expression);
}
