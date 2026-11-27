package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Compare a scope that owns every variable with one where half must survive.
func BenchmarkSMTBooleanEnodeFlagTrail(b *testing.B) {
	for _, survive := range []int{0, 256} {
		b.Run(strconv.Itoa(survive)+"Surviving", func(b *testing.B) {
			m := astTestManager(b)
			_, _, s := smtInternalizerFixture(b, m, nil)
			var terms [512]ast.ASTHandle
			for i := range terms {
				terms[i] = smtRelevancyBool(b, m, "flag-"+strconv.Itoa(i))
				astTestPin(b, m, terms[i])
				if i < survive {
					astTestMust(b, s.Internalize(terms[i], true))
				}
			}
			var entries int
			b.ReportAllocs()
			for b.Loop() {
				if err := s.PushScope(); err != nil {
					b.Fatal(err)
				}
				for _, h := range terms {
					if err := s.Internalize(h, false); err != nil {
						b.Fatal(err)
					}
				}
				entries = len(s.enodeFlagTrail)
				if err := s.PopScopes(1); err != nil {
					b.Fatal(err)
				}
			}
			for i, h := range terms {
				if _, ok := s.BoolVar(h); ok != (i < survive) || s.context.enodes.get(h) != nil {
					b.Fatalf("incorrect scope restoration for term %d", i)
				}
			}
			b.ReportMetric(float64(entries), "flag-entries/op")
			b.ReportMetric(float64(4*cap(s.enodeFlagTrail)), "flag-capacity-B")
		})
	}
}
