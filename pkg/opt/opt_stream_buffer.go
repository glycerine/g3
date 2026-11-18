package opt

// Source: src/opt/opt_parse.cpp class opt_stream_buffer.

import (
	"fmt"
	"io"
	"math"
)

// InputParseExit is opt_stream_buffer's fatal parse error: the source writes
// Message to std::cerr and calls exit(Status).
type InputParseExit struct {
	Message string
	Status  int
}

func (e *InputParseExit) Error() string { return e.Message }

const optStreamEOF = -1

// optStreamBuffer is opt_stream_buffer, shared by the WCNF and OPB readers.
type optStreamBuffer struct {
	data []byte
	pos  int
	val  int
	line uint32
}

func newOptStreamBuffer(input io.Reader) (*optStreamBuffer, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	b := &optStreamBuffer{data: data}
	b.next()
	return b, nil
}

func (b *optStreamBuffer) ch() int   { return b.val }
func (b *optStreamBuffer) eof() bool { return b.val == optStreamEOF }
func (b *optStreamBuffer) next() {
	if b.pos >= len(b.data) {
		b.val = optStreamEOF
		return
	}
	b.val = int(b.data[b.pos])
	b.pos++
}

func (b *optStreamBuffer) skipWhitespace() {
	for (b.ch() >= 9 && b.ch() <= 13) || b.ch() == 32 {
		if b.ch() == 10 {
			b.line++
		}
		b.next()
	}
}

func (b *optStreamBuffer) skipSpace() {
	for b.ch() != 10 && ((b.ch() >= 9 && b.ch() <= 13) || b.ch() == 32) {
		b.next()
	}
}

func (b *optStreamBuffer) skipLine() {
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
func (b *optStreamBuffer) parseToken(token string) bool {
	b.skipWhitespace()
	i := 0
	for i < len(token) && b.ch() == int(token[i]) {
		b.next()
		i++
	}
	return i == len(token)
}

func (b *optStreamBuffer) parseUnsigned() uint32 {
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

func (b *optStreamBuffer) parseInt() (int32, error) {
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
	// FORK FIX #134: the source accumulates in unsigned (4.7.1 used a
	// signed int, undefined past INT_MAX), so the value wraps modulo 2^32
	// like Go's int32 arithmetic.
	for b.ch() >= '0' && b.ch() <= '9' {
		val = val*10 + int32(b.ch()-'0')
		b.next()
	}
	if neg {
		return -val, nil
	}
	return val, nil
}
