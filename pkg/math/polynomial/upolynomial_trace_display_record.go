package polynomial

// Source: TRACE("upolynomial", tout << text; display(tout, sz, p); tout << "\n";)
// sites of src/math/polynomial/upolynomial.cpp outside the Sturm and
// refinement scopes.

import "io"

// traceUPolynomialRecord emits prefix, display(v) and suffix under tag
// "upolynomial". The display receipt is call-local and completed at once.
func (m *UPolynomialManager) traceUPolynomialRecord(function string, line int, prefix string, v UPolynomialView, suffix string) error {
	return m.traceUPolynomialRecordLazy(function, line, func() (string, error) { return prefix, nil }, v, suffix)
}

// traceUPolynomialRecordLazy evaluates the prefix operands only when the tag
// is on, as the source evaluates TRACE operands only then. They are
// evaluated before the record is opened, outside the output callback.
func (m *UPolynomialManager) traceUPolynomialRecordLazy(function string, line int, prefix func() (string, error), v UPolynomialView, suffix string) error {
	tags := m.config.Integer.BorrowTraceTags()
	if !tags.ShouldTrace("upolynomial") {
		return nil
	}
	text, err := prefix()
	if err != nil {
		return err
	}
	var receipt upolyDisplayInvocationReceipt
	var computation error
	m.callbackDepth++
	_ = tags.EmitConditional("upolynomial", function, "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(w io.Writer) error {
		out := upolyRemainderTraceStream{w}
		_ = out.AppendCString(text)
		computation = m.displayIntoWithReceipt(v, out, "x", false, &receipt)
		if computation == nil {
			_ = out.AppendCString(suffix)
		}
		return computation
	})
	m.callbackDepth--
	if computation != nil {
		return computation
	}
	return m.cleanupDisplayReceipt(&receipt)
}

// traceUPolynomialText emits a text-only "upolynomial" record. The text is
// computed only when the tag is on, before the record is opened.
func (m *UPolynomialManager) traceUPolynomialText(function string, line int, text func() (string, error)) error {
	tags := m.config.Integer.BorrowTraceTags()
	if !tags.ShouldTrace("upolynomial") {
		return nil
	}
	s, err := text()
	if err != nil {
		return err
	}
	// Stream failures set the diagnostic failbit; they do not fail the caller.
	_ = tags.Emit("upolynomial", function, "src/math/polynomial/upolynomial.cpp", line, s)
	return nil
}
