package automata

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// AUDIT.md math_misc_01-F7: sinkify_dead_states traces the dead set and
// each dead state that still had moves.
func TestSinkifyTraceListsDeadStates(t *testing.T) {
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("seq")
	if !tags.ShouldTrace("seq") {
		t.Skip("TRACE records need -tags g3trace")
	}
	automatonTraceTags = tags
	defer func() { automatonTraceTags = nil }()
	a := New(0, []uint32{2}, []Move{ConditionMove(0, 1, 3)})
	a.sinkifyDeadStates()
	text := out.String()
	if !strings.Contains(text, "[seq] sinkify_dead_states src/math/automata/automaton.h") || !strings.Contains(text, "remove: {0, 1}\n") || !strings.Contains(text, "live state 0\n") || strings.Contains(text, "live state 1\n") {
		t.Fatal(text)
	}
}
