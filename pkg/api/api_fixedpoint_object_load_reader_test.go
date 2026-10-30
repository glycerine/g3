package api

import (
	"strings"
	"testing"
)

func TestAPIFixedpointObjectLoadSMT2Reader(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.LoadSMT2(strings.NewReader(`(assert true) (rule (! api-object-fp-p :named reader-p))`)))
	assertions, err := f.fp.Assertions()
	astTestMust(t, err)
	rules, err := f.fp.Rules()
	astTestMust(t, err)
	if len(assertions) != 1 || len(rules) != 1 {
		t.Fatal("loaded state", len(assertions), len(rules))
	}
	releaseAPIRefs(t, f.m, assertions)
	releaseAPIRefs(t, f.m, rules)
}
