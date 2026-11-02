package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestTabSessionPersistsSelectionDecayCacheAndSequenceAcrossQueries(t *testing.T) {
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
	querySolver, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(querySolver.Close()) }()
	subSolver, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(subSolver.Close()) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	integer, err := m.GetSort(one)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	decl := func(text string) ast.ASTHandle {
		name, e := m.Symbols.Intern(text)
		must(e)
		value, e := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		return value
	}
	p, q := decl("session-p"), decl("session-q")
	app := func(d, value ast.ASTHandle) ast.ASTHandle {
		result, e := m.MkApp(d, []ast.ASTHandle{value})
		must(e)
		return result
	}
	x, err := m.MkVar(0, integer)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	falsehood, err := m.MkFalse()
	must(err)
	step, err := NewClause(m, rw, app(p, x), []ast.ASTHandle{app(q, x)}, truth)
	must(err)
	defer func() { must(step.Close()) }()
	fact, err := NewClause(m, rw, app(q, one), nil, truth)
	must(err)
	defer func() { must(fact.Close()) }()
	rules := &RuleIndex{manager: m, clauses: []*Clause{step, fact}, byHead: map[ast.ASTHandle][]int{p: {0}, q: {1}}}
	session, err := NewSession(m, "weight")
	must(err)
	defer func() { must(session.Close()) }()
	queryApplication := app(p, one)
	previousCache := 0
	for queryIndex := uint32(0); queryIndex < 2; queryIndex++ {
		query, e := NewClause(m, rw, falsehood, []ast.ASTHandle{queryApplication}, truth)
		must(e)
		search, e := newTabSearchWithSession(query, rules, rw, lite, querySolver, subSolver, "first", session)
		if e != nil {
			_ = query.Close()
			t.Fatal(e)
		}
		status, e := search.Run()
		must(e)
		if status != lbool.True {
			t.Fatal("session query status", queryIndex, status)
		}
		if search.stack[0].sequence != queryIndex*3 || search.stack[len(search.stack)-1].sequence != queryIndex*3+2 {
			t.Fatal("sequence did not continue across queries", queryIndex, search.stack[0].sequence, search.stack[len(search.stack)-1].sequence)
		}
		must(search.Close())
		if session.nextSequence != (queryIndex+1)*3 || session.selection.next != 20-(queryIndex+1)*2 {
			t.Fatal("persistent session state", session.nextSequence, session.selection.next)
		}
		if queryIndex == 0 {
			previousCache = len(session.selection.cache)
			if previousCache == 0 || len(session.selection.refs) != previousCache {
				t.Fatal("first query did not populate selection cache")
			}
		} else if len(session.selection.cache) != previousCache || len(session.selection.refs) != previousCache {
			t.Fatal("second query rebuilt persistent selection cache")
		}
	}
}
