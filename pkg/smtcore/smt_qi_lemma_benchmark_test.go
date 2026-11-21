package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Exercise the expression, disjunction and constant cases of qi_queue::instantiate.
func BenchmarkSMTQuantifierLemma(b *testing.B) {
	f := newQuantifierCandidateFixture(b, 4)
	qm := f.manager
	m := qm.state.manager
	astTestMust(b, qm.state.Internalize(f.quantifier, true))
	qstat, err := qm.registry.Stat(f.quantifier)
	astTestMust(b, err)
	prep, err := qm.builder.Prepare(SMTQIQueueEntry{Fingerprint: &SMTFingerprint{data: f.quantifier, args: f.bindings[4:]}, Cost: 1})
	astTestMust(b, err)
	if prep.AlreadySatisfied() {
		b.Fatal("unexpected satisfied instance")
	}
	b.Cleanup(func() { astTestMust(b, prep.Close()) })
	boolean, err := m.MkBoolSort()
	astTestMust(b, err)
	for _, width := range []int{0, 1, 3, 16} {
		b.Run(fmt.Sprintf("Width%d", width), func(b *testing.B) {
			term, err := m.MkFalse()
			astTestMust(b, err)
			args := make([]ast.ASTHandle, width)
			for i := range args {
				args[i], err = m.MkConst(symbol.Numerical(uint32(i+5000)), boolean)
				astTestMust(b, err)
			}
			if width == 1 {
				term = args[0]
			} else if width > 1 {
				term, err = m.MkOrArgs(args)
				astTestMust(b, err)
			}
			astTestMust(b, m.IncRef(term))
			b.Cleanup(func() { astTestMust(b, m.DecRef(term)) })
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := qm.builder.Finish(prep, term, qstat, 0)
				if err != nil || result == nil || result.ReducedTrue() {
					b.Fatal(result, err)
				}
				if err = result.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
