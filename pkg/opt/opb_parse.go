package opt

// Source: src/opt/opt_parse.cpp class opb (parse_id, parse_ids,
// parse_coeff_r, parse_term, parse_objective, parse_constraint, parse)
// over opt_stream_buffer.

import (
	"fmt"
	"io"
)

// ParseOPB is opb::parse. The source writes a malformed input's message to
// std::cerr and calls exit(3); ParseOPB returns it as an *InputParseExit.
// Statements come back in file order, which is the order the source builds
// and registers them.
func ParseOPB(input io.Reader) (OPBDocument, error) {
	if input == nil {
		return OPBDocument{}, fmt.Errorf("opb: nil input")
	}
	in, err := newOptStreamBuffer(input)
	if err != nil {
		return OPBDocument{}, err
	}
	p := opbParser{in: in}
	if err = p.parse(); err != nil {
		return OPBDocument{}, err
	}
	return p.document, nil
}

type opbParser struct {
	in       *optStreamBuffer
	document OPBDocument
}

func (p *opbParser) parse() error {
	in := p.in
	for {
		in.skipWhitespace()
		if in.eof() {
			return nil
		}
		var err error
		if in.ch() == '*' {
			in.skipLine()
		} else if in.ch() == 'm' {
			// z3 4.7.1 tests parse_token("min:") and then parse_token("max:")
			// here; parse_token consumes a matching prefix, so the "m" of
			// "max:" is gone and every max: objective is a parse error. The
			// fork reads the common "m" once (FORK FIX #39; AUDIT.md
			// opt_02-F4) and g3 follows the fixed reader.
			in.next()
			if in.parseToken("in:") {
				err = p.parseObjective(true)
			} else if in.parseToken("ax:") {
				err = p.parseObjective(false)
			} else {
				err = p.parseConstraint()
			}
		} else {
			err = p.parseConstraint()
		}
		if err != nil {
			return err
		}
	}
}

func (p *opbParser) parseID() (OPBLiteral, error) {
	in := p.in
	negated := in.parseToken("~")
	if !in.parseToken("x") {
		return OPBLiteral{}, &InputParseExit{Message: fmt.Sprintf("(error line %d \"unexpected char: %s\" expected \"x\")", in.line, string([]byte{byte(in.ch())})), Status: 3}
	}
	id, err := in.parseInt()
	if err != nil {
		return OPBLiteral{}, err
	}
	in.skipWhitespace()
	return OPBLiteral{Variable: id, Negative: negated}, nil
}

func (p *opbParser) parseIDs() ([]OPBLiteral, error) {
	first, err := p.parseID()
	if err != nil {
		return nil, err
	}
	result := []OPBLiteral{first}
	for p.in.ch() == '~' || p.in.ch() == 'x' {
		next, err := p.parseID()
		if err != nil {
			return nil, err
		}
		result = append(result, next)
	}
	return result, nil
}

// parseCoeffR is parse_coeff_r's scan. rational("") and rational("-") are
// 0, so a coefficient without digits reads as "0".
func (p *opbParser) parseCoeffR() string {
	in := p.in
	in.skipWhitespace()
	num := make([]byte, 0, 16)
	pos := true
	if in.ch() == '-' {
		pos = false
		in.next()
	}
	if in.ch() == '+' {
		in.next()
	}
	if !pos {
		num = append(num, '-')
	}
	in.skipWhitespace()
	digits := 0
	for '0' <= in.ch() && in.ch() <= '9' {
		num = append(num, byte(in.ch()))
		in.next()
		digits++
	}
	if digits == 0 {
		return "0"
	}
	return string(num)
}

func (p *opbParser) parseTerm() (OPBTerm, error) {
	coefficient := p.parseCoeffR()
	literals, err := p.parseIDs()
	if err != nil {
		return OPBTerm{}, err
	}
	return OPBTerm{Coefficient: coefficient, Literals: literals}, nil
}

func (p *opbParser) parseObjective(isMin bool) error {
	statement := OPBStatement{Objective: true, MinimizeSyntax: isMin}
	term, err := p.parseTerm()
	if err != nil {
		return err
	}
	statement.Terms = append(statement.Terms, term)
	for !p.in.parseToken(";") && !p.in.eof() {
		if term, err = p.parseTerm(); err != nil {
			return err
		}
		statement.Terms = append(statement.Terms, term)
	}
	p.document.Statements = append(p.document.Statements, statement)
	return nil
}

func (p *opbParser) parseConstraint() error {
	in := p.in
	var statement OPBStatement
	term, err := p.parseTerm()
	if err != nil {
		return err
	}
	statement.Terms = append(statement.Terms, term)
	related := false
	for !in.eof() {
		if in.parseToken(">=") {
			statement.Relation, statement.Bound = OPBGreaterEqual, p.parseCoeffR()
		} else if in.parseToken("=") {
			statement.Relation, statement.Bound = OPBEqual, p.parseCoeffR()
		} else if in.parseToken("<=") {
			statement.Relation, statement.Bound = OPBLessEqual, p.parseCoeffR()
		} else {
			if term, err = p.parseTerm(); err != nil {
				return err
			}
			statement.Terms = append(statement.Terms, term)
			continue
		}
		in.parseToken(";")
		related = true
		break
	}
	if !related {
		// z3 4.7.1 asserts the integer-valued sum itself as a hard
		// constraint when the input ends before a relation, and crashes
		// (SIGSEGV). The fork reports the truncated constraint instead
		// (FORK FIX #NEXT, see FORK_BUGFIXES.md).
		return &InputParseExit{Message: fmt.Sprintf("(error line %d \"unexpected end of input\" expected \">=\", \"=\" or \"<=\")", in.line), Status: 3}
	}
	p.document.Statements = append(p.document.Statements, statement)
	return nil
}
