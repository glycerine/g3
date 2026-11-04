package base_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestInvalidRuleHeadDisplayAllocationOwnership(t *testing.T) {
	failures, complete := 0, false
	for allowance := int64(0); allowance < 1024 && !complete; allowance++ {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f := newRulePropertiesFixture(t)
			head, err := f.manager.MkFalse()
			ruleMust(t, err)
			root, err := f.manager.NewRef(head)
			ruleMust(t, err)
			defer func() { ruleMust(t, f.manager.DelRef(&root)) }()
			formats, err := f.manager.GetFormatManager()
			ruleMust(t, err)
			// The printer interns its permanent token declarations on first use.
			// Warm those caches before measuring temporary format ownership.
			_, warm := f.rules.NewRule(head, nil, nil, "")
			var warmHead *base.InvalidRuleHeadError
			if !errors.As(warm, &warmHead) {
				t.Fatal("warmup", warm)
			}
			before, beforeFormat := contextSMT2LiveCounts(f.manager.Stats()), contextSMT2LiveCounts(formats.Stats())
			f.manager.Budget().FailAfter(allowance)
			rule, cause := f.rules.NewRule(head, nil, nil, "")
			f.manager.Budget().FailAfter(-1)
			if rule != nil {
				t.Fatal("accepted invalid head")
			}
			var headErr *base.InvalidRuleHeadError
			if errors.As(cause, &headErr) {
				complete = true
				if headErr.SourceMessage != "Illegal head. The head predicate needs to be uninterpreted and registered (as recursive) false" {
					t.Fatal(headErr.SourceMessage)
				}
			} else {
				failures++
				var receipt memory.ConstructionFailure
				if errors.As(cause, &receipt) {
					ruleMust(t, receipt.Cleanup())
					ruleMust(t, receipt.Cleanup())
				}
				var resource memory.Failure
				if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &resource) || resource.Kind != memory.FailureMemout) {
					t.Fatal("unexpected failure", cause)
				}
			}
			if after := contextSMT2LiveCounts(f.manager.Stats()); after != before {
				t.Fatalf("main AST ownership: %v -> %v, %v", before, after, cause)
			}
			if after := contextSMT2LiveCounts(formats.Stats()); after != beforeFormat {
				t.Fatalf("format ownership: %v -> %v, %v", beforeFormat, after, cause)
			}
		})
	}
	if failures == 0 || !complete {
		t.Fatalf("failure frontier: failures=%d complete=%t", failures, complete)
	}
}
