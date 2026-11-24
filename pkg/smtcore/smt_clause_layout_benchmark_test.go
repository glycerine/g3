package smtcore

import (
	"fmt"
	"runtime"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type smtLayoutClauseFixture struct {
	m       *ast.ASTManager
	lits    []SMTLiteral
	atoms   []ast.ASTHandle
	pool    smtClauseAtomPool
	proof   smtClauseTestJustification
	handler smtClauseTestDeleteHandler
}

func newSMTLayoutClauseFixture(b *testing.B, width int) *smtLayoutClauseFixture {
	f := &smtLayoutClauseFixture{m: astTestManager(b), atoms: make([]ast.ASTHandle, width+1), lits: make([]SMTLiteral, width)}
	for i := range f.lits {
		h := smtRelevancyBool(b, f.m, fmt.Sprintf("clause-layout-%d", i))
		astTestPin(b, f.m, h)
		f.atoms[i+1] = h
		f.lits[i] = NewSMTLiteral(int32(i+1), i%2 != 0)
	}
	return f
}

func (f *smtLayoutClauseFixture) create(mode string) (*SMTClause, error) {
	k := SMTClauseAux
	var proof SMTJustification
	var handler SMTClauseDeleteHandler
	switch mode {
	case "Proof":
		proof = &f.proof
	case "Handler":
		handler = &f.handler
	case "Learned", "Saved":
		k = SMTClauseLearned
	}
	return newSMTClause(f.m, f.lits, k, proof, handler, mode == "Saved", f.atoms, &f.pool)
}

func (f *smtLayoutClauseFixture) check(b *testing.B, c *SMTClause, mode string) {
	b.Helper()
	if len(c.Literals()) != len(f.lits) || c.IsLearned() != (mode == "Learned" || mode == "Saved") || c.ReinternalizeAtoms() != (mode == "Saved") {
		b.Fatal("incorrect clause metadata")
	}
	for i, lit := range c.Literals() {
		if lit != f.lits[i] {
			b.Fatal("wrong literal", i)
		}
	}
	if mode == "Proof" {
		if c.Justification() != &f.proof {
			b.Fatal("lost proof")
		}
	} else if c.Justification() != nil {
		b.Fatal("unexpected proof")
	}
	if c.IsLearned() {
		a, e := c.Activity()
		if e != nil || a != 1 {
			b.Fatal("wrong activity", a, e)
		}
	}
	if mode == "Saved" {
		for i := range f.lits {
			h, sign, e := c.Atom(i)
			if e != nil || h != f.atoms[i+1] || sign != f.lits[i].Sign() {
				b.Fatal("wrong saved atom", i, e)
			}
		}
	}
}

func BenchmarkSMTClauseCreateClose(b *testing.B) {
	for _, width := range []int{3, 8, 65} {
		for _, mode := range []string{"Bare", "Proof", "Handler", "Learned", "Saved"} {
			b.Run(strconv.Itoa(width)+"/"+mode, func(b *testing.B) {
				f := newSMTLayoutClauseFixture(b, width)
				trial, e := f.create(mode)
				benchmarkMust(b, e)
				f.check(b, trial, mode)
				benchmarkMust(b, trial.Close())
				before := f.handler.calls
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					c, e := f.create(mode)
					if e != nil {
						b.Fatal(e)
					}
					if e = c.Close(); e != nil {
						b.Fatal(e)
					}
				}
				b.StopTimer()
				if mode == "Handler" && f.handler.calls-before != b.N {
					b.Fatal("missed deletion callback")
				}
			})
		}
	}
}

// Measure whole active clauses, including inline literals, optional attached
// data and the memory for retained atoms. The input expressions stay live.
func BenchmarkSMTLiveClauseMemory(b *testing.B) {
	const workers, perWorker = 8, 512
	for _, width := range []int{3, 8, 65} {
		for _, mode := range []string{"Bare", "Proof", "Handler", "Learned", "Saved", "Mixed"} {
			b.Run(strconv.Itoa(width)+"/"+mode, func(b *testing.B) {
				type worker struct {
					*smtLayoutClauseFixture
					live [perWorker]*SMTClause
				}
				ws := make([]worker, workers)
				for i := range ws {
					ws[i].smtLayoutClauseFixture = newSMTLayoutClauseFixture(b, width)
				}
				shape := func(i int) string {
					if mode != "Mixed" {
						return mode
					}
					switch {
					case i%100 < 70:
						return "Bare"
					case i%100 < 98:
						return "Proof"
					case i%100 == 98:
						return "Learned"
					default:
						return "Saved"
					}
				}
				b.ReportAllocs()
				b.ResetTimer()
				b.StopTimer()
				var peak, idle int64
				for rep := 0; rep < b.N; rep++ {
					base := scopeBenchmarkLiveHeap()
					b.StartTimer()
					for wi := range ws {
						w := &ws[wi]
						for i := range w.live {
							c, err := w.create(shape(i))
							benchmarkMust(b, err)
							w.live[i] = c
						}
					}
					b.StopTimer()
					peak += scopeBenchmarkLiveHeap() - base
					for wi := range ws {
						w := &ws[wi]
						for i, c := range w.live {
							w.check(b, c, shape(i))
						}
					}
					b.StartTimer()
					for wi := range ws {
						w := &ws[wi]
						for i, c := range w.live {
							benchmarkMust(b, c.Close())
							w.live[i] = nil
						}
					}
					b.StopTimer()
					idle += scopeBenchmarkLiveHeap() - base
					if mode == "Handler" {
						for wi := range ws {
							if ws[wi].handler.calls != (rep+1)*perWorker {
								b.Fatal("missed deletion callback")
							}
						}
					}
				}
				b.ReportMetric(float64(peak)/float64(b.N*workers*perWorker), "live-B/clause")
				b.ReportMetric(float64(idle)/float64(b.N*workers), "idle-B/solver")
				runtime.KeepAlive(ws)
			})
		}
	}
}
