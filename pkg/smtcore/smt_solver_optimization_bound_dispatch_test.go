package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestOptimizationObjectiveGEDifferenceEpsilonFallback(t *testing.T) {
	for _, name := range []string{"real-difference", "mixed-difference", "dense"} {
		t.Run(name, func(t *testing.T) {
			s, family := optimizerWithoutArithmeticFixture(t)
			var theory SMTTheoryInternalizer
			var err error
			switch name {
			case "real-difference":
				theory, err = NewSMTRealDiffTheory(family)
			case "mixed-difference":
				theory, err = NewSMTMixedDiffTheory(family)
			case "dense":
				theory, err = NewSMTDenseDiffTheory(family)
			}
			astTestMust(t, err)
			astTestMust(t, s.kernel.RegisterTheory(theory))
			sort, err := s.manager.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			x, err := s.manager.MkConst(astTestSymbol(t, s.manager, "dispatch-bound-x"), sort)
			astTestMust(t, err)
			index, err := s.AddObjective(x)
			astTestMust(t, err)
			stream := optimizationDumpVerboseStream(t, s)
			astTestMust(t, diagnostic.SetProcessVerbosityLevel(0))
			for _, epsilon := range []int64{0, 1, -1} {
				value := SMTArithmeticObjectiveValue{}
				value.Finite.Main.SetInt64(3)
				value.Finite.Epsilon.SetInt64(epsilon)
				before := len(s.kernel.internalizer.boolExpr)
				bound, err := s.optimizationObjectiveGE(index, nil, value)
				astTestMust(t, err)
				root, err := s.manager.NewRef(bound)
				astTestMust(t, err)
				if epsilon == 0 {
					kind, err := s.manager.GetDeclKind(bound)
					astTestMust(t, err)
					if kind != smtArithOpGE {
						t.Fatal("rational bound did not dispatch", kind)
					}
				} else if !s.manager.IsTrue(bound) || len(s.kernel.internalizer.boolExpr) != before {
					t.Fatal("unsupported epsilon bound constrained solver", epsilon, bound)
				}
				astTestMust(t, s.manager.DelRef(&root))
			}
			text, err := stream.String()
			astTestMust(t, err)
			if strings.Count(text, "WARNING: unhandled theory ") != 2 || !strings.Contains(text, "smtcore.SMT") {
				t.Fatal("source level-zero fallback diagnostic", text)
			}
			stream.FileBad = true
			value := SMTArithmeticObjectiveValue{}
			value.Finite.Epsilon.SetInt64(1)
			bound, err := s.optimizationObjectiveGE(index, nil, value)
			astTestMust(t, err)
			if !s.manager.IsTrue(bound) {
				t.Fatal("bad diagnostic stream changed fallback")
			}
		})
	}
}

func TestOptimizationObjectiveGEInfinityErasesObjectiveLookup(t *testing.T) {
	m, s := optimizationSMTFixture(t, true)
	for _, truth := range []bool{false, true} {
		if truth {
			_, err := m.MkTrue()
			astTestMust(t, err)
		} else {
			_, err := m.MkFalse()
			astTestMust(t, err)
		}
	}
	for _, sign := range []int64{-2, 2, 0} {
		value := SMTArithmeticObjectiveValue{Unbounded: sign == 0}
		value.Finite.Infinity.SetInt64(sign)
		m.Budget().FailAfter(0)
		bound, err := s.optimizationObjectiveGE(^uint32(0), nil, value)
		m.Budget().FailAfter(-1)
		astTestMust(t, err)
		if m.IsTrue(bound) != (sign < 0) || m.IsFalse(bound) != (sign >= 0) {
			t.Fatal("infinite bound polarity", sign, bound)
		}
	}
}

func TestOptimizationObjectiveGELinearEpsilonStillBuildsTheoryBound(t *testing.T) {
	s, f, filter := optimizationRepairFixture(t, false)
	x := f.variable("dispatch-linear-x")
	index, err := s.AddObjective(x)
	astTestMust(t, err)
	value := SMTArithmeticObjectiveValue{}
	value.Finite.Main.SetInt64(3)
	value.Finite.Epsilon.SetInt64(-1)
	bound, err := s.optimizationObjectiveGE(index, filter, value)
	astTestMust(t, err)
	if s.manager.IsTrue(bound) || s.manager.IsFalse(bound) {
		t.Fatal("linear epsilon took unsupported fallback")
	}
	if _, found := s.kernel.internalizer.BoolVar(bound); !found {
		t.Fatal("linear threshold was not internalized")
	}
}
