package smtcore

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
)

// Deliver Boolean assignments without running feasibility or propagation. This
// lets final_check itself discover the actual numerical result.
func lraFinalCheckAssignments(t *testing.T, f *lraFrontendFixture) {
	t.Helper()
	astTestMust(t, f.s.propagateTheoryRelevancy())
	astTestMust(t, f.s.PropagateTheoryAssignments())
	for f.a.assignmentHead < len(f.a.assignments) {
		a := f.a.assignments[f.a.assignmentHead]
		b := f.a.boundByBoolean[a.boolean]
		_, err := f.a.numeric.assertBound(b.variable, a.boolean, &b.value, b.upper, a.truth)
		astTestMust(t, err)
		f.a.assignmentHead++
	}
	// Model a retry after an inconclusive search. The callback must compute
	// its result by solving these real constraints, rather than reuse OPTIMAL.
	f.a.numeric.solver.Bounds.Status.Value = lp.StatusUnknown
}

func TestSMTLRAFinalCheckFeasibilityCacheAndUnsupportedScope(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x := f.variable("lra-final-x")
	f.assert(f.app(smtArithOpGE, x, f.number(3)), true)
	lraFinalCheckAssignments(t, f)
	f.a.modelValues = map[uint32]*big.Rat{0: big.NewRat(-17, 1)}
	status, err := f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone || f.a.numeric.solver.Bounds.Status.Value != lp.StatusOptimal || f.a.modelValues != nil {
		t.Fatal("final feasibility did not solve and invalidate model", status)
	}
	v := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	value, err := f.a.numeric.value(v)
	astTestMust(t, err)
	if value.Main.Cmp(big.NewRat(3, 1)) < 0 {
		t.Fatal("infeasible final value", value)
	}
	// An optimal solver skips make_feasible and preserves cached model values.
	cached := big.NewRat(19, 1)
	f.a.modelValues = map[uint32]*big.Rat{0: cached}
	// The source does not add an eager cancellation check to an already
	// optimal state without shared variables.
	f.m.Limit.Cancel()
	status, err = f.a.FinalCheck()
	f.m.Limit.ResetCancel()
	astTestMust(t, err)
	if status != SMTFinalCheckDone || f.a.modelValues[0] != cached {
		t.Fatal("optimal final check discarded model", status)
	}
	astTestMust(t, f.s.PushScope())
	opaque := f.app(smtArithOpMul, x, x)
	astTestMust(t, f.s.Internalize(opaque, false))
	status, err = f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckGiveUp || f.a.notHandled != opaque {
		t.Fatal("nonlinear term accepted", status)
	}
	astTestMust(t, f.s.PopScopes(1))
	status, err = f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone {
		t.Fatal("popped unsupported term blocked parent", status)
	}
}

func TestSMTLRAFinalCheckNumericalConflict(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	x, y := f.variable("lra-final-conflict-x"), f.variable("lra-final-conflict-y")
	f.assert(f.app(smtArithOpGE, x, f.number(0)), true)
	f.assert(f.app(smtArithOpGE, y, f.number(0)), true)
	f.assert(f.app(smtArithOpLE, f.app(smtArithOpAdd, x, y), f.number(-1)), true)
	lraFinalCheckAssignments(t, f)
	if f.s.inconsistent {
		t.Fatal("Boolean layer preempted numerical conflict")
	}
	status, err := f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || !f.s.inconsistent || f.a.conflictCount != 1 || f.a.numeric.solver.Bounds.Status.Value != lp.StatusInfeasible {
		t.Fatal("final check lost numerical conflict", status, f.a.conflictCount)
	}
}

type lraFinalCheckLimit struct {
	calls int
	err   error
}

func (l *lraFinalCheckLimit) CancelFlag() (bool, error) { l.calls++; return true, l.err }

func TestSMTLRAFinalCheckUnknownCancellationAndFailure(t *testing.T) {
	for _, mode := range []string{"resource-limit", "manager-canceled", "error"} {
		t.Run(mode, func(t *testing.T) {
			f := newLRAFrontendFixture(t, true)
			f.a.numeric.solver.Settings.SimplexStrategy = lp.StrategyLU
			x, y := f.variable("lra-final-limit-x"), f.variable("lra-final-limit-y")
			f.assert(f.app(smtArithOpGE, f.app(smtArithOpAdd, x, y), f.number(3)), true)
			lraFinalCheckAssignments(t, f)
			limit := &lraFinalCheckLimit{}
			f.a.numeric.solver.Settings.SetResourceLimit(limit)
			want := SMTFinalCheckGiveUp
			if mode == "manager-canceled" {
				f.m.Limit.Cancel()
				defer f.m.Limit.ResetCancel()
				want = SMTFinalCheckContinue
			}
			if mode == "error" {
				limit.err = errors.New("LRA final cancellation read")
			}
			status, err := f.a.FinalCheck()
			if status != want || !errors.Is(err, limit.err) || limit.calls == 0 {
				t.Fatal("final resource result", status, err, limit.calls)
			}
			if mode == "error" {
				if !f.a.numeric.stopped {
					t.Fatal("numerical failure was not retained")
				}
				_, err = f.a.FinalCheck()
				if err != lp.ErrExactVectorPending {
					t.Fatal("failed owner was reused", err)
				}
			} else if f.a.numeric.solver.Bounds.Status.Value != lp.StatusTimeExhausted {
				t.Fatal("fixture did not reach actual unknown", f.a.numeric.solver.Bounds.Status.Value)
			}
		})
	}
}

func TestSMTLRAFinalCheckDelayedBeforeFreshAndUnsupported(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	config := f.a.config
	config.PropagateEqs = false
	astTestMust(t, f.a.ConfigureArithmetic(config))
	x, y := f.variable("lra-final-delayed-x"), f.variable("lra-final-delayed-y")
	lraFix(t, f, x, 1)
	lraFix(t, f, y, 1)
	f.propagate(true)
	// These variables are not shared. Only the already queued candidate can
	// request progress, and it takes precedence over the unsupported marker.
	xv := f.s.context.enodes.get(x).TheoryVar(f.a.FamilyID())
	yv := f.s.context.enodes.get(y).TheoryVar(f.a.FamilyID())
	f.a.assumeCandidates = append(f.a.assumeCandidates, smtArithmeticEqualityCandidate{xv, yv})
	astTestMust(t, f.s.Internalize(f.app(smtArithOpMul, x, x), false))
	status, err := f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || f.a.assumeHead != 1 {
		t.Fatal("delayed candidate was bypassed", status, f.a.assumeHead)
	}
	eq := f.pin(f.m.MkEq(x, y))
	lit, err := f.s.Literal(eq)
	astTestMust(t, err)
	value, err := f.s.Assignment(lit)
	astTestMust(t, err)
	if value != lbool.Undef {
		t.Fatal("model candidate became an assertion", value)
	}
	status, err = f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckGiveUp {
		t.Fatal("unsupported marker was lost after candidates", status)
	}
}

func TestSMTLRAFinalCheckFreshSharedBeforeUnsupported(t *testing.T) {
	f := newLRAFrontendFixture(t, true)
	config := f.a.config
	config.PropagateEqs = false
	astTestMust(t, f.a.ConfigureArithmetic(config))
	x, y := f.variable("lra-final-shared-x"), f.variable("lra-final-shared-y")
	decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "lra-final-shared-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{}))
	for _, v := range []ast.ASTHandle{x, y} {
		astTestMust(t, f.s.Internalize(f.pin(f.m.MkApp(decl, []ast.ASTHandle{v})), false))
		lraFix(t, f, v, 1)
	}
	f.propagate(true)
	for _, v := range []ast.ASTHandle{x, y} {
		astTestMust(t, f.s.relevancy.MarkRelevant(v))
	}
	astTestMust(t, f.s.relevancy.Propagate())
	astTestMust(t, f.s.Internalize(f.app(smtArithOpMul, x, x), false))
	status, err := f.a.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || len(f.a.assumeCandidates) != 1 || f.a.assumeHead != 1 {
		t.Fatal("fresh shared candidate was bypassed", status, f.a.assumeCandidates)
	}
}

func TestSMTLRAFinalCheckKernelModelConflictAndRecovery(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	k := smtTestKernel(t, m)
	astTestMust(t, k.ConfigureModelGeneration(true, cfg))
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	a, err := NewSMTLRATheory(family)
	astTestMust(t, err)
	astTestMust(t, k.RegisterTheory(a))
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	f := &lraFrontendFixture{t, m, k.internalizer, a, sort}
	x, y := f.variable("lra-final-kernel-x"), f.variable("lra-final-kernel-y")
	for _, h := range []ast.ASTHandle{
		f.app(smtArithOpGE, x, f.number(0)),
		f.app(smtArithOpLE, x, f.number(4)),
		f.app(smtArithOpLE, y, f.number(2)),
		f.app(smtArithOpGE, f.app(smtArithOpAdd, x, y), f.number(5)),
	} {
		astTestMust(t, k.Assert(h, ast.ASTHandle{}))
	}
	check := func(want lbool.LBool, wantModel []*big.Rat) {
		t.Helper()
		got, err := k.Check()
		astTestMust(t, err)
		if got != want {
			t.Fatal("concrete LRA kernel result", got, want)
		}
		if got != lbool.True {
			return
		}
		if k.internalizer.theories[family] != a || a.numeric.assertLower+a.numeric.assertUpper == 0 {
			t.Fatal("kernel replaced the concrete backend")
		}
		md := k.GeneratedModel()
		if md == nil {
			t.Fatal("missing generated model")
		}
		values := make([]*big.Rat, 2)
		for i, v := range []ast.ASTHandle{x, y} {
			decl, err := m.GetDecl(v)
			astTestMust(t, err)
			h, err := md.ConstantInterpretation(decl)
			astTestMust(t, err)
			if h.IsNull() {
				t.Fatal("missing arithmetic interpretation", i)
			}
			var matched bool
			values[i], matched, err = a.numeral(h)
			astTestMust(t, err)
			if !matched {
				t.Fatal("model value is not a numeral")
			}
		}
		if values[0].Cmp(wantModel[0]) != 0 || values[1].Cmp(wantModel[1]) != 0 {
			t.Fatal("generated arithmetic model", values, wantModel)
		}
	}
	// z3 4.7 theory_lra gives x=4, y=1 (QF_LRA; lar_term's unordered_map
	// visits x before y in the tableau row).
	check(lbool.True, []*big.Rat{big.NewRat(4, 1), big.NewRat(1, 1)})
	astTestMust(t, k.Push())
	astTestMust(t, k.Assert(f.app(smtArithOpLE, x, f.number(2)), ast.ASTHandle{}))
	check(lbool.False, nil)
	astTestMust(t, k.Pop(1))
	// Source behaviour, reproduced: lar_solver::pop restores m_status to the
	// outer OPTIMAL but keeps the inner scope's column values (x=2, y=3), so
	// final_check_eh skips make_feasible and the model violates y <= 2. z3
	// 4.7 prints the same model for the QF_LRA script.
	check(lbool.True, []*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)})
}

func TestSMTLRAFinalCheckKernelSharedEquality(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	k := smtTestKernel(t, m)
	k.arithState.PropagateEqs = false
	a, err := NewSMTLRATheory(family)
	astTestMust(t, err)
	astTestMust(t, k.RegisterTheory(a))
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	f := &lraFrontendFixture{t, m, k.internalizer, a, sort}
	x, y := f.variable("lra-final-uf-x"), f.variable("lra-final-uf-y")
	decl := f.pin(m.MkFuncDecl(astTestSymbol(t, m, "lra-final-uf-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{}))
	fx, fy := f.pin(m.MkApp(decl, []ast.ASTHandle{x})), f.pin(m.MkApp(decl, []ast.ASTHandle{y}))
	for _, v := range []ast.ASTHandle{x, y} {
		astTestMust(t, k.Assert(f.app(smtArithOpGE, v, f.number(7)), ast.ASTHandle{}))
		astTestMust(t, k.Assert(f.app(smtArithOpLE, v, f.number(7)), ast.ASTHandle{}))
	}
	astTestMust(t, k.Push())
	astTestMust(t, k.Assert(f.pin(m.MkNot(f.pin(m.MkEq(fx, fy)))), ast.ASTHandle{}))
	status, err := k.Check()
	astTestMust(t, err)
	if status != lbool.False || a.fixedEqualities != 0 || k.internalizer.statistics.NumInterfaceEqs == 0 {
		t.Fatal("shared equality did not reach Boolean/EUF conflict", status, a.fixedEqualities, k.internalizer.statistics.NumInterfaceEqs)
	}
	astTestMust(t, k.Pop(1))
	status, err = k.Check()
	astTestMust(t, err)
	if status != lbool.True || k.internalizer.theories[family] != a {
		t.Fatal("shared equality scope recovery", status)
	}
}
