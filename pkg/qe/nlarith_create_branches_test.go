package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"testing"
)

func TestNLCreateQuadraticBranchesSubstitutionShapeAndOwnership(t *testing.T) {
	f := newArithmeticQEFixture(t)
	u, contains, p, polyExpr := nonlinearQuadratic(t, f)
	pluginMust(t, p.Close())
	defer func() { pluginMust(t, contains.Close()); pluginMust(t, u.Close()) }()
	pluginMust(t, u.SetEnableLinear(true))
	zero := f.realNumeral(t, "0")
	literal := f.pin(f.manager.MkArithmeticLE(polyExpr, zero))
	branches, err := qe.NewNLBranchConditions(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, branches.Close()) }()
	ok, err := u.CreateBranches(contains, []ast.ASTHandle{literal}, branches)
	pluginMust(t, err)
	if !ok {
		t.Fatal("quadratic branches rejected")
	}
	if len(branches.Predicates) != 1 || len(branches.Branches) != 2 {
		t.Fatal("branch count", len(branches.Predicates), len(branches.Branches))
	}
	for i, row := range branches.Branches {
		if len(row.Substitutions) != 1 {
			t.Fatal("substitution count", i)
		}
		sub, err := f.manager.RefGet(row.Substitutions[0])
		pluginMust(t, err)
		if found, err := contains.Check(sub); err != nil || found {
			t.Fatal("branch substitution retained x", i, found, err)
		}
		definition, err := f.manager.RefGet(row.Definition)
		pluginMust(t, err)
		if found, err := contains.Check(definition); err != nil || found {
			t.Fatal("definition retained x", i, found, err)
		}
	}
}

func TestNLCreateBranchesLinearGateAndDegreeThreeRejection(t *testing.T) {
	f := newArithmeticQEFixture(t)
	u, err := qe.NewNonlinearArithmeticUtil(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, u.Close()) }()
	contains, err := ast.NewContainsApp(f.manager, f.rx)
	pluginMust(t, err)
	defer func() { pluginMust(t, contains.Close()) }()
	zero := f.realNumeral(t, "0")
	linear := f.pin(f.manager.MkArithmeticLE(f.rx, zero))
	branches, err := qe.NewNLBranchConditions(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, branches.Close()) }()
	ok, err := u.CreateBranches(contains, []ast.ASTHandle{linear}, branches)
	pluginMust(t, err)
	if ok {
		t.Fatal("disabled linear handling accepted")
	}
	pluginMust(t, u.SetEnableLinear(true))
	ok, err = u.CreateBranches(contains, []ast.ASTHandle{linear}, branches)
	pluginMust(t, err)
	if !ok || len(branches.Branches) != 2 {
		t.Fatal("linear branches", ok, len(branches.Branches))
	}
	x2 := f.pin(f.manager.MkArithmeticMul(f.rx, f.rx))
	x3 := f.pin(f.manager.MkArithmeticMul(x2, f.rx))
	cubic := f.pin(f.manager.MkArithmeticLE(x3, zero))
	ok, err = u.CreateBranches(contains, []ast.ASTHandle{cubic}, branches)
	pluginMust(t, err)
	if ok {
		t.Fatal("degree-three accepted")
	}
}

func TestNLCreateBranchesDeclinesNonRealTargets(t *testing.T) {
	f := newArithmeticQEFixture(t)
	u, err := qe.NewNonlinearArithmeticUtil(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, u.Close()) }()
	branches, err := qe.NewNLBranchConditions(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, branches.Close()) }()
	boolean := f.pin(f.manager.MkBoolSort())
	name, err := f.manager.Symbols.Intern("non-real-variable")
	pluginMust(t, err)
	value := f.pin(f.manager.MkConst(name, boolean))
	for _, target := range []ast.ASTHandle{value, f.x} {
		contains, e := ast.NewContainsApp(f.manager, target)
		pluginMust(t, e)
		ok, e := u.CreateBranches(contains, nil, branches)
		pluginMust(t, e)
		pluginMust(t, contains.Close())
		if ok || len(branches.Branches) != 0 {
			t.Fatal("accepted non-real variable", target)
		}
	}
}
