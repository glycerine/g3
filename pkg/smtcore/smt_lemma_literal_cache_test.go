package smtcore

import "testing"

func TestSMTLemmaLiteralCacheChecksBooleanCreation(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "lemma-cache-base")
	av, err := s.MkBoolVar(a)
	astTestMust(t, err)
	astTestMust(t, s.PushScope())
	b := smtRelevancyBool(t, m, "lemma-cache-scoped")
	bv, err := s.MkBoolVar(b)
	astTestMust(t, err)
	atoms, err := s.captureLemmaAtoms([]SMTLiteral{NewSMTLiteral(av, true), NewSMTLiteral(bv, false)})
	astTestMust(t, err)
	defer func() { astTestMust(t, s.releaseLemmaAtoms(atoms)) }()
	astTestMust(t, s.PopScopes(1))
	c := smtRelevancyBool(t, m, "lemma-cache-reused")
	cv, err := s.MkBoolVar(c)
	astTestMust(t, err)
	if cv != bv {
		t.Fatal("fixture did not reuse the scoped Boolean slot")
	}
	got, err := s.reinternalizeLemmaAtoms(atoms)
	astTestMust(t, err)
	if got[0] != NewSMTLiteral(av, true) {
		t.Fatal("surviving signed literal changed", got)
	}
	if got[1].Var() == cv || s.boolExpr[got[1].Var()] != b {
		t.Fatal("stale cached variable aliased another expression", got)
	}
}

func TestSMTLemmaLiteralCachePreservesNongateNotVariable(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "lemma-cache-not-child")
	_, err := s.MkBoolVar(a)
	astTestMust(t, err)
	neg, err := m.MkNot(a)
	astTestMust(t, err)
	nv, err := s.MkBoolVar(neg)
	astTestMust(t, err)
	want := NewSMTLiteral(nv, true)
	atoms, err := s.captureLemmaAtoms([]SMTLiteral{want})
	astTestMust(t, err)
	defer func() { astTestMust(t, s.releaseLemmaAtoms(atoms)) }()
	astTestMust(t, s.PushScope())
	astTestMust(t, s.PopScopes(1))
	got, err := s.reinternalizeLemmaAtoms(atoms)
	astTestMust(t, err)
	if len(got) != 1 || got[0] != want {
		t.Fatal("surviving nongate NOT was normalized to a different variable", got, want)
	}
}
