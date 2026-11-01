package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"math/big"
	"testing"
)

func TestDualityExtractStoresPersistentNamesAndOnePassUndo(t *testing.T) {
	ctx, m, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	zero := wrap(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
	one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	two := wrap(m.ArithmeticNumeralRat(big.NewRat(2, 1), true))
	integer, err := m.GetSort(raw(zero))
	must(err)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), integer)
	must(err)
	sortRef, err := m.NewRef(arraySort)
	must(err)
	defer func() { must(m.DelRef(&sortRef)) }()
	base := wrap(m.MkConstArray(arraySort, raw(zero)))
	family, err := m.ArrayFamilyID()
	must(err)
	store := func(array, index, value *Expr) *Expr {
		return wrap(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{raw(array), raw(index), raw(value)}))
	}
	inner := store(base, one, one)
	outer := store(inner, two, two)
	extraction, err := NewStoreExtraction(ctx)
	must(err)
	defer func() { must(extraction.Close()) }()
	named, err := extraction.Extract(outer)
	must(err)
	defer named.Close()
	names, err := extraction.NamingConstraints()
	must(err)
	defer func() {
		for _, name := range names {
			must(name.Close())
		}
	}()
	if len(names) != 2 {
		t.Fatal("nested stores did not create two source-order definitions")
	}
	innerName, err := m.GetArg(raw(names[0]), 0)
	must(err)
	outerName, err := m.GetArg(raw(names[1]), 0)
	must(err)
	if raw(named) != outerName || innerName == outerName {
		t.Fatal("fresh store names not distinct/reused")
	}
	secondRHS, err := m.GetArg(raw(names[1]), 1)
	must(err)
	child, err := m.GetArg(secondRHS, 0)
	must(err)
	if child != innerName {
		t.Fatal("nested store definition did not refer to earlier name")
	}
	repeated, err := extraction.Extract(outer)
	must(err)
	defer repeated.Close()
	if raw(repeated) != outerName || len(extraction.constraints) != 2 {
		t.Fatal("memoized store renamed twice")
	}
	pair := wrap(m.MkEq(raw(outer), raw(outer)))
	shared, err := extraction.Extract(pair)
	must(err)
	defer shared.Close()
	for i := uint32(0); i < 2; i++ {
		h, err := m.GetArg(raw(shared), i)
		must(err)
		if h != outerName {
			t.Fatal("shared input store did not share fresh output")
		}
	}
	once, err := extraction.Undo(named)
	must(err)
	defer once.Close()
	if raw(once) != secondRHS || raw(once) == raw(outer) {
		t.Fatal("source one-pass nested reverse substitution changed")
	}
	twice, err := extraction.Undo(once)
	must(err)
	defer twice.Close()
	if raw(twice) != raw(outer) {
		t.Fatal("second reverse pass did not restore nested store")
	}
	// Source extraction leaves entire quantified expressions opaque.
	opaque := wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: raw(pair)}))
	unchanged, err := extraction.Extract(opaque)
	must(err)
	defer unchanged.Close()
	if raw(unchanged) != raw(opaque) || len(extraction.constraints) != 2 {
		t.Fatal("store extraction descended under quantifier")
	}
	// Independent native solver validation of the generated naming equations.
	selected := wrap(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{raw(outer), raw(two)}))
	formula := wrap(m.MkEq(raw(selected), raw(two)))
	transformed, err := extraction.Extract(formula)
	must(err)
	defer transformed.Close()
	difference := wrap(m.MkNot(raw(wrap(m.MkIff(raw(formula), raw(transformed))))))
	must(s.Push())
	for _, name := range names {
		must(s.Assert(raw(name)))
	}
	must(s.Assert(raw(difference)))
	status, err := s.CheckSat(nil)
	must(err)
	must(s.Pop(1))
	if status != lbool.False {
		t.Fatal("naming equations changed array formula", status)
	}
	must(extraction.Close())
	// Caller-owned outputs survive destruction of all cache-owned references.
	if raw(named) != outerName || raw(names[1]) == (ast.ASTHandle{}) {
		t.Fatal("output lifetime depended on cache")
	}
	if _, err := extraction.Extract(outer); err == nil {
		t.Fatal("closed store cache accepted extraction")
	}
}
