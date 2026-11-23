package api

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/solver"
)

// Every table denotes a concrete finite interpretation. The asserted formula
// includes two differently ordered domain variables and one key variable.
func TestEnumerationPredicateDependencies(t *testing.T) {
	for _, definition := range []string{"direct", "split", "incomplete", "disjunction", "existential", "fixed-argument", "multiple-functions", "different-selector"} {
		t.Run(definition, func(t *testing.T) {
			pre, _ := enumerationProofInput()
			pre += `(declare-fun rr (A C B) Bool)
(declare-fun keyA (B) A)
(declare-fun keyC (B) C)
(declare-fun allowed (B) Bool)
(declare-fun switch (B) Bool)
(declare-fun other (B) Bool)
`
			yes := func(x bool) string {
				if x {
					return "true"
				}
				return "false"
			}
			pred := "(rr a c b)"
			mask := "(and (= a (keyA b)) (= (keyC b) c) (allowed b))"
			row := func(s string) string { return "(assert (forall ((b B)(a A)(c C)) " + s + "))\n" }
			direct := row("(= " + pred + " " + mask + ")")
			one := row("(= (and " + pred + " (switch b)) (and " + mask + " (switch b)))")
			two := row("(= (and (not (switch b)) " + pred + ") (and " + mask + " (not (switch b))))")
			switch definition {
			case "direct":
				pre += direct
			case "split":
				pre += one + two
			case "incomplete":
				pre += one
			case "disjunction":
				pre += row("(or (not (other b)) (= " + pred + " " + mask + "))")
			case "different-selector":
				pre += one + row("(= (and (not (other b)) "+pred+") (and "+mask+" (not (other b))))")
			case "existential":
				pre += "(assert (exists ((b B)(a A)(c C)) (= " + pred + " " + mask + ")))\n"
			case "fixed-argument":
				pre += "(assert (forall ((a A)(b B)) (= (rr a c0 b) (and (allowed b) (= a (keyA b))))))\n"
			case "multiple-functions":
				pre += direct + "(assert (forall ((a A)(b B)(c C)) (=> (rr a c b) (= a (choose b)))))\n"
			}
			for v := range 3 {
				pre += fmt.Sprintf("(assert (= (keyA b%d) a%d))\n(assert (= (keyC b%d) c%d))\n(assert (= (allowed b%d) %s))\n(assert (= (switch b%d) %s))\n(assert (= (other b%d) %s))\n", v, (v+1)%3, v, v%2, v, yes(v != 1), v, yes(v == 0), v, yes(v != 0))
			}
			example := func(a, c, b int) bool {
				inside := a == (b+1)%3 && c == b%2 && b != 1
				switch definition {
				case "incomplete":
					return inside || b != 0
				case "disjunction":
					return inside || b == 0
				case "different-selector":
					return inside || b != 0
				case "existential":
					return inside || b == 1
				case "fixed-argument":
					if c == 0 {
						return b != 1 && a == (b+1)%3
					}
					return true
				default:
					return inside
				}
			}
			for a := range 3 {
				for c := range 2 {
					for b := range 3 {
						pre += fmt.Sprintf("(assert (= (rr a%d c%d b%d) %s))\n", a, c, b, yes(example(a, c, b)))
					}
				}
			}
			var formulas []string
			var expects []bool
			queries := []string{
				"(forall ((a A)(c C)(b B)) (=> (rr a c b) (= (keyA b) a)))",
				"(exists ((a A)(c C)(b B)) (and (rr a c b) (distinct (keyC b) c)))",
				"(forall ((a A)(c C)(b B)) (or (not (rr a c b)) (and (= a (keyA b)) (= (keyC b) c))))",
				"(forall ((a A)(c C)(b B)) (= (and (rr a c b) (switch b)) (and (= a (keyA b)) (= c (keyC b)) (allowed b) (switch b))))",
				"(forall ((a A)(c C)(b B)) (=> (rr a c b) (exists ((a A)) (and (= a (keyA b)) (rr a c b)))))",
				"(forall ((a A)(c C)(b B)) (ite (rr a c b) (= (keyC b) c) true))",
			}
			eval := func(i int) bool {
				ex := i == 1
				for a := range 3 {
					for c := range 2 {
						for b := range 3 {
							r := example(a, c, b)
							ka := (b + 1) % 3
							kc := b % 2
							v := false
							switch i {
							case 0:
								v = !r || a == ka
							case 1:
								v = r && c != kc
							case 2:
								v = !r || a == ka && c == kc
							case 3:
								v = (r && b == 0) == (a == ka && c == kc && b != 1 && b == 0)
							case 4:
								v = !r || example(ka, c, b)
							case 5:
								v = !r || c == kc
							}
							if v == ex {
								return ex
							}
						}
					}
				}
				return !ex
			}
			for i, q := range queries {
				v := eval(i)
				formulas = append(formulas, q, "(not "+q+")")
				expects = append(expects, v, !v)
			}
			script := pre + "(assert true)\n"
			for _, q := range formulas {
				script += "(assert " + q + ")\n"
			}
			ctx, donor := orderSolver(t, script)
			m, e := ctx.Manager()
			astTestMust(t, e)
			forms, e := solver.Assertions(donor.solver)
			astTestMust(t, e)
			cut := len(forms) - len(formulas) - 1
			anchor := forms[cut]
			var batch enumerationRefutation
			t.Cleanup(func() { astTestMust(t, batch.release()) })
			var initial order.EnumerationPremises
			structure, e := initial.Project(m, forms[:cut+1])
			astTestMust(t, e)
			if structure == nil {
				t.Fatal("base projection declined")
			}
			hasSupport := strings.Contains(structure.Text, "(forall ((garg")
			wantSupport := definition == "direct" || definition == "split" || definition == "multiple-functions"
			if hasSupport != wantSupport {
				t.Fatalf("predicate dependency detected = %t, want %t", hasSupport, wantSupport)
			}
			for i, q := range formulas {
				basis := append(slices.Clone(forms[:cut]), forms[cut+1+i])
				input := append(basis, anchor)
				v, e := batch.check(m, input)
				astTestMust(t, e)
				if v != !expects[i] {
					t.Fatalf("%s formula %d expected truth %t: %s", definition, i, expects[i], q)
				}

			}
		})
	}
}

// Both directions functionally determine the other variable. A substitution
// must keep one binder in scope, including beneath another quantifier.
func TestEnumerationCyclicArgumentDependencies(t *testing.T) {
	base, _ := enumerationProofInput()
	const equations = `
(declare-fun edge (A A) Bool)
(declare-fun next (A) A)
(declare-fun prev (A) A)
(assert (= (next a0) a1))
(assert (= (next a1) a2))
(assert (= (next a2) a0))
(assert (= (prev a0) a2))
(assert (= (prev a1) a0))
(assert (= (prev a2) a1))
(assert (forall ((x A)(y A)) (= (edge x y) (and (= x (prev y)) (= y (next x))))))
`
	const goal = "(exists ((x A)(y A)) (and (edge x y) (forall ((x A)) (distinct (prev y) x))))"
	ctx, donor := orderSolver(t, base+equations+"(assert "+goal+")\n(assert (not "+goal+"))\n(assert true)")
	m, err := ctx.Manager()
	astTestMust(t, err)
	forms, err := solver.Assertions(donor.solver)
	astTestMust(t, err)
	n := len(forms) - 3
	var batch enumerationRefutation
	t.Cleanup(func() { astTestMust(t, batch.release()) })
	for i := range 2 {
		input := append(slices.Clone(forms[:n]), forms[n+i], forms[n+2])
		proof, err := batch.premises.Project(m, input)
		astTestMust(t, err)
		if proof == nil || !strings.Contains(proof.Premises(), "(forall ((garg") {
			t.Fatal("missing cyclic dependency premises")
		}
		proved, err := batch.check(m, input)
		astTestMust(t, err)
		if proved != (i == 0) {
			t.Fatalf("query %d, proved %t", i, proved)
		}
	}
}
