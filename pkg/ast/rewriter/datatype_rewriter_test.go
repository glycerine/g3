package rewriter

// Source: src/ast/rewriter/datatype_rewriter.cpp:24-140.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type datatypeRewriteFixture struct {
	m                          *ast.ASTManager
	r                          *DatatypeRewriter
	pair, empty, first, second ast.ASTHandle
	recognizer, isPair, update ast.ASTHandle
	truth, falsity             ast.ASTHandle
}

func newDatatypeRewriteFixture(t *testing.T) datatypeRewriteFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	parameterSymbol := func(text string) ast.ASTParameter {
		p, e := m.NewParameterSymbol(astTestSymbol(t, m, text))
		astTestMust(t, e)
		return p
	}
	// The accessor index follows the datatype's definition
	// (util::get_constructor_accessors), so Pair is defined first; the
	// declarations below hash-cons to the definition's own.
	_, err = m.MkDatatypes([]ast.DatatypeDefinition{{
		Name: astTestSymbol(t, m, "Pair"),
		Constructors: []ast.DatatypeConstructorDefinition{
			{Name: astTestSymbol(t, m, "pair"), Recognizer: astTestSymbol(t, m, "is-pair"), Accessors: []ast.DatatypeAccessorDefinition{
				{Name: astTestSymbol(t, m, "first"), Range: boolSort},
				{Name: astTestSymbol(t, m, "second"), Range: boolSort},
			}},
			{Name: astTestSymbol(t, m, "empty"), Recognizer: astTestSymbol(t, m, "is-empty")},
		},
	}})
	astTestMust(t, err)
	name := parameterSymbol("Pair")
	dt, err := m.MkFamilySort(family, ast.ASTDatatypeSort, []ast.ASTParameter{name})
	astTestMust(t, err)
	astTestPin(t, m, dt)
	astTestMust(t, m.DelParameter(&name))
	constructor := func(text string, domain []ast.ASTHandle) ast.ASTHandle {
		p := parameterSymbol(text)
		d, e := m.MkFamilyFuncDecl(family, ast.ASTDatatypeConstructor, []ast.ASTParameter{p}, domain, dt)
		astTestMust(t, e)
		astTestPin(t, m, d)
		astTestMust(t, m.DelParameter(&p))
		return d
	}
	pair := constructor("pair", []ast.ASTHandle{boolSort, boolSort})
	empty := constructor("empty", nil)
	accessor := func(field, constructor string) ast.ASTHandle {
		p0, p1 := parameterSymbol(field), parameterSymbol(constructor)
		d, e := m.MkFamilyFuncDecl(family, ast.ASTDatatypeAccessor, []ast.ASTParameter{p0, p1}, []ast.ASTHandle{dt}, boolSort)
		astTestMust(t, e)
		astTestPin(t, m, d)
		astTestMust(t, m.DelParameter(&p1))
		astTestMust(t, m.DelParameter(&p0))
		return d
	}
	first := accessor("first", "pair")
	second := accessor("second", "pair")
	pairParameter, err := m.NewParameterAST(pair)
	astTestMust(t, err)
	recognizerName := parameterSymbol("is-pair")
	recognizer, err := m.MkFamilyFuncDecl(family, ast.ASTDatatypeRecognizer, []ast.ASTParameter{pairParameter, recognizerName}, []ast.ASTHandle{dt}, ast.ASTHandle{})
	astTestMust(t, err)
	astTestPin(t, m, recognizer)
	astTestMust(t, m.DelParameter(&recognizerName))
	isPair, err := m.MkFamilyFuncDecl(family, ast.ASTDatatypeIs, []ast.ASTParameter{pairParameter}, []ast.ASTHandle{dt}, ast.ASTHandle{})
	astTestMust(t, err)
	astTestPin(t, m, isPair)
	astTestMust(t, m.DelParameter(&pairParameter))
	firstParameter, err := m.NewParameterAST(first)
	astTestMust(t, err)
	update, err := m.MkFamilyFuncDecl(family, ast.ASTDatatypeUpdateField, []ast.ASTParameter{firstParameter}, []ast.ASTHandle{dt, boolSort}, ast.ASTHandle{})
	astTestMust(t, err)
	astTestPin(t, m, update)
	astTestMust(t, m.DelParameter(&firstParameter))
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	r, err := NewDatatypeRewriter(m)
	astTestMust(t, err)
	return datatypeRewriteFixture{m: m, r: r, pair: pair, empty: empty, first: first, second: second, recognizer: recognizer, isPair: isPair, update: update, truth: truth, falsity: falsity}
}

func TestDatatypeRewriterOriginalApplicationReductions(t *testing.T) {
	f := newDatatypeRewriteFixture(t)
	pair, err := f.m.MkApp(f.pair, []ast.ASTHandle{f.truth, f.falsity})
	astTestMust(t, err)
	astTestPin(t, f.m, pair)
	empty, err := f.m.MkApp(f.empty, nil)
	astTestMust(t, err)
	astTestPin(t, f.m, empty)
	out, err := f.m.NewRef(pair)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&out)) }()

	status, err := f.r.RewriteApp(f.recognizer, ast.SliceArguments([]ast.ASTHandle{pair}), out)
	astTestMust(t, err)
	got, err := f.m.RefGet(out)
	astTestMust(t, err)
	kind, err := f.m.GetDeclKind(got)
	astTestMust(t, err)
	if status != Rewrite1 || kind != ast.ASTDatatypeIs {
		t.Fatal("recognizer rewrite", status, kind)
	}

	for _, row := range []struct {
		value ast.ASTHandle
		want  ast.ASTHandle
	}{{pair, f.truth}, {empty, f.falsity}} {
		status, err = f.r.RewriteApp(f.isPair, ast.SliceArguments([]ast.ASTHandle{row.value}), out)
		astTestMust(t, err)
		got, err = f.m.RefGet(out)
		astTestMust(t, err)
		if status != RewriteDone || got != row.want {
			t.Fatal("is rewrite", status, got, row.want)
		}
	}

	status, err = f.r.RewriteApp(f.second, ast.SliceArguments([]ast.ASTHandle{pair}), out)
	astTestMust(t, err)
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || got != f.falsity {
		t.Fatal("accessor rewrite", status, got)
	}

	status, err = f.r.RewriteApp(f.update, ast.SliceArguments([]ast.ASTHandle{pair, f.falsity}), out)
	astTestMust(t, err)
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	first, err := f.m.GetArg(got, 0)
	astTestMust(t, err)
	second, err := f.m.GetArg(got, 1)
	astTestMust(t, err)
	if status != RewriteDone || first != f.falsity || second != f.falsity {
		t.Fatal("field update", status, first, second)
	}

	status, err = f.r.RewriteApp(f.update, ast.SliceArguments([]ast.ASTHandle{empty, f.falsity}), out)
	astTestMust(t, err)
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || got != empty {
		t.Fatal("other constructor update", status, got)
	}
}

func TestDatatypeRewriterOriginalConstructorEquality(t *testing.T) {
	f := newDatatypeRewriteFixture(t)
	left, err := f.m.MkApp(f.pair, []ast.ASTHandle{f.truth, f.falsity})
	astTestMust(t, err)
	astTestPin(t, f.m, left)
	right, err := f.m.MkApp(f.pair, []ast.ASTHandle{f.falsity, f.truth})
	astTestMust(t, err)
	astTestPin(t, f.m, right)
	empty, err := f.m.MkApp(f.empty, nil)
	astTestMust(t, err)
	astTestPin(t, f.m, empty)
	out, err := f.m.NewRef(left)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&out)) }()

	status, err := f.r.RewriteEquality(left, empty, out)
	astTestMust(t, err)
	got, err := f.m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || got != f.falsity {
		t.Fatal("different constructors", status, got)
	}
	status, err = f.r.RewriteEquality(left, right, out)
	astTestMust(t, err)
	got, err = f.m.RefGet(out)
	astTestMust(t, err)
	kind, err := f.m.GetDeclKind(got)
	astTestMust(t, err)
	count, err := f.m.GetNumArgs(got)
	astTestMust(t, err)
	if status != Rewrite2 || kind != ast.ASTOpAnd || count != 2 {
		t.Fatal("same constructor", status, kind, count)
	}
}
