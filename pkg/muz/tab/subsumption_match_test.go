package tab

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type countingTabSolver struct {
	solver.Solver
	checks uint32
}

func (s *countingTabSolver) CheckSat(assumptions []ast.ASTHandle) (lbool.LBool, error) {
	s.checks++
	return s.Solver.CheckSat(assumptions)
}

func TestClauseSubsumptionBacktracksPredicateMatches(t *testing.T) {
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

func TestClauseSubsumptionCachesFalseNormalizedLiterals(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := newTabTestContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m := ctx.manager
	rw, err := rewriter.NewTheoryRewriter(m, ctx.params, ctx.globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	coop, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer func() { must(coop.Close()) }()
	lite, err := qe.NewLite(m, nil, false, qe.Config{Context: ctx.params, Globals: ctx.globals, Cooperation: coop})
	must(err)
	defer func() { must(lite.Close()) }()
	services := model.ModelEvaluatorConfig{Context: ctx.params, Globals: ctx.globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: coop}
	backend, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: ctx.params, Globals: ctx.globals, ModelEvaluator: services})
	must(err)
	must(backend.SetProduceModels(true))
	counted := &countingTabSolver{Solver: backend}
	defer func() { must(counted.Close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("cache-false-literal-value")
	must(err)
	value, err := m.MkConst(name, integer)
	must(err)
	name, err = m.Symbols.Intern("cache-false-literal-head")
	must(err)
	headDecl, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	head, err := m.MkApp(headDecl, []ast.ASTHandle{value})
	must(err)
	truth, err := m.MkTrue()
	must(err)
	query, err := NewClause(m, rw, head, nil, truth)
	must(err)
	defer func() { must(query.Close()) }()
	forward, err := m.MkEq(value, zero)
	must(err)
	backward, err := m.MkEq(zero, value)
	must(err)
	first, err := NewClause(m, rw, head, nil, forward)
	must(err)
	defer func() { must(first.Close()) }()
	second, err := NewClause(m, rw, head, nil, backward)
	must(err)
	defer func() { must(second.Close()) }()
	found, _, err := isClauseSubsumed(query, []*Clause{first, second}, lite, counted, &services)
	must(err)
	if found {
		t.Fatal("inconsistent equality candidates unexpectedly subsume the query")
	}
	if counted.checks != 1 {
		t.Fatalf("solver checks=%d, want one check before the cached normalized literal rejects the second candidate", counted.checks)
	}
}
