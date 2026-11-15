/*++
Module Name:

    stl_lar_signature_order_control.cpp

Abstract:

    Positive control for g3's LARSolutionSignature
    (pkg/util/lp/lar_signature.go, AUDIT.md util_lp_03-F1).
    lar_solution_signature is a
    std::unordered_map<unsigned, non_basic_column_value_position>;
    lar_core_solver::solve fills one local signature twice through
    extract_signature_from_lp_core_solver (clear(), then operator[] in
    ascending column order), and prepare_solver_x_with_signature walks it
    in node order, drawing a random bit per boxed not_at_bound column.
    clear() keeps the bucket array, so the second fill's order depends on
    the first.

    Standalone program (not part of the z3 test runner):

        g++ -O1 -std=c++17 -o /tmp/larsig stl_lar_signature_order_control.cpp
        /tmp/larsig > pkg/util/lp/test_vectors/libstdcxx_lar_signature_order.txt

    Output lines:
        order <tag> <bucket_count> <k1> <k2> ...   iteration order

--*/
#include <cstdio>
#include <unordered_map>
#include <vector>

typedef std::unordered_map<unsigned, int> signature;

static void dump(const char *tag, const signature &m) {
    std::printf("order %s %zu", tag, m.bucket_count());
    for (auto const &kv : m)
        std::printf(" %u", kv.first);
    std::printf("\n");
}

static void fill(signature &m, std::vector<unsigned> const &keys) {
    m.clear();
    for (unsigned j : keys)
        m[j] = 1;
}

int main() {
    std::vector<unsigned> first = {0, 2, 3, 5, 7, 8, 11, 12, 20, 21, 22, 23, 30};
    std::vector<unsigned> second = {1, 2, 4, 6, 9, 13, 14, 15, 16, 30, 31};
    std::vector<unsigned> third;
    for (unsigned j = 0; j < 60; j += 3)
        third.push_back(j);
    std::vector<unsigned> small = {4, 9};
    signature s;
    fill(s, first);
    dump("first", s);
    fill(s, second);
    dump("cleared-second", s);
    fill(s, third);
    dump("cleared-third", s);
    fill(s, small);
    dump("cleared-small", s);
    signature fresh;
    fill(fresh, small);
    dump("fresh-small", fresh);
    // The nonbasic boxed columns of the preparation test.
    signature boxed;
    fill(boxed, {1, 2, 3, 4, 5});
    dump("fresh-boxed", boxed);
    return 0;
}
