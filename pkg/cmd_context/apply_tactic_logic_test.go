package cmd_context

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type logicRecordingTactic struct {
	tactic.Tactic
	setLogic int
}

func (t *logicRecordingTactic) SetLogic(l usymbol.Symbol) error {
	t.setLogic++
	return t.Tactic.SetLogic(l)
}

// AUDIT.md cmd_context_01-F13: apply_tactic_cmd::execute never calls
// set_logic; only check-sat-using does.
func TestApplyTacticDoesNotSetLogic(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_LIA"))
	recorder := &logicRecordingTactic{Tactic: tactic.NewSkipTactic()}
	var out bytes.Buffer
	contextMust(t, c.ApplyTactic(&out, recorder, nil))
	if recorder.setLogic != 0 {
		t.Fatalf("SetLogic called %d times", recorder.setLogic)
	}
}
