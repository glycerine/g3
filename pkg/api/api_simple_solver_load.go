package api

// Source: src/api/api_solver.cpp:129-150 solver_from_stream.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/parsers/smt2"
)

// LoadSMT2 parses an SMT-LIB command stream and adds its final assertions to
// the solver. Check commands are syntax-checked but not executed. Parsing is
// completed before the solver is mutated.
func (s *APISimpleSolver) LoadSMT2(input io.Reader) (resultErr error) {
	return s.loadSMT2(input, nil)
}

// loadSMT2 with a non-nil out prints parse errors there as (error "...")
// and returns a *smt2.ReportedParseError, as solver_from_stream's
// parse_smt2_commands does on std::cout.
func (s *APISimpleSolver) loadSMT2(input io.Reader, out io.Writer) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	if input == nil {
		return ErrSMTContextOwner
	}
	parser, err := (&SMTContext{s.owner}).newSMT2ParserContext(SMT2Bindings{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parser.Close()) }()
	if out == nil {
		err = smt2.ParseAssertions(parser, input)
	} else {
		err = smt2.ParseAssertionsReporting(parser, input, out)
	}
	if err != nil {
		return err
	}
	assertions, err := parser.Assertions()
	if err != nil {
		return err
	}
	// solver_from_stream parses first, then init_solver.
	if err = s.ensure(); err != nil {
		return err
	}
	for _, assertion := range assertions {
		if err = s.solver.Assert(assertion); err != nil {
			return err
		}
	}
	return nil
}
