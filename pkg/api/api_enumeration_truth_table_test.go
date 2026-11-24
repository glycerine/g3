package api

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/solver"
)

// Every function application in the goals has a concrete value fixed by its
// table. Quantifying over the three constructor lists gives a solver-independent
// answer, including when an inner variable occurs in another function's argument.
func TestEnumerationQuantifiersAgainstTruthTables(t *testing.T) {
	for table := range 3 {
		t.Run(fmt.Sprintf("table%d", table), func(t *testing.T) {
			base, assertions := enumerationProofInput()
			var script strings.Builder
			script.WriteString(base)
			choice := func(b int) int { return (b + 1) % 3 }
			value := func(a, b int) int {
				return (a*(b+1) + b*table) % 2
			}
			for b := range 3 {
				fmt.Fprintf(&script, "(assert (= (choose b%d) a%d))\n", b, choice(b))
				assertions++
				for a := range 3 {
					fmt.Fprintf(&script, "(assert (= (x0 a%d b%d) c%d))\n", a, b, value(a, b))
					assertions++
				}
			}
			predicates := []struct {
				term string
				eval func(a, b, c int) bool
			}{
				{"(= (x0 x y) z)", func(a, b, c int) bool { return value(a, b) == c }},
				{"(xor (= (choose y) x) (= (x0 x y) z))", func(a, b, c int) bool {
					return (choice(b) == a) != (value(a, b) == c)
				}},
				{"(=> (distinct (choose y) x) (= (x0 (choose y) y) z))", func(a, b, c int) bool {
					return choice(b) == a || value(choice(b), b) == c
				}},
				{"(= (x0 (ite (= z c1) (choose y) x) y) z)", func(a, b, c int) bool {
					if c == 1 {
						a = choice(b)
					}
					return value(a, b) == c
				}},
			}
			var wants []bool
			for _, pred := range predicates {
				for mask := range 8 {
					outer, middle, inner := mask&1 != 0, mask&2 != 0, mask&4 != 0
					goal := fmt.Sprintf("(%s ((x A)) (%s ((y B)) (%s ((z C)) %s)))",
						enumerationQuantifier(outer), enumerationQuantifier(middle), enumerationQuantifier(inner), pred.term)
					result := enumerationFold(3, outer, func(a int) bool {
						return enumerationFold(3, middle, func(b int) bool {
							return enumerationFold(2, inner, func(c int) bool { return pred.eval(a, b, c) })
						})
					})
					fmt.Fprintf(&script, "(assert %s)\n(assert (not %s))\n", goal, goal)
					wants = append(wants, result, !result)
				}
			}
			c, donor := orderSolver(t, script.String())
			m, err := c.Manager()
			astTestMust(t, err)
			forms, err := solver.Assertions(donor.solver)
			astTestMust(t, err)
			if len(forms) != assertions+len(wants) {
				t.Fatal("unexpected number of assertions", len(forms))
			}
			baseForms := forms[:assertions]
			batch := enumerationRefutation{}
			t.Cleanup(func() { astTestMust(t, batch.release()) })
			for i, want := range wants {
				input := append(slices.Clone(baseForms), forms[assertions+i])
				// Exercise both the speculative and already asserted goal paths.
				proved, err := batch.check(m, input, i%2 == 1)
				astTestMust(t, err)
				if proved == want {
					t.Fatalf("goal %d has truth value %v, refuted=%v", i, want, proved)
				}
			}
		})
	}
}

func enumerationQuantifier(universal bool) string {
	if universal {
		return "forall"
	}
	return "exists"
}

func enumerationFold(size int, universal bool, eval func(int) bool) bool {
	for v := range size {
		if eval(v) != universal {
			return !universal
		}
	}
	return universal
}
