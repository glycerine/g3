package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:85-88,111.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BindTrace borrows an isolated trace stream. Nil restores the manager stream.
func (t *NLA2BVTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}

func (t *NLA2BVTactic) traceTags() *diagnostic.TraceTags {
	if t.trace != nil {
		return t.trace
	}
	return t.manager.BorrowMemoryTraceTags()
}

func nla2BVTraceWrite(out io.Writer, text string) error {
	n, err := io.WriteString(out, text)
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}

// Source trace stream failures set badbit but never alter tactic execution.
func (c *nla2BVInvocation) traceGoalCount(g *tactic.Goal, line int, display bool) {
	tags := c.t.traceTags()
	if !tags.ShouldTrace("nla2bv") {
		return
	}
	_ = tags.EmitConditional("nla2bv", "operator()", "src/tactic/arith/nla2bv_tactic.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			if display {
				if err := g.Display(out); err != nil {
					return err
				}
			}
			count, err := c.countMultiplications(g)
			if err != nil {
				return err
			}
			return nla2BVTraceWrite(out, "Muls: "+strconv.FormatUint(uint64(count), 10)+"\n")
		})
}

func (c *nla2BVInvocation) traceGoalDisplay(g *tactic.Goal, tag string, line int, heading string) {
	tags := c.t.traceTags()
	if !tags.ShouldTrace(tag) {
		return
	}
	_ = tags.EmitConditional(tag, "operator()", "src/tactic/arith/nla2bv_tactic.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			if heading != "" {
				if err := nla2BVTraceWrite(out, heading); err != nil {
					return err
				}
			}
			return g.Display(out)
		})
}
