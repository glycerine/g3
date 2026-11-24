// Test-only observer for smt::qi_queue::instantiate in a single-threaded GoIvy run.
#pragma once

#include "ast/ast.h"
#include <cstdio>
#include <fstream>
#include <map>
#include <sstream>
#include <string>

namespace ivy_quantifier_candidates {
inline void write_term(std::ostream& out, expr* value) {
    if (is_var(value)) {
        out << '$' << to_var(value)->get_idx();
        return;
    }
    if (is_app(value)) {
        app* term = to_app(value);
        unsigned size = term->get_num_args();
        if (size) out << '(';
        out << term->get_decl()->get_name();
        for (unsigned i = 0; i < size; ++i) {
            out << ' ';
            write_term(out, term->get_arg(i));
        }
        if (size) out << ')';
        return;
    }
    quantifier* q = to_quantifier(value);
    out << (q->is_forall() ? "(forall" : "(exists");
    for (unsigned i = 0; i < q->get_num_decls(); ++i)
        out << ' ' << q->get_decl_sort(i)->get_name();
    out << ' ';
    write_term(out, q->get_expr());
    for (unsigned i = 0; i < q->get_num_patterns(); ++i) {
        out << ' ';
        write_term(out, q->get_pattern(i));
    }
    out << ')';
}

inline void observe(quantifier* q, const char* stage, unsigned generation) {
    std::ifstream marker("watch-quantifiers");
    if (!marker) return;
    std::string goal;
    std::getline(marker, goal);
    static std::string last_goal;
    static std::map<unsigned, std::string> names;
    if (goal != last_goal) {
        names.clear();
        last_goal = goal;
    }
    unsigned id = q->get_id();
    // A later quantifier can reuse a deleted AST's numerical ID.
    if (std::string(stage) == "candidate") {
        std::ostringstream out;
        write_term(out, q);
        std::string name = out.str();
        if (names[id] != name) {
            names[id] = name;
            std::fprintf(stderr, "QUANT-NAME %s %u %s\n", goal.c_str(), id, name.c_str());
        }
    }
    std::fprintf(stderr, "QUANT-COUNT %s %u %s %u\n", goal.c_str(), id, stage, generation);
}
}
