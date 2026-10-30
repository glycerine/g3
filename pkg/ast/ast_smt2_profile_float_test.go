package ast

// Source: src/ast/ast_smt2_pp.cpp:632,1077.

import (
	"testing"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestASTSMT2PrinterParamsDriveFPRealLiterals(t *testing.T) {
	m, c, g, local, budget := smt2ProfileFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	paramsMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	paramsMust(t, err)
	_, err = m.RegisterFloatingPointPlugin()
	paramsMust(t, err)
	significand := unumeral.NewInt(4)
	var value unumeral.ArbitraryFloat
	paramsMust(t, m.floatingPointPlugin.numeric.SetComponents(&value, 8, 4, false, 3, &significand))
	node, err := m.floatingPointPlugin.mkNumeral(&value, nil)
	paramsMust(t, err)
	defer func() { paramsMust(t, m.floatingPointPlugin.numeric.Del(&value)) }()
	key := "fp_real_literals"
	paramsMust(t, c.SetBoolName(local, &key, true))
	format, err := m.SMT2PPExprTreeWithParams(m.external(node), nil, c, g, local)
	paramsMust(t, err)
	if got := smt2ProfileRender(t, m, format, c, g, local, budget); got != "((_ to_fp 8 4) RTZ 1.5 3)" {
		t.Fatal("parameter-backed FP literal", got)
	}
}
