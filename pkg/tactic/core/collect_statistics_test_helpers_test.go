package core

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type collectStatisticsFixture struct {
	m             *ast.ASTManager
	sort, a, b, f ast.ASTHandle
	output        *bytes.Buffer
}

func newCollectStatisticsFixture(t *testing.T) collectStatisticsFixture {
	t.Helper()
	m, _, _ := simplifyFixture(t)
	name, err := m.Symbols.Intern("StatsSort")
	simplifyMust(t, err)
	sort, err := m.MkUninterpretedSort(name, nil)
	simplifyMust(t, err)
	simplifyPin(t, m, sort)
	constant := func(text string) ast.ASTHandle {
		s, e := m.Symbols.Intern(text)
		simplifyMust(t, e)
		h, e := m.MkConst(s, sort)
		simplifyMust(t, e)
		simplifyPin(t, m, h)
		return h
	}
	fn, err := m.Symbols.Intern("stats-f")
	simplifyMust(t, err)
	f, err := m.MkFuncDecl(fn, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	simplifyPin(t, m, f)
	return collectStatisticsFixture{m: m, sort: sort, a: constant("stats-a"), b: constant("stats-b"), f: f, output: &bytes.Buffer{}}
}

func (f collectStatisticsFixture) app(t *testing.T, d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, e := f.m.MkApp(d, args)
	simplifyMust(t, e)
	return h
}
func (f collectStatisticsFixture) goal(t *testing.T, forms ...ast.ASTHandle) *tactic.Goal {
	t.Helper()
	g, e := tactic.NewGoal(f.m, false, false, false)
	simplifyMust(t, e)
	t.Cleanup(func() { simplifyMust(t, g.Close()) })
	for _, h := range forms {
		simplifyMust(t, g.Assert(h))
	}
	return g
}
func (f collectStatisticsFixture) runner(t *testing.T) *CollectStatisticsTactic {
	t.Helper()
	r, e := NewCollectStatisticsTacticWithOutput(f.m, nil, f.output)
	simplifyMust(t, e)
	t.Cleanup(func() { simplifyMust(t, r.Close()) })
	return r
}
func applyStatistics(t *testing.T, r *CollectStatisticsTactic, g *tactic.Goal) *tactic.TacticResult {
	t.Helper()
	out := &tactic.TacticResult{}
	t.Cleanup(func() { simplifyMust(t, out.Close()) })
	simplifyMust(t, r.Apply(g, out))
	return out
}
