package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:570-586.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func bvPositionKeys(t *testing.T, m *ast.ASTManager, n int) []ast.ASTHandle {
	t.Helper()
	s, e := m.BitVectorSort(8)
	astTestMust(t, e)
	keys := make([]ast.ASTHandle, n)
	for i := range keys {
		keys[i], e = m.MkVar(uint32(i), s)
		astTestMust(t, e)
		astTestPin(t, m, keys[i])
	}
	return keys
}

func TestBVPolynomialPositionsNativeGrowthUpdateAndBorrowedKeys(t *testing.T) {
	m, _ := bvAddFixture(t)
	keys := bvPositionKeys(t, m, 257)
	before := m.Budget().Used()
	p, e := newBVPolynomialPositions(m)
	astTestMust(t, e)
	defer func() {
		m.Budget().FailAfter(-1)
		p.close()
		if m.Budget().Used() != before {
			t.Fatal("shared binding header retained")
		}
	}()
	used, storage := m.Budget().Used(), m.Stats()
	m.Budget().FailAfter(0)
	for i, key := range keys {
		rc, e := m.GetRefCount(key)
		astTestMust(t, e)
		astTestMust(t, p.insert(key, uint32(i)))
		after, e := m.GetRefCount(key)
		astTestMust(t, e)
		if rc != after {
			t.Fatal("native map acquired a borrowed key")
		}
	}
	astTestMust(t, p.insert(keys[0], 999))
	for i, key := range keys {
		want := uint32(i)
		if i == 0 {
			want = 999
		}
		got, ok, e := p.find(key)
		astTestMust(t, e)
		if !ok || got != want {
			t.Fatal("native map lost a position", i, got)
		}
	}
	if len(p.entries) != len(keys) || m.Budget().Used() != used || m.Stats() != storage {
		t.Fatal("native growth changed actual object storage")
	}
	astTestMust(t, p.reset())
	if len(p.entries) != 0 {
		t.Fatal("native reset retained positions")
	}
	astTestMust(t, p.insert(keys[1], 42))
	v, ok, e := p.find(keys[1])
	astTestMust(t, e)
	if !ok || v != 42 || m.Budget().Used() != used || m.Stats() != storage {
		t.Fatal("native reset/reuse changed actual object storage")
	}
}

func TestBVPolynomialPositionsNativeBorrowedKeyLifetimeAndReset(t *testing.T) {
	m, _ := bvAddFixture(t)
	s, e := m.BitVectorSort(8)
	astTestMust(t, e)
	astTestPin(t, m, s)
	keys := make([]ast.ASTHandle, 4)
	pins := make([]ast.ASTRef, len(keys))
	for i := range keys {
		keys[i], e = m.MkVar(uint32(i), s)
		astTestMust(t, e)
		pins[i], e = m.NewRef(keys[i])
		astTestMust(t, e)
	}
	t.Cleanup(func() {
		for i := range pins {
			if pins[i] != (ast.ASTRef{}) {
				astTestMust(t, m.DelRef(&pins[i]))
			}
		}
	})
	p, e := newBVPolynomialPositions(m)
	astTestMust(t, e)
	defer p.close()
	for i, h := range keys {
		astTestMust(t, p.insert(h, uint32(i+1)))
	}
	for i := range pins {
		astTestMust(t, m.DelRef(&pins[i]))
	}
	if m.IsLiveHandle(keys[0]) {
		t.Fatal("map retained an expression reference")
	}
	if ast.ASTValidateHandles { // foreign, stale or closed-owner handles are rejected only in validating builds
		if _, _, e := p.find(keys[0]); !errors.Is(e, ast.ErrASTHandle) {
			t.Fatal("stale queried handle was not validated", e)
		}
	}
	// Native clear and destruction never dereference borrowed stored keys.
	astTestMust(t, p.reset())
	if len(p.entries) != 0 {
		t.Fatal("reset retained stale borrowed keys")
	}
}

func TestBVPolynomialPositionsColdBindingAndNativeReset(t *testing.T) {
	m, _ := bvAddFixture(t)
	keys := bvPositionKeys(t, m, 14)
	before := m.Budget().Used()
	for ordinal := int64(0); ordinal < 2; ordinal++ {
		m.Budget().FailAfter(ordinal)
		p, e := newBVPolynomialPositions(m)
		m.Budget().FailAfter(-1)
		if ordinal == 0 {
			if p != nil || !errors.Is(e, arena.ErrInjectedFailure) {
				t.Fatal("actual cold binding refusal", e)
			}
		} else {
			astTestMust(t, e)
			used := m.Budget().Used()
			m.Budget().FailAfter(0)
			for i, key := range keys {
				astTestMust(t, p.insert(key, uint32(i)))
			}
			for i := 0; i < 3; i++ {
				astTestMust(t, p.reset())
				astTestMust(t, p.insert(keys[0], uint32(i)))
			}
			if m.Budget().Used() != used || len(p.entries) != 1 {
				t.Fatal("native sparse reset introduced an allocation frontier")
			}
			m.Budget().FailAfter(-1)
			p.close()
			p.close()
			if e := p.insert(keys[0], 1); !errors.Is(e, ast.ErrASTPending) {
				t.Fatal("closed shared binding accepted mutation", e)
			}
		}
		if m.Budget().Used() != before {
			t.Fatal("cold binding retained storage")
		}
	}
}
