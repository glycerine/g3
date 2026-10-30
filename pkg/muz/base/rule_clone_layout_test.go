package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleClonePreservesNativeTailLayoutVerbatim(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	zero, err := f.manager.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	one, err := f.manager.ArithmeticIntegerNumeralText("1")
	ruleMust(t, err)
	first, err := f.manager.MkEq(f.x, zero)
	ruleMust(t, err)
	second, err := f.manager.MkEq(f.x, one)
	ruleMust(t, err)
	source, err := f.rules.NewRule(f.head, []ast.ASTHandle{first, f.head, second, f.head}, []bool{false, true, false, false}, "layout")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, source.Close()) })
	clone, err := f.rules.CloneRule(source)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, clone.Close()) })
	size, err := source.TailSize()
	ruleMust(t, err)
	for index := 0; index < size; index++ {
		want, wantNegative, getErr := source.Tail(index)
		ruleMust(t, getErr)
		got, gotNegative, getErr := clone.Tail(index)
		ruleMust(t, getErr)
		if got != want || gotNegative != wantNegative {
			t.Fatal("tail layout", index, got, gotNegative, want, wantNegative)
		}
	}
	for _, query := range []func(*base.Rule) (int, error){(*base.Rule).PositiveTailSize, (*base.Rule).UninterpretedTailSize} {
		want, err := query(source)
		ruleMust(t, err)
		got, err := query(clone)
		ruleMust(t, err)
		if got != want {
			t.Fatal("tail boundary", got, want)
		}
	}
}
