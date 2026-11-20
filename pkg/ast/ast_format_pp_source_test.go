package ast

// Source: src/ast/pp.cpp:22-132; src/ast/pp_params.pyg:1-23.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestASTFormatPrettyPrinterLiteralWidthIndentAndSingleLine(t *testing.T) {
	c, g, p, b := dynAckSourceFixture(t)
	m, e := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: b})
	paramsMust(t, e)
	defer func() { paramsMust(t, m.Close()) }()
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b})
	paramsMust(t, e)
	defer func() { paramsMust(t, out.Close()) }()
	a, e := m.FormatString("aa")
	paramsMust(t, e)
	z, e := m.FormatString("bb")
	paramsMust(t, e)
	line, e := m.FormatLineBreak()
	paramsMust(t, e)
	body, e := m.FormatCompose([]ASTHandle{a, line, z})
	paramsMust(t, e)
	indent, e := m.FormatIndent(3, body)
	paramsMust(t, e)
	group, e := m.FormatGroup(indent)
	paramsMust(t, e)
	paramsMust(t, m.PrintFormat(out, group, c, p, g))
	got, e := out.String()
	paramsMust(t, e)
	if got != "aa bb" {
		t.Fatal("wide choice", got)
	}
	paramsMust(t, out.ResetText())
	paramsMust(t, c.SetUIntName(p, dynAckName("max_width"), 3))
	paramsMust(t, m.PrintFormat(out, group, c, p, g))
	got, e = out.String()
	paramsMust(t, e)
	if got != "aa\n   bb" {
		t.Fatal("actual pp todo and indent", got)
	}
	paramsMust(t, out.ResetText())
	paramsMust(t, c.SetBoolName(p, dynAckName("single_line"), true))
	paramsMust(t, m.PrintFormat(out, group, c, p, g))
	got, e = out.String()
	paramsMust(t, e)
	if got != "aa bb" {
		t.Fatal("single line pushes actual space app", got)
	}
}

func TestASTFormatDefaultPrinterStoppedFrameOwningCleanup(t *testing.T) {
	completed := false
	for ordinal := int64(0); ordinal < 64; ordinal++ {
		m := astTestManager(t)
		word, e := m.FormatString("word")
		paramsMust(t, e)
		line, e := m.FormatLineBreak()
		paramsMust(t, e)
		root, e := m.FormatCompose([]ASTHandle{word, line, word})
		paramsMust(t, e)
		out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
		paramsMust(t, e)
		m.BudgetField.FailAfter(ordinal)
		call := m.PrintFormat(out, root, nil, nil, nil)
		m.BudgetField.FailAfter(-1)
		if call != nil && !errors.Is(call, arena.ErrInjectedFailure) {
			t.Fatal("unexpected default formatter stop", ordinal, call)
		}
		paramsMust(t, out.Close())
		paramsMust(t, m.Close())
		if call == nil {
			completed = true
			break
		}
	}
	if !completed {
		t.Fatal("default formatter never completed")
	}
}

func astFormatPrettyNamedCold(t *testing.T, m *ASTManager, g *uparams.GlobalParamsService, b *arena.Budget, out *utext.NumericTextStream) {
	t.Helper()
	total := m.Stats().AllocatedBytes + g.OwnerBytes + catalogGraph(g.Config.Catalog)
	if g.Current != nil {
		total += gparamsGraph(g.Current)
	}
	for v := g.Superseded; v != nil; v = v.RetainedNext {
		total += gparamsGraph(v)
	}
	for v := g.Failed; v != nil; v = v.RetainedNext {
		total += gparamsGraph(v)
	}
	if out != nil {
		st := out.Stats()
		total += st.OwnerBytes + st.ScratchBytes + st.OutputOwnerBytes + st.OutputOverflowBytes
	}
	if total != b.Used() {
		t.Fatal("actual source owners before fault lift", total, b.Used())
	}
}
func TestASTFormatPrettyEveryGetterStackAndOutputReservation(t *testing.T) {
	first := -1
	for ordinal := int64(0); ordinal < 80; ordinal++ {
		c, g, p, b := dynAckSourceFixture(t)
		m, e := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: b})
		paramsMust(t, e)
		a, e := m.FormatString("word")
		paramsMust(t, e)
		line, e := m.FormatLineBreak()
		paramsMust(t, e)
		root, e := m.FormatCompose([]ASTHandle{a, line, a, line, a})
		paramsMust(t, e)
		out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b})
		paramsMust(t, e)
		b.FailAfter(ordinal)
		e = m.PrintFormat(out, root, c, p, g)
		astFormatPrettyNamedCold(t, m, g, b, out)
		text, read := out.String()
		paramsMust(t, read)
		if !strings.HasPrefix("word\nword\nword", text) {
			t.Fatal("literal source output prefix", ordinal, text, e)
		}
		if e != nil && !errors.Is(e, arena.ErrInjectedFailure) {
			t.Fatal(ordinal, e)
		}
		if e == nil && text != "word\nword\nword" {
			t.Fatal("source complete output", ordinal, text)
		}
		b.FailAfter(-1)
		paramsMust(t, out.Close())
		paramsMust(t, m.Close())
		astParamsNamedColdGraph(t, g, b)
		if e == nil {
			first = int(ordinal)
			break
		}
	}
	if first < 2 {
		t.Fatal("actual pp helper/output horizon", first)
	}
	t.Log("first success", first)
}

func TestASTFormatPrettyNativeWorklistGrowth(t *testing.T) {
	c, g, p, b := dynAckSourceFixture(t)
	m, e := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: b})
	paramsMust(t, e)
	defer func() { paramsMust(t, m.Close()) }()
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b})
	paramsMust(t, e)
	defer func() { paramsMust(t, out.Close()) }()
	empty, e := m.FormatString("")
	paramsMust(t, e)
	parts := make([]ASTHandle, 129)
	for i := range parts {
		parts[i] = empty
	}
	root, e := m.FormatCompose(parts)
	paramsMust(t, e)
	// The frame and parameter helper still reserve owner storage. Expanding
	// its borrowed traversal stack uses a native slice, even when later arena
	// reservations are refused.
	b.FailAfter(2)
	e = m.PrintFormat(out, root, c, p, g)
	b.FailAfter(-1)
	paramsMust(t, e)
	got, e := out.String()
	paramsMust(t, e)
	if got != "" {
		t.Fatal("empty compose output", got)
	}
	astFormatPrettyNamedCold(t, m, g, b, out)
}

// AUDIT.md ast_04-F1: pp() reads pp_params p(_p), which falls back to the
// process global "pp" module even when the caller passes no parameters.
func TestPrintFormatNilOwnersUsesGlobalPP(t *testing.T) {
	c, g, _, b := dynAckSourceFixture(t)
	m, e := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: b})
	paramsMust(t, e)
	defer func() { paramsMust(t, m.Close()) }()
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b})
	paramsMust(t, e)
	defer func() { paramsMust(t, out.Close()) }()
	a, e := m.FormatString("aa")
	paramsMust(t, e)
	z, e := m.FormatString("bb")
	paramsMust(t, e)
	line, e := m.FormatLineBreak()
	paramsMust(t, e)
	body, e := m.FormatCompose([]ASTHandle{a, line, z})
	paramsMust(t, e)
	print := func() string {
		t.Helper()
		paramsMust(t, out.ResetText())
		paramsMust(t, m.PrintFormat(out, body, nil, nil, nil))
		got, e := out.String()
		paramsMust(t, e)
		return got
	}
	if got := print(); got != "aa\nbb" {
		t.Fatal("unbound defaults", got)
	}
	previous, e := uparams.BindProcessGlobalParamsService(g)
	paramsMust(t, e)
	defer func() { _, e := uparams.BindProcessGlobalParamsService(previous); paramsMust(t, e) }()
	if got := print(); got != "aa\nbb" {
		t.Fatal("global defaults", got)
	}
	paramsMust(t, g.SourceSetName("pp.single_line", "true"))
	if got := print(); got != "aa bb" {
		t.Fatal("global pp.single_line ignored", got)
	}
}

func TestPrintFormatUsesPrivateManagerParameters(t *testing.T) {
	c, globals, _, budget := dynAckSourceFixture(t)
	m, err := NewASTManager(ASTManagerConfig{
		Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: budget,
		Params: c, Globals: globals,
	})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.Close()) }()
	paramsMust(t, globals.SourceSetName("pp.single_line", "true"))
	left, err := m.FormatString("left")
	paramsMust(t, err)
	line, err := m.FormatLineBreak()
	paramsMust(t, err)
	right, err := m.FormatString("right")
	paramsMust(t, err)
	body, err := m.FormatCompose([]ASTHandle{left, line, right})
	paramsMust(t, err)
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: budget})
	paramsMust(t, err)
	defer func() { paramsMust(t, out.Close()) }()
	paramsMust(t, m.PrintFormat(out, body, nil, nil, nil))
	got, err := out.String()
	paramsMust(t, err)
	if got != "left right" {
		t.Fatalf("private pp.single_line produced %q", got)
	}
}
