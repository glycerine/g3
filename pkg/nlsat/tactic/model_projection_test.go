package tactic

import (
	"testing"

	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestModelProjectorEliminatesRealVariableAndBuildsBlockingClause(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	zero, err := fixture.manager.ArithmeticRealNumeralText("0")
	compilerMust(t, err)
	sort, err := fixture.manager.GetSort(zero)
	compilerMust(t, err)
	x := compilerConst(t, fixture.manager, "project-x", sort)
	y := compilerConst(t, fixture.manager, "project-y", sort)
	difference, err := fixture.manager.MkArithmeticSub(y, x)
	compilerMust(t, err)
	dependent, err := fixture.manager.MkArithmeticGE(difference, zero)
	compilerMust(t, err)
	independent, err := fixture.manager.MkArithmeticGE(y, zero)
	compilerMust(t, err)
	compilerMust(t, fixture.compiler.Compile(compilerGoal(t, fixture.manager, dependent)))
	compilerMust(t, fixture.compiler.Compile(compilerGoal(t, fixture.manager, independent)))
	dependentLiteral, err := fixture.compiler.Literal(dependent)
	compilerMust(t, err)
	independentLiteral, err := fixture.compiler.Literal(independent)
	compilerMust(t, err)
	status, err := fixture.solver.Check()
	compilerMust(t, err)
	if status != lbool.True {
		t.Fatal("projection fixture is not satisfiable", status)
	}
	xVariable, err := fixture.compiler.TermMapping().ToVar(x)
	compilerMust(t, err)
	projector, err := NewModelProjector(fixture.manager, fixture.solver, fixture.compiler.AtomMapping(), fixture.compiler.TermMapping())
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, projector.Close()) })
	clause, err := projector.ProjectClause([]uint32{xVariable}, []sat.Literal{dependentLiteral, independentLiteral})
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, fixture.manager.DelRef(&clause)) })
	handle, err := fixture.manager.RefGet(clause)
	compilerMust(t, err)
	want, err := fixture.manager.MkArithmeticLT(y, zero)
	compilerMust(t, err)
	if handle != want {
		gotText, _ := fixture.manager.SMT2PPExprTree(handle, nil)
		wantText, _ := fixture.manager.SMT2PPExprTree(want, nil)
		t.Fatal("projected blocking clause", gotText, wantText)
	}
}
