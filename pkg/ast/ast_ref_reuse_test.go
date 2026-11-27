package ast

import (
	"errors"
	"testing"
)

func TestRecycledReferenceRetainsGenerationAndOwnership(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	astTestPin(t, m, sort)
	before, err := m.GetRefCount(sort)
	astTestMust(t, err)
	ref, err := m.NewRef(ASTHandle{})
	astTestMust(t, err)
	stale := ref
	astTestMust(t, m.DelRef(&ref))
	if _, err := m.NewRef(ASTHandle{origin: m.origin}); !errors.Is(err, ErrASTHandle) {
		t.Fatalf("malformed handle on warm free list returned %v", err)
	}
	ref, err = m.NewRef(sort)
	astTestMust(t, err)
	if ref.slot != stale.slot || ref.generation != stale.generation+1 {
		t.Fatalf("recycled reference %v after %v", ref, stale)
	}
	if err := m.DelRef(&stale); !errors.Is(err, ErrASTHandle) {
		t.Fatalf("deleting stale owner returned %v", err)
	}
	count, err := m.GetRefCount(sort)
	astTestMust(t, err)
	if count != before+1 {
		t.Fatalf("reused reference holds %d references, want %d", count, before+1)
	}
	astTestMust(t, m.DelRef(&ref))
	count, err = m.GetRefCount(sort)
	astTestMust(t, err)
	if count != before {
		t.Fatalf("shared count after release %d, want %d", count, before)
	}
}

func BenchmarkASTRecycledReference(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	astTestPin(b, m, sort)
	for _, tc := range []struct {
		name string
		node ASTHandle
	}{{"Empty", ASTHandle{}}, {"Shared", sort}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				ref, err := m.NewRef(tc.node)
				if err != nil {
					b.Fatal(err)
				}
				if err := m.DelRef(&ref); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// A failed owning release must leave the reference available for retry, even
// if its node slot currently names another live generation with several owners.
func TestSharedReferenceReleaseRejectsStaleNode(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	astTestPin(t, m, sort)
	ref, err := m.NewRef(sort)
	astTestMust(t, err)
	record, err := m.reference(ref)
	astTestMust(t, err)
	original := record.node
	record.node.generation++
	err = m.DelRef(&ref)
	record.node = original
	if !errors.Is(err, ErrASTHandle) || ref == (ASTRef{}) {
		t.Fatalf("stale release returned %v and left reference %v", err, ref)
	}
	astTestMust(t, m.DelRef(&ref))
}
