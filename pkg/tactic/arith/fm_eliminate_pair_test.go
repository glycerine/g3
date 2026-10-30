package arith

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFMEliminatePairRegistersResolvent(t *testing.T) {
	store, v := fmResolutionFixture(t)
	x, y, z := v.Add(false, false), v.Add(false, false), v.Add(false, false)
	lower, err := store.New(nil, []uint32{x, y}, []big.Rat{fmCRat(-1, 1), fmCRat(1, 1)}, big.NewRat(2, 1), false, ast.ASTDependency{})
	equationMust(t, err)
	upper, err := store.New(nil, []uint32{x, z}, []big.Rat{fmCRat(1, 1), fmCRat(1, 1)}, big.NewRat(3, 1), false, ast.ASTDependency{})
	equationMust(t, err)
	v.Register(lower)
	v.Register(upper)
	e, err := NewFMEliminator(store, v, DefaultFMEliminationConfig())
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, e.Close()) })
	ok, err := e.Try(x)
	equationMust(t, err)
	if !ok || !lower.Dead || !upper.Dead {
		t.Fatal("source bounds were not retired")
	}
	var result *FMConstraint
	for _, c := range v.Index.Upper[y] {
		if !c.Dead {
			result = c
		}
	}
	if result == nil || len(result.Variables) != 2 || result.Constant.RatString() != "5" {
		t.Fatalf("resolvent = %#v", result)
	}
}
