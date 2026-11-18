package core

// Source: src/util/trace.h TRACE(tag, g->display(tout)) in operator().

import (
	"io"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// coreTraceGoal is TRACE(tag, g->display(tout);) in a tactic's operator().
func coreTraceGoal(tag, file string, line int, g *tactic.Goal) error {
	tags := diagnostic.BorrowProcessTraceTags()
	err := tags.EmitConditional(tag, "operator()", file, line, func() bool { return true }, func(w io.Writer) error { return g.Display(w) })
	if err != nil && !tags.IsStreamFailure(err) {
		return err
	}
	return nil
}
