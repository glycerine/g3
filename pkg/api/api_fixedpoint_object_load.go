package api

// Source: src/api/api_datalog.cpp:357-400.

import (
	"io"

	"github.com/glycerine/g3/pkg/parsers/smt2"
)

// LoadSMT2 parses fixedpoint declarations, rules, background assertions, and
// fixedpoint scopes into this object. Query and check commands are parsed but
// not executed, matching the native fixedpoint-from-stream loading phase.
func (f *APIFixedpoint) LoadSMT2(input io.Reader) error {
	if err := f.open(); err != nil {
		return err
	}
	if input == nil {
		return ErrSMTContextOwner
	}
	before, err := f.command.Assertions()
	if err != nil {
		return err
	}
	if err = smt2.ParseAssertions(f.command, input); err != nil {
		return err
	}
	after, err := f.command.Assertions()
	if err != nil {
		return err
	}
	first := 0
	for first < len(before) && first < len(after) && before[first] == after[first] {
		first++
	}
	// Native fixedpoint_from_stream transfers the parser's final assertion
	// suffix into the fixedpoint background after parsing succeeds.
	for _, assertion := range after[first:] {
		if err = f.command.AssertFixedpoint(assertion); err != nil {
			return err
		}
	}
	return nil
}
