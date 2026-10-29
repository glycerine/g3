package rel_test

import (
	"bytes"
	. "github.com/glycerine/g3/pkg/muz/rel"
	"testing"
)

func TestAAGTransitionLatchOutputAndLifecycle(t *testing.T) {
	m := relationASTManager(t)
	sort, e := m.MkBoolSort()
	must(t, e)
	current, e := m.MkVar(10, sort)
	must(t, e)
	next, e := m.MkVar(11, sort)
	must(t, e)
	b, e := NewAAGBuilder(m)
	must(t, e)
	tr, e := b.FreshInput()
	must(t, e)
	must(t, b.AddTransitionLatch(current, next, tr))
	must(t, b.AddOutput(tr))
	var out bytes.Buffer
	must(t, b.Write(&out))
	i, l, o, g, e := b.Counts()
	must(t, e)
	if i != 2 || l != 1 || o != 1 || g != 3 {
		t.Fatal(i, l, o, g, out.String())
	}
	must(t, b.Close())
	must(t, b.Close())
	if e = b.Write(&out); e == nil {
		t.Fatal("closed builder")
	}
}
