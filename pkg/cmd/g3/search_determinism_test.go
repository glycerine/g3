package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Everything but timing and memory figures must repeat exactly between runs.
var volatileStatistic = regexp.MustCompile(`(?m)^\s*\(?:(time|total-time|max-memory|memory|num-allocs)\s+[0-9.]+\)?\s*$`)

func repeatableOutput(t *testing.T, script string) string {
	t.Helper()
	return volatileStatistic.ReplaceAllString(runScriptWithin(t, script+"(get-info :all-statistics)", 2*time.Minute), "")
}

func requireRepeatable(t *testing.T, script string, runs int) {
	t.Helper()
	first := repeatableOutput(t, script)
	for run := 1; run < runs; run++ {
		if got := repeatableOutput(t, script); got != first {
			t.Fatalf("run %d differs from run 0:\n--- run 0\n%s\n--- run %d\n%s", run, first, run, got)
		}
	}
}

// Model-finder choices used to follow Go's random map order.
func TestMBQISearchIsRepeatable(t *testing.T) {
	requireRepeatable(t, `(set-logic UFLIA)
(set-option :smt.mbqi.max_iterations 5)
(declare-fun f (Int) Int)
(declare-fun g (Int Int) Int)
(declare-const c Int)
(declare-const d Int)
(assert (forall ((x Int)) (! (= (f x) (g x x)) :pattern ((f x)))))
(assert (forall ((x Int)) (! (<= (f x) (+ x 0)) :pattern ((f x)))))
(assert (forall ((x Int)) (! (>= (g x x) (g x d)) :pattern ((f x)))))
(assert (distinct (g c d) (f 1)))
(assert (<= (g d c) c))
(assert (distinct d (+ 1 2)))
(check-sat)`, 8)
}

// Releasing references in map order used to change later slots and ids, which
// reordered terms for checks after a scope.
func TestRepeatedChecksAcrossScopesAreRepeatable(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "card2bv_weighted_models.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	requireRepeatable(t, strings.TrimSpace(string(script)), 12)
}

// Ackermann abstraction names and lemma order used to follow map order.
func TestAckermannizationIsRepeatable(t *testing.T) {
	for _, name := range []string{"ackermann_usort_models.smt2", "qfbv_uf_models.smt2"} {
		script, err := os.ReadFile(filepath.Join("test_vectors", name))
		if err != nil {
			t.Fatal(err)
		}
		requireRepeatable(t, strings.TrimSpace(string(script)), 12)
	}
}
