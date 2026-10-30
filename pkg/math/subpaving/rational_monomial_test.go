package subpaving

import (
	"math"
	"testing"
)

func TestRationalMonomialRegistrationAndForwardBounds(t *testing.T) {
	c, numbers, value, x, y := rationalClauseFixture(t)
	z, err := c.DefineMonomial([]RationalPower{{Variable: y, Degree: 1}, {Variable: x, Degree: 1}, {Variable: x, Degree: 1}})
	if err != nil {
		t.Fatal(err)
	}
	m := c.monomials[z]
	if z != 2 || c.variables[z] || m == nil || len(m.powers) != 2 || m.powers[0] != (RationalPower{x, 2}) || m.powers[1] != (RationalPower{y, 1}) || len(c.watchOrder[x]) != 1 || len(c.watchOrder[y]) != 1 {
		t.Fatal("source sorted/merged monomial and distinct watches")
	}
	unit := func(variable uint32, n int32) {
		t.Helper()
		if err := numbers.SetInt32(value, n); err != nil {
			t.Fatal(err)
		}
		for _, lower := range []bool{true, false} {
			if _, err := c.AddInequality(variable, value, lower, false, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	unit(x, 2)
	unit(y, 3)
	if err := c.InitializeRoot(); err != nil {
		t.Fatal(err)
	}
	root := c.root
	if root.conflictVar != math.MaxUint32 || root.lower[z] == nil || root.upper[z] == nil || root.lower[z].reasonDefinition != z || root.upper[z].reasonDefinition != z {
		t.Fatal("forward monomial bounds or justification")
	}
	if err := numbers.SetInt32(value, 12); err != nil {
		t.Fatal(err)
	}
	for _, b := range []*RationalBound{root.lower[z], root.upper[z]} {
		cmp, err := numbers.Compare(&b.value, value)
		if err != nil || cmp != 0 {
			t.Fatal("wrong forward product", cmp, err)
		}
	}
	visits := c.numVisited
	if err := c.InitializeRoot(); err != nil || c.numVisited != visits {
		t.Fatal("completed initialization replayed", err)
	}
}

func TestRationalMonomialReverseRootAndZeroShortcut(t *testing.T) {
	for _, zero := range []bool{false, true} {
		name := "reverse"
		if zero {
			name = "upper-zero"
		}
		t.Run(name, func(t *testing.T) {
			c, numbers, value, x, _ := rationalClauseFixture(t)
			z, err := c.DefineMonomial([]RationalPower{{Variable: x, Degree: 2}})
			if err != nil {
				t.Fatal(err)
			}
			variable, number := z, int32(4)
			if zero {
				variable, number = x, 0
			}
			if err := numbers.SetInt32(value, number); err != nil {
				t.Fatal(err)
			}
			if zero {
				if _, err := c.AddInequality(variable, value, false, false, true); err != nil {
					t.Fatal(err)
				}
			} else {
				for _, lower := range []bool{true, false} {
					if _, err := c.AddInequality(variable, value, lower, false, true); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := c.InitializeRoot(); err != nil {
				t.Fatal(err)
			}
			root := c.root
			target := x
			if zero {
				target = z
			}
			if root.conflictVar != math.MaxUint32 || root.lower[target] == nil || root.upper[target] == nil || root.lower[target].reasonDefinition != z || root.upper[target].reasonDefinition != z {
				t.Fatal("reverse root or zero shortcut failed", root.conflictVar)
			}
			wantLower, wantUpper := int32(-2), int32(2)
			if zero {
				wantLower, wantUpper = 0, 0
			}
			for _, check := range []struct {
				b    *RationalBound
				want int32
			}{{root.lower[target], wantLower}, {root.upper[target], wantUpper}} {
				if err := numbers.SetInt32(value, check.want); err != nil {
					t.Fatal(err)
				}
				cmp, err := numbers.Compare(&check.b.value, value)
				if err != nil || cmp != 0 {
					t.Fatal("wrong inverse bound", cmp, err)
				}
			}
		})
	}
}

func TestRationalMonomialIntegerClassification(t *testing.T) {
	c, _, _, x, _ := rationalClauseFixture(t)
	c.variables[x] = true
	z, err := c.DefineMonomial([]RationalPower{{Variable: 1, Degree: 1}, {Variable: x, Degree: 1}})
	if err != nil || !c.variables[z] {
		t.Fatal("source any-integer factor rule", err)
	}
	zero, err := c.DefineMonomial([]RationalPower{{Variable: x, Degree: 0}})
	if err != nil || c.monomials[zero].powers[0].Degree != 0 {
		t.Fatal("source mk_monomial accepts a zero degree", zero, err)
	}
	wrapped, err := c.DefineMonomial([]RationalPower{{Variable: x, Degree: math.MaxUint32}, {Variable: x, Degree: 1}})
	if err != nil || c.monomials[wrapped].powers[0].Degree != 0 {
		t.Fatal("source unsigned degree merge preserves zero wrap", wrapped, err)
	}
}

func TestRationalMonomialQueuedClauseBoundTriggersWatch(t *testing.T) {
	c, numbers, value, x, y := rationalClauseFixture(t)
	z, err := c.DefineMonomial([]RationalPower{{Variable: x, Degree: 1}, {Variable: y, Degree: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := numbers.SetInt32(value, 1); err != nil {
		t.Fatal(err)
	}
	xLower, err := c.NewInequality(x, value, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := numbers.SetInt32(value, 3); err != nil {
		t.Fatal(err)
	}
	yLower, err := c.NewInequality(y, value, true, false)
	if err != nil {
		t.Fatal(err)
	}
	clause, err := c.AddClause([]*RationalInequality{yLower, xLower})
	if err != nil {
		t.Fatal(err)
	}
	if err := numbers.SetInt32(value, 2); err != nil {
		t.Fatal(err)
	}
	for _, lower := range []bool{true, false} {
		if _, err := c.AddInequality(y, value, lower, false, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.InitializeRoot(); err != nil {
		t.Fatal(err)
	}
	root := c.root
	if root.conflictVar != math.MaxUint32 || root.lower[x] == nil || root.lower[x].reasonClause != clause || root.lower[z] == nil || root.lower[z].reasonDefinition != z || root.upper[z] != nil || len(c.queue) != 0 {
		t.Fatal("clause bound did not wake the watched monomial")
	}
	cmp, err := numbers.Compare(&root.lower[z].value, value)
	if err != nil || cmp != 0 {
		t.Fatal("wrong watched product lower bound", cmp, err)
	}
}
