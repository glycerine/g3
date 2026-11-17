package api

// Source: src/api/api_datalog.cpp: Z3_fixedpoint_query, _query_relations,
// _push, _pop, _add_cover and _get_cover_delta run inside Z3_TRY/Z3_CATCH,
// so a default_exception the engines throw reaches the error handler with
// its own message (Z3_EXCEPTION).

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// APIFixedpointException is the source default_exception behind a Go
// fixedpoint error: Error and Message are the source text, Unwrap keeps
// the Go cause.
type APIFixedpointException struct {
	z3exception.DefaultException
	Cause error
}

func (e *APIFixedpointException) Unwrap() error { return e.Cause }

// fixedpointException classifies err as the source exception it stands for
// (fp.SourceDefaultExceptionMessage); other errors are returned unchanged.
func fixedpointException(err error) error {
	if err == nil {
		return nil
	}
	var already *APIFixedpointException
	if errors.As(err, &already) {
		return err
	}
	if message := fp.SourceDefaultExceptionMessage(err); message != "" {
		return &APIFixedpointException{DefaultException: z3exception.NewDefaultException(message), Cause: err}
	}
	return err
}
