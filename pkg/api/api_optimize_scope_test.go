package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizePushPopTrailsAssertionsAndObjectives(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.Assert(f.lower))
	astTestMust(t, f.opt.Push())
	astTestMust(t, f.opt.Assert(f.upper))
	_, err := f.opt.Maximize(f.x)
	astTestMust(t, err)
	if n, countErr := f.opt.NumObjectives(); countErr != nil || n != 1 {
		t.Fatal("scoped objective count", n, countErr)
	}
	astTestMust(t, f.opt.Pop())
	if n, countErr := f.opt.NumObjectives(); countErr != nil || n != 0 {
		t.Fatal("objective survived pop", n, countErr)
	}
	assertions, err := f.opt.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 || assertions[0] != f.lower {
		t.Fatal("assertion scope", assertions)
	}
	status, err := f.opt.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("post-pop check", status)
	}
}
