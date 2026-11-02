package spacer

// Source: src/muz/spacer/spacer_context.cpp context::solve prints collected
// statistics after every completed solve when print_statistics is enabled.

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type spacerStatisticsRefusingWriter struct{}

func (spacerStatisticsRefusingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestContextConsumesSolveStatisticsPolicy(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	if err := globals.SourceSetName("fixedpoint.print_statistics", "true"); err != nil {
		t.Fatal(err)
	}
	context, err := NewContext(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if !context.printStatistics {
		t.Fatal("print_statistics policy was not consumed")
	}
}

func TestContextSolvePrintsStatisticsForEveryResult(t *testing.T) {
	for _, test := range []struct {
		name      string
		recursive bool
		levels    uint32
		want      lbool.LBool
		early     bool
	}{
		{"sat", false, 1, lbool.True, false},
		{"unreachable", true, 3, lbool.False, false},
		{"bounded-unknown", true, 0, lbool.Undef, false},
		{"early-unreachable", true, 1, lbool.False, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newContextAnswerFixture(t, test.recursive)
			context, manager := fixture.context, fixture.context.manager
			if test.early {
				installEmptyFixtureQuery(t, fixture)
			}
			context.printStatistics = true
			original, err := diagnostic.BorrowProcessVerboseStream()
			if err != nil {
				t.Fatal(err)
			}
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: manager.Budget()})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := diagnostic.SetProcessVerboseStream(original); err != nil {
					t.Fatal(err)
				}
				if err := stream.Close(); err != nil {
					t.Fatal(err)
				}
			}()
			if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
				t.Fatal(err)
			}
			status, err := context.Solve(0, test.levels)
			if err != nil || status != test.want {
				t.Fatal("solve", status, err)
			}
			text, err := stream.String()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(text, "BRUNCH_STAT max_query_lvl ") ||
				!strings.Contains(text, "BRUNCH_STAT cex_depth ") ||
				!strings.Contains(text, ":time.spacer.solve ") {
				t.Fatal("solve statistics output", text)
			}
		})
	}
}

func TestContextSolveStatisticsPreservesStreamAndTimerSemantics(t *testing.T) {
	fixture := newContextAnswerFixture(t, false)
	context, manager := fixture.context, fixture.context.manager
	context.printStatistics = true
	original, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	refusing, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: manager.Budget(), BorrowedWriter: spacerStatisticsRefusingWriter{}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := diagnostic.SetProcessVerboseStream(original); err != nil {
			t.Fatal(err)
		}
		if err := refusing.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err = diagnostic.SetProcessVerboseStream(refusing); err != nil {
		t.Fatal(err)
	}
	status, err := context.Solve(0, 1)
	if err != nil || status != lbool.True || !refusing.FileBad {
		t.Fatal("refused statistics stream changed solve semantics", status, err, refusing.FileBad)
	}

	collected := statistics.New()
	if err = context.CollectStatistics(collected); err != nil {
		t.Fatal(err)
	}
	foundSolveTime := false
	for index := 0; index < collected.Size(); index++ {
		foundSolveTime = foundSolveTime || collected.Key(index) == "time.spacer.solve"
	}
	if !foundSolveTime {
		t.Fatal("statistics printing reset the completed solve timer")
	}
	if err = context.ResetStatistics(); err != nil {
		t.Fatal(err)
	}
	collected.Reset()
	if err = context.CollectStatistics(collected); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < collected.Size(); index++ {
		if collected.Key(index) == "time.spacer.solve" {
			t.Fatal("statistics reset left the solve timer")
		}
	}
}

func TestContextUnknownSearchCompletionPrintsStatistics(t *testing.T) {
	fixture := newContextAnswerFixture(t, true)
	context, manager := fixture.context, fixture.context.manager
	context.printStatistics = true
	original, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: manager.Budget()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := diagnostic.SetProcessVerboseStream(original); err != nil {
			t.Fatal(err)
		}
		if err := stream.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
		t.Fatal(err)
	}
	status, err := context.finishSearchFailure(fmt.Errorf("weak search: %w", ErrWeakAbstractionExhausted))
	if err != nil || status != lbool.Undef || context.bounded {
		t.Fatal("source unknown completion", status, err, context.bounded)
	}
	foreign := errors.New("foreign search failure")
	if _, err = context.finishSearchFailure(foreign); !errors.Is(err, foreign) {
		t.Fatal("non-unknown search failure was swallowed", err)
	}
	if err = context.printSolveStatistics(); err != nil {
		t.Fatal(err)
	}
	text, err := stream.String()
	if err != nil || !strings.Contains(text, "BRUNCH_STAT max_query_lvl ") || !strings.Contains(text, ":time.spacer.init_rules ") {
		t.Fatal("unknown statistics output", text, err)
	}
}
