package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSpacerGroundAnswerClosedDiagnosticDoesNotInventReferenceError(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c, m := f.context, f.context.manager
	status, err := c.Solve(0, 4)
	depthMust(t, err)
	if status != lbool.True {
		t.Fatal("expected reachable fixture", status)
	}
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	depthMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	depthMust(t, err)
	budget := arena.NewBudget(1 << 20)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: budget})
	depthMust(t, err)
	defer func() {
		depthMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		depthMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		depthMust(t, stream.Close())
		if budget.Used() != 0 {
			t.Fatal("diagnostic stream leaked")
		}
	}()
	depthMust(t, diagnostic.SetProcessVerbosityLevel(2))
	depthMust(t, diagnostic.SetProcessVerboseStream(stream))
	depthMust(t, stream.Close())
	before := m.Stats().LiveReferences

	answer, cause := c.GroundSatAnswer(f.query)
	depthMust(t, diagnostic.SetProcessVerboseStream(oldStream))
	if answer != (ast.ASTRef{}) {
		depthMust(t, m.DelRef(&answer))
		t.Fatal("failed answer published a reference", cause)
	}
	if !errors.Is(cause, arena.ErrClosed) {
		t.Fatal("missing closed diagnostic refusal", cause)
	}
	if errors.Is(cause, ast.ErrASTHandle) {
		t.Fatal("cleanup invented a reference error", cause)
	}
	if m.Stats().LiveReferences != before {
		t.Fatal("failed answer leaked reference slots")
	}
	answer, err = c.GroundSatAnswer(f.query)
	depthMust(t, err)
	h, err := m.RefGet(answer)
	depthMust(t, err)
	truth := m.IsTrue(h)
	if !truth {
		t.Fatal("trivial source answer changed")
	}
	depthMust(t, m.DelRef(&answer))
}
