package pdr_test

// Source: pdr_context.cpp context::solve IF_VERBOSE(1) reachable branch.

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type pdrVerboseRefusingWriter struct{}

func (pdrVerboseRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestContextSolveVerboseSearchOperandIsLazy(t *testing.T) {
	for _, level := range []uint32{0, 1} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			fixture := newValidationBatchFixture(t, "solve_verbose_search")
			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			pdrMust(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			pdrMust(t, err)
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: fixture.manager.Budget()})
			pdrMust(t, err)
			defer func() {
				pdrMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
				pdrMust(t, diagnostic.SetProcessVerboseStream(oldStream))
				pdrMust(t, stream.Close())
			}()
			pdrMust(t, diagnostic.SetProcessVerbosityLevel(level))
			pdrMust(t, diagnostic.SetProcessVerboseStream(stream))
			status, err := fixture.context.Solve(fixture.declaration)
			pdrMust(t, err)
			if status != lbool.True {
				t.Fatal("reachable result", status)
			}
			output, err := stream.String()
			pdrMust(t, err)
			if level == 0 {
				if output != "" {
					t.Fatalf("disabled IF_VERBOSE evaluated operand: %q", output)
				}
			} else if !strings.HasPrefix(output, "\n") || !strings.Contains(output, "goals ") {
				t.Fatalf("reachable search display = %q", output)
			}
		})
	}
}

func TestContextSolveIgnoresVerboseStreamRefusal(t *testing.T) {
	fixture := newValidationBatchFixture(t, "solve_verbose_refusal")
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	pdrMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	pdrMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: fixture.manager.Budget(), BorrowedWriter: pdrVerboseRefusingWriter{}})
	pdrMust(t, err)
	defer func() {
		pdrMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		pdrMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		pdrMust(t, stream.Close())
	}()
	pdrMust(t, diagnostic.SetProcessVerbosityLevel(1))
	pdrMust(t, diagnostic.SetProcessVerboseStream(stream))
	status, err := fixture.context.Solve(fixture.declaration)
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("verbose refusal changed reachable result", status)
	}
}
