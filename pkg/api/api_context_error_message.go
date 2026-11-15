package api

// Source: src/api/api_context.cpp:248-272 (context::handle_exception),
// 447-464 (_get_error_msg); src/api/z3_api.h Z3_error_code.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

// Z3_error_code values from src/api/z3_api.h.
const (
	apiErrorOK             uint32 = 0
	apiErrorSort           uint32 = 1
	apiErrorIOB            uint32 = 2
	apiErrorInvalidArg     uint32 = 3
	apiErrorParser         uint32 = 4
	apiErrorNoParser       uint32 = 5
	apiErrorInvalidPattern uint32 = 6
	apiErrorMemout         uint32 = 7
	apiErrorFileAccess     uint32 = 8
	apiErrorInternalFatal  uint32 = 9
	apiErrorInvalidUsage   uint32 = 10
	apiErrorDecRef         uint32 = 11
	apiErrorException      uint32 = 12
)

// errorMessage is _get_error_msg for this context.
func (c *apiContextResults) errorMessage(code uint32) string {
	switch code {
	case apiErrorOK:
		return "ok"
	case apiErrorSort:
		return "type error"
	case apiErrorIOB:
		return "index out of bounds"
	case apiErrorInvalidArg:
		return "invalid argument"
	case apiErrorParser:
		return "parser error"
	case apiErrorNoParser:
		return "parser (data) is not available"
	case apiErrorInvalidPattern:
		return "invalid pattern"
	case apiErrorMemout:
		return "out of memory"
	case apiErrorFileAccess:
		return "file access error"
	case apiErrorInternalFatal:
		return "internal error"
	case apiErrorInvalidUsage:
		return "invalid usage"
	case apiErrorDecRef:
		return "invalid dec_ref command"
	case apiErrorException:
		if c == nil {
			return "Z3 exception"
		}
		return c.exceptionMessage
	default:
		return "unknown"
	}
}

// apiErrorCodeOf maps the Go errors that stand for an API SET_ERROR_CODE
// (rather than a thrown z3_exception) to their Z3_error_code.
func apiErrorCodeOf(err error) (uint32, bool) {
	switch {
	case errors.Is(err, errAPINumeralParse):
		return apiErrorParser, true
	}
	return apiErrorOK, false
}

// handleAPIError publishes a failed API call the way the C API does: an
// error that stands for SET_ERROR_CODE sets that code, and anything else is
// the z3_exception caught by Z3_CATCH and passed to context::handle_exception.
func (c *apiContextResults) handleAPIError(err error) {
	if code, ok := apiErrorCodeOf(err); ok {
		c.setErrorCode(code)
		return
	}
	var exception z3exception.Z3Exception
	if errors.As(err, &exception) && exception.HasErrorCode() {
		switch exception.ErrorCode() {
		case z3exception.ErrorMemout:
			c.setErrorCode(apiErrorMemout)
		case z3exception.ErrorParser:
			c.setErrorCode(apiErrorParser)
		case z3exception.ErrorINIFile:
			c.setErrorCode(apiErrorInvalidArg)
		case z3exception.ErrorOpenFile:
			c.setErrorCode(apiErrorFileAccess)
		default:
			c.setErrorCode(apiErrorInternalFatal)
		}
		return
	}
	if exception != nil {
		c.exceptionMessage = exception.Message()
	} else {
		c.exceptionMessage = err.Error()
	}
	c.setErrorCode(apiErrorException)
}
