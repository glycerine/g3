package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSpacerCoalesceAndUnfoldComposeRuleProofs(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	for key, value := range map[string]string{
		"fixedpoint.generate_proof_trace":      "true",
		"fixedpoint.xform.slice":               "false",
		"fixedpoint.xform.inline_eager":        "false",
		"fixedpoint.xform.inline_linear":       "false",
		"fixedpoint.xform.subsumption_checker": "false",
		"fixedpoint.xform.coalesce_rules":      "true",
		"fixedpoint.xform.unfold_rules":        "1",
	} {
		fragmentMust(t, globals.SourceSetName(key, value))
	}
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	fragmentMust(t, e.SelectEngine("spacer"))
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	integer, err := m.GetSort(zero)
	fragmentMust(t, err)
	declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
		symbol, err := m.Symbols.Intern(name)
		fragmentMust(t, err)
		decl, err := m.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, err)
		fragmentMust(t, e.RegisterPredicate(decl))
		x, err := m.MkVar(0, integer)
		fragmentMust(t, err)
		app, err := m.MkApp(decl, []ast.ASTHandle{x})
		fragmentMust(t, err)
		return decl, app
	}
	_, q := declare("coalesce-unfold-q")
	goal, p := declare("coalesce-unfold-p")
	x, err := m.GetArg(p, 0)
	fragmentMust(t, err)
	le, err := m.MkArithmeticLE(x, zero)
	fragmentMust(t, err)
	ge, err := m.MkArithmeticGE(x, zero)
	fragmentMust(t, err)
	for i, tails := range [][]ast.ASTHandle{nil, {q, le}, {q, ge}} {
		head := p
		if i == 0 {
			head = q
		}
		rule, err := e.rules.NewRule(head, tails, nil, "")
		fragmentMust(t, err)
		fragmentMust(t, e.set.Add(rule))
		fragmentMust(t, rule.Close())
	}
	fragmentMust(t, e.validateFragment())
	originalProofs := make(map[ast.ASTHandle]bool)
	count, err := e.set.Len()
	fragmentMust(t, err)
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		fragmentMust(t, err)
		proof, found, err := rule.Proof()
		fragmentMust(t, err)
		if !found {
			t.Fatal("input rule has no proof")
		}
		originalProofs[proof] = true
		fragmentMust(t, rule.Close())
	}
	prepared, err := e.preprocessHornRules(goal, true, true)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, prepared.Close()) }()
	rules, err := prepared.rules.PredicateRules(goal)
	fragmentMust(t, err)
	defer func() {
		for _, rule := range rules {
			fragmentMust(t, rule.Close())
		}
	}()
	if len(rules) != 1 {
		t.Fatalf("coalesced goal rules = %d, want 1", len(rules))
	}
	proof, found, err := rules[0].Proof()
	fragmentMust(t, err)
	if !found {
		t.Fatal("composed preprocessing lost its proof")
	}
	formula, err := rules[0].ToFormula()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, m.DelRef(&formula)) }()
	want, err := m.RefGet(formula)
	fragmentMust(t, err)
	fact, err := m.GetProofFact(proof)
	fragmentMust(t, err)
	if fact != want {
		t.Fatal("composed proof concludes a different rule")
	}
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{proof}
	resolutions := 0
	for len(stack) != 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[node] {
			continue
		}
		seen[node] = true
		isProof, err := m.IsProof(node)
		fragmentMust(t, err)
		if !isProof {
			continue
		}
		kind, err := m.GetDeclKind(node)
		fragmentMust(t, err)
		if kind == ast.ASTPrHyperResolve {
			resolutions++
		}
		if kind == ast.ASTPrAsserted && !originalProofs[node] {
			t.Fatal("preprocessing introduced an asserted proof leaf for a transformed rule")
		}
		count, err := m.GetNumArgs(node)
		fragmentMust(t, err)
		for i := uint32(0); i < count; i++ {
			child, err := m.GetArg(node, i)
			fragmentMust(t, err)
			stack = append(stack, child)
		}
	}
	if resolutions < 2 {
		t.Fatalf("proof has %d hyper-resolution nodes, want coalesce and unfold", resolutions)
	}
}
