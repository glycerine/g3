package qe_test

// Source: eq::der members m_subst_map and m_new_args in src/qe/qe_lite.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// AUDIT.md FUZZ_F-F4: m_subst_map keeps the substituted terms after
// reduce_quantifier returns; ~der releases them.
func TestEqualityDERSubstMapOutlivesReduceQuantifier(t *testing.T) {
	f := setup(t)
	x0, x1 := f.variable(t, 0, f.integer), f.variable(t, 1, f.integer)
	c := f.constant(t, "der-member-c", f.integer)
	g := termFunction(t, f, "der-member-g", f.integer, x1)
	p := predicate(t, f, "der-member-p", x0)
	body, err := f.m.MkAndArgs([]ast.ASTHandle{f.equality(t, x0, g), f.equality(t, x1, c), p})
	must(t, err)
	f.pin(t, body)
	var names []usymbol.Symbol
	for _, s := range []string{"x0", "x1"} {
		n, err := f.m.Symbols.Intern(s)
		must(t, err)
		names = append(names, n)
	}
	q, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer, f.integer}, Names: names, Body: body})
	must(t, err)
	f.pin(t, q)
	d := equalityReducer(t, f, nil, nil)
	refs := f.refs(t, ast.ASTHandle{}, ast.ASTHandle{})
	must(t, d.ReduceQuantifier(q, refs[0], refs[1]))
	// (g c) is built by create_substitution for x0.
	gc := termFunction(t, f, "der-member-g", f.integer, c)
	held, err := f.m.GetRefCount(gc)
	must(t, err)
	must(t, d.Close())
	released, err := f.m.GetRefCount(gc)
	must(t, err)
	if held != released+1 {
		t.Fatalf("m_subst_map did not hold (g c) until ~der: %d then %d", held, released)
	}
}
