package api

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

type pausedSolverInput struct {
	io.Reader
	entered chan struct{}
	resume  chan struct{}
}

func (r *pausedSolverInput) Read(p []byte) (int, error) {
	if r.entered != nil {
		close(r.entered)
		r.entered = nil
		<-r.resume
	}
	return r.Reader.Read(p)
}

// Finish and close the first owner while the second owner is still in its
// parser. Each solver stays on its own goroutine throughout its lifetime.
func TestOverlappingPrivateSolverParsers(t *testing.T) {
	start := func(script string) (chan struct{}, chan error) {
		entered, resume := make(chan struct{}), make(chan struct{})
		result := make(chan error, 1)
		go func() {
			result <- checkPausedSolver(&pausedSolverInput{strings.NewReader(script), entered, resume})
			close(result)
		}()
		t.Cleanup(func() {
			select {
			case <-resume:
			default:
				close(resume)
			}
			<-result
		})
		select {
		case <-entered:
		case err := <-result:
			t.Fatalf("solver returned before reading its assertions: %v", err)
		}
		return resume, result
	}
	first, firstResult := start("(declare-const p Bool)(assert p)")
	second, secondResult := start("(declare-const p Bool)(assert (not p))")
	close(first)
	if err := <-firstResult; err != nil {
		t.Fatal(err)
	}
	close(second)
	if err := <-secondResult; err != nil {
		t.Fatal(err)
	}
}

func checkPausedSolver(input io.Reader) (err error) {
	ctx, err := NewSMTContext()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, ctx.Close()) }()
	s, err := ctx.NewSolver()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	if err := s.LoadSMT2(input); err != nil {
		return err
	}
	answer, err := s.Check()
	if err != nil {
		return err
	}
	if answer != lbool.True {
		return errors.New("satisfiable private assertions were not satisfied")
	}
	return nil
}
