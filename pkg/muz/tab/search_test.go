package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestTabSearchUnfoldsRulesAndBacktracks(t *testing.T) {
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
	querySolver, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(querySolver.Close()) }()
	subSolver, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(subSolver.Close()) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	intSort, err := m.GetSort(one)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	makeDecl := func(name string) ast.ASTHandle {
		sym, e := m.Symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(sym, []ast.ASTHandle{intSort}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		return d
	}
	p, q := makeDecl("search-p"), makeDecl("search-q")
	app := func(d, a ast.ASTHandle) ast.ASTHandle { h, e := m.MkApp(d, []ast.ASTHandle{a}); must(e); return h }
	x, err := m.MkVar(0, intSort)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	falsehood, err := m.MkFalse()
	must(err)
	rule1, err := NewClause(m, rw, app(p, x), []ast.ASTHandle{app(q, x)}, truth)
	must(err)
	defer func() { must(rule1.Close()) }()
	rule2, err := NewClause(m, rw, app(q, one), nil, truth)
	must(err)
	defer func() { must(rule2.Close()) }()
	rules := &RuleIndex{manager: m, clauses: []*Clause{rule1, rule2}, byHead: map[ast.ASTHandle][]int{p: {0}, q: {1}}}
	for _, tc := range []struct {
		term ast.ASTHandle
		want lbool.LBool
	}{{one, lbool.True}, {two, lbool.False}} {
		query, err := NewClause(m, rw, falsehood, []ast.ASTHandle{app(p, tc.term)}, truth)
		must(err)
		search, err := newTabSearch(query, rules, rw, lite, querySolver, subSolver, "first")
		if err != nil {
			_ = query.Close()
			t.Fatal(err)
		}
		status, err := search.Run()
		if err != nil {
			_ = search.Close()
			t.Fatal(err)
		}
		if status != tc.want {
			_ = search.Close()
			t.Fatalf("status %v want %v", status, tc.want)
		}
		if status == lbool.True && (search.unfold != 2 || len(search.stack) != 3 || search.stack[2].parentIndex != 1) {
			_ = search.Close()
			t.Fatal("unfolded path was not recorded")
		}
		if status == lbool.True {
			proof, e := search.Proof()
			must(e)
			proofHandle, e := m.RefGet(proof)
			must(e)
			proofKind, e := m.GetDeclKind(proofHandle)
			must(e)
			if proofKind != ast.ASTPrHyperResolve {
				t.Fatal("winning path did not produce a hyper-resolution proof", proofKind)
			}
			fact, e := m.GetProofFact(proofHandle)
			must(e)
			leaf, e := search.stack[len(search.stack)-1].Formula()
			must(e)
			leafFormula, e := m.RefGet(leaf)
			must(e)
			if fact != leafFormula {
				t.Fatal("proof conclusion does not match the final clause")
			}
			must(m.DelRef(&leaf))
			must(m.DelRef(&proof))
		}
		if status == lbool.False && search.unfoldFail == 0 {
			_ = search.Close()
			t.Fatal("expected failed rule match")
		}
		must(search.Close())
	}
	cycleRule, err := NewClause(m, rw, app(p, x), []ast.ASTHandle{app(p, x)}, truth)
	must(err)
	defer func() { must(cycleRule.Close()) }()
	cycleRules := &RuleIndex{manager: m, clauses: []*Clause{cycleRule}, byHead: map[ast.ASTHandle][]int{p: {0}}}
	cycleQuery, err := NewClause(m, rw, falsehood, []ast.ASTHandle{app(p, one)}, truth)
	must(err)
	cycleSearch, err := newTabSearch(cycleQuery, cycleRules, rw, lite, querySolver, subSolver, "first")
	must(err)
	status, err := cycleSearch.Run()
	must(err)
	if status != lbool.False || cycleSearch.subsumed == 0 {
		t.Fatal("recursive goal was not stopped by subsumption", status, cycleSearch.subsumed)
	}
	must(cycleSearch.Close())
	a, err := querySolver.ScopeLevel()
	must(err)
	b, err := subSolver.ScopeLevel()
	must(err)
	if a != 0 || b != 0 {
		t.Fatal("solver scope leaked", a, b)
	}
}
