package ast

// Source: src/ast/ast_smt2_pp.cpp:626,1074-1075.

import "testing"

func TestASTSMT2PrinterParamsDriveDecimalAndPrecision(t *testing.T) {
	m, c, g, local, budget := smt2ProfileFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	paramsMust(t, err)
	value, err := c.NumeralsField.NewValue()
	paramsMust(t, err)
	defer func() { paramsMust(t, c.NumeralsField.CleanupValue(&value)) }()
	paramsMust(t, c.NumeralsField.SetString(&value, "1/3"))
	literal, err := m.ArithmeticNumeral(&value, false)
	paramsMust(t, err)
	key := "decimal"
	paramsMust(t, c.SetBoolName(local, &key, true))
	key = "decimal_precision"
	paramsMust(t, c.SetUIntName(local, &key, 4))
	format, err := m.SMT2PPExprTreeWithParams(literal, nil, c, g, local)
	paramsMust(t, err)
	if got := smt2ProfileRender(t, m, format, c, g, local, budget); got != "0.3333?" {
		t.Fatal("parameter-backed decimal", got)
	}
}
