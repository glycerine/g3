package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func arrayMust(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}

type arrayFixture struct {
	m                   *ast.ASTManager
	tactic              *BVArray2UFTactic
	index, value, array ast.ASTHandle
	a, b, i, j, v       ast.ASTHandle
}

func newArrayFixture(t *testing.T) arrayFixture {
	return newArrayFixtureProof(t, false)
}

func newArrayFixtureProof(t *testing.T, proofs bool) arrayFixture {
	t.Helper()
	base := sharingTacticFixture(t, proofs)
	m := base.manager
	arrayMust(t, m.RegisterDeclarationPlugins())
	cooperation, e := cooperate.NewCooperation(m.Budget())
	arrayMust(t, e)
	t.Cleanup(func() { arrayMust(t, cooperation.Close()) })
	services := model.ModelEvaluatorConfig{
		Context: base.context, Globals: uparams.BorrowProcessGlobalParamsService(),
		Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation,
	}
	runner, e := NewBVArray2UFTactic(m, nil, services)
	arrayMust(t, e)
	t.Cleanup(func() {
		arrayMust(t, runner.Close())
	})
	index, e := m.BitVectorSort(3)
	arrayMust(t, e)
	value, e := m.BitVectorSort(5)
	arrayMust(t, e)
	array, e := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{index}), value)
	arrayMust(t, e)
	mk := func(name string, sort ast.ASTHandle) ast.ASTHandle {
		s, e := m.Symbols.Intern(name)
		arrayMust(t, e)
		h, e := m.MkConst(s, sort)
		arrayMust(t, e)
		ref, e := m.NewRef(h)
		arrayMust(t, e)
		t.Cleanup(func() { arrayMust(t, m.DelRef(&ref)) })
		return h
	}
	return arrayFixture{m: m, tactic: runner, index: index, value: value, array: array, a: mk("array-a", array), b: mk("array-b", array), i: mk("array-i", index), j: mk("array-j", index), v: mk("array-v", value)}
}
func (f arrayFixture) app(t *testing.T, op int32, args ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, e := f.m.MkArrayAppArguments(f.tactic.arrayFamily, op, ast.SliceArguments(args))
	arrayMust(t, e)
	return h
}
func (f arrayFixture) goal(t *testing.T, proofs, models, cores bool, forms ...ast.ASTHandle) *tactic.Goal {
	t.Helper()
	g, e := tactic.NewGoal(f.m, proofs, models, cores)
	arrayMust(t, e)
	t.Cleanup(func() { arrayMust(t, g.Close()) })
	for _, h := range forms {
		arrayMust(t, g.Assert(h))
	}
	return g
}
