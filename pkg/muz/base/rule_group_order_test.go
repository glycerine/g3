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

func TestRuleSetGroupedPredicateResetPreservesSourceCapacity(t *testing.T) {
	names := make([]string, 48)
	for i := range names {
		names[i] = fmt.Sprintf("reset-bucket-%d", i)
	}
	f := newRuleSetFixture(t, names...)
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	// Eight unique insertions grow the source table from eight to sixteen slots.
	for _, name := range names[:8] {
		r := f.rule(name, nil, nil)
		ruleMust(t, set.Add(r))
		ruleMust(t, r.Close())
	}
	ruleMust(t, set.Reset())
	// A second empty reset must also preserve the source table's capacity.
	ruleMust(t, set.Reset())
	first, second := "", ""
	for _, a := range names {
		ha, e := f.manager.GetHash(f.decl[a])
		ruleMust(t, e)
		for _, b := range names {
			hb, e := f.manager.GetHash(f.decl[b])
			ruleMust(t, e)
			if ha&15 > hb&15 && ha&7 < hb&7 {
				first, second = a, b
				break
			}
		}
		if first != "" {
			break
		}
	}
	if first == "" {
		t.Fatal("fixture needs opposite eight- and sixteen-slot ordering")
	}
	for _, name := range []string{first, second} {
		f.app[name], err = f.manager.MkApp(f.decl[name], nil)
		ruleMust(t, err)
		r := f.rule(name, nil, nil)
		ruleMust(t, set.Add(r))
		ruleMust(t, r.Close())
	}
	got, err := set.GroupedPredicates()
	ruleMust(t, err)
	if len(got) != 2 || got[0] != f.decl[second] || got[1] != f.decl[first] {
		t.Fatal("reset did not preserve source sixteen-slot order", got)
	}
	// Growing to 32 slots and resetting a dense table keeps its capacity. A
	// subsequent reset with one entry shrinks it to 16, exactly once.
	ruleMust(t, set.Reset())
	for _, name := range names[:14] {
		f.app[name], err = f.manager.MkApp(f.decl[name], nil)
		ruleMust(t, err)
		r := f.rule(name, nil, nil)
		ruleMust(t, set.Add(r))
		ruleMust(t, r.Close())
	}
	ruleMust(t, set.Reset())
	f.app[names[0]], err = f.manager.MkApp(f.decl[names[0]], nil)
	ruleMust(t, err)
	r := f.rule(names[0], nil, nil)
	ruleMust(t, set.Add(r))
	ruleMust(t, r.Close())
	ruleMust(t, set.Reset())
	first, second = "", ""
	for _, a := range names {
		ha, e := f.manager.GetHash(f.decl[a])
		ruleMust(t, e)
		for _, b := range names {
			hb, e := f.manager.GetHash(f.decl[b])
			ruleMust(t, e)
			if ha&31 > hb&31 && ha&15 < hb&15 {
				first, second = a, b
				break
			}
		}
		if first != "" {
			break
		}
	}
	if first == "" {
		t.Fatal("fixture needs opposite sixteen- and thirty-two-slot ordering")
	}
	for _, name := range []string{first, second} {
		f.app[name], err = f.manager.MkApp(f.decl[name], nil)
		ruleMust(t, err)
		r := f.rule(name, nil, nil)
		ruleMust(t, set.Add(r))
		ruleMust(t, r.Close())
	}
	got, err = set.GroupedPredicates()
	ruleMust(t, err)
	if len(got) != 2 || got[0] != f.decl[first] || got[1] != f.decl[second] {
		t.Fatal("sparse reset did not shrink source table to sixteen slots", got)
	}
}
