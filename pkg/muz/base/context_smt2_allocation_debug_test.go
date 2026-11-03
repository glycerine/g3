//go:build g3debug

package base_test

import (
	"fmt"
	"testing"
)

// The debug allocator reserves every temporary node directly. Keep exhaustive
// coverage on the smaller page-crossing fixture and independently exercise the
// original cold 128-query vector, negation owner and final allocation boundary.
func TestContextSMT2LargeDebugAllocationBoundaries(t *testing.T) {
	if !t.Run("query-reference-page", func(t *testing.T) {
		if checkContextSMT2Allocation(t, false, 128, 0, false) {
			t.Fatal("cold query vector did not require reference storage")
		}
	}) {
		return
	}
	if !t.Run("negated-query-reference-page", func(t *testing.T) {
		if checkContextSMT2Allocation(t, false, 128, -1, true) {
			t.Fatal("cold negated query did not require reference storage")
		}
	}) {
		return
	}
	t.Run("completion-boundary", func(t *testing.T) {
		probe := func(allowance int64) (complete, valid bool) {
			valid = t.Run(fmt.Sprint(allowance), func(t *testing.T) {
				complete = checkContextSMT2Allocation(t, false, 128, allowance, false)
			})
			return
		}
		// Zero was proved to fail above. Bracket completion on fresh fixtures,
		// then exercise the adjacent last failure and first complete dump.
		low, high := int64(0), int64(1)
		for {
			complete, valid := probe(high)
			if !valid {
				return
			}
			if complete {
				break
			}
			if high == 65536 {
				t.Fatal("large debug dump missed allocation completion", high)
			}
			low, high = high, high*2
		}
		for high-low > 1 {
			middle := low + (high-low)/2
			complete, valid := probe(middle)
			if !valid {
				return
			}
			if complete {
				high = middle
			} else {
				low = middle
			}
		}
		t.Logf("cold 128-query debug dump: last failure %d, first completion %d", low, high)
	})
}
