package opt

// Source: src/opt/maxsmt.cpp post-mutex soft reconstruction.

import "math/big"

func (d *maxSMTMutexDecomposition) terms() []maxSMTWeightedTerm {
	entries := d.soft.Entries()
	result := make([]maxSMTWeightedTerm, 0, len(entries))
	for _, entry := range entries {
		expression := entry.Key
		weight := d.weights[expression]
		if weight == nil || weight.Sign() <= 0 {
			continue
		}
		result = append(result, maxSMTWeightedTerm{expression: expression, weight: *new(big.Int).Set(weight)})
	}
	return result
}

func (d *maxSMTMutexDecomposition) lowerOffset() *big.Int {
	return new(big.Int).Set(&d.offset)
}
