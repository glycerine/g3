package shell

// Source: src/shell/datalog_frontend.cpp read_datalog (parse step);
// src/muz/fp/datalog_parser.cpp parser_impl::parse_file.

import (
	"fmt"
	"os"
	"time"

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
	if err = datalogFrontendVerbose("parsing finished\n"); err != nil {
		return err
	}
	return datalogFrontendVerbose("running saturation...\n")
}
