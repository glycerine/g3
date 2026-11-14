package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:2706-2800.

import (
	"fmt"
	"testing"
)

func TestASTBitVectorMultiplyBoundEveryActualReservation(t *testing.T) {
	for _, mode := range []string{"unsigned", "signed", "underflow", "fast"} {
		complete := false
		for ordinal := int64(0); ordinal < 2048 && !complete; ordinal++ {
			t.Run(fmt.Sprintf("%s/%d", mode, ordinal), func(t *testing.T) {
				m := bvOffsetRuntimeTestManager(t)
				_, e := m.RegisterArithmeticPlugin()
				astTestMust(t, e)
				_, e = m.RegisterBitVectorPlugin()
				astTestMust(t, e)
				v, e := m.numerals.NewValue()
				astTestMust(t, e)
				astTestMust(t, m.numerals.SetString(&v, "36893488147419103231"))
				high, e := m.BitVectorNumeral(&v, 65)
				astTestMust(t, e)
				astTestPin(t, m, high)
				astTestMust(t, m.numerals.Del(&v))
				first := high
				if mode == "fast" {
					first, e = m.BitVectorNumeralUnsigned(1, 65)
					astTestMust(t, e)
					astTestPin(t, m, first)
				}
				m.Budget().FailAfter(ordinal)
				n, body := NewBitVectorMultiplyBoundNumerals(m)
				var width uint32
				if body == nil {
					_, body = n.Read(0, first, &width)
				}
				if body == nil {
					_, body = n.Read(1, high, &width)
				}
				answer := false
				if body == nil && mode == "fast" {
					answer, body = n.ZeroOrOne(0)
				}
				if mode == "signed" || mode == "underflow" {
					// Upstream mk_bvsmul_no_overflow: 2^sz, 2^(sz-1),
					// magnitudes with signs, product, one comparison.
					var sign0, sign1 bool
					if body == nil {
						body = n.Power(2, width)
					}
					if body == nil {
						body = n.Power(3, width-1)
					}
					if body == nil {
						sign0, body = n.Magnitude(0)
					}
					if body == nil {
						sign1, body = n.Magnitude(1)
					}
					if body == nil {
						body = n.Product(4)
					}
					if body == nil {
						var below bool
						if mode == "signed" {
							below, body = n.Compare(4, 3, false)
							answer = sign0 != sign1 || below
						} else {
							below, body = n.Compare(3, 4, false)
							answer = sign0 == sign1 || !below
						}
					}
				}
				if mode == "unsigned" {
					if body == nil {
						body = n.Product(2)
					}
					if body == nil {
						body = n.Power(3, width)
					}
					if body == nil {
						answer, body = n.Compare(2, 3, false)
					}
				}
				m.Budget().FailAfter(-1)
				if body == nil {
					complete = true
					// high is 2^65-1, i.e. -1 at width 65: (-1)*(-1) = 1
					// neither overflows nor underflows; unsigned it does.
					if answer != (mode != "unsigned") {
						t.Fatal("original bound", mode, answer)
					}
					if mode == "signed" || mode == "underflow" {
						for _, entry := range []struct {
							i    int
							text string
						}{{0, "1"}, {1, "1"}, {3, "18446744073709551616"}, {4, "1"}} {
							text, e := m.numerals.String(&n.values[entry.i])
							astTestMust(t, e)
							if text != entry.text {
								t.Fatal(entry, text)
							}
						}
					}
					if mode == "fast" && n.placed[2] {
						t.Fatal("fast path constructed product")
					}
					astTestMust(t, n.SourceEnd())
				} else if n != nil {
					if _, e = n.ZeroOrOne(0); e == nil {
						t.Fatal("stopped replay")
					}
				}
				if n != nil {
					b, s, _ := n.scalar.Causes()
					astTestMust(t, n.Close())
					afterB, afterS, _ := n.scalar.Causes()
					if b != afterB || s != afterS {
						t.Fatal("cleanup changed source history")
					}
				}
			})
		}
		if !complete {
			t.Fatal("actual campaign incomplete", mode)
		}
	}
}
