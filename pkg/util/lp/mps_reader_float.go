package lp

// Source: src/util/lp/mps_reader.h:41-798 (floating primal/dual consumers).

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/glycerine/insdict"
)

var ErrMPSStalled = errors.New("lp: source MPS row-header loop cannot advance")
var ErrMPSUnhandled = errors.New("lp: source MPS unhandled row or bound type")

type mpsRowType uint32

const (
	mpsCost mpsRowType = iota
	mpsLess
	mpsGreater
	mpsEqual
)

type floatMPSBound struct {
	low, upper, fixed                float64
	lowSet, upperSet, fixedSet, free bool
}
type floatMPSColumn struct {
	name  string
	index uint32
	bound *floatMPSBound
}
type floatMPSRow struct {
	name         string
	index        uint32
	typ          mpsRowType
	rhs, span    float64
	coefficients *insdict.Dict[string, float64]
}

// FloatMPSReader uses native Go text and insertion-ordered dictionaries. Input
// order chooses a deterministic iteration order where C++ used unordered maps,
// except m_columns, whose libstdc++ order decides the model report order
// (floatMPSColumnMap, AUDIT.md util_lp_06-F1). Numeric tokens follow
// numeric_traits<double>::from_string, libc atof (mpsNumber, AUDIT.md
// util_lp_04-F1).
type FloatMPSReader struct {
	input                 *bufio.Reader
	file                  *os.File
	rows                  *insdict.Dict[string, *floatMPSRow]
	columns               *floatMPSColumnMap
	line, Name, costName  string
	LineNumber, costLines uint32
	ok                    bool
	Messages              io.Writer
}

func NewFloatMPSReader(in io.Reader) *FloatMPSReader {
	m := &FloatMPSReader{ok: true, Messages: os.Stdout, rows: insdict.NewDict[string, *floatMPSRow](), columns: newFloatMPSColumnMap()}
	if in != nil {
		m.input = bufio.NewReader(in)
	}
	return m
}

// OpenFloatMPSReader retains the source delayed open failure: IsOK is true
// until Read observes the absent stream and emits the source diagnostic.
func OpenFloatMPSReader(name string) *FloatMPSReader {
	f, err := os.Open(name)
	if err != nil {
		return NewFloatMPSReader(nil)
	}
	m := NewFloatMPSReader(f)
	m.file = f
	return m
}
func (m *FloatMPSReader) Close() error {
	if m.file == nil {
		return nil
	}
	f := m.file
	m.file = nil
	m.input = nil
	return f.Close()
}
func (m *FloatMPSReader) IsOK() bool { return m.ok }
func (m *FloatMPSReader) SetMessageStream(out io.Writer) error {
	if out == nil {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Messages = out
	return nil
}
func (m *FloatMPSReader) ColumnNames() []string {
	v := make([]string, 0, m.columns.Len())
	for name := range m.columns.All() {
		v = append(v, name)
	}
	return v
}
func (m *FloatMPSReader) message(format string, args ...any) error {
	if m.Messages == nil {
		return ErrBounds
	}
	_, err := fmt.Fprintf(m.Messages, format, args...)
	return err
}
func (m *FloatMPSReader) fail() error {
	if err := m.message("setting m_is_OK to false\n"); err != nil {
		return err
	}
	m.ok = false
	return nil
}
func mpsWords(s string) []string {
	return strings.FieldsFunc(s, func(c rune) bool { return c == ' ' || c == '\t' })
}

// mpsNumber is numeric_traits<double>::from_string, libc atof: leading C
// isspace bytes are skipped, the longest strtod prefix is converted, and the
// rest is ignored. It never fails; text without a number gives 0.
func mpsNumber(s string) (float64, error) {
	i := 0
	for i < len(s) && mpsIsCSpace(s[i]) {
		i++
	}
	text := mpsStrtodPrefix(s[i:])
	if text == "" {
		return 0, nil
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		// The prefix follows the strtod grammar, which ParseFloat accepts.
		return 0, nil
	}
	return n, nil
}

// mpsIsCSpace is C isspace in the "C" locale.
func mpsIsCSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

func mpsIsDigit(c byte) bool { return '0' <= c && c <= '9' }

func mpsIsHexDigit(c byte) bool {
	return mpsIsDigit(c) || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func mpsHasFoldPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// mpsStrtodPrefix returns the longest prefix of s that strtod converts, in
// the spelling strconv.ParseFloat reads ("" when strtod converts nothing):
// a NaN payload is dropped and a hex mantissa without exponent gets "p0".
func mpsStrtodPrefix(s string) string {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	rest := s[i:]
	switch {
	case mpsHasFoldPrefix(rest, "infinity"):
		return s[:i+len("infinity")]
	case mpsHasFoldPrefix(rest, "inf"):
		return s[:i+len("inf")]
	case mpsHasFoldPrefix(rest, "nan"):
		j := i + len("nan")
		if j < len(s) && s[j] == '(' {
			k := j + 1
			for k < len(s) && (mpsIsDigit(s[k]) || ('a' <= s[k] && s[k] <= 'z') || ('A' <= s[k] && s[k] <= 'Z') || s[k] == '_') {
				k++
			}
			if k < len(s) && s[k] == ')' {
				return s[:j]
			}
		}
		return s[:j]
	}
	if len(rest) >= 2 && rest[0] == '0' && (rest[1] == 'x' || rest[1] == 'X') {
		j, digits := i+2, 0
		for j < len(s) && mpsIsHexDigit(s[j]) {
			j, digits = j+1, digits+1
		}
		if j < len(s) && s[j] == '.' {
			j++
			for j < len(s) && mpsIsHexDigit(s[j]) {
				j, digits = j+1, digits+1
			}
		}
		if digits == 0 {
			// Only the "0" converts.
			return s[:i+1]
		}
		if j < len(s) && (s[j] == 'p' || s[j] == 'P') {
			k := j + 1
			if k < len(s) && (s[k] == '+' || s[k] == '-') {
				k++
			}
			if k < len(s) && mpsIsDigit(s[k]) {
				for k < len(s) && mpsIsDigit(s[k]) {
					k++
				}
				return s[:k]
			}
		}
		return s[:j] + "p0"
	}
	j, digits := i, 0
	for j < len(s) && mpsIsDigit(s[j]) {
		j, digits = j+1, digits+1
	}
	if j < len(s) && s[j] == '.' {
		j++
		for j < len(s) && mpsIsDigit(s[j]) {
			j, digits = j+1, digits+1
		}
	}
	if digits == 0 {
		return ""
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && mpsIsDigit(s[k]) {
			for k < len(s) && mpsIsDigit(s[k]) {
				k++
			}
			return s[:k]
		}
	}
	return s[:j]
}

func (m *FloatMPSReader) readLine() error {
	for m.ok {
		if m.input == nil {
			return ErrBounds
		}
		line, err := m.input.ReadString('\n')
		m.line = strings.TrimSuffix(line, "\n")
		if err != nil && !(err == io.EOF && len(line) > 0) {
			m.LineNumber++
			if e := m.fail(); e != nil {
				return e
			}
			if e := m.message("cannot read from file\n"); e != nil {
				return e
			}
			if err != io.EOF {
				return err
			}
		}
		m.LineNumber++
		if m.line != "" && m.line[0] != '*' && strings.Trim(m.line, " \t") != "" {
			break
		}
	}
	return nil
}
func (m *FloatMPSReader) readName() error {
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if strings.HasPrefix(m.line, "NAME") {
			m.line = strings.Trim(m.line[4:], " \t")
			m.Name = m.line
			return nil
		}
		if !m.ok {
			return nil
		}
	}
}
func newFloatMPSRow(name string, index uint32, typ mpsRowType) *floatMPSRow {
	return &floatMPSRow{name: name, index: index, typ: typ, coefficients: insdict.NewDict[string, float64]()}
}
func (m *FloatMPSReader) addRow() error {
	if len(m.line) < 2 {
		return nil
	}
	m.line = strings.Trim(m.line, " \t")
	if len(m.line) == 0 {
		return ErrBounds
	}
	c := m.line[0]
	m.line = strings.Trim(m.line[1:], " \t")
	index := uint32(m.rows.Len()) - m.costLines
	var typ mpsRowType
	switch c {
	case 'E':
		typ = mpsEqual
	case 'L':
		typ = mpsLess
	case 'G':
		typ = mpsGreater
	case 'N':
		typ = mpsCost
	default:
		return nil
	}
	m.rows.Set(m.line, newFloatMPSRow(m.line, index, typ))
	if c == 'N' {
		m.costName = m.line
		m.costLines++
	}
	return nil
}
func (m *FloatMPSReader) readRows() error {
	if err := m.readLine(); err != nil {
		return err
	}
	// The source header-search loop does not read another line.
	if !strings.Contains(m.line, "ROWS") && m.ok {
		return ErrMPSStalled
	}
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if strings.HasPrefix(m.line, "COLUMNS") {
			return nil
		}
		if err := m.addRow(); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}

type mpsRowField uint8

const (
	mpsCoefficient mpsRowField = iota
	mpsRHS
	mpsRange
)

func (m *FloatMPSReader) assignRow(row *floatMPSRow, column, value string, field mpsRowField) error {
	n, err := mpsNumber(value)
	if err != nil {
		return err
	}
	switch field {
	case mpsCoefficient:
		row.coefficients.Set(column, n)
	case mpsRHS:
		row.rhs = n
	case mpsRange:
		row.span = n
		return m.expandRange(row)
	}
	return nil
}
func (m *FloatMPSReader) cannotUnderstand() error {
	if err := m.fail(); err != nil {
		return err
	}
	if err := m.message("cannot understand this line\n"); err != nil {
		return err
	}
	return m.message("line = %s, line number is %d\n", m.line, m.LineNumber)
}
func (m *FloatMPSReader) shortLine(column bool) error {
	if column {
		if err := m.message("line is too short for a column\n"); err != nil {
			return err
		}
	} else if err := m.message("line is too short\n"); err != nil {
		return err
	}
	if err := m.message("%s\n", m.line); err != nil {
		return err
	}
	if err := m.message("line number is %d\n", m.LineNumber); err != nil {
		return err
	}
	return m.fail()
}

// Fixed pairs retain the source 8-byte name and 25-byte recursive stride.
func (m *FloatMPSReader) fixedRowPairs(data, column string, field mpsRowField) error {
	if field == mpsRange && len(m.line) < 14 {
		return m.shortLine(false)
	}
	for {
		if len(data) < 22 {
			return m.cannotUnderstand()
		}
		name := strings.Trim(data[:8], " \t")
		row, ok := m.rows.Get2(name)
		if !ok {
			if err := m.message("cannot find %s\n", name); err != nil {
				return err
			}
			return m.cannotUnderstand()
		}
		if err := m.assignRow(row, column, data[8:], field); err != nil {
			return err
		}
		if len(data) <= 24 {
			return nil
		}
		data = data[25:]
		if len(data) < 22 {
			return nil
		}
	}
}
func (m *FloatMPSReader) rowPairs(data, column string, start int, field mpsRowField) error {
	words := mpsWords(data)
	if len(words) == 0 {
		return ErrBounds
	} // unsigned tokens.size()-1 source underflow
	for i := start; i < len(words)-1; i += 2 {
		name := words[i]
		if field == mpsCoefficient && name == "'MARKER'" {
			return nil
		}
		row, ok := m.rows.Get2(name)
		if !ok {
			if field == mpsRange {
				if len(m.line) < 14 {
					return m.shortLine(false)
				}
				data = m.line[14:]
			}
			return m.fixedRowPairs(data, column, field)
		}
		if err := m.assignRow(row, column, words[i+1], field); err != nil {
			return err
		}
	}
	return nil
}
func (m *FloatMPSReader) readColumns() error {
	name := ""
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if strings.HasPrefix(m.line, "RHS") {
			return nil
		}
		if len(m.line) < 22 {
			return m.shortLine(true)
		}
		if n := strings.Trim(m.line[4:12], " \t"); n != "" {
			name = n
		}
		if _, ok := m.columns.Get2(name); !ok {
			m.columns.Set(name, &floatMPSColumn{name: name, index: uint32(m.columns.Len())})
		}
		if err := m.rowPairs(m.line[14:], name, 0, mpsCoefficient); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}
func (m *FloatMPSReader) readRHS() error {
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if strings.HasPrefix(m.line, "BOUNDS") || strings.HasPrefix(m.line, "ENDATA") || strings.HasPrefix(m.line, "RANGES") {
			return nil
		}
		if len(m.line) < 14 {
			return m.shortLine(false)
		}
		if err := m.rowPairs(m.line[14:], "", 0, mpsRHS); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}
func (m *FloatMPSReader) expandRange(row *floatMPSRow) error {
	index := uint32(m.rows.Len()) - m.costLines
	name := row.name + "_range"
	var typ mpsRowType
	var rhs float64
	abs := row.span
	if abs < 0 {
		abs = -abs
	}
	switch row.typ {
	case mpsGreater:
		typ = mpsLess
		rhs = row.rhs + abs
	case mpsLess:
		typ = mpsGreater
		rhs = row.rhs - abs
	case mpsEqual:
		if row.span > 0 {
			row.typ = mpsGreater
			typ = mpsLess
		} else {
			row.typ = mpsLess
			typ = mpsGreater
		}
		rhs = row.rhs + row.span
	default:
		if err := m.message("unexpected bound type %d at line %d\n", row.typ, m.LineNumber); err != nil {
			return err
		}
		if err := m.fail(); err != nil {
			return err
		}
		return ErrMPSUnhandled
	}
	other := newFloatMPSRow(name, index, typ)
	m.rows.Set(name, other)
	other.rhs = rhs
	for col, v := range row.coefficients.All() {
		if _, ok := m.columns.Get2(col); sourceDebugEnabled && !ok {
			return ErrAssertion
		}
		other.coefficients.Set(col, v)
	}
	return nil
}
func (m *FloatMPSReader) readRanges() error {
	if !strings.HasPrefix(m.line, "RANGES") {
		return nil
	}
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		if len(mpsWords(m.line)) < 2 {
			return nil
		}
		if err := m.rowPairs(m.line, "", 1, mpsRange); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}
func (m *FloatMPSReader) updateBound(b *floatMPSBound, words []string) error {
	if len(m.line) < 1 {
		return ErrBounds
	}
	i := len(m.line)
	if k := strings.IndexByte(m.line[1:], ' '); k >= 0 {
		i = k + 1
	}
	// Keep the source's literal shift assertion, including its LIMITMAX failure.
	if sourceDebugEnabled && uint(len(m.line))>>uint(i) == 0 {
		return ErrAssertion
	}
	typ := m.line[1:i]
	if typ == "BV" {
		b.upperSet = true
		b.upper = 1
		return nil
	}
	switch typ {
	case "UP", "UI", "LIMITMAX", "LO", "LI", "FX":
		if len(words) <= 1 {
			return m.fail()
		}
		// Source publishes the matching flag before numeric conversion.
		switch typ {
		case "UP", "UI", "LIMITMAX":
			b.upperSet = true
		case "LO", "LI":
			b.lowSet = true
		case "FX":
			b.fixedSet = true
		}
		v, err := mpsNumber(words[1])
		if err != nil {
			return err
		}
		switch typ {
		case "UP", "UI", "LIMITMAX":
			b.upper = v
		case "LO", "LI":
			b.low = v
		case "FX":
			b.fixed = v
		}
	case "FR":
		b.free = true
	case "PL":
		b.lowSet = true
		b.low = 0
	case "MI":
		b.upperSet = true
		b.upper = 0 // source retains its default lower flag
	default:
		if err := m.message("unexpected bound type %s at line %d\n", typ, m.LineNumber); err != nil {
			return err
		}
		if err := m.fail(); err != nil {
			return err
		}
		return ErrMPSUnhandled
	}
	return nil
}
func (m *FloatMPSReader) createBound() error {
	if len(m.line) < 14 {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	data := m.line[14:]
	words := mpsWords(data)
	if len(words) == 0 {
		if err := m.fail(); err != nil {
			return err
		}
		if err := m.message("error at line %d\n", m.LineNumber); err != nil {
			return err
		}
		return ErrMPSUnhandled
	}
	col, ok := m.columns.Get2(words[0])
	if !ok {
		if len(data) < 14 {
			return m.shortLine(false)
		}
		if len(data) < 22 {
			return m.cannotUnderstand()
		}
		name := strings.Trim(data[:8], " \t")
		col, ok = m.columns.Get2(name)
		if !ok {
			if err := m.message("cannot find %s\n", name); err != nil {
				return err
			}
			return m.cannotUnderstand()
		}
		words = []string{name, data[14:]}
	}
	if col.bound == nil {
		col.bound = &floatMPSBound{lowSet: true}
	}
	return m.updateBound(col.bound, words)
}
func (m *FloatMPSReader) readBounds() error {
	if !strings.HasPrefix(m.line, "BOUNDS") {
		return nil
	}
	for {
		if err := m.readLine(); err != nil {
			return err
		}
		// m_line[0] on an empty std::string is '\0': the loop just ends.
		if len(m.line) == 0 || m.line[0] != ' ' {
			return nil
		}
		if err := m.createBound(); err != nil {
			return err
		}
		if !m.ok {
			return nil
		}
	}
}
func (m *FloatMPSReader) Read() error {
	if m.input == nil {
		return m.fail()
	}
	if err := m.readName(); err != nil {
		return err
	}
	if err := m.readRows(); err != nil {
		return err
	}
	if err := m.readColumns(); err != nil {
		return err
	}
	if err := m.readRHS(); err != nil {
		return err
	}
	if strings.HasPrefix(m.line, "BOUNDS") {
		if err := m.readBounds(); err != nil {
			return err
		}
		return m.readRanges()
	}
	if strings.HasPrefix(m.line, "RANGES") {
		if err := m.readRanges(); err != nil {
			return err
		}
		return m.readBounds()
	}
	return nil
}
func (m *FloatMPSReader) relation(typ mpsRowType) (LPRelation, error) {
	switch typ {
	case mpsLess:
		return RelationLessOrEqual, nil
	case mpsGreater:
		return RelationGreaterOrEqual, nil
	case mpsEqual:
		return RelationEqual, nil
	}
	if err := m.message("Unexpected rt %d\n", typ); err != nil {
		return 0, err
	}
	if err := m.fail(); err != nil {
		return 0, err
	}
	return 0, ErrMPSUnhandled
}

// CreatePrimalSolver and CreateDualSolver share the concrete floating model
// population. Source fill_solver visits rows before columns and does not gate
// creation on IsOK. A population error returns the partially populated owner.
func (m *FloatMPSReader) CreatePrimalSolver() (*FloatPrimalSimplex, error) {
	solver, err := NewFloatPrimalSimplex()
	if err != nil {
		return nil, err
	}
	return solver, m.fillFloatSolver(solver.FloatLPSolver)
}
func (m *FloatMPSReader) CreateDualSolver() (*FloatDualSimplex, error) {
	solver, err := NewFloatDualSimplex()
	if err != nil {
		return nil, err
	}
	return solver, m.fillFloatSolver(solver.FloatLPSolver)
}
func (m *FloatMPSReader) fillFloatSolver(solver *FloatLPSolver) error {
	for _, row := range m.rows.All() {
		cost := row.name == m.costName
		if !cost {
			rel, err := m.relation(row.typ)
			if err != nil {
				return err
			}
			if err := solver.AddConstraint(rel, row.rhs, row.index); err != nil {
				return err
			}
		}
		for name, v := range row.coefficients.All() {
			col, ok := m.columns.Get2(name)
			if !ok {
				if err := sourceAssertionFailure(); err != nil {
					return err
				}
				m.columns.Set(name, nil)
				return ErrBounds
			}
			if col == nil {
				return ErrBounds
			}
			if cost {
				solver.GetOrCreateColumnInfo(col.index).Cost = v
			} else {
				solver.SetRowColumnCoefficient(row.index, col.index, v)
			}
		}
	}
	for _, col := range m.columns.All() {
		if col == nil {
			return ErrBounds
		}
		ci := solver.GetOrCreateColumnInfo(col.index)
		b := col.bound
		if b == nil {
			ci.SetLowBound(0)
		} else if !b.free {
			if b.lowSet {
				ci.SetLowBound(b.low)
			}
			if b.upperSet {
				ci.SetUpperBound(b.upper)
			}
			if b.fixedSet {
				ci.SetFixedValue(b.fixed)
			}
		}
		solver.GiveSymbolicNameToColumn(col.name, col.index)
	}
	return nil
}
