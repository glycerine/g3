package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// An inequality with a numeral on the right is already in this normal form
// when its left side has no constant coefficient. Include both scalar and
// polynomial operands so the benchmark exercises both monomial representations.
func BenchmarkArithmeticMonomialCancellation(b *testing.B) {
	m, c, g, local := boolRewriterConfigFixture(b)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, r.Close()) })
	for _, domain := range []struct {
		name    string
		integer bool
	}{{"Int", true}, {"Real", false}} {
		bound, err := r.makeNumeralInt(2, domain.integer)
		astTestMust(b, err)
		astTestPin(b, m, bound)
		st, err := m.GetSort(bound)
		astTestMust(b, err)
		x, err := m.MkConst(astTestSymbol(b, m, "x"), st)
		astTestMust(b, err)
		astTestPin(b, m, x)
		y, err := m.MkConst(astTestSymbol(b, m, "y"), st)
		astTestMust(b, err)
		astTestPin(b, m, y)
		sum, err := r.app(arithOpAdd, x, y)
		astTestMust(b, err)
		astTestPin(b, m, sum)
		for _, tc := range []struct {
			name string
			left ast.ASTHandle
		}{{"Scalar", x}, {"Sum", sum}} {
			b.Run(domain.name+"/"+tc.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_, _, changed, err := r.cancelMonomials(tc.left, bound, true)
					if err != nil || changed {
						b.Fatalf("changed=%t error=%v", changed, err)
					}
				}
			})
		}
	}
}

// Time the full comparison reducer for bounds that are common in quantified
// integer assertions. A strict comparison still has to introduce a negation.
func BenchmarkArithmeticVariableBound(b *testing.B) {
	m, c, g, local := boolRewriterConfigFixture(b)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, r.Close()) })
	bound, err := r.makeNumeralInt(0, true)
	astTestMust(b, err)
	astTestPin(b, m, bound)
	st, err := m.GetSort(bound)
	astTestMust(b, err)
	variable, err := m.MkVar(0, st)
	astTestMust(b, err)
	astTestPin(b, m, variable)
	atom, err := m.MkConst(astTestSymbol(b, m, "bound-x"), st)
	astTestMust(b, err)
	astTestPin(b, m, atom)
	out := astTestPin(b, m, ast.ASTHandle{})
	for _, tc := range []struct {
		name   string
		term   ast.ASTHandle
		kind   int32
		status RewriteStatus
	}{
		{"VariableLE", variable, arithOpLE, RewriteFailed},
		{"ConstantGE", atom, arithOpGE, RewriteFailed},
		{"StrictLT", atom, arithOpLT, Rewrite2},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				status, err := r.rewriteComparison(tc.kind, tc.term, bound, out)
				if err != nil || status != tc.status {
					b.Fatalf("status=%v error=%v", status, err)
				}
			}
		})
	}
}
