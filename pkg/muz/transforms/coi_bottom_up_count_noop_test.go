package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

// Source bottom_up intentionally detects change by rule count alone, even
// after constructing a replacement that removes an unreachable negative tail.
func TestConeBottomUpDiscardsNegativeReplacementWhenRuleCountIsUnchanged(t *testing.T) {
	f := newConeFixture(t, "Goal", "Missing")
	addTransformRule(t, f, f.applications["Goal"], []ast.ASTHandle{f.applications["Missing"]}, []bool{true}, "negative-only")
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("unstratified fixture")
	}
	before := f.manager.Stats().LiveReferences
	plugin := transforms.NewConePlugin(0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("source count-only no-change branch published a replacement")
	}
	steps, err := plugin.TakeModelSteps()
	coneMust(t, err)
	defer func() { coneMust(t, transforms.CloseConeModelSteps(steps)) }()
	if len(steps) != 0 {
		t.Fatal("unchanged pass published model definitions", len(steps))
	}
	if after := f.manager.Stats().LiveReferences; after != before {
		t.Fatal("discarded negative replacement leaked AST roots", before, after)
	}
}
