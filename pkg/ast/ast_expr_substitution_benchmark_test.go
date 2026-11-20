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
	for offset, name := range []string{"Hit", "Miss"} {
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
}
