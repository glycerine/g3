package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPICoreRegisteredTacticFactories(t *testing.T) {
	for _, tc := range []struct{ name, description, script string }{
		{"blast-term-ite", "blast term if-then-else by hoisting them.", `(declare-const p Bool) (declare-fun f (Int) Int) (assert (= (f (ite p 1 2)) 3))`},
		{"cofactor-term-ite", "eliminate term if-the-else using cofactors.", `(declare-const p Bool) (assert (= (+ (ite p 1 2) 3) 4))`},
		{"pb-preprocess", "pre-process pseudo-Boolean constraints a la Davis Putnam.", `(declare-const p Bool) (declare-const q Bool) (assert ((_ at-least 1) p q))`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newSMTAPIFixture(t, "ALL", nil)
			desc, err := f.c.TacticDescription(tc.name)
			astTestMust(t, err)
			if desc != tc.description {
				t.Fatal("description", desc)
			}
			first, err := f.c.Tactic(tc.name)
			astTestMust(t, err)
			smt, err := f.c.Tactic("smt")
			astTestMust(t, err)
			combined, err := first.AndThen(smt)
			astTestMust(t, err)
			s, err := f.c.NewSolverFromTactic(combined)
			astTestMust(t, err)
			for _, v := range []*APITactic{combined, smt, first} {
				astTestMust(t, v.Close())
			}
			defer func() { astTestMust(t, s.Close()) }()
			astTestMust(t, s.LoadSMT2String(tc.script))
			status, err := s.Check()
			astTestMust(t, err)
			if status != lbool.True {
				reason, _ := s.ReasonUnknown()
				t.Fatal("status", status, reason)
			}
			refs, err := s.Assertions()
			astTestMust(t, err)
			if len(refs) != 1 {
				t.Fatal("assertions", len(refs))
			}
			defer releaseAPIRefs(t, f.m, refs)
			assertion, err := f.m.RefGet(refs[0])
			astTestMust(t, err)
			md, err := s.Model()
			astTestMust(t, err)
			defer func() { astTestMust(t, md.Close()) }()
			value, err := md.Evaluate(assertion, true)
			astTestMust(t, err)
			defer func() { astTestMust(t, f.m.DelRef(&value)) }()
			h, err := f.m.RefGet(value)
			astTestMust(t, err)
			if !f.m.IsTrue(h) {
				t.Fatal("model does not satisfy source assertion", h)
			}
		})
	}
}

func TestAPILocalSearchTacticRegistry(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_BV", nil)
	found, err := f.c.HasTactic("qfbv-sls")
	astTestMust(t, err)
	if !found {
		t.Fatal("qfbv-sls missing from tactic catalog")
	}
	desc, err := f.c.TacticDescription("qfbv-sls")
	astTestMust(t, err)
	if desc != "(try to) solve using stochastic local search for QF_BV." {
		t.Fatal("description", desc)
	}
	tac, err := f.c.Tactic("qfbv-sls")
	astTestMust(t, err)
	s, err := f.c.NewSolverFromTactic(tac)
	astTestMust(t, err)
	astTestMust(t, tac.Close())
	defer func() { astTestMust(t, s.Close()) }()
	astTestMust(t, s.LoadSMT2String(`(declare-const x (_ BitVec 4)) (assert (= x #x3))`))
	status, err := s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		reason, _ := s.ReasonUnknown()
		t.Fatal("status", status, reason)
	}
}
