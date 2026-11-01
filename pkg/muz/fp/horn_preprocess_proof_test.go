package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestHornPreprocessingCarriesRequestedRuleProofs(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	fragmentMust(t, globals.SourceSetName("fixedpoint.generate_proof_trace", "true"))
	fragmentMust(t, globals.SourceSetName("fixedpoint.xform.slice", "false"))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	fragmentMust(t, engine.SelectEngine("qbmc"))
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
		s, e := manager.Symbols.Intern(name)
		fragmentMust(t, e)
		d, e := manager.MkFuncDecl(s, nil, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, e)
		fragmentMust(t, engine.RegisterPredicate(d))
		h, e := manager.MkApp(d, nil)
		fragmentMust(t, e)
		return d, h
	}
	_, fact := declare("proof-preprocess-fact")
	goal, goalApp := declare("proof-preprocess-goal")
	fragmentMust(t, engine.AddRule(fact, "fact"))
	step, err := manager.MkImplies(fact, goalApp)
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(step, "step"))
	fragmentMust(t, engine.validateFragment())
	prepared, err := engine.preprocessHornRules(goal, true, true)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, prepared.Close()) }()
	group, err := prepared.rules.PredicateRules(goal)
	fragmentMust(t, err)
	defer func() {
		for i := range group {
			fragmentMust(t, group[i].Close())
		}
	}()
	if len(group) == 0 {
		t.Fatal("preprocessing lost the goal rule")
	}
	for _, rule := range group {
		proof, found, e := rule.Proof()
		fragmentMust(t, e)
		if !found {
			t.Fatal("preprocessed goal rule lost its proof")
		}
		formula, e := rule.ToFormula()
		fragmentMust(t, e)
		value, e := manager.RefGet(formula)
		fragmentMust(t, e)
		fact, e := manager.GetProofFact(proof)
		fragmentMust(t, e)
		fragmentMust(t, manager.DelRef(&formula))
		if value != fact {
			t.Fatal("preprocessed proof concludes a different rule")
		}
	}
}
