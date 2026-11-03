package base_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleSetGroupedPredicatesSourceBucketOrder(t *testing.T) {
	names := make([]string, 16)
	for i := range names {
		names[i] = fmt.Sprintf("bucket-%d", i)
	}
	f := newRuleSetFixture(t, names...)
	first, second := "", ""
	var hFirst, hSecond uint32
	for _, a := range names {
		ha, err := f.manager.GetHash(f.decl[a])
		ruleMust(t, err)
		for _, b := range names {
			hb, err := f.manager.GetHash(f.decl[b])
			ruleMust(t, err)
			if ha&7 > hb&7 {
				first, second, hFirst, hSecond = a, b, ha, hb
				break
			}
		}
		if first != "" {
			break
		}
	}
	if first == "" {
		t.Fatal("fixture needs distinct buckets")
	}
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	r1, r2 := f.rule(first, nil, nil), f.rule(second, nil, nil)
	ruleMust(t, set.Add(r1))
	ruleMust(t, set.Add(r2))
	got, err := set.GroupedPredicates()
	ruleMust(t, err)
	if len(got) != 2 || got[0] != f.decl[second] || got[1] != f.decl[first] {
		t.Fatalf("source bucket order: %v; hashes %d %d", got, hFirst, hSecond)
	}
	ruleMust(t, set.Remove(r2))
	got, err = set.GroupedPredicates()
	ruleMust(t, err)
	if len(got) != 2 || got[0] != f.decl[second] || got[1] != f.decl[first] {
		t.Fatal("source keeps empty group after removal", got)
	}
	ruleMust(t, r1.Close())
	ruleMust(t, r2.Close())
}
