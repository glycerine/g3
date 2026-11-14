package ast

// Source: src/ast/rewriter/poly_rewriter_def.h:453-487.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func bvHoistVariables(t *testing.T, m *ASTManager, n int) (FamilyID, ASTHandle, []ASTHandle) {
	t.Helper()
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	s, e := m.BitVectorSort(8)
	astTestMust(t, e)
	xs := make([]ASTHandle, n)
	for i := range xs {
		xs[i], e = m.MkVar(uint32(i), s)
		astTestMust(t, e)
		astTestPin(t, m, xs[i])
	}
	return fid, s, xs
}

func bvHoistProduct(t *testing.T, m *ASTManager, n, x ASTHandle, pin bool) ASTHandle {
	t.Helper()
	h, e := m.MkBitVectorMul(n, x)
	astTestMust(t, e)
	if pin {
		astTestPin(t, m, h)
	}
	return h
}

func TestASTBitVectorCoefficientHoistOriginalGroupsAndLiteralReuse(t *testing.T) {
	m := astTestManager(t)
	fid, s, xs := bvHoistVariables(t, m, 7)
	zero := bvRawOffsetTestNumeral(t, m, fid, s, "0", 8)
	three := bvRawOffsetTestNumeral(t, m, fid, s, "3", 8)
	large := bvRawOffsetTestNumeral(t, m, fid, s, "257", 8)
	z0 := bvHoistProduct(t, m, zero, xs[0], true)
	z1 := bvHoistProduct(t, m, zero, xs[1], true)
	a := bvHoistProduct(t, m, three, xs[2], true)
	b := bvHoistProduct(t, m, three, xs[3], true)
	c := bvHoistProduct(t, m, three, xs[4], true)
	d := bvHoistProduct(t, m, large, xs[5], true)
	e := bvHoistProduct(t, m, large, xs[6], true)
	v := bvHoistRoots(t, m, []ASTHandle{e, c, xs[1], a, z1, d, xs[0], b, z0})
	defer func() { astTestMust(t, bvReleaseHoistRoots(m, &v)) }()
	h, err := NewBitVectorCoefficientHoist(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, h.Close()) }()
	// Group factories have multiple operands, so the current sort is unread.
	astTestMust(t, h.Hoist(ASTHandle{}, &v))
	n := len(v)
	if n != 5 {
		t.Fatal("group compaction", n)
	}
	for i, want := range xs[:2] {
		got := v[uint32(i)]
		if got != want {
			t.Fatal("nonproducts must sort first")
		}
	}
	for i, row := range []struct {
		num ASTHandle
		pps []ASTHandle
	}{{zero, xs[:2]}, {three, xs[2:5]}, {large, xs[5:]}} {
		group := v[uint32(i+2)]
		literal, err := m.GetArg(group, 0)
		astTestMust(t, err)
		sum, err := m.GetArg(group, 1)
		astTestMust(t, err)
		kind, err := m.GetDeclKind(sum)
		astTestMust(t, err)
		count, err := m.GetNumArgs(sum)
		astTestMust(t, err)
		if literal != row.num || kind != 4 || int(count) != len(row.pps) {
			t.Fatal("coefficient replaced, normalized or group shortcut added")
		}
		for j, want := range row.pps {
			got, err := m.GetArg(sum, uint32(j))
			astTestMust(t, err)
			if got != want {
				t.Fatal("power-product ordering", i, j)
			}
		}
	}
	astTestMust(t, h.SourceReady())
	if h.placed[0] || h.placed[1] || h.pps != nil {
		t.Fatal("source locals did not end")
	}
}

func TestASTBitVectorCoefficientHoistSoleSliceOwnershipAndGrowth(t *testing.T) {
	m := astTestManager(t)
	fid, s, xs := bvHoistVariables(t, m, 40)
	n := bvRawOffsetTestNumeral(t, m, fid, s, "-3", 8)
	var v []ASTHandle
	defer func() { astTestMust(t, bvReleaseHoistRoots(m, &v)) }()
	old := make([]ASTHandle, len(xs))
	refs := make([]uint32, len(xs))
	for i := len(xs) - 1; i >= 0; i-- {
		old[i] = bvHoistProduct(t, m, n, xs[i], false)
		astTestMust(t, m.IncRef(old[i]))
		v = append(v, old[i])
		count, e := m.GetRefCount(xs[i])
		astTestMust(t, e)
		refs[i] = count
	}
	h, e := NewBitVectorCoefficientHoist(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, h.Close()) }()
	astTestMust(t, h.Hoist(s, &v))
	size := len(v)
	if size != 1 {
		t.Fatal("long group was not combined", size)
	}
	group := v[0]
	sum, e := m.GetArg(group, 1)
	astTestMust(t, e)
	for i, want := range xs {
		got, e := m.GetArg(sum, uint32(i))
		astTestMust(t, e)
		if got != want {
			t.Fatal("spilled power products reordered")
		}
		after, e := m.GetRefCount(want)
		astTestMust(t, e)
		if after != refs[i] {
			t.Fatal("borrowed power-product buffer acquired references", i)
		}
	}
	for _, mon := range old {
		if m.IsLiveHandle(mon) {
			t.Fatal("original products stayed alive after compaction")
		}
	}
}

func TestASTBitVectorCoefficientHoistOneKeepsMultiplication(t *testing.T) {
	m := astTestManager(t)
	fid, s, xs := bvHoistVariables(t, m, 2)
	one := bvRawOffsetTestNumeral(t, m, fid, s, "1", 8)
	left := bvHoistProduct(t, m, one, xs[0], true)
	right := bvHoistProduct(t, m, one, xs[1], true)
	v := bvHoistRoots(t, m, []ASTHandle{right, left})
	defer func() { astTestMust(t, bvReleaseHoistRoots(m, &v)) }()
	h, e := NewBitVectorCoefficientHoist(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, h.Close()) }()
	astTestMust(t, h.Hoist(s, &v))
	group := v[0]
	kind, e := m.GetDeclKind(group)
	astTestMust(t, e)
	if kind != 6 {
		t.Fatal("coefficient-one shortcut removed multiplication")
	}
	literal, e := m.GetArg(group, 0)
	astTestMust(t, e)
	if literal != one {
		t.Fatal("coefficient-one node was replaced")
	}
}

func TestASTBitVectorCoefficientHoistSingletonReadAndHeaderRefusal(t *testing.T) {
	m := bvOffsetRuntimeTestManager(t)
	fid, s, xs := bvHoistVariables(t, m, 1)
	n := bvRawOffsetTestNumeral(t, m, fid, s, strings.Repeat("9", 1300), 8)
	mon := bvHoistProduct(t, m, n, xs[0], true)
	v := bvHoistRoots(t, m, []ASTHandle{mon})
	defer func() { astTestMust(t, bvReleaseHoistRoots(m, &v)) }()
	used := m.Budget().Used()
	m.Budget().FailAfter(0)
	h, e := NewBitVectorCoefficientHoist(m)
	m.Budget().FailAfter(-1)
	if h != nil || !errors.Is(e, arena.ErrInjectedFailure) || m.Budget().Used() != used {
		t.Fatal("header refusal leaked", e)
	}
	h, e = NewBitVectorCoefficientHoist(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, h.Close()) }()
	copied := *h
	if e = copied.Close(); !errors.Is(e, ErrASTHandle) {
		t.Fatal("shallow copied owner accepted", e)
	}
	m.Budget().FailAfter(0)
	body := h.Hoist(ASTHandle{}, &v)
	m.Budget().FailAfter(-1)
	if body == nil || !h.placed[0] || !h.placed[1] {
		t.Fatal("last-element guard skipped first coefficient read", body)
	}
	child, _, _ := h.reads[0].Causes()
	if child == nil || errors.Is(child, arena.ErrInvalidSlot) {
		t.Fatal("missing singleton coefficient call", child)
	}
	got := v[0]
	if got != mon {
		t.Fatal("failed singleton changed caller vector")
	}
}

func TestASTBitVectorCoefficientHoistEmptySingletonAndIndependentGroups(t *testing.T) {
	m := astTestManager(t)
	fid, s, xs := bvHoistVariables(t, m, 2)
	n := bvRawOffsetTestNumeral(t, m, fid, s, "7", 8)
	product := bvHoistProduct(t, m, n, xs[0], true)
	for _, args := range [][]ASTHandle{nil, {xs[0]}, {product}, {product, xs[1]}} {
		v := bvHoistRoots(t, m, args)
		h, e := NewBitVectorCoefficientHoist(m)
		astTestMust(t, e)
		m.Budget().FailAfter(0)
		body := h.Hoist(ASTHandle{}, &v)
		m.Budget().FailAfter(-1)
		astTestMust(t, body)
		size := len(v)
		if int(size) != len(args) {
			t.Fatal("unmatched group changed size")
		}
		if e = h.Hoist(s, &v); !errors.Is(e, ErrASTPending) {
			t.Fatal("source invocation reused", e)
		}
		astTestMust(t, h.Close())
		astTestMust(t, h.Close())
		astTestMust(t, bvReleaseHoistRoots(m, &v))
	}
}

func TestASTBitVectorCoefficientHoistEveryActualReservation(t *testing.T) {
	complete, failures := false, 0
	for ordinal := int64(0); ordinal < 300 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m := bvOffsetRuntimeTestManager(t)
			fid, s, xs := bvHoistVariables(t, m, 18)
			n := bvRawOffsetTestNumeral(t, m, fid, s, strings.Repeat("9", 1300), 8)
			var v []ASTHandle
			defer func() { astTestMust(t, bvReleaseHoistRoots(m, &v)) }()
			for i := len(xs) - 1; i >= 0; i-- {
				mon := bvHoistProduct(t, m, n, xs[i], true)
				astTestMust(t, m.IncRef(mon))
				v = append(v, mon)
			}
			h, e := NewBitVectorCoefficientHoist(m)
			astTestMust(t, e)
			defer func() { astTestMust(t, h.Close()) }()
			m.Budget().FailAfter(ordinal)
			body := h.Hoist(s, &v)
			m.Budget().FailAfter(-1)
			if body != nil {
				failures++
				if !errors.Is(body, arena.ErrInjectedFailure) {
					var f umemory.Failure
					if !errors.As(body, &f) || f.Kind != umemory.FailureMemout {
						t.Fatal("unexpected refusal", body)
					}
				}
				if e = h.Hoist(s, &v); !errors.Is(e, ErrASTPending) {
					t.Fatal("failed call retried", e)
				}
			} else {
				complete = true
				size := len(v)
				if size != 1 {
					t.Fatal("hoist horizon result", size)
				}
			}
			ready := h.SourceReady()
			astTestMust(t, h.Close())
			if after := h.SourceReady(); after != ready {
				t.Fatal("owning close replaced source history", ready, after)
			}
		})
	}
	if !complete || failures == 0 {
		t.Fatal("hoist reservation horizon", complete, failures)
	}
}

// Test fixtures acquire the same caller-owned roots required by Hoist.
func bvHoistRoots(t *testing.T, m *ASTManager, args []ASTHandle) []ASTHandle {
	t.Helper()
	roots := append([]ASTHandle(nil), args...)
	for _, h := range roots {
		astTestMust(t, m.IncRef(h))
	}
	return roots
}
func bvReleaseHoistRoots(m *ASTManager, roots *[]ASTHandle) error {
	for len(*roots) != 0 {
		consumed, e := m.DecRefConsumed((*roots)[0])
		if consumed {
			(*roots)[0] = ASTHandle{}
			*roots = (*roots)[1:]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ErrASTPending
		}
	}
	*roots = nil
	return nil
}
