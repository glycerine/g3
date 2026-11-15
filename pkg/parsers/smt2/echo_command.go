package smt2

// Source: src/cmd_context/basic_cmds.cpp echo_cmd.

import "fmt"

func (p *commandParser) echoCommand() error {
	// parse_ext_cmd / parse_next_cmd_arg's CPK_STRING check.
	if p.token == RightParenToken {
		return fmt.Errorf("invalid command, argument(s) missing")
	}
	if p.token != StringToken {
		return fmt.Errorf("invalid command argument, string expected")
	}
	value := p.scanner.String()
	if err := p.advance(); err != nil {
		return err
	}
	// UNARY_CMD runs its action in set_next_arg, before parse_ext_cmd
	// looks for ')'.
	if err := p.context.WriteEcho(p.out, value); err != nil {
		return err
	}
	return p.optionValueEnd()
}
