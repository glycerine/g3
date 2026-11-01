package tactic

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type recordingBaseTactic struct {
	*primitiveTactic
	applyErr error
	cleaned  int
}

func (t *recordingBaseTactic) Apply(*Goal, *TacticResult) error { return t.applyErr }
func (t *recordingBaseTactic) Cleanup() error {
	t.cleaned++
	return nil
}

func TestBaseTacticSkipFailUndecidedAndExecCleanup(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	p := goalTestAtom(t, m, "base-p")
	astTestMust(t, g.Assert(p))
	result := &TacticResult{}
	skip := NewSkipTactic()
	astTestMust(t, Exec(skip, g, result))
	if len(result.Goals) != 1 || result.Goals[0] != g || g.RefCount != 0 {
		t.Fatal("skip did not borrow exactly the input goal", len(result.Goals), g.RefCount)
	}
	astTestMust(t, result.Close())
	if g.RefCount != 0 {
		t.Fatal("result did not release goal", g.RefCount)
	}
	if err := Exec(NewFailTactic(), g, &TacticResult{}); err == nil || err.Error() != "fail tactic" {
		t.Fatal("fail tactic", err)
	}
	if err := Exec(NewFailIfUndecidedTactic(), g, &TacticResult{}); err == nil || err.Error() != "undecided" {
		t.Fatal("fail-if-undecided", err)
	}
	decided := goalTestNew(t, m, false, false)
	for _, passthrough := range []Tactic{
		NewFailIfUndecidedTactic(),
		NewReportVerboseTactic("base report", ^uint32(0)),
		NewTraceTactic("base-disabled-trace"),
	} {
		out := &TacticResult{}
		astTestMust(t, Exec(passthrough, decided, out))
		if len(out.Goals) != 1 || out.Goals[0] != decided {
			t.Fatal("primitive did not return the decided input", out.Goals)
		}
		astTestMust(t, out.Close())
	}
	record := &recordingBaseTactic{primitiveTactic: newPrimitiveTactic(primitiveSkip, "", 0), applyErr: errors.New("apply stopped")}
	if err := Exec(record, g, &TacticResult{}); err == nil || record.cleaned != 1 {
		t.Fatal("exec must clean after an apply error", err, record.cleaned)
	}
}

func TestBaseTacticCheckSatStatusesModelsAndReason(t *testing.T) {
	m := astTestManager(t)
	md := &ModelRef{}
	t.Cleanup(func() { astTestMust(t, md.Close()) })
	labels := &ModelLabels{}
	proof := astTestPin(t, m, ast.ASTHandle{})
	core := &DependencyRef{}
	t.Cleanup(func() { astTestMust(t, core.Close()) })
	empty := goalTestNew(t, m, false, false)
	status, reason, err := CheckSat(NewSkipTactic(), empty, md, labels, proof, core)
	astTestMust(t, err)
	if status != lbool.True || reason != "" {
		t.Fatal("empty precise goal", status, reason)
	}
	if value, err := md.Get(); err != nil || value == nil {
		t.Fatal("SAT check did not create empty model", value, err)
	}
	status, reason, err = CheckSat(NewFailTactic(), empty, md, labels, proof, core)
	astTestMust(t, err)
	if status != lbool.Undef || reason != "fail tactic" {
		t.Fatal("failure reason", status, reason)
	}
	unsat := goalTestNew(t, m, false, false)
	no, err := m.MkFalse()
	astTestMust(t, err)
	astTestMust(t, unsat.Assert(no))
	status, reason, err = CheckSat(NewSkipTactic(), unsat, md, labels, proof, core)
	astTestMust(t, err)
	if status != lbool.False || reason != "" {
		t.Fatal("false goal", status, reason)
	}
	unknown := goalTestNew(t, m, false, false)
	astTestMust(t, unknown.Assert(goalTestAtom(t, m, "unknown-p")))
	status, reason, err = CheckSat(NewSkipTactic(), unknown, md, labels, proof, core)
	astTestMust(t, err)
	if status != lbool.Undef || reason != "incomplete" {
		t.Fatal("undecided goal", status, reason)
	}
}

func TestBaseTacticFeatureGuardsAndReferenceLifetime(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	all := goalTestNew(t, m, true, true)
	for _, row := range []struct {
		call func(string, *Goal) error
		want string
	}{
		{FailIfProofGeneration, "x does not support proof production"},
		{FailIfUnsatCoreGeneration, "x does not support unsat core production"},
		{FailIfModelGeneration, "x does not generate models"},
	} {
		if err := row.call("x", all); err == nil || err.Error() != row.want {
			t.Fatal(row.want, err)
		}
	}
	tactic := NewSkipTactic()
	astTestMust(t, tactic.IncRef())
	astTestMust(t, tactic.DecRef())
	if err := tactic.Cleanup(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("final tactic decrement did not retire owner", err)
	}
}

func TestBaseTacticResultGoalOwnershipAndFactory(t *testing.T) {
	m := astTestManager(t)
	owned := goalTestNew(t, m, false, false)
	result := &TacticResult{manager: m}
	astTestMust(t, result.TakeGoal(owned))
	astTestMust(t, result.Close())
	if _, err := owned.Size(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("transferred goal remained open", err)
	}

	factory := TacticFactoryFunc(func(got *ast.ASTManager, _ *uparams.ParamsRef) (Tactic, error) {
		if got != m {
			t.Fatal("factory received another manager")
		}
		return NewSkipTactic(), nil
	})
	created, err := factory.New(m, nil)
	astTestMust(t, err)
	astTestMust(t, created.IncRef())
	astTestMust(t, created.DecRef())
	if _, err = (TacticFactoryFunc(nil)).New(m, nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil factory accepted", err)
	}
}

func TestBaseTacticExceptionMessages(t *testing.T) {
	got := [...]string{
		TacticCanceledMessage,
		TacticMaxMemoryMessage,
		TacticMaxScopesMessage,
		TacticMaxStepsMessage,
		TacticMaxFramesMessage,
		TacticNoProofGenerationMessage,
	}
	want := [...]string{
		"canceled",
		"max. memory exceeded",
		"max. scopes exceeded",
		"max. steps exceeded",
		"max. frames exceeded",
		"component does not support proof generation",
	}
	if got != want {
		t.Fatal("tactic exception message aliases", got)
	}
}

func TestTacticExceptionPreservesCauseAndMessage(t *testing.T) {
	cause := errors.Join(arena.ErrInjectedFailure, errors.New("cleanup pending"))
	wrapped := WrapTacticException(cause)
	if wrapped.Message() != cause.Error() || wrapped.Error() != cause.Error() || !errors.Is(wrapped, arena.ErrInjectedFailure) {
		t.Fatal("tactic exception lost its message or cause", wrapped)
	}
	var typed TacticException
	if !errors.As(wrapped, &typed) || typed.HasErrorCode() {
		t.Fatal("tactic exception type changed", wrapped)
	}
}
