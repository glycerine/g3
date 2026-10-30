package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticAtomSetOrderDuplicatesAndCopyGuard(t *testing.T) {
	f := newFMFixture(t)
	a := f.constant("atom-a", f.boolean)
	b := f.constant("atom-b", f.boolean)
	s, e := NewArithmeticAtomSet(f.r.manager)
	fmMust(t, e)
	fmMust(t, s.AddPositive(a))
	fmMust(t, s.AddPositive(b))
	fmMust(t, s.AddPositive(a))
	hs, e := s.Positive()
	fmMust(t, e)
	if len(hs) != 2 || hs[0] != a || hs[1] != b {
		t.Fatal(hs)
	}
	copy := *s
	if e = copy.Close(); e != ast.ErrASTHandle {
		t.Fatal(e)
	}
	fmMust(t, s.Close())
}
