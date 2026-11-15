package realclosure

// Source: src/math/realclosure/realclosure.cpp:5865-5897 (display_poly_sign,
// display_sign_conditions, display_algebraic_def).

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) displayRankedAlgebraicDefinition(w io.Writer, root *algebraicRoot, compact, pretty bool) error {
	if root == nil || root.kind != realExtensionAlgebraic || !root.isRegistered() || root.owner.values != s || root.isolating == nil {
		return arena.ErrInvalidSlot
	}
	if err := writeRealClosureString(w, "root("); err != nil {
		return err
	}
	if err := s.displayRankedPolynomial(w, root.polynomial.coefficients, func() error {
		return writeRealClosureString(w, "x")
	}, compact, pretty); err != nil {
		return err
	}
	if err := writeRealClosureString(w, ", "); err != nil {
		return err
	}
	if pretty {
		if err := root.isolating.DisplayPP(w); err != nil {
			return err
		}
	} else if err := root.isolating.Display(w); err != nil {
		return err
	}
	if err := writeRealClosureString(w, ", "); err != nil {
		return err
	}
	if root.signs == nil {
		if err := writeRealClosureString(w, "{}"); err != nil {
			return err
		}
	} else {
		if root.signIndex < 0 || root.signIndex >= len(root.signs.conditions) {
			return arena.ErrInvalidSlot
		}
		if err := s.displayRankedSignConditions(w, root.signs.conditions[root.signIndex], root.signs.qs, compact, pretty); err != nil {
			return err
		}
	}
	return writeRealClosureString(w, ")")
}

// displayRankedSignConditions is display_sign_conditions(out, sc, qs,
// compact, pp): it walks the antecedent chain of the sign condition.
func (s *realValueStore) displayRankedSignConditions(w io.Writer, condition *signCondition, qs []realPolynomial, compact, pretty bool) error {
	if err := writeRealClosureString(w, "{"); err != nil {
		return err
	}
	first := true
	for condition != nil {
		if first {
			first = false
		} else if err := writeRealClosureString(w, ", "); err != nil {
			return err
		}
		if condition.polynomial < 0 || condition.polynomial >= len(qs) {
			return arena.ErrInvalidSlot
		}
		if err := s.displayRankedPolynomial(w, qs[condition.polynomial].coefficients, func() error {
			return writeRealClosureString(w, "x")
		}, compact, pretty); err != nil {
			return err
		}
		sign := " > 0"
		if condition.sign < 0 {
			sign = " < 0"
		} else if condition.sign == 0 {
			sign = " = 0"
		}
		if err := writeRealClosureString(w, sign); err != nil {
			return err
		}
		condition = condition.previous
	}
	return writeRealClosureString(w, "}")
}
