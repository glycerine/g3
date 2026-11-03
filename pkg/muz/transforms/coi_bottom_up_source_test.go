package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestConeBottomUpStopsBeforeLaterMaterializedTail(t *testing.T) {
	f := newConeFixture(t, "Goal", "Missing", "Stored")
	addTransformRule(t, f, f.applications["Goal"], []ast.ASTHandle{f.applications["Missing"], f.applications["Stored"]}, nil, "unreachable-first")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("unstratified fixture")
	}
	relations, err := rel.NewRelationManager(f.manager)
	coneMust(t, err)
	defer func() { coneMust(t, relations.Close()) }()
	table, err := sparseTransformRelation(relations, f.declarations["Stored"], false)
	coneMust(t, err)
	_, err = table.Add(rel.Fact{})
	coneMust(t, err)
	plugin := transforms.NewConePluginWithRelations(relations, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("later materialized tail prevented source early pruning")
	}
	defer func() { coneMust(t, result.Close()) }()
	if count, err := result.Len(); err != nil || count != 0 {
		t.Fatal("unreachable rule survived", count, err)
	}
	steps, err := plugin.TakeModelSteps()
	coneMust(t, err)
	defer func() { coneMust(t, transforms.CloseConeModelSteps(steps)) }()
	if len(steps) != 1 || len(steps[0]) != 1 || steps[0][0].Declaration != f.declarations["Missing"] {
		t.Fatal("bottom-up converter visited beyond first unreachable positive tail", steps)
	}
}

func TestConeBottomUpReplacementClearsNameAndPreservesVariableSlots(t *testing.T) {
	f := newConeFixture(t, "Dead", "Missing")
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	first, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	second, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	p := declareTransformPredicate(t, f, "coi-slot-P", []ast.ASTHandle{boolean}, []ast.ASTHandle{second})
	goal := declareTransformPredicate(t, f, "coi-slot-Goal", []ast.ASTHandle{boolean}, []ast.ASTHandle{second})
	missing := declareTransformPredicate(t, f, "coi-slot-Missing", []ast.ASTHandle{boolean}, []ast.ASTHandle{first})
	pFact, err := f.manager.MkApp(p.declaration, []ast.ASTHandle{first})
	coneMust(t, err)
	addTransformRule(t, f, pFact, nil, nil, "p-fact")
	addTransformRule(t, f, goal.application, []ast.ASTHandle{p.application, missing.application}, []bool{false, true}, "removed-negative")
	addTransformRule(t, f, f.applications["Dead"], []ast.ASTHandle{f.applications["Missing"]}, nil, "unfounded")
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Dead"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("unstratified fixture")
	}
	result, err := transforms.ConeOfInfluence(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("bottom-up pruning did not publish a rule set")
	}
	defer func() { coneMust(t, result.Close()) }()
	rules, err := result.PredicateRules(goal.declaration)
	coneMust(t, err)
	if len(rules) != 1 {
		t.Fatal("goal replacement count", len(rules))
	}
	defer func() { coneMust(t, rules[0].Close()) }()
	name, err := rules[0].Name()
	coneMust(t, err)
	if name != "" {
		t.Fatal("source replacement must have no rule name", name)
	}
	head, err := rules[0].Head()
	coneMust(t, err)
	argument, err := f.manager.GetArg(head, 0)
	coneMust(t, err)
	index, err := f.manager.GetVarIndex(argument)
	coneMust(t, err)
	if index != 1 {
		t.Fatal("source replacement renumbered its surviving variable", index)
	}
	if size, err := rules[0].TailSize(); err != nil || size != 1 {
		t.Fatal("unreachable negative tail survived", size, err)
	}
}

func TestConeBottomUpMaterializedBailoutReleasesCurrentRule(t *testing.T) {
	f := newConeFixture(t, "Goal", "Stored")
	before := f.manager.Stats().LiveReferences
	addTransformRule(t, f, f.applications["Goal"], []ast.ASTHandle{f.applications["Stored"]}, nil, "stored-tail")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("unstratified fixture")
	}
	relations, err := rel.NewRelationManager(f.manager)
	coneMust(t, err)
	table, err := sparseTransformRelation(relations, f.declarations["Stored"], false)
	coneMust(t, err)
	_, err = table.Add(rel.Fact{})
	coneMust(t, err)
	result, err := transforms.ConeOfInfluenceWithRelations(f.set, relations)
	coneMust(t, err)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("materialized first tail did not bail out")
	}
	coneMust(t, relations.Close())
	coneMust(t, f.set.Close())
	if after := f.manager.Stats().LiveReferences; after != before {
		t.Fatal("materialized-tail bailout leaked rule roots", before, after)
	}
}
