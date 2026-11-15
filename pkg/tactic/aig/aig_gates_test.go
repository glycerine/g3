package aig

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestAIGDefaultGatesTruthTablesAndFailureCleanup(t *testing.T) {
	m, budget := aigFixture(t)
	h, err := m.MkTrue()
	aigMust(t, err)
	g, err := NewManager(m)
	aigMust(t, err)
	a, err := g.MakeVariable(h)
	aigMust(t, err)
	b, err := g.MakeVariable(h)
	aigMust(t, err)
	c, err := g.MakeVariable(h)
	aigMust(t, err)
	inputs := []*Ref{a, b, c}
	for _, left := range inputs {
		for _, right := range inputs {
			for _, operation := range []struct {
				name  string
				build func(*Ref, *Ref) (*Ref, error)
				want  func(bool, bool) bool
			}{
				{"or", g.Or, func(x, y bool) bool { return x || y }},
				{"iff", g.Iff, func(x, y bool) bool { return x == y }},
				{"xor", g.Xor, func(x, y bool) bool { return x != y }},
				{"implies", g.Implies, func(x, y bool) bool { return !x || y }},
			} {
				out, e := operation.build(left, right)
				aigMust(t, e)
				for mask := uint32(0); mask < 8; mask++ {
					want := operation.want(aigEval(left.lit, mask), aigEval(right.lit, mask))
					if got := aigEval(out.lit, mask); got != want {
						t.Fatalf("%s mask %03b: %v want %v", operation.name, mask, got, want)
					}
				}
				aigMust(t, out.Close())
			}
		}
	}
	for _, condition := range inputs {
		for _, then := range inputs {
			for _, otherwise := range inputs {
				out, e := g.Ite(condition, then, otherwise)
				aigMust(t, e)
				for mask := uint32(0); mask < 8; mask++ {
					want := aigEval(otherwise.lit, mask)
					if aigEval(condition.lit, mask) {
						want = aigEval(then.lit, mask)
					}
					if got := aigEval(out.lit, mask); got != want {
						t.Fatalf("ite mask %03b: %v want %v", mask, got, want)
					}
				}
				aigMust(t, out.Close())
			}
		}
	}
	before := budget.Used()
	for _, after := range []int64{0, 1, 2} {
		budget.FailAfter(after)
		if out, e := g.Iff(a, b); !errors.Is(e, arena.ErrInjectedFailure) || out != nil {
			t.Fatalf("failure after %d: %v, %v", after, e, out)
		}
		budget.FailAfter(-1)
		if budget.Used() != before {
			t.Fatalf("failure after %d retained %d bytes", after, budget.Used()-before)
		}
	}
	aigMust(t, c.Close())
	aigMust(t, b.Close())
	aigMust(t, a.Close())
	aigMust(t, g.Close())
}

func TestAIGNarySortedFolds(t *testing.T) {
	m, _ := aigFixture(t)
	h, e := m.MkTrue()
	aigMust(t, e)
	g, e := NewManager(m)
	aigMust(t, e)
	a, e := g.MakeVariable(h)
	aigMust(t, e)
	b, e := g.MakeVariable(h)
	aigMust(t, e)
	c, e := g.MakeVariable(h)
	aigMust(t, e)
	forward := []*Ref{a, b, c}
	reverse := []*Ref{c, b, a}
	for _, operation := range []struct {
		name  string
		build func([]*Ref) (*Ref, error)
		want  func(bool, bool, bool) bool
	}{
		{"and", g.AndArgs, func(x, y, z bool) bool { return x && y && z }},
		{"or", g.OrArgs, func(x, y, z bool) bool { return x || y || z }},
	} {
		first, e := operation.build(forward)
		aigMust(t, e)
		second, e := operation.build(reverse)
		aigMust(t, e)
		if !first.Same(second) {
			t.Fatalf("%s permutation changed sorted fold", operation.name)
		}
		for mask := uint32(0); mask < 8; mask++ {
			want := operation.want(aigEval(a.lit, mask), aigEval(b.lit, mask), aigEval(c.lit, mask))
			if got := aigEval(first.lit, mask); got != want {
				t.Fatalf("%s mask %03b: %v want %v", operation.name, mask, got, want)
			}
		}
		if forward[0] != a || reverse[0] != c {
			t.Fatal("caller arguments mutated")
		}
		aigMust(t, second.Close())
		aigMust(t, first.Close())
	}
	zeroAnd, e := g.AndArgs(nil)
	aigMust(t, e)
	zeroOr, e := g.OrArgs(nil)
	aigMust(t, e)
	for mask := uint32(0); mask < 8; mask++ {
		if !aigEval(zeroAnd.lit, mask) || aigEval(zeroOr.lit, mask) {
			t.Fatal("empty fold identity")
		}
	}
	one, e := g.AndArgs([]*Ref{a})
	aigMust(t, e)
	if !one.Same(a) {
		t.Fatal("singleton fold did not reuse node")
	}
	aigMust(t, one.Close())
	aigMust(t, zeroOr.Close())
	aigMust(t, zeroAnd.Close())
	aigMust(t, c.Close())
	aigMust(t, b.Close())
	aigMust(t, a.Close())
	aigMust(t, g.Close())
}

// AUDIT.md tactic_small_01-F1: mk_ite dec_refs n1 and then n2; id_gen
// recycles freed ids LIFO, so the next node reuses n2's id.
func TestAIGIteReleasesTemporariesInSourceOrder(t *testing.T) {
	m, _ := aigFixture(t)
	h, err := m.MkTrue()
	aigMust(t, err)
	g, err := NewManager(m)
	aigMust(t, err)
	t.Cleanup(func() { aigMust(t, g.Close()) })
	vars := make([]*Ref, 4)
	for i := range vars {
		vars[i], err = g.MakeVariable(h)
		aigMust(t, err)
	}
	q, x := vars[0], vars[1]
	// Free one node id so n1 reuses it and n2 takes a fresh id.
	probe, err := g.InternAnd(vars[2], vars[3])
	aigMust(t, err)
	n1ID, _, err := probe.ID()
	aigMust(t, err)
	aigMust(t, probe.Close())
	out, err := g.Ite(q, x, x)
	aigMust(t, err)
	if !out.Same(x) {
		t.Fatal("ite(q, x, x) did not reduce to x")
	}
	aigMust(t, out.Close())
	next, err := g.InternAnd(vars[2], vars[3])
	aigMust(t, err)
	nextID, _, err := next.ID()
	aigMust(t, err)
	aigMust(t, next.Close())
	if nextID == n1ID {
		t.Fatal("n2 was released before n1: reused id", nextID)
	}
	for _, v := range vars {
		aigMust(t, v.Close())
	}
}

// AUDIT.md tactic_small_01-F2: mk_and(unsigned, aig_lit*) does not sort two
// operands, and mk_node is not symmetric.
func TestAIGBinaryAndKeepsOperandOrder(t *testing.T) {
	m, _ := aigFixture(t)
	h, err := m.MkTrue()
	aigMust(t, err)
	g, err := NewManager(m)
	aigMust(t, err)
	t.Cleanup(func() { aigMust(t, g.Close()) })
	x, err := g.MakeVariable(h)
	aigMust(t, err)
	y, err := g.MakeVariable(h)
	aigMust(t, err)
	z, err := g.MakeVariable(h)
	aigMust(t, err)
	n1, err := g.And(x, y)
	aigMust(t, err)
	n2, err := g.And(x, z)
	aigMust(t, err)
	given, err := g.And(n2, n1)
	aigMust(t, err)
	swapped, err := g.And(n1, n2)
	aigMust(t, err)
	if given.Same(swapped) {
		t.Fatal("mk_node was symmetric for this input; the test needs asymmetric operands")
	}
	got, err := g.AndArgs([]*Ref{n2, n1})
	aigMust(t, err)
	if !got.Same(given) {
		t.Fatal("binary AndArgs reordered its operands")
	}
	for _, r := range []*Ref{got, swapped, given, n2, n1, z, y, x} {
		aigMust(t, r.Close())
	}
}
