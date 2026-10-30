package arith

// Source: src/tactic/arith/fm_tactic.cpp:196-203 and converter ref ownership.
import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestFMModelConverterOwnsInsertedStateAndReferenceLifetime(t *testing.T) {
	c, m := fmModelFixture(t)
	x := fixDLConst(t, m, false, "fm-life-x")
	zero := fixDLNumber(t, m, false, "0")
	bound, e := m.MkArithmeticLE(x, zero)
	equationMust(t, e)
	d, e := m.GetDecl(x)
	equationMust(t, e)
	before, e := m.GetRefCount(bound)
	equationMust(t, e)
	equationMust(t, c.Insert(d, []ast.ASTHandle{bound}))
	after, e := m.GetRefCount(bound)
	equationMust(t, e)
	if after != before+1 {
		t.Fatal("clause not retained", before, after)
	}
	equationMust(t, c.IncRef())
	consumed, e := c.DecRefConsumed()
	equationMust(t, e)
	if !consumed || !c.closed {
		t.Fatal("last reference did not close", consumed, c.closed)
	}
	if e = c.open(); !errors.Is(e, tactic.ErrModelConverterOwner) {
		t.Fatal("closed converter accepted", e)
	}
}
