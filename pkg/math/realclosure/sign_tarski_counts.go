package realclosure

// Source: src/math/realclosure/realclosure.cpp:1613-1660 and 1667-1691.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// countSignsFromTarski follows count_signs_at_zeros_core. The q² Tarski
// result is read only on the source's expensive branch; queriedSquare reports
// that branch even when the initial q query equals +/-numRoots.
func countSignsFromTarski(numRoots, taqQ, taqQ2 int) (equal, positive, negative int, queriedSquare bool, err error) {
	if numRoots <= 0 || taqQ < -numRoots || taqQ > numRoots {
		return 0, 0, 0, false, arena.ErrInvalidSlot
	}
	if taqQ == numRoots {
		equal, positive, negative = 0, numRoots, 0
	} else if taqQ == -numRoots {
		equal, positive, negative = 0, 0, numRoots
	}
	if taqQ == numRoots-1 {
		equal, positive, negative = 1, numRoots-1, 0
	} else if taqQ == -(numRoots - 1) {
		equal, positive, negative = 1, 0, numRoots-1
	} else {
		queriedSquare = true
		if taqQ2 < 0 || taqQ2 > numRoots || (taqQ2+taqQ)%2 != 0 || (taqQ2-taqQ)%2 != 0 {
			return 0, 0, 0, true, arena.ErrInvalidSlot
		}
		equal = numRoots - taqQ2
		positive = (taqQ2 + taqQ) / 2
		negative = (taqQ2 - taqQ) / 2
	}
	if equal < 0 || positive < 0 || negative < 0 || equal+positive+negative != numRoots {
		return 0, 0, 0, queriedSquare, arena.ErrInvalidSlot
	}
	return equal, positive, negative, queriedSquare, nil
}

// countSignsAtZerosRational is the retained rational-coefficient
// count_signs_at_zeros producer. The second source if/else is intentionally
// independent of the all-positive/all-negative branch, so those two cases
// still construct q^2 and issue the second Tarski query.
func (s *realValueStore) countSignsAtZerosRational(p, q []*realValue, interval *BinaryInterval, numRoots int, usePrem bool) (equal, positive, negative int, queriedSquare bool, err error) {
	if s == nil || s.numbers == nil || interval == nil || numRoots <= 0 {
		return 0, 0, 0, false, arena.ErrInvalidSlot
	}
	taqQ, err := s.tarskiBinaryIntervalRational(p, q, interval, usePrem)
	if err != nil {
		return 0, 0, 0, false, err
	}
	queriedSquare = taqQ != numRoots-1 && taqQ != -(numRoots-1)
	if !queriedSquare {
		equal, positive, negative, _, err = countSignsFromTarski(numRoots, taqQ, 0)
		return equal, positive, negative, false, err
	}
	q2, err := s.multiplyRationalPolynomials(q, q)
	if err != nil {
		return 0, 0, 0, true, err
	}
	taqQ2, bodyErr := s.tarskiBinaryIntervalRational(p, q2, interval, usePrem)
	if bodyErr == nil {
		equal, positive, negative, _, bodyErr = countSignsFromTarski(numRoots, taqQ, taqQ2)
	}
	cleanupErr := s.releaseSturmCoefficients(q2)
	return equal, positive, negative, true, errors.Join(bodyErr, cleanupErr)
}

// advanceFromTarski reaches the sign-determination transition from the two
// source Tarski query results. Computing those queries from p and q remains
// part of the unfinished polynomial/root-isolation owner.
func (s *signDetermination) advanceFromTarski(q []*realValue, taqQ, taqQ2 int, expandedTarski []int) (bool, bool, error) {
	if s == nil {
		return false, false, arena.ErrInvalidSlot
	}
	equal, positive, negative, _, err := countSignsFromTarski(s.numRoots, taqQ, taqQ2)
	if err != nil {
		return false, false, err
	}
	return s.advanceRationalProducts(equal, positive, negative, q, expandedTarski)
}
