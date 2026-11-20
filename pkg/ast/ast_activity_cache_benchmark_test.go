package ast

import "testing"

// Many short-lived substitution engines close their cache without inserting.
// Include a first insertion to measure the cost when the cache is used.
func BenchmarkASTActivityCacheLifetime(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	key, err := m.MkVar(0, sort)
	astTestMust(b, err)
	astTestPin(b, m, key)
	for _, insert := range []bool{false, true} {
		name := "Empty"
		if insert {
			name = "OneEntry"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				c, err := NewActivityCache(m, nil)
				if err != nil {
					b.Fatal(err)
				}
				var want ASTHandle
				if insert {
					want = key
					if err := c.Insert(key, key); err != nil {
						b.Fatal(err)
					}
				}
				if got, err := c.Find(key); err != nil || got != want {
					b.Fatal("cache lookup", got, err)
				}
				if err := c.Close(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
