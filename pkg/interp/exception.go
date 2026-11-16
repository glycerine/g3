package interp

// Source: src/interp/iz3exception.h.

import "github.com/glycerine/g3/pkg/util/z3exception"

type Error struct{ message string }

func NewError(message string) Error              { return Error{message: message} }
func (e Error) Error() string                    { return e.message }
func (e Error) Message() string                  { return e.message }
func (e Error) ErrorCode() z3exception.ErrorCode { return z3exception.ErrorOK }
func (e Error) HasErrorCode() bool               { return false }

// iz3Exception is an iz3_exception thrown while interpolating a proof
// (iz3proof_itp.cpp's "help!" and bad_ineq_inference). It is a
// default_exception that neither iz3translation_full::translate, which
// catches only proof_error and unsupported, nor the Duality solver, which
// catches the char* of an interpolation_failure, handles, so the query
// fails with its message.
func iz3Exception(message string) error { return z3exception.NewDefaultException(message) }
