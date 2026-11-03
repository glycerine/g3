// Reproducer for the recursive QBMC proof case in bmc_numeric_proof_test.go.
// Build against the Z3 revision being investigated; the process returns failure
// if QBMC cannot answer the reachable query or construct its answer.
#include <iostream>
#include "z3++.h"

int main() {
    try {
        unsigned major, minor, build, revision;
        Z3_get_version(&major, &minor, &build, &revision);
        std::cout << "Z3 " << major << '.' << minor << '.' << build << '.'
                  << revision << std::endl;
        z3::config cfg;
        cfg.set("proof", true);
        z3::context ctx(cfg);
        z3::fixedpoint fp(ctx);
        z3::params p(ctx);
        p.set("engine", "qbmc");
        p.set("xform.inline_eager", false);
        p.set("xform.inline_linear", false);
        p.set("xform.slice", false);
        p.set("xform.subsumption_checker", false);
        p.set("xform.coalesce_rules", false);
        p.set("xform.unfold_rules", 0u);
        fp.set(p);
        auto queries = fp.from_string(R"(
(declare-rel numeric-goal ())
(declare-rel numeric-left (Int))
(declare-rel numeric-right (Int))
(declare-rel numeric-leaf (Int))
(declare-var x Int)
(rule (! (numeric-leaf 0) :named recursive-base))
(rule (! (=> (and (numeric-leaf x) (< x 3)) (numeric-leaf (+ x 1))) :named recursive-successor))
(rule (! (=> (numeric-leaf 3) numeric-goal) :named recursive-goal))
(query numeric-goal)
)");
        if (queries.size() != 1) {
            std::cerr << "expected exactly one query, got " << queries.size() << '\n';
            return 1;
        }
        z3::expr query = queries[0];
        auto result = fp.query(query);
        if (result != z3::sat) {
            std::cerr << "expected reachable goal, got " << result << '\n';
            return 1;
        }
        std::cout << "sat\n" << fp.get_answer() << '\n';
        return 0;
    } catch (z3::exception const& ex) {
        std::cerr << "QBMC failed: " << ex.msg() << '\n';
        return 1;
    }
}
