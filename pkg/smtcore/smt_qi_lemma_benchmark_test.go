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

// Distinct simplified disjunctions let the nested call overwrite an outer
// operand if construction accidentally lends out a buffer that is still live.
func smtLemmaOperandFixture(t testing.TB) (*SMTQIInstanceBuilder, *SMTQIPreparedInstance, *SMTQuantifierStat, [2]ast.ASTHandle) {
	f := newQuantifierCandidateFixture(t, 4)
	q := f.manager
	m := q.state.manager
	astTestMust(t, q.state.Internalize(f.quantifier, true))
	stat, err := q.registry.Stat(f.quantifier)
	astTestMust(t, err)
	prepared, err := q.builder.Prepare(SMTQIQueueEntry{Fingerprint: &SMTFingerprint{data: f.quantifier, args: f.bindings[4:]}, Cost: 1})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, prepared.Close()) })
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	var bodies [2]ast.ASTHandle
	for i := range bodies {
		var args [4]ast.ASTHandle
		for j := range args {
			args[j], err = m.MkConst(symbol.Numerical(uint32(6000+4*i+j)), boolean)
			astTestMust(t, err)
		}
		bodies[i], err = m.MkOrArgs(args[:])
		astTestMust(t, err)
		astTestMust(t, m.IncRef(bodies[i]))
		t.Cleanup(func() { astTestMust(t, m.DecRef(bodies[i])) })
	}
	return q.builder, prepared, stat, bodies
}

func TestSMTNestedQuantifierLemmaOperands(t *testing.T) {
	b, prepared, stat, bodies := smtLemmaOperandFixture(t)
	var outer, inner smtQIInstanceLocals
	// Prime the reusable storage before exercising overlapping borrows.
	warm, err := b.finishWithoutProof(prepared, bodies[1], stat, 0, &inner.arguments)
	astTestMust(t, err)
	astTestMust(t, warm.Close())
	astTestMust(t, inner.release(b))
	x, err := b.finishWithoutProof(prepared, bodies[0], stat, 0, &outer.arguments)
	astTestMust(t, err)
	y, err := b.finishWithoutProof(prepared, bodies[1], stat, 0, &inner.arguments)
	astTestMust(t, err)
	windows := [2]*smtQIInstanceLocals{&outer, &inner}
	for i, body := range bodies {
		window := windows[i]
		for j := 0; j < 4; j++ {
			want, err := b.manager.GetArg(body, uint32(j))
			astTestMust(t, err)
			if window.arguments[j+1] != want {
				t.Fatalf("operand %d of lemma %d changed during construction", j, i)
			}
		}
	}
	astTestMust(t, y.Close())
	astTestMust(t, inner.release(b))
	astTestMust(t, x.Close())
	astTestMust(t, outer.release(b))
}

func BenchmarkSMTQuantifierLemmaOperands(b *testing.B) {
	builder, prepared, stat, bodies := smtLemmaOperandFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var locals smtQIInstanceLocals
		x, err := builder.finishWithoutProof(prepared, bodies[i%2], stat, 0, &locals.arguments)
		if err != nil {
			b.Fatal(err)
		}
		if err = x.Close(); err != nil {
			b.Fatal(err)
		}
		if err = locals.release(builder); err != nil {
			b.Fatal(err)
		}
	}
}
