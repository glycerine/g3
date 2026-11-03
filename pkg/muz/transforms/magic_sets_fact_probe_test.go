package transforms_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

type magicFactProbeFixture struct {
	cone *coneFixture
	q    ast.ASTHandle
	goal ast.ASTHandle
}

func newMagicFactProbeFixture(t *testing.T) magicFactProbeFixture {
	t.Helper()
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	declare := func(name string) transformPredicate {
		return declareTransformPredicate(t, f, "magic-fact-probe-"+name, []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	}
	extensional := declare("E")
	q := declare("Q")
	goal := declare("Goal")
	addTransformRule(t, f, q.application, []ast.ASTHandle{extensional.application}, nil, "q-rule")
	addTransformRule(t, f, goal.application, []ast.ASTHandle{q.application}, nil, "goal-rule")
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("magic fact probe fixture is unstratified")
	}
	return magicFactProbeFixture{cone: f, q: q.declaration, goal: goal.declaration}
}

func newMagicFactProbeRelations(t *testing.T, fixture magicFactProbeFixture, populated bool) *rel.RelationManager {
	t.Helper()
	relations, err := rel.NewRelationManager(fixture.cone.manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, relations.Close()) })
	if populated {
		table, err := relations.GetFiniteRelation(fixture.q)
		coneMust(t, err)
		inserted, err := table.Add(rel.Fact{1})
		coneMust(t, err)
		if !inserted {
			t.Fatal("failed to populate magic fact probe relation")
		}
	}
	return relations
}

func magicTransferRuleCount(t *testing.T, manager *ast.ASTManager, set *base.RuleSet, predicate ast.ASTHandle) int {
	t.Helper()
	count, err := set.Len()
	coneMust(t, err)
	transfers := 0
	for i := 0; i < count; i++ {
		rule, err := set.Rule(i)
		coneMust(t, err)
		tails, err := rule.TailSize()
		if err == nil && tails == 2 {
			first, _, tailErr := rule.Tail(0)
			if tailErr == nil {
				declaration, declarationErr := manager.GetDecl(first)
				if declarationErr == nil && declaration == predicate {
					transfers++
				}
				err = declarationErr
			} else {
				err = tailErr
			}
		}
		coneMust(t, errors.Join(err, rule.Close()))
	}
	return transfers
}

func TestMagicSetsFactProbePrecedenceAndFallback(t *testing.T) {
	t.Run("probe-precedes-empty-manager", func(t *testing.T) {
		fixture := newMagicFactProbeFixture(t)
		relations := newMagicFactProbeRelations(t, fixture, false)
		calls := 0
		plugin := transforms.NewMagicSetsPluginWithFactProbe(fixture.cone.manager, fixture.cone.rules, fixture.goal, true, 0, relations, func(predicate ast.ASTHandle) (bool, error) {
			calls++
			return predicate == fixture.q, nil
		})
		defer func() { coneMust(t, plugin.Close()) }()
		result, err := plugin.Transform(fixture.cone.set)
		coneMust(t, err)
		defer func() { coneMust(t, result.Close()) }()
		if calls == 0 || magicTransferRuleCount(t, fixture.cone.manager, result, fixture.q) != 1 {
			t.Fatal("fact probe did not supply the transfer relation", calls)
		}
	})

	t.Run("false-falls-through-to-manager", func(t *testing.T) {
		fixture := newMagicFactProbeFixture(t)
		relations := newMagicFactProbeRelations(t, fixture, true)
		plugin := transforms.NewMagicSetsPluginWithFactProbe(fixture.cone.manager, fixture.cone.rules, fixture.goal, true, 0, relations, func(ast.ASTHandle) (bool, error) {
			return false, nil
		})
		defer func() { coneMust(t, plugin.Close()) }()
		result, err := plugin.Transform(fixture.cone.set)
		coneMust(t, err)
		defer func() { coneMust(t, result.Close()) }()
		if magicTransferRuleCount(t, fixture.cone.manager, result, fixture.q) != 1 {
			t.Fatal("false fact probe suppressed relation-manager transfer")
		}
	})

	t.Run("nil-preserves-manager", func(t *testing.T) {
		fixture := newMagicFactProbeFixture(t)
		relations := newMagicFactProbeRelations(t, fixture, true)
		plugin := transforms.NewMagicSetsPluginWithFactProbe(fixture.cone.manager, fixture.cone.rules, fixture.goal, true, 0, relations, nil)
		defer func() { coneMust(t, plugin.Close()) }()
		result, err := plugin.Transform(fixture.cone.set)
		coneMust(t, err)
		defer func() { coneMust(t, result.Close()) }()
		if magicTransferRuleCount(t, fixture.cone.manager, result, fixture.q) != 1 {
			t.Fatal("nil fact probe changed relation-manager transfer")
		}
	})
}

func TestMagicSetsFactProbeErrorPrecedesRelationManager(t *testing.T) {
	fixture := newMagicFactProbeFixture(t)
	relations := newMagicFactProbeRelations(t, fixture, true)
	want := errors.New("magic fact probe failure")
	plugin := transforms.NewMagicSetsPluginWithFactProbe(fixture.cone.manager, fixture.cone.rules, fixture.goal, true, 0, relations, func(predicate ast.ASTHandle) (bool, error) {
		if predicate == fixture.q {
			return false, want
		}
		return false, nil
	})
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(fixture.cone.set)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("fact probe error returned a transformed rule set")
	}
	if !errors.Is(err, want) {
		t.Fatalf("fact probe error %v, want %v", err, want)
	}
}
