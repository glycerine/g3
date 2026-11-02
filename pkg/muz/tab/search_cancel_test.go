package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestTabSearchCancellationImmediatelyCleansActiveClauses(t *testing.T) {
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
	subsumptionSolver, err := ctx.NewSimpleSolver()
	must(err)
	defer func() { must(subsumptionSolver.Close()) }()

	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("tab-cancel-p")
	must(err)
	predicate, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	application, err := m.MkApp(predicate, nil)
	must(err)
	falsehood, err := m.MkFalse()
	must(err)
	truth, err := m.MkTrue()
	must(err)
	query, err := NewClause(m, rw, falsehood, []ast.ASTHandle{application}, truth)
	must(err)
	rules := &RuleIndex{manager: m, byHead: make(map[ast.ASTHandle][]int)}
	search, err := newTabSearch(query, rules, rw, lite, querySolver, subsumptionSolver, "first")
	must(err)

	m.ResourceLimit().Cancel()
	status, err := search.Run()
	m.ResourceLimit().ResetCancel()
	must(err)
	if status != lbool.Undef {
		t.Fatal("canceled search status", status)
	}
	if !query.closed || len(search.stack) != 0 || len(search.index) != 0 || len(search.owned) != 0 {
		t.Fatal("canceled search kept active clauses", query.closed, len(search.stack), len(search.index), len(search.owned))
	}
	if search.closed || search.selection == nil {
		t.Fatal("clause cleanup disposed reusable search services")
	}
	must(search.Close())
	if !search.closed || search.selection != nil {
		t.Fatal("final search disposal incomplete")
	}
}
