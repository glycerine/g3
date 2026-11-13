package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// ErrUnstratifiedNegation is the source context::close query exception.
var ErrUnstratifiedNegation = errors.New("muz/fp: negation is not stratified")

// SourceDefaultExceptionMessage returns the source default_exception text
// carried by err: a rule_properties check, an illegal rule head, or the
// unstratified-negation failure, possibly wrapped by the Go fragment
// marker. Joined errors holding anything else (such as cleanup failures)
// are not classified, and the result is "".
func SourceDefaultExceptionMessage(err error) string {
	message := ""
	var visit func(error) bool
	visit = func(e error) bool {
		if e == nil || e == ErrPropositionalHorn {
			return true
		}
		if p, ok := e.(*base.RulePropertyError); ok {
			if message != "" || (p.Kind != base.RulePropertyNegativePredicate && p.Kind != base.RulePropertyInfiniteSort && p.Kind != base.RulePropertyUninterpreted && p.Kind != base.RulePropertyQuantifier && p.Kind != base.RulePropertyNested && p.Kind != base.RulePropertyExistentialTail) || p.SourceMessage == "" {
				return false
			}
			message = p.SourceMessage
			return true
		}
		if head, ok := e.(*base.InvalidRuleHeadError); ok {
			if message != "" || head.SourceMessage == "" {
				return false
			}
			message = head.SourceMessage
			return true
		}
		// iz3's interpolation_error is an iz3_exception, a z3_exception
		// that dl_query_cmd catches as "query failed".
		if _, ok := e.(interp.InterpolationError); ok {
			if message != "" {
				return false
			}
			message = e.Error()
			return true
		}
		if e == ErrUnstratifiedNegation {
			if message != "" {
				return false
			}
			message = "Negation is not stratified!"
			return true
		}
		if multi, ok := e.(interface{ Unwrap() []error }); ok {
			for _, child := range multi.Unwrap() {
				if !visit(child) {
					return false
				}
			}
			return true
		}
		if single, ok := e.(interface{ Unwrap() error }); ok {
			return visit(single.Unwrap())
		}
		return false
	}
	if visit(err) {
		return message
	}
	return ""
}

// HornTacticException is the source default_exception that escapes the
// horn and horn-simplify tactics (horn_tactic::imp::verify rethrows it and
// simplify lets it propagate from check_rules). Error and Message are the
// source text; Unwrap keeps the Go cause for errors.Is.
type HornTacticException struct {
	z3exception.DefaultException
	Cause error
}

func (e *HornTacticException) Unwrap() error { return e.Cause }

func hornTacticException(err error) error {
	if err == nil {
		return nil
	}
	var already *HornTacticException
	if errors.As(err, &already) {
		return err
	}
	if message := SourceDefaultExceptionMessage(err); message != "" {
		return &HornTacticException{DefaultException: z3exception.NewDefaultException(message), Cause: err}
	}
	return err
}
