package numeral

// Source: src/util/mpfx.cpp:280-306,419-444. Nonexceptional zero-word results
// can retain the source allocated slot and sign after directed rounding.

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestFixedPointProductUnderflowRetainsAllocatedZeroWords(t *testing.T) {
	m, err := NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.New()
	b, _ := m.New()
	out, _ := m.New()
	defer func() {
		for _, v := range []*FixedPoint{a, b, out} {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(m.SetRatio(a, 1, 1<<32))
	must(m.SetRatio(b, 1, 2))
	must(m.Mul(a, b, out))
	if zero, err := m.IsZero(out); err != nil || zero {
		t.Fatal("positive underflow lost allocated slot", zero, err)
	}
	if pos, err := m.IsPos(out); err != nil || !pos {
		t.Fatal("positive underflow sign", pos, err)
	}
	must(m.SetRounding(true))
	must(m.SetRatio(b, -1, 2))
	must(m.Mul(a, b, out))
	if text, err := m.String(out); err != nil || text != "-0" {
		t.Fatal("negative underflow slot and sign", text, err)
	}
	must(m.Mul(out, a, out))
	if neg, err := m.IsNeg(out); err != nil || !neg {
		t.Fatal("aliased zero-word product lost source sign", neg, err)
	}
	must(m.Reset(a))
	must(m.Mul(a, b, out))
	if zero, err := m.IsZero(out); err != nil || !zero {
		t.Fatal("canonical zero operand did not reset product", zero, err)
	}
}

func TestFixedPointNonintegerRationalUnderflowRetainsDestinationSign(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	q, err := NewRationalService(RationalServiceConfig{Integers: IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := m.New()
	r := NewRational()
	defer func() {
		if err := out.Close(); err != nil {
			t.Error(err)
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
		if err := q.Manager.Del(&r); err != nil {
			t.Error(err)
		}
		if err := q.Close(); err != nil {
			t.Error(err)
		}
		if budget.Used() != 0 {
			t.Error("rational conversion retained budget", budget.Used())
		}
	}()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(q.Manager.SetString(&r, "1/8589934592"))
	must(m.SetFromRational(out, q.Manager, &r))
	if zero, err := m.IsZero(out); err != nil || zero {
		t.Fatal("noninteger zero quotient released source slot", zero, err)
	}
	must(m.SetInt64(out, -1))
	must(m.SetRounding(true))
	must(m.SetFromRational(out, q.Manager, &r))
	if text, err := m.String(out); err != nil || text != "-0" {
		t.Fatal("noninteger setter lost prior negative sign", text, err)
	}
	must(m.SetFromRational(out, q.Manager, &r))
	if neg, err := m.IsNeg(out); err != nil || !neg {
		t.Fatal("repeated setter did not read retained sign", neg, err)
	}
	must(q.Manager.SetString(&r, "0"))
	must(m.SetFromRational(out, q.Manager, &r))
	if zero, err := m.IsZero(out); err != nil || !zero {
		t.Fatal("integer zero setter did not reset slot", zero, err)
	}
}

func TestFixedPointAddSubAllocatedZeroWordBranches(t *testing.T) {
	m, err := NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	values := make([]*FixedPoint, 6)
	for i := range values {
		values[i], err = m.New()
		if err != nil {
			t.Fatal(err)
		}
	}
	epsilon, half, slot, other, out, zero := values[0], values[1], values[2], values[3], values[4], values[5]
	defer func() {
		for _, v := range values {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(m.SetRatio(epsilon, 1, 1<<32))
	must(m.SetRatio(half, 1, 2))
	must(m.Mul(epsilon, half, slot))
	must(m.Copy(other, slot))
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("mpfx")
	must(m.BindTrace(tags))
	want := func(text string, canonical bool) {
		t.Helper()
		got, err := m.String(out)
		if err != nil || got != text {
			t.Fatal("zero-word text", got, text, err)
		}
		isZero, err := m.IsZero(out)
		if err != nil || isZero != canonical {
			t.Fatal("source zero-slot classification", isZero, canonical, err)
		}
	}
	must(m.Add(slot, zero, out))
	want("0", false)
	if output.Len() != 0 {
		t.Fatal("source shortcut emitted inner trace", output.String())
	}
	must(m.Sub(zero, slot, out))
	want("-0", false)
	if output.Len() != 0 {
		t.Fatal("source negated shortcut emitted inner trace", output.String())
	}
	must(m.Add(slot, zero, slot))
	if pos, err := m.IsPos(slot); err != nil || !pos {
		t.Fatal("aliased copy shortcut lost allocated slot", pos, err)
	}
	must(m.Sub(zero, slot, slot))
	if neg, err := m.IsNeg(slot); err != nil || !neg {
		t.Fatal("aliased negate shortcut lost allocated slot", neg, err)
	}
	must(m.Neg(slot))
	must(m.Add(slot, other, out))
	want("0", false)
	if tags.ShouldTrace("mpfx") && !strings.Contains(output.String(), "add(0, 0)") {
		t.Fatal("allocated zero operands skipped source inner trace", output.String())
	}
	output.Reset()
	must(m.Sub(slot, other, out))
	want("0", true)
	must(m.Neg(other))
	must(m.Add(slot, other, out))
	want("0", true)
	must(m.Sub(slot, other, out))
	want("0", false)
	must(m.Add(other, other, out))
	want("-0", false)
}

func TestFixedPointAddSubOverflowPublishesWrappedWords(t *testing.T) {
	m, err := NewFixedPointManager(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.New()
	b, _ := m.New()
	out, _ := m.New()
	defer func() {
		for _, v := range []*FixedPoint{a, b, out} {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(m.SetRatio(b, 3, 2))
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("mpfx")
	tags.Enable("mpfx_trace")
	must(m.BindTrace(tags))
	check := func(v *FixedPoint, want string) {
		t.Helper()
		got, err := m.String(v)
		if err != nil || got != want {
			t.Fatal("wrapped source words", got, want, err)
		}
		if tags.ShouldTrace("mpfx") && strings.Contains(output.String(), "result: ") {
			t.Fatal("source overflow reached post-add TRACE", output.String())
		}
		output.Reset()
	}
	for _, alias := range []bool{false, true} {
		must(m.SetUint64(a, (1<<32)-1))
		dst := out
		if alias {
			dst = a
		}
		if err := m.Add(a, b, dst); !errors.Is(err, ErrFixedPointOverflow) {
			t.Fatal("positive add carry", alias, err)
		}
		check(dst, "1/2")
		must(m.SetUint64(a, (1<<32)-1))
		must(m.Neg(a))
		if err := m.Sub(a, b, dst); !errors.Is(err, ErrFixedPointOverflow) {
			t.Fatal("negative sub carry", alias, err)
		}
		check(dst, "-1/2")
	}
}

func TestFixedPointMultiplyOverflowPublishesSignBeforeProductWords(t *testing.T) {
	m, err := NewFixedPointManager(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.New()
	b, _ := m.New()
	out, _ := m.New()
	defer func() {
		for _, v := range []*FixedPoint{a, b, out} {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("mpfx_trace")
	must(m.BindTrace(tags))
	check := func(v *FixedPoint, want string, zeroSlot bool) {
		t.Helper()
		got, err := m.String(v)
		if err != nil || got != want {
			t.Fatal("old product words with new sign", got, want, err)
		}
		zero, err := m.IsZero(v)
		if err != nil || zero || fixedZeroSlot(v) != zeroSlot {
			t.Fatal("source product slot after failure", zero, fixedZeroSlot(v), zeroSlot, err)
		}
		if tags.ShouldTrace("mpfx_trace") && (!strings.Contains(output.String(), " <= ") && !strings.Contains(output.String(), " >= ") || strings.HasSuffix(output.String(), want+"\n")) {
			t.Fatal("source multiplication reached result trace after overflow", output.String())
		}
		output.Reset()
	}
	must(m.SetUint64(a, (1<<32)-1))
	must(m.SetInt64(b, 2))
	must(m.SetInt64(out, -3))
	if err := m.Mul(a, b, out); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatal("positive multiplication overflow", err)
	}
	check(out, "3", false)
	must(m.Neg(a))
	must(m.Reset(out))
	if err := m.Mul(a, b, out); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatal("negative multiplication overflow into canonical zero", err)
	}
	check(out, "-0", true)
	must(m.Neg(a))
	if err := m.Mul(a, b, out); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatal("positive multiplication overflow into allocated zero", err)
	}
	check(out, "0", true)
	must(m.Neg(b))
	if err := m.Mul(a, b, a); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatal("aliased multiplication overflow", err)
	}
	check(a, "-4294967295", false)
}

func TestFixedPointDivideOverflowPublishesSignBeforeQuotientWords(t *testing.T) {
	m, err := NewFixedPointManager(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.New()
	b, _ := m.New()
	out, _ := m.New()
	defer func() {
		for _, v := range []*FixedPoint{a, b, out} {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("mpfx_trace")
	must(m.BindTrace(tags))
	check := func(v *FixedPoint, want string, slot bool) {
		t.Helper()
		got, err := m.String(v)
		if err != nil || got != want || fixedZeroSlot(v) != slot {
			t.Fatal("source quotient failure prefix", got, want, fixedZeroSlot(v), slot, err)
		}
		if tags.ShouldTrace("mpfx_trace") && (!strings.Contains(output.String(), " / ") || strings.HasSuffix(output.String(), want+"\n")) {
			t.Fatal("division overflow reached result trace", output.String())
		}
		output.Reset()
	}
	must(m.SetUint64(a, (1<<32)-1))
	must(m.SetRatio(b, 1, 2))
	must(m.SetInt64(out, -3))
	if err := m.Div(a, b, out); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatal("positive division overflow", err)
	}
	check(out, "3", false)
	must(m.Neg(a))
	must(m.Reset(out))
	if err := m.Div(a, b, out); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatal("negative division overflow into canonical zero", err)
	}
	check(out, "-0", true)
	must(m.Neg(a))
	if err := m.Div(a, b, out); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatal("positive division overflow into allocated zero", err)
	}
	check(out, "0", true)
	must(m.Neg(b))
	if err := m.Div(a, b, a); !errors.Is(err, ErrFixedPointOverflow) {
		t.Fatal("aliased division overflow", err)
	}
	check(a, "-4294967295", false)
}

// fixedZeroSlot reports the source's allocated numeral whose words are all
// zero, which is_zero does not treat as zero.
func fixedZeroSlot(v *FixedPoint) bool { return v.words != nil && fixedWordsZero(v.words) }
