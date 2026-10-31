package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

func TestClauseSubsumptionBacktracksPredicateMatches(t *testing.T) {
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
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	sym := func(name string) ast.ASTHandle {
		s, e := m.Symbols.Intern(name)
		must(e)
		h, e := m.MkConst(s, integer)
		must(e)
		return h
	}
	a, b := sym("match-a"), sym("match-b")
	decl := func(name string) ast.ASTHandle {
		s, e := m.Symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(s, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		return d
	}
	headDecl, predDecl := decl("match-head"), decl("match-predicate")
	app := func(d, x ast.ASTHandle) ast.ASTHandle { h, e := m.MkApp(d, []ast.ASTHandle{x}); must(e); return h }
	family, err := m.ArithmeticFamilyID()
	must(err)
	cmp := func(op int32, x, y ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{x, y})
		must(e)
		return h
	}
	pre, err := m.MkAnd(cmp(3, a, one), cmp(2, b, zero))
	must(err)
	head := app(headDecl, zero)
	query, err := NewClause(m, rw, head, []ast.ASTHandle{app(predDecl, a), app(predDecl, b)}, pre)
	must(err)
	defer func() { must(query.Close()) }()
	x, err := m.MkVar(0, integer)
	must(err)
	candidate, err := NewClause(m, rw, head, []ast.ASTHandle{app(predDecl, x)}, cmp(2, x, zero))
	must(err)
	candidate.sequence = 37
	defer func() { must(candidate.Close()) }()
	found, seq, err := isClauseSubsumed(query, []*Clause{candidate}, lite, smt)
	must(err)
	if !found || seq != 37 {
		t.Fatalf("subsumption=%v seq=%d", found, seq)
	}
	currentPre, err := query.Constraint()
	must(err)
	onlyFirst, err := NewClause(m, rw, head, []ast.ASTHandle{app(predDecl, a)}, currentPre)
	must(err)
	defer func() { must(onlyFirst.Close()) }()
	found, _, err = isClauseSubsumed(onlyFirst, []*Clause{candidate}, lite, smt)
	must(err)
	if found {
		t.Fatal("failed implication accepted")
	}
	level, err := smt.ScopeLevel()
	must(err)
	if level != 0 {
		t.Fatalf("scope leak %d", level)
	}
}
