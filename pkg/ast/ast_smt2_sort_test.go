package ast

import (
	"testing"

	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestASTSMT2SortSourceBuiltinAndParametricFormats(t *testing.T) {
	c, g, p, budget := dynAckSourceFixture(t)
	m, err := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: budget})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.Close()) }()
	_, err = m.RegisterArithmeticPlugin()
	paramsMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	paramsMust(t, err)
	_, err = m.RegisterArrayPlugin()
	paramsMust(t, err)
	_, err = m.RegisterFloatingPointPlugin()
	paramsMust(t, err)
	seqFamily, err := m.RegisterSequencePlugin()
	paramsMust(t, err)
	datatypeFamily, err := m.RegisterDatatypePlugin()
	paramsMust(t, err)
	boolSort, err := m.MkBoolSort()
	paramsMust(t, err)
	intSort := m.external(m.arithmeticPlugin.fields[1])
	realSort := m.external(m.arithmeticPlugin.fields[0])
	bvSort, err := m.BitVectorSort(13)
	paramsMust(t, err)
	arraySort, err := m.MkArraySort(SliceArguments([]ASTHandle{boolSort}), bvSort)
	paramsMust(t, err)
	ebits, err := m.NewParameterInt(8)
	paramsMust(t, err)
	sbits, err := m.NewParameterInt(24)
	paramsMust(t, err)
	floatSort, err := m.MkFamilySort(m.floatingPointPlugin.family, 0, []ASTParameter{ebits, sbits})
	paramsMust(t, err)
	paramsMust(t, m.DelParameter(&ebits))
	paramsMust(t, m.DelParameter(&sbits))
	seqParam, err := m.NewParameterAST(boolSort)
	paramsMust(t, err)
	seqSort, err := m.MkFamilySort(seqFamily, 0, []ASTParameter{seqParam})
	paramsMust(t, err)
	reSort, err := m.MkFamilySort(seqFamily, 1, []ASTParameter{seqParam})
	paramsMust(t, err)
	paramsMust(t, m.DelParameter(&seqParam))
	stringSort := m.external(m.sequencePlugin.stringSort)
	custom, err := m.MkSort(astTestSymbol(t, m, "Custom"), ASTSortInfo{})
	paramsMust(t, err)
	userArgument, err := m.NewParameterAST(boolSort)
	paramsMust(t, err)
	userParametric, err := m.MkUninterpretedSort(astTestSymbol(t, m, "UserBox"), []ASTParameter{userArgument})
	paramsMust(t, err)
	paramsMust(t, m.DelParameter(&userArgument))
	typeName, err := m.NewParameterSymbol(astTestSymbol(t, m, "Box"))
	paramsMust(t, err)
	typeArg, err := m.NewParameterAST(boolSort)
	paramsMust(t, err)
	typeInfo, err := m.NewSortInfo(datatypeFamily, 0, []ASTParameter{typeName, typeArg}, InfiniteSortSize(), false)
	paramsMust(t, err)
	parametric, err := m.MkSort(astTestSymbol(t, m, "Box"), typeInfo)
	paramsMust(t, err)
	paramsMust(t, m.DelSortInfo(&typeInfo))
	paramsMust(t, m.DelParameter(&typeName))
	paramsMust(t, m.DelParameter(&typeArg))
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: budget})
	paramsMust(t, err)
	defer func() { paramsMust(t, out.Close()) }()
	for _, tc := range []struct {
		sort ASTHandle
		want string
	}{
		{boolSort, "Bool"}, {intSort, "Int"}, {realSort, "Real"},
		{bvSort, "(_ BitVec 13)"}, {arraySort, "(Array Bool (_ BitVec 13))"},
		{floatSort, "(_ FloatingPoint 8 24)"}, {stringSort, "String"},
		{seqSort, "(Seq Bool)"}, {reSort, "(RegEx Bool)"},
		{parametric, "(Box Bool)"}, {userParametric, "UserBox"}, {custom, "Custom"},
	} {
		format, e := m.SMT2PPSort(tc.sort)
		paramsMust(t, e)
		paramsMust(t, m.PrintFormat(out, format, c, p, g))
		got, e := out.String()
		paramsMust(t, e)
		if got != tc.want {
			t.Fatalf("sort %v: %q, want %q", tc.sort, got, tc.want)
		}
		paramsMust(t, out.ResetText())
	}
	if _, err := m.SMT2PPSort(ASTHandle{}); err == nil {
		t.Fatal("invalid sort accepted")
	}
}
