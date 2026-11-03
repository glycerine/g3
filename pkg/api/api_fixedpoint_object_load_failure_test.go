package api

import (
	"reflect"
	"testing"
)

func TestAPIFixedpointObjectLoadSMT2ReportsParserFailure(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	before, err := f.fp.command.FixedpointPredicates()
	astTestMust(t, err)
	if err = f.fp.LoadSMT2String(`(declare-rel Partial ()) (rule Partial partial) (assert`); err == nil {
		t.Fatal("malformed fixedpoint stream loaded")
	}
	rules, err := f.fp.Rules()
	astTestMust(t, err)
	defer releaseAPIRefs(t, f.m, rules)
	if len(rules) != 0 {
		t.Fatal("failed parse added rules", len(rules))
	}
	assertions, err := f.fp.Assertions()
	astTestMust(t, err)
	defer releaseAPIRefs(t, f.m, assertions)
	if len(assertions) != 0 {
		t.Fatal("failed parse added assertions", len(assertions))
	}
	after, err := f.fp.command.FixedpointPredicates()
	astTestMust(t, err)
	if !reflect.DeepEqual(after, before) {
		t.Fatal("failed parse added predicates", len(before), len(after))
	}
}
