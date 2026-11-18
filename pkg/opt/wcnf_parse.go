package opt

// Source: src/opt/opt_parse.cpp class wcnf (parse, parse_spec, read_clause)
// over opt_stream_buffer.

import (
	"fmt"
	"io"
)

// ParseWCNF is wcnf::parse: a token stream, not a line format. The p line is
// optional (max_weight stays 0, so every clause is hard), a missing header
// field reads as UINT_MAX, clauses may span lines, and literals are not
// checked against the declared counts. Weights are 32-bit unsigned with
// wraparound. A malformed literal is parse_int's fatal error, returned as
// an *InputParseExit.
func ParseWCNF(input io.Reader) (WCNFDocument, error) {
	if input == nil {
		return WCNFDocument{}, fmt.Errorf("wcnf: nil input")
	}
	in, err := newOptStreamBuffer(input)
	if err != nil {
		return WCNFDocument{}, err
	}
	var result WCNFDocument
	var maxWeight uint32
	for {
		in.skipWhitespace()
		if in.eof() {
			break
		}
		if in.ch() == 'c' {
			in.skipLine()
		} else if in.ch() == 'p' {
			in.next()
			// parse_spec
			in.parseToken("wcnf")
			result.NumVariables = in.parseUnsigned()
			result.NumClauses = in.parseUnsigned()
			maxWeight = in.parseUnsigned()
			result.TopWeight = uint64(maxWeight)
		} else {
			// read_clause
			clause := WCNFClause{Weight: uint64(in.parseUnsigned())}
			for {
				lit, err := in.parseInt()
				if err != nil {
					return WCNFDocument{}, err
				}
				if lit == 0 {
					break
				}
				negative := lit < 0
				variable := uint32(lit)
				if negative {
					variable = uint32(-lit) // FORK FIX #134: the magnitude in unsigned
				}
				clause.Literals = append(clause.Literals, WCNFLiteral{Variable: variable, Negative: negative})
			}
			result.Clauses = append(result.Clauses, clause)
		}
	}
	return result, nil
}
