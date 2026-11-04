package realclosure

// Source: src/math/realclosure/realclosure.cpp:5849-5863, 5912-5942.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) displayRankedExpression(w io.Writer, value *realValue, compact, pretty bool) error {
	if w == nil || s == nil {
		return arena.ErrInvalidSlot
	}
	if value == nil {
		return writeRealClosureString(w, "0")
	}
	if value.store != s || value.deleted || value.refs == 0 {
		return arena.ErrInvalidSlot
	}
	if value.isRational {
		return s.numbers.DisplayTo(w, &value.rational)
	}
	return s.displayRankedPolynomial(w, value.functionNumerator.coefficients, func() error {
		if value.root.kind != realExtensionAlgebraic {
			return writeRealClosureString(w, rankedRootName(value.root, pretty))
		}
		if compact {
			return writeRealClosureString(w, rankedRootName(value.root, pretty))
		}
		return s.displayRankedAlgebraicDefinition(w, value.root, compact, pretty)
	}, compact, pretty)
}
