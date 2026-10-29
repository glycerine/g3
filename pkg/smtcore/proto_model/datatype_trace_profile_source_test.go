package protomodel

// Source: src/ast/ast_smt2_pp.cpp mk_ismt2_pp and pp_params::updt_params.
// These tests go through datatypeFactoryTraceWriter.ast so the factory's
// concrete parameter acquisition, formatter ownership, and output path are
// covered together rather than testing a detached profile value.

import (
	"bytes"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func renderDatatypeFactoryProfile(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) (string, ast.SMT2PPExprProfile) {
	t.Helper()
	var out bytes.Buffer
	w := &datatypeFactoryTraceWriter{manager: m, out: &out}
	protoMust(t, w.ast(h))
	if w.parameters != nil || w.stream != nil || w.format != (ast.ASTRef{}) {
		t.Fatal("datatype trace printer returned with scoped owners", w.parameters, w.stream, w.format)
	}
	if w.formatManager == nil {
		t.Fatal("datatype trace expression did not construct a format document")
	}
	if got := w.formatManager.Stats().LiveReferences; got != 0 {
		t.Fatal("datatype trace printer retained its format document", got)
	}
	return out.String(), w.profile
}

func TestDatatypeFactoryPrinterProfileDecimalAndPrecision(t *testing.T) {
	var trace bytes.Buffer
	m, _, _, _, _ := datatypeFactoryTraceFixture(t, &trace)
	literal, err := m.ArithmeticNumeralRat(big.NewRat(1, 3), false)
	protoMust(t, err)
	protoPin(t, m, literal)
	g := uparams.BorrowProcessGlobalParamsService()
	protoMust(t, g.SourceSetName("pp.decimal", "true"))

	for _, tc := range []struct {
		precision uint32
		want      string
	}{
		{precision: 3, want: "0.333?"},
		{precision: 5, want: "0.33333?"},
	} {
		protoMust(t, g.SourceSetName("pp.decimal_precision", new(big.Int).SetUint64(uint64(tc.precision)).String()))
		got, profile := renderDatatypeFactoryProfile(t, m, literal)
		if got != tc.want || !profile.Decimal || profile.DecimalPrecision != tc.precision {
			t.Fatalf("factory decimal profile precision %d: got %q, profile %+v", tc.precision, got, profile)
		}
	}
}

func TestDatatypeFactoryPrinterProfileFlatAssociative(t *testing.T) {
	var trace bytes.Buffer
	m, _, _, _, _ := datatypeFactoryTraceFixture(t, &trace)
	realLiteral, err := m.ArithmeticNumeralRat(big.NewRat(0, 1), false)
	protoMust(t, err)
	realSort, err := m.GetSort(realLiteral)
	protoMust(t, err)
	x, err := m.MkConst(protoSymbol(t, m, "flat-x"), realSort)
	protoMust(t, err)
	y, err := m.MkConst(protoSymbol(t, m, "flat-y"), realSort)
	protoMust(t, err)
	z, err := m.MkConst(protoSymbol(t, m, "flat-z"), realSort)
	protoMust(t, err)
	inner, err := m.MkArithmeticAdd(y, z)
	protoMust(t, err)
	root, err := m.MkArithmeticAdd(x, inner)
	protoMust(t, err)
	protoPin(t, m, root)
	g := uparams.BorrowProcessGlobalParamsService()
	protoMust(t, g.SourceSetName("pp.max_depth", "100"))
	protoMust(t, g.SourceSetName("pp.min_alias_size", "100"))

	for _, tc := range []struct {
		flat bool
		want string
	}{
		{flat: true, want: "(+ flat-x flat-y flat-z)"},
		{flat: false, want: "(+ flat-x (+ flat-y flat-z))"},
	} {
		value := "false"
		if tc.flat {
			value = "true"
		}
		protoMust(t, g.SourceSetName("pp.flat_assoc", value))
		got, profile := renderDatatypeFactoryProfile(t, m, root)
		if got != tc.want || profile.FlatAssoc != tc.flat {
			t.Fatalf("factory flat_assoc=%v: got %q, profile %+v", tc.flat, got, profile)
		}
	}
}

func TestDatatypeFactoryPrinterProfileMinimumAliasSize(t *testing.T) {
	var trace bytes.Buffer
	m, _, _, _, _ := datatypeFactoryTraceFixture(t, &trace)
	one, err := m.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	protoMust(t, err)
	integerSort, err := m.GetSort(one)
	protoMust(t, err)
	x, err := m.MkConst(protoSymbol(t, m, "alias-x"), integerSort)
	protoMust(t, err)
	y, err := m.MkConst(protoSymbol(t, m, "alias-y"), integerSort)
	protoMust(t, err)
	shared, err := m.MkArithmeticAdd(x, y)
	protoMust(t, err)
	root, err := m.MkArithmeticMul(shared, shared)
	protoMust(t, err)
	protoPin(t, m, root)
	g := uparams.BorrowProcessGlobalParamsService()
	protoMust(t, g.SourceSetName("pp.max_depth", "100"))
	protoMust(t, g.SourceSetName("pp.flat_assoc", "false"))

	protoMust(t, g.SourceSetName("pp.min_alias_size", "100"))
	withoutAlias, high := renderDatatypeFactoryProfile(t, m, root)
	if strings.Contains(withoutAlias, "(let ") || high.MinAliasSize != 100 {
		t.Fatal("large minimum alias size unexpectedly introduced an alias", withoutAlias, high)
	}

	protoMust(t, g.SourceSetName("pp.min_alias_size", "1"))
	withAlias, low := renderDatatypeFactoryProfile(t, m, root)
	if !strings.Contains(withAlias, "(let ((a!1 (+ alias-x alias-y)))") || !strings.Contains(withAlias, "(* a!1 a!1)") || low.MinAliasSize != 1 {
		t.Fatal("small minimum alias size did not name the shared expression", withAlias, low)
	}
}

func TestDatatypeFactoryPrinterProfileFloatingPointRealLiterals(t *testing.T) {
	var trace bytes.Buffer
	m, _, _, _, _ := datatypeFactoryTraceFixture(t, &trace)
	floatSort, err := m.FloatingPointSort(8, 4)
	protoMust(t, err)
	zero, err := m.MkFPAZeroOfSort(floatSort, false)
	protoMust(t, err)
	protoPin(t, m, zero)
	g := uparams.BorrowProcessGlobalParamsService()
	protoMust(t, g.SourceSetName("pp.fp_real_literals", "true"))
	got, profile := renderDatatypeFactoryProfile(t, m, zero)
	if got != "(_ +zero 8 4)" || !profile.FPRealLiterals {
		t.Fatal("factory did not acquire pp.fp_real_literals", got, profile)
	}
}

func TestDatatypeFactoryPrinterWholeProfileSourceOrder(t *testing.T) {
	var trace bytes.Buffer
	m, _, _, _, constructors := datatypeFactoryTraceFixture(t, &trace)
	value, err := m.MkApp(constructors[0], nil)
	protoMust(t, err)
	protoPin(t, m, value)
	g := uparams.BorrowProcessGlobalParamsService()
	settings := [][2]string{
		{"pp.decimal", "true"},
		{"pp.decimal_precision", "19"},
		{"pp.bv_literals", "false"},
		{"pp.fp_real_literals", "true"},
		{"pp.bv_neg", "true"},
		{"pp.max_depth", "101"},
		{"pp.min_alias_size", "77"},
		{"pp.flat_assoc", "false"},
	}
	for _, setting := range settings {
		protoMust(t, g.SourceSetName(setting[0], setting[1]))
	}
	got, profile := renderDatatypeFactoryProfile(t, m, value)
	want := ast.SMT2PPExprProfile{
		Decimal:          true,
		DecimalPrecision: 19,
		BVLiterals:       false,
		FPRealLiterals:   true,
		BVNegative:       true,
		MaxDepth:         101,
		MinAliasSize:     77,
		FlatAssoc:        false,
	}
	if got != "tz" || profile != want {
		t.Fatal("factory profile diverged from smt2_printer field order", got, profile, want)
	}
}

func TestDatatypeFactoryPrinterParameterFailurePrecedesDocumentOwnership(t *testing.T) {
	var trace bytes.Buffer
	m, _, _, _, constructors := datatypeFactoryTraceFixture(t, &trace)
	value, err := m.MkApp(constructors[0], nil)
	protoMust(t, err)
	protoPin(t, m, value)
	var out bytes.Buffer
	w := &datatypeFactoryTraceWriter{manager: m, out: &out}
	m.Budget().FailAfter(0)
	err = w.ast(value)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("printer parameter failure was not propagated", err)
	}
	protoMust(t, w.close())
	if out.Len() != 0 || w.parameters != nil || w.stream != nil || w.format != (ast.ASTRef{}) {
		t.Fatal("parameter failure crossed into output or document ownership", out.String(), w.parameters, w.stream, w.format)
	}
	fm, err := m.GetFormatManager()
	protoMust(t, err)
	if got := fm.Stats().LiveReferences; got != 0 {
		t.Fatal("parameter failure retained a format document", got)
	}
}
