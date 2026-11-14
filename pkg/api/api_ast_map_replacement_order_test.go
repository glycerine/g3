package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIASTMapReplacementNativeIDRecycleBeforePublishingNew(t *testing.T) {
	m := astTestManager(t)
	c, err := newAPIContextResults(m, false)
	astTestMust(t, err)
	a, err := newAPIASTMapObject(c)
	astTestMust(t, err)
	astTestMust(t, c.saveObject(&a.object))
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	key, err := m.MkVar(33101, sort)
	astTestMust(t, err)
	old, err := m.MkVar(33102, sort)
	astTestMust(t, err)
	incoming, err := m.MkVar(33103, sort)
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{sort, key, incoming} {
		astTestMust(t, m.IncRef(h))
	}
	astTestMust(t, a.insert(key, old))
	m.ExprIDs.Cleanup(m.ExprIDs.Range())
	m.Budget().FailAfter(0)
	err = a.insert(key, incoming)
	m.Budget().FailAfter(-1)
	astTestMust(t, err)
	got, err := a.find(key)
	if err != nil || got != incoming || !a.entries[key].valueOwned {
		t.Fatalf("replacement publication after native recycle: got=%v, err=%v, entry=%+v", got, err, a.entries[key])
	}
	if live := m.IsLiveHandle(old); live || len(a.pending) != 0 {
		t.Fatalf("old value survived or pending owner remained: old live=%v, pending=%d", live, len(a.pending))
	}
	astTestMust(t, a.reset())
	if rc, err := m.GetRefCount(incoming); err != nil || rc != 1 {
		t.Fatalf("incoming reference cleanup: rc=%d, err=%v", rc, err)
	}
	astTestMust(t, c.Close())
	for _, h := range []ast.ASTHandle{incoming, key, sort} {
		astTestMust(t, m.DecRef(h))
	}
}
