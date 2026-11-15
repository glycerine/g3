package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
	"testing"
)

func TestNonlinearArithmeticPluginEndToEndBranchesAssignSubstitute(t *testing.T) {
	f := newArithmeticQEFixture(t)
	context, err := qe.NewSolverContext(f.manager, []ast.ASTHandle{f.rx})
	pluginMust(t, err)
	plugin, err := qe.NewNonlinearArithmeticPlugin(context, f.config, false)
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
	plugin, err := qe.NewNonlinearArithmeticPlugin(context, f.config, false)
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
	// AUDIT.md qe_04-F2: only a binary product with a numeral is
	// interpreted (a.is_mul(f, m, n) is MATCH_BINARY).
	family, err := f.manager.ArithmeticFamilyID()
	pluginMust(t, err)
	ternary := f.pin(f.manager.MkFamilyApp(family, 9, nil, []ast.ASTHandle{two, f.rx, f.ry}))
	if opaque, err := plugin.IsUninterpreted(ternary); err != nil || !opaque {
		t.Fatal("ternary classification", opaque, err)
	}
	modelPlugin, err := qe.NewNonlinearArithmeticPlugin(context, f.config, true)
	pluginMust(t, err)
	defer func() { pluginMust(t, modelPlugin.Close()) }()
	if opaque, err := modelPlugin.IsUninterpreted(linear); err != nil || !opaque {
		t.Fatal("model classification", opaque, err)
	}
}

// AUDIT.md qe_04-F1: nlarith_plugin::assign adds th_rewriter(factor_rw(
// branch)) and subst conjoins the branch constraint with the simultaneously
// substituted formula and factors the result.
func TestNonlinearArithmeticPluginAssignAndSubstituteFactor(t *testing.T) {
	f := newArithmeticQEFixture(t)
	m := f.manager
	context, err := qe.NewSolverContext(m, []ast.ASTHandle{f.rx})
	pluginMust(t, err)
	plugin, err := qe.NewNonlinearArithmeticPlugin(context, f.config, false)
	pluginMust(t, err)
	pluginMust(t, context.AddPlugin(plugin))
	defer func() { pluginMust(t, context.Close()) }()
	contains, err := ast.NewContainsApp(m, f.rx)
	pluginMust(t, err)
	defer func() { pluginMust(t, contains.Close()) }()
	minusOne := f.realNumeral(t, "-1")
	zero := f.realNumeral(t, "0")
	x2 := f.pin(m.MkArithmeticMul(f.rx, f.rx))
	poly := f.pin(m.MkArithmeticAdd(x2, minusOne))
	formula := f.pin(m.MkArithmeticLE(poly, zero))
	ok, err := plugin.Prepare(contains, formula, []ast.ASTHandle{formula}, nil)
	pluginMust(t, err)
	if !ok {
		t.Fatal("prepare")
	}
	// The same branch table, built independently (hash-consing makes the
	// handles equal).
	util, err := qe.NewNonlinearArithmeticUtilConfig(m, f.config)
	pluginMust(t, err)
	defer func() { pluginMust(t, util.Close()) }()
	pluginMust(t, util.SetEnableLinear(true))
	branches, err := qe.NewNLBranchConditions(m)
	pluginMust(t, err)
	defer func() { pluginMust(t, branches.Close()) }()
	ok, err = util.CreateBranches(contains, []ast.ASTHandle{formula}, branches)
	pluginMust(t, err)
	if !ok || len(branches.Branches) == 0 {
		t.Fatal("branches", ok, len(branches.Branches))
	}
	factor, err := astrewriter.NewFactorRewriter(m)
	pluginMust(t, err)
	defer func() { pluginMust(t, factor.Close()) }()
	theory, err := astrewriter.NewTheoryRewriter(m, f.config.Context, f.config.Globals, nil)
	pluginMust(t, err)
	defer func() { pluginMust(t, theory.Close()) }()
	service := m.NumeralService()
	for i, row := range branches.Branches {
		branch, err := service.NewValue()
		pluginMust(t, err)
		pluginMust(t, service.SetUnsigned(&branch, uint32(i)))
		pluginMust(t, plugin.Assign(contains, formula, &branch))
		constraints, err := plugin.TakeConstraints()
		pluginMust(t, err)
		if len(constraints) != 1 {
			t.Fatal("assigned constraints", len(constraints))
		}
		got, err := m.RefGet(constraints[0].Formula)
		pluginMust(t, err)
		raw, err := m.RefGet(row.Branch)
		pluginMust(t, err)
		tmp, err := m.NewRef(ast.ASTHandle{})
		pluginMust(t, err)
		pluginMust(t, factor.Rewrite(raw, tmp))
		factored, err := m.RefGet(tmp)
		pluginMust(t, err)
		pluginMust(t, theory.Rewrite(factored, tmp))
		want, err := m.RefGet(tmp)
		pluginMust(t, err)
		if got != want {
			t.Fatal("assign must add th_rewriter(factor_rw(branch))", i)
		}
		// subst: factor_rw(and(constraint, fml[pred := subst])).
		result, err := m.NewRef(formula)
		pluginMust(t, err)
		pluginMust(t, plugin.Substitute(contains, &branch, result, nil))
		gotSubst, err := m.RefGet(result)
		pluginMust(t, err)
		sub, err := m.RefGet(row.Substitutions[0])
		pluginMust(t, err)
		constraint, err := m.RefGet(row.Constraint)
		pluginMust(t, err)
		and, err := m.MkAnd(constraint, sub)
		pluginMust(t, err)
		pluginMust(t, factor.Rewrite(and, tmp))
		wantSubst, err := m.RefGet(tmp)
		pluginMust(t, err)
		if gotSubst != wantSubst {
			t.Fatal("subst must factor and(constraint, substituted formula)", i)
		}
		pluginMust(t, m.DelRef(&result))
		pluginMust(t, m.DelRef(&tmp))
		pluginMust(t, m.DelRef(&constraints[0].Formula))
		pluginMust(t, service.Del(&branch))
	}
}
