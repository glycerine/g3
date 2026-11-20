package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestUFBVUsedVarsExpressionDepthAndQuantifierAnnotations(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	satOK(t, e)
	vars := make([]ast.ASTHandle, 3)
	for i := range vars {
		vars[i], e = m.MkVar(uint32(i), b)
		satOK(t, e)
		astTestPin(t, m, vars[i])
	}
	q := func(body ast.ASTHandle, noPatterns []ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		h, e := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{b},
			Names: []symbol.Symbol{symbol.Numerical(1)},
			Body:  body, NoPatterns: noPatterns,
		})
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	inner := q(vars[1], nil)
	root, e := m.MkAnd(vars[1], inner)
	satOK(t, e)
	astTestPin(t, m, root)
	u, e := newUFBVUsedVars(m)
	satOK(t, e)
	satOK(t, u.process(root))
	if !u.usesAll(2) || u.count() != 2 || u.found[0] != b || u.found[1] != b {
		t.Fatal("same variable at depth zero and one must occupy different free indices", u.found)
	}
	u.reset()
	if u.usesAny(2) || u.count() != 0 {
		t.Fatal("collector reset retained variable sorts")
	}
	d, e := m.MkFuncDecl(symbol.Numerical(2), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	satOK(t, e)
	astTestPin(t, m, d)
	pattern, e := m.MkApp(d, []ast.ASTHandle{vars[1]})
	satOK(t, e)
	astTestPin(t, m, pattern)
	onlyAnnotation := q(vars[0], []ast.ASTHandle{pattern})
	satOK(t, u.process(onlyAnnotation))
	if !u.usesAny(1) || u.count() != 1 || !u.contains(0) || u.contains(1) {
		t.Fatal("bound body var must be ignored while no-pattern free var is retained", u.found)
	}
	satOK(t, u.process(vars[2]))
	if u.count() != 2 || !u.contains(2) || u.usesAll(3) {
		t.Fatal("process must accumulate across calls without filling absent indices", u.found)
	}
}

func BenchmarkUsedVarsUnderQuantifiers(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	f, err := m.MkFuncDecl(symbol.Numerical(11), []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(b, err)
	app := func(x, y ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(f, []ast.ASTHandle{x, y})
		astTestMust(b, err)
		return h
	}
	v, err := m.MkVar(2, sort)
	astTestMust(b, err)
	ground, err := m.MkTrue()
	astTestMust(b, err)
	for i := 0; i < 8; i++ {
		ground = app(ground, ground)
	}
	shared := app(v, ground)
	bind := func(body ast.ASTHandle) ast.ASTHandle {
		q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{sort},
			Names: []symbol.Symbol{symbol.Numerical(12)}, Body: body,
		})
		astTestMust(b, err)
		return q
	}
	// The same shared application occurs at binder depths zero, one and two.
	root := app(shared, bind(app(shared, bind(shared))))
	astTestPin(b, m, root)
	u, err := newUFBVUsedVars(m)
	astTestMust(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.reset()
		if err := u.process(root); err != nil {
			b.Fatal(err)
		}
		if !u.usesAll(3) || len(u.found) != 3 {
			b.Fatal(u.found)
		}
	}
}
