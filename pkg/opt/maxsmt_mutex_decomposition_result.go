package opt

// Source: src/opt/maxsmt.cpp post-mutex soft reconstruction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// terms returns caller-owned copies of the positive new_soft entries.
func (d *maxSMTMutexDecomposition) terms() ([]maxSMTWeightedTerm, error) {
	entries := d.soft.Entries()
	result := make([]maxSMTWeightedTerm, 0, len(entries))
	for _, entry := range entries {
		expression := entry.Key
		weight := d.weights[expression]
		if weight == nil {
			continue
		}
		sign, err := d.sign(weight)
		if err != nil {
			return nil, errors.Join(err, releaseMaxSMTTerms(d.q, result))
		}
		if sign <= 0 {
			continue
		}
		term := maxSMTWeightedTerm{expression: expression, weight: numeral.NewRational()}
		if err = d.q.Copy(&term.weight, weight); err != nil {
			return nil, errors.Join(err, releaseMaxSMTTerms(d.q, result))
		}
		result = append(result, term)
	}
	return result, nil
}
