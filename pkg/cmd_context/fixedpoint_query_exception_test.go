package cmd_context

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestFixedpointQueryExceptionPreservesOtherFailures(t *testing.T) {
	for _, kind := range []base.RulePropertyKind{base.RulePropertyNegativePredicate, base.RulePropertyInfiniteSort, base.RulePropertyUninterpreted, base.RulePropertyQuantifier, base.RulePropertyNested} {
		property := &base.RulePropertyError{Kind: kind, SourceMessage: "source diagnostic"}
		cause := fmt.Errorf("%w: validation: %w", fp.ErrPropositionalHorn, property)
		caught := classifyFixedpointQueryException(cause)
		exception := FixedpointCaughtQueryException(errors.Join(caught))
		if exception == nil || exception.Message != property.SourceMessage || !errors.Is(exception, fp.ErrPropositionalHorn) {
			t.Fatalf("query exception = %v", caught)
		}
		cleanup := errors.New("cleanup failure")
		if got := FixedpointCaughtQueryException(errors.Join(caught, cleanup)); got != nil {
			t.Fatalf("swallowed timeout cleanup failure: %v", got)
		}
		joined := errors.Join(cause, cleanup)
		if got := classifyFixedpointQueryException(joined); got != joined {
			t.Fatalf("swallowed backend cleanup failure: %v", got)
		}
	}
	if got := classifyFixedpointQueryException(&base.RulePropertyError{Kind: base.RulePropertyExistentialTail, SourceMessage: "other"}); FixedpointCaughtQueryException(got) != nil {
		t.Fatal("classified an unverified exception kind")
	}
	caught := classifyFixedpointQueryException(fp.ErrUnstratifiedNegation)
	if exception := FixedpointCaughtQueryException(caught); exception == nil || exception.Message != "Negation is not stratified!" || !errors.Is(exception, fp.ErrUnstratifiedNegation) {
		t.Fatalf("unstratified exception = %v", caught)
	}
	cleanup := errors.New("cleanup failure")
	joined := errors.Join(fp.ErrUnstratifiedNegation, cleanup)
	if got := classifyFixedpointQueryException(joined); got != joined {
		t.Fatalf("swallowed backend cleanup failure: %v", got)
	}
	if got := FixedpointCaughtQueryException(errors.Join(caught, cleanup)); got != nil {
		t.Fatalf("swallowed timeout cleanup failure: %v", got)
	}
	head := &base.InvalidRuleHeadError{SourceMessage: "illegal head"}
	caught = classifyFixedpointQueryException(head)
	if exception := FixedpointCaughtQueryException(caught); exception == nil || exception.Message != head.SourceMessage {
		t.Fatalf("invalid head exception = %v", caught)
	}
	joined = errors.Join(head, cleanup)
	if got := classifyFixedpointQueryException(joined); got != joined {
		t.Fatalf("swallowed invalid-head cleanup failure: %v", got)
	}
}
