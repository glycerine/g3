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

// One cycle holds both ends of each cache entry outside the cache, as a
// rewriter does when a retained AST survives a reset.
func BenchmarkActivityCacheCycle(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	keys := make([]ASTHandle, 32)
	for i := range keys {
		keys[i], err = m.MkVar(uint32(i), sort)
		astTestMust(b, err)
		astTestPin(b, m, keys[i])
	}
	c, err := NewActivityCache(m, nil)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, c.Close()) })
	insert := func() {
		for i, k := range keys {
			if err := c.Insert(k, keys[(i+1)%len(keys)]); err != nil {
				b.Fatal(err)
			}
		}
	}
	insert()
	b.Run("Find", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			i := n % len(keys)
			if got, err := c.Find(keys[i]); err != nil || got != keys[(i+1)%len(keys)] {
				b.Fatal(got, err)
			}
		}
	})
	b.Run("InsertReset", func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			if err := c.Reset(); err != nil {
				b.Fatal(err)
			}
			insert()
		}
	})
}
