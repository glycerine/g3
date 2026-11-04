package pdr_test

// Source: src/muz/pdr/pdr_reachable_cache.cpp cache modes and symbolic
// disjunction-chain reachability checks.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
)

func TestReachableCacheModes(t *testing.T) {
	for _, mode := range []pdr.ReachableCacheMode{pdr.ReachableNoCache, pdr.ReachableHashCache, pdr.ReachableConstraintCache} {
		t.Run(map[pdr.ReachableCacheMode]string{pdr.ReachableNoCache: "none", pdr.ReachableHashCache: "hash", pdr.ReachableConstraintCache: "constraint"}[mode], func(t *testing.T) {
			command, manager, contexts := pdrContextFixture(t, 0)
			truth, err := manager.MkTrue()
			pdrMust(t, err)
			cache, err := pdr.NewReachableCache(manager, contexts, truth, mode)
			pdrMust(t, err)
			p := pdrBool(t, manager, "reachable-p")
			q := pdrBool(t, manager, "reachable-q")
			notP, err := manager.MkNot(p)
			pdrMust(t, err)
			if found, x := cache.IsReachable(p); x != nil || found {
				t.Fatal("empty cache", found, x)
			}
			pdrMust(t, cache.AddReachable(p))
			found, err := cache.IsReachable(p)
			pdrMust(t, err)
			if found != (mode != pdr.ReachableNoCache) {
				t.Fatal("inserted cube", mode, found)
			}
			found, err = cache.IsReachable(notP)
			pdrMust(t, err)
			if found {
				t.Fatal("incompatible cube reported reachable")
			}
			pdrMust(t, cache.AddReachable(q))
			found, err = cache.IsReachable(q)
			pdrMust(t, err)
			if found != (mode != pdr.ReachableNoCache) {
				t.Fatal("second cube", mode, found)
			}
			stats, err := cache.Statistics()
			pdrMust(t, err)
			if mode != pdr.ReachableNoCache && (stats.Inserts != 2 || stats.Hits != 2 || stats.Misses != 2) {
				t.Fatal("cache statistics", stats)
			}
			pdrMust(t, cache.ResetStatistics())
			stats, err = cache.Statistics()
			pdrMust(t, err)
			if stats != (pdr.ReachableCacheStatistics{}) {
				t.Fatal("statistics reset", stats)
			}
			pdrMust(t, cache.Close())
			pdrMust(t, contexts.Close())
			pdrMust(t, command.Close())
		})
	}
}

func TestReachableConstraintCacheHonorsBackgroundAndInit(t *testing.T) {
	command, manager, contexts := pdrContextFixture(t, 1)
	p := pdrBool(t, manager, "reachable-background-p")
	q := pdrBool(t, manager, "reachable-background-q")
	cache, err := pdr.NewReachableCache(manager, contexts, p, pdr.ReachableConstraintCache)
	pdrMust(t, err)
	pdrMust(t, cache.AddInit(q))
	conjunction, err := manager.MkAnd(p, q)
	pdrMust(t, err)
	found, err := cache.IsReachable(conjunction)
	pdrMust(t, err)
	if !found {
		t.Fatal("background-compatible init state was not reachable")
	}
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	found, err = cache.IsReachable(notP)
	pdrMust(t, err)
	if found {
		t.Fatal("background-incompatible state was reachable")
	}
	pdrMust(t, cache.Close())
	pdrMust(t, contexts.Close())
	pdrMust(t, command.Close())
}

// A failed background assertion must retire the freshly-created context so
// its manager can close and a later cache construction can proceed.
func TestReachableCacheConstructorRejectsFreeBackgroundAndRetries(t *testing.T) {
	for _, maximum := range []uint32{0, 1} {
		t.Run(map[uint32]string{0: "fresh", 1: "shared"}[maximum], func(t *testing.T) {
			command, manager, contexts := pdrContextFixture(t, maximum)
			boolean, err := manager.MkBoolSort()
			pdrMust(t, err)
			free, err := manager.MkVar(0, boolean)
			pdrMust(t, err)
			cache, err := pdr.NewReachableCache(manager, contexts, free, pdr.ReachableConstraintCache)
			if err == nil || cache != nil {
				t.Fatal("free background accepted", cache, err)
			}
			truth, err := manager.MkTrue()
			pdrMust(t, err)
			cache, err = pdr.NewReachableCache(manager, contexts, truth, pdr.ReachableConstraintCache)
			pdrMust(t, err)
			pdrMust(t, cache.Close())
			pdrMust(t, contexts.Close())
			pdrMust(t, command.Close())
		})
	}
}

func TestReachableConstraintCacheRetainsEarlierDisjunct(t *testing.T) {
	command, manager, contexts := pdrContextFixture(t, 1)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	cache, err := pdr.NewReachableCache(manager, contexts, truth, pdr.ReachableConstraintCache)
	pdrMust(t, err)
	p := pdrBool(t, manager, "reachable-chain-p")
	q := pdrBool(t, manager, "reachable-chain-q")
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	notQ, err := manager.MkNot(q)
	pdrMust(t, err)
	first, err := manager.MkAnd(p, notQ)
	pdrMust(t, err)
	second, err := manager.MkAnd(notP, q)
	pdrMust(t, err)
	neither, err := manager.MkAnd(notP, notQ)
	pdrMust(t, err)
	var references []ast.ASTRef
	for _, cube := range []ast.ASTHandle{first, second, neither} {
		reference, x := manager.NewRef(cube)
		pdrMust(t, x)
		references = append(references, reference)
	}
	pdrMust(t, cache.AddReachable(first))
	pdrMust(t, cache.AddReachable(second))
	for _, check := range []struct {
		cube ast.ASTHandle
		want bool
	}{{first, true}, {second, true}, {neither, false}} {
		found, err := cache.IsReachable(check.cube)
		pdrMust(t, err)
		if found != check.want {
			t.Fatalf("chain reachability: got %v, want %v", found, check.want)
		}
	}
	pdrMust(t, cache.Close())
	for i := range references {
		pdrMust(t, manager.DelRef(&references[i]))
	}
	pdrMust(t, contexts.Close())
	pdrMust(t, command.Close())
}
