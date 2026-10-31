package duality_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestMissingHeadClausesQuantifiedAndGround(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		sym, e := m.Symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(sym, domain, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		return d
	}
	p := decl("duality-head-p", nil)
	q := decl("duality-head-q", []ast.ASTHandle{integer, boolean})
	r := decl("duality-head-r", nil)
	present, err := m.MkApp(p, nil)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	impl, err := m.MkImplies(truth, present)
	must(err)
	quantified, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: impl})
	must(err)
	owner, err := m.NewRef(quantified)
	must(err)
	defer func() { must(m.DelRef(&owner)) }()
	refs, err := duality.MissingHeadClauses(m, []ast.ASTHandle{quantified}, []ast.ASTHandle{p, q, r})
	must(err)
	defer func() {
		for i := range refs {
			must(m.DelRef(&refs[i]))
		}
	}()
	if len(refs) != 2 {
		t.Fatal("missing clause count", len(refs))
	}
	h, err := m.RefGet(refs[0])
	must(err)
	count, err := m.GetNumDeclarations(h)
	must(err)
	if count != 2 {
		t.Fatal("quantified missing head arity", count)
	}
	for i, want := range []ast.ASTHandle{integer, boolean} {
		got, e := m.GetDeclarationSort(h, uint32(i))
		must(e)
		if got != want {
			t.Fatal("binder sort", i, got, want)
		}
	}
	body, err := m.GetQuantifierBody(h)
	must(err)
	lhs, err := m.GetArg(body, 0)
	must(err)
	head, err := m.GetArg(body, 1)
	must(err)
	d, err := m.GetDecl(head)
	must(err)
	if !m.IsFalse(lhs) || d != q {
		t.Fatal("dummy rule", lhs, d)
	}
	for i, want := range []uint32{1, 0} {
		arg, e := m.GetArg(head, uint32(i))
		must(e)
		index, e := m.GetVarIndex(arg)
		must(e)
		if index != want {
			t.Fatal("abstracted argument", i, index, want)
		}
	}
	h, err = m.RefGet(refs[1])
	must(err)
	kind, err := m.GetKind(h)
	must(err)
	if kind != ast.ASTApp {
		t.Fatal("ground missing head was quantified", kind)
	}
	lhs, err = m.GetArg(h, 0)
	must(err)
	head, err = m.GetArg(h, 1)
	must(err)
	d, err = m.GetDecl(head)
	must(err)
	if !m.IsFalse(lhs) || d != r {
		t.Fatal("ground dummy rule", lhs, d)
	}
}
