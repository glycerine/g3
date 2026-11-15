/*++
Module Name:

    stl_unordered_string_map_order_control.cpp

Abstract:

    Positive control for g3's DELIBERATE, USER-APPROVED (2026-11-14)
    emulation of libstdc++ (GCC 13) std::unordered_map<std::string, unsigned>
    (pkg/util/lp/std_unordered_string_map.go, AUDIT.md util_lp_06-F1).
    lp_solver::print_model walks m_names_to_columns, and mps_reader's
    m_columns decides the insertion order, so the MPS front end's
    "column values" order is this container's node order.

    Standalone program (not part of the z3 test runner):

        g++ -O1 -std=c++17 -o /tmp/stlstrmap stl_unordered_string_map_order_control.cpp
        /tmp/stlstrmap > pkg/util/lp/test_vectors/libstdcxx_unordered_string_map_order.txt

    Output lines (words are hex encoded):
        hash <word> <std::hash<std::string>>
        order <tag> <bucket_count> <word> <word> ...

--*/
#include <cstdint>
#include <cstdio>
#include <string>
#include <unordered_map>
#include <vector>

// Same 64-bit LCG as the Go test (Knuth MMIX constants).
static uint64_t lcg_state = 88172645463325252ull;
static uint32_t lcg_next() {
    lcg_state = lcg_state * 6364136223846793005ull + 1442695040888963407ull;
    return (uint32_t)(lcg_state >> 33);
}

static std::string hex(std::string const &s) {
    static char const *digits = "0123456789abcdef";
    std::string out;
    for (unsigned char c : s) {
        out.push_back(digits[c >> 4]);
        out.push_back(digits[c & 15]);
    }
    return out.empty() ? "-" : out;
}

typedef std::unordered_map<std::string, unsigned> smap;

static void dump(char const *tag, smap const &m) {
    std::printf("order %s %zu", tag, m.bucket_count());
    for (auto const &kv : m)
        std::printf(" %s", hex(kv.first).c_str());
    std::printf("\n");
}

static void put(smap &m, std::string const &w, unsigned v) {
    if (m.find(w) == m.end())
        std::printf("hash %s %zu\n", hex(w).c_str(), std::hash<std::string>()(w));
    m[w] = v;
}

int main() {
    char tag[64];
    // Scenario 1: the util_lp_06-F1 MPS column names in read order (as
    // mps_reader fills m_columns), then the names in m_columns' iteration
    // order (as fill_solver_on_columns fills m_names_to_columns).
    std::vector<std::string> mps = {"XONE", "YTWO", "ZTHREE", "ALPHA", "BETA", "GAMMA"};
    smap columns;
    for (unsigned i = 0; i < mps.size(); ++i) {
        put(columns, mps[i], i);
        std::snprintf(tag, sizeof(tag), "mps%u", i);
        dump(tag, columns);
    }
    smap names;
    for (auto const &kv : columns)
        put(names, kv.first, kv.second);
    dump("names", names);
    // Scenario 2: 300 random words of length 0..20 over all byte values,
    // order every 25 inserts, then every third word erased.
    smap m;
    std::vector<std::string> words;
    for (unsigned i = 0; i < 300; ++i) {
        unsigned len = lcg_next() % 21;
        std::string w;
        for (unsigned j = 0; j < len; ++j)
            w.push_back((char)(lcg_next() & 255));
        words.push_back(w);
        put(m, w, i);
        if ((i + 1) % 25 == 0) {
            std::snprintf(tag, sizeof(tag), "rand%u", i + 1);
            dump(tag, m);
        }
    }
    for (unsigned i = 0; i < words.size(); i += 3)
        m.erase(words[i]);
    dump("randerase", m);
    for (unsigned i = 0; i < words.size(); i += 6)
        put(m, words[i], i);
    dump("randreinsert", m);
}
