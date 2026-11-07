package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These satisfiable array formulas were reported unsat when a disequality
// conflict omitted the merges linking its terms to the disequal roots.
func TestDisequalityConflictKeepsProofPathAntecedents(t *testing.T) {
	for _, name := range []string{
		"array_diseq_conflict_antecedents.smt2",
		"array_diseq_conflict_antecedents_b.smt2",
		"array_diseq_conflict_antecedents_c.smt2",
	} {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = run(bytes.NewReader(script), &out); err != nil {
				t.Fatal(err, out.String())
			}
			if out.String() != "sat\n" {
				t.Fatalf("got %q, want sat", out.String())
			}
		})
	}
}

func runScriptWithin(t *testing.T, script string, limit time.Duration) string {
	t.Helper()
	type outcome struct {
		output string
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		var out bytes.Buffer
		err := run(strings.NewReader(script), &out)
		done <- outcome{out.String(), err}
	}()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err, result.output)
		}
		return result.output
	case <-time.After(limit):
		t.Fatalf("script did not finish within %v:\n%s", limit, script)
	}
	return ""
}

// Quantified linear arithmetic needs the source's QSAT route, for every logic
// selecting it. MBQI alone does not terminate on these goals, also in the source.
func TestQuantifiedArithmeticSelectsQSATRoute(t *testing.T) {
	bodies := map[string]struct{ reals, ints string }{
		"sat":   {"(forall ((x Real)) (exists ((y Real)) (> y x)))", "(forall ((x Int)) (exists ((y Int)) (> y x)))"},
		"unsat": {"(exists ((x Real)) (forall ((y Real)) (>= x y)))", "(exists ((x Int)) (forall ((y Int)) (>= x y)))"},
	}
	for want, body := range bodies {
		for _, tc := range []struct{ logic, formula string }{
			{"(set-logic LRA)", body.reals},
			{"(set-logic LIA)", body.ints},
			{"(set-logic NIA)", body.ints},
			{"", body.reals},
			{"", body.ints},
		} {
			t.Run(want+"/"+tc.logic+"/"+tc.formula, func(t *testing.T) {
				script := fmt.Sprintf("%s(assert %s)(check-sat)", tc.logic, tc.formula)
				if got := runScriptWithin(t, script, time.Minute); got != want+"\n" {
					t.Fatalf("got %q, want %s", got, want)
				}
			})
		}
	}
}

func diamondScript(n int) string {
	var s strings.Builder
	s.WriteString("(set-logic QF_UF)(declare-sort U 0)")
	for i := 0; i <= n; i++ {
		fmt.Fprintf(&s, "(declare-const a%d U)(declare-const b%d U)(declare-const c%d U)", i, i, i)
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(&s, "(assert (or (and (= a%d b%d) (= b%d a%d)) (and (= a%d c%d) (= c%d a%d))))", i, i, i, i+1, i, i, i, i+1)
	}
	fmt.Fprintf(&s, "(assert (not (= a0 a%d)))(check-sat)(get-info :all-statistics)", n)
	return s.String()
}

// Without atoms for a[i]=a[i+1], refuting a diamond chain by resolution needs
// exponentially many conflicts. Linear counts mean lemmas are too strong.
func TestDiamondConflictsGrowLikeResolution(t *testing.T) {
	pattern := regexp.MustCompile(`:conflicts +([0-9.]+)`)
	conflicts := func(n int) float64 {
		output := runScriptWithin(t, diamondScript(n), time.Minute)
		if !strings.HasPrefix(output, "unsat\n") {
			t.Fatalf("diamond %d: %q", n, output)
		}
		match := pattern.FindStringSubmatch(output)
		if match == nil {
			t.Fatalf("diamond %d has no conflict count: %q", n, output)
		}
		value, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	small, large := conflicts(5), conflicts(9)
	if large < 8*small {
		t.Fatalf("conflicts grew from %v to %v over four extra diamonds", small, large)
	}
}
