package rel_test

import (
	. "github.com/glycerine/g3/pkg/muz/rel"
	"testing"
)

func TestAAGGateCanonicalizationConstantsAndFreshInputs(t *testing.T) {
	m := relationASTManager(t)
	b, e := NewAAGBuilder(m)
	must(t, e)
	defer b.Close()
	x, e := b.FreshInput()
	must(t, e)
	y, e := b.FreshInput()
	must(t, e)
	a, e := b.And(x, y)
	must(t, e)
	same, e := b.And(y, x)
	must(t, e)
	if a != same || AAGNegate(AAGNegate(a)) != a {
		t.Fatal(a, same)
	}
	empty, e := b.AndMany(nil)
	must(t, e)
	if empty != AAGTrue() {
		t.Fatal(empty)
	}
	or, e := b.Or(x, y)
	must(t, e)
	if or == a {
		t.Fatal(or)
	}
	i, _, _, g, e := b.Counts()
	must(t, e)
	if i != 2 || g != 2 {
		t.Fatal(i, g)
	}
}
