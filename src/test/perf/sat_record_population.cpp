// Companion to the Go watch population and justification column benchmarks.
#include "sat/sat_watched.h"
#include "sat/sat_justification.h"
#include <chrono>
#include <cstdint>
#include <cstdlib>
#include <iostream>
#include <string>
#include <vector>

static_assert(sizeof(sat::watched) == 8, "expected two native watch words");
static_assert(sizeof(sat::justification) == 8, "expected two native reason words");

namespace {
uint64_t value(sat::watched const& w) {
    switch (w.get_kind()) {
    case sat::watched::BINARY:
        return w.get_literal().to_uint() + unsigned(w.is_learned());
    case sat::watched::TERNARY:
        return w.get_literal1().to_uint() + w.get_literal2().to_uint();
    case sat::watched::CLAUSE:
        return w.get_blocked_literal().to_uint() + w.get_clause_offset();
    default:
        std::abort();
    }
}

int watches(std::string const& mode, unsigned rounds) {
    constexpr unsigned workers = 8;
    unsigned lists = mode == "Short" ? 2048 : 32;
    unsigned count = mode == "Short" ? 4 : 256;
    std::vector<sat::watch_list> rows(workers * lists);
    uint64_t expected = 0, capacity = 0;
    for (auto& list : rows) {
        for (unsigned j = 0; j < count; ++j) {
            unsigned n = 2 * j + 3;
            auto literal = sat::to_literal(n);
            if (mode == "Binary" || j % 4 < 2) {
                list.push_back(sat::watched(literal, j % 2 == 0));
                expected += n + unsigned(j % 2 == 0);
            }
            else if (j % 4 == 2) {
                list.push_back(sat::watched(literal, sat::to_literal(n + 1)));
                expected += 2 * n + 1;
            }
            else {
                list.push_back(sat::watched(literal, sat::clause_offset(n)));
                expected += 2 * n;
            }
        }
        capacity += list.capacity() * sizeof(sat::watched);
    }
    uint64_t sum = 0;
    auto start = std::chrono::steady_clock::now();
    for (unsigned r = 0; r < rounds; ++r)
        for (auto const& list : rows)
            for (auto const& w : list) sum += value(w);
    auto ns = std::chrono::duration<double, std::nano>(std::chrono::steady_clock::now() - start).count();
    uint64_t total = uint64_t(rows.size()) * count;
    if (sum != expected * rounds) return 2;
    std::cout << mode << " rows=" << total << " row_bytes=" << sizeof(sat::watched)
              << " payload_capacity_bytes=" << capacity << " lists=" << rows.size()
              << " ns/read=" << ns / total / rounds << " checksum=" << sum << '\n';
    return 0;
}

int reasons(std::string const& mode, unsigned rounds) {
    constexpr unsigned size = 16384;
    std::vector<sat::justification> data(size);
    unsigned every = mode == "ReasonBinary" ? 0 : mode == "ReasonMixed" ? 8 : 1;
    uint64_t sum = 0, expected = 0;
    auto start = std::chrono::steady_clock::now();
    for (unsigned j = 0; j < rounds; ++j) {
        unsigned i = j * 4271u & (size - 1);
        bool clause = every && i % every == 0;
        data[i] = clause ? sat::justification(sat::clause_offset(i + 1))
                         : sat::justification(sat::to_literal(i));
        auto const& reason = data[i];
        if (reason.is_clause()) sum += reason.get_clause_offset();
        else if (reason.is_binary_clause()) sum += reason.get_literal().to_uint();
        else return 2;
        expected += i + unsigned(clause);
    }
    auto ns = std::chrono::duration<double, std::nano>(std::chrono::steady_clock::now() - start).count();
    if (sum != expected) return 2;
    std::cout << mode << " rows=" << size << " row_bytes=" << sizeof(sat::justification)
              << " ns/write_read=" << ns / rounds << " checksum=" << sum << '\n';
    return 0;
}
}

int main(int argc, char** argv) {
    if (argc != 3) return 1;
    std::string mode(argv[1]);
    char* end = nullptr;
    unsigned long n = std::strtoul(argv[2], &end, 10);
    if (!n || *end || n > UINT32_MAX) return 1;
    memory::initialize(0);
    if (mode == "Binary" || mode == "Mixed" || mode == "Short") return watches(mode, n);
    if (mode == "ReasonBinary" || mode == "ReasonMixed" || mode == "ReasonClause") return reasons(mode, n);
    return 1;
}
