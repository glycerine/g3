package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Compare the cost of supplying a 64-root candidate when the check finishes
// without needing a context term. ARM in the verifier panel exercises lookup.
func BenchmarkModelCheckerCandidateTerms(b *testing.B) {
	m, c, model, _, value := smtAuxiliaryQueryFixture(b, false)
	g := &SMTModelGenerator{model: model, rootValues: make(map[*SMTEnode]ast.ASTHandle), rootOrder: ast.NewObjMap(m)}
	sort, err := m.GetSort(value)
	astTestMust(b, err)
	for i := uint32(0); i < 64; i++ {
		term, err := m.MkVar(i, sort)
		astTestMust(b, err)
		astTestPin(b, m, term)
		node := &SMTEnode{owner: term}
		g.rootValues[node] = value
		astTestMust(b, g.rootOrder.InsertNum(term, uint64(i)))
		g.rootOrderNodes = append(g.rootOrderNodes, node)
	}
	for _, mode := range []string{"Eager", "OnDemand"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var valid bool
				var err error
				if mode == "Eager" {
					if err := c.SetCandidateTerms(g.rootValues, g.rootValuesInTableOrder()); err != nil {
						b.Fatal(err)
					}
					valid, err = c.Check(model, nil)
				} else {
					valid, err = c.checkCandidate(g, nil)
				}
				if !valid || err != nil {
					b.Fatalf("candidate check returned %v, %v", valid, err)
				}
			}
		})
	}
}
