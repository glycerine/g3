package transforms_test

// Source: src/muz/transforms/dl_mk_unfold.cpp one-round tail expansion.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestRuleUnfoldExpandsOriginalTailOnce(t *testing.T) {
	f := newConeFixture(t, "P", "Q", "Goal")
	f.add(t, f.applications["P"], "fact-P")
	f.edge(t, "P", "Q")
	f.edge(t, "Q", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("unfold fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewRuleUnfoldPlugin(f.manager, f.rules, f.globals.Context, f.globals, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("unfold plugin returned no replacement")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 3 {
		t.Fatalf("unfolded rules %d, want 3", count)
	}
	wantTails := map[ast.ASTHandle]int{
		f.declarations["P"]:    0,
		f.declarations["Q"]:    0,
		f.declarations["Goal"]: 1,
	}
	for index := 0; index < count; index++ {
		rule, getErr := f.set.Rule(index)
		coneMust(t, getErr)
		head, getErr := rule.HeadDeclaration()
		tails, tailErr := rule.UninterpretedTailSize()
		getErr = errors.Join(getErr, tailErr, rule.Close())
		coneMust(t, getErr)
		if tails != wantTails[head] {
			t.Fatal("unfolded tail count", head, tails, wantTails[head])
		}
	}
}

func TestRuleUnfoldCarriesHyperResolutionProof(t *testing.T) {
	f := newConeFixture(t, "P", "Q", "Goal")
	if err := f.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
		t.Fatal(err)
	}
	f.add(t, f.applications["P"], "proof-fact")
	f.edge(t, "P", "Q")
	f.edge(t, "Q", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("proof unfold fixture rejected")
	}
	var premiseProofs []ast.ASTHandle
	count, err := f.set.Len()
	coneMust(t, err)
	for i := 0; i < count; i++ {
		rule, getErr := f.set.Rule(i)
		coneMust(t, getErr)
		formula, getErr := rule.ToFormula()
		coneMust(t, getErr)
		fact, getErr := f.manager.RefGet(formula)
		coneMust(t, getErr)
		proof, getErr := f.manager.MkProofAsserted(fact)
		coneMust(t, getErr)
		coneMust(t, rule.SetProof(proof))
		premiseProofs = append(premiseProofs, proof)
		coneMust(t, f.manager.DelRef(&formula))
		coneMust(t, rule.Close())
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewRuleUnfoldPlugin(f.manager, f.rules, f.globals.Context, f.globals, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("unfold plugin returned no replacement")
	}
	for i := 0; ; i++ {
		count, getErr := f.set.Len()
		coneMust(t, getErr)
		if i >= count {
			break
		}
		rule, getErr := f.set.Rule(i)
		coneMust(t, getErr)
		head, getErr := rule.HeadDeclaration()
		if getErr != nil {
			coneMust(t, errors.Join(getErr, rule.Close()))
		}
		if head == f.declarations["Goal"] {
			proof, found, proofErr := rule.Proof()
			coneMust(t, proofErr)
			if !found {
				coneMust(t, rule.Close())
				t.Fatal("unfolded rule lost its proof")
			}
			kind, getErr := f.manager.GetDeclKind(proof)
			coneMust(t, getErr)
			if kind != ast.ASTPrHyperResolve {
				coneMust(t, rule.Close())
				t.Fatalf("unfold proof kind %d, want hyper-resolution", kind)
			}
			if n, getErr := f.manager.GetNumArgs(proof); getErr != nil || n != 3 {
				coneMust(t, rule.Close())
				t.Fatalf("unfold proof argument count %d, want two premises and conclusion: %v", n, getErr)
			}
			first, getErr := f.manager.GetArg(proof, 0)
			coneMust(t, getErr)
			second, getErr := f.manager.GetArg(proof, 1)
			coneMust(t, getErr)
			foundFirst, foundSecond := false, false
			for _, premise := range premiseProofs {
				foundFirst = foundFirst || premise == first
				foundSecond = foundSecond || premise == second
			}
			if !foundFirst || !foundSecond {
				coneMust(t, rule.Close())
				t.Fatal("unfold proof did not retain source premises")
			}
			proofDecl, getErr := f.manager.GetDecl(proof)
			coneMust(t, getErr)
			parameters, getErr := f.manager.GetDeclParametersCopy(proofDecl)
			coneMust(t, getErr)
			if len(parameters) != 2 {
				for j := range parameters {
					coneMust(t, f.manager.DelParameter(&parameters[j]))
				}
				coneMust(t, rule.Close())
				t.Fatalf("ground unfold proof parameter count %d, want two positions", len(parameters))
			}
			firstPosition, getErr := f.manager.ParameterInt(parameters[0])
			coneMust(t, getErr)
			secondPosition, getErr := f.manager.ParameterInt(parameters[1])
			coneMust(t, errors.Join(getErr, f.manager.DelParameter(&parameters[1]), f.manager.DelParameter(&parameters[0])))
			if firstPosition != 1 || secondPosition != 0 {
				coneMust(t, rule.Close())
				t.Fatalf("unfold proof resolution position (%d,%d), want (1,0)", firstPosition, secondPosition)
			}
			formula, getErr := rule.ToFormula()
			coneMust(t, getErr)
			formulaHandle, getErr := f.manager.RefGet(formula)
			coneMust(t, getErr)
			proofFact, getErr := f.manager.GetProofFact(proof)
			coneMust(t, getErr)
			equal, getErr := f.manager.AreEqual(proofFact, formulaHandle)
			coneMust(t, errors.Join(getErr, f.manager.DelRef(&formula)))
			if !equal {
				coneMust(t, rule.Close())
				t.Fatal("unfold hyper-resolution proof proves the wrong rule formula")
			}
			coneMust(t, rule.Close())
		} else {
			coneMust(t, rule.Close())
		}
	}
}
