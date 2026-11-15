package realclosure

// Source: src/math/realclosure/realclosure.cpp:5965-5981 and manager display
// wrappers at 6241-6254.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func writeRealClosureString(w io.Writer, text string) error {
	if w == nil {
		return arena.ErrInvalidSlot
	}
	_, err := io.WriteString(w, text)
	return err
}

// displayValue is manager::imp::display(out, numeral, compact, pp): the
// compact form lists the algebraic extensions the value references.
func (s *realValueStore) displayValue(w io.Writer, value *realValue, compact, pretty bool) error {
	if w == nil || s == nil {
		return arena.ErrInvalidSlot
	}
	if compact {
		return s.displayCompactRankedValue(w, value, pretty)
	}
	return s.displayRankedExpression(w, value, false, pretty)
}

func (s *realValueStore) displayValueInterval(w io.Writer, value *realValue, bounds *numeral.BinaryRationalManager) error {
	if value == nil {
		return writeRealClosureString(w, "[0, 0]")
	}
	interval, err := s.valueInterval(value, bounds)
	if err != nil {
		return err
	}
	return interval.Display(w)
}
