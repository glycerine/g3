package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestAssumptionLiteralCacheReusesGuardsAndPreservesConjunctOrder(t *testing.T) {
	ctx, _, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := ctx.BoolConst("cache-p")
	must(err)
	closeExpr(t, p)
	negative, err := p.Not()
	must(err)
	closeExpr(t, negative)
	repeated, err := p.And(p)
	must(err)
	closeExpr(t, repeated)
	cache, err := NewAssumptionLiteralCache(ctx, s)
	must(err)
	defer cache.Close()
	mappings := make(map[ast.ASTHandle]ast.ASTHandle)
	literals, err := cache.GetAssumptionLits(repeated, mappings)
	must(err)
	if len(literals) != 2 {
		t.Fatal("duplicate conjunct removed")
	}
	first, err := literals[0].Raw()
	must(err)
	second, err := literals[1].Raw()
	must(err)
	if first != second || len(mappings) != 1 {
		t.Fatal("literal was not reused")
	}
	ph, err := p.Raw()
	must(err)
	if mappings[first] != ph {
		t.Fatal("wrong optional mapping")
	}
	for _, literal := range literals {
		must(literal.Close())
	}
	count, err := s.NumAssertions()
	must(err)
	if count != 1 {
		t.Fatal("duplicate guard assertion", count)
	}
	literals, err = cache.GetAssumptionLits(negative, mappings)
	must(err)
	defer literals[0].Close()
	nh, err := literals[0].Raw()
	must(err)
	if nh == first {
		t.Fatal("different conjunct shared literal")
	}
	check := func(assumptions []ast.ASTHandle, want lbool.LBool) {
		t.Helper()
		got, err := s.CheckSat(assumptions)
		must(err)
		if got != want {
			t.Fatal("guard semantics", got, want)
		}
	}
	check(nil, lbool.True)
	check([]ast.ASTHandle{first}, lbool.True)
	check([]ast.ASTHandle{first, nh}, lbool.False)
	again, err := cache.GetAssumptionLits(p, nil)
	must(err)
	got, err := again[0].Raw()
	must(err)
	if got != first {
		t.Fatal("cache key lost after caller release")
	}
	must(again[0].Close())
	count, err = s.NumAssertions()
	must(err)
	if count != 2 {
		t.Fatal("guard was reasserted", count)
	}
	must(cache.Close())
	must(cache.Close())
	// Solver assertions and caller copies own their ASTs after cache cleanup.
	check([]ast.ASTHandle{nh}, lbool.True)
	if _, err = cache.GetAssumptionLits(p, nil); err == nil {
		t.Fatal("closed cache accepted formula")
	}
}
