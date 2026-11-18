package api

import "github.com/glycerine/g3/pkg/util/numeral"

// softRat is the small rational n/d (lowest terms, d > 0) as a borrowed
// soft-constraint weight; it holds no numeral storage.
func softRat(n, d int64) *numeral.Rational {
	return &numeral.Rational{Num: numeral.NewInt(int32(n)), Den: numeral.NewInt(int32(d))}
}
