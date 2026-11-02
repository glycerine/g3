package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalRelationASTFactMembership(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	astTestPin(t, m, real)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle {
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	one, two, three := own(m.ArithmeticRealNumeralText("1")), own(m.ArithmeticRealNumeralText("2")), own(m.ArithmeticRealNumeralText("3"))
	x, y := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	r := NewIntervalFull(2)
	astTestMust(t, r.FilterEqualExpression(m, 0, two))
	astTestMust(t, r.FilterIdenticalColumns([]uint32{0, 1}))
	for _, tc := range []struct {
		name   string
		values []ast.ASTHandle
		want   bool
	}{
		{name: "exact", values: []ast.ASTHandle{two, two}, want: true},
		{name: "outside", values: []ast.ASTHandle{one, one}},
		{name: "different-equal-class-handles", values: []ast.ASTHandle{x, y}},
		{name: "same-symbolic-handle", values: []ast.ASTHandle{x, x}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.ContainsFactExpressions(m, tc.values)
			astTestMust(t, err)
			if got != tc.want {
				t.Fatalf("membership=%v, want %v", got, tc.want)
			}
		})
	}
	if got, err := NewIntervalEmpty(1).ContainsFactExpressions(m, []ast.ASTHandle{three}); err != nil || !got {
		t.Fatal("source empty relation bounds should remain conservatively visible", got, err)
	}
	if _, err = r.ContainsFactExpressions(m, []ast.ASTHandle{two}); err == nil {
		t.Fatal("wrong arity accepted")
	}
	if _, err = r.ContainsFactExpressions(nil, []ast.ASTHandle{two, two}); err == nil {
		t.Fatal("nil manager accepted")
	}
	if _, err = (*IntervalRelation)(nil).ContainsFactExpressions(m, nil); err == nil {
		t.Fatal("nil relation accepted")
	}
}

func TestIntervalRelationASTFactInsertion(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	astTestPin(t, m, real)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle {
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	one, three := own(m.ArithmeticRealNumeralText("1")), own(m.ArithmeticRealNumeralText("3"))
	r := NewIntervalEmpty(1)
	astTestMust(t, r.AddFactExpressions(m, []ast.ASTHandle{real}, []ast.ASTHandle{one}))
	astTestMust(t, r.AddFactExpressions(m, []ast.ASTHandle{real}, []ast.ASTHandle{three}))
	for _, value := range []ast.ASTHandle{one, three} {
		found, err := r.ContainsFactExpressions(m, []ast.ASTHandle{value})
		astTestMust(t, err)
		if !found {
			t.Fatal("numeric fact missing")
		}
	}

	name, err := m.Symbols.Intern("interval-symbolic-fact")
	astTestMust(t, err)
	declaration, err := m.MkFuncDecl(name, nil, real, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, declaration)
	symbolic := own(m.MkApp(declaration, nil))
	symbols := NewIntervalEmpty(1)
	astTestMust(t, symbols.AddFactExpressions(m, []ast.ASTHandle{real}, []ast.ASTHandle{symbolic}))
	if !symbols.IsFull() {
		t.Fatal("unrecognized symbolic source fact should produce the conservative full interval")
	}

	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	truth := own(m.MkTrue())
	if err = r.AddFactExpressions(m, []ast.ASTHandle{real}, []ast.ASTHandle{truth}); err == nil {
		t.Fatal("wrong-sort fact accepted")
	}
	if err = r.AddFactExpressions(m, []ast.ASTHandle{boolean}, []ast.ASTHandle{truth}); err == nil {
		t.Fatal("non-arithmetic signature accepted")
	}
	if err = r.AddFactExpressions(m, nil, nil); err == nil {
		t.Fatal("wrong-arity fact accepted")
	}
}
