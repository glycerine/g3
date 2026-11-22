package solver

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type retryTacticFactory struct {
	builds, closes             int
	buildFailure, closeFailure error
}

func (f *retryTacticFactory) New(*ast.ASTManager, *uparams.ParamsRef) (tactic.Tactic, error) {
	f.builds++
	if f.buildFailure != nil {
		err := f.buildFailure
		f.buildFailure = nil
		return nil, err
	}
	return tactic.NewSkipTactic(), nil
}
func (f *retryTacticFactory) Close() error {
	f.closes++
	err := f.closeFailure
	f.closeFailure = nil
	return err
}

func TestDeferredTacticRetriesConstructionAndCleanup(t *testing.T) {
	m := solverTestManager(t)
	params := solverTestParams(t, m)
	injected := errors.New("injected failure")
	f := &retryTacticFactory{buildFailure: injected, closeFailure: injected}
	s, err := NewDeferredTacticSolver(m, f, nil, false, true, true, "ALL", TacticSolverConfig{Params: params})
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, s.Close()) })
	solverMust(t, s.Push())
	solverMust(t, s.Pop(1))
	if f.builds != 0 || f.closes != 0 {
		t.Fatalf("scope change constructed or closed the factory: %+v", f)
	}
	for i := 0; i < 2; i++ {
		if _, err = s.TranslationTactic(); !errors.Is(err, injected) {
			t.Fatal("expected injected failure", i, err)
		}
	}
	a, err := s.TranslationTactic()
	solverMust(t, err)
	b, err := s.TranslationTactic()
	solverMust(t, err)
	if a == nil || a != b || f.builds != 2 || f.closes != 2 {
		t.Fatalf("expected one constructed tactic and one successful release: %+v", f)
	}
	solverMust(t, s.Close())
	if f.closes != 2 {
		t.Fatal("factory released again by solver close")
	}
}

func TestClosingUnusedTacticFactory(t *testing.T) {
	m := solverTestManager(t)
	params := solverTestParams(t, m)
	f := new(retryTacticFactory)
	s, err := NewDeferredTacticSolver(m, f, nil, false, true, true, "", TacticSolverConfig{Params: params})
	solverMust(t, err)
	solverMust(t, s.Close())
	solverMust(t, s.Close())
	if f.builds != 0 || f.closes != 1 {
		t.Fatalf("unused factory lifetime: %+v", f)
	}
}
