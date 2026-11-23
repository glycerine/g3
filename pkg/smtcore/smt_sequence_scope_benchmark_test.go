package smtcore

import "testing"

func sequenceScopeFixture(t testing.TB) (*SMTInternalizerState, *SMTTheorySeq) {
	t.Helper()
	m, state, _, _ := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	family, err := m.RegisterSequencePlugin()
	astTestMust(t, err)
	theory, err := NewSMTTheorySeq(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	return state, theory
}

func TestSMTSequenceScopesRestoreAxiomHeads(t *testing.T) {
	state, theory := sequenceScopeFixture(t)
	astTestMust(t, theory.PushScope())
	state.scopeLevel++
	theory.axiomsHead = 2
	astTestMust(t, theory.PushScope())
	state.scopeLevel++
	theory.axiomsHead = 3
	astTestMust(t, theory.PopScopes(1))
	state.scopeLevel--
	if theory.axiomsHead != 2 {
		t.Fatalf("inner pop restored head %d, want 2", theory.axiomsHead)
	}
	astTestMust(t, theory.PopScopes(1))
	state.scopeLevel--
	if theory.axiomsHead != 0 {
		t.Fatalf("outer pop restored head %d, want 0", theory.axiomsHead)
	}
}

func BenchmarkSMTSequenceEmptyScope(b *testing.B) {
	state, theory := sequenceScopeFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := theory.PushScope(); err != nil {
			b.Fatal(err)
		}
		state.scopeLevel++
		if err := theory.PopScopes(1); err != nil {
			b.Fatal(err)
		}
		state.scopeLevel--
	}
}
