package ast

// Source: src/ast/array_decl_plugin.cpp:329-431;
// src/ast/array_decl_plugin.h:176-185.

import (
	"errors"
	"testing"
)

func TestASTArraySetOriginalDeclarationsParametersFlagsAndAssociativeApps(t *testing.T) {
	m := astTestManager(t)
	fid, s, sorts := arrayDeclFixture(t, m, 2)
	for _, op := range []int32{ASTArrayOpSetUnion, ASTArrayOpSetIntersect, ASTArrayOpSetDifference, ASTArrayOpSetComplement, ASTArrayOpSetSubset} {
		domain := []ASTHandle{s, s}
		if op == ASTArrayOpSetComplement {
			domain = domain[:1]
		}
		d, e := m.MkFamilyFuncDecl(fid, op, nil, domain, ASTHandle{})
		astTestMust(t, e)
		astTestPin(t, m, d)
		r := m.declAt(mustInternal(t, m, d).slot)
		info, e := m.funcInfo(r.info)
		astTestMust(t, e)
		if op == ASTArrayOpSetUnion || op == ASTArrayOpSetIntersect {
			flags := ASTFuncFlags{LeftAssociative: true, RightAssociative: true, Commutative: true, Idempotent: true}
			if info.flags != flags || r.arity != 2 {
				t.Fatal("set flags/arity", op, info.flags, r.arity)
			}
			count := uint32(0)
			if op == ASTArrayOpSetUnion {
				count = 1
			}
			if info.count != count {
				t.Fatal("union-only sort parameter", op, info.count)
			}
			if count == 1 {
				p, e := m.infoParameter(info, 0)
				astTestMust(t, e)
				h, e := m.ParameterAST(p)
				astTestMust(t, e)
				if h != s {
					t.Fatal("retained union sort")
				}
			}
			for _, n := range []int{1, 3, 18} {
				domains := make([]ASTHandle, n)
				for i := range domains {
					domains[i] = s
				}
				again, e := m.MkFamilyFuncDecl(fid, op, nil, domains, ASTHandle{})
				astTestMust(t, e)
				if again != d {
					t.Fatal("canonical binary set declaration", op, n)
				}
			}
		} else if info.flags != (ASTFuncFlags{}) || info.count != 0 || r.arity != uint32(len(domain)) {
			t.Fatal("fixed set declaration", op)
		}
		want := s
		if op == ASTArrayOpSetSubset {
			want = sorts[2]
		}
		if m.external(r.rangeSort) != want {
			t.Fatal("set result sort", op)
		}
	}
	atomDecl, e := m.MkFuncDecl(astTestSymbol(t, m, "set-array"), nil, s, ASTFuncDeclInfo{})
	astTestMust(t, e)
	a, e := m.MkApp(atomDecl, nil)
	astTestMust(t, e)
	astTestPin(t, m, a)
	for _, op := range []int32{ASTArrayOpSetUnion, ASTArrayOpSetIntersect} {
		one, e := m.MkFamilyApp(fid, op, nil, []ASTHandle{a})
		astTestMust(t, e)
		astTestPin(t, m, one)
		count, e := m.GetNumArgs(one)
		astTestMust(t, e)
		if count != 1 {
			t.Fatal("unary set application")
		}
		three, e := m.MkFamilyApp(fid, op, nil, []ASTHandle{a, a, a})
		astTestMust(t, e)
		astTestPin(t, m, three)
		count, e = m.GetNumArgs(three)
		astTestMust(t, e)
		nested, e := m.GetArg(three, 1)
		astTestMust(t, e)
		nestedCount, e := m.GetNumArgs(nested)
		astTestMust(t, e)
		if count != 2 || nestedCount != 2 {
			t.Fatal("source right-associated set tree")
		}
	}
	empty, e := m.MkEmptySet(s)
	astTestMust(t, e)
	astTestPin(t, m, empty)
	full, e := m.MkFullSet(s)
	astTestMust(t, e)
	astTestPin(t, m, full)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	for _, pair := range [][2]ASTHandle{{empty, no}, {full, yes}} {
		v, e := m.GetArg(pair[0], 0)
		astTestMust(t, e)
		if v != pair[1] {
			t.Fatal("empty/full set value")
		}
		again, e := m.MkConstArray(s, pair[1])
		astTestMust(t, e)
		if again != pair[0] {
			t.Fatal("set constant identity")
		}
	}
}

func TestASTArraySetOriginalValidation(t *testing.T) {
	m := astTestManager(t)
	fid, s, sorts := arrayDeclFixture(t, m, 1)
	_, multi, _ := arrayDeclFixture(t, m, 2)
	other, e := m.MkUninterpretedSort(astTestSymbol(t, m, "set-non-bool-range"), nil)
	astTestMust(t, e)
	astTestPin(t, m, other)
	bp, e := m.NewParameterAST(sorts[0])
	astTestMust(t, e)
	op, e := m.NewParameterAST(other)
	astTestMust(t, e)
	badRange, e := m.MkFamilySort(fid, 0, []ASTParameter{bp, op})
	astTestMust(t, e)
	astTestPin(t, m, badRange)
	astTestMust(t, m.DelParameter(&bp))
	astTestMust(t, m.DelParameter(&op))
	for _, kind := range []int32{ASTArrayOpSetUnion, ASTArrayOpSetIntersect, ASTArrayOpSetDifference, ASTArrayOpSetComplement, ASTArrayOpSetSubset} {
		for _, domain := range [][]ASTHandle{nil, {sorts[0]}, {badRange}, {s, multi}, {s, badRange}} {
			if _, e = m.MkFamilyFuncDecl(fid, kind, nil, domain, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
				t.Fatal("bad set domain", kind, domain, e)
			}
		}
	}
	for _, kind := range []int32{ASTArrayOpSetDifference, ASTArrayOpSetSubset} {
		for _, n := range []int{1, 3} {
			domain := make([]ASTHandle, n)
			for i := range domain {
				domain[i] = s
			}
			if _, e = m.MkFamilyFuncDecl(fid, kind, nil, domain, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
				t.Fatal("binary set arity")
			}
		}
	}
	if _, e = m.MkFamilyFuncDecl(fid, ASTArrayOpSetComplement, nil, []ASTHandle{s, s}, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
		t.Fatal("complement arity")
	}
}

// AUDIT.md ast_01-F1: array declaration rejections carry the
// array_decl_plugin raise_exception text and still match ErrASTDomain.
func TestArrayDeclErrorMessages(t *testing.T) {
	m := astTestManager(t)
	fid, s, sorts := arrayDeclFixture(t, m, 1)
	x, err := m.MkConst(astTestSymbol(t, m, "s"), s)
	astTestMust(t, err)
	astTestPin(t, m, x)
	_, err = m.MkArrayAppArguments(fid, ASTArrayOpSetComplement, SliceArguments([]ASTHandle{x, x}))
	if err == nil || err.Error() != "set complement takes one argument" || !errors.Is(err, ErrASTDomain) {
		t.Fatalf("complement arity error: %v", err)
	}
	for _, tc := range []struct {
		op     int32
		domain []ASTHandle
		want   string
	}{
		{ASTArrayOpSetUnion, nil, "union takes at least one argument"},
		{ASTArrayOpSetIntersect, nil, "intersection takes at least one argument"},
		{ASTArrayOpSetSubset, []ASTHandle{s}, "subset takes two arguments"},
		{ASTArrayOpSetDifference, []ASTHandle{s}, "set difference takes precisely two arguments"},
		{ASTArrayOpSetUnion, []ASTHandle{s, sorts[0]}, "arguments 1 and 2 have different sorts"},
		{ASTArrayOpSetUnion, []ASTHandle{sorts[0]}, "argument 1 is not of array sort"},
		{ASTArrayOpDefault, []ASTHandle{s, s}, "invalid default array definition, invalid domain size"},
		{ASTArrayOpDefault, []ASTHandle{sorts[0]}, "parameter mismatch. There should be more than one parameter to defaults"},
		{ASTArrayOpAsArray, nil, "as-array takes one parameter, a function declaration with arity greater than zero"},
		{ASTArrayOpConst, []ASTHandle{sorts[0]}, "array operation requires one sort parameter (the array sort)"},
		{ASTArrayOpMap, []ASTHandle{s}, "array operation requires one function declaration parameter (the function to be mapped)"},
	} {
		_, e := m.MkFamilyFuncDecl(fid, tc.op, nil, tc.domain, ASTHandle{})
		if e == nil || e.Error() != tc.want || !errors.Is(e, ErrASTDomain) {
			t.Fatalf("op %d: %v, want %q", tc.op, e, tc.want)
		}
	}
	_, e := m.MkFamilyFuncDecl(fid, ASTArrayOpConst, nil, []ASTHandle{sorts[0]}, sorts[0])
	if e == nil || e.Error() != "invalid const array definition, parameter is not an array sort" {
		t.Fatalf("const with non-array range: %v", e)
	}
	_, e = m.MkFamilyFuncDecl(fid, ASTArrayOpConst, nil, nil, s)
	if e == nil || e.Error() != "invalid const array definition, invalid domain size" {
		t.Fatalf("const without argument: %v", e)
	}
	p, e := m.NewParameterAST(sorts[0])
	astTestMust(t, e)
	_, e = m.MkFamilySort(fid, 0, []ASTParameter{p})
	if e == nil || e.Error() != "invalid array sort definition, invalid number of parameters" {
		t.Fatalf("array sort with one parameter: %v", e)
	}
	astTestMust(t, m.DelParameter(&p))
}
