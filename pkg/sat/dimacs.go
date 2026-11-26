package sat

// Source: src/sat/dimacs.h; src/sat/dimacs.cpp.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

var ErrDIMACSUnexpectedChar = errors.New("sat: DIMACS unexpected character")

// DIMACSParseExit is parse_int's malformed-input diagnostic: the source
// prints Error() to stderr and calls exit(3).
type DIMACSParseExit struct{ Char int }

func (e *DIMACSParseExit) Error() string {
	return fmt.Sprintf("(error, \"unexpected char: %d\")", e.Char)
}
func (e *DIMACSParseExit) Unwrap() error { return ErrDIMACSUnexpectedChar }

// dimacsStreamBuffer is the source one-character lookahead buffer. Any stream
// failure has the same observable effect as std::istream::get returning EOF.
type dimacsStreamBuffer struct {
	input     *bufio.Reader
	value     int
	window    []byte
	borrowed  int
	shared    bool
	knownVars uint32
}

func newDIMACSStreamBuffer(input io.Reader) *dimacsStreamBuffer {
	b := &dimacsStreamBuffer{input: bufio.NewReader(input)}
	if r, ok := input.(*bufio.Reader); ok {
		b.shared = r == b.input
	}
	b.advance()
	return b
}

func (b *dimacsStreamBuffer) advance() {
	if len(b.window) == 0 && !b.refill() {
		b.value = -1
		return
	}
	b.value = int(b.window[0])
	b.window = b.window[1:]
}

func (b *dimacsStreamBuffer) commitWindow() {
	consumed := b.borrowed - len(b.window)
	if b.shared && consumed != 0 {
		// Preserve UnreadByte on the caller's reader, including at EOF.
		_, _ = b.input.Discard(consumed - 1)
		_, _ = b.input.ReadByte()
	} else if consumed != 0 {
		_, _ = b.input.Discard(consumed)
	}
	b.window = nil
	b.borrowed = 0
}

// A solver callback or the caller can observe a shared reader's position.
func (b *dimacsStreamBuffer) syncShared() {
	if b.shared {
		b.commitWindow()
	}
}

func (b *dimacsStreamBuffer) refill() bool {
	b.commitWindow()
	if b.input.Buffered() == 0 {
		// The underlying Read can call back into the solver. Use ReadByte
		// to preserve the reader's last successful byte on an input error.
		b.knownVars = 0
		if _, err := b.input.ReadByte(); err != nil {
			return false
		}
		_ = b.input.UnreadByte()
	}
	b.borrowed = b.input.Buffered()
	b.window, _ = b.input.Peek(b.borrowed)
	return true
}

func (b *dimacsStreamBuffer) skipWhitespace() {
	for (b.value >= 9 && b.value <= 13) || b.value == 32 {
		b.advance()
	}
}

func (b *dimacsStreamBuffer) skipLine() {
	for b.value != -1 {
		if b.value == '\n' {
			b.advance()
			return
		}
		b.advance()
	}
}

func (b *dimacsStreamBuffer) parseInt() (int32, error) {
	b.skipWhitespace()
	negative := false
	switch b.value {
	case '-':
		negative = true
		b.advance()
	case '+':
		b.advance()
	}
	if b.value < '0' || b.value > '9' {
		return 0, &DIMACSParseExit{Char: b.value}
	}
	// The source accumulates in a 32-bit int; its signed overflow is UB.
	// FORK FIX #125 accumulates in unsigned (the wraparound the release
	// build shows), and g3 does the same.
	value := uint32(b.value - '0')
digits:
	for {
		for i, ch := range b.window {
			digit := ch - '0'
			if digit > 9 {
				b.value = int(ch)
				b.window = b.window[i+1:]
				break digits
			}
			value = value*10 + uint32(digit)
		}
		b.window = nil
		if !b.refill() {
			b.value = -1
			break
		}
	}
	if negative {
		value = -value
	}
	return int32(value), nil
}

func (b *dimacsStreamBuffer) readClause(solver *Solver, literals []Literal) ([]Literal, error) {
	literals = literals[:0]
	b.knownVars = 0
	for {
		parsed, err := b.parseInt()
		if err != nil {
			return literals, err
		}
		if parsed == 0 {
			return literals, nil
		}
		// abs(parsed_lit) as an unsigned value: FORK FIX #125 (abs of
		// INT_MIN is UB in the source).
		variable := uint32(parsed)
		if parsed < 0 {
			variable = -variable
		}
		// The source deliberately uses the absolute DIMACS number as its SAT
		// variable, leaving variable zero unused.
		if variable >= b.knownVars {
			n, err := solver.sourceNumVars()
			if err != nil {
				return literals, err
			}
			for variable >= n {
				b.syncShared()
				if _, err = solver.MkVar(false, true); err != nil {
					return literals, err
				}
				n++
			}
			b.knownVars = n
		}
		literals = append(literals, MakeLiteral(BoolVar(variable), parsed < 0))
	}
}

// ParseDIMACS appends the clauses read from input to solver. Header and comment
// lines are informational, as in the source parser. A parse or solver error
// leaves variables and clauses already published before the error intact.
//
// The C++ command-line parser prints malformed-input diagnostics and exits the
// process. The Go library reports the same offending character to its caller.
func ParseDIMACS(input io.Reader, solver *Solver) error {
	if input == nil || solver == nil {
		return ErrSATOwner
	}
	if err := solver.open(); err != nil {
		return err
	}
	buffer := newDIMACSStreamBuffer(input)
	defer buffer.syncShared()
	literals := make([]Literal, 0)
	for buffer.nextClause() {
		var err error
		literals, err = buffer.readClause(solver, literals)
		if err != nil {
			return err
		}
		buffer.syncShared()
		if err = solver.MkClause(literals); err != nil {
			return err
		}
	}
	return nil
}

// nextClause consumes separators and header/comment lines before a clause.
func (b *dimacsStreamBuffer) nextClause() bool {
	for {
		b.skipWhitespace()
		switch b.value {
		case -1:
			return false
		case 'c', 'p':
			b.skipLine()
		default:
			return true
		}
	}
}
