package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"testing"
)

func TestNLArithmeticLiteralAnalysisComparisonsAndDegreePolicy(t *testing.T) {
	f := newArithmeticQEFixture(t)
	u, err := qe.NewNonlinearArithmeticUtil(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, u.Close()) }()
	contains, err := ast.NewContainsApp(f.manager, f.rx)
	pluginMust(t, err)
	defer func() { pluginMust(t, contains.Close()) }()
	zero := f.realNumeral(t, "0")
	x2 := f.pin(f.manager.MkArithmeticMul(f.rx, f.rx))
	le := f.pin(f.manager.MkArithmeticLE(x2, zero))
	neqBase := f.pin(f.manager.MkEq(f.rx, f.ry))
	neq := f.pin(f.manager.MkNot(neqBase))
	analysis, ok, err := u.Analyze(contains, []ast.ASTHandle{le, neq})
	pluginMust(t, err)
	if !ok {
		t.Fatal("analysis rejected")
	}
	defer func() { pluginMust(t, analysis.Close()) }()
	if len(analysis.Literals) != 2 || analysis.Literals[0].Comparison != qe.NLComparisonLE || analysis.Literals[1].Comparison != qe.NLComparisonNE {
		t.Fatal("comparison order")
	}
	degree, err := analysis.MaxDegree()
	pluginMust(t, err)
	if degree != 2 {
		t.Fatal("max degree", degree)
	}
	idx, single, err := analysis.HasSingleQuadratic()
	pluginMust(t, err)
	if !single || idx != 0 {
		t.Fatal("single quadratic", idx, single)
	}
	linear, err := analysis.IsLinear()
	pluginMust(t, err)
	if linear {
		t.Fatal("quadratic called linear")
	}
}

func TestNLBranchConditionsOwnResetAndReuse(t *testing.T) {
	f := newArithmeticQEFixture(t)
	b, err := qe.NewNLBranchConditions(f.manager)
	pluginMust(t, err)
	defer func() { pluginMust(t, b.Close()) }()
	truth := f.pin(f.manager.MkTrue())
	zero := f.realNumeral(t, "0")
	one := f.realNumeral(t, "1")
	pluginMust(t, b.AddPredicate(truth))
	substitutions := []ast.ASTHandle{truth}
	pluginMust(t, b.AddBranch(truth, truth, substitutions, zero, one, zero, one))
	substitutions[0] = zero
	if len(b.Predicates) != 1 || len(b.Branches) != 1 || len(b.Branches[0].Substitutions) != 1 {
		t.Fatal("branch table shape")
	}
	stored, err := f.manager.RefGet(b.Branches[0].Substitutions[0])
	pluginMust(t, err)
	if stored != truth {
		t.Fatal("branch substitution changed with caller's slice")
	}
	pluginMust(t, b.Reset())
	if len(b.Predicates) != 0 || len(b.Branches) != 0 {
		t.Fatal("reset retained entries")
	}
	pluginMust(t, b.AddPredicate(truth))
}
