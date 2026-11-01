package spacer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestExpandInductiveLiteralSourceTrace(t *testing.T) {
	base, _ := newSpacerTestServices(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("spacer_expand")
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		t.Fatal(err)
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	if err != nil {
		t.Fatal(err)
	}
	equality, err := m.MkEq(zero, one)
	if err != nil {
		t.Fatal(err)
	}
	p := &PredicateTransformer{manager: m}
	resultOwner, err := p.expandInductiveLiteral(equality)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resultOwner.Close(); err != nil {
			t.Error(err)
		}
	}()
	result := resultOwner.literals
	if len(result) != 2 {
		t.Fatal("expanded bound count", len(result))
	}
	if tags.ShouldTrace("spacer_expand") {
		text := output.String()
		if !strings.Contains(text, "begin expand\n(= 0 1)\n") || !strings.Contains(text, "end expand\n(<= 0 1)\n(>= 0 1)\n") {
			t.Fatal("source expansion trace", text)
		}
	} else if output.Len() != 0 {
		t.Fatal("disabled trace emitted output", output.String())
	}
}
