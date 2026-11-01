package spacer

// Source: src/muz/spacer/spacer_util.cpp expand_literals bit-vector branch.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExpandInductiveBitVectorEqualityFiniteOracle(t *testing.T) {
	m, _ := newSpacerTestServices(t)
	p := &PredicateTransformer{manager: m}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for width := uint32(1); width <= 6; width++ {
		zero, err := m.BitVectorNumeralUnsigned(0, width)
		must(err)
		sort, err := m.GetSort(zero)
		must(err)
		name, err := m.Symbols.Intern("expansion-oracle")
		must(err)
		x, err := m.MkConst(name, sort)
		must(err)
		for value := uint64(0); value < uint64(1)<<width; value++ {
			numeral, err := m.BitVectorNumeralUnsigned(uint32(value), width)
			must(err)
			for _, reverse := range []bool{false, true} {
				left, right := x, numeral
				if reverse {
					left, right = right, left
				}
				equality, err := m.MkEq(left, right)
				must(err)
				literals, err := p.expandInductiveLiteral(equality)
				must(err)
				if len(literals) != int(width) {
					t.Fatalf("width %d value %d: %d literals", width, value, len(literals))
				}
				for bit, literal := range literals {
					negated, atom, err := m.IsNot(literal)
					must(err)
					if negated != (value&(uint64(1)<<uint(bit)) == 0) {
						t.Fatalf("width %d value %d bit %d: polarity", width, value, bit)
					}
					if !negated {
						atom = literal
					}
					kind, err := m.GetDeclKind(atom)
					must(err)
					if kind != ast.ASTOpEq {
						t.Fatal("expanded bit is not equality")
					}
					one, err := m.GetArg(atom, 0)
					must(err)
					text, matched, err := m.BitVectorNumeralText(one)
					must(err)
					if !matched || text != "1" {
						t.Fatal("expanded bit does not compare to one", text)
					}
					extract, err := m.GetArg(atom, 1)
					must(err)
					var effective uint32
					for extract != x {
						hi, err := m.BitVectorExtractHigh(extract)
						must(err)
						lo, err := m.BitVectorExtractLow(extract)
						must(err)
						if hi != lo {
							t.Fatal("expanded bit selects more than one bit", hi, lo)
						}
						effective += lo
						extract, err = m.GetArg(extract, 0)
						must(err)
					}
					if effective != uint32(bit) {
						t.Fatalf("width %d value %d bit %d: effective extraction %d", width, value, bit, effective)
					}
				}
			}
		}
	}
}
