package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifiedPremiseReconstructsTheoryGuard(t *testing.T) {
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
	seven := raw(wrap(m.ArithmeticIntegerNumeralText("7")))
	integer, err := m.GetSort(seven)
	must(err)
	boolean, err := m.GetSort(truth)
	must(err)
	pName, err := m.Symbols.Intern("quantified-theory-P")
	must(err)
	pDecl, err := m.MkFuncDecl(pName, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)

	targetVariable := raw(wrap(m.MkVar(0, integer)))
	targetBody := raw(wrap(m.MkApp(pDecl, []ast.ASTHandle{targetVariable, seven})))
	targetForall := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer},
		Names:  []usymbol.Symbol{usymbol.Numerical(0)},
		Body:   targetBody,
	})))
	target := raw(wrap(m.MkEq(targetForall, truth)))

	booleanWitness := raw(wrap(m.MkVar(0, boolean)))
	witness := raw(wrap(m.MkVar(1, integer)))
	constantArgument := raw(wrap(m.MkVar(2, integer)))
	universalArgument := raw(wrap(m.MkVar(3, integer)))
	sourceHead := raw(wrap(m.MkApp(pDecl, []ast.ASTHandle{universalArgument, constantArgument})))
	witnessEquivalence := raw(wrap(m.MkIff(booleanWitness, truth)))
	if op, err := m.GetDeclKind(witnessEquivalence); err != nil || op != ast.ASTOpIff {
		t.Fatal("Boolean witness guard is not an equivalence", op, err)
	}
	witnessEquation := raw(wrap(m.MkEq(witness, constantArgument)))
	theoryGuard := raw(wrap(m.MkArithmeticLE(witness, constantArgument)))
	antecedent := raw(wrap(m.MkAndArgs([]ast.ASTHandle{witnessEquivalence, witnessEquation, theoryGuard})))
	definitionBody := raw(wrap(m.MkImplies(antecedent, sourceHead)))
	definition := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer, integer, integer, boolean},
		Names: []usymbol.Symbol{
			usymbol.Numerical(0),
			usymbol.Numerical(1),
			usymbol.Numerical(2),
			usymbol.Numerical(3),
		},
		Body: definitionBody,
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
		if proof != (ast.ASTRef{}) {
			must(m.DelRef(&proof))
		}
		for i := range parents {
			must(m.DelRef(&parents[i]))
		}
	}()
	if proof == (ast.ASTRef{}) {
		t.Fatal("valid background-theory guard was not reconstructed")
	}
	proofHandle, err := m.RefGet(proof)
	must(err)
	checker, err := proofs.NewChecker(m)
	must(err)
	valid, side, err := checker.Check(proofHandle)
	must(err)
	defer func() { must(checker.ReleaseSideConditions(side)) }()
	if !valid {
		t.Fatal("background-theory guard proof is structurally invalid")
	}
	closed, err := proofs.IsClosed(m, proofHandle)
	must(err)
	if !closed {
		t.Fatal("background-theory guard proof is open")
	}
	if len(side) == 0 {
		t.Fatal("background-theory guard disappeared from proof obligations")
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
			t.Fatal("background-theory side condition", i, status)
		}
	}
	semantic, err := factory.New(m, nil, false, false, false, "")
	must(err)
	defer func() { must(semantic.Close()) }()
	must(semantic.Assert(definition))
	negatedTarget, err := m.MkNot(target)
	must(err)
	must(semantic.Assert(negatedTarget))
	status, err := semantic.CheckSat(nil)
	must(err)
	if status != lbool.False {
		t.Fatal("source definition does not independently entail reconstructed target", status)
	}

	asserted := 0
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{proofHandle}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		leaf, err := m.IsProofKind(current, ast.ASTPrAsserted)
		must(err)
		if leaf {
			fact, err := m.GetProofFact(current)
			must(err)
			if fact != definition {
				t.Fatal("proof fabricated an asserted source")
			}
			asserted++
		}
		n, err := m.GetProofNumParents(current)
		must(err)
		for i := uint32(0); i < n; i++ {
			parent, err := m.GetProofParent(current, i)
			must(err)
			stack = append(stack, parent)
		}
	}
	if asserted != 1 {
		t.Fatal("asserted source leaf count", asserted)
	}
}

func TestQuantifiedPremiseRejectsInvalidTheoryGuard(t *testing.T) {
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
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.GetSort(truth)
	must(err)
	pName, err := m.Symbols.Intern("quantified-invalid-theory-P")
	must(err)
	pDecl, err := m.MkFuncDecl(pName, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	variable := raw(wrap(m.MkVar(0, integer)))
	predicate := raw(wrap(m.MkApp(pDecl, []ast.ASTHandle{variable})))
	binder := []usymbol.Symbol{usymbol.Numerical(0)}
	targetForall := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer},
		Names:  binder,
		Body:   predicate,
	})))
	target := raw(wrap(m.MkEq(targetForall, truth)))
	guard := raw(wrap(m.MkArithmeticGT(variable, zero)))
	definitionBody := raw(wrap(m.MkImplies(guard, predicate)))
	definition := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer},
		Names:  binder,
		Body:   definitionBody,
	})))

	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	definitionRef, err := m.NewRef(definition)
	must(err)
	graph.sourceClauses = append(graph.sourceClauses, definitionRef)
	supported, err := graph.structuralQuantifiedDefinition(definitionBody)
	must(err)
	if !supported {
		t.Fatal("relation-free theory guard did not reach checked specialization")
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer func() { must(factory.Close()) }()
	var parents []ast.ASTRef
	proof, err := graph.proveQuantifiedHornLiteral(factory, graph, -1, target, &parents)
	must(err)
	defer func() {
		if proof != (ast.ASTRef{}) {
			must(m.DelRef(&proof))
		}
		for i := range parents {
			must(m.DelRef(&parents[i]))
		}
	}()
	if proof != (ast.ASTRef{}) || len(parents) != 0 {
		t.Fatal("invalid theory guard produced a quantified proof")
	}

	oracle, err := factory.New(m, nil, false, false, false, "")
	must(err)
	defer func() { must(oracle.Close()) }()
	must(oracle.Assert(definition))
	negatedTarget, err := m.MkNot(target)
	must(err)
	must(oracle.Assert(negatedTarget))
	status, err := oracle.CheckSat(nil)
	must(err)
	if status != lbool.True {
		t.Fatal("negative semantic oracle did not expose non-entailment", status)
	}
}
