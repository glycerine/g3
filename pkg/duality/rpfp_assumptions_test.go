package duality

import "testing"

func TestRPFPUnderapproximationFlagOwnershipAndReverseLookup(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	relation, err := ctx.BoolConst("flag-R")
	must(err)
	closeExpr(t, relation)
	unknown, err := ctx.BoolConst("unknown-flag")
	must(err)
	closeExpr(t, unknown)
	before := m.Stats()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	first, err := g.CreateNode(relation)
	must(err)
	second, err := g.CreateNode(relation)
	must(err)
	for _, node := range []*Node{first, second} {
		flag, err := g.UnderapproxFlag(node)
		must(err)
		decl, err := flag.Decl()
		must(err)
		name, err := decl.Name()
		must(err)
		text, err := name.String()
		must(err)
		expected := "@under1"
		if node == second {
			expected = "@under2"
		}
		if text != expected {
			t.Fatal("wrong source flag name", text)
		}
		must(decl.Close())
		h, err := flag.Raw()
		must(err)
		must(flag.Close())
		// The graph owns the lookup key even after its caller drops the flag.
		snapshot := m.Stats()
		for n := 0; n < 3; n++ {
			copy, err := g.UnderapproxFlag(node)
			must(err)
			got, err := copy.Raw()
			must(err)
			if got != h {
				t.Fatal("flag identity changed")
			}
			reverse, err := g.UnderapproxFlagRev(copy)
			must(err)
			if reverse != node {
				t.Fatal("wrong reverse node")
			}
			must(copy.Close())
		}
		after := m.Stats()
		if after.LiveNodes != snapshot.LiveNodes || after.LiveReferences != snapshot.LiveReferences {
			t.Fatal("repeated flags leaked", snapshot, after)
		}
	}
	absent, err := g.UnderapproxFlagRev(unknown)
	must(err)
	if absent != nil {
		t.Fatal("unknown flag resolved")
	}
	other, err := NewRPFP(ctx)
	must(err)
	defer func() { must(other.Close()) }()
	if _, err = other.UnderapproxFlag(first); err == nil {
		t.Fatal("foreign graph node accepted")
	}
	must(other.Close())
	must(g.Close())
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("flag map leaked", before, after)
	}
}
