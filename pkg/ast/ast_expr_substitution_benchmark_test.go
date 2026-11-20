package ast

import "testing"

// Rewriting asks one substitution about many different expressions in a row.
func BenchmarkExprSubstitutionFind(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	subst, err := m.NewExprSubstitutionPolicy(false, false)
	astTestMust(b, err)
	const size = 1024
	var keys [size]ASTHandle
	for i := range keys {
		key, err := m.MkVar(uint32(i), sort)
		astTestMust(b, err)
		astTestPin(b, m, key)
		keys[i] = key
		if i%2 == 0 {
			astTestMust(b, m.ExprSubstitutionInsert(subst, key, key, ASTHandle{}, ASTDependency{}))
		}
	}
	b.Cleanup(func() { astTestMust(b, m.DelExprSubstitution(&subst)) })
	find := func(name string, offset int) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			var def, proof ASTHandle
			var dep ASTDependency
			for i := 0; i < b.N; i++ {
				key := keys[(2*i+offset)%size]
				found, err := m.ExprSubstitutionFindDependency(subst, key, &def, &proof, &dep)
				if err != nil || found != (offset == 0) || found && def != key {
					b.Fatal(found, err)
				}
			}
		})
	}
	find("Hit", 0)
	find("Miss", 1)

	other, err := m.NewExprSubstitutionPolicy(false, false)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, m.DelExprSubstitution(&other)) })
	for i := 1; i < size; i += 2 {
		astTestMust(b, m.ExprSubstitutionInsert(other, keys[i], keys[i], ASTHandle{}, ASTDependency{}))
	}
	find("KeyInOtherTable", 1)
}
