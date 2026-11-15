package shell

// Source: src/shell/datalog_frontend.cpp read_datalog (parse step);
// src/muz/fp/datalog_parser.cpp parser_impl::parse_file.

import (
	"fmt"
	"os"
	"time"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// datalogFrontendVerbose is read_datalog's IF_VERBOSE(1, ...); these
// progress lines are compiled only into g3trace builds.
func datalogFrontendVerbose(text string) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	return diagnostic.ProgressVerbose(1, text)
}

// DatalogParseError is a failed parse_file / parse_directory. The parser has
// its own message (Message); read_datalog then prints "ERROR: failed to
// parse file" and returns, and main ignores that result.
type DatalogParseError struct {
	Message string
	Cause   error
}

func (e *DatalogParseError) Error() string { return e.Cause.Error() }
func (e *DatalogParseError) Unwrap() error { return e.Cause }

func (f *DatalogFrontend) Load(path string) error {
	if f == nil || f.context == nil || f.loaded || path == "" {
		return ErrDatalogFrontend
	}
	if err := datalogFrontendVerbose("Z3 Datalog Engine\n"); err != nil {
		return err
	}
	f.started, f.path = time.Now(), path
	// IF_VERBOSE(1, "Z3 Datalog Engine") (AUDIT.md FUZZ-F5; g3trace only).
	if err := datalogFrontendVerbose(1, "Z3 Datalog Engine\n"); err != nil {
		return err
	}
	// read_datalog constructs its context with a local Datalog engine
	// choice, overriding a conflicting fixedpoint module engine parameter.
	if err := f.context.SelectDatalogFrontendEngine(); err != nil {
		return err
	}
	// datalog::is_directory(file) selects the WPA parser.
	var err error
	if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
		err = f.context.ParseWPADirectory(path)
	} else {
		// parse_file: char_reader reader(filename); if (!reader()) ...
		input, openErr := os.Open(path)
		if openErr != nil {
			f.parseFinished = time.Now()
			return &DatalogParseError{Message: fmt.Sprintf("ERROR: could not open file '%s'.\n", path), Cause: openErr}
		}
		if err = input.Close(); err != nil {
			return err
		}
		err = f.context.ParseDatalogFile(path)
	}
	f.parseFinished = time.Now()
	if err != nil {
		return &DatalogParseError{Message: err.Error() + "\n", Cause: fmt.Errorf("Datalog parse %s: %w", path, err)}
	}
	f.loaded = true
	// IF_VERBOSE(1, "parsing finished"), IF_VERBOSE(1, "running
	// saturation..."), then ctx.close() and IF_VERBOSE(10,
	// original_rules.display_deps(verbose_stream())).
	if err = datalogFrontendVerbose(1, "parsing finished\nrunning saturation...\n"); err != nil {
		return err
	}
	if diagnostic.VerboseProgress {
		if enabled, verr := diagnostic.ProcessVerboseEnabled(10); verr != nil {
			return verr
		} else if enabled {
			stream, verr := diagnostic.BorrowProcessVerboseStream()
			if verr != nil {
				return verr
			}
			if verr = f.context.WriteFixedpointRuleDependencies(datalogVerboseWriter{stream}); verr != nil {
				return verr
			}
			if verr = stream.Flush(); verr != nil {
				return verr
			}
		}
	}
	if err = datalogFrontendVerbose("parsing finished\n"); err != nil {
		return err
	}
	return datalogFrontendVerbose("running saturation...\n")
}

// datalogFrontendVerbose is IF_VERBOSE(level, verbose_stream() << text),
// compiled only into g3trace builds (AUDIT.md FUZZ-F5).
func datalogFrontendVerbose(level uint32, text string) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	return diagnostic.ProgressVerbose(level, text)
}

type datalogVerboseWriter struct{ stream *utext.NumericTextStream }

func (w datalogVerboseWriter) Write(p []byte) (int, error) {
	if err := w.stream.AppendCString(string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}
