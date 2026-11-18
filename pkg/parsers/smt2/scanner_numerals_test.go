package smt2

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// testScannerNumerals is a standalone rational manager for scanner tests
// that run without a command context.
var testScannerNumerals = func() *unumeral.RationalManager {
	b := arena.NewBudget(1 << 30)
	s, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	if err != nil {
		panic(err)
	}
	return s.Manager
}()

// scannerNumberText is the current numeral token as rational::to_string.
func scannerNumberText(s *Scanner) string {
	text, err := s.Numbers().String(s.Number())
	if err != nil {
		return "error:" + err.Error()
	}
	return text
}
