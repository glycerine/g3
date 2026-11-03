package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp's zero-rule query and
// query_from_lvl branches return false through context::set_unsat without
// entering context::solve.

import (
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type spacerEmptyProgramRefusingWriter struct{}

func (spacerEmptyProgramRefusingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestSpacerEmptyProgramBypassesSolveLifecycle(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	fragmentMust(t, f.engine.globals.SourceSetName("fixedpoint.pdr.validate_result", "true"))
	fragmentMust(t, f.engine.globals.SourceSetName("fixedpoint.print_statistics", "true"))
	fragmentMust(t, f.engine.SelectEngine("spacer"))

	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	fragmentMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	fragmentMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{
		Budget:         f.manager.Budget(),
		BorrowedWriter: spacerEmptyProgramRefusingWriter{},
	})
	fragmentMust(t, err)
	defer func() {
		f.manager.Budget().FailAfter(-1)
		fragmentMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		fragmentMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		fragmentMust(t, stream.Close())
	}()
	fragmentMust(t, diagnostic.SetProcessVerbosityLevel(0))
	fragmentMust(t, diagnostic.SetProcessVerboseStream(stream))

	for iteration := 0; iteration < 2; iteration++ {
		var status lbool.LBool
		var queryErr error
		if iteration == 0 {
			status, queryErr = f.engine.QueryPredicate(f.declarations["u"])
		} else {
			status, queryErr = f.engine.QueryPredicateFromLevel(f.declarations["u"], 4)
		}
		fragmentMust(t, queryErr)
		if status != lbool.False {
			t.Fatal("empty Spacer program result", iteration, status)
		}
		if stream.FileBad {
			t.Fatal("empty Spacer program entered solve statistics", iteration)
		}

		result, modelErr := f.engine.Model(f.declarations["u"])
		fragmentMust(t, modelErr)
		fragmentMust(t, result.Close())
		answer, answerErr := f.engine.Answer(f.declarations["u"])
		fragmentMust(t, answerErr)
		fragmentMust(t, f.manager.DelRef(&answer))
	}
}
