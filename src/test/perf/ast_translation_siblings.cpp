// Companion to BenchmarkASTTranslationSiblings in pkg/ast.
#include "ast/ast.h"
#include "ast/ast_translation.h"
#include "util/memory_manager.h"
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <string>

static void benchmark(unsigned width, bool reuse, unsigned iterations) {
    ast_manager from, to;
    sort* boolean = from.mk_bool_sort();
    func_decl_ref binary(from.mk_func_decl(symbol("binary"), boolean, boolean, boolean), from);
    expr_ref_vector roots(from);
    for (unsigned j = 0; j < 4; ++j) {
        expr_ref_vector level(from);
        for (unsigned i = 0; i < width; ++i) {
            auto a = "c" + std::to_string(i);
            auto b = "d" + std::to_string(j + i % 11);
            auto x = from.mk_const(symbol(a.c_str()), boolean);
            auto y = from.mk_const(symbol(b.c_str()), boolean);
            level.push_back(from.mk_app(binary, x, y));
        }
        while (level.size() > 1) {
            expr_ref_vector next(from);
            for (unsigned i = 0; i < level.size(); i += 2)
                next.push_back(from.mk_and(level.get(i), level.get(i + 1)));
            level.swap(next);
        }
        roots.push_back(level.get(0));
    }
    ast_translation translate(from, to, false);
    expr_ref result(to);
    auto start = std::chrono::steady_clock::now();
    for (unsigned i = 0; i < iterations; ++i) {
        if (!reuse || i % roots.size() == 0)
            translate.reset_cache();
        result = translate(roots.get(i % roots.size()));
    }
    auto stop = std::chrono::steady_clock::now();
    if (!result || !to.is_and(result) || to_app(result)->get_num_args() != 2)
        std::abort();
    double ns = std::chrono::duration<double, std::nano>(stop - start).count();
    std::printf("BenchmarkASTTranslationSiblings/%u/cache-%s %u %.2f ns/op\n",
                width, reuse ? "true" : "false", iterations, ns / iterations);
}

int main(int argc, char** argv) {
    unsigned iterations = argc == 2 ? static_cast<unsigned>(std::strtoul(argv[1], nullptr, 10)) : 24;
    if (!iterations || iterations > 1000000 || argc > 2)
        return 2;
    memory::initialize(0);
    for (unsigned width : {8u, 128u, 1024u})
        for (bool reuse : {false, true})
            benchmark(width, reuse, iterations);
    memory::finalize();
}
