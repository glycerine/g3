package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type fpa2BVFixture struct {
	m         *ast.ASTManager
	converter *astfpa.FPA2BVConverter
	rewriter  *FPA2BVRewriter
	family    ast.FamilyID
	sort      ast.ASTHandle
	out       ast.ASTRef
	simp      *TheoryRewriter
}

func newFPA2BVFixture(t *testing.T) fpa2BVFixture {
	t.Helper()
	m, c, g, _ := boolRewriterConfigFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	family, err := m.RegisterFloatingPointPlugin()
	astTestMust(t, err)
	sort, err := m.FloatingPointSort(8, 24)
	astTestMust(t, err)
	converter, err := NewFPA2BVConverter(m, c, g)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, converter.Close()) })
	rw, err := NewFPA2BVRewriter(m, converter)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&out)) })
	simp, err := NewTheoryRewriter(m, c, g, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simp.Close()) })
	return fpa2BVFixture{m: m, converter: converter, rewriter: rw, family: family, sort: sort, out: out, simp: simp}
}

func (f fpa2BVFixture) rewrite(t *testing.T, value ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	astTestMust(t, f.rewriter.Rewrite(value, f.out))
	result, err := f.m.RefGet(f.out)
	astTestMust(t, err)
	return result
}

func TestFPA2BVRewriterSpecialsComparisonsAndSMTEquality(t *testing.T) {
	f := newFPA2BVFixture(t)
	positive, err := f.m.MkFPAZeroOfSort(f.sort, false)
	astTestMust(t, err)
	negative, err := f.m.MkFPAZeroOfSort(f.sort, true)
	astTestMust(t, err)

	converted := f.rewrite(t, positive)
	if kind, e := f.m.GetDeclKind(converted); e != nil || kind != ast.FPAOpFP {
		t.Fatal("special value was not lowered to FP components", kind, e)
	}

	fpEquality, err := f.m.MkFamilyApp(f.family, ast.FPAOpEq, nil, []ast.ASTHandle{positive, negative})
	astTestMust(t, err)
	if result := f.rewrite(t, fpEquality); !f.m.IsTrue(result) {
		t.Fatal("IEEE fp.eq must identify signed zeros", result)
	}

	smtEquality, err := f.m.MkEq(positive, negative)
	astTestMust(t, err)
	if result := f.rewrite(t, smtEquality); !f.m.IsFalse(result) {
		t.Fatal("SMT equality must distinguish signed zeros", result)
	}

	toIEEE, err := f.m.MkFamilyApp(f.family, ast.FPAOpToIEEEBV, nil, []ast.ASTHandle{negative})
	astTestMust(t, err)
	bits := f.rewrite(t, toIEEE)
	bitsSort, err := f.m.GetSort(bits)
	astTestMust(t, err)
	if width, e := f.m.BitVectorSize(bitsSort); e != nil || width != 32 {
		t.Fatal("IEEE conversion width", width, e)
	}
}

func TestFPA2BVRewriterQuantifiedVariablesAndUninterpretedFunctions(t *testing.T) {
	f := newFPA2BVFixture(t)
	variable, err := f.m.MkVar(0, f.sort)
	astTestMust(t, err)
	isZero, err := f.m.MkFamilyApp(f.family, int32(ast.FPAIsZero), nil, []ast.ASTHandle{variable})
	astTestMust(t, err)
	name := astTestSymbol(t, f.m, "x")
	quantifier, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{f.sort},
		Names:  []usymbol.Symbol{name},
		Body:   isZero,
	})
	astTestMust(t, err)
	rewritten := f.rewrite(t, quantifier)
	boundSort, err := f.m.GetDeclarationSort(rewritten, 0)
	astTestMust(t, err)
	if width, e := f.m.BitVectorSize(boundSort); e != nil || width != 32 {
		t.Fatal("quantified FP variable was not packed", width, e)
	}
	boundName, err := f.m.GetDeclarationName(rewritten, 0)
	astTestMust(t, err)
	text, err := f.m.Symbols.Display(boundName)
	astTestMust(t, err)
	if text != "x.bv" {
		t.Fatal("converted binder name", text)
	}

	functionName := astTestSymbol(t, f.m, "uf")
	declaration, err := f.m.MkFuncDecl(functionName, []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	zero, err := f.m.MkFPAZeroOfSort(f.sort, false)
	astTestMust(t, err)
	application, err := f.m.MkApp(declaration, []ast.ASTHandle{zero})
	astTestMust(t, err)
	convertedUF := f.rewrite(t, application)
	if kind, e := f.m.GetDeclKind(convertedUF); e != nil || kind != ast.FPAOpFP {
		t.Fatal("FPA-valued UF was not lowered", kind, e)
	}
	if len(f.extra(t)) != 1 {
		t.Fatal("UF connection assertion", len(f.extra(t)))
	}
}

func TestFPA2BVRewriterArithmeticDispatch(t *testing.T) {
	f := newFPA2BVFixture(t)
	rounding, err := f.m.MkFamilyApp(f.family, ast.FPAOpRMNearestEven, nil, nil)
	astTestMust(t, err)
	positive, err := f.m.MkFPAInfinityOfSort(f.sort, false)
	astTestMust(t, err)
	negative, err := f.m.MkFPAInfinityOfSort(f.sort, true)
	astTestMust(t, err)
	addition, err := f.m.MkFamilyApp(f.family, ast.FPAOpAdd, nil, []ast.ASTHandle{rounding, positive, negative})
	astTestMust(t, err)
	got := f.rewrite(t, addition)
	want, err := f.special(t, func() (ast.ASTHandle, error) { return f.m.MkFPANaNOfSort(f.sort) })
	astTestMust(t, err)
	if got != want {
		t.Fatal("opposite infinities did not lower to NaN", got, want)
	}
	zero, err := f.m.MkFPAZeroOfSort(f.sort, false)
	astTestMust(t, err)
	multiplication, err := f.m.MkFamilyApp(f.family, ast.FPAOpMul, nil, []ast.ASTHandle{rounding, zero, positive})
	astTestMust(t, err)
	if got = f.rewrite(t, multiplication); got != want {
		t.Fatal("zero times infinity did not lower to NaN", got, want)
	}
	division, err := f.m.MkFamilyApp(f.family, ast.FPAOpDiv, nil, []ast.ASTHandle{rounding, zero, zero})
	astTestMust(t, err)
	if got = f.rewrite(t, division); got != want {
		t.Fatal("zero divided by zero did not lower to NaN", got, want)
	}
	remainder, err := f.m.MkFamilyApp(f.family, ast.FPAOpRem, nil, []ast.ASTHandle{positive, negative})
	astTestMust(t, err)
	if got = f.rewrite(t, remainder); got != want {
		t.Fatal("infinity remainder did not lower to NaN", got, want)
	}
	negativeZero, err := f.m.MkFPAZeroOfSort(f.sort, true)
	astTestMust(t, err)
	integral, err := f.m.MkFamilyApp(f.family, ast.FPAOpRoundToIntegral, nil, []ast.ASTHandle{rounding, negativeZero})
	astTestMust(t, err)
	wantNegativeZero, err := f.special(t, func() (ast.ASTHandle, error) { return f.m.MkFPAZeroOfSort(f.sort, true) })
	astTestMust(t, err)
	if got = f.rewrite(t, integral); got != wantNegativeZero {
		t.Fatal("round-to-integral did not preserve negative zero", got, wantNegativeZero)
	}
	squareRoot, err := f.m.MkFamilyApp(f.family, ast.FPAOpSqrt, nil, []ast.ASTHandle{rounding, negative})
	astTestMust(t, err)
	got = f.rewrite(t, squareRoot)
	isNaN, err := f.isNaN(t, got)
	astTestMust(t, err)
	if !f.m.IsTrue(isNaN) {
		t.Fatal("negative-infinity square root did not lower to NaN", got, isNaN)
	}
	fused, err := f.m.MkFamilyApp(f.family, ast.FPAOpFMA, nil, []ast.ASTHandle{rounding, positive, positive, negative})
	astTestMust(t, err)
	got = f.rewrite(t, fused)
	isNaN, err = f.isNaN(t, got)
	astTestMust(t, err)
	if !f.m.IsTrue(isNaN) {
		t.Fatal("opposed infinite FMA did not lower to NaN", got, isNaN)
	}

	left := astTestSymbol(t, f.m, "add-x")
	right := astTestSymbol(t, f.m, "add-y")
	leftValue, err := f.m.MkConst(left, f.sort)
	astTestMust(t, err)
	rightValue, err := f.m.MkConst(right, f.sort)
	astTestMust(t, err)
	addition, err = f.m.MkFamilyApp(f.family, ast.FPAOpAdd, nil, []ast.ASTHandle{rounding, leftValue, rightValue})
	astTestMust(t, err)
	got = f.rewrite(t, addition)
	if kind, e := f.m.GetDeclKind(got); e != nil || kind != ast.FPAOpFP {
		t.Fatal("symbolic addition did not lower to FP components", kind, e)
	}
}

func TestFPA2BVRewriterToFPFormatDispatch(t *testing.T) {
	f := newFPA2BVFixture(t)
	targetSort, err := f.m.FloatingPointSort(5, 11)
	astTestMust(t, err)
	rounding, err := f.m.MkFamilyApp(f.family, ast.FPAOpRMNearestEven, nil, nil)
	astTestMust(t, err)
	ebits, err := f.m.NewParameterInt(5)
	astTestMust(t, err)
	sbits, err := f.m.NewParameterInt(11)
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, f.m.DelParameter(&sbits))
		astTestMust(t, f.m.DelParameter(&ebits))
	})
	rmSort, err := f.m.GetSort(rounding)
	astTestMust(t, err)
	declaration, err := f.m.MkFamilyFuncDecl(
		f.family, ast.FPAOpToFP,
		[]ast.ASTParameter{ebits, sbits},
		[]ast.ASTHandle{rmSort, f.sort}, ast.ASTHandle{},
	)
	astTestMust(t, err)
	positiveInfinity, err := f.m.MkFPAInfinityOfSort(f.sort, false)
	astTestMust(t, err)
	conversion, err := f.m.MkApp(declaration, []ast.ASTHandle{rounding, positiveInfinity})
	astTestMust(t, err)
	got := f.rewrite(t, conversion)
	want, err := f.special(t, func() (ast.ASTHandle, error) { return f.m.MkFPAInfinityOfSort(targetSort, false) })
	astTestMust(t, err)
	if got != want {
		t.Fatal("format conversion dispatch", got, want)
	}
	bvSort, err := f.m.BitVectorSort(32)
	astTestMust(t, err)
	zero, err := f.m.BitVectorNumeralUnsigned(0, 32)
	astTestMust(t, err)
	for _, kind := range []int32{ast.FPAOpToFP, ast.FPAOpToFPUnsigned} {
		integerDeclaration, e := f.m.MkFamilyFuncDecl(
			f.family, kind,
			[]ast.ASTParameter{ebits, sbits},
			[]ast.ASTHandle{rmSort, bvSort}, ast.ASTHandle{},
		)
		astTestMust(t, e)
		integerConversion, e := f.m.MkApp(integerDeclaration, []ast.ASTHandle{rounding, zero})
		astTestMust(t, e)
		// mk_to_fp_signed/unsigned build raw m.mk_eq/mk_ite: constant
		// inputs fold only under the simplifier that follows fpa2bv.
		got = f.simplified(t, f.rewrite(t, integerConversion))
		want, e = f.special(t, func() (ast.ASTHandle, error) { return f.m.MkFPAZeroOfSort(targetSort, false) })
		astTestMust(t, e)
		if want = f.simplified(t, want); got != want {
			t.Fatal("integer conversion dispatch", kind, got, want)
		}
	}
}

func TestFPA2BVRewriterToBVDispatch(t *testing.T) {
	f := newFPA2BVFixture(t)
	rounding, err := f.m.MkFamilyApp(f.family, ast.FPAOpRMNearestEven, nil, nil)
	astTestMust(t, err)
	rmSort, err := f.m.GetSort(rounding)
	astTestMust(t, err)
	width, err := f.m.NewParameterInt(13)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelParameter(&width)) })
	zero, err := f.m.MkFPAZeroOfSort(f.sort, true)
	astTestMust(t, err)
	want, err := f.m.BitVectorNumeralUnsigned(0, 13)
	astTestMust(t, err)
	for _, kind := range []int32{ast.FPAOpToUBV, ast.FPAOpToSBV} {
		declaration, e := f.m.MkFamilyFuncDecl(
			f.family, kind, []ast.ASTParameter{width},
			[]ast.ASTHandle{rmSort, f.sort}, ast.ASTHandle{},
		)
		astTestMust(t, e)
		conversion, e := f.m.MkApp(declaration, []ast.ASTHandle{rounding, zero})
		astTestMust(t, e)
		if got := f.simplified(t, f.rewrite(t, conversion)); got != want {
			t.Fatal("float-to-BV dispatch", kind, got, want)
		}
	}
}

func TestFPA2BVRewriterToRealDispatch(t *testing.T) {
	f := newFPA2BVFixture(t)
	zero, err := f.m.MkFPAZeroOfSort(f.sort, true)
	astTestMust(t, err)
	conversion, err := f.m.MkFamilyApp(f.family, ast.FPAOpToReal, nil, []ast.ASTHandle{zero})
	astTestMust(t, err)
	got := f.simplified(t, f.rewrite(t, conversion))
	text, integer, matched, err := f.m.ArithmeticNumeralText(got)
	astTestMust(t, err)
	if !matched || integer || text != "0" {
		t.Fatal("float-to-real dispatch", text, integer, matched)
	}
}

func TestFPA2BVRewriterFromRealDispatch(t *testing.T) {
	f := newFPA2BVFixture(t)
	rounding, err := f.m.MkFamilyApp(f.family, ast.FPAOpRMNearestEven, nil, nil)
	astTestMust(t, err)
	rmSort, err := f.m.GetSort(rounding)
	astTestMust(t, err)
	realValue, err := f.m.ArithmeticRealNumeralText("3/2")
	astTestMust(t, err)
	realSort, err := f.m.GetSort(realValue)
	astTestMust(t, err)
	ebits, err := f.m.NewParameterInt(8)
	astTestMust(t, err)
	sbits, err := f.m.NewParameterInt(24)
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, f.m.DelParameter(&sbits))
		astTestMust(t, f.m.DelParameter(&ebits))
	})
	declaration, err := f.m.MkFamilyFuncDecl(
		f.family, ast.FPAOpToFP, []ast.ASTParameter{ebits, sbits},
		[]ast.ASTHandle{rmSort, realSort}, ast.ASTHandle{},
	)
	astTestMust(t, err)
	conversion, err := f.m.MkApp(declaration, []ast.ASTHandle{rounding, realValue})
	astTestMust(t, err)
	got := f.rewrite(t, conversion)
	if kind, e := f.m.GetDeclKind(got); e != nil || kind != ast.FPAOpFP {
		t.Fatal("real numeral conversion dispatch", kind, e)
	}

	symbolic, err := f.m.MkFreshConst("fpa2bv_real", realSort)
	astTestMust(t, err)
	conversion, err = f.m.MkApp(declaration, []ast.ASTHandle{rounding, symbolic})
	astTestMust(t, err)
	before := len(f.extra(t))
	got = f.rewrite(t, conversion)
	if kind, e := f.m.GetDeclKind(got); e != nil || kind != ast.FPAOpFP {
		t.Fatal("symbolic real conversion dispatch", kind, e)
	}
	if len(f.extra(t)) != before+1 {
		t.Fatal("symbolic real conversion assertion", before, len(f.extra(t)))
	}

	integerValue, err := f.m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	integerSort, err := f.m.GetSort(integerValue)
	astTestMust(t, err)
	pairDeclaration, err := f.m.MkFamilyFuncDecl(
		f.family, ast.FPAOpToFP, []ast.ASTParameter{ebits, sbits},
		[]ast.ASTHandle{rmSort, realSort, integerSort}, ast.ASTHandle{},
	)
	astTestMust(t, err)
	pair, err := f.m.MkApp(pairDeclaration, []ast.ASTHandle{rounding, realValue, integerValue})
	astTestMust(t, err)
	got = f.rewrite(t, pair)
	if kind, e := f.m.GetDeclKind(got); e != nil || kind != ast.FPAOpFP {
		t.Fatal("real/integer conversion dispatch", kind, e)
	}
}

func TestFPA2BVRewriterLowUnspecifiedDispatch(t *testing.T) {
	f := newFPA2BVFixture(t)
	astTestMust(t, f.rewriter.SetHighUnspecified(false))
	nan, err := f.m.MkFPANaNOfSort(f.sort)
	astTestMust(t, err)

	toIEEE, err := f.m.MkFamilyApp(f.family, ast.FPAOpToIEEEBV, nil, []ast.ASTHandle{nan})
	astTestMust(t, err)
	before := len(f.extra(t))
	bits := f.rewrite(t, toIEEE)
	if width, e := f.m.BitVectorSize(bits); e != nil || width != 32 {
		t.Fatal("low-unspecified IEEE result", width, e)
	}
	if len(f.extra(t)) != before+2 {
		t.Fatal("low-unspecified NaN constraints", before, len(f.extra(t)))
	}

	rounding, err := f.m.MkFamilyApp(f.family, ast.FPAOpRMNearestEven, nil, nil)
	astTestMust(t, err)
	rmSort, err := f.m.GetSort(rounding)
	astTestMust(t, err)
	widthParameter, err := f.m.NewParameterInt(13)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelParameter(&widthParameter)) })
	toBVDecl, err := f.m.MkFamilyFuncDecl(
		f.family, ast.FPAOpToUBV, []ast.ASTParameter{widthParameter},
		[]ast.ASTHandle{rmSort, f.sort}, ast.ASTHandle{},
	)
	astTestMust(t, err)
	toBV, err := f.m.MkApp(toBVDecl, []ast.ASTHandle{rounding, nan})
	astTestMust(t, err)
	convertedBV := f.rewrite(t, toBV)
	if width, e := f.m.BitVectorSize(convertedBV); e != nil || width != 13 {
		t.Fatal("low-unspecified float-to-BV result", width, e)
	}

	toReal, err := f.m.MkFamilyApp(f.family, ast.FPAOpToReal, nil, []ast.ASTHandle{nan})
	astTestMust(t, err)
	convertedReal := f.rewrite(t, toReal)
	zero, err := f.m.ArithmeticRealNumeralText("0")
	astTestMust(t, err)
	gotSort, err := f.m.GetSort(convertedReal)
	astTestMust(t, err)
	wantSort, err := f.m.GetSort(zero)
	astTestMust(t, err)
	if gotSort != wantSort || convertedReal == zero {
		t.Fatal("low-unspecified float-to-real result", gotSort, wantSort, convertedReal, zero)
	}
}

// special is the converted special value: the rewrite of its literal.
func (f fpa2BVFixture) special(t *testing.T, literal func() (ast.ASTHandle, error)) (ast.ASTHandle, error) {
	t.Helper()
	h, err := literal()
	if err != nil {
		return h, err
	}
	return f.rewrite(t, h), nil
}

// isNaN rewrites fp.isNaN(x) for an already converted x.
func (f fpa2BVFixture) isNaN(t *testing.T, x ast.ASTHandle) (ast.ASTHandle, error) {
	t.Helper()
	app, err := f.m.MkFamilyApp(f.family, int32(ast.FPAIsNaN), nil, []ast.ASTHandle{x})
	if err != nil {
		return app, err
	}
	return f.rewrite(t, app), nil
}

func (f fpa2BVFixture) extra(t *testing.T) []ast.ASTHandle {
	t.Helper()
	out, err := f.converter.ExtraAssertions()
	astTestMust(t, err)
	return out
}

// simplified is th_rewriter applied to a converted term.
func (f fpa2BVFixture) simplified(t *testing.T, h ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	out, err := f.m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelRef(&out)) })
	astTestMust(t, f.simp.Rewrite(h, out))
	r, err := f.m.RefGet(out)
	astTestMust(t, err)
	return r
}
