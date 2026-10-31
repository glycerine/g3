package api

// Source: src/api/api_ast.cpp:306-327,335-339; api_numeral.cpp:28-38.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIASTKindUsesNumeralSortAndUniqueValue(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	check := func(name string, h ast.ASTHandle, want APIASTKind) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if got, err := f.c.APIKind(h); err != nil || got != want {
				t.Fatal("source AST category", got, want, err)
			}
		})
	}
	integer := f.pin(f.c.IntegerSort())
	check("integer", f.pin(f.c.IntNumeral(3, integer)), APIASTNumeral)
	bits := f.pin(f.c.BitVectorSort(8))
	check("bit-vector", f.pin(f.c.UintNumeral(7, bits)), APIASTNumeral)
	finite := f.pin(f.c.FiniteDomainSort("kind-finite", 9))
	check("finite-domain", f.pin(f.c.UintNumeral(3, finite)), APIASTNumeral)
	check("boolean", f.pin(f.c.Bool(true)), APIASTApplication)
	check("string", f.pin(f.m.MkString("3")), APIASTApplication)
	arithmetic, err := f.m.ArithmeticFamilyID()
	astTestMust(t, err)
	check("pi", f.pin(f.m.MkFamilyApp(arithmetic, 31, nil, nil)), APIASTNumeral)
	check("e", f.pin(f.m.MkFamilyApp(arithmetic, 32, nil, nil)), APIASTNumeral)
	floating, err := f.m.ExistingFloatingPointFamilyID()
	astTestMust(t, err)
	for _, tc := range []struct {
		name      string
		operation int32
	}{
		{"nearest-even", ast.FPAOpRMNearestEven},
		{"nearest-away", ast.FPAOpRMNearestAway},
		{"toward-positive", ast.FPAOpRMTowardPositive},
		{"toward-negative", ast.FPAOpRMTowardNegative},
		{"toward-zero", ast.FPAOpRMTowardZero},
	} {
		check(tc.name, f.pin(f.m.MkFamilyApp(floating, tc.operation, nil, nil)), APIASTNumeral)
	}
	sort := f.pin(f.m.FloatingPointSort(8, 24))
	signSort := f.pin(f.c.BitVectorSort(1))
	sign := f.pin(f.c.UintNumeral(0, signSort))
	exponent := f.pin(f.c.UintNumeral(0, bits))
	significandSort := f.pin(f.c.BitVectorSort(23))
	significand := f.pin(f.c.UintNumeral(0, significandSort))
	constructed := f.pin(f.m.MkFamilyApp(floating, ast.FPAOpFP, nil, []ast.ASTHandle{sign, exponent, significand}))
	check("fp-constructed-value", constructed, APIASTNumeral)
	unknownSign := f.pin(f.c.Constant("unknown-sign", signSort))
	nonvalue := f.pin(f.m.MkFamilyApp(floating, ast.FPAOpFP, nil, []ast.ASTHandle{unknownSign, exponent, significand}))
	check("fp-constructed-nonvalue", nonvalue, APIASTApplication)
	check("fp-number", f.pin(f.m.MkFPANumeralInt32(3, sort)), APIASTApplication)
	check("fp-NaN", f.pin(f.m.MkFPANaNOfSort(sort)), APIASTApplication)
	for _, negative := range []bool{false, true} {
		name := "positive"
		if negative {
			name = "negative"
		}
		check("fp-"+name+"-zero", f.pin(f.m.MkFPAZeroOfSort(sort, negative)), APIASTApplication)
		check("fp-"+name+"-infinity", f.pin(f.m.MkFPAInfinityOfSort(sort, negative)), APIASTApplication)
	}
	check("sort", sort, APIASTSort)
}

func TestAPIASTKindAlgebraicIsApplication(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	if kind, err := f.c.APIKind(f.sqrt2(t)); err != nil || kind != APIASTApplication {
		t.Fatal("algebraic AST category", kind, err)
	}
}

func TestAPIIsApplicationNullAndInvalidOwners(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if yes, err := f.c.IsApplication(ast.ASTHandle{}); err != nil || yes {
		t.Fatal("null is not an application", yes, err)
	}
	other := newSMTAPIFixture(t, "ALL", nil)
	foreign := other.pin(other.c.Bool(true))
	if _, err := f.c.IsApplication(foreign); err == nil {
		t.Fatal("foreign handle accepted")
	}
	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err := closed.IsApplication(ast.ASTHandle{}); err == nil {
		t.Fatal("closed context accepted null")
	}
}
