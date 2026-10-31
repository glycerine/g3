package arith

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestBV2RealVectorGrowthFailurePreservesPrefixAndReferences(t *testing.T) {
	for _, declarations := range []bool{false, true} {
		t.Run(map[bool]string{false: "conditions", true: "declarations"}[declarations], func(t *testing.T) {
			u, f := bv2RealFixture(t, 64)
			h, err := f.m.MkTrue()
			equationMust(t, err)
			if declarations {
				h, err = u.PositiveLTDecl()
				equationMust(t, err)
			}
			fixDLPin(t, f.m, h)
			push := u.AddSideCondition
			values := &u.side
			if declarations {
				push, values = u.AddAuxDecl, &u.decls
			} else {
				equationMust(t, push(h))
				equationMust(t, push(h))
			}
			// Both vectors have a full first allocation of two elements.
			before := append([]ast.ASTHandle(nil), (*values)...)
			refs, err := f.m.GetRefCount(h)
			equationMust(t, err)
			used := f.m.Budget().Used()
			f.m.Budget().FailAfter(0)
			err = push(h)
			f.m.Budget().FailAfter(-1)
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("missing vector allocation frontier", err)
			}
			if len(*values) != len(before) || f.m.Budget().Used() != used {
				t.Fatal("failed vector growth changed the completed prefix or charge")
			}
			for i, prior := range before {
				if (*values)[i] != prior {
					t.Fatal("failed vector growth changed a prior element", i)
				}
			}
			after, err := f.m.GetRefCount(h)
			equationMust(t, err)
			if after != refs {
				t.Fatal("failed insertion lost its uncommitted reference", refs, after)
			}
			equationMust(t, push(h))
			if len(*values) != len(before)+1 || cap(*values) != 3 {
				t.Fatal("growth retry did not append after the completed prefix")
			}
		})
	}
}

func TestBV2RealSideVectorResetReusesCapacityAndCloseReleasesStorage(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	h, err := f.m.MkTrue()
	equationMust(t, err)
	fixDLPin(t, f.m, h)
	refs, err := f.m.GetRefCount(h)
	equationMust(t, err)
	for i := 0; i < 8; i++ {
		equationMust(t, u.AddSideCondition(h))
	}
	capacity, bytes := cap(u.side), u.sideBytes
	if capacity != 8 || bytes == 0 {
		t.Fatal("source growth sequence 2,3,5,8", capacity, bytes)
	}
	used := f.m.Budget().Used()
	equationMust(t, u.Reset())
	if len(u.side) != 0 || cap(u.side) != capacity || u.sideBytes != bytes || f.m.Budget().Used() != used {
		t.Fatal("reset discarded vector storage")
	}
	after, err := f.m.GetRefCount(h)
	equationMust(t, err)
	if after != refs {
		t.Fatal("reset did not release every condition reference", refs, after)
	}
	f.m.Budget().FailAfter(0)
	for i := 0; i < capacity; i++ {
		equationMust(t, u.AddSideCondition(h))
	}
	err = u.AddSideCondition(h)
	f.m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || len(u.side) != capacity {
		t.Fatal("reset capacity was not reused through the next growth frontier", err)
	}
	equationMust(t, u.Close())
	if u.sideBytes != 0 || u.declBytes != 0 || u.side != nil || u.decls != nil {
		t.Fatal("vector storage survived destruction")
	}
	after, err = f.m.GetRefCount(h)
	equationMust(t, err)
	if after != refs {
		t.Fatal("destruction did not release condition references", refs, after)
	}
}
