package ast

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func astTestMust(t testing.TB, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func astTestOwners(t testing.TB) (*usymbol.Manager, *unumeral.RationalService, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(1 << 30)
	s, e := usymbol.NewManager(usymbol.Config{Budget: b})
	astTestMust(t, e)
	q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	astTestMust(t, e)
	t.Cleanup(func() {
		astTestMust(t, q.Close())
		astTestMust(t, s.Close())
		if b.Used() != 0 {
			t.Errorf("owner graph retained %d bytes", b.Used())
		}
	})
	return s, q, b
}
func astTestManager(t testing.TB) *ASTManager {
	t.Helper()
	s, q, b := astTestOwners(t)
	m, e := NewASTManager(ASTManagerConfig{Symbols: s, Numerals: q, Budget: b})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	return m
}
func astTestSymbol(t testing.TB, m *ASTManager, text string) usymbol.Symbol {
	t.Helper()
	s, e := m.Symbols.Intern(text)
	astTestMust(t, e)
	return s
}
func astTestPin(t testing.TB, m *ASTManager, h ASTHandle) ASTRef {
	t.Helper()
	r, e := m.NewRef(h)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&r)) })
	return r
}
