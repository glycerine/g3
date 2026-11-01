package ast

// Source: src/ast/expr2polynomial.cpp:25-518; src/ast/expr2polynomial.h:29-110.

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestExpr2PolynomialValueConstructionReleasesFreshPolynomial(t *testing.T) {
	for _, stop := range []int64{0, 1} {
		for _, borrowed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/borrowed=%v", stop, borrowed), func(t *testing.T) {
				m, pm := exprPolynomialFixture(t)
				_, err := m.RegisterArithmeticPlugin()
				astTestMust(t, err)
				converter, err := NewExpr2Polynomial(Expr2PolynomialConfig{AST: m, Polynomial: pm})
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, converter.Close()) })
				core, err := pm.ArithmeticCore()
				astTestMust(t, err)
				variable, err := core.MkVar()
				astTestMust(t, err)
				p, err := core.MkVariable(variable, 1)
				astTestMust(t, err)
				info, err := core.Info(p)
				astTestMust(t, err)
				if info.RefCount != 0 {
					t.Fatal("fresh polynomial already has an owner", info.RefCount)
				}
				if borrowed {
					astTestMust(t, core.IncRef(p))
					t.Cleanup(func() { astTestMust(t, core.DecRef(p)) })
				}
				m.Budget().FailAfter(stop)
				value, err := converter.newValue(p, nil)
				m.Budget().FailAfter(-1)
				if value != nil || !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal("expected injected construction failure", value, err)
				}
				info, err = core.Info(p)
				if borrowed {
					astTestMust(t, err)
					if info.RefCount != 1 {
						t.Fatal("borrowed polynomial reference changed", info.RefCount)
					}
				} else if err == nil {
					t.Fatal("fresh polynomial was not released")
				}
			})
		}
	}
}

func exprPolynomialFixture(t *testing.T) (*ASTManager, *polynomial.MultivariatePolynomialManager) {
	t.Helper()
	sy, q, budget := astTestOwners(t)
	m, err := NewASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: budget})
	astTestMust(t, err)
	allocator, err := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: q.Manager.Integers().RuntimeOwner()})
	astTestMust(t, err)
	pm, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core: polynomial.MultivariatePolynomialConfig{
			Integer: q.Manager.Integers(), Rationals: q.Manager,
			Checkpoint: func() bool { return true }, Cooperation: func() error { return nil },
		},
		Allocator: allocator,
	})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, m.Close())
		astTestMust(t, pm.CloseChecked())
		astTestMust(t, allocator.Close())
	})
	return m, pm
}

func exprPolynomialNumeral(t *testing.T, m *ASTManager, text string, integer bool) ASTHandle {
	t.Helper()
	v, err := m.numerals.NewValue()
	astTestMust(t, err)
	astTestMust(t, m.numerals.SetString(&v, text))
	h, err := m.ArithmeticNumeral(&v, integer)
	astTestMust(t, err)
	astTestMust(t, m.numerals.Del(&v))
	astTestPin(t, m, h)
	return h
}

func TestExpr2PolynomialForwardRationalArithmeticCacheAndOwnership(t *testing.T) {
	m, pm := exprPolynomialFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort := m.external(m.arithmeticPlugin.fields[0])
	xh, err := m.MkConst(astTestSymbol(t, m, "expr-poly-x"), realSort)
	astTestMust(t, err)
	astTestPin(t, m, xh)
	half := exprPolynomialNumeral(t, m, "1/2", false)
	plus, err := m.MkFamilyApp(family, 6, nil, []ASTHandle{xh, half})
	astTestMust(t, err)
	astTestPin(t, m, plus)
	minus, err := m.MkFamilyApp(family, 7, nil, []ASTHandle{xh, half})
	astTestMust(t, err)
	astTestPin(t, m, minus)
	root, err := m.MkFamilyApp(family, 9, nil, []ASTHandle{plus, minus})
	astTestMust(t, err)
	astTestPin(t, m, root)
	// A second source reference enables the persistent cache branch.
	extra, err := m.NewRef(root)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&extra)) }()

	converter, err := NewExpr2Polynomial(Expr2PolynomialConfig{AST: m, Polynomial: pm})
	astTestMust(t, err)
	defer func() { astTestMust(t, converter.Close()) }()
	pref, denominator, matched, err := converter.ToPolynomial(root)
	astTestMust(t, err)
	if !matched {
		t.Fatal("real arithmetic expression rejected")
	}
	defer func() { astTestMust(t, pref.Close()); astTestMust(t, denominator.Close()) }()
	if text, e := denominator.Display(); e != nil || text != "4" {
		t.Fatal("source common denominator", text, e)
	}
	p, err := pref.Get()
	astTestMust(t, err)
	core, err := pm.ArithmeticCore()
	astTestMust(t, err)
	info, err := core.Info(p)
	astTestMust(t, err)
	monomials, err := core.MonomialManager()
	astTestMust(t, err)
	integers, err := core.IntegerManager()
	astTestMust(t, err)
	terms := map[uint32]string{}
	for i := uint32(0); i < info.Size; i++ {
		coefficient, e := core.Coefficient(p, i)
		astTestMust(t, e)
		text, e := integers.String(&coefficient)
		astTestMust(t, e)
		monomial, e := core.TermMonomial(p, i)
		astTestMust(t, e)
		mi, e := monomials.Info(monomial)
		astTestMust(t, e)
		degree := uint32(0)
		if mi.Size != 0 {
			power, e := monomials.ReadPower(monomial, 0)
			astTestMust(t, e)
			degree = power.Degree
		}
		terms[degree] = text
	}
	if len(terms) != 2 || terms[0] != "-1" || terms[2] != "4" {
		t.Fatal("source normalized numerator", terms)
	}
	reconstructed, err := converter.ToExpr(p, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&reconstructed)) }()
	reconstructedHandle, err := m.RefGet(reconstructed)
	astTestMust(t, err)
	round, roundDenominator, matched, err := converter.ToPolynomial(reconstructedHandle)
	astTestMust(t, err)
	if !matched {
		t.Fatal("reconstructed polynomial refused")
	}
	defer func() { astTestMust(t, round.Close()); astTestMust(t, roundDenominator.Close()) }()
	roundPolynomial, err := round.Get()
	astTestMust(t, err)
	equal, err := core.Equal(p, roundPolynomial)
	astTestMust(t, err)
	if text, e := roundDenominator.Display(); e != nil || text != "1" || !equal {
		t.Fatal("source polynomial-to-expression round trip", text, equal, e)
	}
	if len(converter.cached) == 0 {
		t.Fatal("shared expression cache not populated")
	}
	yes, err := converter.IsVar(xh)
	astTestMust(t, err)
	if !yes {
		t.Fatal("variable mapping not published")
	}
	astTestMust(t, converter.ResetCache())
	if len(converter.cached) != 0 || len(converter.cache) != 0 {
		t.Fatal("cache reset", len(converter.cached), len(converter.cache))
	}
}

func TestExpr2PolynomialForwardRefusalCancelAndVarIndexMode(t *testing.T) {
	m, pm := exprPolynomialFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	converter, err := NewExpr2Polynomial(Expr2PolynomialConfig{AST: m, Polynomial: pm})
	astTestMust(t, err)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	boolean, err := m.MkConst(astTestSymbol(t, m, "expr-poly-b"), boolSort)
	astTestMust(t, err)
	astTestPin(t, m, boolean)
	if p, d, matched, e := converter.ToPolynomial(boolean); e != nil || matched || p != nil || d != nil {
		t.Fatal("non-arithmetic refusal", p, d, matched, e)
	}
	astTestMust(t, converter.SetCancel(true))
	realSort := m.external(m.arithmeticPlugin.fields[0])
	xh, err := m.MkConst(astTestSymbol(t, m, "expr-poly-cancel"), realSort)
	astTestMust(t, err)
	add, err := m.MkFamilyApp(m.arithmeticPlugin.family, 6, nil, []ASTHandle{xh, xh})
	astTestMust(t, err)
	if _, _, _, e := converter.ToPolynomial(add); !errors.Is(e, ErrExprPolynomialCanceled) {
		t.Fatal("source cancel checkpoint", e)
	}
	astTestMust(t, converter.Close())

	indexed, err := NewExpr2Polynomial(Expr2PolynomialConfig{AST: m, Polynomial: pm, UseVarIndices: true})
	astTestMust(t, err)
	defer func() { astTestMust(t, indexed.Close()) }()
	integerSort := m.external(m.arithmeticPlugin.fields[1])
	variable, err := m.MkVar(3, integerSort)
	astTestMust(t, err)
	if _, _, _, e := indexed.ToPolynomial(variable); e != ErrExprPolynomialIntegerVariable {
		t.Fatal("source integer free-variable refusal", e)
	}
	realVariable, err := m.MkVar(3, realSort)
	astTestMust(t, err)
	indexedPolynomial, indexedDenominator, matched, err := indexed.ToPolynomial(realVariable)
	astTestMust(t, err)
	if !matched {
		t.Fatal("real free variable refused")
	}
	indexedValue, err := indexedPolynomial.Get()
	astTestMust(t, err)
	indexedExpr, err := indexed.ToExpr(indexedValue, true)
	astTestMust(t, err)
	indexedHandle, err := m.RefGet(indexedExpr)
	astTestMust(t, err)
	if kind, e := m.GetKind(indexedHandle); e != nil || kind != ASTVar {
		t.Fatal("indexed reverse variable", kind, e)
	}
	astTestMust(t, m.DelRef(&indexedExpr))
	astTestMust(t, indexedPolynomial.Close())
	astTestMust(t, indexedDenominator.Close())
	copyOwner := *indexed
	if _, _, _, e := copyOwner.ToPolynomial(variable); e != ErrASTHandle {
		t.Fatal("copied owner guard", e)
	}
}

func TestExpr2PolynomialSourceOperatorBranchesAndAtomicInvalidPower(t *testing.T) {
	m, pm := exprPolynomialFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integerSort := m.external(m.arithmeticPlugin.fields[1])
	xh, err := m.MkConst(astTestSymbol(t, m, "expr-poly-int-x"), integerSort)
	astTestMust(t, err)
	astTestPin(t, m, xh)
	two := exprPolynomialNumeral(t, m, "2", true)
	power, err := m.MkFamilyApp(family, 18, nil, []ASTHandle{xh, two})
	astTestMust(t, err)
	negative, err := m.MkFamilyApp(family, 8, nil, []ASTHandle{xh})
	astTestMust(t, err)
	sum, err := m.MkFamilyApp(family, 6, nil, []ASTHandle{power, negative})
	astTestMust(t, err)
	converter, err := NewExpr2Polynomial(Expr2PolynomialConfig{AST: m, Polynomial: pm})
	astTestMust(t, err)
	defer func() { astTestMust(t, converter.Close()) }()
	p, d, matched, err := converter.ToPolynomial(sum)
	astTestMust(t, err)
	if !matched {
		t.Fatal("integer power/negation refused")
	}
	value, err := p.Get()
	astTestMust(t, err)
	repeated, err := converter.ToExpr(value, false)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&repeated))
	astTestMust(t, p.Close())
	astTestMust(t, d.Close())

	half := exprPolynomialNumeral(t, m, "1/2", false)
	realX, err := m.MkFamilyApp(family, 14, nil, []ASTHandle{xh})
	astTestMust(t, err)
	invalidPower, err := m.MkFamilyApp(family, 18, nil, []ASTHandle{realX, half})
	astTestMust(t, err)
	p, d, matched, err = converter.ToPolynomial(invalidPower)
	astTestMust(t, err)
	if !matched {
		t.Fatal("invalid power should become an atomic variable")
	}
	atomic, err := converter.IsVar(invalidPower)
	astTestMust(t, err)
	baseMapped, err := converter.IsVar(realX)
	astTestMust(t, err)
	if !atomic || baseMapped {
		t.Fatal("source invalid-power side-effect order", atomic, baseMapped)
	}
	astTestMust(t, p.Close())
	astTestMust(t, d.Close())
}

func TestExpr2PolynomialSourceFrameTraceOrder(t *testing.T) {
	c, globals, _, budget := dynAckSourceFixture(t)
	previous, err := uparams.BindProcessGlobalParamsService(globals)
	astTestMust(t, err)
	t.Cleanup(func() { _, e := uparams.BindProcessGlobalParamsService(previous); astTestMust(t, e) })
	allocator, err := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: c.NumeralsField.Manager.Integers().RuntimeOwner()})
	astTestMust(t, err)
	pm, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core:      polynomial.MultivariatePolynomialConfig{Integer: c.NumeralsField.Manager.Integers(), Rationals: c.NumeralsField.Manager, Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }},
		Allocator: allocator,
	})
	astTestMust(t, err)
	defer func() { astTestMust(t, pm.CloseChecked()); astTestMust(t, allocator.Close()) }()
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	m, err := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: budget, TraceTags: tags})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort := m.external(m.arithmeticPlugin.fields[0])
	xh, err := m.MkConst(astTestSymbol(t, m, "expr-poly-trace-x"), realSort)
	astTestMust(t, err)
	numeral, err := m.numerals.NewValue()
	astTestMust(t, err)
	astTestMust(t, m.numerals.SetString(&numeral, "1"))
	one, err := m.ArithmeticNumeral(&numeral, false)
	astTestMust(t, err)
	astTestMust(t, m.numerals.Del(&numeral))
	inner, err := m.MkFamilyApp(family, 6, nil, []ASTHandle{xh, one})
	astTestMust(t, err)
	outer, err := m.MkFamilyApp(family, 9, nil, []ASTHandle{inner, inner})
	astTestMust(t, err)
	converter, err := NewExpr2Polynomial(Expr2PolynomialConfig{AST: m, Polynomial: pm})
	astTestMust(t, err)
	defer func() { astTestMust(t, converter.Close()) }()
	if p, d, _, e := converter.ToPolynomial(outer); e != nil {
		t.Fatal(e)
	} else {
		astTestMust(t, p.Close())
		astTestMust(t, d.Close())
	}
	if output.Len() != 0 {
		t.Fatal("disabled source trace evaluated", output.String())
	}
	astTestMust(t, converter.ResetCache())
	tags.Enable("expr2polynomial")
	if p, d, _, e := converter.ToPolynomial(outer); e != nil {
		t.Fatal(e)
	} else {
		astTestMust(t, p.Close())
		astTestMust(t, d.Close())
	}
	if !tags.ShouldTrace("expr2polynomial") {
		if output.Len() != 0 {
			t.Fatal("erased source trace wrote", output.String())
		}
		return
	}
	got := output.String()
	header := "-------- [expr2polynomial] to_polynomial src/ast/expr2polynomial.cpp:372 ---------\n"
	footer := "------------------------------------------------\n"
	want := header + "processing: 0\n(* (+ expr-poly-trace-x 1.0) (+ expr-poly-trace-x 1.0))\n" + footer +
		header + "processing: 0\n(+ expr-poly-trace-x 1.0)\n" + footer +
		header + "processing: 1\n(* (+ expr-poly-trace-x 1.0) (+ expr-poly-trace-x 1.0))\n" + footer
	if got != want || strings.Count(got, header) != 3 {
		t.Fatalf("source frame trace order: got %q, want %q", got, want)
	}
}
