package smtcore

// Source: src/opt/opt_solver.cpp:109-157 get_optimizer.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func optimizerWithoutArithmeticFixture(t *testing.T) (*SMTSolver, ast.FamilyID) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	p, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
	s, err := NewSMTSolver(m, &p, "QF_UF", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	if s.kernel.internalizer.theories[family] != nil {
		t.Fatal("fixture unexpectedly installed arithmetic")
	}
	return s, family
}

func TestSMTSolverOptimizerInstallsMissingMixedArithmetic(t *testing.T) {
	s, family := optimizerWithoutArithmeticFixture(t)
	// optsmt::setup performs a base-level push/pop before registration.
	astTestMust(t, s.Push())
	astTestMust(t, s.Pop(1))
	before := len(s.kernel.internalizer.theoryOrder)
	optimizer, err := s.arithmeticObjectiveTheory()
	astTestMust(t, err)
	mixed, ok := optimizer.(*SMTTheoryArith)
	if !ok || !(!mixed.integerExt) || mixed.integerExt || mixed.infExt {
		t.Fatal("source fallback specialization", optimizer)
	}
	if s.kernel.internalizer.theories[family] != mixed || len(s.kernel.internalizer.theoryOrder) != before+1 {
		t.Fatal("fallback publication")
	}
	again, err := s.arithmeticObjectiveTheory()
	astTestMust(t, err)
	if again != optimizer || len(s.kernel.internalizer.theoryOrder) != before+1 {
		t.Fatal("lookup reinstalled arithmetic")
	}
	// Exercise the public consumer, not just discovery: objective registration
	// must use the newly installed concrete mixed arithmetic theory.
	sort, err := s.manager.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	name, err := s.manager.Symbols.Intern("fallback-objective")
	astTestMust(t, err)
	x, err := s.manager.MkConst(name, sort)
	astTestMust(t, err)
	ref, err := s.manager.NewRef(x)
	astTestMust(t, err)
	defer func() { astTestMust(t, s.manager.DelRef(&ref)) }()
	index, err := s.AddObjective(x)
	astTestMust(t, err)
	if index != 0 || s.objectives[0].theory != mixed {
		t.Fatal("objective bypassed fallback", index)
	}
	astTestMust(t, s.ResetObjectives())
}

func TestSMTSolverOptimizerPreservesExistingSpecialization(t *testing.T) {
	for _, integer := range []bool{false, true} {
		s, f := smtModelUpdateFixture(t, integer)
		startSMTModelUpdate(t, s, f)
		family, err := s.manager.ArithmeticFamilyID()
		astTestMust(t, err)
		previous := s.kernel.internalizer.theories[family]
		before := len(s.kernel.internalizer.theoryOrder)
		optimizer, err := s.arithmeticObjectiveTheory()
		astTestMust(t, err)
		if optimizer != previous.(smtArithmeticObjectiveTheory) || len(s.kernel.internalizer.theoryOrder) != before {
			t.Fatal("existing arithmetic replaced")
		}
	}
}

func TestSMTSolverOptimizerRejectsIncompatibleTheoryWithoutReplacement(t *testing.T) {
	s, family := optimizerWithoutArithmeticFixture(t)
	dummy, err := NewSMTDummyTheory(family, "disabled arithmetic")
	astTestMust(t, err)
	astTestMust(t, s.kernel.RegisterTheory(dummy))
	before := len(s.kernel.internalizer.theoryOrder)
	if _, err = s.arithmeticObjectiveTheory(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("incompatible theory accepted", err)
	}
	if s.kernel.internalizer.theories[family] != dummy || len(s.kernel.internalizer.theoryOrder) != before {
		t.Fatal("incompatible plugin replaced")
	}
}

func TestSMTSolverOptimizerLateInstallMatchesContextScopes(t *testing.T) {
	for _, stage := range []string{"cold", "prepared", "checked", "nested"} {
		t.Run(stage, func(t *testing.T) {
			s, family := optimizerWithoutArithmeticFixture(t)
			if stage == "prepared" {
				astTestMust(t, s.Push())
				astTestMust(t, s.Pop(1))
			}
			if stage == "checked" || stage == "nested" {
				status, err := s.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal(status)
				}
			}
			if stage == "nested" {
				astTestMust(t, s.Push())
				astTestMust(t, s.Push())
			}
			s.kernel.arithState.PropagationThreshold = 17
			before := len(s.kernel.internalizer.scopes)
			optimizer, err := s.arithmeticObjectiveTheory()
			astTestMust(t, err)
			mixed := optimizer.(*SMTTheoryArith)
			if mixed.params.PropagationThreshold != 17 || len(mixed.scopes) != before || len(mixed.arithEqAdapter.limits) != before || len(mixed.SMTTheoryBase.limits) != before {
				t.Fatal("late plugin scope/configuration", stage, before, len(mixed.scopes), mixed.params.PropagationThreshold)
			}
			if s.kernel.internalizer.theories[family] != mixed {
				t.Fatal("late plugin not published")
			}
			if stage == "nested" {
				astTestMust(t, s.Pop(2))
				if len(mixed.scopes) != len(s.kernel.internalizer.scopes) {
					t.Fatal("late plugin did not pop with context")
				}
			}
		})
	}
}

func TestSMTSolverOptimizerDispatchesAllConcreteFamilies(t *testing.T) {
	constructors := []struct {
		name string
		make func(ast.FamilyID) (SMTTheoryInternalizer, error)
	}{
		{"mixed", func(f ast.FamilyID) (SMTTheoryInternalizer, error) { return NewSMTTheoryArith(f, false) }},
		{"integer", func(f ast.FamilyID) (SMTTheoryInternalizer, error) { return NewSMTTheoryArith(f, true) }},
		{"real", func(f ast.FamilyID) (SMTTheoryInternalizer, error) { return NewSMTTheoryArith(f, false) }},
		{"infinite", func(f ast.FamilyID) (SMTTheoryInternalizer, error) { return NewSMTTheoryInfArith(f) }},
		{"lra", func(f ast.FamilyID) (SMTTheoryInternalizer, error) { return NewSMTLRATheory(f) }},
		{"rdl", func(f ast.FamilyID) (SMTTheoryInternalizer, error) { return NewSMTRealDiffTheory(f) }},
		{"mixed-difference", func(f ast.FamilyID) (SMTTheoryInternalizer, error) { return NewSMTMixedDiffTheory(f) }},
		{"dense", func(f ast.FamilyID) (SMTTheoryInternalizer, error) { return NewSMTDenseDiffTheory(f) }},
	}
	for _, test := range constructors {
		t.Run(test.name, func(t *testing.T) {
			s, family := optimizerWithoutArithmeticFixture(t)
			theory, err := test.make(family)
			astTestMust(t, err)
			astTestMust(t, s.kernel.RegisterTheory(theory))
			before := len(s.kernel.internalizer.theoryOrder)
			optimizer, err := s.arithmeticObjectiveTheory()
			astTestMust(t, err)
			if optimizer != theory.(smtArithmeticObjectiveTheory) || len(s.kernel.internalizer.theoryOrder) != before {
				t.Fatal("concrete dispatch replaced theory")
			}
		})
	}
}
