package rewriter

import (
	"strconv"
	"testing"
)

// A chain of unshared if-then-else terms with Boolean values at its leaves.
func BenchmarkTheoryITEValueTree(b *testing.B) {
	for _, depth := range []int{0, 1, 16, 128} {
		b.Run(strconv.Itoa(depth), func(b *testing.B) {
			m, context, globals, local := boolRewriterConfigFixture(b)
			r, err := NewTheoryRewriter(m, context, globals, local)
			astTestMust(b, err)
			b.Cleanup(func() { astTestMust(b, r.Close()) })
			sort, err := m.MkBoolSort()
			astTestMust(b, err)
			root, err := m.MkFalse()
			astTestMust(b, err)
			leaf, err := m.MkTrue()
			astTestMust(b, err)
			for i := 0; i < depth; i++ {
				cond, err := m.MkConst(astTestSymbol(b, m, "cond-"+strconv.Itoa(i)), sort)
				astTestMust(b, err)
				root, err = m.MkIte(cond, root, leaf)
				astTestMust(b, err)
			}
			astTestPin(b, m, root)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				found, err := r.configuration.iteValueTree(root)
				if err != nil || found != (depth > 0) {
					b.Fatal(found, err)
				}
			}
		})
	}
}

func TestTheoryITEValueTreeSharedBranch(t *testing.T) {
	m, context, globals, local := boolRewriterConfigFixture(t)
	r, err := NewTheoryRewriter(m, context, globals, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	leaf, err := m.MkTrue()
	astTestMust(t, err)
	other, err := m.MkFalse()
	astTestMust(t, err)
	branch, err := m.MkIte(goalTestAtom(t, m, "inner"), leaf, other)
	astTestMust(t, err)
	root, err := m.MkIte(goalTestAtom(t, m, "outer"), branch, other)
	astTestMust(t, err)
	astTestPin(t, m, root)
	check := func(want bool) {
		t.Helper()
		got, err := r.configuration.iteValueTree(root)
		if err != nil || got != want {
			t.Fatalf("iteValueTree = %v, %v; want %v", got, err, want)
		}
	}
	check(true)
	extra, err := m.NewRef(branch)
	astTestMust(t, err)
	check(false)
	astTestMust(t, m.DelRef(&extra))
	check(true)
}
