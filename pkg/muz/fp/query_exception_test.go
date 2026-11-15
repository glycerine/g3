package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

// AUDIT.md muz_pdr_02-F1: an engine default_exception reaches dl_query_cmd's
// z3_exception handler as "query failed: <msg>", even through Go wrapping.
func TestSourceDefaultExceptionMessageAcceptsWrappedDefaultException(t *testing.T) {
	cause := z3exception.NewDefaultException("PDR cannot solve non-ground tails: x")
	wrapped := fmt.Errorf("muz/pdr: initialize predicate rules: %w", cause)
	if got := SourceDefaultExceptionMessage(wrapped); got != "PDR cannot solve non-ground tails: x" {
		t.Fatalf("wrapped default_exception message %q", got)
	}
	// A cleanup failure joined to the exception must stay an error.
	if got := SourceDefaultExceptionMessage(errors.Join(cause, errors.New("cleanup"))); got != "" {
		t.Fatalf("joined cleanup failure classified as %q", got)
	}
}
