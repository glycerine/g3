package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointObjectLoadSMT2String(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	input := `(declare-rel loaded-p ()) (rule (! loaded-p :named loaded-fact))`
	astTestMust(t, f.fp.LoadSMT2String(input))
	predicates, err := f.fp.command.FixedpointPredicates()
	astTestMust(t, err)
	if len(predicates) != 4 {
		t.Fatal("loaded predicate count", len(predicates))
	}
	status, err := f.fp.Query(predicates[3])
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("loaded query", status)
	}
}

func TestAPIFixedpointObjectLoadSMT2UsesFreshDeclarationScope(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.LoadSMT2String(`(declare-rel reload-r ()) (rule (! reload-r :named first))`))

	if err := f.fp.LoadSMT2String(`(rule (! reload-r :named bare))`); err == nil {
		t.Fatal("later load reused an earlier parser declaration")
	}
	rules, err := f.fp.Rules()
	astTestMust(t, err)
	if len(rules) != 1 {
		t.Fatal("failed bare rule changed rules", len(rules))
	}
	releaseAPIRefs(t, f.m, rules)
	predicates, err := f.fp.command.FixedpointPredicates()
	astTestMust(t, err)
	if len(predicates) != 4 {
		t.Fatal("failed bare rule changed predicates", len(predicates))
	}

	astTestMust(t, f.fp.LoadSMT2String(`(declare-rel reload-r ()) (rule (! reload-r :named second))`))
	rules, err = f.fp.Rules()
	astTestMust(t, err)
	if len(rules) != 2 {
		t.Fatal("complete later load rule count", len(rules))
	}
	releaseAPIRefs(t, f.m, rules)
	predicates, err = f.fp.command.FixedpointPredicates()
	astTestMust(t, err)
	if len(predicates) != 4 {
		t.Fatal("redeclared predicate count", len(predicates))
	}
}
