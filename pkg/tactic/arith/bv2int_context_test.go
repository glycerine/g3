package arith

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func bv2IntContextFixture(t *testing.T) (*BV2IntContext, *ast.ASTManager) {
	t.Helper()
	base, m := fixDLFixture(t, false)
	r, err := NewBV2IntContext(m, base.context, nil)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	return r, m
}
func bv2IntDisjunction(t *testing.T, m *ast.ASTManager, x ast.ASTHandle, values []string) ast.ASTHandle {
	t.Helper()
	var eqs []ast.ASTHandle
	for i, text := range values {
		n := fixDLNumber(t, m, true, text)
		a, b := x, n
		if i%2 == 1 {
			a, b = b, a
		}
		eq, err := m.MkEq(a, b)
		equationMust(t, err)
		fixDLPin(t, m, eq)
		eqs = append(eqs, eq)
	}
	h, err := m.MkOrArgs(eqs)
	equationMust(t, err)
	fixDLPin(t, m, h)
	return h
}
func TestBV2IntContextConsecutivePowersAndWidths(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 7, 8, 9, 33, 65} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			r, m := bv2IntContextFixture(t)
			x := fixDLConst(t, m, true, "power")
			// Descending order includes integers beyond uint64 and alternates equality sides.
			values := make([]string, n)
			for i := range values {
				values[i] = new(big.Int).Lsh(big.NewInt(1), uint(n-i-1)).String()
			}
			f := bv2IntDisjunction(t, m, x, values)
			// MkOrArgs deliberately retains even the unary OR required by the source.
			goal := fixDLGoal(t, m, false, false, false, f)
			equationMust(t, r.CollectPower2(goal))
			logx, found, err := r.IsPower2(x)
			equationMust(t, err)
			if !found {
				t.Fatal("missed consecutive powers")
			}
			snapshot, err := r.Power2()
			equationMust(t, err)
			if len(snapshot) != 1 || snapshot[x] != logx {
				t.Fatal("missing NLA substitution mapping")
			}
			delete(snapshot, x)
			if _, ok, err := r.IsPower2(x); err != nil || !ok {
				t.Fatal("map accessor exposed owner storage", ok, err)
			}
			width, err := m.BitVectorSize(logx)
			equationMust(t, err)
			if width != uint32(n) {
				t.Fatalf("extended width %d, want %d", width, n)
			}
			kind, err := m.GetDeclKind(logx)
			equationMust(t, err)
			if kind != 39 {
				t.Fatal("missing source zero extension", kind)
			}
			fresh, err := m.GetArg(logx, 0)
			equationMust(t, err)
			w, err := m.BitVectorSize(fresh)
			equationMust(t, err)
			want := uint32(1)
			for k := n; k >= 2; k /= 2 {
				want++
			}
			if w != want {
				t.Fatalf("exponent width %d, want %d", w, want)
			}
			// Recollecting replaces only the map value. The old expression remains owned.
			equationMust(t, r.CollectPower2(goal))
			newer, found, err := r.IsPower2(x)
			equationMust(t, err)
			if !found || newer == logx {
				t.Fatal("expected a fresh replacement")
			}
			if _, err = m.GetKind(logx); err != nil {
				t.Fatal("old exponent retired early", err)
			}
			equationMust(t, r.Reset())
			if _, found, err = r.IsPower2(x); err != nil || found {
				t.Fatal("reset left mapping", found, err)
			}
			if _, err = m.GetKind(logx); err == nil {
				t.Fatal("reset retained old trail entry")
			}
			if _, err = m.GetKind(newer); err == nil {
				t.Fatal("reset retained current trail entry")
			}
			equationMust(t, r.CollectPower2(goal))
		})
	}
}
func TestBV2IntContextRejectsNonPowerDisjunctions(t *testing.T) {
	for _, values := range [][]string{{"0", "1"}, {"1", "4"}, {"1", "2", "2"}, {"2", "4"}, {"-1", "1"}, {"1", "2", "5"}} {
		t.Run(fmt.Sprint(values), func(t *testing.T) {
			r, m := bv2IntContextFixture(t)
			x := fixDLConst(t, m, true, "x")
			f := bv2IntDisjunction(t, m, x, values)
			equationMust(t, r.CollectPower2(fixDLGoal(t, m, false, false, false, f)))
			if _, ok, err := r.IsPower2(x); err != nil || ok {
				t.Fatal("accepted invalid set", ok, err)
			}
		})
	}
	r, m := bv2IntContextFixture(t)
	x := fixDLConst(t, m, true, "x")
	y := fixDLConst(t, m, true, "y")
	a, err := m.MkEq(x, fixDLNumber(t, m, true, "1"))
	equationMust(t, err)
	fixDLPin(t, m, a)
	b, err := m.MkEq(y, fixDLNumber(t, m, true, "2"))
	equationMust(t, err)
	fixDLPin(t, m, b)
	f, err := m.MkOrArgs([]ast.ASTHandle{a, b})
	equationMust(t, err)
	fixDLPin(t, m, f)
	real := fixDLConst(t, m, false, "real")
	realEq, err := m.MkEq(real, fixDLNumber(t, m, false, "1"))
	equationMust(t, err)
	fixDLPin(t, m, realEq)
	realOr, err := m.MkOrArgs([]ast.ASTHandle{realEq})
	equationMust(t, err)
	fixDLPin(t, m, realOr)
	// A bare equality and OR with a non-equality are also refused.
	badOr, err := m.MkOrArgs([]ast.ASTHandle{a, f})
	equationMust(t, err)
	fixDLPin(t, m, badOr)
	equationMust(t, r.CollectPower2(fixDLGoal(t, m, false, false, false, f, a, realOr, badOr)))
	for _, e := range []ast.ASTHandle{x, y, real} {
		if _, ok, err := r.IsPower2(e); err != nil || ok {
			t.Fatal("invalid form published a mapping", ok, err)
		}
	}
}
func TestBV2IntContextParametersSideOwnershipAndReset(t *testing.T) {
	base, m := fixDLFixture(t, false)
	c := base.context
	p, err := c.NewValue()
	equationMust(t, err)
	key := "max_bv_size"
	equationMust(t, c.SetUIntName(&p, &key, 17))
	r, err := NewBV2IntContext(m, c, &p)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	equationMust(t, c.SetUIntName(&p, &key, 2))
	equationMust(t, c.Del(&p))
	n, err := r.MaxNumBits()
	equationMust(t, err)
	if n != 17 {
		t.Fatal("parameter scalar changed after construction", n)
	}
	boolSort, err := m.MkBoolSort()
	equationMust(t, err)
	h, err := m.MkFreshConst("side", boolSort)
	equationMust(t, err)
	equationMust(t, r.AddSideCondition(h))
	equationMust(t, r.AddSideCondition(h))
	side, err := r.SideConditions()
	equationMust(t, err)
	if len(side) != 2 || side[0] != h || side[1] != h {
		t.Fatal("side condition order/deduplication")
	}
	side[0] = ast.ASTHandle{}
	side, err = r.SideConditions()
	equationMust(t, err)
	if side[0] != h {
		t.Fatal("caller mutated owner slice")
	}
	equationMust(t, r.Reset())
	if _, err = m.GetKind(h); err == nil {
		t.Fatal("side references survived reset")
	}
	n, err = r.MaxNumBits()
	equationMust(t, err)
	if n != 17 {
		t.Fatal("reset changed scalar parameter")
	}
	equationMust(t, r.Close())
	equationMust(t, r.Close())
	if _, err = r.SideConditions(); err == nil {
		t.Fatal("closed owner accepted operation")
	}
}

func TestBV2IntContextAllowsExpressionKeysAndIgnoresSizeLimitDuringCollection(t *testing.T) {
	base, m := fixDLFixture(t, false)
	p, err := base.context.NewValue()
	equationMust(t, err)
	key := "max_bv_size"
	equationMust(t, base.context.SetUIntName(&p, &key, 1))
	r, err := NewBV2IntContext(m, base.context, &p)
	equationMust(t, err)
	equationMust(t, base.context.Del(&p))
	t.Cleanup(func() { equationMust(t, r.Close()) })
	x := fixDLConst(t, m, true, "x")
	// The source allows an arbitrary expression, rather than only a variable.
	term := fixDLApp(t, m, 6, x, fixDLNumber(t, m, true, "1"))
	f := bv2IntDisjunction(t, m, term, []string{"4", "1", "2"})
	equationMust(t, r.CollectPower2(fixDLGoal(t, m, false, false, false, f)))
	h, ok, err := r.IsPower2(term)
	equationMust(t, err)
	if !ok {
		t.Fatal("expression key or width beyond max_bv_size was refused")
	}
	width, err := m.BitVectorSize(h)
	equationMust(t, err)
	if width != 3 {
		t.Fatal("collector incorrectly enforced arithmetic rewrite size limit", width)
	}
}

func TestBV2IntContextConstructorRefusalAndCopiedOwner(t *testing.T) {
	base, m := fixDLFixture(t, false)
	warm, err := NewBV2IntContext(m, base.context, nil)
	equationMust(t, err)
	max, err := warm.MaxNumBits()
	equationMust(t, err)
	if max != ^uint32(0) {
		t.Fatal("default max_bv_size", max)
	}
	copied := *warm
	if _, err = copied.MaxNumBits(); err == nil {
		t.Fatal("copied owner accepted")
	}
	equationMust(t, warm.Close())
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	refused, err := NewBV2IntContext(m, base.context, nil)
	m.Budget().FailAfter(-1)
	if err == nil || refused != nil {
		t.Fatal("constructor ignored reservation refusal", err)
	}
	if after := m.Budget().Used(); after != before {
		t.Fatal("refused constructor leaked", before, after)
	}
}

func TestBV2IntCollectPower2RefusalFrontierRetriesWithoutPartialMapping(t *testing.T) {
	for fail := int64(0); fail < 16; fail++ {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			r, m := bv2IntContextFixture(t)
			x := fixDLConst(t, m, true, "power-refusal")
			goal := fixDLGoal(t, m, false, false, false, bv2IntDisjunction(t, m, x, []string{"1", "2", "4"}))
			m.Budget().FailAfter(fail)
			err := r.CollectPower2(goal)
			m.Budget().FailAfter(-1)
			if err == nil {
				if _, found, e := r.IsPower2(x); e != nil || !found {
					t.Fatal("successful collection omitted its map entry", found, e)
				}
			} else {
				if _, found, e := r.IsPower2(x); e != nil || found {
					t.Fatal("failed collection published a partial mapping", found, e)
				}
				if e := r.CollectPower2(goal); e != nil {
					t.Fatal("collection did not recover after allocation refusal", e)
				}
				if _, found, e := r.IsPower2(x); e != nil || !found {
					t.Fatal("retry omitted the complete mapping", found, e)
				}
			}
			if e := r.Close(); e != nil {
				t.Fatal("close after collection frontier", e)
			}
		})
	}
}

type bv2IntTraceWriter struct {
	bytes.Buffer
	check func()
}

func (w *bv2IntTraceWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *bv2IntTraceWriter) Write(p []byte) (int, error) {
	if w.check != nil {
		w.check()
	}
	return w.Buffer.Write(p)
}
func TestBV2IntContextTracePublicationOrderAndErasure(t *testing.T) {
	r, m := bv2IntContextFixture(t)
	x := fixDLConst(t, m, true, "power_trace")
	f := bv2IntDisjunction(t, m, x, []string{"1", "2", "4"})
	writer := &bv2IntTraceWriter{}
	calls := 0
	writer.check = func() {
		calls++
		if len(r.trail) != 1 {
			t.Fatal("trace before trail publication")
		}
		if _, ok, err := r.findPower2(x); err != nil || ok {
			t.Fatal("map published before trace")
		}
	}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("bv2int_rewriter")
	equationMust(t, r.BindTrace(tags))
	equationMust(t, r.CollectPower2(fixDLGoal(t, m, false, false, false, f)))
	if tags.ShouldTrace("bv2int_rewriter") {
		text := writer.String()
		if calls == 0 || !strings.Contains(text, "collect_power2 src/tactic/arith/bv2int_rewriter.cpp:86") || !strings.Contains(text, "power_trace |-> ((_ zero_extend 1) log2_v!") {
			t.Fatal("source trace", text, calls)
		}
	} else if calls != 0 || writer.Len() != 0 {
		t.Fatal("erased trace executed body")
	}
	// A failed output stream must not prevent map publication.
	refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	refused.EnableAll(true)
	equationMust(t, r.BindTrace(refused))
	equationMust(t, r.CollectPower2(fixDLGoal(t, m, false, false, false, f)))
	if _, ok, err := r.IsPower2(x); err != nil || !ok {
		t.Fatal("stream failure prevented mapping", ok, err)
	}
}
