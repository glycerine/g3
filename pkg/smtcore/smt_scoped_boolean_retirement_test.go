package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTScopedBooleanRangesAcrossLastReference(t *testing.T) {
	m, s, _ := delayedActivityFixture(t, 0)
	base := make([]int32, 4)
	for i := range base {
		var err error
		base[i], err = s.MkBoolVar(smtRelevancyBool(t, m, fmt.Sprintf("survivor-%d", i)))
		astTestMust(t, err)
	}
	for cycle := 0; cycle < 3; cycle++ {
		smtTestUserPush(t, s)
		var expressions []ast.ASTHandle
		var variables []int32
		var expected []uint32
		before := s.statistics.NumDelBoolVar
		for i := 0; i < 19; i++ {
			h := smtRelevancyBool(t, m, fmt.Sprintf("mixed-scope-%d-%d", cycle, i))
			// The middle variable loses its last AST reference during the pop.
			// On either side, each application has a surviving caller reference.
			if i != 8 {
				astTestMust(t, m.IncRef(h))
			}
			ref, err := m.GetRefCount(h)
			astTestMust(t, err)
			expected = append(expected, ref)
			v, err := s.MkBoolVar(h)
			astTestMust(t, err)
			if !s.basicActivity.Contains(v) {
				astTestMust(t, s.basicActivity.Insert(v))
			}
			if !s.delayedActivity.Contains(v) {
				astTestMust(t, s.delayedActivity.Insert(v))
			}
			expressions = append(expressions, h)
			variables = append(variables, v)
		}
		astTestMust(t, s.PopScopes(1))
		if n := s.statistics.NumDelBoolVar - before; n != uint32(len(expressions)) {
			t.Fatal("deleted variables", n)
		}
		for i, h := range expressions {
			v := variables[i]
			if s.basicActivity.Contains(v) || s.delayedActivity.Contains(v) {
				t.Fatal("retired queue membership", v)
			}
			if _, ok := s.boolVar.lookup(h); ok {
				t.Fatal("retired binding", v)
			}
			if i == 8 {
				if m.IsLiveHandle(h) {
					t.Fatal("last reference still alive")
				}
				continue
			}
			refs, err := m.GetRefCount(h)
			astTestMust(t, err)
			if refs != expected[i] {
				t.Fatal("surviving reference", v, refs, expected[i])
			}
			astTestMust(t, m.DecRef(h))
		}
		for _, v := range base {
			if !s.basicActivity.Contains(v) && !s.delayedActivity.Contains(v) {
				t.Fatal("lost base member", v)
			}
		}
	}
}

func TestSMTScopedBooleanRangesResumeAfterRefusal(t *testing.T) {
	m, s, _ := delayedActivityFixture(t, 0)
	smtTestUserPush(t, s)
	before := s.statistics.NumDelBoolVar
	const size, stop = 25, 9
	roots := make([]ast.ASTHandle, size)
	var missing ast.ASTHandle
	for i := range roots {
		h := smtRelevancyBool(t, m, fmt.Sprintf("resumed-%d", i))
		roots[i] = h
		astTestMust(t, m.IncRef(h))
		v, e := s.MkBoolVar(h)
		astTestMust(t, e)
		if i == stop {
			// Give this trail entry a live RC-zero expression, the same refused
			// ownership decrement used in TestSMTRelevancyPopRetriesOnlyUnconsumedOwners.
			// Supplying the missing reference below makes retry possible.
			missing = smtRelevancyBool(t, m, "missing-reference")
			s.boolVar.del(h)
			s.boolVar.set(missing, v)
			s.boolExpr[v] = missing
			astTestMust(t, m.DecRef(h))
		}
	}
	if e := s.PopScopes(1); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal("missing decrement", e)
	}
	if !s.retiringBoolean || s.retiringBooleanRoot != missing || s.statistics.NumDelBoolVar-before != size-stop-1 {
		t.Fatal("interrupted ownership or retirement count", s.retiringBoolean, s.statistics.NumDelBoolVar-before)
	}
	for i, h := range roots {
		n, e := m.GetRefCount(h)
		astTestMust(t, e)
		want := uint32(1)
		if i < stop {
			want++
		}
		if n != want {
			t.Fatal("interrupted references", i, n, want)
		}
	}
	astTestMust(t, m.IncRef(missing))
	astTestMust(t, s.PopScopes(1))
	if s.statistics.NumDelBoolVar-before != size || s.retiringBoolean || m.IsLiveHandle(missing) {
		t.Fatal("retry did not finish the original trail")
	}
	for _, h := range roots {
		n, e := m.GetRefCount(h)
		astTestMust(t, e)
		if n != 1 {
			t.Fatal("retry repeated an owned decrement", n)
		}
		astTestMust(t, m.DecRef(h))
	}
}

func TestSMTScopedBooleanRangesPreserveCachedActivity(t *testing.T) {
	m, s, _ := cachedActivityFixture(t)
	astTestMust(t, s.beginSearchPersistence(0, SMTParamsState{}))
	smtTestUserPush(t, s)
	roots := make([]ast.ASTHandle, 22)
	for i := range roots {
		h := smtRelevancyBool(t, m, fmt.Sprintf("cached-range-%d", i))
		astTestMust(t, m.IncRef(h))
		roots[i] = h
		v, e := s.MkBoolVar(h)
		astTestMust(t, e)
		if i == 6 || i == 13 {
			astTestMust(t, s.incrementBooleanActivity(v))
		}
	}
	astTestMust(t, s.PopScopes(1))
	if len(s.activityCacheDomain) != 2 || s.activityCacheDomain[0] != roots[13] || s.activityCacheDomain[1] != roots[6] {
		t.Fatal("cached score retirement order", s.activityCacheDomain)
	}
	for i, h := range roots {
		n, e := m.GetRefCount(h)
		astTestMust(t, e)
		want := uint32(1)
		if i == 6 || i == 13 {
			want++
			if s.activityCache[h] != 1 {
				t.Fatal("lost activity", i)
			}
		}
		if n != want {
			t.Fatal("cached references", i, n, want)
		}
	}
	astTestMust(t, s.endSearchPersistence())
	astTestMust(t, s.resetBooleanActivityCache())
	for _, h := range roots {
		astTestMust(t, m.DecRef(h))
	}
}
