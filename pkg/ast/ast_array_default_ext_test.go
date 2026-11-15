package ast

// Source: src/ast/array_decl_plugin.cpp:211-232,307-325,478-487,598-602.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"testing"
)

func TestASTArrayOriginalDefaultAndExtensionalityDeclarations(t *testing.T) {
	m := astTestManager(t)
	fid, s, sorts := arrayDeclFixture(t, m, 18)
	other, e := m.MkUninterpretedSort(astTestSymbol(t, m, "ext-index"), nil)
	astTestMust(t, e)
	astTestPin(t, m, other)
	parameters := make([]ASTParameter, 19)
	for i := range parameters {
		h := sorts[i]
		if i == 7 {
			h = other
		}
		parameters[i], e = m.NewParameterAST(h)
		astTestMust(t, e)
	}
	s, e = m.MkFamilySort(fid, 0, parameters)
	astTestMust(t, e)
	astTestPin(t, m, s)
	for i := range parameters {
		astTestMust(t, m.DelParameter(&parameters[i]))
	}
	d, e := m.MkFamilyFuncDecl(fid, ASTArrayOpDefault, nil, []ASTHandle{s}, ASTHandle{})
	astTestMust(t, e)
	astTestPin(t, m, d)
	arity, e := m.GetArity(d)
	astTestMust(t, e)
	r, e := m.GetRange(d)
	astTestMust(t, e)
	if arity != 1 || r != sorts[18] {
		t.Fatal("default signature", arity, r)
	}
	for _, i := range []uint32{0, 7, 17} {
		ext, e := m.MkArrayExtDecl(s, i)
		astTestMust(t, e)
		astTestPin(t, m, ext)
		r, e = m.GetRange(ext)
		astTestMust(t, e)
		want := sorts[i]
		if i == 7 {
			want = other
		}
		if r != want {
			t.Fatal("extensionality index range", i, r, want)
		}
		decl := m.declAt(mustInternal(t, m, ext).slot)
		info, e := m.funcInfo(decl.info)
		astTestMust(t, e)
		if decl.arity != 2 || info.count != 1 || info.flags != (ASTFuncFlags{}) || info.private {
			t.Fatal("extensionality signature")
		}
		p, e := m.infoParameter(info, 0)
		astTestMust(t, e)
		index, e := m.ParameterInt(p)
		astTestMust(t, e)
		if index != int32(i) {
			t.Fatal("retained extensionality index")
		}
		if i == 0 {
			again, e := m.MkFamilyFuncDecl(fid, ASTArrayOpExt, nil, []ASTHandle{s, s}, ASTHandle{})
			astTestMust(t, e)
			if again != ext {
				t.Fatal("implicit zero extensionality parameter")
			}
		}
	}
	baseDecl, e := m.MkFuncDecl(astTestSymbol(t, m, "default-array"), nil, s, ASTFuncDeclInfo{})
	astTestMust(t, e)
	a, e := m.MkApp(baseDecl, nil)
	astTestMust(t, e)
	astTestPin(t, m, a)
	h, e := m.MkFamilyApp(fid, ASTArrayOpDefault, nil, []ASTHandle{a})
	astTestMust(t, e)
	astTestPin(t, m, h)
	got, e := m.GetDecl(h)
	astTestMust(t, e)
	if got != d {
		t.Fatal("default expression overload")
	}
}

func TestASTArrayDefaultExtensionalityOriginalValidationAndStructuralSorts(t *testing.T) {
	m := astTestManager(t)
	fid, s, sorts := arrayDeclFixture(t, m, 1)
	_, different, _ := arrayDeclFixture(t, m, 2)
	for _, domain := range [][]ASTHandle{nil, {s, s}, {sorts[0]}} {
		if _, e := m.MkFamilyFuncDecl(fid, ASTArrayOpDefault, nil, domain, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
			t.Fatal("default domain", domain, e)
		}
	}
	for _, domain := range [][]ASTHandle{nil, {s}, {s, different}, {sorts[0], sorts[0]}} {
		if _, e := m.MkFamilyFuncDecl(fid, ASTArrayOpExt, nil, domain, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
			t.Fatal("ext domain", domain, e)
		}
	}
	sp, e := m.NewParameterAST(s)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&sp)) }()
	ip, e := m.NewParameterInt(1)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&ip)) }()
	for _, ps := range [][]ASTParameter{{sp}, {ip}, {ip, ip}} {
		if _, e = m.MkFamilyFuncDecl(fid, ASTArrayOpExt, ps, []ASTHandle{s, s}, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
			t.Fatal("ext parameter", ps, e)
		}
	}
	for _, i := range []uint32{1, ^uint32(0)} {
		if _, e = m.MkArrayExtDecl(s, i); !errors.Is(e, ErrASTDomain) {
			t.Fatal("ext index bounds", i, e)
		}
	}
	// These source factories read structural sort parameters, without checking
	// the array family. Do not add a family restriction absent from the source.
	p, e := m.NewParameterAST(sorts[0])
	astTestMust(t, e)
	info, e := m.NewSortInfo(NullFamilyID, 0, []ASTParameter{p, p}, InfiniteSortSize(), false)
	astTestMust(t, e)
	structural, e := m.MkSort(astTestSymbol(t, m, "structural-array-like"), info)
	astTestMust(t, e)
	astTestPin(t, m, structural)
	astTestMust(t, m.DelSortInfo(&info))
	astTestMust(t, m.DelParameter(&p))
	for _, kind := range []int32{ASTArrayOpDefault, ASTArrayOpExt} {
		domain := []ASTHandle{structural}
		if kind == ASTArrayOpExt {
			domain = append(domain, structural)
		}
		d, e := m.MkFamilyFuncDecl(fid, kind, nil, domain, ASTHandle{})
		astTestMust(t, e)
		astTestPin(t, m, d)
		r, e := m.GetRange(d)
		astTestMust(t, e)
		if r != sorts[0] {
			t.Fatal("structural sort parameter range")
		}
	}
}

func TestASTArrayExtensionalityWarmRefusalAndParameterCleanup(t *testing.T) {
	m := astTestManager(t)
	_, s, _ := arrayDeclFixture(t, m, 2)
	d, e := m.MkArrayExtDecl(s, 1)
	astTestMust(t, e)
	astTestPin(t, m, d)
	baseline, parameters := m.Budget().Used(), m.liveParameters
	allocated := m.Stats().AllocatedBytes
	for fault := int64(0); fault < 64; fault++ {
		m.Budget().FailAfter(fault)
		got, e := m.MkArrayExtDecl(s, 1)
		m.Budget().FailAfter(-1)
		if m.liveParameters != parameters {
			t.Fatal("extensionality parameter cleanup", fault, e, m.Budget().Used(), baseline, m.liveParameters, parameters)
		}
		if e == nil {
			if got != d {
				t.Fatal("extensionality reuse")
			}
			// A successful checked/debug call can grow retained manager capacity.
			// Account that real owning storage rather than requiring its rollback.
			if m.Budget().Used()-baseline != m.Stats().AllocatedBytes-allocated {
				t.Fatal("successful extensionality retained unowned charge")
			}
			return
		}
		if m.Budget().Used() != baseline {
			t.Fatal("failed extensionality reservation retained charge", fault, m.Budget().Used(), baseline)
		}
		if !errors.Is(e, arena.ErrInjectedFailure) {
			t.Fatal("extensionality refusal", fault, e)
		}
	}
	t.Fatal("extensionality refusal campaign never succeeded")
}
