package polynomial

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestUPolynomialFullTransformFiniteOracles(t *testing.T) {
	m, q, _ := upolySourceOwner(t)
	p := upolySourceNew(t, m, []int32{1, 2, 3})
	out := upolySourceNew(t, m, nil)
	b := unumeral.NewBinaryIntPower(1, 1)
	r := unumeral.NewRational()
	if e := q.SetFraction(&r, 1, 3); e != nil {
		t.Fatal(e)
	}
	defer q.Del(&r)
	c := unumeral.NewInt(2)
	three := unumeral.NewInt(3)
	for _, f := range []struct {
		name string
		call func(UPolynomialView) error
		want []int64
	}{
		{"one", m.TranslateOneViewInPlace, []int64{6, 8, 3}},
		{"source_shadowed_k", func(v UPolynomialView) error { return m.TranslatePowerTwoViewInPlace(v, 7) }, []int64{9, 14, 3}},
		{"integer", func(v UPolynomialView) error { return m.TranslateIntegerViewInPlace(v, &c) }, []int64{17, 14, 3}},
		{"binary", func(v UPolynomialView) error { return m.TranslateBinaryViewInPlace(v, &b) }, []int64{11, 20, 12}},
		{"rational", func(v UPolynomialView) error { return m.TranslateRationalViewInPlace(v, &r) }, []int64{18, 36, 27}},
		{"two_degree_divide_x", m.ComposeTwoDegreeDivideXViewInPlace, []int64{4, 4, 3}},
		{"two_power_degree_divide_x", func(v UPolynomialView) error { return m.ComposePowerTwoDegreeDivideXViewInPlace(v, 2) }, []int64{16, 8, 3}},
		{"two_power_x", func(v UPolynomialView) error { return m.ComposePowerTwoXViewInPlace(v, 2) }, []int64{1, 8, 48}},
		{"integer_degree_divide_x", func(v UPolynomialView) error { return m.ComposeIntegerDegreeDivideXViewInPlace(v, &three) }, []int64{9, 6, 3}},
		{"integer_x", func(v UPolynomialView) error { return m.ComposeIntegerXViewInPlace(v, &three) }, []int64{1, 6, 27}},
		{"binary_x", func(v UPolynomialView) error { return m.ComposeBinaryXViewInPlace(v, &b) }, []int64{4, 4, 3}},
		{"rational_x", func(v UPolynomialView) error { return m.ComposeRationalXViewInPlace(v, &r) }, []int64{9, 6, 3}},
		{"negative_x", m.PMinusXViewInPlace, []int64{1, -2, 3}},
		{"reciprocal", m.ReciprocalViewInPlace, []int64{3, 2, 1}},
	} {
		if e := m.Copy(out, p); e != nil {
			t.Fatal(e)
		}
		if e := f.call(mustUPolyView(t, m, out)); e != nil {
			t.Fatal(f.name, e)
		}
		upolyGCDExpect(t, m, out, f.want)
	}
	// All five literal header Set-before-transform delegations execute.
	for _, f := range []struct {
		call func() error
		want []int64
	}{
		{func() error { return m.TranslateOne(mustUPolyView(t, m, p), out) }, []int64{6, 8, 3}},
		{func() error { return m.TranslatePowerTwo(mustUPolyView(t, m, p), 99, out) }, []int64{9, 14, 3}},
		{func() error { return m.TranslateInteger(mustUPolyView(t, m, p), &c, out) }, []int64{17, 14, 3}},
		{func() error { return m.TranslateBinary(mustUPolyView(t, m, p), &b, out) }, []int64{11, 20, 12}},
		{func() error { return m.TranslateRational(mustUPolyView(t, m, p), &r, out) }, []int64{18, 36, 27}},
	} {
		if e := f.call(); e != nil {
			t.Fatal(e)
		}
		upolyGCDExpect(t, m, out, f.want)
	}
}
func TestUPolynomialRootCountBoundsSourceBranches(t *testing.T) {
	m, q, b := upolySourceOwner(t)
	p := upolySourceNew(t, m, []int32{-2, 0, 1})
	out := upolySourceNew(t, m, nil)
	changes, e := m.SignChanges(mustUPolyView(t, m, p))
	if e != nil || changes != 1 {
		t.Fatal(changes, e)
	}
	bound, e := m.DescartesBound(mustUPolyView(t, m, p))
	if e != nil || bound != 1 {
		t.Fatal(bound, e)
	}
	bound, e = m.DescartesBoundZeroOne(mustUPolyView(t, m, p))
	if e != nil || bound != 0 {
		t.Fatal(bound, e)
	}
	var upper unumeral.Int
	if e = m.RootUpperBound(mustUPolyView(t, m, p), &upper); e != nil {
		t.Fatal(e)
	}
	n, e := m.IntegerManager().Int64(&upper)
	if e != nil || n != 4 {
		t.Fatal(n, e)
	}
	if e = m.IntegerManager().Del(&upper); e != nil {
		t.Fatal(e)
	}
	positive, e := m.KnuthPositiveRootUpperBound(mustUPolyView(t, m, p))
	if e != nil || positive != 2 {
		t.Fatal(positive, e)
	}
	negative, e := m.KnuthNegativeRootUpperBound(mustUPolyView(t, m, p))
	if e != nil || negative != 2 {
		t.Fatal(negative, e)
	}
	lower, e := m.NonzeroRootLowerBound(mustUPolyView(t, m, p))
	if e != nil || lower != 1 {
		t.Fatal(lower, e)
	}
	upolyGCDExpect(t, m, p, []int64{-2, 0, 1})
	z := upolySourceNew(t, m, []int32{0, 0, 6, -3})
	zero, e := m.HasZeroRoots(mustUPolyView(t, m, z))
	if e != nil || !zero {
		t.Fatal(zero, e)
	}
	if e = m.RemoveZeroRoots(mustUPolyView(t, m, z), out); e != nil {
		t.Fatal(e)
	}
	upolyGCDExpect(t, m, out, []int64{6, -3})
	diagnostics, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{PageSize: 8, Budget: b}, Words: arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 8, Budget: b}}}})
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := diagnostics.Close(); e != nil {
			t.Error(e)
		}
	}()
	bqm, e := unumeral.NewBinaryRationalManager(q.Integers(), unumeral.BinaryRationalConfig{Diagnostics: diagnostics})
	if e != nil {
		t.Fatal(e)
	}
	defer bqm.Close() // actual borrowed numeric owner Close is void
	for _, f := range []struct {
		a, b int32
		want uint32
	}{{0, 1, 0}, {1, 2, 1}, {-2, -1, 1}, {-2, 2, 2}, {-1, 1, 0}} {
		a, binaryB := unumeral.NewBinaryInt(f.a), unumeral.NewBinaryInt(f.b)
		count, e := m.DescartesBoundInterval(mustUPolyView(t, m, p), bqm, &a, &binaryB)
		if e != nil || count != f.want {
			t.Fatal(f, count, e)
		}
	}
	before := b.Used()
	if before == 0 {
		t.Fatal("real borrowed owner graph")
	}
}

// AUDIT.md math_polynomial_11-F1: translate_bq and translate_q trace the
// polynomial after step 1 and after the special translation.
func TestUPolynomialTranslateRationalTrace(t *testing.T) {
	m, q, _ := upolySourceOwner(t)
	b := unumeral.NewBinaryIntPower(1, 1)
	r := unumeral.NewRational()
	if e := q.SetFraction(&r, 1, 3); e != nil {
		t.Fatal(e)
	}
	defer q.Del(&r)
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("upolynomial")
	if !tags.ShouldTrace("upolynomial") {
		t.Skip("TRACE compiled out")
	}
	for _, tc := range []struct {
		call func(UPolynomialView) error
		want []string
	}{
		{func(v UPolynomialView) error { return m.TranslateBinaryViewInPlace(v, &b) },
			[]string{"translate_bq src/math/polynomial/upolynomial.cpp:1543", "after compose 2kn_p_x_div_2k\n3 x^2 + 4 x + 4\n", "translate_bq src/math/polynomial/upolynomial.cpp:1556", "after special translation\n12 x^2 + 20 x + 11\n"}},
		{func(v UPolynomialView) error { return m.TranslateRationalViewInPlace(v, &r) },
			[]string{"translate_q src/math/polynomial/upolynomial.cpp:1565", "after compose_an_p_x_div_a\n3 x^2 + 6 x + 9\n", "translate_q src/math/polynomial/upolynomial.cpp:1578", "after special translation\n27 x^2 + 36 x + 18\n"}},
	} {
		p := upolySourceNew(t, m, []int32{1, 2, 3})
		text.Reset()
		serviceMust(t, m.config.Integer.BindTrace(tags))
		e := tc.call(mustUPolyView(t, m, p))
		serviceMust(t, m.config.Integer.BindTrace(nil))
		serviceMust(t, e)
		pos := 0
		for _, part := range tc.want {
			i := strings.Index(text.String()[pos:], part)
			if i < 0 {
				t.Fatalf("missing/out-of-order %q in\n%s", part, text.String())
			}
			pos += i + len(part)
		}
	}
}
