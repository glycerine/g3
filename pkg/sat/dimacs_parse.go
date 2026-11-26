package sat

// Source: src/sat/dimacs.cpp:64-127 (parse_dimacs_core, read_clause,
// parse_int), reading clauses for Z3_solver_from_file's .dimacs path.

import (
	"errors"
	"io"
)

// ReadDIMACSCNF scans input with the source's one-character lookahead
// (AUDIT.md sat_01-F6): a number ends at the first non-digit, so "1-2 0" is
// the clause {1, -2}, and values wrap at 32 bits as FORK FIX #125 does. A
// malformed character is reported as a DIMACSParseExit joined with
// ErrDIMACSParse.
func ReadDIMACSCNF(input io.Reader) (DIMACSCNF, error) {
	if input == nil {
		return DIMACSCNF{}, errors.Join(ErrDIMACSParse, io.ErrUnexpectedEOF)
	}
	buffer := newDIMACSStreamBuffer(input)
	defer buffer.syncShared()
	var result DIMACSCNF
	for buffer.nextClause() {
		var clause []DIMACSLiteral
		for {
			parsed, err := buffer.parseInt()
			if err != nil {
				return DIMACSCNF{}, errors.Join(ErrDIMACSParse, err)
			}
			if parsed == 0 {
				break
			}
			magnitude := uint32(parsed)
			if parsed < 0 {
				magnitude = -magnitude
			}
			if magnitude > result.VariableCount {
				result.VariableCount = magnitude
			}
			clause = append(clause, DIMACSLiteral(parsed))
		}
		result.Clauses = append(result.Clauses, clause)
	}
	return result, nil
}
