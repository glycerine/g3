package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSimplifyFourierMotzkinPublishesFalseResolvent(t *testing.T) {
	s, _, _ := fmIntervalFixture(t, false, 3, 1)
	count, supported, err := s.SimplifyFourierMotzkin()
	assignmentMust(t, err)
	if !supported || count != 1 || len(s.clauses.originals) != 2 {
		t.Fatal("conflict resolvent", count, supported, len(s.clauses.originals))
	}
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.False {
		t.Fatal("expected conflict", result)
	}
}
