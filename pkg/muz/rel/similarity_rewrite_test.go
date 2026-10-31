package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"testing"
)

func TestSimilarityRewriteRuleAndAuxiliaryRows(t *testing.T) {
	m := astTestManager(t)
	family, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	integer, e := m.MkFamilySort(family, 1, nil)
	astTestMust(t, e)
	boolean, e := m.MkBoolSort()
	astTestMust(t, e)
	decl := func(name string, arity int) ast.ASTHandle {
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = integer
		}
		h, e := m.MkFuncDecl(astTestSymbol(t, m, name), domain, boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	p, q, aux := decl("p", 3), decl("q", 3), decl("aux", 1)
	rm, e := base.NewRuleManager(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, rm.Close()) })
	for _, d := range []ast.ASTHandle{p, q, aux} {
		astTestMust(t, rm.RegisterPredicate(d))
	}
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, m, h); return h }
	x := own(m.MkVar(0, integer))
	one := own(m.ArithmeticIntegerNumeralText("1"))
	two := own(m.ArithmeticIntegerNumeralText("2"))
	stable := own(m.ArithmeticIntegerNumeralText("9"))
	rule := func(v ast.ASTHandle) *base.Rule {
		head := own(m.MkApp(p, []ast.ASTHandle{x, v, v}))
		tail := own(m.MkApp(q, []ast.ASTHandle{x, v, stable}))
		r, e := rm.NewRule(head, []ast.ASTHandle{tail}, nil, "original")
		astTestMust(t, e)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		return r
	}
	first, second := rule(one), rule(two)
	positions, e := similarityVaryingConstants([]*base.Rule{first, second})
	astTestMust(t, e)
	domains, rows, e := similarityOrphanRows([]*base.Rule{first, second}, positions)
	astTestMust(t, e)
	if len(domains) != 1 || domains[0] != integer || len(rows) != 2 || len(rows[0]) != 1 || rows[0][0] != one || rows[1][0] != two {
		t.Fatal("auxiliary rows", domains, rows)
	}
	rewritten, e := similarityRewriteRule(rm, first, positions, aux)
	astTestMust(t, e)
	defer func() { astTestMust(t, rewritten.Close()) }()
	name, e := rewritten.Name()
	astTestMust(t, e)
	if name != "original" {
		t.Fatal("rule name", name)
	}
	head, e := rewritten.Head()
	astTestMust(t, e)
	a, e := m.GetArg(head, 1)
	astTestMust(t, e)
	b, e := m.GetArg(head, 2)
	astTestMust(t, e)
	if a != b {
		t.Fatal("equal columns did not share variable")
	}
	index, e := m.GetVarIndex(a)
	astTestMust(t, e)
	if index != 1 {
		t.Fatal("fresh variable index", index)
	}
	n, e := rewritten.TailSize()
	astTestMust(t, e)
	if n != 2 {
		t.Fatal("tail count", n)
	}
	tail, neg, e := rewritten.Tail(0)
	astTestMust(t, e)
	if neg {
		t.Fatal("tail polarity")
	}
	tailArg, e := m.GetArg(tail, 1)
	astTestMust(t, e)
	fixed, e := m.GetArg(tail, 2)
	astTestMust(t, e)
	if tailArg != a || fixed != stable {
		t.Fatal("rewritten tail")
	}
	auxTail, neg, e := rewritten.Tail(1)
	astTestMust(t, e)
	if neg {
		t.Fatal("auxiliary polarity")
	}
	d, e := m.GetDecl(auxTail)
	astTestMust(t, e)
	arg, e := m.GetArg(auxTail, 0)
	astTestMust(t, e)
	if d != aux || arg != a {
		t.Fatal("auxiliary predicate")
	}
}
