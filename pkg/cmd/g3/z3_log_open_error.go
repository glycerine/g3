package main

import "fmt"

// z3LogOpenError preserves the log frontend's separate diagnostic and exit code.
type z3LogOpenError struct {
	filename string
	cause    error
}

func (e *z3LogOpenError) Error() string {
	return fmt.Sprintf("Error: failed to open file \"%s\".", e.filename)
}

func (e *z3LogOpenError) Unwrap() error { return e.cause }

type shellInputOpenError struct {
	filename string
	cause    error
}

func (e *shellInputOpenError) Error() string {
	return fmt.Sprintf("failed to open file \"%s\".", e.filename)
}

func (e *shellInputOpenError) Unwrap() error { return e.cause }
