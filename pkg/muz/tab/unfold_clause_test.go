package tab

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"testing"
)

func TestUnfoldClauseExpressionsUnifiesBanksAndOrdersPredicates(t *testing.T) {
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
	globals, err := ctx.GlobalParameters()
	must(err)
	rw, err := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
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
	makeDecl := func(name string) ast.ASTHandle {
		sym, e := m.Symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		return d
	}
	goal, p, q, s, r := makeDecl("goal"), makeDecl("p"), makeDecl("q"), makeDecl("s"), makeDecl("r")
	app := func(d, a ast.ASTHandle) ast.ASTHandle { h, e := m.MkApp(d, []ast.ASTHandle{a}); must(e); return h }
	family, err := m.ArithmeticFamilyID()
	must(err)
	bound := func(op int32, a, b ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{a, b})
		must(e)
		return h
	}
	target, err := NewClause(m, rw, app(goal, x), []ast.ASTHandle{app(p, x), app(r, x)}, bound(3, x, one))
	must(err)
	defer func() { must(target.Close()) }()
	source, err := NewClause(m, rw, app(p, x), []ast.ASTHandle{app(q, x), app(s, x)}, bound(2, x, two))
	must(err)
	defer func() { must(source.Close()) }()
	result, matched, err := unfoldClauseExpressionsWithBindings(target, 0, source, true)
	must(err)
	if !matched || result == nil {
		t.Fatal("unification failed")
	}
	defer func() { must(result.Close()) }()
	head, err := m.RefGet(result.head)
	must(err)
	variable, err := m.GetArg(head, 0)
	must(err)
	if len(result.predicates) != 3 {
		t.Fatalf("predicate count %d", len(result.predicates))
	}
	for i, want := range []ast.ASTHandle{q, s, r} {
		h, e := m.RefGet(result.predicates[i])
		must(e)
		d, e := m.GetDecl(h)
		must(e)
		a, e := m.GetArg(h, 0)
		must(e)
		if d != want || a != variable {
			t.Fatalf("predicate %d has wrong declaration or binding", i)
		}
	}
	condition, err := m.RefGet(result.constraint)
	must(err)
	parts, err := m.FlattenAnd([]ast.ASTHandle{condition})
	must(err)
	if len(parts) != 2 {
		t.Fatalf("constraints %d", len(parts))
	}
	for _, part := range parts {
		a, e := m.GetArg(part, 0)
		must(e)
		if a != variable {
			t.Fatal("constraint lost unification")
		}
	}
	cooperation, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer func() { must(cooperation.Close()) }()
	lite, err := qe.NewLite(m, nil, false, qe.Config{Context: globals.Context, Globals: globals, Cooperation: cooperation})
	must(err)
	defer func() { must(lite.Close()) }()
	final, matched, err := finalizeUnfoldedClause(result, rw, lite)
	must(err)
	if !matched || final == nil {
		t.Fatal("finalization rejected a satisfiable unfolding")
	}
	defer func() { must(final.Close()) }()
	finalPreds, err := final.Predicates()
	must(err)
	if len(finalPreds) != 3 {
		t.Fatal("finalization lost predicates")
	}
	substitutions, err := result.proofSubstitutions(target, source)
	must(err)
	defer func() { must(substitutions.Close()) }()
	finalHead, err := final.Head()
	must(err)
	finalVariable, err := m.GetArg(finalHead, 0)
	must(err)
	for _, bank := range substitutions.banks {
		if len(bank) != 1 {
			t.Fatal("wrong proof substitution size")
		}
		value, e := m.RefGet(bank[0])
		must(e)
		if value != finalVariable {
			t.Fatal("proof substitution disagrees with unfolded clause")
		}
	}
	truth, err := m.MkTrue()
	must(err)
	mismatch, err := NewClause(m, rw, app(p, two), nil, truth)
	must(err)
	defer func() { must(mismatch.Close()) }()
	grounded, err := NewClause(m, rw, app(goal, one), []ast.ASTHandle{app(p, one)}, truth)
	must(err)
	defer func() { must(grounded.Close()) }()
	absent, matched, err := unfoldClauseExpressions(grounded, 0, mismatch)
	must(err)
	if matched || absent != nil {
		t.Fatal("incompatible numerals unified")
	}
}
