package cmd_context

// Source: src/muz/fp/datalog_parser.cpp line_reader, char_reader and dlexer.
//
// Characters are C++ `char` values: signed, so a 0xFF input byte reads as
// EOF (-1), exactly as `m_curr_char == EOF` sees it in the C++ lexer.

import (
	"io"
	"os"
)

const dlEOF = -1

// dlChar converts an input byte to the signed char the C++ lexer stores.
func dlChar(b byte) int { return int(int8(b)) }

// datalogLineReader is line_reader: the file split at '\n'. The text after
// the last '\n' (possibly empty) is a final line, and eof() becomes true once
// that line has been handed out.
type datalogLineReader struct {
	data []byte
	next int
	eof  bool
	ok   bool
}

// newDatalogLineReader is line_reader(fname): fopen(fname, "rb"). A read
// failure after a successful open ends the data, as fread's short count does.
func newDatalogLineReader(fname string) *datalogLineReader {
	r := &datalogLineReader{}
	file, err := os.Open(fname)
	if err != nil {
		return r
	}
	r.ok = true
	r.data, _ = io.ReadAll(file)
	file.Close()
	return r
}

// getLine is line_reader::get_line. The returned line is a C string: it
// ends at its first NUL byte.
func (r *datalogLineReader) getLine() []byte {
	start := r.next
	end := start
	for end < len(r.data) && r.data[end] != '\n' {
		end++
	}
	if end == len(r.data) {
		r.eof = true
	}
	r.next = end + 1
	line := r.data[start:end]
	for i, c := range line {
		if c == 0 {
			return line[:i]
		}
	}
	return line
}

// datalogCharReader is char_reader.
type datalogCharReader struct {
	lineReader *datalogLineReader
	line       []byte
	haveLine   bool
}

func newDatalogCharReader(fname string) *datalogCharReader {
	return &datalogCharReader{lineReader: newDatalogLineReader(fname)}
}

func (r *datalogCharReader) ok() bool { return r.lineReader.ok }

func (r *datalogCharReader) get() int {
	if !r.haveLine {
		if r.lineReader.eof {
			return dlEOF
		}
		r.line, r.haveLine = r.lineReader.getLine(), true
	}
	if len(r.line) == 0 {
		r.haveLine = false
		return '\n'
	}
	result := dlChar(r.line[0])
	r.line = r.line[1:]
	return result
}

func (r *datalogCharReader) eofReached() bool { return r.lineReader.eof }

// datalogStreamInput is the std::istream read with get() (parse_string and
// the WPA rules files).
type datalogStreamInput struct {
	data []byte
	pos  int
	eof  bool
}

func (s *datalogStreamInput) get() int {
	if s.pos >= len(s.data) {
		s.eof = true
		return dlEOF
	}
	c := s.data[s.pos]
	s.pos++
	return dlChar(c)
}

// dlIsDigit is isdigit/iswdigit on a char in the C locale.
func dlIsDigit(c int) bool { return c >= '0' && c <= '9' }

// dlIsSpace is iswspace on a char in the C locale. A negative char converts
// to a wint_t outside the ASCII range, which is not a space.
func dlIsSpace(c int) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

// dlexer is the C++ dlexer.
type dlexer struct {
	input    *datalogStreamInput
	reader   *datalogCharReader
	prevChar int
	currChar int
	line     int
	buffer   []byte
	// When parsing domains, '.' is allowed in IDs, but elsewhere it is not
	// (because of the "y." in rules like "P(x,y):-x=y.").
	parsingDomains bool
}

func newDlexer() *dlexer { return &dlexer{line: 1} }

func (l *dlexer) eos() bool { return l.currChar == dlEOF }

func (l *dlexer) next() {
	l.prevChar = l.currChar
	if l.reader != nil {
		l.currChar = l.reader.get()
	} else {
		l.currChar = l.input.get()
	}
}

func (l *dlexer) saveChar(c int) { l.buffer = append(l.buffer, byte(c)) }

func (l *dlexer) saveAndNext() {
	l.buffer = append(l.buffer, byte(l.currChar))
	l.next()
}

func (l *dlexer) setStream(s *datalogStreamInput, r *datalogCharReader) {
	l.input = s
	l.reader = r
	l.next()
}

func (l *dlexer) readNum() dtoken {
	for dlIsDigit(l.currChar) {
		l.saveAndNext()
	}
	return dtkNum
}

func (l *dlexer) readID() dtoken {
	for !l.eos() && l.currChar != '(' && l.currChar != ')' &&
		l.currChar != '#' && l.currChar != ',' && (l.parsingDomains || l.currChar != '.') &&
		l.currChar != ':' && l.currChar != '=' && !dlIsSpace(l.currChar) {
		l.saveAndNext()
	}
	return string2dtoken(l.tokenData())
}

// readBid reads an id of the form '|'.*'|'.
func (l *dlexer) readBid() dtoken {
	for !l.eos() && l.currChar != '|' {
		l.saveAndNext()
	}
	if l.currChar == '|' {
		l.next()
	}
	return string2dtoken(l.tokenData())
}

func (l *dlexer) readString() dtoken {
	l.next()
	for l.currChar != '"' {
		if l.input != nil && l.input.eof {
			return dtkError
		}
		if l.reader != nil && l.reader.eofReached() {
			return dtkError
		}
		if l.currChar == '\n' {
			return dtkError
		}
		l.saveAndNext()
	}
	l.next()
	return dtkString
}

func (l *dlexer) readComment() {
	lineComment := l.prevChar == '\n' || l.prevChar == 0
	for l.currChar != '\n' && !l.eos() {
		l.next()
	}
	if lineComment && l.currChar == '\n' {
		l.line++
		l.next()
	}
}

func (l *dlexer) lookaheadNewline() bool {
	for l.currChar == ' ' {
		l.saveAndNext()
	}
	if l.currChar == '\n' {
		l.next()
		l.line++
		l.buffer = l.buffer[:0]
		return true
	}
	if l.currChar == '#' {
		l.buffer = l.buffer[:0]
		l.prevChar = 0
		l.readComment()
		return true
	}
	return false
}

func (l *dlexer) nextToken() dtoken {
	for {
		if l.eos() {
			return dtkEOS
		}
		l.buffer = l.buffer[:0]
		switch l.currChar {
		case '#': // comment
			l.readComment()
		case '\n':
			l.next()
			l.line++
			return dtkNewline
		case '\\':
			// Here we ignore a newline if it is preceded by a backslash.
			// We need to take care, since anywhere else backslash is used
			// as a regular character.
			l.next()
			l.saveChar('\\')
			if l.lookaheadNewline() {
				break
			}
			return l.readID()
		case '(':
			l.next()
			return dtkLP
		case ')':
			l.next()
			return dtkRP
		case ',':
			l.next()
			return dtkComma
		case '=':
			l.next()
			return dtkEQ
		case '!':
			l.next()
			if l.currChar == '=' {
				l.next()
				return dtkNEQ
			}
			return dtkNeg
		case ':':
			l.next()
			if l.currChar == '-' {
				l.next()
				return dtkLeftArrow
			}
			return dtkColon
		case '"':
			return l.readString()
		case '|':
			l.next()
			return l.readBid()
		default:
			if dlIsSpace(l.currChar) {
				l.next()
				break
			} else if dlIsDigit(l.currChar) {
				l.saveAndNext()
				return l.readNum()
			} else {
				old := l.currChar
				l.saveAndNext()
				if old == '-' && dlIsDigit(l.currChar) {
					return l.readNum()
				}
				return l.readID()
			}
		}
	}
}

// tokenData is get_token_data: m_buffer.c_str(), which ends at a NUL byte.
func (l *dlexer) tokenData() string {
	for i, c := range l.buffer {
		if c == 0 {
			return string(l.buffer[:i])
		}
	}
	return string(l.buffer)
}

func (l *dlexer) getLine() int { return l.line }
