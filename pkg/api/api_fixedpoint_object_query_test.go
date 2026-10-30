package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointRulesScopesAndQueries(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	truth := f.pin(f.m.MkTrue())
	astTestMust(t, f.fp.AddRule(f.uAtom, "scoped"))
	astTestMust(t, f.fp.Assert(truth))
	rules, err := f.fp.Rules()
	astTestMust(t, err)
	if len(rules) != 1 {
		t.Fatal("scoped rules", len(rules))
	}
	releaseAPIRefs(t, f.m, rules)
	assertions, err := f.fp.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("scoped assertions", len(assertions))
	}
	releaseAPIRefs(t, f.m, assertions)

	astTestMust(t, f.fp.AddFact(f.p, nil))
	astTestMust(t, f.fp.AddRuleWithBound(f.step, "step", 4))
	astTestMust(t, f.fp.UpdateRule(f.step, "step"))
	status, err := f.fp.QueryRelations([]ast.ASTHandle{f.q})
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("reachable fixedpoint relation", status)
	}
	if reason, x := f.fp.ReasonUnknown(); x != nil || reason != "ok" {
		t.Fatal("fixedpoint reason", reason, x)
	}
}
