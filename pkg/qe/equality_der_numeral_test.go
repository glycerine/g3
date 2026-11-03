package qe

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestEqualityDERStoredNumeralCoefficients(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	d, err := NewEqualityDER(m, nil, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, d.Close()) })
	c := &equalityDERCall{owner: d}
	d.pending = append(d.pending, c)
	coefficient, err := c.rational()
	fmMust(t, err)
	wide := strings.Repeat("9", 600)
	cases := []struct {
		text                string
		integer, invertible bool
	}{
		{"0", true, false}, {"1", true, true}, {"-1", true, true},
		{"2", true, false}, {wide, true, false}, {"0", false, false},
		{"2", false, true}, {"-2/3", false, true}, {wide, false, true},
	}
	for _, tc := range cases {
		var h ast.ASTHandle
		if tc.integer {
			h = f.own(m.ArithmeticIntegerNumeralText(tc.text))
		} else {
			h = f.own(m.ArithmeticRealNumeralText(tc.text))
		}
		got, err := d.invertibleConstant(c, tc.integer, h, coefficient)
		fmMust(t, err)
		text, err := m.NumeralService().String(coefficient)
		fmMust(t, err)
		if got != tc.invertible || text != tc.text {
			t.Fatalf("integer=%v coefficient=%s: invertible=%v copied=%s", tc.integer, tc.text, got, text)
		}
		if c.numeralRead.receiver != nil {
			t.Fatal("completed coefficient read still owns its receiver")
		}
	}
	minus := f.own(m.MkFamilyApp(d.arithmetic, 8, nil, []ast.ASTHandle{f.number("1")}))
	got, err := d.invertibleConstant(c, true, minus, coefficient)
	fmMust(t, err)
	text, err := m.NumeralService().String(coefficient)
	fmMust(t, err)
	if !got || text != "-1" {
		t.Fatal("source unary minus coefficient", got, text)
	}
	reader := c.numeralRead
	variable := f.variable(0, f.integer)
	got, err = d.invertibleConstant(c, true, variable, coefficient)
	fmMust(t, err)
	text, err = m.NumeralService().String(coefficient)
	fmMust(t, err)
	if got || text != "-1" || reader != c.numeralRead {
		t.Fatal("non-numeral changed coefficient or replaced reusable reader", got, text)
	}
	fmMust(t, d.drain())
	fmMust(t, d.drain())
}

func TestEqualityDERNumeralReadAllocationAndNativeFailure(t *testing.T) {
	var c *equalityDERCall
	active, reached := false, false
	f := newFMFixtureTransport(t, false, func(failure memory.Failure) error {
		if active {
			reached = true
			reader := c.numeralRead
			if err := c.close(); !errors.Is(err, arena.ErrInvalidSlot) && !errors.Is(err, numeral.ErrRationalLexicalPending) {
				t.Errorf("executing native copy did not refuse reader cleanup: %v", err)
			}
			if c.numeralRead != reader || reader.receiver == nil || reader.bytes == 0 || len(c.numbers) != 1 {
				t.Error("refused native copy cleanup discarded its concrete owner")
			}
		}
		return failure
	})
	m := f.r.manager
	h := f.own(m.ArithmeticRealNumeralText(strings.Repeat("9", 600)))
	d, err := NewEqualityDER(m, nil, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, d.Close()) })
	c = &equalityDERCall{owner: d}
	d.pending = append(d.pending, c)
	coefficient, err := c.rational()
	fmMust(t, err)
	fmMust(t, m.NumeralService().SetInt64(coefficient, 7))
	m.Budget().FailAfter(0)
	_, err = d.invertibleConstant(c, false, h, coefficient)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || c.numeralRead != nil {
		t.Fatal("failed concrete reader reservation changed ownership", err)
	}
	text, err := m.NumeralService().String(coefficient)
	fmMust(t, err)
	if text != "7" {
		t.Fatal("failed reader reservation changed destination", text)
	}
	fmMust(t, f.runtime.SetMaxSize(1))
	active = true
	_, body := d.invertibleConstant(c, false, h, coefficient)
	active = false
	fmMust(t, f.runtime.SetMaxSize(0))
	var failure memory.Failure
	if body == nil || !errors.As(body, &failure) || !reached {
		t.Fatalf("stored numeral copy did not reach native allocation failure: %v", body)
	}
	fmMust(t, d.drain())
	fmMust(t, d.drain())
	if *coefficient != (numeral.OwnedRational{}) || c.numeralRead != nil {
		t.Fatal("native failure cleanup left a coefficient or copy owner")
	}
	// Reuse the actual DER after the failed call, with the original AST numeral.
	c = &equalityDERCall{owner: d}
	d.pending = append(d.pending, c)
	coefficient, err = c.rational()
	fmMust(t, err)
	matched, err := d.invertibleConstant(c, false, h, coefficient)
	fmMust(t, err)
	text, err = m.NumeralService().String(coefficient)
	fmMust(t, err)
	if !matched || text != strings.Repeat("9", 600) {
		t.Fatal("native failure changed the source numeral or blocked reuse")
	}
	fmMust(t, d.drain())
}
