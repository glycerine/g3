// Linux link-time benchmark for the linked GoIvy Z3 library.
#include "ast/ast.h"
#include "api/z3.h"
#include "test/perf/thread_counter.h"
#include <cstring>

class asserted_formulas;
class cached_var_subst;
class macro_manager;
namespace smt { class enode; class qi_queue; }

namespace {
unsigned ordinal, calls;
unsigned long long instructions, empty_interval;
char const* selected;
bool active;

unsigned long long count() {
    unsigned long long data[3];
    if (read(event_fd, data, sizeof(data)) != sizeof(data) || data[1] != data[2]) std::abort();
    return data[0];
}

class interval {
    unsigned long long start = 0;
public:
    explicit interval(char const* name) {
        if (active && std::strcmp(name, selected) == 0) start = count();
    }
    ~interval() {
        if (start) {
            auto elapsed = count() - start;
            instructions += elapsed > empty_interval ? elapsed - empty_interval : 0;
            ++calls;
        }
    }
};
}

extern "C" void __real_Z3_solver_push(Z3_context, Z3_solver);
extern "C" void __wrap_Z3_solver_push(Z3_context ctx, Z3_solver solver) {
    ++ordinal;
    selected = std::getenv("IVY_PUSH_STAGE");
    if (!selected) std::abort();
    calls = 0;
    instructions = 0;
    open_counter();
    signal_counter(true);
    auto start = count();
    empty_interval = count() - start;
    active = true;
    __real_Z3_solver_push(ctx, solver);
    active = false;
    signal_counter(false);
    close(event_fd);
    std::fprintf(stderr, "IVY-PUSH-STAGE %u %s %u %llu\n", ordinal, selected, calls, instructions);
}

// GNU ld wraps references from other object files. The matching static
// library supplies the method bodies and the checker supplies the input.
#define SUBST _ZN16cached_var_substclEP10quantifierjPKPN3smt5enodeER7obj_refI4expr11ast_managerE
#define MACRO _ZN13macro_manager13expand_macrosEP4exprP3appPN18dependency_managerIN11ast_manager22expr_dependency_configEE10dependencyER7obj_refIS0_S5_ERSA_IS2_S5_ERSA_IS8_S5_E
#define REDUCE _ZN17asserted_formulas6reduceEv
#define QUEUE _ZN3smt8qi_queue11instantiateEv
#define JOIN_DIRECT(a, b) a ## b
#define JOIN(a, b) JOIN_DIRECT(a, b)
#define REAL(name) JOIN(__real_, name)
#define WRAP(name) JOIN(__wrap_, name)

extern "C" void REAL(SUBST)(cached_var_subst*, quantifier*, unsigned, smt::enode* const*, expr_ref&);
extern "C" void WRAP(SUBST)(cached_var_subst* self, quantifier* q, unsigned n, smt::enode* const* bindings, expr_ref& result) {
    interval sample("instantiate-body");
    REAL(SUBST)(self, q, n, bindings, result);
}

extern "C" void REAL(MACRO)(macro_manager*, expr*, proof*, expr_dependency*, expr_ref&, proof_ref&, expr_dependency_ref&);
extern "C" void WRAP(MACRO)(macro_manager* self, expr* e, proof* proof_value, expr_dependency* dep, expr_ref& result, proof_ref& new_proof, expr_dependency_ref& new_dep) {
    interval sample("expand");
    REAL(MACRO)(self, e, proof_value, dep, result, new_proof, new_dep);
}

extern "C" void REAL(REDUCE)(asserted_formulas*);
extern "C" void WRAP(REDUCE)(asserted_formulas* self) {
    interval sample("reduce-assertions");
    REAL(REDUCE)(self);
}

extern "C" void REAL(QUEUE)(smt::qi_queue*);
extern "C" void WRAP(QUEUE)(smt::qi_queue* self) {
    interval sample("instantiate-queue");
    REAL(QUEUE)(self);
}

// Used by the benchmark's copy of asserted_formulas::invoke.
extern "C" unsigned long long ivy_asserted_pass_start() {
    return active && std::strcmp(selected, "passes") == 0 ? count() : 0;
}
extern "C" void ivy_asserted_pass_end(unsigned long long start, char const* name, unsigned inputs) {
    if (!start) return;
    auto n = count() - start;
    std::fprintf(stderr, "IVY-PUSH-PASS %u %s %u %llu\n", ordinal, name, inputs,
                 n > empty_interval ? n - empty_interval : 0);
}
