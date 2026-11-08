package smtcore

// Execution traces for comparison with a z3 TRACE build. Each call site is
// guarded by xtrace.Enabled, a constant that is false unless the build uses
// -tags xtrace, so the arguments are not evaluated in ordinary builds.
//
// Lines have the form "XTRACE: [tag] text", where tag is the z3 TRACE tag
// and text is the z3 trace text with whitespace collapsed to single spaces.
// util/xtrace/z3trace.py converts a z3 .z3-trace file to the same form.

import (
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// xtraceExpr is mk_pp(e, m) on one line.
func xtraceExpr(m *ast.ASTManager, h ast.ASTHandle) string {
	if h == (ast.ASTHandle{}) {
		return "null"
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	if err != nil {
		return "<" + err.Error() + ">"
	}
	defer stream.Close()
	f, err := m.SMT2PPExprTree(h, nil)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	fm, err := m.GetFormatManager()
	if err != nil {
		return "<" + err.Error() + ">"
	}
	ref, err := fm.NewRef(f)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	defer fm.DelRef(&ref)
	if err = m.PrintFormat(stream, f, nil, nil, nil); err != nil {
		return "<" + err.Error() + ">"
	}
	text, err := stream.String()
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return xtraceLine(text)
}

// xtraceLine collapses whitespace runs, including newlines, to one space.
func xtraceLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// xtraceLiteral is the z3 literal display "pN" or "(not pN)" followed by the
// literal's expression, as display_literal_verbose prints it.
func (s *SMTInternalizerState) xtraceLiteral(l SMTLiteral) string {
	e := ""
	if int(l.Var()) < len(s.boolExpr) {
		e = xtraceExpr(s.manager, s.boolExpr[l.Var()])
	}
	if l.Sign() {
		return "(not p" + itoa32(l.Var()) + ") (not " + e + ")"
	}
	return "p" + itoa32(l.Var()) + " " + e
}

func itoa32(v int32) string {
	var b [12]byte
	i := len(b)
	neg := v < 0
	u := int64(v)
	if neg {
		u = -u
	}
	for {
		i--
		b[i] = byte('0' + u%10)
		u /= 10
		if u == 0 {
			break
		}
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
