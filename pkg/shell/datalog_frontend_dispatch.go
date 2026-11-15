package shell

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/cmd_context"
)

// RunDatalogFrontend is read_datalog for the shell. A parse failure prints
// the parser's message and "ERROR: failed to parse file" to the diagnostic
// stream (std::cerr) and ends the run successfully: main ignores
// read_datalog's return value.
func RunDatalogFrontend(context *cmd_context.Context, path string, output, diagnostics io.Writer) error {
	frontend, err := NewDatalogFrontend(context, output)
	if err != nil {
		return err
	}
	err = frontend.Run(path)
	var parseError *DatalogParseError
	if errors.As(err, &parseError) {
		if diagnostics == nil {
			diagnostics = os.Stderr
		}
		if _, err = io.WriteString(diagnostics, parseError.Message+"ERROR: failed to parse file\n"); err != nil {
			return err
		}
		return nil
	}
	return err
}
