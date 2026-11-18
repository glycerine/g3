package qe_test

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/qe"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type arithmeticQEFixture struct {
	t             *testing.T
	manager       *ast.ASTManager
	util          *qe.ArithmeticQETermUtil
	integer, real ast.ASTHandle
	x, y          ast.ASTHandle
	rx, ry        ast.ASTHandle
	config        qe.Config
}

func arithmeticQEMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newArithmeticQEFixture(t *testing.T) *arithmeticQEFixture {
	t.Helper()
	command, err := cmd_context.NewStandaloneContext()
	arithmeticQEMust(t, err)
	t.Cleanup(func() { arithmeticQEMust(t, command.Close()) })
	m, err := command.Manager()
	arithmeticQEMust(t, err)
	integer, err := command.LookupSort("Int")
	arithmeticQEMust(t, err)
	real, err := command.LookupSort("Real")
	arithmeticQEMust(t, err)
	g, err := command.GlobalParameters()
	arithmeticQEMust(t, err)
	config := qe.Config{Context: g.Context, Globals: g}
	u, err := qe.NewArithmeticQETermUtil(m, config)
	arithmeticQEMust(t, err)
	f := &arithmeticQEFixture{t: t, manager: m, util: u, integer: integer, real: real, config: config}
	f.x = f.pin(m.MkFreshConst("qe-arith-x", integer))
	f.y = f.pin(m.MkFreshConst("qe-arith-y", integer))
	f.rx = f.pin(m.MkFreshConst("qe-arith-rx", real))
	f.ry = f.pin(m.MkFreshConst("qe-arith-ry", real))
	t.Cleanup(func() { arithmeticQEMust(t, u.Close()) })
	return f
}

func (f *arithmeticQEFixture) pin(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	arithmeticQEMust(f.t, err)
	ref, err := f.manager.NewRef(h)
	arithmeticQEMust(f.t, err)
	f.t.Cleanup(func() { arithmeticQEMust(f.t, f.manager.DelRef(&ref)) })
	return h
}

func (f *arithmeticQEFixture) integerNumeral(t *testing.T, text string) ast.ASTHandle {
	t.Helper()
	return f.pin(f.manager.ArithmeticIntegerNumeralText(text))
}

func (f *arithmeticQEFixture) realNumeral(t *testing.T, text string) ast.ASTHandle {
	t.Helper()
	return f.pin(f.manager.ArithmeticRealNumeralText(text))
}

func TestArithmeticQECoefficientAndConstantSourceOrder(t *testing.T) {
	f := newArithmeticQEFixture(t)
	two := f.integerNumeral(t, "2")
	three := f.integerNumeral(t, "3")
	twoX := f.pin(f.manager.MkArithmeticMul(two, f.x))
	polynomial := f.pin(f.manager.MkArithmeticAdd(f.y, twoX))
	contains, err := ast.NewContainsApp(f.manager, f.x)
	arithmeticQEMust(t, err)
	defer func() { arithmeticQEMust(t, contains.Close()) }()
	coefficient := unumeral.NewRational()
	rest, matched, err := f.util.Coefficient(contains, polynomial, &coefficient)
	arithmeticQEMust(t, err)
	if !matched || !isSmallRat(&coefficient, 2, 1) {
		t.Fatal("coefficient match", matched)
	}
	restHandle, err := f.manager.RefGet(rest)
	arithmeticQEMust(t, err)
	if restHandle != f.y {
		t.Fatal("coefficient rest differs from y")
	}
	arithmeticQEMust(t, f.manager.DelRef(&rest))

	repeated := f.pin(f.manager.MkArithmeticAdd(f.x, f.x))
	rest, matched, err = f.util.Coefficient(contains, repeated, &coefficient)
	arithmeticQEMust(t, err)
	if matched || rest != (ast.ASTRef{}) {
		t.Fatal("nonlinear repeated variable accepted")
	}

	inner := f.pin(f.manager.MkArithmeticAdd(three, f.x))
	withConstant := f.pin(f.manager.MkArithmeticAdd(f.y, inner))
	constant := unumeral.NewRational()
	rest, err = f.util.Constant(withConstant, &constant)
	arithmeticQEMust(t, err)
	if !isSmallRat(&constant, 3, 1) {
		t.Fatal("constant extraction")
	}
	restHandle, err = f.manager.RefGet(rest)
	arithmeticQEMust(t, err)
	if found, err := contains.Check(restHandle); err != nil || !found {
		t.Fatal("constant rest lost nonconstant x", found, err)
	}
	arithmeticQEMust(t, f.manager.DelRef(&rest))
}

// AUDIT.md qe_01-F5: get_coeff builds rest with one n-ary mk_add over the
// LIFO remainder.
func TestCoefficientRestIsNaryAdd(t *testing.T) {
	f := newArithmeticQEFixture(t)
	family, err := f.manager.ArithmeticFamilyID()
	arithmeticQEMust(t, err)
	a := f.pin(f.manager.MkFreshConst("qe-arith-a", f.integer))
	b := f.pin(f.manager.MkFreshConst("qe-arith-b", f.integer))
	c := f.pin(f.manager.MkFreshConst("qe-arith-c", f.integer))
	twoX := f.pin(f.manager.MkArithmeticMul(f.integerNumeral(t, "2"), f.x))
	polynomial := f.pin(f.manager.MkFamilyApp(family, 6, nil, []ast.ASTHandle{a, b, c, twoX}))
	contains, err := ast.NewContainsApp(f.manager, f.x)
	arithmeticQEMust(t, err)
	defer func() { arithmeticQEMust(t, contains.Close()) }()
	coefficient := unumeral.NewRational()
	rest, matched, err := f.util.Coefficient(contains, polynomial, &coefficient)
	arithmeticQEMust(t, err)
	if !matched || !isSmallRat(&coefficient, 2, 1) {
		t.Fatal("coefficient match", matched)
	}
	defer func() { arithmeticQEMust(t, f.manager.DelRef(&rest)) }()
	restHandle, err := f.manager.RefGet(rest)
	arithmeticQEMust(t, err)
	want := f.pin(f.manager.MkFamilyApp(family, 6, nil, []ast.ASTHandle{c, b, a}))
	if restHandle != want {
		t.Fatal("rest is not (+ c b a)")
	}
}

func TestArithmeticQERelationAndDivisibilityMatching(t *testing.T) {
	f := newArithmeticQEFixture(t)
	zero := f.integerNumeral(t, "0")
	le := f.pin(f.manager.MkArithmeticLE(f.x, f.y))
	polynomial, matched, err := f.util.MatchLE(le)
	arithmeticQEMust(t, err)
	if !matched {
		t.Fatal("<= relation not matched")
	}
	h, err := f.manager.RefGet(polynomial)
	arithmeticQEMust(t, err)
	// is_le_ge_core: p = mk_sub(p, a2); simplify(p) gives x + -1*y.
	if kind, err := f.manager.GetDeclKind(h); err != nil || kind != 6 {
		t.Fatal("<= normalization kind", kind, err)
	}
	arithmeticQEMust(t, f.manager.DelRef(&polynomial))
	gt := f.pin(f.manager.MkArithmeticGT(f.y, zero))
	polynomial, matched, err = f.util.MatchLT(gt)
	arithmeticQEMust(t, err)
	if !matched {
		t.Fatal("> relation not matched")
	}
	h, err = f.manager.RefGet(polynomial)
	arithmeticQEMust(t, err)
	// is_lt: y > 0 gives p = mk_sub(0, y), simplified to -1*y.
	if kind, err := f.manager.GetDeclKind(h); err != nil || kind != 9 {
		t.Fatal("> 0 did not produce the negated term", kind, err)
	}
	arithmeticQEMust(t, f.manager.DelRef(&polynomial))

	divides, err := f.util.MakeDivides(smallRat(7, 1), f.x)
	arithmeticQEMust(t, err)
	dividesHandle, err := f.manager.RefGet(divides)
	arithmeticQEMust(t, err)
	divisor := unumeral.NewRational()
	term, matched, err := f.util.MatchDivides(dividesHandle, &divisor)
	arithmeticQEMust(t, err)
	if !matched || !isSmallRat(&divisor, 7, 1) || term != f.x {
		t.Fatal("divides round trip", matched, term)
	}
	negative, err := f.manager.MkNot(dividesHandle)
	arithmeticQEMust(t, err)
	atom, term, matched, err := f.util.MatchNotDivides(negative, &divisor)
	arithmeticQEMust(t, err)
	if !matched || atom != dividesHandle || !isSmallRat(&divisor, 7, 1) || term != f.x {
		t.Fatal("not-divides round trip")
	}
	arithmeticQEMust(t, f.manager.DelRef(&divides))
}

func TestArithmeticQEBoundsAbsMinMaxAndFlattening(t *testing.T) {
	f := newArithmeticQEFixture(t)
	bound, err := f.util.MakeBound(smallRat(2, 1), f.x, smallRat(3, 1), f.y, false)
	arithmeticQEMust(t, err)
	h, err := f.manager.RefGet(bound)
	arithmeticQEMust(t, err)
	if kind, err := f.manager.GetDeclKind(h); err != nil || kind != 2 {
		t.Fatal("integer bound kind", kind, err)
	}
	arithmeticQEMust(t, f.manager.DelRef(&bound))
	strict, err := f.util.MakeBound(smallRat(1, 1), f.rx, smallRat(1, 1), f.ry, true)
	arithmeticQEMust(t, err)
	h, err = f.manager.RefGet(strict)
	arithmeticQEMust(t, err)
	// mk_strict_bound over reals is mk_lt: not (0 <= e), simplified.
	if not, _, err := f.manager.IsNot(h); err != nil || !not {
		t.Fatal("real strict bound is not mk_lt's negated <=", err)
	}
	arithmeticQEMust(t, f.manager.DelRef(&strict))

	minusThree := f.integerNumeral(t, "-3")
	abs, err := f.util.MakeAbs(minusThree)
	arithmeticQEMust(t, err)
	absHandle, err := f.manager.RefGet(abs)
	arithmeticQEMust(t, err)
	if kind, err := f.manager.GetDeclKind(absHandle); err != nil || kind != 8 {
		t.Fatal("negative numeral abs kind", kind, err)
	}
	arithmeticQEMust(t, f.manager.DelRef(&abs))
	maximum, err := f.util.MakeMax([]ast.ASTHandle{f.x, f.y})
	arithmeticQEMust(t, err)
	maxHandle, err := f.manager.RefGet(maximum)
	arithmeticQEMust(t, err)
	if kind, err := f.manager.GetDeclKind(maxHandle); err != nil || kind != ast.ASTOpIte {
		t.Fatal("max kind", kind, err)
	}
	arithmeticQEMust(t, f.manager.DelRef(&maximum))

	one, err := f.manager.MkArithmeticLE(f.x, f.y)
	arithmeticQEMust(t, err)
	two, err := f.manager.MkArithmeticLE(f.y, f.x)
	arithmeticQEMust(t, err)
	three, err := f.manager.MkEq(f.x, f.y)
	arithmeticQEMust(t, err)
	left, err := f.manager.MkAnd(one, two)
	arithmeticQEMust(t, err)
	flat, err := f.util.MakeFlatAnd(left, three)
	arithmeticQEMust(t, err)
	flatHandle, err := f.manager.RefGet(flat)
	arithmeticQEMust(t, err)
	parts, err := f.manager.FlattenAnd([]ast.ASTHandle{flatHandle})
	arithmeticQEMust(t, err)
	if len(parts) != 3 {
		t.Fatal("flat conjunction part count", len(parts))
	}
	arithmeticQEMust(t, f.manager.DelRef(&flat))

	copyOwner := *f.util
	divisor := unumeral.NewRational()
	if _, _, err = copyOwner.MatchDivides(h, &divisor); err == nil {
		t.Fatal("copied arithmetic QE utility accepted")
	}
	arithmeticQEMust(t, f.util.Close())
	if _, _, err = f.util.MatchLE(h); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed arithmetic QE utility accepted", err)
	}
}

// mk_flat_and tests m.is_and on each argument: a quantifier (qe-sat's
// blast_or leaves an exists when expr_quant_elim is interrupted) is kept
// as one conjunct, not rejected.
func TestArithmeticQEFlatAndKeepsQuantifierConjunct(t *testing.T) {
	f := newArithmeticQEFixture(t)
	le, err := f.manager.MkArithmeticLE(f.x, f.y)
	arithmeticQEMust(t, err)
	name, err := f.manager.Symbols.Intern("flat-and-z")
	arithmeticQEMust(t, err)
	q, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{name}, Body: le})
	arithmeticQEMust(t, err)
	flat, err := f.util.MakeFlatAnd(le, q)
	arithmeticQEMust(t, err)
	h, err := f.manager.RefGet(flat)
	arithmeticQEMust(t, err)
	parts, err := f.manager.FlattenAnd([]ast.ASTHandle{h})
	arithmeticQEMust(t, err)
	if len(parts) != 2 || parts[0] != q && parts[1] != q {
		t.Fatal("quantifier conjunct", parts)
	}
	arithmeticQEMust(t, f.manager.DelRef(&flat))
}
