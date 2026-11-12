package cmd_context

// Source: dl_query_cmd::execute catches default_exception from rel_query.
import "github.com/glycerine/g3/pkg/muz/fp"

// FixedpointQueryException marks a source default_exception thrown inside
// rel_query. Unwrap retains the original Go error for API callers.
type FixedpointQueryException struct {
	Message string
	Cause   error
}

func (e *FixedpointQueryException) Error() string { return e.Cause.Error() }
func (e *FixedpointQueryException) Unwrap() error { return e.Cause }

// classifyFixedpointQueryException accepts only a known source exception and
// the Go fragment marker. A joined cleanup failure must remain an error.
func classifyFixedpointQueryException(err error) error {
	if message := fp.SourceDefaultExceptionMessage(err); message != "" {
		return &FixedpointQueryException{Message: message, Cause: err}
	}
	return err
}

// FixedpointCaughtQueryException rejects joined errors containing anything
// besides the marked exception, including timeout-scope cleanup failures.
func FixedpointCaughtQueryException(err error) *FixedpointQueryException {
	for err != nil {
		if exception, ok := err.(*FixedpointQueryException); ok {
			return exception
		}
		if multi, ok := err.(interface{ Unwrap() []error }); ok {
			children := multi.Unwrap()
			if len(children) != 1 {
				return nil
			}
			err = children[0]
			continue
		}
		if single, ok := err.(interface{ Unwrap() error }); ok {
			err = single.Unwrap()
			continue
		}
		return nil
	}
	return nil
}
