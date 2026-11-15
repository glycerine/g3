package polynomial

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

// manager::imp::checkpoint throws polynomial_exception(Z3_CANCELED_MSG): the
// message reaching reason-unknown is "canceled", and the Go cancellation
// classification still holds.
func TestPolynomialCheckpointCancellationIsSourceException(t *testing.T) {
	var source z3exception.Z3Exception
	if !errors.As(polynomialCancellationError, &source) || source.Message() != "canceled" {
		t.Fatal("polynomial cancellation is not a default_exception", polynomialCancellationError)
	}
	var exception PolynomialException
	if !errors.As(polynomialCancellationError, &exception) || !errors.Is(polynomialCancellationError, ErrUPolynomialCanceled) {
		t.Fatal("polynomial cancellation lost its classification")
	}
	if z3exception.SourceMessage(errors.Join(polynomialCancellationError, errors.New("cleanup"))) != "canceled" {
		t.Fatal("joined cancellation message")
	}
}
