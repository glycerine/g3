package cmd_context

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/duality"
)

// A rule can split during Horn normalization, but Duality consumes the original
// formula and its bound together. Exercise that boundary through command input.
func TestFixedpointCommandBoundsReachDualityPreparation(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	var decls, atoms []ast.ASTHandle
	for _, name := range []string{"bounds-P", "bounds-Q", "bounds-R"} {
		d, x := c.DeclareRelation(name, nil)
		contextMust(t, x)
		a, x := m.MkApp(d, nil)
		contextMust(t, x)
		decls = append(decls, d)
		atoms = append(atoms, a)
	}
	body, err := m.MkOr(atoms[0], atoms[1])
	contextMust(t, err)
	rule, err := m.MkImplies(body, atoms[2])
	contextMust(t, err)
	contextMust(t, c.AddFixedpointRuleWithBound(rule, "split", 7))
	contextMust(t, c.AddFixedpointRule(atoms[0], "default"))
	engine, err := c.fixedpointContext()
	contextMust(t, err)
	check := func(want []uint32) {
		t.Helper()
		forms, names, bounds, x := engine.RawRules()
		contextMust(t, x)
		defer func() {
			for i := range forms {
				contextMust(t, m.DelRef(&forms[i]))
			}
		}()
		if !reflect.DeepEqual(bounds, want) {
			t.Fatalf("raw bounds %v, want %v (names %v)", bounds, want, names)
		}
		handles := make([]ast.ASTHandle, len(forms))
		for i := range forms {
			handles[i], x = m.RefGet(forms[i])
			contextMust(t, x)
		}
		prepared, x := duality.PrepareQuery(m, handles, bounds, atoms[2], decls, 11)
		contextMust(t, x)
		defer func() { contextMust(t, prepared.Close()) }()
		actual, x := prepared.Bounds()
		contextMust(t, x)
		for i, bound := range want {
			if bound == ^uint32(0) {
				bound = 11
			}
			if actual[i] != bound {
				t.Fatalf("prepared bound[%d] = %d, want %d", i, actual[i], bound)
			}
		}
	}
	check([]uint32{7, ^uint32(0)})
	contextMust(t, c.PushFixedpoint())
	contextMust(t, c.AddFixedpointRuleWithBound(atoms[1], "scoped-zero", 0))
	check([]uint32{7, ^uint32(0), 0})
	contextMust(t, c.PopFixedpoint())
	check([]uint32{7, ^uint32(0)})
}
