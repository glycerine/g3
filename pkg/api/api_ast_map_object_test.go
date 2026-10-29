package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIASTMapConcreteReferenceCleanup(t *testing.T) {
	m := astTestManager(t)
	c, e := newAPIContextResults(m, false)
	astTestMust(t, e)
	a, e := newAPIASTMapObject(c)
	astTestMust(t, e)
	astTestMust(t, c.saveObject(&a.object))
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	key, e := m.MkVar(33001, sort)
	astTestMust(t, e)
	value, e := m.MkVar(33002, sort)
	astTestMust(t, e)
	for _, h := range []ast.ASTHandle{key, value, sort} {
		astTestMust(t, m.IncRef(h))
	}
	checkRefs := func(h ast.ASTHandle, want uint32) {
		t.Helper()
		rc, e := m.GetRefCount(h)
		if e != nil || rc != want {
			t.Fatalf("AST map ref count = %d, want %d: %v", rc, want, e)
		}
	}
	keyBase, e := m.GetRefCount(key)
	astTestMust(t, e)
	valueBase, e := m.GetRefCount(value)
	astTestMust(t, e)
	sortBase, e := m.GetRefCount(sort)
	astTestMust(t, e)
	astTestMust(t, a.insert(key, value))
	checkRefs(key, keyBase+1)
	checkRefs(value, valueBase+1)
	astTestMust(t, a.insert(key, value)) // replacement INC precedes DEC
	checkRefs(value, valueBase+1)
	astTestMust(t, a.insert(key, sort)) // any AST kind is a valid value
	checkRefs(value, valueBase)
	checkRefs(sort, sortBase+1)
	if got, e := a.find(key); e != nil || got != sort {
		t.Fatal("replacement lookup", got, e)
	}
	if size, e := a.size(); e != nil || size != 1 {
		t.Fatal("map size", size, e)
	}
	if keys, e := a.keys(); e != nil || len(keys) != 1 || keys[0] != key {
		t.Fatal("borrowed keys", keys, e)
	}
	astTestMust(t, a.erase(key))
	checkRefs(key, keyBase)
	checkRefs(sort, sortBase)
	if _, e := a.find(key); !errors.Is(e, errAPIASTMapMissing) {
		t.Fatal("missing key category", e)
	}
	astTestMust(t, a.insert(key, value))
	astTestMust(t, a.reset())
	checkRefs(key, keyBase)
	checkRefs(value, valueBase)
	astTestMust(t, a.insert(key, value))
	astTestMust(t, c.Close()) // derived destructor runs dec_ref_key_values
	checkRefs(key, keyBase)
	checkRefs(value, valueBase)
	for _, h := range []ast.ASTHandle{key, value, sort} {
		astTestMust(t, m.DecRef(h))
	}
}

func TestAPIASTMapKeysOwnIndependentVectorRootsAndDisplay(t *testing.T) {
	m, c := apiResultsFixture(t, false)
	a, err := newAPIASTMapObject(c)
	astTestMust(t, err)
	astTestMust(t, c.saveObject(&a.object))
	astTestMust(t, a.object.incRef()) // external map handle survives the next object result.
	key := apiResultsAtom(t, m, "api-map-key-vector")
	value := apiResultsAtom(t, m, "api-map-value-vector")
	astTestMust(t, a.insert(key, value))

	text, err := a.string()
	astTestMust(t, err)
	if !strings.HasPrefix(text, "(ast-map\n  (") || !strings.Contains(text, "api-map-key-vector") || !strings.Contains(text, "api-map-value-vector") || !strings.HasSuffix(text, "))") {
		t.Fatal("source AST-map display", text)
	}
	v, err := a.keyVector()
	astTestMust(t, err)
	if n, e := v.size(); e != nil || n != 1 {
		t.Fatal("key vector size", n, e)
	}
	ownedKey, err := v.get(0)
	astTestMust(t, err)
	if ownedKey != key {
		t.Fatal("key vector changed handle", ownedKey, key)
	}
	astTestMust(t, a.reset())
	if _, e := m.GetKind(value); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("map value survived reset", e)
	}
	if _, e := m.GetKind(ownedKey); e != nil {
		t.Fatal("key vector borrowed map ownership", e)
	}
	astTestMust(t, c.Close())
	if _, e := m.GetKind(ownedKey); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("key vector failed to release key", e)
	}
}

func TestAPIASTMapResetNativeIDRecycleWithoutBackingReservation(t *testing.T) {
	m := astTestManager(t)
	c, err := newAPIContextResults(m, false)
	astTestMust(t, err)
	a, err := newAPIASTMapObject(c)
	astTestMust(t, err)
	astTestMust(t, c.saveObject(&a.object))
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	key, err := m.MkVar(33111, sort)
	astTestMust(t, err)
	value, err := m.MkVar(33112, sort)
	astTestMust(t, err)
	astTestMust(t, a.insert(key, value))
	m.ExprIDs.Cleanup(m.ExprIDs.Range())
	m.Budget().FailAfter(0)
	err = a.reset()
	m.Budget().FailAfter(-1)
	astTestMust(t, err)
	if len(a.entries) != 0 || len(a.pending) != 0 {
		t.Fatal("native ID recycle left map owners", a.entries, a.pending)
	}
	if _, err := m.GetKind(key); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("released key should be invalid", err)
	}
	if _, err := m.GetKind(value); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("released value should be invalid", err)
	}
	astTestMust(t, c.Close())
}

func TestAPIASTMapExpressionPairTransfer(t *testing.T) {
	m := astTestManager(t)
	c, e := newAPIContextResults(m, false)
	astTestMust(t, e)
	a, e := newAPIASTMapObject(c)
	astTestMust(t, e)
	astTestMust(t, c.saveObject(&a.object))
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	key, e := m.MkVar(33011, sort)
	astTestMust(t, e)
	value, e := m.MkVar(33012, sort)
	astTestMust(t, e)
	for _, h := range []ast.ASTHandle{key, value} {
		astTestMust(t, m.IncRef(h))
	}
	s, e := ast.NewASTExprMapWithProofs(m, false)
	astTestMust(t, e)
	astTestMust(t, s.Insert(key, value, ast.ASTHandle{}))
	astTestMust(t, a.copyExprPairs(s))
	astTestMust(t, s.Close())
	if got, e := a.find(key); e != nil || got != value {
		t.Fatal("API map lost source-local pair", got, e)
	}
	for _, h := range []ast.ASTHandle{key, value} {
		rc, e := m.GetRefCount(h)
		if e != nil || rc != 2 {
			t.Fatal("API map failed to retain pair", rc, e)
		}
	}
	astTestMust(t, c.Close())
	for _, h := range []ast.ASTHandle{key, value} {
		astTestMust(t, m.DecRef(h))
	}
}
