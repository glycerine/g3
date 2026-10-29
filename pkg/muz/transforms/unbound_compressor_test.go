package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestUnboundCompressorProjectsHeadAndTailArgument(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	q := declareTransformPredicate(t, f, "compress-Q", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	p := declareTransformPredicate(t, f, "compress-P", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{x, y})
	goal := declareTransformPredicate(t, f, "compress-Goal", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	addTransformRule(t, f, p.application, []ast.ASTHandle{q.application}, nil, "definition")
	addTransformRule(t, f, goal.application, []ast.ASTHandle{p.application}, nil, "consumer")
	coneMust(t, f.set.SetOutputPredicate(goal.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("compressor fixture rejected")
	}
	plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("unbound head argument was not compressed")
	}
	defer func() { coneMust(t, result.Close()) }()
	mapping := plugin.CompressedPredicates()
	compressed := mapping[p.declaration]
	if compressed.IsNull() {
		t.Fatal("missing compressed predicate")
	}
	if arity, err := f.manager.GetArity(compressed); err != nil || arity != 1 {
		t.Fatal("compressed arity", arity, err)
	}
	definitions, err := result.PredicateRules(compressed)
	coneMust(t, err)
	if len(definitions) != 1 {
		t.Fatalf("compressed definitions %d, want 1", len(definitions))
	}
	coneMust(t, definitions[0].Close())
	goals, err := result.PredicateRules(goal.declaration)
	coneMust(t, err)
	if len(goals) != 1 {
		t.Fatalf("goal rules %d, want 1", len(goals))
	}
	if declaration, err := goals[0].TailDeclaration(0); err != nil || declaration != compressed {
		t.Fatal("goal tail was not decompressed", declaration, err)
	}
	coneMust(t, goals[0].Close())
}

func TestUnboundCompressorSkipsOutputsDisabledAndBoundHeads(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	p := declareTransformPredicate(t, f, "compress-skip-P", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	addTransformRule(t, f, p.application, []ast.ASTHandle{p.application}, nil, "bound")
	coneMust(t, f.set.SetOutputPredicate(p.declaration))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("skip fixture rejected")
	}
	for _, enabled := range []bool{false, true} {
		plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, enabled, 0)
		result, transformErr := plugin.Transform(f.set)
		coneMust(t, transformErr)
		if result != nil {
			coneMust(t, result.Close())
			t.Fatal("skip fixture changed", enabled)
		}
		coneMust(t, plugin.Close())
	}
}
