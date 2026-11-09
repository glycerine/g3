package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func modelFinderQuantifiers(t *testing.T) (*ast.ASTManager, []ast.ASTHandle) {
	t.Helper()
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	result := make([]ast.ASTHandle, 3)
	for i := range result {
		body, e := m.MkVar(uint32(i), b)
		astTestMust(t, e)
		q, e := m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{b},
			Names: []symbol.Symbol{symbol.Numerical(uint32(i))}, Body: body,
		})
		astTestMust(t, e)
		astTestMust(t, m.IncRef(q))
		result[i] = q
	}
	t.Cleanup(func() {
		for _, q := range result {
			astTestMust(t, m.DecRef(q))
		}
	})
	return m, result
}

func TestModelFinderScopedQuantifierSetTrailsAndEvents(t *testing.T) {
	m, q := modelFinderQuantifiers(t)
	type event struct {
		q       ast.ASTHandle
		insert  bool
		present bool
	}
	var events []event
	var satisfied *scopedQuantifierSet
	satisfied = newScopedQuantifierSet(m, func(v ast.ASTHandle, ins bool) {
		events = append(events, event{q: v, insert: ins, present: satisfied.contains(v)})
	})
	satisfied.insert(q[0])
	satisfied.insert(q[1])
	satisfied.pushScope()
	satisfied.erase(q[0])
	satisfied.insert(q[2])
	satisfied.insert(q[2]) // duplicate adds no trail
	satisfied.pushScope()
	satisfied.erase(q[1])
	satisfied.insert(q[0])
	astTestMust(t, satisfied.popScope())
	if satisfied.contains(q[0]) || !satisfied.contains(q[1]) || !satisfied.contains(q[2]) {
		t.Fatal("inner rollback membership")
	}
	astTestMust(t, satisfied.popScope())
	if !satisfied.contains(q[0]) || !satisfied.contains(q[1]) || satisfied.contains(q[2]) {
		t.Fatal("outer rollback membership")
	}
	want := []event{
		{q[0], true, true}, {q[1], false, false},
		{q[2], true, true}, {q[0], false, false},
	}
	if len(events) != len(want) {
		t.Fatalf("undo event count %d, want %d", len(events), len(want))
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("undo event %d = %+v, want %+v", i, events[i], want[i])
		}
	}
	if got := satisfied.snapshot(); len(got) != 2 {
		t.Fatal("borrowed quantifier snapshot", got)
	}
	satisfied.reset()
	if !satisfied.empty() || len(events) != 4 {
		t.Fatal("reset must clear without undo events")
	}
	if e := satisfied.popScope(); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal("empty scope guard", e)
	}
}

func TestModelFinderScopedQuantifierSetLevelZeroEraseSourceBranch(t *testing.T) {
	m, q := modelFinderQuantifiers(t)
	residue := newScopedQuantifierSet(m, nil)
	residue.erase(q[0]) // source level-zero branch calls insert
	if !residue.contains(q[0]) {
		t.Fatal("source level-zero erase branch")
	}
	residue.pushScope()
	residue.erase(q[0])
	if !residue.empty() {
		t.Fatal("scoped erase did not remove")
	}
	astTestMust(t, residue.popScope())
	if !residue.contains(q[0]) {
		t.Fatal("scoped erase not restored")
	}
}
