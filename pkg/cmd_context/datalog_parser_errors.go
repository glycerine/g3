package cmd_context

// Source: dparser::unexpected and the default_exception throws of
// datalog_parser.cpp, caught by parse_stream as z3_exception.

import "fmt"

// datalogException is a default_exception thrown by the Datalog parser.
type datalogException struct{ msg string }

func (e *datalogException) Error() string { return e.msg }

func datalogExceptionf(format string, args ...any) error {
	return &datalogException{msg: fmt.Sprintf(format, args...)}
}

// unexpected throws "%s at line %u '%s' found '%s'\n".
func (p *datalogParser) unexpected(tok dtoken, msg string) (dtoken, error) {
	return tok, datalogExceptionf("%s at line %d '%s' found '%s'\n", msg, p.lexer.getLine(), p.lexer.tokenData(), dtokenStrings[tok])
}
