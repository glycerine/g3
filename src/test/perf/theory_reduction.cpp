// Counterpart to BenchmarkTheoryReduction. The configuration is private to
// th_rewriter.cpp, which this standalone executable links in place of its
// corresponding object in libz3.a.
#include "ast/rewriter/th_rewriter.cpp"
#include "ast/reg_decl_plugins.h"
#include <chrono>
#include <iomanip>
#include <iostream>
#include <string>

namespace {
using reduction = br_status (*)(th_rewriter_cfg&, func_decl*, unsigned, expr* const*, expr_ref&);

br_status core(th_rewriter_cfg& cfg, func_decl* f, unsigned n, expr* const* args, expr_ref& result) {
    return cfg.reduce_app_core(f, n, args, result);
}
br_status blank(th_rewriter_cfg&, func_decl*, unsigned, expr* const*, expr_ref&) {
    return BR_FAILED;
}

bool run(std::string const& filter, std::string const& mode, unsigned iterations) {
    ast_manager m;
    reg_decl_plugins(m);
    params_ref p;
    th_rewriter_cfg cfg(m, p);
    sort* b = m.mk_bool_sort();
    app_ref x(m.mk_const(symbol("x"), b), m), y(m.mk_const(symbol("y"), b), m);
    sort* domain[] = {b, b};
    func_decl_ref f(m.mk_func_decl(symbol("f"), 2, domain, b), m);
    app_ref neg(m.mk_not(x), m);
    app_ref_vector roots(m);
    roots.push_back(m.mk_app(f.get(), x.get(), y.get()));
    roots.push_back(neg);
    roots.push_back(m.mk_not(neg));
    roots.push_back(m.mk_or(x, y));
    roots.push_back(m.mk_or(x, x));
    roots.push_back(m.mk_iff(x, y));
    char const* names[] = {"UF", "Not", "DoubleNot", "Or", "OrDuplicate", "Iff"};
    expr* expected[] = {nullptr, nullptr, x.get(), nullptr, x.get(), nullptr};
    bool measured = false;
    for (unsigned i = 0; i < roots.size(); ++i) {
        if (!filter.empty() && filter != names[i]) continue;
        app* root = roots.get(i);
        // The Go traversal supplies a contiguous slice of rewritten children.
        // Make the same borrowed array once, outside the timed native loop.
        expr* args[2] = {};
        for (unsigned j = 0; j < root->get_num_args(); ++j) args[j] = root->get_arg(j);
        for (bool empty : {false, true}) {
            std::string name = empty ? "Blank" : "Core";
            if (!mode.empty() && mode != name) continue;
            measured = true;
            expr_ref result(m);
            br_status status = BR_FAILED;
            reduction volatile dispatch = empty ? blank : core;
            unsigned n = root->get_num_args();
            func_decl* decl = root->get_decl();
            auto begin = std::chrono::steady_clock::now();
            for (unsigned j = 0; j < iterations; ++j)
                status = dispatch(cfg, decl, n, args, result);
            auto elapsed = std::chrono::steady_clock::now() - begin;
            expr* want = empty ? nullptr : expected[i];
            VERIFY(status == (want ? BR_DONE : BR_FAILED) && result.get() == want);
            double ns = std::chrono::duration<double, std::nano>(elapsed).count() / iterations;
            std::cout << names[i] << ',' << name << ',' << iterations << ','
                      << std::fixed << std::setprecision(2) << ns << '\n';
        }
    }
    return measured;
}
}

int main(int argc, char** argv) {
    if (argc != 1 && argc != 4) {
        std::cerr << "usage: theory_reduction [case Core|Blank iterations]\n";
        return 1;
    }
    unsigned n = argc == 1 ? 1000000u : std::stoul(argv[3]);
    if (!n) return 1;
    memory::initialize(0);
    std::cout << "case,mode,iterations,ns_per_op\n";
    return run(argc == 1 ? "" : argv[1], argc == 1 ? "" : argv[2], n) ? 0 : 1;
}
