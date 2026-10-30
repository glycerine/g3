package sls

// Source: src/tactic/sls/sls_tracker.h:530-536 and
// src/tactic/sls/sls_evaluator.h:798-823.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSLSTrackerDisplayAndRandomizeSourceOverloads(t *testing.T) {
	_, search, evaluator, tracker, manager, numbers := slsEngineFixture(t)
	p := slsBoolConst(t, manager, "sls-display-p")
	q := slsBoolConst(t, manager, "sls-display-q")
	root, err := manager.MkAnd(p, q)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, p, q, root)
	assertions := []ast.ASTHandle{root}
	if err = search.Initialize(assertions); err != nil {
		t.Fatal(err)
	}
	if err = search.Reset(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = tracker.DisplayModel(&out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "sls-display-p = 0\n") || !strings.Contains(text, "sls-display-q = 0\n") {
		t.Fatal("source insertion-ordered model display", text)
	}
	if err = search.SetRandomSeed(17); err != nil {
		t.Fatal(err)
	}
	if err = evaluator.RandomizeExpression(search, root); err != nil {
		t.Fatal(err)
	}
	if err = evaluator.RandomizeAssertions(search, assertions); err != nil {
		t.Fatal(err)
	}
	for _, node := range []ast.ASTHandle{p, q} {
		value, err := tracker.GetValue(node)
		if err != nil {
			t.Fatal(err)
		}
		zero, err := numbers.IsZero(value)
		if err != nil {
			t.Fatal(err)
		}
		one, err := numbers.IsOne(value)
		if err != nil || !zero && !one {
			t.Fatal("randomized Boolean left source domain", zero, one, err)
		}
	}
}
