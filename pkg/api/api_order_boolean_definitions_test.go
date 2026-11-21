package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOrderProofSharesOnlyBooleanDefinitionsFromPremises(t *testing.T) {
	const base = orderPremises + `
(declare-const b B)
(declare-const first Bool)
(declare-const second Bool)
(declare-fun q (B) Bool)
(assert (= first (not (q a))))`
	for _, tc := range []struct {
		name, premise, query string
		shared               bool
		result               lbool.LBool
	}{
		{"same body", "(= second (not (q a)))", "(distinct first second)", true, lbool.False},
		{"same body admits model", "(= second (not (q a)))", "(and first second)", true, lbool.True},
		{"different argument", "(= second (not (q b)))", "(distinct first second)", false, lbool.True},
		{"different sign", "(= second (q a))", "(distinct first second)", false, lbool.True},
		{"definition in query", "(distinct first second)", "(= second (not (q a)))", false, lbool.False},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, base+"(assert "+tc.premise+")(assert "+tc.query+")")
			m, err := c.Manager()
			astTestMust(t, err)
			assertions, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			ordinary, ok, err := order.Relax(m, assertions)
			astTestMust(t, err)
			if !ok {
				t.Fatal("missing order problem")
			}
			proof, ok, err := order.ForProof(m, assertions, 100_000)
			astTestMust(t, err)
			if !ok || proof.Order.Text == "" {
				t.Fatal("missing private proof problem")
			}
			if (proof.Order.Text != ordinary.Text) != tc.shared {
				t.Fatalf("unexpected private problem\n%s", proof.Order.Text)
			}
			_, check := orderSolver(t, proof.Order.Text)
			got, err := check.Check()
			astTestMust(t, err)
			if got != tc.result {
				t.Fatalf("private check returned %v, want %v", got, tc.result)
			}
		})
	}
}
