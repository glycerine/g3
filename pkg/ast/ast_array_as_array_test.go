package ast

// Source: src/ast/array_decl_plugin.cpp:434-444,498-512,561-564.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"testing"
)

func TestASTArrayAsArrayOriginalDeclarationsAndRecognizer(t *testing.T) {
	m := astTestManager(t)
	fid, e := m.RegisterArrayPlugin()
	astTestMust(t, e)
	for _, n := range []int{1, 2, 18} {
		_, sort, sorts := arrayDeclFixture(t, m, n)
		f, e := m.MkFuncDecl(astTestSymbol(t, m, "array-backed-function"), sorts[:n], sorts[n], ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestPin(t, m, f)
		p, e := m.NewParameterAST(f)
		astTestMust(t, e)
		d, e := m.MkFamilyFuncDecl(fid, ASTArrayOpAsArray, []ASTParameter{p}, nil, ASTHandle{})
		astTestMust(t, e)
		astTestPin(t, m, d)
		arity, e := m.GetArity(d)
		astTestMust(t, e)
		rangeSort, e := m.GetRange(d)
		astTestMust(t, e)
		if arity != 0 || rangeSort != sort {
			t.Fatal("as-array shape", arity, rangeSort, sort)
		}
		h, e := m.MkFamilyApp(fid, ASTArrayOpAsArray, []ASTParameter{p}, nil)
		astTestMust(t, e)
		astTestPin(t, m, h)
		astTestMust(t, m.DelParameter(&p))
		referenced, e := m.AsArrayFunction(h)
		astTestMust(t, e)
		if referenced != f {
			t.Fatal("as-array function", referenced, f)
		}
		again, e := m.MkAsArray(f)
		astTestMust(t, e)
		if again != h {
			t.Fatal("as-array canonical identity")
		}
	}
}

func TestASTArrayAsArrayRejectsOriginalInvalidParameters(t *testing.T) {
	m := astTestManager(t)
	fid, e := m.RegisterArrayPlugin()
	astTestMust(t, e)
	bs, e := m.MkBoolSort()
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "zero-arity-array-function"), nil, bs, ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, d)
	cp, e := m.NewParameterAST(d)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&cp)) }()
	sp, e := m.NewParameterAST(bs)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&sp)) }()
	ip, e := m.NewParameterInt(1)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&ip)) }()
	for _, ps := range [][]ASTParameter{nil, {cp}, {sp}, {ip}, {cp, cp}} {
		if _, e = m.MkFamilyFuncDecl(fid, ASTArrayOpAsArray, ps, nil, ASTHandle{}); !errors.Is(e, ErrASTDomain) {
			t.Fatal("bad as-array parameter", ps, e)
		}
	}
	yes, e := m.MkTrue()
	astTestMust(t, e)
	if _, e = m.AsArrayFunction(yes); e != ErrASTDomain {
		t.Fatal("non-as-array recognizer", e)
	}
}

func TestASTArrayAsArrayParameterVectorRefusalAndReuse(t *testing.T) {
	m := astTestManager(t)
	_, _, sorts := arrayDeclFixture(t, m, 18)
	f, e := m.MkFuncDecl(astTestSymbol(t, m, "refused-as-array"), sorts[:18], sorts[18], ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, f)
	h, e := m.MkAsArray(f)
	astTestMust(t, e)
	astTestPin(t, m, h)
	baseline, parameters := m.Budget().Used(), m.liveParameters
	m.Budget().FailAfter(0)
	_, e = m.MkAsArray(f)
	m.Budget().FailAfter(-1)
	if !errors.Is(e, arena.ErrInjectedFailure) {
		t.Fatal("actual parameter vector allocation not refused", e)
	}
	if m.Budget().Used() != baseline || m.liveParameters != parameters {
		t.Fatal("parameter vector refusal leaked")
	}
	again, e := m.MkAsArray(f)
	astTestMust(t, e)
	if again != h {
		t.Fatal("as-array reuse")
	}
}

func TestASTAppArgumentSubrangesRemainBorrowedAndBounded(t *testing.T) {
	m := astTestManager(t)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	app, e := m.MkAndArgs([]ASTHandle{yes, no, yes})
	astTestMust(t, e)
	astTestPin(t, m, app)
	borrowed, e := m.AppArgumentView(app)
	astTestMust(t, e)
	for _, args := range []AppArguments{SliceArguments([]ASTHandle{yes, no, yes}), borrowed} {
		tail, e := args.Range(1, 2)
		astTestMust(t, e)
		sub, e := tail.Range(1, 1)
		astTestMust(t, e)
		first, e := tail.At(0)
		astTestMust(t, e)
		last, e := sub.At(0)
		astTestMust(t, e)
		if first != no || last != yes || tail.Size() != 2 || sub.Count() != 1 {
			t.Fatal("borrowed subrange")
		}
		if _, e = tail.At(2); e != ErrASTDomain {
			t.Fatal("view escaped bounds", e)
		}
		empty, e := args.Range(0, 0)
		astTestMust(t, e)
		if empty.Size() != 0 {
			t.Fatal("zero-length view")
		}
		if _, e = args.Range(2, 2); e != ErrASTDomain {
			t.Fatal("bad range", e)
		}
	}
}

func TestASTArrayAsArrayPreservesHeterogeneousDomainOrder(t *testing.T) {
	m := astTestManager(t)
	fid, e := m.RegisterArrayPlugin()
	astTestMust(t, e)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	other, e := m.MkUninterpretedSort(astTestSymbol(t, m, "as-array-index-sort"), nil)
	astTestMust(t, e)
	astTestPin(t, m, other)
	sorts := []ASTHandle{b, other, b}
	parameters := make([]ASTParameter, 3)
	for i, s := range sorts {
		parameters[i], e = m.NewParameterAST(s)
		astTestMust(t, e)
	}
	expected, e := m.MkFamilySort(fid, 0, parameters)
	astTestMust(t, e)
	astTestPin(t, m, expected)
	for i := range parameters {
		astTestMust(t, m.DelParameter(&parameters[i]))
	}
	f, e := m.MkFuncDecl(astTestSymbol(t, m, "heterogeneous-array-function"), sorts[:2], b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, f)
	array, e := m.MkAsArray(f)
	astTestMust(t, e)
	astTestPin(t, m, array)
	actual, e := m.GetSort(array)
	astTestMust(t, e)
	if actual != expected {
		t.Fatal("function domains were reordered", actual, expected)
	}
}
