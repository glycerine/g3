package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/sat"
	basetactic "github.com/glycerine/g3/pkg/tactic"
)

func TestNLQInitializeMapsCompilerVariablesAndArithmeticLevels(t *testing.T) {
	f := newFMFixture(t)
	solver := nlqBareSolverWithFixture(t, f)
	compiler, err := nlsattactic.NewGoalCompiler(f.r.manager, solver)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, compiler.Close()) })
	x := f.constant("nlq-init-x", f.real)
	y := f.constant("nlq-init-y", f.real)
	zero := f.own(f.r.manager.ArithmeticRealNumeralText("0"))
	difference := f.own(f.r.manager.MkArithmeticSub(y, x))
	atom := f.own(f.r.manager.MkArithmeticGE(difference, zero))
	goal, err := basetactic.NewGoal(f.r.manager, false, false, false)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, goal.Close()) })
	fmMust(t, goal.Assert(atom))
	fmMust(t, compiler.Compile(goal))
	boolean, err := f.r.manager.MkBoolSort()
	fmMust(t, err)
	truth, err := f.r.manager.MkFreshConst("nlq-is-true", boolean)
	fmMust(t, err)
	refs := make([]ast.ASTRef, 2)
	refs[0], err = f.r.manager.NewRef(x)
	fmMust(t, err)
	refs[1], err = f.r.manager.NewRef(y)
	fmMust(t, err)
	t.Cleanup(func() {
		for i := len(refs) - 1; i >= 0; i-- {
			fmMust(t, f.r.manager.DelRef(&refs[i]))
		}
	})
	search, err := newNLQSearchFromCompiler(solver, compiler, [][]ast.ASTRef{{refs[0]}, {refs[1]}, {}}, truth)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, search.close()) })
	xVariable, err := compiler.TermMapping().ToVar(x)
	fmMust(t, err)
	yVariable, err := compiler.TermMapping().ToVar(y)
	fmMust(t, err)
	if len(search.boundReal) != 3 || len(search.boundReal[0]) != 1 || search.boundReal[0][0] != xVariable || len(search.boundReal[1]) != 1 || search.boundReal[1][0] != yVariable {
		t.Fatal("quantifier block mapping", search.boundReal)
	}
	var arithmetic []sat.Literal
	fmMust(t, solver.ArithmeticLiterals(&arithmetic))
	if len(arithmetic) != 1 {
		t.Fatal("arithmetic atom registry", arithmetic)
	}
	want := MaxLevel{Existential: 0, Universal: 1}
	if got := search.state.booleanLevels[arithmetic[0].Variable()]; got != want {
		t.Fatal("arithmetic predicate level", got, want)
	}
}
