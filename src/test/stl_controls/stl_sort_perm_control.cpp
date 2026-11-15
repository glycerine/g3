/*++
Module Name:

    stl_sort_perm_control.cpp

Abstract:

    Positive control for the libstdc++ std::sort call sites approved on
    2026-11-14 (AUDIT.md opt_01-F3, util_lp_05-F1, math_polynomial_04-F1,
    muz_transforms_02-F5, math_misc_01-F4). Reads lines "<kind> k0 k1 ..."
    and prints the index permutation std::sort leaves; the Go unit tests at
    those sites quote this program's output.
--*/
//   lt : comp(a,b) = key[a] < key[b]      (lt_degree, rule_size_comparator)
//   gt : comp(a,b) = key[a] > key[b]      (maxres sort_assumptions, mutex, steepest edge)
//   nz : comp(a,b) = ca==0&&cb!=0 ? false : ca<cb   (sort_non_basis_rational)
//
//   g++ -O1 -std=c++17 -o /tmp/stlsortperm stl_sort_perm_control.cpp
//   echo "gt 2 2 2 3 1 1 3 1 2 3 3 3 3 3 1 2 1" | /tmp/stlsortperm
#include <algorithm>
#include <cstdio>
#include <iostream>
#include <sstream>
#include <string>
#include <vector>
int main() {
    std::string line;
    while (std::getline(std::cin, line)) {
        std::istringstream in(line);
        std::string kind;
        in >> kind;
        std::vector<long> key;
        long k;
        while (in >> k) key.push_back(k);
        std::vector<unsigned> idx(key.size());
        for (unsigned i = 0; i < idx.size(); ++i) idx[i] = i;
        if (kind == "lt")
            std::sort(idx.begin(), idx.end(), [&](unsigned a, unsigned b) { return key[a] < key[b]; });
        else if (kind == "gt")
            std::sort(idx.begin(), idx.end(), [&](unsigned a, unsigned b) { return key[a] > key[b]; });
        else if (kind == "nz")
            std::sort(idx.begin(), idx.end(), [&](unsigned a, unsigned b) {
                long ca = key[a], cb = key[b];
                if (ca == 0 && cb != 0) return false;
                return ca < cb;
            });
        std::printf("%s:", kind.c_str());
        for (unsigned i : idx) std::printf(" %u", i);
        std::printf("\n");
    }
}
