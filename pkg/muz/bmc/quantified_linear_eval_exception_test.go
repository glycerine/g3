package bmc

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestQuantifiedLinearModelEvalCatchesOnlyRewriterException(t *testing.T) {
	rewrite := rewriter.NewRewriterException("rewrite failed")
	other := errors.New("cleanup failed")
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"rewriter", rewrite, true},
		{"ordinary", other, false},
		{"joined cleanup", errors.Join(rewrite, other), false},
		{"joined rewrites", errors.Join(rewrite, rewrite), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := quantifiedLinearEvalException(tc.err); got != tc.want {
				t.Fatalf("caught %v: got %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
