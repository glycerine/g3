package interval

// Source: src/test/ext_numeral.cpp tst1 and tst2.
import (
	"bytes"
	"fmt"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

type sourceExtendedArithmetic struct {
	op   string
	a    int32
	ak   ExtendedKind
	b    int32
	bk   ExtendedKind
	want int32
	wk   ExtendedKind
}
type sourceExtendedRelation struct {
	op   string
	a    int32
	ak   ExtendedKind
	b    int32
	bk   ExtendedKind
	want bool
}

var sourceExtendedArithmeticCases = []sourceExtendedArithmetic{
	{"add", 0, MinusInfinity, 0, MinusInfinity, 0, MinusInfinity},
	{"add", 0, MinusInfinity, 0, Finite, 0, MinusInfinity},
	{"add", 0, MinusInfinity, -1, Finite, 0, MinusInfinity},
	{"add", 0, MinusInfinity, 1, Finite, 0, MinusInfinity},
	{"add", 1, MinusInfinity, -1, Finite, 0, MinusInfinity},
	{"add", 1, Finite, 0, MinusInfinity, 0, MinusInfinity},
	{"add", -1, Finite, 0, MinusInfinity, 0, MinusInfinity},
	{"add", 0, Finite, 0, MinusInfinity, 0, MinusInfinity},
	{"add", 0, Finite, 2, Finite, 2, Finite},
	{"add", -3, Finite, 4, Finite, 1, Finite},
	{"add", -2, Finite, 0, Finite, -2, Finite},
	{"add", 3, Finite, 4, Finite, 7, Finite},
	{"add", 0, PlusInfinity, 0, PlusInfinity, 0, PlusInfinity},
	{"add", 0, PlusInfinity, 0, Finite, 0, PlusInfinity},
	{"add", 0, PlusInfinity, 1, Finite, 0, PlusInfinity},
	{"add", 0, PlusInfinity, -1, Finite, 0, PlusInfinity},
	{"add", 0, Finite, 0, PlusInfinity, 0, PlusInfinity},
	{"add", -1, Finite, 0, PlusInfinity, 0, PlusInfinity},
	{"add", 1, Finite, 0, PlusInfinity, 0, PlusInfinity},
	{"mul", 0, MinusInfinity, 0, MinusInfinity, 0, PlusInfinity},
	{"mul", 0, MinusInfinity, 0, PlusInfinity, 0, MinusInfinity},
	{"mul", 0, PlusInfinity, 0, MinusInfinity, 0, MinusInfinity},
	{"mul", 0, PlusInfinity, 0, PlusInfinity, 0, PlusInfinity},
	{"mul", 0, MinusInfinity, 0, Finite, 0, Finite},
	{"mul", 0, MinusInfinity, 1, Finite, 0, MinusInfinity},
	{"mul", 0, MinusInfinity, 5, Finite, 0, MinusInfinity},
	{"mul", 0, MinusInfinity, -1, Finite, 0, PlusInfinity},
	{"mul", 0, MinusInfinity, -5, Finite, 0, PlusInfinity},
	{"mul", 0, PlusInfinity, 0, Finite, 0, Finite},
	{"mul", 0, PlusInfinity, 1, Finite, 0, PlusInfinity},
	{"mul", 0, PlusInfinity, 5, Finite, 0, PlusInfinity},
	{"mul", 0, PlusInfinity, -1, Finite, 0, MinusInfinity},
	{"mul", 0, PlusInfinity, -5, Finite, 0, MinusInfinity},
	{"mul", 0, Finite, 3, Finite, 0, Finite},
	{"mul", 2, Finite, 3, Finite, 6, Finite},
	{"mul", -2, Finite, 3, Finite, -6, Finite},
	{"sub", 0, PlusInfinity, 0, MinusInfinity, 0, PlusInfinity},
	{"sub", 0, PlusInfinity, 0, Finite, 0, PlusInfinity},
	{"sub", 0, PlusInfinity, -10, Finite, 0, PlusInfinity},
	{"sub", 0, PlusInfinity, 10, Finite, 0, PlusInfinity},
	{"sub", 0, MinusInfinity, 0, PlusInfinity, 0, MinusInfinity},
	{"sub", 0, MinusInfinity, 0, Finite, 0, MinusInfinity},
	{"sub", 0, MinusInfinity, -10, Finite, 0, MinusInfinity},
	{"sub", 0, MinusInfinity, 10, Finite, 0, MinusInfinity},
	{"sub", 0, MinusInfinity, 0, PlusInfinity, 0, MinusInfinity},
	{"sub", 0, MinusInfinity, 0, Finite, 0, MinusInfinity},
	{"sub", 0, MinusInfinity, 3, Finite, 0, MinusInfinity},
	{"sub", 0, MinusInfinity, -3, Finite, 0, MinusInfinity},
	{"sub", 0, PlusInfinity, 0, MinusInfinity, 0, PlusInfinity},
	{"sub", 0, PlusInfinity, 0, Finite, 0, PlusInfinity},
	{"sub", 0, PlusInfinity, 3, Finite, 0, PlusInfinity},
	{"sub", 0, PlusInfinity, -3, Finite, 0, PlusInfinity},
	{"sub", 0, Finite, 2, Finite, -2, Finite},
	{"sub", 3, Finite, 2, Finite, 1, Finite},
	{"sub", 3, Finite, -3, Finite, 6, Finite},
	{"sub", 3, Finite, 3, Finite, 0, Finite},
	{"sub", 3, Finite, 0, Finite, 3, Finite},
	{"sub", -3, Finite, -5, Finite, 2, Finite},
}
var sourceExtendedRelationCases = []sourceExtendedRelation{
	{"eq", 0, Finite, 0, Finite, true},
	{"eq", 0, Finite, 2, Finite, false},
	{"eq", 3, Finite, 0, Finite, false},
	{"eq", 0, PlusInfinity, 0, Finite, false},
	{"eq", 0, PlusInfinity, 3, Finite, false},
	{"eq", 0, PlusInfinity, -2, Finite, false},
	{"eq", 0, PlusInfinity, 0, MinusInfinity, false},
	{"neq", 0, Finite, 0, Finite, false},
	{"neq", 0, Finite, 2, Finite, true},
	{"neq", 3, Finite, 0, Finite, true},
	{"neq", 0, PlusInfinity, 0, Finite, true},
	{"neq", 0, PlusInfinity, 3, Finite, true},
	{"neq", 0, PlusInfinity, -2, Finite, true},
	{"neq", 0, PlusInfinity, 0, MinusInfinity, true},
	{"lt", 0, MinusInfinity, 10, Finite, true},
	{"lt", 0, MinusInfinity, 0, Finite, true},
	{"lt", 0, MinusInfinity, -3, Finite, true},
	{"lt", 30, MinusInfinity, 10, Finite, true},
	{"lt", 20, MinusInfinity, 0, Finite, true},
	{"lt", -20, MinusInfinity, -3, Finite, true},
	{"lt", 0, MinusInfinity, 10, PlusInfinity, true},
	{"lt", 0, MinusInfinity, 0, PlusInfinity, true},
	{"lt", 10, MinusInfinity, -30, PlusInfinity, true},
	{"lt", 0, PlusInfinity, 10, Finite, false},
	{"lt", 0, PlusInfinity, 0, Finite, false},
	{"lt", 0, PlusInfinity, -3, Finite, false},
	{"lt", 30, PlusInfinity, 10, Finite, false},
	{"lt", 20, PlusInfinity, 0, Finite, false},
	{"lt", -20, PlusInfinity, -3, Finite, false},
	{"lt", 0, PlusInfinity, 10, MinusInfinity, false},
	{"lt", 0, PlusInfinity, 0, MinusInfinity, false},
	{"lt", 10, PlusInfinity, -30, MinusInfinity, false},
	{"lt", 0, Finite, 0, PlusInfinity, true},
	{"lt", 20, Finite, 10, PlusInfinity, true},
	{"lt", -20, Finite, -100, PlusInfinity, true},
	{"lt", 0, Finite, 10, Finite, true},
	{"lt", 0, Finite, 0, Finite, false},
	{"lt", 10, Finite, 10, Finite, false},
	{"lt", 0, Finite, -3, Finite, false},
	{"lt", 30, Finite, 10, Finite, false},
	{"lt", 30, Finite, 40, Finite, true},
	{"lt", 20, Finite, 0, Finite, false},
	{"lt", -20, Finite, -3, Finite, true},
	{"lt", 0, Finite, 10, MinusInfinity, false},
	{"lt", 0, Finite, 0, MinusInfinity, false},
	{"lt", 10, Finite, -30, MinusInfinity, false},
	{"le", 0, MinusInfinity, 10, Finite, true},
	{"le", 0, MinusInfinity, 0, Finite, true},
	{"le", 0, MinusInfinity, -3, Finite, true},
	{"le", 30, MinusInfinity, 10, Finite, true},
	{"le", 20, MinusInfinity, 0, Finite, true},
	{"le", -20, MinusInfinity, -3, Finite, true},
	{"le", 0, MinusInfinity, 10, PlusInfinity, true},
	{"le", 0, MinusInfinity, 0, PlusInfinity, true},
	{"le", 10, MinusInfinity, -30, PlusInfinity, true},
	{"le", 0, PlusInfinity, 10, Finite, false},
	{"le", 0, PlusInfinity, 0, Finite, false},
	{"le", 0, PlusInfinity, -3, Finite, false},
	{"le", 30, PlusInfinity, 10, Finite, false},
	{"le", 20, PlusInfinity, 0, Finite, false},
	{"le", -20, PlusInfinity, -3, Finite, false},
	{"le", 0, PlusInfinity, 10, MinusInfinity, false},
	{"le", 0, PlusInfinity, 0, MinusInfinity, false},
	{"le", 10, PlusInfinity, -30, MinusInfinity, false},
	{"le", 0, Finite, 0, PlusInfinity, true},
	{"le", 20, Finite, 10, PlusInfinity, true},
	{"le", -20, Finite, -100, PlusInfinity, true},
	{"le", 0, Finite, 10, Finite, true},
	{"le", 0, Finite, 0, Finite, true},
	{"le", 10, Finite, 10, Finite, true},
	{"le", 0, Finite, -3, Finite, false},
	{"le", 30, Finite, 10, Finite, false},
	{"le", 30, Finite, 40, Finite, true},
	{"le", 20, Finite, 0, Finite, false},
	{"le", -20, Finite, -3, Finite, true},
	{"le", 0, Finite, 10, MinusInfinity, false},
	{"le", 0, Finite, 0, MinusInfinity, false},
	{"le", 10, Finite, -30, MinusInfinity, false},
	{"ge", 0, MinusInfinity, 10, Finite, false},
	{"ge", 0, MinusInfinity, 0, Finite, false},
	{"ge", 0, MinusInfinity, -3, Finite, false},
	{"ge", 30, MinusInfinity, 10, Finite, false},
	{"ge", 20, MinusInfinity, 0, Finite, false},
	{"ge", -20, MinusInfinity, -3, Finite, false},
	{"ge", 0, MinusInfinity, 10, PlusInfinity, false},
	{"ge", 0, MinusInfinity, 0, PlusInfinity, false},
	{"ge", 10, MinusInfinity, -30, PlusInfinity, false},
	{"ge", 0, PlusInfinity, 10, Finite, true},
	{"ge", 0, PlusInfinity, 0, Finite, true},
	{"ge", 0, PlusInfinity, -3, Finite, true},
	{"ge", 30, PlusInfinity, 10, Finite, true},
	{"ge", 20, PlusInfinity, 0, Finite, true},
	{"ge", -20, PlusInfinity, -3, Finite, true},
	{"ge", 0, PlusInfinity, 10, MinusInfinity, true},
	{"ge", 0, PlusInfinity, 0, MinusInfinity, true},
	{"ge", 10, PlusInfinity, -30, MinusInfinity, true},
	{"ge", 0, Finite, 0, PlusInfinity, false},
	{"ge", 20, Finite, 10, PlusInfinity, false},
	{"ge", -20, Finite, -100, PlusInfinity, false},
	{"ge", 0, Finite, 10, Finite, false},
	{"ge", 0, Finite, 0, Finite, true},
	{"ge", 10, Finite, 10, Finite, true},
	{"ge", 0, Finite, -3, Finite, true},
	{"ge", 30, Finite, 10, Finite, true},
	{"ge", 30, Finite, 40, Finite, false},
	{"ge", 20, Finite, 0, Finite, true},
	{"ge", -20, Finite, -3, Finite, false},
	{"ge", 0, Finite, 10, MinusInfinity, true},
	{"ge", 0, Finite, 0, MinusInfinity, true},
	{"ge", 10, Finite, -30, MinusInfinity, true},
	{"gt", 0, MinusInfinity, 10, Finite, false},
	{"gt", 0, MinusInfinity, 0, Finite, false},
	{"gt", 0, MinusInfinity, -3, Finite, false},
	{"gt", 30, MinusInfinity, 10, Finite, false},
	{"gt", 20, MinusInfinity, 0, Finite, false},
	{"gt", -20, MinusInfinity, -3, Finite, false},
	{"gt", 0, MinusInfinity, 10, PlusInfinity, false},
	{"gt", 0, MinusInfinity, 0, PlusInfinity, false},
	{"gt", 10, MinusInfinity, -30, PlusInfinity, false},
	{"gt", 0, PlusInfinity, 10, Finite, true},
	{"gt", 0, PlusInfinity, 0, Finite, true},
	{"gt", 0, PlusInfinity, -3, Finite, true},
	{"gt", 30, PlusInfinity, 10, Finite, true},
	{"gt", 20, PlusInfinity, 0, Finite, true},
	{"gt", -20, PlusInfinity, -3, Finite, true},
	{"gt", 0, PlusInfinity, 10, MinusInfinity, true},
	{"gt", 0, PlusInfinity, 0, MinusInfinity, true},
	{"gt", 10, PlusInfinity, -30, MinusInfinity, true},
	{"gt", 0, Finite, 0, PlusInfinity, false},
	{"gt", 20, Finite, 10, PlusInfinity, false},
	{"gt", -20, Finite, -100, PlusInfinity, false},
	{"gt", 0, Finite, 10, Finite, false},
	{"gt", 0, Finite, 0, Finite, false},
	{"gt", 10, Finite, 10, Finite, false},
	{"gt", 0, Finite, -3, Finite, true},
	{"gt", 30, Finite, 10, Finite, true},
	{"gt", 30, Finite, 40, Finite, false},
	{"gt", 20, Finite, 0, Finite, true},
	{"gt", -20, Finite, -3, Finite, false},
	{"gt", 0, Finite, 10, MinusInfinity, true},
	{"gt", 0, Finite, 0, MinusInfinity, true},
	{"gt", 10, Finite, -30, MinusInfinity, true},
}

func TestExtendedNumeralSourceArithmeticAndRelations(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	a, b, out := numeral.NewRational(), numeral.NewRational(), numeral.NewRational()
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	defer func() {
		must(m.Del(&a))
		must(m.Del(&b))
		must(m.Del(&out))
		must(m.CloseChecked())
		if budget.Used() != 0 {
			t.Fatal("numeric ownership", budget.Used())
		}
	}()
	e := RationalExtended{Numbers: m}
	for i, tc := range sourceExtendedArithmeticCases {
		for _, swap := range []bool{false, true} {
			if swap && tc.op == "sub" {
				continue
			}
			av, ak, bv, bk := tc.a, tc.ak, tc.b, tc.bk
			if swap {
				av, ak, bv, bk = bv, bk, av, ak
			}
			must(m.SetInt32(&a, av))
			must(m.SetInt32(&b, bv))
			var kind ExtendedKind
			var err error
			switch tc.op {
			case "add":
				kind, err = e.Add(&a, ak, &b, bk, &out)
			case "sub":
				kind, err = e.Sub(&a, ak, &b, bk, &out)
			case "mul":
				kind, err = e.Mul(&a, ak, &b, bk, &out)
			}
			must(err)
			if kind != tc.wk {
				t.Fatalf("arithmetic %d %s swapped=%v kind=%v want=%v", i, tc.op, swap, kind, tc.wk)
			}
			if kind == Finite {
				text, err := m.String(&out)
				must(err)
				if text != fmt.Sprint(tc.want) {
					t.Fatalf("arithmetic %d %s got %s want %d", i, tc.op, text, tc.want)
				}
			}
		}
	}
	for i, tc := range sourceExtendedRelationCases {
		for _, swap := range []bool{false, true} {
			if swap && tc.op != "eq" && tc.op != "neq" {
				continue
			}
			av, ak, bv, bk := tc.a, tc.ak, tc.b, tc.bk
			if swap {
				av, ak, bv, bk = bv, bk, av, ak
			}
			must(m.SetInt32(&a, av))
			must(m.SetInt32(&b, bv))
			cmp, err := e.Compare(&a, ak, &b, bk)
			must(err)
			var got bool
			switch tc.op {
			case "eq":
				got = cmp == 0
			case "neq":
				got = cmp != 0
			case "lt":
				got = cmp < 0
			case "le":
				got = cmp <= 0
			case "gt":
				got = cmp > 0
			case "ge":
				got = cmp >= 0
			}
			if got != tc.want {
				t.Fatalf("relation %d %s swapped=%v got %v want %v", i, tc.op, swap, got, tc.want)
			}
		}
	}
	for _, tc := range []struct {
		value int32
		kind  ExtendedKind
		want  int32
	}{{0, MinusInfinity, 0}, {0, PlusInfinity, 0}, {1, Finite, 1}, {-1, Finite, -1}} {
		must(m.SetInt32(&a, tc.value))
		kind, err := e.Inv(&a, tc.kind)
		must(err)
		text, err := m.String(&a)
		must(err)
		if kind != Finite || text != fmt.Sprint(tc.want) {
			t.Fatal("inverse", tc, kind, text)
		}
	}
	for _, tc := range []struct {
		value int32
		kind  ExtendedKind
		want  int32
		wk    ExtendedKind
	}{
		{0, MinusInfinity, 0, PlusInfinity}, {30, MinusInfinity, 30, PlusInfinity},
		{0, Finite, 0, Finite}, {10, Finite, -10, Finite}, {-7, Finite, 7, Finite},
		{0, PlusInfinity, 0, MinusInfinity}, {30, PlusInfinity, 30, MinusInfinity},
		{-7, PlusInfinity, -7, MinusInfinity},
	} {
		if err = m.SetInt32(&a, tc.value); err != nil {
			t.Fatal(err)
		}
		kind, err := e.Neg(&a, tc.kind)
		if err != nil {
			t.Fatal(err)
		}
		text, err := m.String(&a)
		if err != nil || kind != tc.wk || text != fmt.Sprint(tc.want) {
			t.Fatalf("neg(%d, %v) = (%s, %v), want (%d, %v): %v", tc.value, tc.kind, text, kind, tc.want, tc.wk, err)
		}
	}
}

// Source: src/test/ext_numeral.cpp tst3.
func TestExtendedNumeralSourcePredicatesAndDisplay(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	value := numeral.NewRational()
	defer func() {
		if e := m.Del(&value); e != nil {
			t.Error(e)
		}
		if e := m.CloseChecked(); e != nil {
			t.Error(e)
		}
		if budget.Used() != 0 {
			t.Error("numeric ownership", budget.Used())
		}
	}()
	e := RationalExtended{Numbers: m}
	for _, tc := range []struct {
		v              int32
		k              ExtendedKind
		zero, pos, neg bool
	}{
		{0, Finite, true, false, false}, {0, PlusInfinity, false, true, false}, {0, MinusInfinity, false, false, true},
		{10, Finite, false, true, false}, {-5, Finite, false, false, true},
	} {
		if err = m.SetInt32(&value, tc.v); err != nil {
			t.Fatal(err)
		}
		z, err := e.IsZero(&value, tc.k)
		if err != nil {
			t.Fatal(err)
		}
		p, err := e.IsPos(&value, tc.k)
		if err != nil {
			t.Fatal(err)
		}
		n, err := e.IsNeg(&value, tc.k)
		if err != nil {
			t.Fatal(err)
		}
		if z != tc.zero || p != tc.pos || n != tc.neg {
			t.Fatal(tc, z, p, n)
		}
	}
	if err = m.Reset(&value); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		v    int32
		k    ExtendedKind
		want string
	}{{0, Finite, "0"}, {-10, Finite, "-10"}, {-10, PlusInfinity, "+oo"}, {-10, MinusInfinity, "-oo"}} {
		if err = m.SetInt32(&value, tc.v); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err = e.Display(&out, &value, tc.k); err != nil || out.String() != tc.want {
			t.Fatal(tc, out.String(), err)
		}
	}
}
