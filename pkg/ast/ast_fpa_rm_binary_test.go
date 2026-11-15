package ast

import (
	"strings"
	"testing"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestASTFPARMBinaryDeclarationsAndNumericCleanup(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	family, err := m.RegisterFloatingPointPlugin()
	astTestMust(t, err)
	rmSort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	floatSort, err := m.MkFamilySort(family, 3, nil)
	astTestMust(t, err)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	domain := []ASTHandle{rmSort, floatSort, floatSort}
	for _, tc := range []struct {
		kind int32
		name string
	}{{FPAOpAdd, "fp.add"}, {FPAOpSub, "fp.sub"}, {FPAOpMul, "fp.mul"}, {FPAOpDiv, "fp.div"}} {
		decl, e := m.MkFamilyFuncDecl(family, tc.kind, nil, domain, ASTHandle{})
		astTestMust(t, e)
		r, e := m.internal(decl)
		astTestMust(t, e)
		name, e := m.Symbols.String(m.declAt(r.slot).name)
		astTestMust(t, e)
		floatRef, e := m.internal(floatSort)
		astTestMust(t, e)
		if name != tc.name || m.declAt(r.slot).arity != 3 || m.declAt(r.slot).rangeSort != floatRef {
			t.Fatal("rounding binary declaration", tc.kind, name)
		}
		for _, invalid := range [][]ASTHandle{nil, {rmSort, floatSort}, {rmSort, floatSort, floatSort, floatSort}, {boolSort, floatSort, floatSort}, {rmSort, boolSort, boolSort}, {rmSort, floatSort, boolSort}} {
			_, e := m.MkFamilyFuncDecl(family, tc.kind, nil, invalid, ASTHandle{})
			if e == nil || !strings.Contains(e.Error(), "floating point") && !strings.Contains(e.Error(), "RoundingMode") && !strings.Contains(e.Error(), "FloatingPoint") {
				t.Fatal("invalid rounding binary declaration", tc.kind, invalid, e)
			}
		}
	}
	makeValue := func(n float64) ASTHandle {
		var value unumeral.ArbitraryFloat
		astTestMust(t, m.floatingPointPlugin.numeric.SetDouble(&value, 8, 24, n))
		ref, e := m.floatingPointPlugin.mkNumeral(&value, nil)
		astTestMust(t, e)
		astTestMust(t, m.floatingPointPlugin.numeric.Del(&value))
		return m.external(ref)
	}
	one, two, three, six := makeValue(1), makeValue(2), makeValue(3), makeValue(6)
	rm, err := m.MkFamilyApp(family, astFPARMNearestEven, nil, nil)
	astTestMust(t, err)
	for _, tc := range []struct {
		kind       int32
		x, y, want ASTHandle
	}{{FPAOpAdd, one, two, three}, {FPAOpMul, two, three, six}, {FPAOpDiv, six, two, three}} {
		app, e := m.MkFamilyApp(family, tc.kind, nil, []ASTHandle{rm, tc.x, tc.y})
		astTestMust(t, e)
		gotSort, e := m.GetSort(app)
		astTestMust(t, e)
		if gotSort != floatSort {
			t.Fatal("rounding binary application sort", tc.kind)
		}
		out, found, e := m.RewriteFPARMBinaryNumerals(rm, tc.x, tc.y, tc.kind)
		astTestMust(t, e)
		if !found || out != tc.want || m.floatingPointPlugin.numeralProbeFrames != nil {
			t.Fatal("numeric rounding binary or cleanup", tc.kind, found, out)
		}
	}
	variableFloat, err := m.MkVar(0, floatSort)
	astTestMust(t, err)
	variableRM, err := m.MkVar(0, rmSort)
	astTestMust(t, err)
	for _, kind := range []int32{FPAOpAdd, FPAOpMul, FPAOpDiv} {
		for _, tc := range [][3]ASTHandle{{variableRM, one, two}, {rm, variableFloat, two}, {rm, one, variableFloat}} {
			out, found, e := m.RewriteFPARMBinaryNumerals(tc[0], tc[1], tc[2], kind)
			astTestMust(t, e)
			if found || out != (ASTHandle{}) || m.floatingPointPlugin.numeralProbeFrames != nil {
				t.Fatal("symbolic rounding binary retained", kind, tc, found, out)
			}
		}
	}
}

// AUDIT.md ast_05-F1: fpa_decl_plugin::mk_func_decl turns a unary OP_FPA_SUB
// into mk_unary_decl(OP_FPA_NEG, ...).
func TestASTFPAUnarySubIsNegation(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	family, err := m.RegisterFloatingPointPlugin()
	astTestMust(t, err)
	floatSort, err := m.MkFamilySort(family, 3, nil)
	astTestMust(t, err)
	floatRef, err := m.internal(floatSort)
	astTestMust(t, err)
	checkNeg := func(decl ASTHandle) {
		t.Helper()
		r, e := m.internal(decl)
		astTestMust(t, e)
		name, e := m.Symbols.String(m.declAt(r.slot).name)
		astTestMust(t, e)
		_, kind, e := m.declIdentity(r)
		astTestMust(t, e)
		if name != "fp.neg" || kind != astFPANeg || m.declAt(r.slot).arity != 1 || m.declAt(r.slot).rangeSort != floatRef {
			t.Fatal("unary fp.sub declaration", name, kind, m.declAt(r.slot).arity)
		}
	}
	decl, err := m.MkFamilyFuncDecl(family, FPAOpSub, nil, []ASTHandle{floatSort}, ASTHandle{})
	astTestMust(t, err)
	checkNeg(decl)
	x, err := m.MkConst(astTestSymbol(t, m, "x"), floatSort)
	astTestMust(t, err)
	app, err := m.MkFamilyApp(family, FPAOpSub, nil, []ASTHandle{x})
	astTestMust(t, err)
	head, err := m.GetDecl(app)
	astTestMust(t, err)
	checkNeg(head)
}
