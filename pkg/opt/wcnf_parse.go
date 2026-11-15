package opt

// Source: src/opt/opt_parse.cpp class wcnf (parse, parse_spec, read_clause)
// over opt_stream_buffer.

import (
	"fmt"
	"io"
	"math"
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
	in, err := newWCNFStreamBuffer(input)
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
					variable = uint32(-lit) // abs(parsed_lit)
				}
				clause.Literals = append(clause.Literals, WCNFLiteral{Variable: variable, Negative: negative})
			}
			result.Clauses = append(result.Clauses, clause)
		}
	}
	return result, nil
}

// InputParseExit is opt_stream_buffer's fatal parse error: the source writes
// Message to std::cerr and calls exit(Status).
type InputParseExit struct {
	Message string
	Status  int
}

func (e *InputParseExit) Error() string { return e.Message }

const wcnfEOF = -1

// wcnfStreamBuffer is opt_stream_buffer for the WCNF reader.
type wcnfStreamBuffer struct {
	data []byte
	pos  int
	val  int
	line uint32
}

func newWCNFStreamBuffer(input io.Reader) (*wcnfStreamBuffer, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	b := &wcnfStreamBuffer{data: data}
	b.next()
	return b, nil
}

func (b *wcnfStreamBuffer) ch() int   { return b.val }
func (b *wcnfStreamBuffer) eof() bool { return b.val == wcnfEOF }
func (b *wcnfStreamBuffer) next() {
	if b.pos >= len(b.data) {
		b.val = wcnfEOF
		return
	}
	b.val = int(b.data[b.pos])
	b.pos++
}

func (b *wcnfStreamBuffer) skipWhitespace() {
	for (b.ch() >= 9 && b.ch() <= 13) || b.ch() == 32 {
		if b.ch() == 10 {
			b.line++
		}
		b.next()
	}
}

func (b *wcnfStreamBuffer) skipSpace() {
	for b.ch() != 10 && ((b.ch() >= 9 && b.ch() <= 13) || b.ch() == 32) {
		b.next()
	}
}

func (b *wcnfStreamBuffer) skipLine() {
	for {
		if b.eof() {
			return
		}
		if b.ch() == '\n' {
			b.line++
			b.next()
			return
		}
		b.next()
	}
}

// parseToken consumes the matching prefix even when the token does not
// match as a whole.
func (b *wcnfStreamBuffer) parseToken(token string) bool {
	b.skipWhitespace()
	i := 0
	for i < len(token) && b.ch() == int(token[i]) {
		b.next()
		i++
	}
	return i == len(token)
}

func (b *wcnfStreamBuffer) parseUnsigned() uint32 {
	b.skipSpace()
	if b.ch() == '\n' {
		return math.MaxUint32
	}
	var val uint32
	for b.ch() >= '0' && b.ch() <= '9' {
		val = val*10 + uint32(b.ch()-'0')
		b.next()
	}
	return val
}

func (b *wcnfStreamBuffer) parseInt() (int32, error) {
	var val int32
	neg := false
	b.skipWhitespace()
	if b.ch() == '-' {
		neg = true
		b.next()
	} else if b.ch() == '+' {
		b.next()
	}
	if b.ch() < '0' || b.ch() > '9' {
		return 0, &InputParseExit{Message: fmt.Sprintf("(error line %d \"unexpected char: %s\" )", b.line, string([]byte{byte(b.ch())})), Status: 3}
	}
	for b.ch() >= '0' && b.ch() <= '9' {
		val = val*10 + int32(b.ch()-'0')
		b.next()
	}
	if neg {
		return -val, nil
	}
	return val, nil
}
