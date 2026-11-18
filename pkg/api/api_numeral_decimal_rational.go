package api

// Source: src/api/api_numeral.cpp:241-246; src/util/mpq.cpp display_decimal.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
)

// apiRationalDecimalString is val.display_decimal(buffer, precision) for a
// rational of q; an integer keeps its exact spelling.
func apiRationalDecimalString(q *numeral.RationalManager, value *numeral.Rational, precision uint32) (string, error) {
	if value.IsInt() {
		return q.String(value)
	}
	return q.DecimalString(value, precision, false)
}
