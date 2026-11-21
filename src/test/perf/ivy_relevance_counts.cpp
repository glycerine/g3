// Link-time observer for the instrumented Apple relevance benchmark.
#include "z3.h"

void report_ivy_relevance_counts();
extern "C" Z3_lbool __real_Z3_solver_check(Z3_context, Z3_solver);

extern "C" Z3_lbool __wrap_Z3_solver_check(Z3_context ctx, Z3_solver solver) {
    Z3_lbool answer = __real_Z3_solver_check(ctx, solver);
    report_ivy_relevance_counts();
    return answer;
}
