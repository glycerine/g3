package rewriter

import (
	"fmt"
	"reflect"
	"testing"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestSmallAndLargeRewriterOptionListsAgree(t *testing.T) {
	c, globals, _, _ := dynAckSourceFixture(t)
	for _, key := range []string{
		"", "arith_lhs", "arith_ineq_lhs", "gcd_rounding", "elim_to_real",
		"push_to_real", "algebraic_number_evaluator", "expand_power", "mul_to_power",
		"elim_rem", "expand_tan", "eq2ineq", "sort_sums", "max_degree", "flat",
		"som", "som_blowup", "hoist_mul", "hoist_cmul", "max_memory", "max_steps",
		"pull_cheap_ite", "cache_all", "push_ite_arith", "push_ite_bv",
		"ignore_patterns_on_ground_qbody", "rewrite_patterns",
	} {
		name := key
		if name == "" {
			name = "defaults"
		}
		t.Run(name, func(t *testing.T) {
			local, err := c.NewValue()
			paramsMust(t, err)
			defer func() { paramsMust(t, c.Del(&local)) }()
			switch key {
			case "":
			case "max_degree", "som_blowup", "max_memory", "max_steps":
				paramsMust(t, c.SetUIntName(&local, &key, 17))
			case "flat", "push_to_real", "algebraic_number_evaluator", "ignore_patterns_on_ground_qbody":
				paramsMust(t, c.SetBoolName(&local, &key, false))
			default:
				paramsMust(t, c.SetBoolName(&local, &key, true))
			}
			read := func() (ArithRewriter, theoryRewriterConfiguration) {
				var arithmetic ArithRewriter
				paramsMust(t, arithmetic.updateParams(c, globals, &local))
				theory := TheoryRewriter{context: c, globals: globals, local: &local}
				paramsMust(t, theory.readLocalParams())
				return arithmetic, theory.configuration
			}
			empty := c.EmptyValue()
			var values uparams.ScalarParams
			if !c.ReadScalarParams(&values, &local, &empty) {
				t.Fatal("expected a small parameter list")
			}
			smallArith, smallTheory := read()
			for i := 0; i < 13; i++ {
				extra := fmt.Sprintf("unused_option_%d", i)
				paramsMust(t, c.SetBoolName(&local, &extra, true))
			}
			if c.ReadScalarParams(&values, &local, &empty) {
				t.Fatal("expected the full parameter lookup")
			}
			largeArith, largeTheory := read()
			if smallArith != largeArith || !reflect.DeepEqual(smallTheory, largeTheory) {
				t.Fatalf("option %q changed configuration across list sizes", key)
			}
		})
	}
}
