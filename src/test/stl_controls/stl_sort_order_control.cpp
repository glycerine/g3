/*++
Module Name:

    stl_sort_order_control.cpp

Abstract:

    Positive control for g3's DELIBERATE, USER-APPROVED (2026-11-13) port of
    libstdc++ (GCC 13) std::sort (pkg/smtcore/libstdcxx_sort.go).

    theory_arith<Ext>::flush_bound_axioms (src/smt/theory_arith_core.h)
    sorts atoms/occs with std::sort and compare_atoms, which compares only
    the bound value k. Atoms with equal k are ties, and once there are more
    than 16 of them std::sort's introsort leaves the ties in an order that is
    neither creation order nor any stable order. That order decides the
    bound-axiom clause creation order, so g3 reproduces it exactly.

    This program sorts (key, tag) pairs with std::sort and a key-only
    comparator (the same shape as compare_atoms) and prints the resulting
    tag order. TestLibstdcxxSortMatchesControl
    (pkg/smtcore/libstdcxx_sort_test.go) replays the same inputs (same LCG)
    through the Go port and must match every line exactly.

    Standalone program (not part of the z3 test runner):

        g++ -O1 -std=c++17 -o /tmp/stlsortorder stl_sort_order_control.cpp
        /tmp/stlsortorder > pkg/smtcore/test_vectors/libstdcxx_sort_order.txt

    Output lines:
        input <tag> <k1> <k2> ...   keys of an adversarial input (only for
                                    the killer scenario; the Go test reads
                                    them instead of re-deriving them)
        order <tag> <t1> <t2> ...   tags after std::sort

--*/
#include <algorithm>
#include <cstdint>
#include <cstdio>
#include <vector>

struct item {
    unsigned key;
    unsigned tag;
};

struct key_only_less {
    bool operator()(const item &a, const item &b) const { return a.key < b.key; }
};

// Same 64-bit LCG as the Go test (Knuth MMIX constants).
static uint64_t lcg_state = 88172645463325252ull;
static uint32_t lcg_next() {
    lcg_state = lcg_state * 6364136223846793005ull + 1442695040888963407ull;
    return (uint32_t)(lcg_state >> 33);
}

static void sort_and_dump(const char *tag, std::vector<unsigned> const &keys) {
    std::vector<item> v;
    for (unsigned i = 0; i < keys.size(); ++i)
        v.push_back(item{keys[i], i});
    std::sort(v.begin(), v.end(), key_only_less());
    std::printf("order %s", tag);
    for (item const &it : v)
        std::printf(" %u", it.tag);
    std::printf("\n");
}

// McIlroy's "A Killer Adversary for Quicksort" (1999): drives introsort to
// its depth limit so the heap-sort fallback (__partial_sort) runs.
static std::vector<unsigned> killer_val;
static unsigned killer_gas, killer_nsolid, killer_candidate;
static unsigned killer_freeze(unsigned x) {
    killer_val[x] = killer_nsolid++;
    return x;
}
struct killer_cmp {
    bool operator()(unsigned x, unsigned y) const {
        if (killer_val[x] == killer_gas && killer_val[y] == killer_gas) {
            if (x == killer_candidate)
                killer_freeze(x);
            else
                killer_freeze(y);
        }
        if (killer_val[x] == killer_gas)
            killer_candidate = x;
        else if (killer_val[y] == killer_gas)
            killer_candidate = y;
        return killer_val[x] < killer_val[y];
    }
};
static std::vector<unsigned> killer_input(unsigned n) {
    killer_val.assign(n, n);
    killer_gas = n;
    killer_nsolid = 0;
    killer_candidate = 0;
    std::vector<unsigned> ptr(n);
    for (unsigned i = 0; i < n; ++i)
        ptr[i] = i;
    std::sort(ptr.begin(), ptr.end(), killer_cmp());
    for (unsigned i = 0; i < n; ++i)
        if (killer_val[i] == killer_gas)
            killer_freeze(i);
    return killer_val;
}

int main() {
    char tag[64];
    // Scenario 1: random keys with many duplicates, sizes 0..300, several
    // duplicate densities (range 1 = all ties).
    unsigned const ranges[] = {1, 3, 20};
    for (unsigned r : ranges) {
        for (unsigned n = 0; n <= 300; ++n) {
            std::vector<unsigned> keys;
            for (unsigned i = 0; i < n; ++i)
                keys.push_back(lcg_next() % r);
            std::snprintf(tag, sizeof(tag), "rand_r%u_n%u", r, n);
            sort_and_dump(tag, keys);
        }
    }
    // Scenario 2: already sorted and reverse-sorted inputs with duplicates.
    for (unsigned n = 0; n <= 300; n += 7) {
        for (unsigned d = 1; d <= 3; d += 2) {
            std::vector<unsigned> asc, desc;
            for (unsigned i = 0; i < n; ++i) {
                asc.push_back(i / d);
                desc.push_back((n - i) / d);
            }
            std::snprintf(tag, sizeof(tag), "asc_d%u_n%u", d, n);
            sort_and_dump(tag, asc);
            std::snprintf(tag, sizeof(tag), "desc_d%u_n%u", d, n);
            sort_and_dump(tag, desc);
        }
    }
    // Scenario 3: flush_bound_axioms shape: a few distinct bound values,
    // each shared by a (<= x k) and a (>= x k) atom, in creation order.
    for (unsigned n = 2; n <= 120; ++n) {
        std::vector<unsigned> keys;
        for (unsigned i = 0; i < n; ++i)
            keys.push_back((i / 2) % (1 + lcg_next() % 9));
        std::snprintf(tag, sizeof(tag), "pairs_n%u", n);
        sort_and_dump(tag, keys);
    }
    // Scenario 4: adversarial inputs that exhaust the depth limit, so the
    // heap-sort fallback runs; keys are halved to create ties.
    unsigned const killers[] = {40, 64, 100, 200, 257, 300};
    for (unsigned n : killers) {
        std::vector<unsigned> keys = killer_input(n);
        for (unsigned &k : keys)
            k /= 2;
        std::snprintf(tag, sizeof(tag), "killer_n%u", n);
        std::printf("input %s", tag);
        for (unsigned k : keys)
            std::printf(" %u", k);
        std::printf("\n");
        sort_and_dump(tag, keys);
    }
    return 0;
}
