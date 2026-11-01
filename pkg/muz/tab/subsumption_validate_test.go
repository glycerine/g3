package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

func TestSubsumptionValidationUsesPreconditionAndRestoresSolverScope(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := newTabTestContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	globals, err := ctx.GlobalParameters()
	must(err)
	rw, err := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	coop, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer func() { must(coop.Close()) }()
	lite, err := qe.NewLite(m, nil, false, qe.Config{Context: globals.Context, Globals: globals, Cooperation: coop})
	must(err)
	defer func() { must(lite.Close()) }()
	smt, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(smt.Close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	integer, err := m.GetSort(one)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	name, err := m.Symbols.Intern("subsumption-a")
	must(err)
	a, err := m.MkConst(name, integer)
	must(err)
	name, err = m.Symbols.Intern("subsumption-head")
	must(err)
	decl, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	head, err := m.MkApp(decl, []ast.ASTHandle{a})
	must(err)
	family, err := m.ArithmeticFamilyID()
	must(err)
	app := func(op int32, left, right ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{left, right})
		must(e)
		return h
	}
	must(smt.Assert(app(2, a, one)))
	must(smt.Assert(app(3, a, one)))
	good, err := NewClause(m, rw, head, nil, app(2, x, two))
	must(err)
	defer func() { must(good.Close()) }()
	bad, err := NewClause(m, rw, head, nil, app(2, x, zero))
	must(err)
	defer func() { must(bad.Close()) }()
	bindings, err := substitution.New(m)
	must(err)
	defer func() { must(bindings.Close()) }()
	must(bindings.Reserve(2, 1))
	must(bindings.Insert(0, 0, substitution.NewExprOffset(a, 1)))
	for _, tc := range []struct {
		clause *Clause
		want   bool
	}{{good, true}, {bad, false}, {good, true}} {
		got, e := validateSubsumption(m, tc.clause, bindings, nil, lite, smt, nil, nil)
		must(e)
		if got != tc.want {
			t.Fatalf("valid=%v want %v", got, tc.want)
		}
		level, e := smt.ScopeLevel()
		must(e)
		if level != 0 {
			t.Fatalf("solver scope leaked: %d", level)
		}
	}
}

func TestSubsumptionLiteralNormalizationOrdersEqualityArguments(t *testing.T) {
	ctx, err := newTabTestContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ctx.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	m, err := ctx.Manager()
	if err != nil {
		t.Fatal(err)
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		t.Fatal(err)
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	if err != nil {
		t.Fatal(err)
	}
	forward, err := m.MkEq(zero, one)
	if err != nil {
		t.Fatal(err)
	}
	backward, err := m.MkEq(one, zero)
	if err != nil {
		t.Fatal(err)
	}
	got, err := normalizeSubsumptionLiteral(m, backward)
	if err != nil {
		t.Fatal(err)
	}
	want, err := normalizeSubsumptionLiteral(m, forward)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("normalized equality handles differ: got %v want %v", got, want)
	}
}
