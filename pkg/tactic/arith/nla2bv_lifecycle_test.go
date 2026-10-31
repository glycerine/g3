package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:144-170,183-194,325-363.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestNLA2BVCollectorDistinguishesBooleanAndProofConstants(t *testing.T) {
	for _, kind := range []int32{0, 1} {
		t.Run(map[int32]string{0: "Boolean", 1: "proof"}[kind], func(t *testing.T) {
			c := nla2BVRealPrefixInvocation(t)
			m := c.t.manager
			sort, err := m.MkFamilySort(m.BasicFamilyID(), kind, nil)
			equationMust(t, err)
			a, err := m.MkFreshConst("proof-a", sort)
			equationMust(t, err)
			fixDLPin(t, m, a)
			b, err := m.MkFreshConst("proof-b", sort)
			equationMust(t, err)
			fixDLPin(t, m, b)
			eq, err := m.MkEq(a, b)
			equationMust(t, err)
			fixDLPin(t, m, eq)
			x := fixDLConst(t, m, true, "ordinary-integer")
			g := fixDLGoal(t, m, false, false, false, eq, fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "0")))
			supported, err := c.collectVars(g)
			equationMust(t, err)
			if supported != (kind == 0) || len(c.vars) != 1 {
				t.Fatal("proof constants accepted as Boolean constants or integer prefix lost", supported, len(c.vars))
			}
		})
	}
}

func TestNLA2BVRealReductionRefusalKeepsCompletedGoalPrefix(t *testing.T) {
	c := nla2BVRealPrefixInvocation(t)
	m := c.t.manager
	a := bv2RealSymbolicEncoding(t, c.real, "prefix-a", 4, 3)
	b := bv2RealSymbolicEncoding(t, c.real, "prefix-b", 4, 3)
	first, err := m.MkEq(a, b)
	equationMust(t, err)
	fixDLPin(t, m, first)
	unencoded := fixDLConst(t, m, false, "refused-real")
	second, err := m.MkEq(a, unencoded)
	equationMust(t, err)
	fixDLPin(t, m, second)
	g := fixDLGoal(t, m, false, false, false, first, second)
	err = c.reduceReal(g)
	if err == nil || !strings.Contains(err.Error(), "nla2bv could not eliminate reals") {
		t.Fatal("missing source real-elimination refusal", err)
	}
	if len(c.local.refs) != 0 || len(c.local.numbers) != 0 {
		t.Fatal("refused reduction kept its local output owners")
	}
	equationMust(t, c.close())
	n, err := g.Size()
	equationMust(t, err)
	foundRefusal, foundChanged := false, false
	for i := uint32(0); i < n; i++ {
		h, err := g.Form(i)
		equationMust(t, err)
		if h == first {
			t.Fatal("completed earlier formula was rolled back")
		}
		if h == second {
			foundRefusal = true
		} else {
			foundChanged = true
			refs, err := m.GetRefCount(h)
			equationMust(t, err)
			if refs != 1 {
				t.Fatal("completed goal output still has helper owners", refs)
			}
		}
	}
	if !foundRefusal || !foundChanged {
		t.Fatal("refusal or completed prefix disappeared")
	}
}

func TestNLA2BVHelpersReleaseOutputsAndPreserveGoalOwnership(t *testing.T) {
	for _, mode := range []string{"substitute", "integer", "real"} {
		t.Run(mode, func(t *testing.T) {
			c := nla2BVRealPrefixInvocation(t)
			m := c.t.manager
			x := fixDLConst(t, m, true, "helper-variable")
			input := fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "0"))
			if mode == "substitute" {
				equationMust(t, m.ExprSubstitutionInsert(c.sub, x, fixDLNumber(t, m, true, "7"), ast.ASTHandle{}, ast.ASTDependency{}))
			} else if mode == "integer" {
				a := bv2RealSymbolicBV(t, m, "helper-a", 4)
				b := bv2RealSymbolicBV(t, m, "helper-b", 4)
				sum := bv2RealCallResult(t, c.real, func(local *bv2RealCall) ast.ASTHandle {
					return local.arith(6, local.hold(m.MkBitVectorToInt(a)), local.hold(m.MkBitVectorToInt(b)))
				})
				input = fixDLApp(t, m, 3, sum, fixDLNumber(t, m, true, "0"))
			} else {
				a := bv2RealSymbolicEncoding(t, c.real, "helper-a", 4, 3)
				b := bv2RealSymbolicEncoding(t, c.real, "helper-b", 4, 3)
				var err error
				input, err = m.MkEq(a, b)
				equationMust(t, err)
				fixDLPin(t, m, input)
			}
			g := fixDLGoal(t, m, false, false, false, input)
			outer := fixDLConst(t, m, true, "outer-helper-owner")
			c.local.hold(outer, nil)
			number := c.local.number()
			equationMust(t, c.local.err)
			equationMust(t, m.NumeralService().SetInt64(number, 37))
			var err error
			switch mode {
			case "substitute":
				err = c.substitute(g)
			case "integer":
				err = c.reduceInteger(g)
			case "real":
				err = c.reduceReal(g)
			}
			equationMust(t, err)
			if len(c.local.refs) != 1 || len(c.local.numbers) != 1 || c.replacer != nil || c.intRewrite != nil || c.realRewrite != nil {
				t.Fatal("helper output or traversal survived the local scope")
			}
			text, err := m.NumeralService().String(number)
			equationMust(t, err)
			if text != "37" {
				t.Fatal("helper destroyed an enclosing numeric owner")
			}
			equationMust(t, c.close())
			n, err := g.Size()
			equationMust(t, err)
			if n == 0 {
				t.Fatal("goal output disappeared")
			}
			for i := uint32(0); i < n; i++ {
				h, err := g.Form(i)
				equationMust(t, err)
				refs, err := m.GetRefCount(h)
				equationMust(t, err)
				if h == input || refs != 1 {
					t.Fatal("goal was not rewritten or helper output still has owners", refs)
				}
			}
		})
	}
}
