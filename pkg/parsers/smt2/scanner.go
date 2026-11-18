package smt2

// Source: src/parsers/smt2/smt2scanner.cpp and smt2scanner.h.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type Token uint8

const (
	NullToken Token = iota
	LeftParenToken
	RightParenToken
	KeywordToken
	SymbolToken
	StringToken
	IntegerToken
	BitVectorToken
	DecimalToken
	EOFToken
)

// ScannerError records the source scanner exception's message and location.
type ScannerError struct {
	Message      string
	Line, Column int
}

func (e *ScannerError) Error() string {
	return fmt.Sprintf("line %d column %d: %s", e.Line, e.Column, e.Message)
}

// Scanner implements the SMT-LIB command parser's byte-level lexer. Go buffered
// input replaces the source stream buffer; token spelling and arithmetic stay
// independent of the AST manager until the parser consumes them.
type Scanner struct {
	reader                 *bufio.Reader
	compliant              bool
	curr                   byte
	eof                    bool
	line, column, position int
	id, text               string
	numbers                *unumeral.RationalManager // rational's mpq manager
	number                 unumeral.Rational         // m_number
	bvSize                 uint32
	caching                bool
	cache                  []byte
	traceTags              *diagnostic.TraceTags
}

// NewScanner borrows numbers, the rational manager of the parser's AST
// manager (ast.ASTManager.NumeralService().Manager), for m_number.
func NewScanner(reader io.Reader, compliant bool, numbers *unumeral.RationalManager) (*Scanner, error) {
	if reader == nil {
		return nil, fmt.Errorf("nil SMT-LIB input")
	}
	if numbers == nil {
		return nil, fmt.Errorf("nil SMT-LIB numeral manager")
	}
	s := &Scanner{reader: bufio.NewReader(reader), compliant: compliant, line: 1, bvSize: ^uint32(0), numbers: numbers, number: unumeral.NewRational()}
	return s, s.next()
}

// Close is the scanner destructor: it destroys m_number.
func (s *Scanner) Close() error {
	if s == nil || s.numbers == nil {
		return nil
	}
	err := s.numbers.Del(&s.number)
	s.numbers = nil
	return err
}

func (s *Scanner) Line() int      { return s.line }
func (s *Scanner) Position() int  { return s.position }
func (s *Scanner) ID() string     { return s.id }
func (s *Scanner) String() string { return s.text }

// Number is get_number(): the current numeral token's value, borrowed
// until the next token; callers copy it with Numbers().Copy.
func (s *Scanner) Number() *unumeral.Rational         { return &s.number }
func (s *Scanner) Numbers() *unumeral.RationalManager { return s.numbers }
func (s *Scanner) BitVectorSize() uint32              { return s.bvSize }
func (s *Scanner) StartCaching()                      { s.caching = true; s.cache = nil }
func (s *Scanner) StopCaching()                       { s.caching = false }
func (s *Scanner) ResetCache()                        { s.cache = nil }
func (s *Scanner) CacheSize() int                     { return len(s.cache) }

func scannerSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\v' || c == '\f'
}

func (s *Scanner) CachedString(begin, end int) (string, error) {
	if begin < 0 || end < begin || end > len(s.cache) {
		return "", fmt.Errorf("invalid SMT-LIB cache range")
	}
	for begin < end && scannerSpace(s.cache[begin]) {
		begin++
	}
	for begin < end && scannerSpace(s.cache[end-1]) {
		end--
	}
	return string(s.cache[begin:end]), nil
}

func (s *Scanner) next() error {
	if s.eof {
		return &ScannerError{"unexpected end of input", s.line, s.column}
	}
	if s.caching {
		s.cache = append(s.cache, s.curr)
	}
	c, err := s.reader.ReadByte()
	s.column++
	if err == io.EOF {
		s.eof = true
		return nil
	}
	if err != nil {
		return err
	}
	s.curr = c
	return nil
}

func scannerSymbol(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("~!@$%^&*_-+=<>.?/", c) >= 0
}

func (s *Scanner) readSymbol(prefix string) (Token, error) {
	var text strings.Builder
	text.WriteString(prefix)
	for !s.eof && scannerSymbol(s.curr) {
		text.WriteByte(s.curr)
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	// The source returns EOF rather than publishing an unterminated final
	// symbol. Complete SMT-LIB commands supply the closing delimiter.
	if s.eof {
		return EOFToken, nil
	}
	s.id = text.String()
	return SymbolToken, s.trace("read_symbol_core", 105, "new symbol: "+s.id+"\n")
}

func (s *Scanner) readQuotedSymbol() (Token, error) {
	if err := s.next(); err != nil {
		return NullToken, err
	}
	var text strings.Builder
	escape := false
	for !s.eof {
		c := s.curr
		if c == '\n' {
			s.line++
			s.column = 0
		}
		if c == '|' && !escape {
			if err := s.next(); err != nil {
				return NullToken, err
			}
			s.id = text.String()
			return SymbolToken, s.trace("read_quoted_symbol", 85, "new quoted symbol: "+s.id+"\n")
		}
		escape = c == '\\'
		text.WriteByte(c)
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	return NullToken, &ScannerError{"unexpected end of quoted symbol", s.line, s.column}
}

func (s *Scanner) readNumber(negative bool) (result Token, resultErr error) {
	m := s.numbers
	// rational q(1); m_number = rational(curr() - '0'); next();
	q, ten, digit := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, m.Del(&q), m.Del(&ten), m.Del(&digit)) }()
	if err := m.SetInt32(&q, 1); err != nil {
		return NullToken, err
	}
	if err := m.SetInt32(&ten, 10); err != nil {
		return NullToken, err
	}
	if err := m.SetInt32(&s.number, int32(s.curr-'0')); err != nil {
		return NullToken, err
	}
	if err := s.next(); err != nil {
		return NullToken, err
	}
	decimal := false
	for !s.eof {
		c := s.curr
		if c >= '0' && c <= '9' {
			// m_number = rational(10)*m_number + rational(c - '0');
			if err := m.Mul(&ten, &s.number, &s.number); err != nil {
				return NullToken, err
			}
			if err := m.SetInt32(&digit, int32(c-'0')); err != nil {
				return NullToken, err
			}
			if err := m.Add(&s.number, &digit, &s.number); err != nil {
				return NullToken, err
			}
			if decimal {
				if err := m.Mul(&q, &ten, &q); err != nil {
					return NullToken, err
				}
			}
		} else if c == '.' && !decimal {
			decimal = true
		} else {
			break
		}
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	if decimal {
		if err := m.Div(&s.number, &q, &s.number); err != nil {
			return NullToken, err
		}
	}
	text, err := m.String(&s.number)
	if err != nil {
		return NullToken, err
	}
	if err := s.trace("read_number", 147, "new number: "+text+"\n"); err != nil {
		return NullToken, err
	}
	if negative {
		if err := m.Neg(&s.number); err != nil {
			return NullToken, err
		}
	}
	if decimal {
		return DecimalToken, nil
	}
	return IntegerToken, nil
}

func (s *Scanner) readString() (Token, error) {
	if err := s.next(); err != nil {
		return NullToken, err
	}
	var text strings.Builder
	for !s.eof {
		c := s.curr
		if c == '\n' {
			s.line++
			s.column = 0
		}
		if c == '"' {
			if err := s.next(); err != nil {
				return NullToken, err
			}
			if s.eof || s.curr != '"' {
				s.text = text.String()
				return StringToken, nil
			}
		}
		text.WriteByte(c)
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	return NullToken, &ScannerError{"unexpected end of string", s.line, s.column}
}

func scannerDigit(c byte, base int64) int64 {
	v := int64(-1)
	if c >= '0' && c <= '9' {
		v = int64(c - '0')
	}
	if c >= 'a' && c <= 'f' {
		v = int64(c-'a') + 10
	}
	if c >= 'A' && c <= 'F' {
		v = int64(c-'A') + 10
	}
	if v >= base {
		return -1
	}
	return v
}

func (s *Scanner) readBitVector() (result Token, resultErr error) {
	if err := s.next(); err != nil {
		return NullToken, err
	}
	base, width := int64(0), uint32(0)
	if !s.eof && s.curr == 'x' {
		base, width = 16, 4
	}
	if !s.eof && s.curr == 'b' {
		base, width = 2, 1
	}
	if base == 0 {
		return NullToken, &ScannerError{"invalid bit-vector literal, expecting 'x' or 'b'", s.line, s.column}
	}
	if err := s.next(); err != nil {
		return NullToken, err
	}
	m := s.numbers
	radix, digitValue := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, m.Del(&radix), m.Del(&digitValue)) }()
	if err := m.SetInt32(&radix, int32(base)); err != nil {
		return NullToken, err
	}
	// m_number = rational(0);
	if err := m.SetInt32(&s.number, 0); err != nil {
		return NullToken, err
	}
	s.bvSize = 0
	for !s.eof {
		digit := scannerDigit(s.curr, base)
		if digit < 0 {
			break
		}
		// m_number *= rational(16); m_number += rational(digit);
		if err := m.Mul(&s.number, &radix, &s.number); err != nil {
			return NullToken, err
		}
		if err := m.SetInt32(&digitValue, int32(digit)); err != nil {
			return NullToken, err
		}
		if err := m.Add(&s.number, &digitValue, &s.number); err != nil {
			return NullToken, err
		}
		s.bvSize += width
		if err := s.next(); err != nil {
			return NullToken, err
		}
	}
	if s.bvSize == 0 {
		return NullToken, &ScannerError{"invalid empty bit-vector literal", s.line, s.column}
	}
	return BitVectorToken, nil
}

func (s *Scanner) trace(function string, line int, text string) error {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	return tags.Emit("scanner", function, "src/parsers/smt2/smt2scanner.cpp", line, text)
}

func (s *Scanner) Scan() (Token, error) {
	for !s.eof {
		s.position = s.column
		c := s.curr
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			if err := s.next(); err != nil {
				return NullToken, err
			}
		case c == '\n':
			if err := s.next(); err != nil {
				return NullToken, err
			}
			s.line++
			s.column = 0
		case c == ';':
			for !s.eof && s.curr != '\n' {
				if err := s.next(); err != nil {
					return NullToken, err
				}
			}
			if !s.eof {
				s.line++
				s.column = 0
				if err := s.next(); err != nil {
					return NullToken, err
				}
			}
		case c == '(' || c == ')':
			if err := s.next(); err != nil {
				return NullToken, err
			}
			if c == '(' {
				return LeftParenToken, nil
			}
			return RightParenToken, nil
		case c == ':':
			if err := s.next(); err != nil {
				return NullToken, err
			}
			_, err := s.readSymbol(":")
			return KeywordToken, err
		case c == '|':
			return s.readQuotedSymbol()
		case c == '"':
			return s.readString()
		case c == '#':
			return s.readBitVector()
		case c >= '0' && c <= '9':
			return s.readNumber(false)
		case c == '-' && !s.compliant:
			if err := s.next(); err != nil {
				return NullToken, err
			}
			if !s.eof && s.curr >= '0' && s.curr <= '9' {
				return s.readNumber(true)
			}
			return s.readSymbol("-")
		case scannerSymbol(c):
			return s.readSymbol("")
		default:
			err := &ScannerError{"unexpected character", s.line, s.column}
			if next := s.next(); next != nil {
				return NullToken, next
			}
			return NullToken, err
		}
	}
	s.position = s.column
	return EOFToken, nil
}
