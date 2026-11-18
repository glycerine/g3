package cmd_context

import "testing"

// AUDIT.md cmd_context_02-F14: declare-sort inserts mk_psort_user_decl(0,
// id, nullptr); the uninterpreted sort is made at its first use.
func TestDeclareSortInstantiatesOnFirstUse(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	_, err = c.DeclareSortConstructor("U", 0)
	contextMust(t, err)
	integer, err := c.LookupSort("Int")
	contextMust(t, err)
	x, err := c.DeclareFunction("x", nil, integer)
	contextMust(t, err)
	u, err := c.LookupSort("U")
	contextMust(t, err)
	again, err := c.LookupSort("U")
	contextMust(t, err)
	if again != u {
		t.Fatal("second use made a new sort")
	}
	xID, err := m.GetASTID(x)
	contextMust(t, err)
	uID, err := m.GetASTID(u)
	contextMust(t, err)
	if uID <= xID {
		t.Fatalf("sort U id %d precedes the earlier declaration x %d", uID, xID)
	}
}
