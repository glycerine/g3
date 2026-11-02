package transforms_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/dataflow"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

// Finalized dependencies include predicate occurrences inside quantified
// interpreted tails introduced by quantifier abstraction.
func TestConePreservesNestedQuantifiedPredicateDefinition(t *testing.T) {
	f := newConeFixture(t, "Goal", "Unused")
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticIntegerNumeralText("0")
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	name, err := f.manager.Symbols.Intern("nested-P")
	coneMust(t, err)
	predicate, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(predicate))
	variable, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	application, err := f.manager.MkApp(predicate, []ast.ASTHandle{variable})
	coneMust(t, err)
	quantified, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: application})
	coneMust(t, err)
	truth, err := f.manager.MkTrue()
	coneMust(t, err)
	interpreted, err := f.manager.MkEq(quantified, truth)
	coneMust(t, err)
	addTransformRule(t, f, application, nil, nil, "nested-definition")
	addTransformRule(t, f, f.applications["Goal"], []ast.ASTHandle{interpreted}, nil, "nested-consumer")
	addTransformRule(t, f, f.applications["Unused"], nil, nil, "disconnected")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("invalid fixture")
	}
	dependencies, err := f.set.Dependencies(f.declarations["Goal"])
	coneMust(t, err)
	if len(dependencies) != 1 || dependencies[0] != predicate {
		t.Fatal("missing finalized nested dependency", dependencies)
	}
	top, err := dataflow.TopDown(f.set)
	coneMust(t, err)
	if !top.IsReachable(predicate) || top.IsReachable(f.declarations["Unused"]) {
		t.Fatal("nested predicate reachability incorrect")
	}
	known := top.KnownDeclarations()
	if len(known) != 2 || known[0] != f.declarations["Goal"] || known[1] != predicate {
		t.Fatal("nested fact order", known)
	}
	plugin := transforms.NewConePlugin(0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("disconnected rule not removed")
	}
	defer func() { coneMust(t, result.Close()) }()
	if n, e := result.Len(); e != nil || n != 2 {
		t.Fatal("nested definition was pruned", n, e)
	}
	rules, err := result.PredicateRules(predicate)
	coneMust(t, err)
	if len(rules) != 1 {
		t.Fatal("original nested definition lost")
	}
	coneMust(t, rules[0].Close())
	steps, err := plugin.TakeModelSteps()
	coneMust(t, err)
	defer func() { coneMust(t, transforms.CloseConeModelSteps(steps)) }()
	definitions := 0
	for _, step := range steps {
		for _, definition := range step {
			if definition.Declaration != f.declarations["Unused"] {
				t.Fatal("model reconstruction replaced a live quantified dependency")
			}
			formula, err := f.manager.RefGet(definition.Formula)
			coneMust(t, err)
			if !f.manager.IsTrue(formula) {
				t.Fatal("pruned original fact lost its true interpretation")
			}
			definitions++
		}
	}
	if definitions != 1 {
		t.Fatal("pruned fact interpretation count", definitions)
	}

}
