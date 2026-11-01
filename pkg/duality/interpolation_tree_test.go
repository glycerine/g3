package duality

import "testing"

func TestInterpolationTreePartitionOrderAndReconstruction(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var terms []*Expr
	for _, name := range []string{"partition-a", "partition-b", "partition-root", "partition-extra"} {
		term, err := ctx.BoolConst(name)
		must(err)
		closeExpr(t, term)
		terms = append(terms, term)
	}
	before := m.Stats()
	left, err := NewTermTree(terms[0], nil)
	must(err)
	right, err := NewTermTree(terms[1], nil)
	must(err)
	must(right.AddTerm(terms[3]))
	root, err := NewTermTree(terms[2], []*TermTree{left, right})
	must(err)
	parts, parents, err := LinearizeAssumptions(root)
	must(err)
	if len(parts) != 3 || len(parts[1]) != 2 || parents[0] != 2 || parents[1] != 2 || parents[2] != 32767 {
		t.Fatal("partition shape", parents)
	}
	for n, refs := range parts {
		got, err := m.RefGet(refs[0])
		must(err)
		want, err := terms[n].Raw()
		must(err)
		if got != want {
			t.Fatal("partition term order", n)
		}
	}
	extra, err := m.RefGet(parts[1][1])
	must(err)
	want, err := terms[3].Raw()
	must(err)
	if extra != want {
		t.Fatal("extra term missing")
	}
	rebuilt, err := UnlinearizeInterpolants(root, []*Expr{terms[1], terms[0]})
	must(err)
	h, err := rebuilt.term.Raw()
	must(err)
	if !m.IsFalse(h) {
		t.Fatal("root interpolant is not false")
	}
	if len(rebuilt.children) != 2 || len(rebuilt.children[1].terms) != 0 {
		t.Fatal("wrong reconstructed shape")
	}
	for n, term := range []*Expr{terms[1], terms[0]} {
		got, err := rebuilt.children[n].term.Raw()
		must(err)
		want, err := term.Raw()
		must(err)
		if got != want {
			t.Fatal("interpolant order", n)
		}
	}
	if _, err = UnlinearizeInterpolants(root, nil); err == nil {
		t.Fatal("incomplete interpolants accepted")
	}
	must(root.Close())
	// Both representations own their ASTs independently of the source tree.
	_, err = m.RefGet(parts[0][0])
	must(err)
	must(rebuilt.Close())
	must(CloseLinearizedAssumptions(ctx, &parts))
	must(CloseLinearizedAssumptions(ctx, &parts))
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("tree conversion leaked", before, after)
	}
}
