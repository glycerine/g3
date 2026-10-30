package arith

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFMConstraintStoreDependencyAndIDLifecycle(t *testing.T) {
	_, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, false, "fm_store_x")
	dep, e := m.DependencyLeaf(x)
	equationMust(t, e)
	s, e := NewFMConstraintStore(m)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, s.Close()) })
	c, e := s.New([]int32{1}, []uint32{0}, []big.Rat{fmCRat(2, 1)}, big.NewRat(3, 1), true, dep)
	equationMust(t, e)
	if rc, e := m.DependencyRefCount(dep); e != nil || rc != 1 {
		t.Fatalf("dependency rc after new = %d, %v", rc, e)
	}
	id := c.ID
	equationMust(t, s.Delete(c))
	if _, e := m.DependencyRefCount(dep); e != ast.ErrASTHandle {
		t.Fatalf("dependency remained live after final delete: %v", e)
	}
	next, e := s.New(nil, []uint32{1}, []big.Rat{fmCRat(1, 1)}, big.NewRat(0, 1), false, ast.ASTDependency{})
	equationMust(t, e)
	if next.ID != id {
		t.Fatalf("id was not recycled: got %d want %d", next.ID, id)
	}
}

func TestFMConstraintStoreRejectsMalformedShape(t *testing.T) {
	_, m := fixDLFixture(t, false)
	s, e := NewFMConstraintStore(m)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, s.Close()) })
	if _, e = s.New(nil, []uint32{0}, nil, big.NewRat(0, 1), false, ast.ASTDependency{}); e != ast.ErrASTDomain {
		t.Fatalf("shape error = %v", e)
	}
}
