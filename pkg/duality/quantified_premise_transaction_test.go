package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifiedPremiseFailedMatchRollsBackBindings(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
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
	zero := raw(wrap(m.ArithmeticIntegerNumeralText("0")))
	one := raw(wrap(m.ArithmeticIntegerNumeralText("1")))
	two := raw(wrap(m.ArithmeticIntegerNumeralText("2")))
	sum := raw(wrap(m.MkArithmeticAdd(one, two)))
	integer, err := m.GetSort(sum)
	must(err)
	boolean, err := m.GetSort(truth)
	must(err)
	name, err := m.Symbols.Intern("quantified-transaction-P")
	must(err)
	pDecl, err := m.MkFuncDecl(name, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	targetVariable := raw(wrap(m.MkVar(0, integer)))
	targetBody := raw(wrap(m.MkApp(pDecl, []ast.ASTHandle{targetVariable, sum})))
	targetForall := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer},
		Names:  []usymbol.Symbol{usymbol.Numerical(0)},
		Body:   targetBody,
	})))
	target := raw(wrap(m.MkEq(targetForall, truth)))

	c := raw(wrap(m.MkVar(0, integer)))
	a := raw(wrap(m.MkVar(1, integer)))
	b := raw(wrap(m.MkVar(2, integer)))
	sourceHead := raw(wrap(m.MkApp(pDecl, []ast.ASTHandle{b, a})))
	cPlusZero := raw(wrap(m.MkArithmeticAdd(c, zero)))
	first := raw(wrap(m.MkEq(a, cPlusZero)))
	second := raw(wrap(m.MkEq(c, a)))
	guard := raw(wrap(m.MkAndArgs([]ast.ASTHandle{first, second})))
	body := raw(wrap(m.MkImplies(guard, sourceHead)))
	definition := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer, integer, integer},
		Names:  []usymbol.Symbol{usymbol.Numerical(0), usymbol.Numerical(1), usymbol.Numerical(2)},
		Body:   body,
	})))

	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	definitionRef, err := m.NewRef(definition)
	must(err)
	graph.sourceClauses = append(graph.sourceClauses, definitionRef)
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer func() { must(factory.Close()) }()
	var parents []ast.ASTRef
	proof, err := graph.proveQuantifiedHornLiteral(factory, graph, -1, target, &parents)
	must(err)
	defer func() {
		must(m.DelRef(&proof))
		for i := range parents {
			must(m.DelRef(&parents[i]))
		}
	}()
	if proof == (ast.ASTRef{}) {
		t.Fatal("failed structural match hid a later valid witness equation")
	}
	h, err := m.RefGet(proof)
	must(err)
	checker, err := proofs.NewChecker(m)
	must(err)
	valid, side, err := checker.Check(h)
	must(err)
	defer func() { must(checker.ReleaseSideConditions(side)) }()
	if !valid {
		t.Fatal("transactional witness proof is structurally invalid")
	}
	closed, err := proofs.IsClosed(m, h)
	must(err)
	if !closed {
		t.Fatal("transactional witness proof is open")
	}
	oracle, err := factory.New(m, nil, false, false, false, "")
	must(err)
	defer func() { must(oracle.Close()) }()
	for i, condition := range side {
		must(oracle.Push())
		negated, err := m.MkNot(condition)
		must(err)
		must(oracle.Assert(negated))
		status, err := oracle.CheckSat(nil)
		must(err)
		must(oracle.Pop(1))
		if status != lbool.False {
			t.Fatal("transactional witness side condition", i, status)
		}
	}
	seenDefinition := false
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{h}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		asserted, err := m.IsProofKind(current, ast.ASTPrAsserted)
		must(err)
		if asserted {
			fact, err := m.GetProofFact(current)
			must(err)
			if fact != definition {
				t.Fatal("transactional proof fabricated an asserted leaf")
			}
			seenDefinition = true
		}
		n, err := m.GetProofNumParents(current)
		must(err)
		for i := uint32(0); i < n; i++ {
			parent, err := m.GetProofParent(current, i)
			must(err)
			stack = append(stack, parent)
		}
	}
	if !seenDefinition {
		t.Fatal("source definition is absent from transactional proof")
	}
}
