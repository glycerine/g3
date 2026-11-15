package opt

// Source: src/util/sorting_network.h psort_nw::mk_max/mk_min/merge.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// AUDIT.md opt_02-F3: mk_max/mk_min return a when a == b, so merge(1,1)
// over equal inputs outputs [a, a] and builds no or/and term; cmp() still
// emits its three cmp_ge clauses (fork fix #A gives sortmax GE mode).
func TestSortMaxComparatorEqualInputs(t *testing.T) {
	manager, fixture := maxSMTTestFixture(t)
	a := maxResTestBool(t, manager, "sortmax-cmp-a")
	config := smtcore.SMTSolverConfig{Params: fixture.Context, Globals: fixture.Globals, ModelEvaluator: fixture.Model}
	solver, err := NewArithmeticSolver(manager, fixture.Model.Params, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	engine := &sortMaxEngine{solver: solver}
	engine.owner.manager = manager
	defer func() {
		if e := engine.close(); e != nil {
			t.Error(e)
		}
	}()
	outputs, err := newSortMaxEncoder(engine).network([]ast.ASTHandle{a, a})
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 2 || outputs[0] != a || outputs[1] != a {
		t.Fatal("equal comparator inputs must pass through", outputs)
	}
	if engine.stats.Clauses != 3 || engine.stats.Comparators != 0 {
		t.Fatal("comparator clauses/vars", engine.stats.Clauses, engine.stats.Comparators)
	}
}
