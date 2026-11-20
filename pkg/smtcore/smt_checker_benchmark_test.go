package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// The quantifier queue asks the checker about each candidate in a fresh query
// cache. Compare a known literal with a formula that needs recursive AST reads.
func BenchmarkSMTCheckerCandidate(b *testing.B) {
	m := astTestManager(b)
	p := smtRelevancyBool(b, m, "candidate-p")
	q := smtRelevancyBool(b, m, "candidate-q")
	ctx, err := NewSMTCheckerContext(m)
	benchmarkMust(b, err)
	benchmarkMust(b, ctx.SetAssignment(p, lbool.True))
	benchmarkMust(b, ctx.SetAssignment(q, lbool.False))
	benchmarkMust(b, ctx.SetRelevant(p, true))
	benchmarkMust(b, ctx.SetRelevant(q, true))
	checker, err := NewSMTChecker(ctx)
	benchmarkMust(b, err)
	choice, err := m.MkIte(p, p, q)
	benchmarkMust(b, err)
	equal, err := m.MkIff(choice, p)
	benchmarkMust(b, err)
	either, err := m.MkOr(q, choice)
	benchmarkMust(b, err)
	body, err := m.MkAnd(equal, either)
	benchmarkMust(b, err)
	for _, c := range []struct {
		name string
		expr ast.ASTHandle
	}{{"Assigned", p}, {"Formula", body}} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if yes, err := checker.IsSatisfiable(c.expr, nil); err != nil || !yes {
					b.Fatalf("candidate check returned %t, %v", yes, err)
				}
			}
		})
	}
}
