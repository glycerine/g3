package api

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
)

type z3ReplayScanner struct {
	reader  *bufio.Reader
	current byte
	line    uint32
	eof     bool
}

func newZ3ReplayScanner(r *bufio.Reader) *z3ReplayScanner {
	s := &z3ReplayScanner{reader: r, line: 1}
	s.next()
	return s
}

func (s *z3ReplayScanner) next() {
	b, err := s.reader.ReadByte()
	if err != nil {
		s.eof = true
		return
	}
	s.current = b
}

func (s *z3ReplayScanner) blanks() {
	for !s.eof {
		if s.current == '\n' {
			s.line++
			s.next()
		} else if s.current == ' ' || s.current == '\t' {
			// skip_blank: '\r' is not blank, so a CRLF log stops at an
			// unknown log command.
			s.next()
		} else {
			return
		}
	}
}

func (s *z3ReplayScanner) quoted(delimiter byte) (string, error) {
	if s.eof || s.current != delimiter {
		return "", fmt.Errorf("invalid string/symbol")
	}
	s.next()
	value := make([]byte, 0, 32)
	for !s.eof {
		c := s.current
		if c == '\n' {
			return "", fmt.Errorf("unexpected end of line")
		}
		if c == delimiter {
			s.next()
			return string(value), nil
		}
		if c == '\\' {
			s.next()
			v := 0
			for i := 0; i < 3; i++ {
				if s.eof || s.current < '0' || s.current > '9' {
					return "", fmt.Errorf("invalid scaped character")
				}
				v = 10*v + int(s.current-'0')
				if v > 255 {
					return "", fmt.Errorf("invalid scaped character")
				}
				s.next()
			}
			value = append(value, byte(v))
			continue
		}
		value = append(value, c)
		s.next()
	}
	return "", fmt.Errorf("unexpected end of file")
}

// uint64 is read_uint64: "invalid unsigned" without a digit, and the value
// accumulates modulo 2^64 as the source's uint64_t does.
func (s *z3ReplayScanner) uint64() (uint64, error) {
	if s.eof || s.current < '0' || s.current > '9' {
		return 0, fmt.Errorf("invalid unsigned")
	}
	var v uint64
	for !s.eof && s.current >= '0' && s.current <= '9' {
		v = 10*v + uint64(s.current-'0')
		s.next()
	}
	return v, nil
}

// int64 is read_int64. The source accumulates in int64_t, so a value past
// INT64_MAX is signed overflow (undefined behavior); the accumulation here
// wraps modulo 2^64, which is what the release build computes.
func (s *z3ReplayScanner) int64() (int64, error) {
	if s.eof || !(s.current == '-' || s.current >= '0' && s.current <= '9') {
		return 0, fmt.Errorf("invalid integer")
	}
	negative := false
	if s.current == '-' {
		negative = true
		s.next()
		if s.eof || s.current < '0' || s.current > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
	}
	var v uint64
	for !s.eof && s.current >= '0' && s.current <= '9' {
		v = 10*v + uint64(s.current-'0')
		s.next()
	}
	if negative {
		v = -v
	}
	return int64(v), nil
}

// hexadecimal is read_ptr: the first character must be a hex digit, an x
// or X is skipped only at position 1, and the value accumulates modulo
// 2^64.
func (s *z3ReplayScanner) hexadecimal() (uint64, error) {
	isHex := func(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
	if s.eof || !isHex(s.current) {
		return 0, fmt.Errorf("invalid ptr")
	}
	var v uint64
	for pos := 0; !s.eof; pos++ {
		c := s.current
		switch {
		case c >= '0' && c <= '9':
			v = v*16 + uint64(c-'0')
		case c >= 'a' && c <= 'f':
			v = v*16 + 10 + uint64(c-'a')
		case c >= 'A' && c <= 'F':
			v = v*16 + 10 + uint64(c-'A')
		case pos == 1 && (c == 'x' || c == 'X'):
		default:
			return v, nil
		}
		s.next()
	}
	return v, nil
}

// floating is read_float / read_double: the double characters are
// collected and strtof/strtod read their longest numeric prefix (0 when
// there is none); the source ignores the end pointer.
func (s *z3ReplayScanner) floating(bits int) (float64, error) {
	text := make([]byte, 0, 24)
	for !s.eof {
		c := s.current
		if c == '-' || c == '.' || c == 'e' || c == 'E' || (c >= '0' && c <= '9') {
			text = append(text, c)
			s.next()
		} else {
			break
		}
	}
	if len(text) == 0 {
		if bits == 32 {
			return 0, fmt.Errorf("invalid float")
		}
		return 0, fmt.Errorf("invalid double")
	}
	for n := len(text); n > 0; n-- {
		v, err := strconv.ParseFloat(string(text[:n]), bits)
		if err == nil || errors.Is(err, strconv.ErrRange) {
			return v, nil
		}
	}
	return 0, nil
}
