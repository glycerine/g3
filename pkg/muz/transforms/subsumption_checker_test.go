package transforms_test

// Source: src/muz/transforms/dl_mk_subsumption_checker.cpp self-recursion,
// total-relation tail removal, and output preservation.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestSubsumptionCheckerDropsTautologicalSelfRule(t *testing.T) {
	f := newConeFixture(t, "Loop")
	f.edge(t, "Loop", "Loop")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Loop"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("self-rule fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	plugin := transforms.NewSubsumptionCheckerPlugin(true, 0)
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("tautological rule was not removed")
	}
	if count, err := f.set.Len(); err != nil || count != 0 {
		t.Fatal("tautological rule count", count, err)
	}
}

func TestSubsumptionCheckerRemovesTotalPositiveTail(t *testing.T) {
	f := newConeFixture(t, "Total", "Goal")
	f.add(t, f.applications["Total"], "total")
	f.edge(t, "Total", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("total-tail fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	plugin := transforms.NewSubsumptionCheckerPlugin(true, 0)
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("total tail was not removed")
	}
	if count, err := f.set.Len(); err != nil || count != 1 {
		t.Fatal("total-tail rule count", count, err)
	}
	rule, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	head, err := rule.HeadDeclaration()
	coneMust(t, err)
	tails, err := rule.TailSize()
	coneMust(t, err)
	if head != f.declarations["Goal"] || tails != 0 {
		t.Fatal("total-tail result", head, tails)
	}
	steps, err := plugin.TakeModelSteps()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transforms.CloseConeModelSteps(steps)) })
	if len(steps) != 1 || len(steps[0]) != 1 || steps[0][0].Declaration != f.declarations["Total"] {
		t.Fatal("total-relation model step", steps)
	}
	formula, err := f.manager.RefGet(steps[0][0].Formula)
	coneMust(t, err)
	if !f.manager.IsTrue(formula) {
		t.Fatal("total-relation interpretation is not true", formula)
	}
}

func TestSubsumptionCheckerTransfersRuleProof(t *testing.T) {
	f := newConeFixture(t, "Total", "Goal")
	f.add(t, f.applications["Total"], "total")
	f.edge(t, "Total", "Goal")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("proof fixture rejected")
	}
	original, err := f.set.Rule(1)
	coneMust(t, err)
	defer func() { coneMust(t, original.Close()) }()
	formula, err := original.ToFormula()
	coneMust(t, err)
	defer func() { coneMust(t, f.manager.DelRef(&formula)) }()
	value, err := f.manager.RefGet(formula)
	coneMust(t, err)
	scope, err := ast.NewScopedProof(f.manager)
	coneMust(t, err)
	prior, err := f.manager.MkProofAsserted(value)
	coneMust(t, err)
	coneMust(t, original.SetProof(prior))
	coneMust(t, scope.Close())
	transformer := base.NewRuleTransformer()
	defer func() { coneMust(t, transformer.Close()) }()
	coneMust(t, transformer.Register(transforms.NewSubsumptionCheckerPlugin(true, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("total tail was not removed")
	}
	updated, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, updated.Close()) }()
	proof, found, err := updated.Proof()
	coneMust(t, err)
	if !found {
		t.Fatal("rewritten rule has no proof")
	}
	fact, err := f.manager.GetProofFact(proof)
	coneMust(t, err)
	newFormula, err := updated.ToFormula()
	coneMust(t, err)
	defer func() { coneMust(t, f.manager.DelRef(&newFormula)) }()
	want, err := f.manager.RefGet(newFormula)
	coneMust(t, err)
	if fact != want {
		t.Fatal("rewritten proof fact differs from the new rule")
	}
	parent, err := f.manager.GetProofParent(proof, 0)
	coneMust(t, err)
	if parent != prior {
		t.Fatal("rewritten proof does not depend on the original proof")
	}
}

func TestSubsumptionCheckerDropsStructurallyDuplicateRules(t *testing.T) {
	f := newConeFixture(t, "P", "Q")
	for _, name := range []string{"first-duplicate", "second-duplicate"} {
		addTransformRule(t, f, f.applications["P"], []ast.ASTHandle{f.applications["Q"]}, nil, name)
	}
	coneMust(t, f.set.SetOutputPredicate(f.declarations["P"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("duplicate-rule fixture rejected")
	}

	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewSubsumptionCheckerPlugin(true, 0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("structurally duplicate rule was not removed")
	}
	if count, countErr := f.set.Len(); countErr != nil || count != 1 {
		t.Fatal("duplicate-rule count", count, countErr)
	}
	remaining, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, remaining.Close()) }()
	name, err := remaining.Name()
	coneMust(t, err)
	if name != "first-duplicate" {
		t.Fatalf("duplicate pruning kept %q, want first source rule", name)
	}
}

func TestSubsumptionCheckerDisabledReturnsNoChange(t *testing.T) {
	f := newConeFixture(t, "Loop")
	f.edge(t, "Loop", "Loop")
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("disabled fixture rejected")
	}
	plugin := transforms.NewSubsumptionCheckerPlugin(false, 0)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	replacement, err := plugin.Transform(f.set)
	coneMust(t, err)
	if replacement != nil {
		coneMust(t, replacement.Close())
		t.Fatal("disabled checker returned a replacement")
	}
}
