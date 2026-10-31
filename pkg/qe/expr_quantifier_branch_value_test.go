package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestExprQEBranchValuesAndEquality(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("value-p", f.boolean)
	bvSort := f.own(m.BitVectorSort(8))
	b := f.constant("value-bv", bvSort)
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	truth := f.own(m.MkTrue())
	thirteen := f.own(m.BitVectorNumeralUint64(13, 8))
	for _, pair := range [][2]ast.ASTHandle{{p, truth}, {b, thirteen}} {
		decl, e := m.GetDecl(pair[0])
		fmMust(t, e)
		fmMust(t, md.Core().RegisterConstant(decl, pair[1]))
	}
	eval, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation})
	fmMust(t, err)
	defer func() { fmMust(t, eval.Close()) }()
	for _, tc := range []struct {
		variable ast.ASTHandle
		expected string
	}{{p, "1"}, {b, "13"}} {
		value, ok, e := exprQEEvalBranch(m, eval, tc.variable)
		fmMust(t, e)
		if !ok || value.RatString() != tc.expected {
			t.Fatal("model branch value", value.RatString())
		}
	}
	zero, err := exprQEBranchEquality(m, p, big.NewRat(0, 1))
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&zero)) }()
	h, err := m.RefGet(zero)
	fmMust(t, err)
	isNot, arg, err := m.IsNot(h)
	fmMust(t, err)
	if !isNot || arg != p {
		t.Fatal("false Boolean branch")
	}
	one, err := exprQEBranchEquality(m, p, big.NewRat(1, 1))
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&one)) }()
	h, err = m.RefGet(one)
	fmMust(t, err)
	if h != p {
		t.Fatal("true Boolean branch")
	}
	eq, err := exprQEBranchEquality(m, b, big.NewRat(13, 1))
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&eq)) }()
	h, err = m.RefGet(eq)
	fmMust(t, err)
	variable, value, found, err := exprQEIsBranchEquality(m, h)
	fmMust(t, err)
	if !found || variable != b || value.RatString() != "13" {
		t.Fatal("bit-vector equality recognition")
	}
	reverse := f.own(m.MkEq(thirteen, b))
	variable, value, found, err = exprQEIsBranchEquality(m, reverse)
	fmMust(t, err)
	if !found || variable != b || value.RatString() != "13" {
		t.Fatal("reversed equality recognition")
	}
	if _, err = exprQEBranchEquality(m, p, big.NewRat(2, 1)); err == nil {
		t.Fatal("invalid Boolean value accepted")
	}
}
