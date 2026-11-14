/*++
Module Name:

    stl_unordered_map_order_control.cpp

Abstract:

    Positive control for g3's port of libstdc++ std::unordered_map
    iteration order (pkg/util/stlhash). lp's lar_term keeps its
    coefficients in a std::unordered_map<unsigned, mpq>, and z3's pivoting
    choices depend on that map's iteration order, so g3 reproduces it.

    Standalone program (not part of the z3 test runner):

        g++ -O1 -std=c++17 -o /tmp/stlorder stl_unordered_map_order_control.cpp
        ./stlorder > pkg/util/stlhash/test_vectors/libstdcxx_unordered_map_order.txt

    Output lines:
        prime <n> <next_bkt(n)>        rehash policy bucket counts
        order <tag> <bucket_count> <k1> <k2> ...   iteration order

--*/
#include <cstdint>
#include <cstdio>
#include <unordered_map>
#include <vector>

typedef std::unordered_map<unsigned, int> umap;

static void dump(const char *tag, const umap &m) {
    std::printf("order %s %zu", tag, m.bucket_count());
    for (auto const &kv : m)
        std::printf(" %u", kv.first);
    std::printf("\n");
}

// Same 64-bit LCG as the Go test (Knuth MMIX constants).
static uint64_t lcg_state = 88172645463325252ull;
static uint32_t lcg_next() {
    lcg_state = lcg_state * 6364136223846793005ull + 1442695040888963407ull;
    return (uint32_t)(lcg_state >> 33);
}

int main() {
    // Bucket counts chosen by the rehash policy, for every hint that yields
    // a new count, up to just past 2^32.
    std::__detail::_Prime_rehash_policy policy;
    for (std::size_t n = 0; n < 20; ++n)
        std::printf("prime %zu %zu\n", n, policy._M_next_bkt(n));
    std::size_t n = 20;
    while (true) {
        std::size_t p = policy._M_next_bkt(n);
        std::printf("prime %zu %zu\n", n, p);
        if (p < n || p > (std::size_t(1) << 33))
            break;
        n = p + 1;
    }

    // Scenario 1: ascending inserts.
    {
        umap m;
        char tag[64];
        for (unsigned k = 0; k < 40; ++k) {
            m.emplace(k, 0);
            std::snprintf(tag, sizeof(tag), "asc%u", k);
            dump(tag, m);
        }
    }
    // Scenario 2: lar_term's (+ x y) shape.
    {
        umap m;
        m.emplace(1, 0);
        m.emplace(0, 0);
        dump("pair10", m);
        umap m2;
        m2.emplace(0, 0);
        m2.emplace(1, 0);
        dump("pair01", m2);
    }
    // Scenario 3: random toggles (insert if absent, erase if present).
    {
        umap m;
        char tag[64];
        for (unsigned step = 1; step <= 3000; ++step) {
            unsigned k = lcg_next() % 97;
            auto it = m.find(k);
            if (it == m.end())
                m.emplace(k, 0);
            else
                m.erase(it);
            if (step % 25 == 0) {
                std::snprintf(tag, sizeof(tag), "toggle%u", step);
                dump(tag, m);
            }
        }
        m.clear();
        m.emplace(5, 0);
        m.emplace(200, 0);
        m.emplace(3, 0);
        dump("afterclear", m);
    }
    // Scenario 4: large shuffled inserts with sparse keys.
    {
        umap m;
        std::vector<unsigned> keys;
        for (unsigned k = 0; k < 5000; ++k)
            keys.push_back(k * 7 + (lcg_next() % 7));
        for (std::size_t i = keys.size(); i > 1; --i) {
            std::size_t j = lcg_next() % i;
            unsigned t = keys[i - 1];
            keys[i - 1] = keys[j];
            keys[j] = t;
        }
        char tag[64];
        for (std::size_t i = 0; i < keys.size(); ++i) {
            m.emplace(keys[i], 0);
            if ((i + 1) % 1000 == 0) {
                std::snprintf(tag, sizeof(tag), "large%zu", i + 1);
                dump(tag, m);
            }
        }
        for (std::size_t i = 0; i < keys.size(); i += 3)
            m.erase(keys[i]);
        dump("largeerase", m);
    }
    return 0;
}
