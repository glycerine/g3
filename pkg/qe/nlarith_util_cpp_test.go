package qe_test

// Source: src/test/nlarith_util.cpp.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"testing"
)

func TestNLArithmeticUtilSourceTwoQuadratics(t *testing.T) {
	f := newArithmeticQEFixture(t)
	m := f.manager
	z := f.pin(m.MkFreshConst("qe-nlarith-z", f.real))
	one := f.realNumeral(t, "1")
	family, err := m.ArithmeticFamilyID()
	pluginMust(t, err)
	product := f.pin(m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{f.rx, f.rx, f.ry}))
	p1 := f.pin(m.MkArithmeticLE(f.pin(m.MkArithmeticAdd(product, f.ry)), one))
	p2 := f.pin(m.MkArithmeticLE(f.pin(m.MkArithmeticAdd(product, z)), one))
	u, err := qe.NewNonlinearArithmeticUtil(m)
	pluginMust(t, err)
	defer func() { pluginMust(t, u.Close()) }()
	contains, err := ast.NewContainsApp(m, f.rx)
	pluginMust(t, err)
	defer func() { pluginMust(t, contains.Close()) }()
	branches, err := qe.NewNLBranchConditions(m)
	pluginMust(t, err)
	defer func() { pluginMust(t, branches.Close()) }()
	ok, err := u.CreateBranches(contains, []ast.ASTHandle{p1, p2}, branches)
	pluginMust(t, err)
	if !ok {
		t.Fatal("source quadratics were declined")
	}
	if len(branches.Predicates) != 2 || len(branches.Branches) == 0 {
		t.Fatal(len(branches.Predicates), len(branches.Branches))
	}
	for _, branch := range branches.Branches {
		if len(branch.Substitutions) != 2 {
			t.Fatal("missing substitution", len(branch.Substitutions))
		}
		for _, ref := range branch.Substitutions {
			h, err := m.RefGet(ref)
			pluginMust(t, err)
			found, err := contains.Check(h)
			pluginMust(t, err)
			if found {
				t.Fatal("substitution still contains eliminated x")
			}
		}
	}
}
