package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"testing"
)

func TestNonlinearArithmeticPluginEndToEndBranchesAssignSubstitute(t *testing.T) {
	f := newArithmeticQEFixture(t)
	context, err := qe.NewSolverContext(f.manager, []ast.ASTHandle{f.rx})
	pluginMust(t, err)
	plugin, err := qe.NewNonlinearArithmeticPlugin(context, false)
	pluginMust(t, err)
	pluginMust(t, context.AddPlugin(plugin))
	defer func() { pluginMust(t, context.Close()) }()
	contains, err := ast.NewContainsApp(f.manager, f.rx)
	pluginMust(t, err)
	defer func() { pluginMust(t, contains.Close()) }()
	minusOne := f.realNumeral(t, "-1")
	zero := f.realNumeral(t, "0")
	x2 := f.pin(f.manager.MkArithmeticMul(f.rx, f.rx))
	poly := f.pin(f.manager.MkArithmeticAdd(x2, minusOne))
	formula := f.pin(f.manager.MkArithmeticLE(poly, zero))
	ok, err := plugin.Prepare(contains, formula, []ast.ASTHandle{formula}, nil)
	pluginMust(t, err)
	if !ok {
		t.Fatal("prepare")
	}
	service := f.manager.NumeralService()
	count, err := service.NewValue()
	pluginMust(t, err)
	defer func() { pluginMust(t, service.Del(&count)) }()
	handled, err := plugin.GetNumBranches(contains, formula, &count)
	pluginMust(t, err)
	text, err := service.String(&count)
	pluginMust(t, err)
	if !handled || text != "2" {
		t.Fatal("branch count", handled, text)
	}
	branch, err := service.NewValue()
	pluginMust(t, err)
	pluginMust(t, service.SetUnsigned(&branch, 0))
	defer func() { pluginMust(t, service.Del(&branch)) }()
	pluginMust(t, plugin.Assign(contains, formula, &branch))
	constraints, err := plugin.TakeConstraints()
	pluginMust(t, err)
	if len(constraints) != 1 {
		t.Fatal("assigned constraints", len(constraints))
	}
	pluginMust(t, f.manager.DelRef(&constraints[0].Formula))
	result, err := f.manager.NewRef(formula)
	pluginMust(t, err)
	defer func() { pluginMust(t, f.manager.DelRef(&result)) }()
	definition, err := f.manager.NewRef(ast.ASTHandle{})
	pluginMust(t, err)
	defer func() { pluginMust(t, f.manager.DelRef(&definition)) }()
	pluginMust(t, plugin.Substitute(contains, &branch, result, &definition))
	h, err := f.manager.RefGet(result)
	pluginMust(t, err)
	if found, err := contains.Check(h); err != nil || found {
		t.Fatal("substitution retained x", found, err)
	}
}

func TestNonlinearArithmeticPluginClassificationAndModelMode(t *testing.T) {
	f := newArithmeticQEFixture(t)
	context, err := qe.NewSolverContext(f.manager, nil)
	pluginMust(t, err)
	plugin, err := qe.NewNonlinearArithmeticPlugin(context, false)
	pluginMust(t, err)
	defer func() { pluginMust(t, plugin.Close()); pluginMust(t, context.Close()) }()
	two := f.realNumeral(t, "2")
	linear := f.pin(f.manager.MkArithmeticMul(two, f.rx))
	nonlinear := f.pin(f.manager.MkArithmeticMul(f.rx, f.ry))
	if opaque, err := plugin.IsUninterpreted(linear); err != nil || opaque {
		t.Fatal("linear classification", opaque, err)
	}
	if opaque, err := plugin.IsUninterpreted(nonlinear); err != nil || !opaque {
		t.Fatal("nonlinear classification", opaque, err)
	}
	modelPlugin, err := qe.NewNonlinearArithmeticPlugin(context, true)
	pluginMust(t, err)
	defer func() { pluginMust(t, modelPlugin.Close()) }()
	if opaque, err := modelPlugin.IsUninterpreted(linear); err != nil || !opaque {
		t.Fatal("model classification", opaque, err)
	}
}
