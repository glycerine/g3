package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityQuantifiedRelationPremiseIsTheoryObligation(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle {
		t.Helper()
		h, err := e.Raw()
		must(err)
		return h
	}
	truth := raw(wrap(m.MkTrue()))
	boolean, err := m.GetSort(truth)
	must(err)
	seven := raw(wrap(m.ArithmeticIntegerNumeralText("7")))
	integer, err := m.GetSort(seven)
	must(err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		symbol, err := m.Symbols.Intern(name)
		must(err)
		decl, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		return decl
	}
	seedDecl := declare("quantified-premise-Seed", nil)
	midDecl := declare("quantified-premise-Mid", nil)
	pDecl := declare("quantified-premise-P", []ast.ASTHandle{integer, integer})
	goalDecl := declare("quantified-premise-Goal", nil)
	seed := raw(wrap(m.MkApp(seedDecl, nil)))
	mid := raw(wrap(m.MkApp(midDecl, nil)))
	goal := raw(wrap(m.MkApp(goalDecl, nil)))
	variable := raw(wrap(m.MkVar(0, integer)))
	pv := raw(wrap(m.MkApp(pDecl, []ast.ASTHandle{variable, seven})))
	binder, err := m.Symbols.Intern("quantified-premise-index")
	must(err)
	forall := func(body ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		return raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{binder}, Body: body})))
	}
	seedToMid := raw(wrap(m.MkImplies(seed, mid)))
	midToP := forall(raw(wrap(m.MkImplies(mid, pv))))
	quantifiedP := forall(pv)
	wrapper := raw(wrap(m.MkEq(quantifiedP, truth)))
	goalRule := raw(wrap(m.MkImplies(wrapper, goal)))
	falsity := raw(wrap(m.MkFalse()))
	queryRule := raw(wrap(m.MkImplies(goal, falsity)))
	clauses := []*Expr{wrap(seed, nil), wrap(seedToMid, nil), wrap(midToP, nil), wrap(goalRule, nil), wrap(queryRule, nil)}

	evaluator := derivationEvaluatorFixture(t, ctx, globals)
	quantifiers, err := qe.NewLite(m, nil, true, qe.Config{Context: ctx.config.params, Globals: globals, Cooperation: evaluator.Cooperation})
	must(err)
	defer quantifiers.Close()
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	auxiliary, err := factory.New(m, nil, false, true, false, "")
	must(err)
	defer auxiliary.Close()
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	must(graph.FromClauses(clauses, nil, simplifier, quantifiers))
	var goalEdge *Edge
	for _, edge := range graph.edges {
		name, err := m.RefGet(edge.Parent.Name)
		must(err)
		if name == goalDecl {
			goalEdge = edge
			break
		}
	}
	if goalEdge == nil || len(goalEdge.Children) != 0 || len(goalEdge.F.relationParams) != 0 {
		t.Fatal("quantified predicate was represented as a point-instance child")
	}
	engine, err := NewEngine(graph, EngineConfig{Native: native, Auxiliary: auxiliary, Factory: factory, Evaluator: evaluator, Simplifier: simplifier, Quantifiers: quantifiers})
	must(err)
	defer engine.Close()
	safe, err := engine.Solve()
	must(err)
	if safe || engine.GetCounterexample().Root() == nil {
		t.Fatal("reachable quantified child was not derived", safe)
	}
	proof, err := engine.GetCounterexample().Proof(factory)
	must(err)
	defer func() { must(m.DelRef(&proof)) }()
	proofHandle, err := m.RefGet(proof)
	must(err)
	checker, err := proofs.NewChecker(m)
	must(err)
	valid, side, err := checker.Check(proofHandle)
	must(err)
	defer func() { must(checker.ReleaseSideConditions(side)) }()
	if !valid {
		t.Fatal("quantified premise proof is structurally invalid")
	}
	closed, err := proofs.IsClosed(m, proofHandle)
	must(err)
	if !closed {
		t.Fatal("quantified premise proof has an escaping hypothesis")
	}
	oracle, err := factory.New(m, nil, false, false, false, "")
	must(err)
	defer oracle.Close()
	for i, condition := range side {
		must(oracle.Push())
		negated, err := m.MkNot(condition)
		must(err)
		must(oracle.Assert(negated))
		status, err := oracle.CheckSat(nil)
		must(err)
		must(oracle.Pop(1))
		if status != lbool.False {
			t.Fatal("invalid quantified-premise side condition", i, status)
		}
	}
	sources := make(map[ast.ASTHandle]bool, len(clauses))
	for _, clause := range clauses {
		sources[raw(clause)] = true
	}
	seen := make(map[ast.ASTHandle]bool)
	assertedSeen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{proofHandle}
	for len(stack) > 0 {
		i := len(stack) - 1
		current := stack[i]
		stack = stack[:i]
		if seen[current] {
			continue
		}
		seen[current] = true
		asserted, err := m.IsProofKind(current, ast.ASTPrAsserted)
		must(err)
		if asserted {
			fact, err := m.GetProofFact(current)
			must(err)
			if !sources[fact] {
				t.Fatal("proof contains an asserted leaf outside transformed source clauses")
			}
			assertedSeen[fact] = true
		}
		parents, err := m.GetProofNumParents(current)
		must(err)
		for j := uint32(0); j < parents; j++ {
			parent, err := m.GetProofParent(current, j)
			must(err)
			stack = append(stack, parent)
		}
	}
	for source := range sources {
		if !assertedSeen[source] {
			t.Fatal("proof omitted a transitive source clause", source)
		}
	}
}

func TestQuantifiedProgramPremisesCloseCyclesInSourceOrder(t *testing.T) {
	ctx, m, _, _ := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		expression, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, expression)
		return expression
	}
	raw := func(expression *Expr) ast.ASTHandle {
		t.Helper()
		h, err := expression.Raw()
		must(err)
		return h
	}
	truth := raw(wrap(m.MkTrue()))
	boolean, err := m.GetSort(truth)
	must(err)
	zero := raw(wrap(m.ArithmeticIntegerNumeralText("0")))
	integer, err := m.GetSort(zero)
	must(err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(name)
		must(err)
		declaration, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		return declaration
	}
	seedDeclaration := declare("quantified-cycle-Seed", nil)
	midDeclaration := declare("quantified-cycle-Mid", nil)
	loopDeclaration := declare("quantified-cycle-Loop", nil)
	pDeclaration := declare("quantified-cycle-P", []ast.ASTHandle{integer})
	goalDeclaration := declare("quantified-cycle-Goal", nil)
	seed := raw(wrap(m.MkApp(seedDeclaration, nil)))
	mid := raw(wrap(m.MkApp(midDeclaration, nil)))
	loop := raw(wrap(m.MkApp(loopDeclaration, nil)))
	goal := raw(wrap(m.MkApp(goalDeclaration, nil)))
	variable := raw(wrap(m.MkVar(0, integer)))
	pVariable := raw(wrap(m.MkApp(pDeclaration, []ast.ASTHandle{variable})))
	binder, err := m.Symbols.Intern("quantified-cycle-index")
	must(err)
	forallP := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{binder}, Body: pVariable,
	})))
	midToPBody := raw(wrap(m.MkImplies(mid, pVariable)))
	midToP := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{binder}, Body: midToPBody,
	})))
	formulas := []ast.ASTHandle{
		seed,
		raw(wrap(m.MkImplies(seed, mid))),
		raw(wrap(m.MkImplies(loop, mid))),
		raw(wrap(m.MkImplies(mid, loop))),
		midToP,
		raw(wrap(m.MkImplies(forallP, goal))),
	}
	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	for _, formula := range formulas {
		ref, err := m.NewRef(formula)
		must(err)
		graph.sourceClauses = append(graph.sourceClauses, ref)
	}
	indices, err := graph.proofProgramPremiseIndices(forallP, len(formulas)-1)
	must(err)
	if len(indices) != len(formulas)-1 {
		t.Fatal("transitive cycle premise count", indices)
	}
	for i, index := range indices {
		if index != i {
			t.Fatal("transitive cycle premises lost source order", indices)
		}
	}
}

func TestQuantifiedPremiseSpecializationRequiresRelationParent(t *testing.T) {
	ctx, m, _, _ := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		e, err := NewExpr(ctx, h)
		must(err)
		closeExpr(t, e)
		return e
	}
	raw := func(e *Expr) ast.ASTHandle {
		t.Helper()
		h, err := e.Raw()
		must(err)
		return h
	}

	truth := raw(wrap(m.MkTrue()))
	boolean, err := m.GetSort(truth)
	must(err)
	zero := raw(wrap(m.ArithmeticIntegerNumeralText("0")))
	integer, err := m.GetSort(zero)
	must(err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		symbol, err := m.Symbols.Intern(name)
		must(err)
		decl, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		return decl
	}
	seedDecl := declare("quantified-parent-Seed", nil)
	pDecl := declare("quantified-parent-P", []ast.ASTHandle{integer})
	seed := raw(wrap(m.MkApp(seedDecl, nil)))
	variable := raw(wrap(m.MkVar(0, integer)))
	head := raw(wrap(m.MkApp(pDecl, []ast.ASTHandle{variable})))
	body := raw(wrap(m.MkImplies(seed, head)))
	definition := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer},
		Names:  []usymbol.Symbol{usymbol.Numerical(0)},
		Body:   body,
	})))

	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	for _, formula := range []ast.ASTHandle{seed, definition} {
		ref, err := m.NewRef(formula)
		must(err)
		graph.sourceClauses = append(graph.sourceClauses, ref)
	}
	supported, err := graph.structuralQuantifiedDefinition(body)
	must(err)
	if supported {
		t.Fatal("relation guard was accepted without its recursive proof parent")
	}
}
