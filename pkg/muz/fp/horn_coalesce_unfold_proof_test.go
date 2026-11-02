package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
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
	var queryWrapper ast.ASTHandle
	checkProof := func(proof ast.ASTHandle) (int, int, int) {
		seen := make(map[ast.ASTHandle]bool)
		stack := []ast.ASTHandle{proof}
		resolutions, modusPonens, originalLeaves := 0, 0, 0
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
			if kind == ast.ASTPrModusPonens {
				modusPonens++
			}
			if kind == ast.ASTPrAsserted && originalProofs[node] {
				originalLeaves++
			}
			if kind == ast.ASTPrHyperResolve {
				resolutions++
			}
			if kind == ast.ASTPrAsserted && !originalProofs[node] {
				if queryWrapper.IsNull() {
					t.Fatal("preprocessing introduced an asserted proof leaf for a transformed rule")
				}
				fact, err := m.GetProofFact(node)
				fragmentMust(t, err)
				hasWrapper, err := m.OccursExpr(queryWrapper, fact)
				fragmentMust(t, err)
				hasQuery, err := m.OccursExpr(p, fact)
				fragmentMust(t, err)
				if !hasWrapper || !hasQuery {
					t.Fatal("trace proof introduced a non-query assertion")
				}
			}
			count, err := m.GetNumArgs(node)
			fragmentMust(t, err)
			for i := uint32(0); i < count; i++ {
				child, err := m.GetArg(node, i)
				fragmentMust(t, err)
				stack = append(stack, child)
			}
		}
		return resolutions, modusPonens, originalLeaves
	}
	resolutions, modusPonens, originalLeaves := checkProof(proof)
	if resolutions < 1 || modusPonens < 3 || originalLeaves != len(originalProofs) {
		t.Fatalf("proof ancestry: hyper-resolution=%d modus-ponens=%d original-clauses=%d, want unfold plus both coalesce clauses", resolutions, modusPonens, originalLeaves)
	}
	status, err := e.QueryPredicate(goal)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("coalesced/unfolded query", status)
	}
	queryWrapper, err = m.MkApp(e.spacerQuery, []ast.ASTHandle{x})
	fragmentMust(t, err)
	trace, err := e.spacer.RulesAlongTrace()
	fragmentMust(t, err)
	defer func() {
		for _, rule := range trace {
			fragmentMust(t, rule.Close())
		}
	}()
	if len(trace) < 2 {
		t.Fatal("missing query-wrapper and goal trace", len(trace))
	}
	traceResolutions, traceModusPonens := 0, 0
	for i, rule := range trace {
		proof, found, err := rule.Proof()
		fragmentMust(t, err)
		if !found {
			t.Fatal("reachable trace rule lost its proof", i)
		}
		formula, err := rule.ToFormula()
		fragmentMust(t, err)
		want, err := m.RefGet(formula)
		fragmentMust(t, err)
		fact, err := m.GetProofFact(proof)
		fragmentMust(t, err)
		if fact != want {
			t.Fatal("trace proof conclusion differs from its rule", i)
		}
		fragmentMust(t, m.DelRef(&formula))
		r, mp, _ := checkProof(proof)
		traceResolutions += r
		traceModusPonens += mp
	}
	if traceResolutions < 1 || traceModusPonens < 3 {
		t.Fatal("query trace lost coalesce/unfold proof ancestry", traceResolutions, traceModusPonens)
	}
}
