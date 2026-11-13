package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
)

type stagedCLPNestedFailure struct {
	cause, refusal error
	cleanupErr     error
	remaining      int
	calls          int
	runtime        *memory.Runtime
}

func (f *stagedCLPNestedFailure) Error() string        { return f.cause.Error() }
func (f *stagedCLPNestedFailure) Unwrap() error        { return f.cause }
func (f *stagedCLPNestedFailure) OriginalCause() error { return f.cause }
func (f *stagedCLPNestedFailure) CleanupCause() error  { return f.cleanupErr }
func (f *stagedCLPNestedFailure) RuntimeOwner() *memory.Runtime {
	return f.runtime
}
func (f *stagedCLPNestedFailure) Cleanup() error {
	f.calls++
	if f.remaining > 0 {
		f.remaining--
		f.cleanupErr = f.refusal
		return f.refusal
	}
	f.cleanupErr = nil
	return nil
}

func TestCLPBackendAttemptDrainsSearchReceiptBeforeBackend(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	local, err := e.params.NewValue()
	must(err)
	backend, err := smtcore.NewSMTSolver(m, &local, "", smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	must(err)
	engine, err := clp.NewEngine(m, e.set, backend, 20)
	must(err)
	// The kernel is the engine's (clp::imp::m_solver); the attempt borrows it.
	attempt := &clpBackendAttempt{manager: m, engine: engine}
	defer func() {
		must(attempt.close())
		must(backend.Close())
		must(e.params.Del(&local))
	}()
	cause := errors.New("CLP search cleanup pending")
	refusal := errors.New("CLP search cleanup retry refused")
	nested := &stagedCLPNestedFailure{
		cause: cause, refusal: refusal, remaining: 1,
		runtime: m.NumeralService().RuntimeOwner(),
	}
	failure := attempt.finish(nested)
	if !errors.Is(failure, cause) || !errors.Is(failure, refusal) || nested.calls != 1 {
		t.Fatal("backend did not expose nested cleanup refusal", failure, nested.calls)
	}
	if attempt.nested != nested || attempt.engine != engine {
		t.Fatal("backend owners advanced before nested search cleanup")
	}
	if _, err = engine.Statistics(); err != nil {
		t.Fatal("nested refusal closed the CLP engine", err)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != nested.runtime {
		t.Fatal("backend refusal has no retry owner", failure)
	}
	original, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if nested.calls != 2 || attempt.nested != nil || attempt.engine != nil || cleanup.CleanupCause() != nil {
		t.Fatal("backend retry did not drain nested search first", nested.calls, attempt.nested, attempt.engine, cleanup.CleanupCause())
	}
	if cleanup.OriginalCause() != original || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message {
		t.Fatal("backend cleanup changed failure evidence")
	}
}

var _ memory.ConstructionFailure = (*stagedCLPNestedFailure)(nil)
