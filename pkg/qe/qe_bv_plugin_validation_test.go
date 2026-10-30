package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBitVectorPluginRejectsNonBitVectorTarget(t *testing.T) {
	f := newBVPluginFixture(t, 8)
	integer := f.base.constant("qe-bv-not-bv", f.base.integer)
	contains, err := ast.NewContainsApp(f.manager, integer)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	if _, err := f.plugin.Weight(contains, f.formula); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("non-BV error=%v", err)
	}
	if _, err := f.plugin.GetNumBranches(nil, f.formula, f.rational("0")); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatalf("nil contains error=%v", err)
	}
}
