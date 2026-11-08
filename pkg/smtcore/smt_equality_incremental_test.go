package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestSMTIncrementalCongruenceAndDisequalityNestedRollback(t *testing.T) {
	m := astTestManager(t)
	u, err := m.MkUninterpretedSort(astTestSymbol(t, m, "incremental-U"), nil)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	s, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	var values []ast.ASTHandle
	var nodes []*SMTEnode
	for _, name := range []string{"a", "b", "c", "d"} {
		h, err := m.MkConst(astTestSymbol(t, m, name), u)
		astTestMust(t, err)
		n, err := s.MkEnode(h, false, false, true)
		astTestMust(t, err)
		values = append(values, h)
		nodes = append(nodes, n)
	}
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	f, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "f"), []ast.ASTHandle{u}, u, info)
	astTestMust(t, err)
	h, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "h"), []ast.ASTHandle{u, u, u}, u, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	var unary, nary []*SMTEnode
	for i := 0; i < 2; i++ {
		app, err := m.MkApp(f, []ast.ASTHandle{values[i]})
		astTestMust(t, err)
		n, err := s.MkEnode(app, false, false, true)
		astTestMust(t, err)
		unary = append(unary, n)
		app, err = m.MkApp(h, []ast.ASTHandle{values[i], values[2], values[3]})
		astTestMust(t, err)
		n, err = s.MkEnode(app, false, false, true)
		astTestMust(t, err)
		nary = append(nary, n)
	}
	v, err := s.MkBoolVar(smtRelevancyBool(t, m, "diseq-reason"))
	astTestMust(t, err)
	astTestMust(t, s.addDisequality(nodes[0], nodes[3], NewSMTLiteral(v, false)))
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(nodes[0], nodes[1], NullSMTEqualityJustification()))
	astTestMust(t, s.propagateCoreEqualities())
	if unary[0].root != unary[1].root || nary[0].root != nary[1].root {
		t.Fatal("affected unary/n-ary parents were not reinserted")
	}
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(nodes[1], nodes[2], NullSMTEqualityJustification()))
	if !context.isDisequal(nodes[2], nodes[3]) {
		t.Fatal("disequality did not follow the merged root")
	}
	// The reason is a bare Boolean variable, not an equality atom, so as in
	// add_eq the merge itself raises no conflict.
	astTestMust(t, s.addEquality(nodes[2], nodes[3], NullSMTEqualityJustification()))
	astTestMust(t, s.PopScopes(1))
	if s.inconsistent || nodes[0].root != nodes[1].root || nodes[2].root != nodes[2] || !context.isDisequal(nodes[0], nodes[3]) {
		t.Fatal("nested equality/disequality rollback")
	}
	astTestMust(t, s.PopScopes(1))
	if unary[0].root == unary[1].root || nary[0].root == nary[1].root {
		t.Fatal("congruence classes survived rollback")
	}
	for _, n := range append(unary, nary...) {
		found, _, err := s.congruence.Find(n)
		astTestMust(t, err)
		if found != n {
			t.Fatal("restored signature lookup", found, n)
		}
	}
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(nodes[1], nodes[3], NullSMTEqualityJustification()))
	astTestMust(t, s.addEquality(nodes[0], nodes[2], NullSMTEqualityJustification()))
	if !context.isDisequal(nodes[1], nodes[2]) {
		t.Fatal("both disequality endpoints must follow their roots")
	}
	astTestMust(t, s.addEquality(nodes[0], nodes[1], NullSMTEqualityJustification()))
	astTestMust(t, s.PopScopes(1))
	if s.inconsistent || !context.isDisequal(nodes[0], nodes[3]) {
		t.Fatal("original disequality not restored")
	}
}
