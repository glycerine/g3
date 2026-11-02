package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifiedPremiseTriesLaterApplicableDefinition(t *testing.T) {
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
	seven := raw(wrap(m.ArithmeticIntegerNumeralText("7")))
	integer, err := m.GetSort(seven)
	must(err)
	boolean, err := m.GetSort(truth)
	must(err)
	name, err := m.Symbols.Intern("quantified-order-P")
	must(err)
	pDecl, err := m.MkFuncDecl(name, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	targetVariable := raw(wrap(m.MkVar(0, integer)))
	targetBody := raw(wrap(m.MkApp(pDecl, []ast.ASTHandle{targetVariable, seven})))
	targetName, err := m.Symbols.Intern("quantified-order-target")
	must(err)
	targetForall := raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer},
		Names:  []usymbol.Symbol{targetName},
		Body:   targetBody,
	})))
	target := raw(wrap(m.MkEq(targetForall, truth)))

	a := raw(wrap(m.MkVar(0, integer)))
	b := raw(wrap(m.MkVar(1, integer)))
	sourceHead := raw(wrap(m.MkApp(pDecl, []ast.ASTHandle{b, a})))
	aName, err := m.Symbols.Intern("quantified-order-a")
	must(err)
	bName, err := m.Symbols.Intern("quantified-order-b")
	must(err)
	definition := func(guard ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		body := raw(wrap(m.MkImplies(guard, sourceHead)))
		return raw(wrap(m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true,
			Sorts:  []ast.ASTHandle{integer, integer},
			Names:  []usymbol.Symbol{bName, aName},
			Body:   body,
		})))
	}
	bad := definition(raw(wrap(m.MkEq(a, zero))))
	recursive := definition(sourceHead)
	good := definition(raw(wrap(m.MkEq(a, seven))))

	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	for _, clause := range []ast.ASTHandle{bad, recursive, good} {
		ref, err := m.NewRef(clause)
		must(err)
		graph.sourceClauses = append(graph.sourceClauses, ref)
	}
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
		t.Fatal("later valid quantified definition was hidden")
	}
	proofHandle, err := m.RefGet(proof)
	must(err)
	seenGood := false
	stack := []ast.ASTHandle{proofHandle}
	seen := make(map[ast.ASTHandle]bool)
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
			if fact == bad || fact == recursive {
				t.Fatal("inapplicable definition was selected")
			}
			seenGood = seenGood || fact == good
		}
		n, err := m.GetProofNumParents(current)
		must(err)
		for i := uint32(0); i < n; i++ {
			parent, err := m.GetProofParent(current, i)
			must(err)
			stack = append(stack, parent)
		}
	}
	if !seenGood {
		t.Fatal("valid later definition is absent from proof")
	}
}
