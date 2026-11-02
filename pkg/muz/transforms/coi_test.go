package transforms_test

// Source: src/muz/transforms/dl_mk_coi_filter.cpp top-down and bottom-up
// reachability pruning.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func coneMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type coneFixture struct {
	manager      *ast.ASTManager
	rules        *base.RuleManager
	set          *base.RuleSet
	globals      *uparams.GlobalParamsService
	declarations map[string]ast.ASTHandle
	applications map[string]ast.ASTHandle
}

func newConeFixture(t *testing.T, names ...string) *coneFixture {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, context.Close()) })
	manager, err := context.Manager()
	coneMust(t, err)
	globals, err := context.GlobalParameters()
	coneMust(t, err)
	rules, err := base.NewRuleManager(manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, rules.Close()) })
	set, err := base.NewRuleSet(rules)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, set.Close()) })
	boolean, err := manager.MkBoolSort()
	coneMust(t, err)
	f := &coneFixture{manager: manager, rules: rules, set: set, globals: globals,
		declarations: make(map[string]ast.ASTHandle), applications: make(map[string]ast.ASTHandle)}
	for _, name := range names {
		symbol, e := manager.Symbols.Intern("cone-" + name)
		coneMust(t, e)
		f.declarations[name], e = manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, e)
		f.applications[name], e = manager.MkApp(f.declarations[name], nil)
		coneMust(t, e)
		coneMust(t, rules.RegisterPredicate(f.declarations[name]))
	}
	return f
}

func (f *coneFixture) add(t *testing.T, formula ast.ASTHandle, name string) {
	t.Helper()
	reference, err := f.manager.NewRef(formula)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, f.manager.DelRef(&reference)) })
	coneMust(t, f.rules.AddFormula(f.set, formula, name))
}

func (f *coneFixture) edge(t *testing.T, from, to string) {
	t.Helper()
	formula, err := f.manager.MkImplies(f.applications[from], f.applications[to])
	coneMust(t, err)
	f.add(t, formula, from+"-"+to)
}

func TestConeOfInfluencePrunesOutsideAndUnfoundedRules(t *testing.T) {
	f := newConeFixture(t, "P", "Q", "R", "X", "S", "U", "V")
	f.add(t, f.applications["P"], "fact-P")
	f.edge(t, "P", "Q")
	f.edge(t, "Q", "R")
	f.edge(t, "X", "S")
	f.add(t, f.applications["U"], "fact-U")
	f.edge(t, "U", "V")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["R"]))
	coneMust(t, f.set.SetOutputPredicate(f.declarations["S"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("cone fixture rejected")
	}

	filtered, err := transforms.ConeOfInfluence(f.set)
	coneMust(t, err)
	if filtered == nil {
		t.Fatal("changed cone returned nil")
	}
	defer func() { coneMust(t, filtered.Close()) }()
	if count, err := filtered.Len(); err != nil || count != 3 {
		t.Fatal("filtered rule count", count, err)
	}
	wantHeads := map[ast.ASTHandle]int{
		f.declarations["P"]: 1,
		f.declarations["Q"]: 1,
		f.declarations["R"]: 1,
	}
	for i := 0; i < 3; i++ {
		rule, e := filtered.Rule(i)
		coneMust(t, e)
		head, e := rule.HeadDeclaration()
		coneMust(t, e)
		wantHeads[head]--
		coneMust(t, rule.Close())
	}
	for head, remaining := range wantHeads {
		if remaining != 0 {
			t.Fatal("filtered head multiplicity", head, remaining)
		}
	}
	outputs, err := filtered.OutputPredicates()
	coneMust(t, err)
	if len(outputs) != 2 || outputs[0] != f.declarations["R"] || outputs[1] != f.declarations["S"] {
		t.Fatal("filtered outputs", outputs)
	}
	if finalized, err := filtered.IsFinalized(); err != nil || !finalized {
		t.Fatal("filtered finalization", finalized, err)
	}
}

func TestConePluginCapturesOrderedModelDefinitions(t *testing.T) {
	f := newConeFixture(t, "P", "R", "X", "S", "U", "V")
	f.add(t, f.applications["P"], "fact-P")
	f.edge(t, "P", "R")
	f.edge(t, "X", "S")
	f.add(t, f.applications["U"], "fact-U")
	f.add(t, f.applications["V"], "fact-V")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["R"]))
	coneMust(t, f.set.SetOutputPredicate(f.declarations["S"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("cone converter fixture rejected")
	}
	plugin := transforms.NewConePlugin(1)
	filtered, err := plugin.Transform(f.set)
	coneMust(t, err)
	if filtered == nil {
		t.Fatal("changed cone returned nil")
	}
	t.Cleanup(func() { coneMust(t, filtered.Close()) })
	steps, err := plugin.TakeModelSteps()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transforms.CloseConeModelSteps(steps)) })
	if len(steps) != 2 {
		t.Fatal("cone converter pass count", len(steps))
	}
	if len(steps[0]) != 2 || steps[0][0].Declaration != f.declarations["U"] || steps[0][1].Declaration != f.declarations["V"] {
		t.Fatal("top-down converter definitions lost source rule order", steps[0])
	}
	foundTop, foundBottom := false, false
	for _, definition := range steps[0] {
		if definition.Declaration != f.declarations["U"] {
			continue
		}
		formula, getErr := f.manager.RefGet(definition.Formula)
		coneMust(t, getErr)
		foundTop = f.manager.IsTrue(formula)
	}
	for _, definition := range steps[1] {
		// In dataflow.h:70-76, step_bottom_up creates a head fact entry only
		// while evaluating a rule reached through m_body2rules. With no seed
		// for X, X -> S is never evaluated, so S is absent from engine facts.
		// dl_mk_coi_filter.cpp:81-86 then emits false for enumerated engine
		// facts followed by explicitly scanned unreachable tails: X, not S.
		if definition.Declaration == f.declarations["S"] {
			t.Fatal("bottom-up converter fabricated an entry for an unevaluated head")
		}
		if definition.Declaration != f.declarations["X"] {
			continue
		}
		formula, getErr := f.manager.RefGet(definition.Formula)
		coneMust(t, getErr)
		foundBottom = f.manager.IsFalse(formula)
	}
	if !foundTop || !foundBottom {
		t.Fatal("cone converter definitions", foundTop, foundBottom)
	}
	coneMust(t, plugin.Close())
}

func TestConeOfInfluenceReturnsNilWhenUnchanged(t *testing.T) {
	f := newConeFixture(t, "P")
	f.add(t, f.applications["P"], "fact-P")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["P"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("single-fact cone rejected")
	}
	filtered, err := transforms.ConeOfInfluence(f.set)
	coneMust(t, err)
	if filtered != nil {
		coneMust(t, filtered.Close())
		t.Fatal("unchanged cone returned a replacement")
	}
}

func TestConeOfInfluenceBailsOutWhenNegativeTailHasMaterializedFacts(t *testing.T) {
	for _, sparse := range []bool{false, true} {
		name := "hash"
		if sparse {
			name = "sparse"
		}
		t.Run(name, func(t *testing.T) { testConeOfInfluenceBailsOutWhenNegativeTailHasMaterializedFacts(t, sparse) })
	}
}

func testConeOfInfluenceBailsOutWhenNegativeTailHasMaterializedFacts(t *testing.T, sparse bool) {
	f := newConeFixture(t, "P", "E")
	addTransformRule(t, f, f.applications["P"], []ast.ASTHandle{f.applications["E"]}, []bool{true}, "negative-fact-table")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["P"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("fact-table fixture is unstratified")
	}
	relations, err := rel.NewRelationManager(f.manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, relations.Close()) })
	table, err := sparseTransformRelation(relations, f.declarations["E"], sparse)
	coneMust(t, err)
	if inserted, addErr := table.Add(rel.Fact{}); addErr != nil || !inserted {
		t.Fatalf("materialize E fact: inserted=%v err=%v", inserted, addErr)
	}
	result, err := transforms.ConeOfInfluenceWithRelations(f.set, relations)
	coneMust(t, err)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("cone filter ignored the source fact-table bailout")
	}
}

func TestRuleTransformerAppliesConePlugin(t *testing.T) {
	f := newConeFixture(t, "P", "U")
	f.add(t, f.applications["P"], "fact-P")
	f.add(t, f.applications["U"], "fact-U")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["P"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("transformer cone fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(transforms.NewConePlugin(0)))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("cone plugin did not report its pruning")
	}
	if count, err := f.set.Len(); err != nil || count != 1 {
		t.Fatal("transformed rule count", count, err)
	}
	if finalized, err := f.set.IsFinalized(); err != nil || !finalized {
		t.Fatal("transformed source finalization", finalized, err)
	}
	rule, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	head, err := rule.HeadDeclaration()
	coneMust(t, err)
	if head != f.declarations["P"] {
		t.Fatal("cone plugin retained wrong fact", head)
	}
}
