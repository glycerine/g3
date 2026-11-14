/*++
Module Name:

    stl_unordered_set_order_control.cpp

Abstract:

    Positive control for g3's port of libstdc++ std::unordered_set<unsigned>
    iteration order (pkg/util/lp/std_unordered_set.go). lu.h's
    get_set_of_columns_to_replace_for_add_last_rows builds such a set and
    add_last_rows_to_B replaces basis columns in its iteration order.

    Standalone program (not part of the z3 test runner):

        g++ -O1 -std=c++17 -o /tmp/stlsetorder stl_unordered_set_order_control.cpp
        ./stlsetorder > pkg/util/lp/test_vectors/libstdcxx_unordered_set_order.txt

    Output lines:
        order <tag> <bucket_count> <k1> <k2> ...   iteration order

--*/
#include <cstdint>
#include <cstdio>
#include <unordered_set>

typedef std::unordered_set<unsigned> uset;

static void dump(const char *tag, const uset &s) {
    std::printf("order %s %zu", tag, s.bucket_count());
    for (unsigned k : s)
        std::printf(" %u", k);
    std::printf("\n");
}

// Same 64-bit LCG as the Go test (Knuth MMIX constants).
static uint64_t lcg_state = 88172645463325252ull;
static uint32_t lcg_next() {
    lcg_state = lcg_state * 6364136223846793005ull + 1442695040888963407ull;
    return (uint32_t)(lcg_state >> 33);
}

int main() {
    // Scenario 1: ascending inserts.
    {
        uset s;
        char tag[64];
        for (unsigned k = 0; k < 40; ++k) {
            s.insert(k);
            std::snprintf(tag, sizeof(tag), "asc%u", k);
            dump(tag, s);
        }
    }
    // Scenario 2: add_last_rows shape: several rows of cells, columns
    // repeating across rows (duplicate inserts are no-ops).
    {
        char tag[64];
        for (unsigned trial = 0; trial < 200; ++trial) {
            uset s;
            unsigned rows = 1 + lcg_next() % 6;
            unsigned width = 2 + lcg_next() % 60;
            for (unsigned r = 0; r < rows; ++r) {
                unsigned cells = 1 + lcg_next() % 12;
                for (unsigned c = 0; c < cells; ++c)
                    s.insert(lcg_next() % width);
            }
            std::snprintf(tag, sizeof(tag), "rows%u", trial);
            dump(tag, s);
        }
    }
    // Scenario 3: large sparse inserts with duplicates.
    {
        uset s;
        char tag[64];
        for (unsigned i = 1; i <= 6000; ++i) {
            s.insert(lcg_next() % 20000);
            if (i % 1000 == 0) {
                std::snprintf(tag, sizeof(tag), "large%u", i);
                dump(tag, s);
            }
        }
    }
    return 0;
}
