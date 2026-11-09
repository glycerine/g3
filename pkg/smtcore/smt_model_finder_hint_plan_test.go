package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestModelFinderHintPlanPartitionAndRunReset(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	pin := func(h ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		astTestMust(t, m.IncRef(h))
		t.Cleanup(func() { astTestMust(t, m.DecRef(h)) })
		return h
	}
	v, e := m.MkVar(0, b)
	astTestMust(t, e)
	pin(v)
	q1, e := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{b},
		Names: []symbol.Symbol{symbol.Numerical(1)}, Body: v,
	})
	astTestMust(t, e)
	pin(q1)
	q2, e := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{b},
		Names: []symbol.Symbol{symbol.Numerical(2)}, Body: v,
	})
	astTestMust(t, e)
	pin(q2)
	f, e := m.MkFuncDecl(symbol.Numerical(3), nil, b, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	pin(f)
	fApp, e := m.MkApp(f, nil)
	astTestMust(t, e)
	pin(fApp)

	s := newHintSearchState(m)
	s.infos[q1] = &hintQuantifierInfo{
		nonGround: []ast.ASTHandle{f},
		macros:    []hintConditionalMacro{{function: f, definition: fApp, satisfyAtom: true}},
	}
	s.infos[q2] = &hintQuantifierInfo{}
	qs := []ast.ASTHandle{q2, q1}

	failed, found, e := s.planSubset(m, qs)
	astTestMust(t, e)
	if found || !equalHintHandles(failed.remaining, qs) || len(failed.definitions) != 0 {
		t.Fatal("cyclic candidate must remain after rejected quantifier", failed, found)
	}
	if len(s.qByDefinition) != 1 || len(s.defsByFunction) != 1 {
		t.Fatal("candidate indexes were not built")
	}
	if !s.residue.empty() || !s.satisfied.empty() || s.definitions.Len() != 0 {
		t.Fatal("failed search did not roll back")
	}

	s.infos[q1].macros[0].definition = v
	winning, found, e := s.planSubset(m, qs)
	astTestMust(t, e)
	if !found || !equalHintHandles(winning.remaining, []ast.ASTHandle{q2}) || len(winning.definitions) != 1 || winning.definitions[0] != (hintFunctionDefinition{f, v}) {
		t.Fatal("successful hint partition or selected definition", winning, found)
	}
	if s.qByDefinition[hintFunctionDefinition{f, fApp}].size() != 0 || len(s.qByDefinition) != 1 {
		t.Fatal("old object-pair index survived run reset")
	}
	if s.infos[q1].theOne != f || !s.satisfied.contains(q1) {
		t.Fatal("winning branch was not retained")
	}
	winning.definitions = nil
	if got, _, _ := s.definitions.Find(f); got[0] != v {
		t.Fatal("plan must snapshot the selected interpretations")
	}

	empty, found, e := s.planSubset(m, []ast.ASTHandle{q2})
	astTestMust(t, e)
	if found || !equalHintHandles(empty.remaining, []ast.ASTHandle{q2}) || len(empty.definitions) != 0 || len(s.qByDefinition) != 0 || s.candidates.size() != 0 || !s.satisfied.empty() || len(s.satisfied.scopes) != 0 {
		t.Fatal("empty candidate run did not reset previous search", empty, found)
	}
}
