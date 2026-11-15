package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func elimBoundsQuantifier(t *testing.T, f factorFixture, forall bool, sorts []ast.ASTHandle, body ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	names := make([]usymbol.Symbol, len(sorts))
	for i := range names {
		names[i] = astTestSymbol(t, f.m, "elim-bound-v")
	}
	q, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: body})
	astTestMust(t, err)
	astTestPin(t, f.m, q)
	return q
}

func TestElimBoundsRecognizesSourceDifferenceBounds(t *testing.T) {
	f := newFactorFixture(t)
	c, err := NewElimBoundsConfiguration(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	x, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	y, err := f.m.MkVar(1, f.sort)
	astTestMust(t, err)
	five := f.numeral(5)
	minusOne := f.numeral(-1)

	le := f.app(2, x, five)
	pair, matched, err := c.isBound(le)
	astTestMust(t, err)
	if !matched || pair.upper != x || pair.lower != (ast.ASTHandle{}) {
		t.Fatal("simple upper bound", matched, pair)
	}
	ge := f.app(3, x, five)
	notGE, err := f.m.MkNot(ge)
	astTestMust(t, err)
	astTestPin(t, f.m, notGE)
	pair, matched, err = c.isBound(notGE)
	astTestMust(t, err)
	if !matched || pair.upper != x || pair.lower != (ast.ASTHandle{}) {
		t.Fatal("negated lower bound", matched, pair)
	}

	minusY := f.app(9, minusOne, y)
	difference := f.app(6, x, minusY)
	pair, matched, err = c.isBound(f.app(2, difference, five))
	astTestMust(t, err)
	if !matched || pair.upper != x || pair.lower != y {
		t.Fatal("difference bound", matched, pair)
	}

	nonlinear := f.app(6, x, x)
	notDifference := f.app(6, nonlinear, minusY)
	_, matched, err = c.isBound(f.app(2, notDifference, five))
	astTestMust(t, err)
	if matched {
		t.Fatal("nonground non-variable first term accepted")
	}
	_, matched, err = c.isBound(f.app(2, x, y))
	astTestMust(t, err)
	if matched {
		t.Fatal("nonnumeral right-hand side accepted")
	}
}

func TestElimBoundsRemovesOnlyOneSidedUnusedVariables(t *testing.T) {
	f := newFactorFixture(t)
	rw, err := NewElimBounds(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	x, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	y, err := f.m.MkVar(1, f.sort)
	astTestMust(t, err)
	five := f.numeral(5)
	bound := f.app(2, x, five)

	out, err := f.m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&out)) }()

	onlyBound := elimBoundsQuantifier(t, f, true, []ast.ASTHandle{f.sort}, bound)
	astTestMust(t, rw.Rewrite(onlyBound, out))
	got, err := f.m.RefGet(out)
	astTestMust(t, err)
	falseValue, err := f.m.MkFalse()
	astTestMust(t, err)
	if got != falseValue {
		t.Fatal("sole one-sided bound was not reduced to false", got, falseValue)
	}
	wrapped, err := f.m.MkNot(onlyBound)
	astTestMust(t, err)
	astTestPin(t, f.m, wrapped)
	astTestMust(t, rw.Rewrite(wrapped, out))
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	child, err := f.m.GetArg(got, 0)
	astTestMust(t, err)
	if child != falseValue {
		t.Fatal("nested quantifier was not reduced", child, falseValue)
	}

	boolSort, err := f.m.MkBoolSort()
	astTestMust(t, err)
	predicate, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "elim-bound-p"), []ast.ASTHandle{f.sort}, boolSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.m, predicate)
	py, err := f.m.MkApp(predicate, []ast.ASTHandle{y})
	astTestMust(t, err)
	astTestPin(t, f.m, py)
	body, err := f.m.MkOr(bound, py)
	astTestMust(t, err)
	astTestPin(t, f.m, body)
	q := elimBoundsQuantifier(t, f, true, []ast.ASTHandle{f.sort, f.sort}, body)
	astTestMust(t, rw.Rewrite(q, out))
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	if count, err := f.m.GetNumDeclarations(got); err != nil || count != 1 {
		t.Fatal("unused bounded declaration count", count, err)
	}
	newBody, err := f.m.GetQuantifierBody(got)
	astTestMust(t, err)
	argument, err := f.m.GetArg(newBody, 0)
	astTestMust(t, err)
	if index, err := f.m.GetVarIndex(argument); err != nil || index != 0 {
		t.Fatal("remaining variable remapping", index, err)
	}
}

func TestElimBoundsRetainsTwoSidedAndExistentialBounds(t *testing.T) {
	f := newFactorFixture(t)
	rw, err := NewElimBounds(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	x, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	zero, five := f.numeral(0), f.numeral(5)
	lower := f.app(3, x, zero)
	upper := f.app(2, x, five)
	body, err := f.m.MkOr(lower, upper)
	astTestMust(t, err)
	astTestPin(t, f.m, body)
	forall := elimBoundsQuantifier(t, f, true, []ast.ASTHandle{f.sort}, body)
	exists := elimBoundsQuantifier(t, f, false, []ast.ASTHandle{f.sort}, upper)
	out, err := f.m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&out)) }()
	for name, root := range map[string]ast.ASTHandle{"two-sided": forall, "existential": exists} {
		astTestMust(t, rw.Rewrite(root, out))
		got, err := f.m.RefGet(out)
		astTestMust(t, err)
		if got != root {
			t.Fatal(name, "quantifier changed", got, root)
		}
	}
}

func TestElimBoundsProofProducingEntry(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	astTestPin(t, m, sort)
	f := factorFixture{t: t, m: m, family: family, sort: sort}
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	bound := f.app(2, x, f.numeral(5))
	q := elimBoundsQuantifier(t, f, true, []ast.ASTHandle{sort}, bound)
	rw, err := NewElimBounds(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	proof, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() {
		astTestMust(t, m.DelRef(&proof))
		astTestMust(t, m.DelRef(&out))
	}()
	for name, root := range map[string]ast.ASTHandle{"rewrite": q, "reflexivity": x} {
		astTestMust(t, rw.RewriteWithProof(root, out, proof))
		p, err := m.RefGet(proof)
		astTestMust(t, err)
		if p == (ast.ASTHandle{}) {
			t.Fatal(name, "proof missing")
		}
	}
}

// AUDIT.md ast_rewriter_03-F3: elim_bounds_cfg::is_bound only asks for the
// value -1; a Real -1.0 coefficient is a difference bound too.
func TestElimBoundsRecognizesRealMinusOne(t *testing.T) {
	f := newFactorFixture(t)
	realSort, err := f.m.MkFamilySort(f.family, 0, nil)
	astTestMust(t, err)
	astTestPin(t, f.m, realSort)
	f.sort = realSort
	realNumeral := func(value int64) ast.ASTHandle {
		t.Helper()
		q := f.m.NumeralService()
		n, err := q.NewValue()
		astTestMust(t, err)
		astTestMust(t, q.SetInt64(&n, value))
		defer func() { astTestMust(t, q.Del(&n)) }()
		h, err := f.m.ArithmeticNumeral(&n, false)
		astTestMust(t, err)
		astTestPin(t, f.m, h)
		return h
	}
	c, err := NewElimBoundsConfiguration(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	x, err := f.m.MkVar(0, realSort)
	astTestMust(t, err)
	y, err := f.m.MkVar(1, realSort)
	astTestMust(t, err)
	three := realNumeral(3)
	minusY := f.app(9, realNumeral(-1), y)
	bound := f.app(2, f.app(6, x, minusY), three)
	pair, matched, err := c.isBound(bound)
	astTestMust(t, err)
	if !matched || pair.upper != x || pair.lower != y {
		t.Fatal("Real difference bound", matched, pair)
	}

	rw, err := NewElimBounds(f.m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	out, err := f.m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&out)) }()
	q := elimBoundsQuantifier(t, f, true, []ast.ASTHandle{realSort, realSort}, bound)
	astTestMust(t, rw.Rewrite(q, out))
	got, err := f.m.RefGet(out)
	astTestMust(t, err)
	falseValue, err := f.m.MkFalse()
	astTestMust(t, err)
	if got != falseValue {
		t.Fatal("Real difference bound quantifier was not reduced to false", got, falseValue)
	}
}
