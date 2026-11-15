package polynomial

// Source: src/math/polynomial/polynomial.h polynomial::exception;
// src/math/polynomial/polynomial.cpp manager::imp::checkpoint.

import "github.com/glycerine/g3/pkg/util/z3exception"

// PolynomialException preserves polynomial::exception and its
// default_exception base. The Go sentinel ErrUPolynomialCanceled stays the
// errors.Is classification of a cancellation.
type PolynomialException struct {
	z3exception.DefaultException
	canceled bool
}

func (e PolynomialException) Unwrap() error { return e.DefaultException }
func (e PolynomialException) Is(target error) bool {
	return e.canceled && target == ErrUPolynomialCanceled
}

// checkpoint: throw polynomial_exception(Z3_CANCELED_MSG). Immutable value
// boxed once, as for the univariate cancellation.
var polynomialCancellationError error = PolynomialException{
	DefaultException: z3exception.NewDefaultException("canceled"), canceled: true,
}
