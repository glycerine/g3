//go:build g3trace

package ast

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestPsortASTCardSourceEntryTrace(t *testing.T) {
	m := astTestManager(t)
	c, e := NewPsortASTContext(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	s, e := NewPsortASTNetwork(c, r, m.BudgetField, nil)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	out, e := NewASTLiteralVector(m.BudgetField)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, out.Close()) })

	var buf bytes.Buffer
	tags := udiag.NewTraceTagsWithWriter(&buf)
	astTestMust(t, s.BindTrace(tags))
	astTestMust(t, s.card(0, 0, psortASTPointer{}, out))
	if buf.Len() != 0 {
		t.Fatalf("disabled pb tag emitted %q", buf.String())
	}
	tags.Enable("pb")
	astTestMust(t, s.card(0, 0, psortASTPointer{}, out))
	want := "-------- [pb] card src/util/sorting_network.h:598 ---------\n" +
		"card k: 0 n: 0\n" +
		"------------------------------------------------\n" +
		"-------- [pb] sorting src/util/sorting_network.h:744 ---------\n" +
		"sorting: 0\n" +
		"------------------------------------------------\n" +
		"-------- [pb] sorting src/util/sorting_network.h:769 ---------\n" +
		"sorting: 0\nin:\nout:\n" +
		"------------------------------------------------\n" +
		"-------- [pb] card src/util/sorting_network.h:612 ---------\n" +
		"card k: 0 n: 0\nin:\nout:\n" +
		"------------------------------------------------\n"
	if got := buf.String(); got != want {
		t.Fatalf("source card entry and completion records:\n got %q\nwant %q", got, want)
	}
}

func TestPsortASTScalarEntryTracesPrecedeSourceGuards(t *testing.T) {
	m := astTestManager(t)
	c, e := NewPsortASTContext(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	s, e := NewPsortASTNetwork(c, r, m.BudgetField, nil)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	out, e := NewASTLiteralVector(m.BudgetField)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, out.Close()) })

	var buf bytes.Buffer
	tags := udiag.NewTraceTagsWithWriter(&buf)
	tags.Enable("pb")
	astTestMust(t, s.BindTrace(tags))
	bad := psortASTPointer{}
	if e := s.merge(1, bad, 1, bad, out); !errors.Is(e, arena.ErrBounds) {
		t.Fatalf("merge source operand guard: %v", e)
	}
	if e := s.mergeSmall(1, 1, bad, 1, bad, out); !errors.Is(e, arena.ErrBounds) {
		t.Fatalf("smerge source operand guard: %v", e)
	}
	if e := s.directMerge(0, 0, bad, 0, bad, out); !errors.Is(e, ErrPsortUninitializedComparison) {
		t.Fatalf("dsmerge source comparison guard: %v", e)
	}
	if e := s.directSort(0, 0, bad, out); !errors.Is(e, ErrPsortUninitializedComparison) {
		t.Fatalf("dsorting source comparison guard: %v", e)
	}
	want := "-------- [pb] merge src/util/sorting_network.h:640 ---------\nmerge a: 1 b: 1\n------------------------------------------------\n" +
		"-------- [pb] smerge src/util/sorting_network.h:805 ---------\nsmerge: c:1 a:1 b:1\n------------------------------------------------\n" +
		"-------- [pb] dsmerge src/util/sorting_network.h:923 ---------\ndsmerge: c:0 a:0 b:0\n------------------------------------------------\n" +
		"-------- [pb] dsorting src/util/sorting_network.h:982 ---------\ndsorting m: 0 n: 0\n------------------------------------------------\n"
	if got := buf.String(); got != want {
		t.Fatalf("source scalar entry sequence:\n got %q\nwant %q", got, want)
	}
}

func TestPsortASTInterleaveEntryTraceBeforeOutput(t *testing.T) {
	m := astTestManager(t)
	c, e := NewPsortASTContext(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	s, e := NewPsortASTNetwork(c, r, m.BudgetField, nil)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	a, e := NewASTLiteralVector(m.BudgetField)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, a.Close()) })
	b, e := NewASTLiteralVector(m.BudgetField)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	out, e := NewASTLiteralVector(m.BudgetField)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, out.Close()) })
	h, e := m.MkFalse()
	astTestMust(t, e)
	astTestMust(t, a.Push(h))

	var buf bytes.Buffer
	tags := udiag.NewTraceTagsWithWriter(&buf)
	tags.Enable("pb")
	astTestMust(t, s.BindTrace(tags))
	astTestMust(t, s.interleave(a, b, out))
	sz, e := out.Size()
	astTestMust(t, e)
	got, e := out.At(0)
	astTestMust(t, e)
	if sz != 1 || got != h {
		t.Fatal("source interleave output", sz, got)
	}
	want := "-------- [pb] interleave src/util/sorting_network.h:712 ---------\n" +
		"interleave: 1 0\n" +
		"------------------------------------------------\n" +
		"-------- [pb] interleave src/util/sorting_network.h:732 ---------\n" +
		"interleave: 1 0\na: false \nb: \nout: false \n" +
		"------------------------------------------------\n"
	if got := buf.String(); got != want {
		t.Fatalf("source interleave entry:\n got %q\nwant %q", got, want)
	}
}

// AUDIT.md ast_08-F1: the AST network emits the dualize, add_subset,
// add_clause and completion records of sorting_network.h. Expected text
// and counts read from build_trace/z3 -tr:pb on
// ((_ at-most 2) a b c d e) with (apply card2bv).
func TestPsortASTGeFullTraceMatchesSource(t *testing.T) {
	m := astTestManager(t)
	c, e := NewPsortASTContext(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	astTestMust(t, c.SetFreshPrefix("sn"))
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	s, e := NewPsortASTNetwork(c, r, m.BudgetField, nil)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	in, e := NewASTLiteralVector(m.BudgetField)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, in.Close()) })
	boolSort, e := m.MkBoolSort()
	astTestMust(t, e)
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		x, e := m.MkConst(astTestSymbol(t, m, name), boolSort)
		astTestMust(t, e)
		astTestPin(t, m, x)
		nx, e := m.MkNot(x)
		astTestMust(t, e)
		astTestPin(t, m, nx)
		astTestMust(t, in.Push(nx))
	}
	span, e := in.Span()
	astTestMust(t, e)
	var buf bytes.Buffer
	tags := udiag.NewTraceTagsWithWriter(&buf)
	tags.Enable("pb")
	astTestMust(t, s.BindTrace(tags))
	_, e = s.Ge(true, 3, 5, span)
	astTestMust(t, e)
	got := buf.String()
	order := []string{
		"[pb] dualize src/util/sorting_network.h:516 ---------\n5: a b c d e  ~ 2\n",
		"[pb] card src/util/sorting_network.h:598 ---------\ncard k: 3 n: 5\n",
		"[pb] dsorting src/util/sorting_network.h:982 ---------\ndsorting m: 3 n: 5\n",
		"[pb] add_subset src/util/sorting_network.h:1017 ---------\nk:1 offset: 0 n: 5 sn!0 \n",
		"[pb] card src/util/sorting_network.h:612 ---------\ncard k: 3 n: 5\nin:a b c d e \nout:sn!0 sn!1 sn!2 \n",
	}
	at := 0
	for _, record := range order {
		i := strings.Index(got[at:], record)
		if i < 0 {
			t.Fatalf("missing or misordered record %q in:\n%s", record, got)
		}
		at += i + len(record)
	}
	if n := strings.Count(got, "[pb] add_subset "); n != 82 {
		t.Fatalf("add_subset records %d, want 82", n)
	}
	if n := strings.Count(got, "[pb] add_clause "); n != 41 {
		t.Fatalf("add_clause records %d, want 41", n)
	}
}
