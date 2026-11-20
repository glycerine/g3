package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2076-2079,2109-2137,2141-2168.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestBVRewriterAdditionCallerEveryActualBitReadReservation(t *testing.T) {
	complete := false
	failures := 0
	sawAfterCore := false
	for ordinal := int64(0); ordinal < 2048 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			b := arena.NewBudget(1 << 27)
			rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error { return f }})
			astTestMust(t, e)
			q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Memory: rt, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
			astTestMust(t, e)
			sy, e := usymbol.NewManager(usymbol.Config{Budget: b})
			astTestMust(t, e)
			m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
			astTestMust(t, e)
			t.Cleanup(func() {
				astTestMust(t, m.Close())
				astTestMust(t, sy.Close())
				astTestMust(t, q.CloseChecked())
				astTestMust(t, rt.Close())
				if b.Used() != 0 {
					t.Fatal("actual graph retained", b.Used())
				}
			})
			_, e = m.RegisterArithmeticPlugin()
			astTestMust(t, e)
			_, e = m.RegisterBitVectorPlugin()
			astTestMust(t, e)
			s, e := m.BitVectorSort(64)
			astTestMust(t, e)
			x, e := m.MkVar(0, s)
			astTestMust(t, e)
			astTestPin(t, m, x)
			low, e := m.MkBitVectorConcat([]ast.ASTHandle{bvAddNum(t, m, 0, 1), x})
			astTestMust(t, e)
			astTestPin(t, m, low)
			value, e := q.NewValue()
			astTestMust(t, e)
			astTestMust(t, q.SetString(&value, "18446744073709551616"))
			high, e := m.BitVectorNumeral(&value, 65)
			astTestMust(t, e)
			astTestPin(t, m, high)
			astTestMust(t, q.Del(&value))
			c, g, _ := bvPolynomialParamsFixture(t)
			p, e := NewBVPolynomialRewriter(m, c, g, nil)
			astTestMust(t, e)
			out, e := m.NewRef(ast.ASTHandle{})
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			args := ast.SliceArguments([]ast.ASTHandle{high, low})
			b.FailAfter(ordinal)
			st, body := p.RewriteBitVectorAddition(args, out)
			b.FailAfter(-1)
			if body != nil {
				failures++
				if !numericHoistRefusal(body) {
					t.Fatal(body)
				}
				if p.addition == nil && p.additionBits.frame != nil && *p.additionBits.frame != (ast.ASTBitVectorNumeralInvocation{}) {
					sawAfterCore = true
				}
				cause := p.cause
				if _, e = p.RewriteBitVectorAddition(args, out); e != cause {
					t.Fatal("stopped retry", e, cause)
				}
				astTestMust(t, p.Close())
				if p.cause != cause {
					t.Fatal("cleanup changed history")
				}
			} else {
				complete = true
				if st != Rewrite1 {
					t.Fatal(st)
				}
				astTestMust(t, m.BitVectorNumeralInvocationSourceReady(p.additionBits.frame))
				astTestMust(t, p.Close())
				got, e := m.RefGet(out)
				astTestMust(t, e)
				want, e := m.MkBitVectorOrArguments(args)
				astTestMust(t, e)
				astTestPin(t, m, want)
				if got != want {
					t.Fatal("wrong width65 disjoint sum", got, want)
				}
			}
		})
	}
	if !complete || failures == 0 || !sawAfterCore {
		t.Fatal("missing success or captured bit-read failure", complete, failures, sawAfterCore)
	}
}
