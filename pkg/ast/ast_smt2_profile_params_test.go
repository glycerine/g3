package ast

// Source: src/ast/ast_smt2_pp.cpp:1063-1082.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func smt2ProfileFixture(t *testing.T) (*ASTManager, *uparams.ParamsContext, *uparams.GlobalParamsService, *uparams.ParamsRef, *arena.Budget) {
	t.Helper()
	c, g, p, budget := dynAckSourceFixture(t)
	m, err := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: budget})
	paramsMust(t, err)
	t.Cleanup(func() { paramsMust(t, m.Close()) })
	return m, c, g, p, budget
}

func smt2ProfileRender(t *testing.T, m *ASTManager, format ASTHandle, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, budget *arena.Budget) string {
	t.Helper()
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: budget})
	paramsMust(t, err)
	defer func() { paramsMust(t, out.Close()) }()
	paramsMust(t, m.PrintFormat(out, format, c, p, g))
	text, err := out.String()
	paramsMust(t, err)
	return text
}

func TestASTSMT2PrinterProfileConstructorOrderDefaultsAndLocalValues(t *testing.T) {
	_, c, g, local, _ := smt2ProfileFixture(t)
	profile, err := SMT2PPExprProfileFromParams(c, g, nil)
	paramsMust(t, err)
	if profile != defaultSMT2PPExprProfile() {
		t.Fatal("source defaults", profile, defaultSMT2PPExprProfile())
	}
	for key, value := range map[string]bool{"decimal": true, "bv_literals": false, "fp_real_literals": true, "bv_neg": true, "flat_assoc": false} {
		name := key
		paramsMust(t, c.SetBoolName(local, &name, value))
	}
	for key, value := range map[string]uint32{"decimal_precision": 17, "max_depth": 19, "min_alias_size": 23} {
		name := key
		paramsMust(t, c.SetUIntName(local, &name, value))
	}
	profile, err = SMT2PPExprProfileFromParams(c, g, local)
	paramsMust(t, err)
	want := SMT2PPExprProfile{Decimal: true, DecimalPrecision: 17, BVLiterals: false, FPRealLiterals: true, BVNegative: true, MaxDepth: 19, MinAliasSize: 23, FlatAssoc: false}
	if profile != want {
		t.Fatal("constructor fields", profile, want)
	}
}
