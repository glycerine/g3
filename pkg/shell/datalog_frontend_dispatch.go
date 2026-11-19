package shell

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// RunDatalogFrontend is read_datalog for the shell. A parse failure prints
// the parser's message and "ERROR: failed to parse file" to the diagnostic
// stream (std::cerr) and ends the run successfully: main ignores
// read_datalog's return value.
func RunDatalogFrontend(context *cmd_context.Context, path string, output, diagnostics io.Writer) error {
	return RunDatalogFrontendWithTimeout(context, path, output, diagnostics, nil)
}

func RunDatalogFrontendWithTimeout(context *cmd_context.Context, path string, output, diagnostics io.Writer, timer *TimeoutService) error {
	if context == nil {
		return ErrDatalogFrontend
	}
	signals := &control.RunSignals{Done: timer.DoneChan()}
	if signals.RunStopped() {
		timer.ReportTimeout()
		return nil
	}
	previous := context.SetRunSignals(signals)
	defer context.SetRunSignals(previous)
	frontend, err := NewDatalogFrontend(context, timeoutGateWriter(output, signals))
	if err != nil {
		return err
	}
	if diagnostics == nil {
		diagnostics = os.Stderr
	}
	frontend.SetDiagnostics(timeoutGateWriter(diagnostics, signals))
	err = frontend.Run(path)
	if signals.RunStopped() {
		timer.ReportTimeout()
		return nil
	}
	var parseError *DatalogParseError
	if errors.As(err, &parseError) {
		if _, err = io.WriteString(diagnostics, parseError.Message+"ERROR: failed to parse file\n"); err != nil {
			return err
		}
		return nil
	}
	// The engines' default_exceptions (rule_properties' checks, say) leave
	// read_datalog uncaught; main prints "ERROR: " << ex.msg().
	if message := fp.SourceDefaultExceptionMessage(err); err != nil && message != "" {
		return errors.Join(z3exception.NewDefaultException(message), err)
	}
	return err
}
