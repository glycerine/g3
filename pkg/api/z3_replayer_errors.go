package api

import (
	"errors"
	"fmt"
)

var ErrZ3ReplayState = errors.New("api: invalid Z3 replay state")
var ErrZ3ReplayReference = errors.New("api: invalid Z3 replay argument reference")

type Z3ReplayError struct {
	Line    uint32
	Message string
}

// Z3ReplayCommandError is the Go counterpart of a z3_exception thrown by a
// registered API command. The source prints it and continues replay; parser,
// reference, and z3_error failures remain fatal.
type Z3ReplayCommandError struct{ Message string }

func (e *Z3ReplayCommandError) Error() string { return e.Message }

func (e *Z3ReplayError) Error() string {
	return fmt.Sprintf("Z3 replay line %d: %s", e.Line, e.Message)
}

func (r *Z3Replayer) failure(message string) error {
	return &Z3ReplayError{Line: r.Line(), Message: message}
}

// Z3ReplayExit is the process exit of api_context.cpp default_error_handler
// (exit(1)) during replay. The log frontend ends without its memory and time
// report and the shell exits with Status.
type Z3ReplayExit struct{ Status int }

func (e *Z3ReplayExit) Error() string { return fmt.Sprintf("z3 replay exit(%d)", e.Status) }

// z3ReplayUnsupported marks a log the Go replayer cannot reproduce (a Go-only
// limitation, not an API error); replay stops on it.
type z3ReplayUnsupported string

func (e z3ReplayUnsupported) Error() string { return string(e) }

// isZ3ReplayStructuralError reports the replayer's own failures (bad
// arguments, references or state, unsupported logs). Every other error a
// command returns is an API error that the context's error handler reports.
func isZ3ReplayStructuralError(err error) bool {
	var replayErr *Z3ReplayError
	var commandErr *Z3ReplayCommandError
	var unsupported z3ReplayUnsupported
	var exit *Z3ReplayExit
	return errors.As(err, &replayErr) || errors.As(err, &commandErr) || errors.As(err, &unsupported) ||
		errors.As(err, &exit) || errors.Is(err, ErrZ3ReplayReference) || errors.Is(err, ErrZ3ReplayState)
}
