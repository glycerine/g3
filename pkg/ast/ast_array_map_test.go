package ast

// Source: src/ast/array_decl_plugin.cpp:139-211,468-476;
// src/ast/array_decl_plugin.h:172-175.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"testing"
)

func TestASTArrayMapOriginalDomainsRangeFlagsAndFunction(t *testing.T) {
	m := astTestManager(t)
	fid, input, sorts := arrayDeclFixture(t, m, 18)
	result, e := m.MkUninterpretedSort(astTestSymbol(t, m, "map-result"), nil)
	astTestMust(t, e)
	astTestPin(t, m, result)
	flags := ASTFuncFlags{LeftAssociative: true, RightAssociative: true, Commutative: true, Idempotent: true}
	info, e := m.NewFuncDeclInfo(NullFamilyID, 0, nil, flags, false)
	astTestMust(t, e)
	f, e := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "map-binary"), []ASTHandle{sorts[18], sorts[18]}, result, info)
	astTestMust(t, e)
	astTestPin(t, m, f)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	p, e := m.NewParameterAST(f)
	astTestMust(t, e)
	d, e := m.MkFamilyFuncDecl(fid, ASTArrayOpMap, []ASTParameter{p}, []ASTHandle{input, input}, ASTHandle{})
	astTestMust(t, e)
	astTestPin(t, m, d)
	astTestMust(t, m.DelParameter(&p))
	got, e := m.ArrayMapFunction(d)
	astTestMust(t, e)
	if got != f {
		t.Fatal("mapped function", got, f)
	}
	dr := m.declAt(mustInternal(t, m, d).slot)
	gotFlags, e := m.funcFlags(mustInternal(t, m, d))
	astTestMust(t, e)
	flags.Idempotent = false
	if gotFlags != flags || dr.arity != 2 {
		t.Fatal("inherited flags/arity", gotFlags, dr.arity)
	}
	rinfo, e := m.sortInfo(m.sortAt(dr.rangeSort.slot).info)
	astTestMust(t, e)
	if rinfo.count != 19 {
		t.Fatal("map array arity", rinfo.count)
	}
	for i := uint32(0); i < rinfo.count; i++ {
		got, e := m.arraySortParameter(rinfo, i)
		astTestMust(t, e)
		want := sorts[i]
		if i == 18 {
			want = result
		}
		if m.external(got) != want {
			t.Fatal("mapped sort", i, m.external(got), want)
		}
	}
	aDecl, e := m.MkFuncDecl(astTestSymbol(t, m, "map-array"), nil, input, ASTFuncDeclInfo{})
	astTestMust(t, e)
	a, e := m.MkApp(aDecl, nil)
	astTestMust(t, e)
	astTestPin(t, m, a)
	h, e := m.MkArrayMap(f, SliceArguments([]ASTHandle{a, a}))
	astTestMust(t, e)
	astTestPin(t, m, h)
	hd, e := m.GetDecl(h)
	astTestMust(t, e)
	if hd != d {
		t.Fatal("map canonical declaration")
	}
	args, e := m.AppArgumentView(h)
	astTestMust(t, e)
	again, e := m.MkArrayMap(f, args)
	astTestMust(t, e)
	if again != h {
		t.Fatal("borrowed map identity")
	}
	info, e = m.NewFuncDeclInfo(NullFamilyID, 0, nil, ASTFuncFlags{Injective: true}, false)
	astTestMust(t, e)
	unary, e := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "map-injective"), sorts[18:], result, info)
	astTestMust(t, e)
	astTestPin(t, m, unary)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	uh, e := m.MkArrayMap(unary, SliceArguments([]ASTHandle{a}))
	astTestMust(t, e)
	astTestPin(t, m, uh)
	ud, e := m.GetDecl(uh)
	astTestMust(t, e)
	uf, e := m.funcFlags(mustInternal(t, m, ud))
	astTestMust(t, e)
	if uf != (ASTFuncFlags{Injective: true}) {
		t.Fatal("map unary injectivity", uf)
	}
}

func TestASTArrayMapOriginalValidation(t *testing.T) {
	m := astTestManager(t)
	fid, input, sorts := arrayDeclFixture(t, m, 1)
	other, e := m.MkUninterpretedSort(astTestSymbol(t, m, "map-other"), nil)
	astTestMust(t, e)
	astTestPin(t, m, other)
	op, e := m.NewParameterAST(other)
	astTestMust(t, e)
	bp, e := m.NewParameterAST(sorts[0])
	astTestMust(t, e)
	differentIndex, e := m.MkFamilySort(fid, 0, []ASTParameter{op, bp})
	astTestMust(t, e)
	astTestPin(t, m, differentIndex)
	differentRange, e := m.MkFamilySort(fid, 0, []ASTParameter{bp, op})
	astTestMust(t, e)
	astTestPin(t, m, differentRange)
	astTestMust(t, m.DelParameter(&op))
	astTestMust(t, m.DelParameter(&bp))
	f, e := m.MkFuncDecl(astTestSymbol(t, m, "valid-map-function"), []ASTHandle{sorts[0], sorts[0]}, sorts[0], ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, f)
	fp, e := m.NewParameterAST(f)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&fp)) }()
	sp, e := m.NewParameterAST(input)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&sp)) }()
	ip, e := m.NewParameterInt(7)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&ip)) }()
	for _, ps := range [][]ASTParameter{nil, {fp, fp}, {sp}, {ip}} {
		if _, e = m.MkFamilyFuncDecl(fid, ASTArrayOpMap, ps, []ASTHandle{input, input}, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
			t.Fatal("invalid map parameter", e)
		}
	}
	_, multi, _ := arrayDeclFixture(t, m, 2)
	for _, domain := range [][]ASTHandle{nil, {input}, {input, input, input}, {sorts[0], input}, {input, multi}, {input, differentIndex}, {input, differentRange}} {
		if _, e = m.MkFamilyFuncDecl(fid, ASTArrayOpMap, []ASTParameter{fp}, domain, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
			t.Fatal("invalid mapped domain", domain, e)
		}
	}
	zero, e := m.MkFuncDecl(astTestSymbol(t, m, "map-zero"), nil, sorts[0], ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, zero)
	zp, e := m.NewParameterAST(zero)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&zp)) }()
	if _, e = m.MkFamilyFuncDecl(fid, ASTArrayOpMap, []ASTParameter{zp}, nil, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
		t.Fatal("zero arity map", e)
	}
	if _, e = m.ArrayMapFunction(f); e != ErrASTDomain {
		t.Fatal("non-map declaration", e)
	}
}

func TestASTArrayMapOriginalParameterVectorRefusalAndReuse(t *testing.T) {
	m := astTestManager(t)
	fid, input, sorts := arrayDeclFixture(t, m, 18)
	f, e := m.MkFuncDecl(astTestSymbol(t, m, "map-refusal"), []ASTHandle{sorts[18]}, sorts[18], ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, f)
	p, e := m.NewParameterAST(f)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&p)) }()
	d, e := m.MkFamilyFuncDecl(fid, ASTArrayOpMap, []ASTParameter{p}, []ASTHandle{input}, ASTHandle{})
	astTestMust(t, e)
	astTestPin(t, m, d)
	baseline, parameters := m.Budget().Used(), m.liveParameters
	for fault := int64(0); fault < 64; fault++ {
		m.Budget().FailAfter(fault)
		got, e := m.MkFamilyFuncDecl(fid, ASTArrayOpMap, []ASTParameter{p}, []ASTHandle{input}, ASTHandle{})
		m.Budget().FailAfter(-1)
		if m.Budget().Used() != baseline || m.liveParameters != parameters {
			t.Fatal("map temporary vector leak", fault)
		}
		if e == nil {
			if got != d {
				t.Fatal("map reuse identity")
			}
			return
		}
		if !errors.Is(e, arena.ErrInjectedFailure) {
			t.Fatal("map refusal", fault, e)
		}
	}
	t.Fatal("map refusal campaign never succeeded")
}

func TestASTArraySortUtilityAbsentPluginAndDomainBounds(t *testing.T) {
	m := astTestManager(t)
	bs, e := m.MkBoolSort()
	astTestMust(t, e)
	absent, e := m.MkArraySort(SliceArguments([]ASTHandle{bs}), bs)
	astTestMust(t, e)
	if !absent.IsNull() {
		t.Fatal("unregistered array sort")
	}
	_, e = m.RegisterArrayPlugin()
	astTestMust(t, e)
	s, e := m.MkArraySort(SliceArguments([]ASTHandle{bs, bs}), bs)
	astTestMust(t, e)
	astTestPin(t, m, s)
	n, e := m.ArrayArity(s)
	astTestMust(t, e)
	if n != 2 {
		t.Fatal("array domain count", n)
	}
	for _, i := range []uint32{0, 1} {
		d, e := m.ArrayDomain(s, i)
		astTestMust(t, e)
		if d != bs {
			t.Fatal("array domain", i)
		}
	}
	for _, i := range []uint32{2, ^uint32(0)} {
		if _, e = m.ArrayDomain(s, i); e != ErrASTDomain {
			t.Fatal("array domain bounds", i, e)
		}
	}
	rangeSort, e := m.ArrayRange(s)
	astTestMust(t, e)
	if rangeSort != bs {
		t.Fatal("array range", rangeSort, bs)
	}
}
