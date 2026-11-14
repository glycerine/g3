package ast

import (
	"testing"
)

func TestASTRefNativeAssignmentUnderRefusedArena(t *testing.T) {
	for _, viaRef := range []bool{false, true} {
		name := "pointer"
		if viaRef {
			name = "reference"
		}
		t.Run(name, func(t *testing.T) {
			m := astTestManager(t)
			sort, e := m.MkBoolSort()
			astTestMust(t, e)
			old, e := m.MkVar(11700, sort)
			astTestMust(t, e)
			incoming, e := m.MkVar(11701, sort)
			astTestMust(t, e)
			dst, e := m.NewRef(old)
			astTestMust(t, e)
			var src ASTRef
			if viaRef {
				src, e = m.NewRef(incoming)
			} else {
				e = m.IncRef(incoming)
			}
			astTestMust(t, e)
			m.BudgetField.FailAfter(0)
			if viaRef {
				e = m.AssignRef(dst, src)
			} else {
				e = m.SetRef(dst, incoming)
			}
			m.BudgetField.FailAfter(-1)
			astTestMust(t, e)
			got, e := m.RefGet(dst)
			astTestMust(t, e)
			if got != incoming {
				t.Fatal("native assignment did not publish incoming", got)
			}
			if m.IsLiveHandle(old) {
				t.Fatal("old reference survived")
			}
			record, e := m.reference(dst)
			astTestMust(t, e)
			if !record.owned || len(record.pending) != 0 {
				t.Fatal("native assignment retained pending receipt", record)
			}
			astTestMust(t, m.DelRef(&dst))
			if rc, e := m.GetRefCount(incoming); e != nil || rc != 1 {
				t.Fatal("incoming reference after close", rc, e)
			}
			if viaRef {
				astTestMust(t, m.DelRef(&src))
			} else {
				astTestMust(t, m.DecRef(incoming))
			}
		})
	}
}

func TestASTRefNativeResetUnderRefusedArena(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	old, e := m.MkVar(11710, sort)
	astTestMust(t, e)
	ref, e := m.NewRef(old)
	astTestMust(t, e)
	m.BudgetField.FailAfter(0)
	astTestMust(t, m.ResetRef(ref))
	m.BudgetField.FailAfter(-1)
	got, e := m.RefGet(ref)
	astTestMust(t, e)
	if !got.IsNull() {
		t.Fatal("native reset retained pointer", got)
	}
	if m.IsLiveHandle(old) {
		t.Fatal("old reference survived reset")
	}
	astTestMust(t, m.DelRef(&ref))
}
