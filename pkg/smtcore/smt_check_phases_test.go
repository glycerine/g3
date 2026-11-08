package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtCheckPhaseTheory struct {
	*smtInternalizerTestTheory
	initCalls     int
	initError     error
	sawAssumption bool
	cancelFinal   bool
	cancelPop     bool
}

func (t *smtCheckPhaseTheory) InitializeSearch() error {
	t.initCalls++
	for _, data := range t.state.booleanData {
		t.sawAssumption = t.sawAssumption || data.Assumption
	}
	return t.initError
}

func (t *smtCheckPhaseTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t.cancelFinal {
		t.state.manager.Limit.Cancel()
	}
	return SMTFinalCheckDone, nil
}

func (t *smtCheckPhaseTheory) PopScopes(count uint32) error {
	if err := t.SMTTheoryBase.PopScopes(count); err != nil {
		return err
	}
	if t.cancelPop {
		t.state.manager.Limit.Cancel()
	}
	return nil
}

func smtCheckPhaseFixture(t *testing.T, proofs bool) (*ast.ASTManager, *SMTKernel, *smtCheckPhaseTheory) {
	t.Helper()
	m := astTestManager(t)
	if proofs {
		astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	}
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	k := smtTestKernel(t, m)
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	theory := &smtCheckPhaseTheory{smtInternalizerTestTheory: &smtInternalizerTestTheory{SMTTheoryBase: base}}
	astTestMust(t, k.RegisterTheory(theory))
	return m, k, theory
}

func TestSMTCheckKnownConflictPreservesProofAndSkipsSearch(t *testing.T) {
	m, k, theory := smtCheckPhaseFixture(t, true)
	theory.initError = errors.New("search initialization must not run")
	a := smtRelevancyBool(t, m, "known-conflict-assumption-a")
	b := smtRelevancyBool(t, m, "known-conflict-assumption-b")
	nonLiteral, err := m.MkOr(a, b)
	astTestMust(t, err)
	smtHoldSubstitutionInput(t, m, nonLiteral)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, k.Assert(falsity, ast.ASTHandle{}))
	status, err := k.Check()
	astTestMust(t, err)
	if status != lbool.False || theory.initCalls != 0 {
		t.Fatal("preprocessing conflict entered search", status, theory.initCalls)
	}
	proof, err := k.Proof()
	astTestMust(t, err)
	if proof.IsNull() {
		t.Fatal("early conflict lost its proof")
	}
	m.Limit.Cancel()
	before := m.Limit.Count()
	m.Budget().FailAfter(0)
	status, err = k.CheckAssumptions([]ast.ASTHandle{nonLiteral})
	m.Budget().FailAfter(-1)
	m.Limit.ResetCancel()
	astTestMust(t, err)
	if status != lbool.False || m.Limit.Count() != before || theory.initCalls != 0 {
		t.Fatal("known base conflict did not precede validation/cancellation/search", status, theory.initCalls)
	}
	again, err := k.Proof()
	astTestMust(t, err)
	if again != proof {
		t.Fatal("repeated known-UNSAT check replaced its retained proof")
	}
}

func TestSMTCheckMemoryPreamblePrecedesKnownConflict(t *testing.T) {
	m, k, theory := smtCheckPhaseFixture(t, false)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, k.Assert(falsity, ast.ASTHandle{}))
	astTestMust(t, k.internalizePending())
	runtime := m.NumeralService().RuntimeOwner()
	astTestMust(t, runtime.SetWatermark(1))
	defer func() { astTestMust(t, runtime.SetWatermark(0)) }()
	before := m.Limit.Count()
	status, err := k.Check()
	astTestMust(t, err)
	if status != lbool.Undef || k.lastFailure != SMTFailureMemory || theory.initCalls != 0 || m.Limit.Count() != before {
		t.Fatal("memory preamble did not precede the known-conflict branch", status, k.lastFailure)
	}
}

func TestSMTCheckNewBooleanConflictAndPushBuildProofWithoutSearch(t *testing.T) {
	for _, push := range []bool{false, true} {
		m, k, theory := smtCheckPhaseFixture(t, true)
		theory.initError = errors.New("unexpected search initialization")
		a := smtRelevancyBool(t, m, "boolean-base-conflict-a")
		na, err := m.MkNot(a)
		astTestMust(t, err)
		astTestMust(t, k.Assert(a, ast.ASTHandle{}))
		astTestMust(t, k.Assert(na, ast.ASTHandle{}))
		if push {
			astTestMust(t, k.Push())
		}
		status, err := k.Check()
		astTestMust(t, err)
		if status != lbool.False || theory.initCalls != 0 {
			t.Fatal("Boolean base conflict entered search", push, status, theory.initCalls)
		}
		proof, err := k.Proof()
		astTestMust(t, err)
		fact, err := m.GetProofFact(proof)
		astTestMust(t, err)
		if !m.IsFalse(fact) {
			t.Fatal("Boolean base conflict/push lost its false proof")
		}
	}
}

func TestSMTCheckAssumptionsResolveBeforeSearchAndInitializeAfterAssumptions(t *testing.T) {
	m, k, theory := smtCheckPhaseFixture(t, true)
	theory.initError = errors.New("conflicting assumption must not initialize search")
	a := smtRelevancyBool(t, m, "check-phase-assumption-a")
	na, err := m.MkNot(a)
	astTestMust(t, err)
	smtHoldSubstitutionInput(t, m, na)
	astTestMust(t, k.Assert(a, ast.ASTHandle{}))
	status, err := k.CheckAssumptions([]ast.ASTHandle{na})
	astTestMust(t, err)
	if status != lbool.False || theory.initCalls != 0 || k.internalizer.Inconsistent() || k.internalizer.scopeLevel != 0 {
		t.Fatal("assumption conflict entered search or escaped its scope", status, theory.initCalls)
	}
	core := k.UnsatCore()
	if len(core) != 1 || core[0] != na {
		t.Fatal("early assumption resolution lost its core", core)
	}
	theory.initError = nil
	status, err = k.CheckAssumptions([]ast.ASTHandle{a})
	astTestMust(t, err)
	if status != lbool.True || theory.initCalls != 1 || !theory.sawAssumption {
		t.Fatal("search initialized before assumptions were installed", status, theory.initCalls, theory.sawAssumption)
	}
}

func TestSMTCheckFinalSATCancellationBecomesUnknown(t *testing.T) {
	m, k, theory := smtCheckPhaseFixture(t, false)
	theory.cancelFinal = true
	status, err := k.Check()
	m.Limit.ResetCancel()
	astTestMust(t, err)
	if status != lbool.Undef || k.lastFailure != SMTFailureCanceled || theory.initCalls != 1 {
		t.Fatal("final SAT cancellation was not observed", status, k.lastFailure)
	}
}

func TestSMTCheckFinalizesBeforeNativeAssumptionCleanup(t *testing.T) {
	m, k, theory := smtCheckPhaseFixture(t, false)
	theory.cancelPop = true
	a := smtRelevancyBool(t, m, "finalize-before-assumption-pop")
	smtHoldSubstitutionInput(t, m, a)
	status, err := k.CheckAssumptions([]ast.ASTHandle{a})
	canceled := m.Limit.CancelFlag()
	m.Limit.ResetCancel()
	astTestMust(t, err)
	// A SAT check keeps the assumption scope; it is undone by the next
	// pop_to_base_lvl, not by the check.
	if status != lbool.True || canceled || k.internalizer.scopeLevel == 0 {
		t.Fatal("SAT check popped its assumption scope", status, canceled, k.internalizer.scopeLevel)
	}
	astTestMust(t, k.Push())
	canceled = m.Limit.CancelFlag()
	m.Limit.ResetCancel()
	if !canceled || k.internalizer.scopeLevel != 1 {
		t.Fatal("pop_to_base_lvl did not undo the assumption scope", canceled, k.internalizer.scopeLevel)
	}
}

func TestSMTCheckBaseConflictAllocationFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 2048 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, k, theory := smtCheckPhaseFixture(t, true)
			a := smtRelevancyBool(t, m, "frontier-base-conflict-a")
			na, err := m.MkNot(a)
			astTestMust(t, err)
			astTestMust(t, k.Assert(a, ast.ASTHandle{}))
			astTestMust(t, k.Assert(na, ast.ASTHandle{}))
			m.Budget().FailAfter(stop)
			status, err := k.Check()
			m.Budget().FailAfter(-1)
			if theory.initCalls != 0 {
				t.Fatal("base conflict allocation path entered search")
			}
			if err == nil {
				if status != lbool.False {
					t.Fatal("base conflict result", status)
				}
				succeeded = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
		})
		if t.Failed() {
			t.Fatalf("base-conflict allocation frontier %d", stop)
		}
	}
	if !succeeded {
		t.Fatal("no successful base-conflict allocation frontier")
	}
}
