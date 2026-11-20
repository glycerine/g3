package smtcore

import (
	"fmt"
	"runtime"
	"testing"
	"unsafe"

	"github.com/glycerine/g3/pkg/ast"
)

// Publish reasons in a live table so constructor payloads escape exactly as
// they do in assignment. The same benchmark builds against the unpacked types.
func BenchmarkSMTBooleanRecord(b *testing.B) {
	for _, kind := range []string{"Axiom", "Clause", "BinarySmall", "BinaryDetached", "Theory"} {
		b.Run(kind, func(b *testing.B) {
			records := make([]SMTBoolVarData, 4096)
			clause := new(SMTClause)
			theory := new(SMTSimpleJustification)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				slot := &records[i%len(records)]
				switch kind {
				case "Axiom":
					slot.SetAxiom()
				case "Clause":
					slot.SetJustification(NewSMTBooleanClause(clause))
				case "BinarySmall":
					slot.SetJustification(NewSMTBooleanBinaryClause(SMTLiteral(i & 255)))
				case "BinaryDetached":
					slot.SetJustification(NewSMTBooleanBinaryClause(4096 + SMTLiteral(i&255)))
				case "Theory":
					j, err := NewSMTBooleanJustification(theory)
					if err != nil {
						b.Fatal(err)
					}
					slot.SetJustification(j)
				}
			}
			b.ReportMetric(float64(unsafe.Sizeof(records[0])), "B/record")
			b.ReportMetric(float64(unsafe.Sizeof(SMTBooleanJustification{})), "B/reason")
			runtime.KeepAlive(records)
		})
	}
}

// Measure live solver storage including sparse gate metadata. Root identical
// ASTs before the first heap reading so the solver pays only for bindings.
func BenchmarkSMTBooleanBindingStorage(b *testing.B) {
	const width = 4096
	for _, shape := range []string{"Leaf", "Not", "And"} {
		b.Run(shape, func(b *testing.B) {
			m := astTestManager(b)
			_, _, s := smtInternalizerFixture(b, m, nil)
			exprs := make([]ast.ASTHandle, width)
			roots := make([]ast.ASTRef, width)
			other := smtRelevancyBool(b, m, "gate-peer")
			for i := range exprs {
				e := smtRelevancyBool(b, m, fmt.Sprintf("binding-%d", i))
				var err error
				switch shape {
				case "Not":
					e, err = m.MkNot(e)
				case "And":
					e, err = m.MkAnd(e, other)
				}
				benchmarkMust(b, err)
				roots[i], err = m.NewRef(e)
				benchmarkMust(b, err)
				exprs[i] = e
			}
			b.Cleanup(func() {
				for i := range roots {
					benchmarkMust(b, m.DelRef(&roots[i]))
				}
			})
			publish := func() {
				benchmarkMust(b, s.PushScope())
				for _, e := range exprs {
					_, err := s.MkBoolVar(e)
					benchmarkMust(b, err)
				}
			}
			before := scopeBenchmarkLiveHeap()
			publish()
			live := scopeBenchmarkLiveHeap() - before
			benchmarkMust(b, s.PopScopes(1))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				publish()
				benchmarkMust(b, s.PopScopes(1))
			}
			b.ReportMetric(float64(live)/width, "live-B/binding")
		})
	}
}
