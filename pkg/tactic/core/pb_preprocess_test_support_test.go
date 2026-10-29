package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type pbPreprocessFixture struct {
	t      *testing.T
	m      *ast.ASTManager
	runner *PBPreprocessTactic
	vars   []ast.ASTHandle
}

func pbPreprocessSymbol(t *testing.T, m *ast.ASTManager, text string) usymbol.Symbol {
	t.Helper()
	s, e := m.Symbols.Intern(text)
	simplifyMust(t, e)
	return s
}

func newPBPreprocessFixture(t *testing.T) pbPreprocessFixture {
	t.Helper()
	m, _, _, services := solveEqFixture(t)
	simplifyMust(t, m.RegisterDeclarationPlugins())
	r, e := NewPBPreprocessTactic(m, services)
	simplifyMust(t, e)
	t.Cleanup(func() { simplifyMust(t, r.Close()) })
	b, e := m.MkBoolSort()
	simplifyMust(t, e)
	var values []ast.ASTHandle
	for _, name := range []string{"pb-x", "pb-y", "pb-z"} {
		h, x := m.MkConst(pbPreprocessSymbol(t, m, name), b)
		simplifyMust(t, x)
		values = append(values, h)
	}
	return pbPreprocessFixture{t: t, m: m, runner: r, vars: values}
}

func (f pbPreprocessFixture) ge(args []ast.ASTHandle, coeff []string, bound string) ast.ASTHandle {
	values := make([]unumeral.OwnedRational, len(coeff))
	for i := range coeff {
		v, e := f.m.NumeralService().NewValue()
		simplifyMust(f.t, e)
		values[i] = v
		simplifyMust(f.t, f.m.NumeralService().SetString(&values[i], coeff[i]))
	}
	k, e := f.m.NumeralService().NewValue()
	simplifyMust(f.t, e)
	simplifyMust(f.t, f.m.NumeralService().SetString(&k, bound))
	h, e := f.m.MkPseudoBooleanGE(values, args, &k)
	simplifyMust(f.t, e)
	for i := range values {
		simplifyMust(f.t, f.m.NumeralService().Del(&values[i]))
	}
	simplifyMust(f.t, f.m.NumeralService().Del(&k))
	return h
}
func (f pbPreprocessFixture) goal(forms ...ast.ASTHandle) *tactic.Goal {
	g, e := tactic.NewGoal(f.m, false, true, false)
	simplifyMust(f.t, e)
	f.t.Cleanup(func() { simplifyMust(f.t, g.Close()) })
	for _, h := range forms {
		simplifyMust(f.t, g.Assert(h))
	}
	return g
}
func (f pbPreprocessFixture) apply(g *tactic.Goal) *tactic.TacticResult {
	out := &tactic.TacticResult{}
	f.t.Cleanup(func() { simplifyMust(f.t, out.Close()) })
	simplifyMust(f.t, f.runner.Apply(g, out))
	return out
}
