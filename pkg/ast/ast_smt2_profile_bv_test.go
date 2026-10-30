package ast

// Source: src/ast/ast_smt2_pp.cpp:629,1076,1078.

import "testing"

func TestASTSMT2PrinterParamsDriveBVRadixAndNegativeSpelling(t *testing.T) {
	m, c, g, local, budget := smt2ProfileFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	paramsMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	paramsMust(t, err)
	literal, err := m.BitVectorNumeralUnsigned(15, 4)
	paramsMust(t, err)
	key := "bv_literals"
	paramsMust(t, c.SetBoolName(local, &key, false))
	key = "bv_neg"
	paramsMust(t, c.SetBoolName(local, &key, true))
	format, err := m.SMT2PPExprTreeWithParams(literal, nil, c, g, local)
	paramsMust(t, err)
	if got := smt2ProfileRender(t, m, format, c, g, local, budget); got != "(bvneg (_ bv1 4))" {
		t.Fatal("parameter-backed BV spelling", got)
	}
}
