package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFixedpointRawRulesPreserveInputBoundsAndScopedLifetime(t *testing.T) {
	e, m := newRoutingEngine(t)
	_, p := routingPredicate(t, e, m, "raw-P")
	_, q := routingPredicate(t, e, m, "raw-Q")
	_, r := routingPredicate(t, e, m, "raw-R")
	body, err := m.MkOr(p, q)
	fpMust(t, err)
	original, err := m.MkImplies(body, r)
	fpMust(t, err)
	fpMust(t, e.AddRuleWithBound(original, "split-rule", 7))
	forms, names, bounds, err := e.RawRules()
	fpMust(t, err)
	defer func() {
		for i := range forms {
			fpMust(t, m.DelRef(&forms[i]))
		}
	}()
	if len(forms) != 1 || len(names) != 1 || names[0] != "split-rule" || len(bounds) != 1 || bounds[0] != 7 {
		t.Fatal("original input metadata", len(forms), names, bounds)
	}
	h, err := m.RefGet(forms[0])
	fpMust(t, err)
	if h != original {
		t.Fatal("raw input was replaced by normalized rules")
	}
	normalized, _, err := e.NamedRules()
	fpMust(t, err)
	if len(normalized) != 2 {
		t.Fatal("fixture must split into two normalized clauses", len(normalized))
	}
	for i := range normalized {
		fpMust(t, m.DelRef(&normalized[i]))
	}
	fpMust(t, e.Push())
	fpMust(t, e.AddRule(p, "scoped-fact"))
	scoped, _, _, err := e.RawRules()
	fpMust(t, err)
	if len(scoped) != 2 {
		t.Fatal("scope did not capture additional raw input")
	}
	for i := range scoped {
		fpMust(t, m.DelRef(&scoped[i]))
	}
	fpMust(t, e.Pop())
	current, currentNames, currentBounds, err := e.RawRules()
	fpMust(t, err)
	if len(current) != 1 || currentNames[0] != names[0] || currentBounds[0] != 7 {
		t.Fatal("pop did not restore original metadata", len(current), currentNames, currentBounds)
	}
	for i := range current {
		fpMust(t, m.DelRef(&current[i]))
	}
	fpMust(t, e.Close())
	if h, err := m.RefGet(forms[0]); err != nil || h != original {
		t.Fatal("owned raw snapshot expired with engine", h, err)
	}
}

func TestFixedpointRawRuleUpdateAndPopRestoreOriginalFormula(t *testing.T) {
	e, m := newRoutingEngine(t)
	_, p := routingPredicate(t, e, m, "raw-update-P")
	_, q := routingPredicate(t, e, m, "raw-update-Q")
	_, r := routingPredicate(t, e, m, "raw-update-R")
	original, err := m.MkImplies(p, q)
	fpMust(t, err)
	fpMust(t, e.AddRuleWithBound(original, "edge", 5))
	fpMust(t, e.Push())
	strongerBody, err := m.MkAnd(p, r)
	fpMust(t, err)
	stronger, err := m.MkImplies(strongerBody, q)
	fpMust(t, err)
	fpMust(t, e.UpdateRule(stronger, "edge"))
	check := func(want ast.ASTHandle) {
		t.Helper()
		forms, names, bounds, err := e.RawRules()
		fpMust(t, err)
		defer func() {
			for i := range forms {
				fpMust(t, m.DelRef(&forms[i]))
			}
		}()
		if len(forms) != 1 || names[0] != "edge" || bounds[0] != 5 {
			t.Fatal("updated raw rule metadata", len(forms), names, bounds)
		}
		h, err := m.RefGet(forms[0])
		fpMust(t, err)
		if h != want {
			t.Fatal("wrong raw formula after update or pop")
		}
	}
	check(stronger)
	if err := e.UpdateRule(p, "edge"); err == nil {
		t.Fatal("non-subsuming update accepted")
	}
	check(stronger)
	fpMust(t, e.Pop())
	check(original)
}
