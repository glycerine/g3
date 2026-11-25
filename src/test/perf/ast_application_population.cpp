// Companion to BenchmarkASTApplicationPopulation. Variables, declarations and
// root pointer arrays already exist before the live application allocations.
#include "ast/ast.h"
#include <chrono>
#include <cstdlib>
#include <iostream>
#include <limits>
#include <memory>
#include <vector>
#ifdef __GLIBC__
#include <malloc.h>
#endif

namespace {
constexpr unsigned terms = 2048;
constexpr unsigned workers = 4;

struct population {
    ast_manager manager;
    expr_ref_vector variables;
    func_decl_ref declaration;
    std::vector<app*> roots;
    expr* args[8]{};

    explicit population(unsigned arity):
        variables(manager), declaration(manager), roots(terms) {
        sort* boolean = manager.mk_bool_sort();
        sort* domain[8];
        for (unsigned j = 0; j < arity; ++j) domain[j] = boolean;
        declaration = manager.mk_func_decl(symbol("population"), arity, domain, boolean);
        for (unsigned j = 0; j < terms; ++j) variables.push_back(manager.mk_var(j, boolean));
        for (unsigned j = 0; j < arity; ++j) args[j] = variables.get(j);
    }
    ~population() {
        for (auto* p : roots) if (p) manager.dec_ref(p);
    }
};

double allocated() {
#ifdef __GLIBC__
#if __GLIBC_PREREQ(2, 33)
    auto info = mallinfo2();
    return double(info.uordblks) + double(info.hblkhd);
#endif
#endif
    return std::numeric_limits<double>::quiet_NaN();
}
}

int main(int argc, char** argv) {
    if (argc != 2) return 1;
    unsigned arity = static_cast<unsigned>(std::strtoul(argv[1], nullptr, 10));
    if (arity != 1 && arity != 2 && arity != 4 && arity != 8) return 1;
    memory::initialize(0);
    std::vector<std::unique_ptr<population>> all;
    size_t request_before = 0;
    for (unsigned i = 0; i < workers; ++i) {
        all.emplace_back(new population(arity));
        request_before += all.back()->manager.get_allocation_size();
    }
    double base = allocated();
    auto start = std::chrono::steady_clock::now();
    for (auto& w : all) {
        for (unsigned j = 0; j < terms; ++j) {
            w->args[0] = w->variables.get(j);
            w->roots[j] = w->manager.mk_app(w->declaration, arity, w->args);
            w->manager.inc_ref(w->roots[j]);
        }
    }
    auto stop = std::chrono::steady_clock::now();
    double peak = allocated();
    size_t requests = 0;
    for (auto& w : all) {
        requests += w->manager.get_allocation_size();
        for (unsigned j = 0; j < terms; ++j) {
            w->args[0] = w->variables.get(j);
            if (w->manager.mk_app(w->declaration, arity, w->args) != w->roots[j]) return 2;
        }
    }
    double count = double(workers * terms);
    std::cout << "arity,workers,apps_per_worker,ast_header,app_header,small_requested_per_app,malloc_per_app,ns_per_app\n"
              << arity << ',' << workers << ',' << terms << ',' << sizeof(ast) << ',' << sizeof(app) << ','
              << double(requests-request_before)/count << ',' << (peak-base)/count << ','
              << std::chrono::duration<double,std::nano>(stop-start).count()/count << '\n';
}
