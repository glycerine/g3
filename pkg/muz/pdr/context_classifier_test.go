package pdr

// Source: context::classifier_proc and init_core_generalizers in
// src/muz/pdr/pdr_context.cpp:1596-1693,1815-1846.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func classifierContextFixture(t *testing.T) (*ast.ASTManager, *Context, *base.RuleManager) {
	t.Helper()
	manager, owner, transformer, _, closeBase := closureFixture(t)
	config := owner.contexts.config
	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	t.Cleanup(closeBase)
	context, err := NewContext(manager, nil, 2, ReachableNoCache, config)
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, context.Close()) })
	closureMust(t, context.SetUseFarkas(true))
	rules, err := base.NewRuleManager(manager)
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, rules.Close()) })
	return manager, context, rules
}

func classifierRuleSet(t *testing.T, manager *ast.ASTManager, rules *base.RuleManager, shape string) (*base.RuleSet, ast.ASTHandle) {
	t.Helper()
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	declare := func(name string, domain []ast.ASTHandle, predicate bool) ast.ASTHandle {
		nameSymbol, err := manager.Symbols.Intern("classifier-" + shape + "-" + name)
		closureMust(t, err)
		declaration, err := manager.MkFuncDecl(nameSymbol, domain, boolean, ast.ASTFuncDeclInfo{})
		closureMust(t, err)
		if predicate {
			closureMust(t, rules.RegisterPredicate(declaration))
		}
		return declaration
	}
	goal := declare("goal", nil, true)
	head, err := manager.MkApp(goal, nil)
	closureMust(t, err)
	set, err := base.NewRuleSet(rules)
	closureMust(t, err)
	t.Cleanup(func() { closureMust(t, set.Close()) })
	var tails []ast.ASTHandle
	if shape != "fact" {
		sort := boolean
		if shape == "tail-int" {
			arithmetic, err := manager.ArithmeticFamilyID()
			closureMust(t, err)
			sort, err = manager.MkFamilySort(arithmetic, 1, nil)
			closureMust(t, err)
		}
		argument, err := manager.MkVar(0, sort)
		closureMust(t, err)
		if shape == "tail-boolean-uf" || shape == "interpreted-boolean-uf" {
			argument, err = manager.MkTrue()
			closureMust(t, err)
			function := declare("function", []ast.ASTHandle{boolean}, false)
			argument, err = manager.MkApp(function, []ast.ASTHandle{argument})
			closureMust(t, err)
		}
		if shape == "quantified-boolean" {
			body, err := manager.MkEq(argument, argument)
			closureMust(t, err)
			argument, err = manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body})
			closureMust(t, err)
			truth, err := manager.MkTrue()
			closureMust(t, err)
			argument, err = manager.MkEq(argument, truth)
			closureMust(t, err)
		}
		switch shape {
		case "interpreted-boolean-uf", "quantified-boolean":
			tails = []ast.ASTHandle{argument}
		case "head-boolean-variable":
			goal = declare("head", []ast.ASTHandle{sort}, true)
			head, err = manager.MkApp(goal, []ast.ASTHandle{argument})
			closureMust(t, err)
		default:
			predicate := declare("undefined-tail", []ast.ASTHandle{sort}, true)
			application, err := manager.MkApp(predicate, []ast.ASTHandle{argument})
			closureMust(t, err)
			tails = []ast.ASTHandle{application}
		}
	}
	rule, err := rules.NewRule(head, tails, nil, shape)
	closureMust(t, err)
	closureMust(t, set.Add(rule))
	closureMust(t, rule.Close())
	return set, goal
}

func TestContextClassifierMatchesSourceArgumentAndTheoryBranches(t *testing.T) {
	for _, test := range []struct {
		shape      string
		nonBoolean bool
		mode       uint32
		status     lbool.LBool
	}{
		{"tail-boolean-variable", false, 0, lbool.False},
		{"head-boolean-variable", false, 0, lbool.True},
		{"tail-int", true, smtcore.SMTArithmeticDifferenceLogic, lbool.False},
		{"tail-boolean-uf", true, smtcore.SMTArithmeticSimplex, lbool.False},
		{"interpreted-boolean-uf", true, smtcore.SMTArithmeticSimplex, lbool.True},
		{"quantified-boolean", true, smtcore.SMTArithmeticSimplex, lbool.True},
	} {
		t.Run(test.shape, func(t *testing.T) {
			manager, context, rules := classifierContextFixture(t)
			set, goal := classifierRuleSet(t, manager, rules, test.shape)
			before, err := manager.ProofMode()
			closureMust(t, err)
			closureMust(t, context.Initialize(set))
			if context.proofActive != test.nonBoolean || context.farkasActive != test.nonBoolean || context.owner.contexts.arithmetic.enabled != test.nonBoolean {
				t.Fatal("source classifier selected different solver policy", context.proofActive, context.farkasActive, context.owner.contexts.arithmetic.enabled)
			}
			if test.nonBoolean {
				for _, solver := range context.owner.contexts.solvers {
					state, err := solver.ArithmeticState()
					closureMust(t, err)
					if state.Mode != test.mode || state.Eq2Ineq != (test.mode == smtcore.SMTArithmeticDifferenceLogic) || state.BoundProp != 0 || !state.AutoConfigSimplex || state.PropagateEqs || state.EagerEqAxioms {
						t.Fatal("source arithmetic profile", state, test.mode)
					}
				}
			}
			if status, err := context.Solve(goal); err != nil || status != test.status {
				t.Fatal("classified program solve", status, err)
			}
			after, err := manager.ProofMode()
			closureMust(t, err)
			if after != before {
				t.Fatal("classification leaked source proof mode", before, after)
			}
		})
	}
}

func TestContextClassifierUpdateCompatibilityMatchesInitialization(t *testing.T) {
	for _, shape := range []string{"tail-int", "tail-boolean-uf", "quantified-boolean"} {
		t.Run(shape, func(t *testing.T) {
			manager, context, rules := classifierContextFixture(t)
			original, goal := classifierRuleSet(t, manager, rules, "fact")
			replacement, replacementGoal := classifierRuleSet(t, manager, rules, shape)
			closureMust(t, context.Initialize(original))
			if compatible, err := context.CanUpdateRules(replacement); err != nil || compatible {
				t.Fatal("changed classifier accepted in-place update", compatible, err)
			}
			if err := context.UpdateRules(replacement); !errors.Is(err, ast.ErrASTDomain) {
				t.Fatal("changed classifier update did not reject before mutation", err)
			}
			if status, err := context.Solve(goal); err != nil || status != lbool.True {
				t.Fatal("rejected classifier update damaged prior program", status, err)
			}
			closureMust(t, context.Reset())
			closureMust(t, context.Initialize(replacement))
			if compatible, err := context.CanUpdateRules(replacement); err != nil || !compatible {
				t.Fatal("same classifier rejected in-place update", compatible, err)
			}
			closureMust(t, context.UpdateRules(replacement))
			want := lbool.False
			if shape == "quantified-boolean" {
				want = lbool.True
			}
			if status, err := context.Solve(replacementGoal); err != nil || status != want {
				t.Fatal("updated classified program solve", status, err)
			}
		})
	}
}
