package smt2

// Source: src/parsers/smt2/smt2parser.cpp:2627-2648 and
// src/cmd_context/cmd_context.cpp check_sat.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (p *commandParser) checkSatCommand(assuming, execute bool) error {
	assumptions, err := p.checkSatAssumptions(assuming)
	if err != nil {
		return err
	}
	// Source parse_check_sat reads assumptions up to the ')'.
	// parse_check_sat_assuming runs check_sat right after the literal
	// list's ')' and does not check the command's ')': its next() skips
	// whatever token follows.
	if !assuming {
		if err = p.requireEnd(); err != nil {
			return err
		}
	}
	if !execute {
		return nil
	}
	status, err := p.context.CheckSat(assumptions)
	if err != nil {
		return err
	}
	text := "unknown"
	if status == lbool.True {
		text = "sat"
	} else if status == lbool.False {
		text = "unsat"
	}
	if _, err = fmt.Fprintln(p.out, text); err != nil {
		return err
	}
	if err = p.context.ValidateInfoStatus(status); err != nil {
		return err
	}
	return p.context.DumpModels(p.out, status)
}
