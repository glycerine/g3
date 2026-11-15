package interp

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func interpASTFixture(t *testing.T) *ast.ASTManager {
	t.Helper()
	b := arena.NewBudget(1 << 30)
	s, err := symbol.NewManager(symbol.Config{Budget: b})
	interpMust(t, err)
	n, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	interpMust(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: s, Numerals: n, Budget: b})
	interpMust(t, err)
	t.Cleanup(func() {
		interpMust(t, m.Close())
		interpMust(t, n.Close())
		interpMust(t, s.Close())
		if b.Used() != 0 {
			t.Errorf("interpolation AST fixture retained %d bytes", b.Used())
		}
	})
	return m
}

func interpPin(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	r, err := m.NewRef(h)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, m.DelRef(&r)) })
	return h
}

func TestInterpolationPrettyPrintProblemExactTreeAndNames(t *testing.T) {
	m := interpASTFixture(t)
	boolSort, err := m.MkBoolSort()
	interpMust(t, err)
	pName, err := m.Symbols.Intern("p")
	interpMust(t, err)
	qName, err := m.Symbols.Intern("q")
	interpMust(t, err)
	p, err := m.MkConst(pName, boolSort)
	interpMust(t, err)
	p = interpPin(t, m, p)
	q, err := m.MkConst(qName, boolSort)
	interpMust(t, err)
	q = interpPin(t, m, q)
	marked, err := m.MkInterp(q)
	interpMust(t, err)
	marked = interpPin(t, m, marked)
	tree, err := m.MkAnd(p, marked)
	interpMust(t, err)
	tree = interpPin(t, m, tree)
	var out bytes.Buffer
	interpMust(t, PrettyPrintProblem(m, []ast.ASTHandle{p, q}, tree, &out))
	want := "(set-option :produce-interpolants true)\n" +
		"(declare-fun p () Bool)\n(declare-fun q () Bool)\n" +
		"(assert (! p :named f!1))\n(assert (! q :named f!2))\n" +
		"(check-sat)\n(get-interpolant (and f!1 (interp f!2)))\n"
	if out.String() != want {
		t.Fatalf("pretty interpolation problem:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestInterpolationPrettyPrintDuplicateAndBadTree(t *testing.T) {
	m := interpASTFixture(t)
	p, err := m.MkTrue()
	interpMust(t, err)
	p = interpPin(t, m, p)
	var out bytes.Buffer
	interpMust(t, PrettyPrintProblem(m, []ast.ASTHandle{p, p}, p, &out))
	if !strings.HasSuffix(out.String(), "(get-interpolant f!2)\n") {
		t.Fatal(out.String())
	}
	bad, err := m.MkNot(p)
	interpMust(t, err)
	bad = interpPin(t, m, bad)
	out.Reset()
	err = PrettyPrintProblem(m, []ast.ASTHandle{p}, bad, &out)
	var classified BadTreeError
	if !errors.As(err, &classified) || classified.Error() != "iz3pp_bad_tree" {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.String(), "(get-interpolant ") {
		t.Fatal("source prefix before bad tree was not preserved", out.String())
	}
}

// AUDIT.md interp_02-F11: iz3pp collects declarations with
// free_func_visitor into a func_decl_set (hash-table order), prints them
// with mk_smt_pp plus "\n", and names every constraint before printing,
// so a duplicate constraint uses its last name. The expected text is what
// the C++ Z3_write_interpolation_problem writes for the same problems.
func TestInterpolationPrettyPrintSourceDeclarationsAndNames(t *testing.T) {
	m := interpASTFixture(t)
	intern := func(name string) symbol.Symbol {
		s, err := m.Symbols.Intern(name)
		interpMust(t, err)
		return s
	}
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		interpMust(t, err)
		return interpPin(t, m, h)
	}
	// A C API context has already registered one user-sort name when the
	// harness declares U, so U is user-sort kind 1 there; match its hash.
	pin(m.MkUninterpretedSort(intern("pp-user-sort-0"), nil))
	u := pin(m.MkUninterpretedSort(intern("U"), nil))
	boolSort := pin(m.MkBoolSort())
	f := pin(m.MkFuncDecl(intern("f"), []ast.ASTHandle{u}, u, ast.ASTFuncDeclInfo{}))
	p := pin(m.MkFuncDecl(intern("p"), []ast.ASTHandle{u}, boolSort, ast.ASTFuncDeclInfo{}))
	a := pin(m.MkConst(intern("a"), u))
	b := pin(m.MkConst(intern("b"), u))
	fa := pin(m.MkApp(f, []ast.ASTHandle{a}))
	fb := pin(m.MkApp(f, []ast.ASTHandle{b}))
	first := pin(m.MkEq(fa, b))
	second := pin(m.MkNot(pin(m.MkEq(fb, a))))
	third := pin(m.MkApp(p, []ast.ASTHandle{a}))
	inner := pin(m.MkAnd(pin(m.MkInterp(first)), second))
	tree := pin(m.MkAnd(pin(m.MkInterp(inner)), third))
	var out bytes.Buffer
	interpMust(t, PrettyPrintProblem(m, []ast.ASTHandle{first, second, third}, tree, &out))
	want := "(set-option :produce-interpolants true)\n" +
		"(declare-fun p (U) Bool)\n(declare-fun b () U)\n(declare-fun a () U)\n(declare-fun f (U) U)\n" +
		"(assert (! (= (f a) b) :named f!1))\n(assert (! (not (= (f b) a)) :named f!2))\n(assert (! (p a) :named f!3))\n" +
		"(check-sat)\n(get-interpolant (and (interp (and (interp f!1) f!2)) f!3))\n"
	if out.String() != want {
		t.Fatalf("pretty interpolation problem:\n%s\nwant:\n%s", out.String(), want)
	}
	truth := pin(m.MkTrue())
	duplicate := pin(m.MkAnd(pin(m.MkInterp(truth)), truth))
	out.Reset()
	interpMust(t, PrettyPrintProblem(m, []ast.ASTHandle{truth, truth}, duplicate, &out))
	want = "(set-option :produce-interpolants true)\n" +
		"(assert (! true :named f!2))\n(assert (! true :named f!2))\n" +
		"(check-sat)\n(get-interpolant (and (interp f!2) f!2))\n"
	if out.String() != want {
		t.Fatalf("duplicate constraint names:\n%s\nwant:\n%s", out.String(), want)
	}
}
