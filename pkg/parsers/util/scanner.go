package util

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type Token uint8

const (
	LeftParen Token = iota + 1
	RightParen
	Colon
	Identifier
	String
	Comment
	Integer
	BitVector
	Float
	EndOfFile
	ScanError
)

type Parameter struct {
	Number *uint32
	Symbol string
}

type Scanner struct {
	reader         *bufio.Reader
	errors         io.Writer
	smt2, bvToken  bool
	line, position int
	id             string
	numbers        *unumeral.RationalManager // rational's mpq manager
	number         unumeral.Rational         // m_number
	numberErr      error                     // a numeral manager failure behind ScanError
	bvSize         uint32
	params         []Parameter
	state          Token
}

// NewScanner borrows numbers (the parser AST manager's
// NumeralService().Manager) for m_number; Close destroys m_number.
func NewScanner(reader io.Reader, errors io.Writer, smt2, bitVectorToken bool, numbers *unumeral.RationalManager) *Scanner {
	if errors == nil {
		errors = io.Discard
	}
	return &Scanner{reader: bufio.NewReader(reader), errors: errors, smt2: smt2, bvToken: bitVectorToken, line: 1, state: Identifier, bvSize: ^uint32(0), numbers: numbers, number: unumeral.NewRational()}
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

// NumberError is the numeral manager failure that ended the scan, if any.
func (s *Scanner) NumberError() error { return s.numberErr }

func (s *Scanner) numberFailed(err error) Token {
	s.numberErr = err
	s.state = ScanError
	return s.state
}

func (s *Scanner) Line() int     { return s.line }
func (s *Scanner) Position() int { return s.position }
func (s *Scanner) ID() string    { return s.id }

// Number is get_number(): borrowed until the next token.
func (s *Scanner) Number() *unumeral.Rational { return &s.number }
func (s *Scanner) Numbers() *unumeral.RationalManager {
	return s.numbers
}
func (s *Scanner) BitVectorSize() uint32   { return s.bvSize }
func (s *Scanner) Parameters() []Parameter { return append([]Parameter(nil), s.params...) }
func (s *Scanner) healthy() bool           { return s.state != ScanError && s.state != EndOfFile }

func (s *Scanner) read() (rune, error) {
	r, _, err := s.reader.ReadRune()
	if err == nil {
		s.position++
	}
	return r, err
}
func (s *Scanner) unread() {
	if s.reader.UnreadRune() == nil {
		s.position--
	}
}

func (s *Scanner) report(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(s.errors, format, args...)
}

func (s *Scanner) readQuotedSymbol() Token {
	var value strings.Builder
	escaped := false
	for {
		r, err := s.read()
		if err == io.EOF {
			s.report("ERROR: unexpected end of file.\n")
			s.state = EndOfFile
			return s.state
		}
		if err != nil {
			s.state = ScanError
			return s.state
		}
		if r == '\n' {
			s.line++
			s.position = 0
		}
		if r == '|' && !escaped {
			s.id = value.String()
			return Identifier
		}
		value.WriteRune(r)
		escaped = r == '\\' && !escaped
		if r != '\\' {
			escaped = false
		}
	}
}

func smt2SymbolRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("~!@$%^&*_-+=<>.?/", r)
}
func legacySymbolRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_=<>+-*/%~&@#|\\.'!?", r)
}

func (s *Scanner) readIdentifier(first rune) Token {
	s.params = nil
	if first == '|' && s.smt2 {
		return s.readQuotedSymbol()
	}
	var value strings.Builder
	value.WriteRune(first)
	isArithmetic := !s.smt2 && strings.ContainsRune("=<>+-*/%~&@#|\\", first)
	isAlpha := unicode.IsLetter(first)
	for {
		r, err := s.read()
		if err == io.EOF {
			s.id = value.String()
			return Identifier
		}
		if err != nil {
			s.state = ScanError
			return s.state
		}
		if !s.smt2 && first == '-' && value.Len() == 1 && unicode.IsDigit(r) {
			return s.readNumber(r, false)
		}
		allowed := smt2SymbolRune(r)
		if !s.smt2 {
			allowed = legacySymbolRune(r)
		}
		if r == '[' {
			s.id = value.String()
			if s.readParameters() {
				return Identifier
			}
			return s.state
		}
		if isArithmetic && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == ':' || r == '.') {
			allowed = false
		}
		if !s.smt2 && !isArithmetic && strings.ContainsRune("=<>+*/%~&@#|\\", r) {
			allowed = false
		}
		if !s.smt2 && !isArithmetic && r == '-' && !isAlpha {
			allowed = false
		}
		if !allowed {
			s.unread()
			s.id = value.String()
			return Identifier
		}
		value.WriteRune(r)
	}
}

func (s *Scanner) readParameters() bool {
	var value strings.Builder
	flush := func() bool {
		text := value.String()
		value.Reset()
		if n, err := strconv.ParseUint(text, 10, 32); err == nil && text != "" {
			v := uint32(n)
			s.params = append(s.params, Parameter{Number: &v})
		} else {
			s.params = append(s.params, Parameter{Symbol: text})
		}
		return true
	}
	for {
		r, err := s.read()
		if err != nil {
			s.report("ERROR: unexpected end of file.\n")
			s.state = ScanError
			return false
		}
		switch r {
		case ':':
			flush()
		case ']':
			flush()
			return true
		default:
			value.WriteRune(r)
		}
	}
}

func (s *Scanner) readNumber(first rune, positive bool) Token {
	if s.numbers == nil {
		return s.numberFailed(fmt.Errorf("parser: nil numeral manager"))
	}
	if first < '0' || first > '9' {
		s.state = ScanError
		return s.state
	}
	m := s.numbers
	divideBy := uint32(0)
	ten, digit := unumeral.NewRational(), unumeral.NewRational()
	result := func(t Token, err error) Token {
		err = errors.Join(err, m.Del(&ten), m.Del(&digit))
		if err != nil {
			return s.numberFailed(err)
		}
		return t
	}
	if err := errors.Join(m.SetInt32(&ten, 10), m.SetInt32(&s.number, int32(first-'0'))); err != nil {
		return result(0, err)
	}
	token := Integer
	for {
		r, err := s.read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.state = ScanError
			return result(s.state, nil)
		}
		if unicode.IsDigit(r) {
			if r > '9' {
				s.state = ScanError
				return result(s.state, nil)
			}
			// m_number = rational(10)*m_number + rational(ch - '0');
			if err = m.Mul(&ten, &s.number, &s.number); err == nil {
				if err = m.SetInt32(&digit, r-'0'); err == nil {
					err = m.Add(&s.number, &digit, &s.number)
				}
			}
			if err != nil {
				return result(0, err)
			}
			if token == Float {
				divideBy++
			}
			continue
		}
		if r == '.' {
			token = Float
			continue
		}
		s.unread()
		break
	}
	if !positive {
		if err := m.Neg(&s.number); err != nil {
			return result(0, err)
		}
	}
	if token == Float {
		// m_number /= power(rational(10), divide_by);
		if err := m.Power(&ten, divideBy, &digit); err != nil {
			return result(0, err)
		}
		if err := m.Div(&s.number, &digit, &s.number); err != nil {
			return result(0, err)
		}
	}
	return result(token, nil)
}

func (s *Scanner) readDelimited(delimiter rune, token Token) Token {
	var value strings.Builder
	for {
		r, err := s.read()
		if err == io.EOF {
			s.id = value.String()
			return token
		}
		if err != nil {
			s.state = ScanError
			return s.state
		}
		if r == '\n' {
			s.line++
			s.position = 0
		}
		if r == delimiter {
			s.id = value.String()
			return token
		}
		if r == '\\' {
			value.WriteRune(r)
			next, readErr := s.read()
			if readErr != nil {
				s.id = value.String()
				return token
			}
			value.WriteRune(next)
			continue
		}
		value.WriteRune(r)
	}
}

func (s *Scanner) readBitVector() Token {
	kind, err := s.read()
	if err != nil {
		s.state = ScanError
		return s.state
	}
	if !s.bvToken {
		prefix, valid := "bvbin", kind == 'b'
		if kind == 'x' {
			prefix, valid = "bvhex", true
		}
		if !valid {
			s.report("ERROR: unexpected character after '#': '%c'.\n", kind)
			s.state = ScanError
			return s.state
		}
		var value strings.Builder
		value.WriteString(prefix)
		for {
			r, readErr := s.read()
			if readErr != nil {
				break
			}
			validDigit := r == '0' || r == '1' || kind == 'x' && (unicode.IsDigit(r) || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F')
			if !validDigit {
				s.unread()
				break
			}
			value.WriteRune(r)
		}
		s.id = value.String()
		return Identifier
	}
	base, width := 0, uint32(0)
	if kind == 'x' {
		base = 16
	} else if kind == 'b' {
		base = 2
	} else {
		s.state = ScanError
		return s.state
	}
	if s.numbers == nil {
		return s.numberFailed(fmt.Errorf("parser: nil numeral manager"))
	}
	m := s.numbers
	radix, digitValue := unumeral.NewRational(), unumeral.NewRational()
	release := func() error { return errors.Join(m.Del(&radix), m.Del(&digitValue)) }
	// m_number = rational(0);
	if err := errors.Join(m.SetInt32(&radix, int32(base)), m.SetInt32(&s.number, 0)); err != nil {
		return s.numberFailed(errors.Join(err, release()))
	}
	for {
		r, readErr := s.read()
		if readErr != nil {
			break
		}
		digit := -1
		switch {
		case r >= '0' && r <= '9':
			digit = int(r - '0')
		case r >= 'a' && r <= 'f':
			digit = int(r-'a') + 10
		case r >= 'A' && r <= 'F':
			digit = int(r-'A') + 10
		}
		if digit < 0 || digit >= base {
			s.unread()
			break
		}
		// m_number *= rational(base); m_number += rational(digit);
		err := m.Mul(&s.number, &radix, &s.number)
		if err == nil {
			if err = m.SetInt32(&digitValue, int32(digit)); err == nil {
				err = m.Add(&s.number, &digitValue, &s.number)
			}
		}
		if err != nil {
			return s.numberFailed(errors.Join(err, release()))
		}
		if base == 16 {
			width += 4
		} else {
			width++
		}
	}
	if err := release(); err != nil {
		return s.numberFailed(err)
	}
	if width == 0 {
		s.state = ScanError
		return s.state
	}
	s.bvSize = width
	return BitVector
}

func (s *Scanner) Scan() Token {
	if !s.healthy() {
		return s.state
	}
	for {
		r, err := s.read()
		if err == io.EOF {
			s.state = EndOfFile
			return s.state
		}
		if err != nil {
			s.state = ScanError
			return s.state
		}
		switch r {
		case ' ', '\t', '\r':
			continue
		case '\n':
			s.line++
			s.position = 0
			continue
		case ';':
			for {
				c, e := s.read()
				if e != nil {
					s.state = EndOfFile
					return s.state
				}
				if c == '\n' {
					s.line++
					s.position = 0
					break
				}
			}
		case ':':
			return Colon
		case '(':
			return LeftParen
		case ')':
			return RightParen
		case '{':
			return s.readDelimited('}', Comment)
		case '"':
			return s.readDelimited('"', String)
		case '#':
			if s.smt2 {
				return s.readBitVector()
			}
			return s.readIdentifier(r)
		default:
			if unicode.IsDigit(r) {
				return s.readNumber(r, true)
			}
			if unicode.IsLetter(r) || r == '?' || r == '$' || r == '.' || r == '|' || strings.ContainsRune("~!@$%^&*_-+=<>?/\\'#", r) {
				return s.readIdentifier(r)
			}
			s.report("ERROR: unexpected character: '%c'.\n", r)
			s.state = ScanError
			return s.state
		}
	}
}
