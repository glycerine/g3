package ast

// Source: src/ast/arith_decl_plugin.cpp:400-449,561-577,627-630.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestASTArithmeticOriginalNumeralFactory(t *testing.T) {
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	q := m.numerals
	n, e := q.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, q.Del(&n)) }()
	for _, row := range []struct {
		text    string
		integer bool
		sort    int
		flag    int32
		pinned  bool
	}{
		{"0", true, 1, 1, true}, {"15", true, 1, 1, true}, {"16", true, 1, 1, false}, {"-1", true, 1, 1, false},
		{"0", false, 0, 0, true}, {"15", false, 0, 0, true}, {"16", false, 0, 0, false}, {"1/3", false, 0, 0, false},
		{"18446744073709551617", true, 1, 1, false},
	} {
		astTestMust(t, q.SetString(&n, row.text))
		h, e := m.ArithmeticNumeral(&n, row.integer)
		astTestMust(t, e)
		s, e := m.GetSort(h)
		astTestMust(t, e)
		if s != m.external(m.arithmeticPlugin.fields[row.sort]) {
			t.Fatal("numeral range", row.text, s)
		}
		d, e := m.GetDecl(h)
		astTestMust(t, e)
		dr, e := m.internal(d)
		astTestMust(t, e)
		info, e := m.funcInfo(m.declAt(dr.slot).info)
		astTestMust(t, e)
		p, e := m.infoParameter(info, 1)
		astTestMust(t, e)
		flag, e := m.parameter(p)
		astTestMust(t, e)
		if info.kind != 0 || info.count != 2 || flag.integer != row.flag {
			t.Fatal("original retained parameter", row.text)
		}
		p, e = m.infoParameter(info, 0)
		astTestMust(t, e)
		value, e := m.parameter(p)
		astTestMust(t, e)
		equal, e := q.Equal(&value.Rational, &n)
		astTestMust(t, e)
		if !equal {
			t.Fatal("original rational value", row.text)
		}
		rc, e := m.GetRefCount(h)
		astTestMust(t, e)
		if (rc > 0) != row.pinned {
			t.Fatal("small-number cache pin", row.text, rc)
		}
		astTestPin(t, m, h)
		again, e := m.ArithmeticNumeral(&n, row.integer)
		astTestMust(t, e)
		if again != h {
			t.Fatal("canonical numeral", row.text)
		}
	}
	astTestMust(t, q.SetString(&n, "1/3"))
	if _, e := m.ArithmeticNumeral(&n, true); e == nil {
		t.Fatal("fraction accepted as integer")
	}
}

func TestASTArithmeticOriginalConvertedNumeralsAndDeclaration(t *testing.T) {
	m := astTestManager(t)
	fid, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	p := m.arithmeticPlugin
	var names ASTBuiltinNames
	astTestMust(t, m.GetArithmeticSortNames(&names, astTestSymbol(t, m, "NRA")))
	names.Close()
	n, e := m.numerals.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, m.numerals.Del(&n)) }()
	for _, row := range []struct {
		n    int64
		flag int32
	}{{0, 0}, {15, 0}, {16, 1}, {-1, 1}} {
		astTestMust(t, m.numerals.SetInt64(&n, row.n))
		h, e := m.ArithmeticNumeral(&n, true)
		astTestMust(t, e)
		astTestPin(t, m, h)
		s, e := m.GetSort(h)
		astTestMust(t, e)
		if s != m.external(p.fields[0]) {
			t.Fatal("converted range", row.n)
		}
		r, e := m.internal(h)
		astTestMust(t, e)
		info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
		astTestMust(t, e)
		parameter, e := m.infoParameter(info, 1)
		astTestMust(t, e)
		flag, e := m.parameter(parameter)
		astTestMust(t, e)
		if flag.integer != row.flag {
			t.Fatal("cached versus uncached conversion flag", row.n, flag.integer)
		}
		if row.n == 15 {
			m.Budget().FailAfter(0)
			again, e := m.ArithmeticNumeral(&n, true)
			m.Budget().FailAfter(-1)
			astTestMust(t, e)
			if again != h {
				t.Fatal("cached lookup identity")
			}
		}
	}
	// mk_num_decl selects range solely from its passed integer flag, does not
	// consult conversion mode, and does not reject a fractional Int parameter.
	astTestMust(t, m.numerals.SetString(&n, "1/3"))
	rp, e := m.NewParameterRational(&n)
	astTestMust(t, e)
	ip, e := m.NewParameterInt(2)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelParameter(&ip)); astTestMust(t, m.DelParameter(&rp)) }()
	h, e := m.MkFamilyApp(fid, 0, []ASTParameter{rp, ip}, nil)
	astTestMust(t, e)
	s, e := m.GetSort(h)
	astTestMust(t, e)
	if s != m.external(p.fields[1]) {
		t.Fatal("nonzero declaration flag is Int despite conversion")
	}
	for _, ps := range [][]ASTParameter{nil, {ip, rp}, {rp}, {rp, ip, rp}} {
		if _, e := m.MkFamilyApp(fid, 0, ps, nil); e == nil {
			t.Fatal("invalid numeral parameters accepted", ps)
		}
	}
	if _, e := m.MkFamilyApp(fid, 0, []ASTParameter{rp, ip}, []ASTHandle{h}); e == nil {
		t.Fatal("nonconstant numeral accepted")
	}
}

func TestASTArithmeticOriginalValueAndZeroCallbacks(t *testing.T) {
	m := astTestManager(t)
	fid, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	for _, index := range []int{0, 1} {
		s := m.external(m.arithmeticPlugin.fields[index])
		h, e := m.GetSomeValue(s)
		astTestMust(t, e)
		value, e := m.IsValue(h)
		astTestMust(t, e)
		unique, e := m.IsUniqueValue(h)
		astTestMust(t, e)
		if !value || !unique {
			t.Fatal("numeric zero must be a unique value")
		}
		r, e := m.internal(h)
		astTestMust(t, e)
		info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
		astTestMust(t, e)
		token, e := m.infoParameter(info, 0)
		astTestMust(t, e)
		p, e := m.parameter(token)
		astTestMust(t, e)
		zero, e := m.numerals.IsZero(&p.Rational)
		astTestMust(t, e)
		if !zero {
			t.Fatal("source zero completion")
		}
		proc := arrayValueProc{values: map[ASTHandle]ASTHandle{s: h}}
		m.Budget().FailAfter(0)
		got, e := m.GetSomeValueWithProc(s, &proc)
		m.Budget().FailAfter(-1)
		astTestMust(t, e)
		if got != h || m.someValueProc != nil {
			t.Fatal("override priority")
		}
	}
	for _, row := range []struct {
		kind   int32
		unique bool
	}{{0, true}, {1, false}, {31, true}, {32, true}, {33, false}} {
		d, e := m.makePluginDeclaration(fid, fmt.Sprint("arith-value-kind-", row.kind), row.kind, nil, m.arithmeticPlugin.fields[0], ASTFuncFlags{})
		astTestMust(t, e)
		r, e := m.mkApp(d, nil)
		astTestMust(t, e)
		h := m.external(r)
		got, e := m.IsValue(h)
		astTestMust(t, e)
		unique, e := m.IsUniqueValue(h)
		astTestMust(t, e)
		if got != (row.kind != 33) || unique != row.unique {
			t.Fatal("source value versus unique-value predicate", row.kind, got, unique)
		}
	}
}

func TestASTArithmeticNumeralEveryCachePublicationReservation(t *testing.T) {
	succeeded := false
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m := astTestManager(t)
			_, e := m.RegisterArithmeticPlugin()
			astTestMust(t, e)
			n, e := m.numerals.NewValue()
			astTestMust(t, e)
			defer func() { astTestMust(t, m.numerals.Del(&n)) }()
			astTestMust(t, m.numerals.SetInt64(&n, 15))
			m.Budget().FailAfter(ordinal)
			h, e := m.ArithmeticNumeral(&n, true)
			m.Budget().FailAfter(-1)
			if m.formatExecuting || m.formatLexical.active {
				t.Fatal("factory scope retained", ordinal)
			}
			if e == nil {
				rc, e := m.GetRefCount(h)
				astTestMust(t, e)
				if rc != 1 || len(m.arithmeticPlugin.small[0]) != 16 {
					t.Fatal("published cache owner", ordinal, rc)
				}
				for _, hole := range m.arithmeticPlugin.small[0][:15] {
					if hole.slot != 0 {
						t.Fatal("native cache gap published a node")
					}
				}
				used := m.Budget().Used()
				m.Budget().FailAfter(0)
				for repeat := 0; repeat < 3; repeat++ {
					got, e := m.ArithmeticNumeral(&n, true)
					astTestMust(t, e)
					if got != h {
						t.Fatal("cached numeral identity")
					}
				}
				m.Budget().FailAfter(-1)
				refs, e := m.GetRefCount(h)
				astTestMust(t, e)
				if refs != 1 || m.Budget().Used() != used {
					t.Fatal("cached lookup acquired another pin or storage")
				}
				succeeded = true
				return
			}
			var failure umemory.Failure
			if !errors.Is(e, arena.ErrInjectedFailure) && (!errors.As(e, &failure) || failure.Kind != umemory.FailureMemout) {
				t.Fatal("cache refusal cause", ordinal, e)
			}
		})
		if succeeded {
			return
		}
	}
	t.Fatal("cache publication campaign never succeeded")
}

func TestASTArithmeticBigNumeralEveryCopyReservation(t *testing.T) {
	succeeded := false
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m := astTestManager(t)
			_, e := m.RegisterArithmeticPlugin()
			astTestMust(t, e)
			n, e := m.numerals.NewValue()
			astTestMust(t, e)
			defer func() { astTestMust(t, m.numerals.Del(&n)) }()
			const literal = "18446744073709551617/18446744073709551619"
			astTestMust(t, m.numerals.SetString(&n, literal))
			m.Budget().FailAfter(ordinal)
			h, e := m.ArithmeticNumeral(&n, false)
			m.Budget().FailAfter(-1)
			if e != nil {
				var failure umemory.Failure
				if !errors.Is(e, arena.ErrInjectedFailure) && (!errors.As(e, &failure) || failure.Kind != umemory.FailureMemout) {
					t.Fatal("numeric copy refusal", ordinal, e)
				}
				return
			}
			r, e := m.internal(h)
			astTestMust(t, e)
			info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
			astTestMust(t, e)
			token, e := m.infoParameter(info, 0)
			astTestMust(t, e)
			p, e := m.parameter(token)
			astTestMust(t, e)
			got, e := m.numerals.String(&p.Rational)
			astTestMust(t, e)
			if got != literal {
				t.Fatal("copied source rational", ordinal, got)
			}
			succeeded = true
		})
		if succeeded {
			return
		}
	}
	t.Fatal("numeric copy campaign never succeeded")
}

func TestASTArithmeticNumeralCachesSourceFinalize(t *testing.T) {
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	n, e := m.numerals.NewValue()
	astTestMust(t, e)
	for _, integer := range []bool{true, false} {
		for _, value := range []int64{15, 0, 7} {
			astTestMust(t, m.numerals.SetInt64(&n, value))
			_, e := m.ArithmeticNumeral(&n, integer)
			astTestMust(t, e)
		}
	}
	astTestMust(t, m.numerals.Del(&n))
	astTestMust(t, m.SourceFinalize())
	if m.arithmeticPlugin != nil {
		t.Fatal("completed cache owner was not released")
	}
	astTestMust(t, m.Close())
}

func TestArithmeticIntegralInt32KeepsNumeralSorts(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	for _, tc := range []struct {
		text           string
		integer, small bool
		want           int32
	}{
		{"-7", true, true, -7}, {"-7", false, true, -7},
		{"1/2", false, false, 0}, {"2147483648", true, false, 0},
	} {
		var h ASTHandle
		if tc.integer {
			h, err = m.ArithmeticIntegerNumeralText(tc.text)
		} else {
			h, err = m.ArithmeticRealNumeralText(tc.text)
		}
		astTestMust(t, err)
		got, ok, err := m.ArithmeticIntegralInt32(h)
		astTestMust(t, err)
		if ok != tc.small || ok && got != tc.want {
			t.Errorf("numeral %s (int=%v) = %d, %v", tc.text, tc.integer, got, ok)
		}
		_, onlyInt, err := m.ArithmeticSmallInteger(h)
		astTestMust(t, err)
		if onlyInt != (tc.integer && tc.small) {
			t.Errorf("Int-only numeral %s (int=%v) matched %v", tc.text, tc.integer, onlyInt)
		}
	}
}
