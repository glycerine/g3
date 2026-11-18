package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::mod2div.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLegacyArithmeticProjectionRewritesModEquality(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArithmeticProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("legacy-mod-x", f.integer)
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	two := f.own(m.ArithmeticIntegerNumeralText("2"))
	three := f.own(m.ArithmeticIntegerNumeralText("3"))
	sum := f.own(m.MkArithmeticAdd(x, one))
	modulus := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{sum, three}))
	equality := f.own(m.MkEq(modulus, two))
	rewritten, err := p.modToDivisibility(equality)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&rewritten)) }()
	handle, err := m.RefGet(rewritten)
	fmMust(t, err)
	family, err := m.GetFamilyID(handle)
	fmMust(t, err)
	op, err := m.GetDeclKind(handle)
	fmMust(t, err)
	count, err := m.GetNumArgs(handle)
	fmMust(t, err)
	if family != m.BasicFamilyID() || op != ast.ASTOpAnd || count != 3 {
		t.Fatal("mod equality expansion", family, op, count)
	}
	divides, err := m.GetArg(handle, 0)
	fmMust(t, err)
	args, matched, err := p.syntax.operation(divides, m.BasicFamilyID(), ast.ASTOpEq)
	fmMust(t, err)
	if !matched || len(args) != 2 {
		t.Fatal("missing divisibility equality")
	}
	mod, matched, err := p.syntax.operation(args[0], f.r.arithmetic, 13)
	fmMust(t, err)
	if !matched || len(mod) != 2 {
		t.Fatal("missing top-level divisibility modulus")
	}
	zero, err := p.syntax.manager.ArithmeticNumeralIsZero(args[1])
	fmMust(t, err)
	if !zero {
		t.Fatal("divisibility right side is not zero")
	}
}
