//go:build g3trace

package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSpacerLiteralExpansionIgnoresSourceTraceWriterFailure(t *testing.T) {
	base, _ := newSpacerTestServices(t)
	tags := diagnostic.NewTraceTagsWithWriter(extendedPrettyFailingWriter{})
	tags.Enable("spacer_expand")
	m, err := ast.NewASTManager(ast.ASTManagerConfig{
		Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags,
	})
	depthMust(t, err)
	defer func() { depthMust(t, m.Close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	depthMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	depthMust(t, err)
	equality, err := m.MkEq(zero, one)
	depthMust(t, err)
	pin, err := m.NewRef(equality)
	depthMust(t, err)
	defer func() { depthMust(t, m.DelRef(&pin)) }()
	p := &PredicateTransformer{manager: m}
	before := m.Stats().LiveReferences
	for attempt := 0; attempt < 2; attempt++ {
		expansion, err := p.expandInductiveLiteral(equality)
		depthMust(t, err)
		if len(expansion.literals) != 2 {
			t.Fatal("trace failure changed arithmetic expansion", len(expansion.literals))
		}
		wantLE, err := m.MkArithmeticLE(zero, one)
		depthMust(t, err)
		wantGE, err := m.MkArithmeticGE(zero, one)
		depthMust(t, err)
		if expansion.literals[0] != wantLE || expansion.literals[1] != wantGE {
			t.Fatal("trace failure changed source bound order")
		}
		depthMust(t, expansion.Close())
		if got := m.Stats().LiveReferences; got != before {
			t.Fatal("trace failure leaked expansion references", got, before)
		}
	}
}
