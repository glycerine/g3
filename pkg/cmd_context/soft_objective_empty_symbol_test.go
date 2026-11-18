package cmd_context

import "testing"

// AUDIT.md api_03-F6: scoped_state::m_indices keys soft groups by symbol,
// so symbol::null and the non-null symbol("") are two groups.
func TestSoftConstraintEmptySymbolIsItsOwnGroup(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	p, err := c.DeclareFunction("p", nil, boolean)
	contextMust(t, err)
	a, err := m.MkApp(p, nil)
	contextMust(t, err)
	null, err := c.AddSoftConstraintIndexed(a, softRat(1, 1), "")
	contextMust(t, err)
	empty, err := c.AddSoftConstraintEmptySymbolIndexed(a, softRat(1, 1))
	contextMust(t, err)
	again, err := c.AddSoftConstraintEmptySymbolIndexed(a, softRat(2, 1))
	contextMust(t, err)
	if null == empty || empty != again || len(c.objectives) != 2 {
		t.Fatalf("groups null=%d empty=%d again=%d objectives=%d", null, empty, again, len(c.objectives))
	}
}
