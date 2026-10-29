package sls

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestBVEvaluatorConcatSourceOrderAndIndependentOracle(t *testing.T) {
	manager, budget := powersManager(t, 1<<28)
	p, err := newPowers(manager)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := newBVEvaluatorSlice(manager, p)
	if err != nil {
		t.Fatal(err)
	}

	values := []numeral.Int{numeral.NewInt(0x12), numeral.NewInt(0x345), numeral.NewInt(6)}
	args := []*numeral.Int{&values[0], &values[1], &values[2]}
	widths := []uint32{8, 12, 4}
	result := numeral.NewInt(-1)
	if err := evaluator.concat(args, widths, &result); err != nil {
		t.Fatal(err)
	}

	want := big.NewInt(0x12)
	want.Lsh(want, 12).Add(want, big.NewInt(0x345))
	want.Lsh(want, 4).Add(want, big.NewInt(6))
	text, err := manager.String(&result)
	if err != nil || text != want.String() {
		t.Fatalf("concat = %q, want %s, err %v", text, want, err)
	}
	if p.byShift[12] == nil || p.byShift[4] == nil || len(p.byShift) != 2 {
		t.Fatalf("source power lookup order/cache keys: %v", p.byShift)
	}
	for i := range values {
		text, err = manager.String(&values[i])
		if err != nil || text != []string{"18", "837", "6"}[i] {
			t.Fatal("borrowed tracker value changed", i, text, err)
		}
	}

	if err := manager.Del(&result); err != nil {
		t.Fatal(err)
	}
	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if budget.Used() != 0 {
		t.Fatal("concat owner leak", budget.Used())
	}
}

func TestBVEvaluatorExtractSourceFormulaAndCachedPower(t *testing.T) {
	manager, budget := powersManager(t, 1<<28)
	p, err := newPowers(manager)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := newBVEvaluatorSlice(manager, p)
	if err != nil {
		t.Fatal(err)
	}
	var child, result numeral.Int
	const input = "1208925819614629174718513"
	if err := manager.SetString(&child, input); err != nil {
		t.Fatal(err)
	}
	if err := evaluator.extract(&child, 70, 12, &result); err != nil {
		t.Fatal(err)
	}

	source, ok := new(big.Int).SetString(input, 10)
	if !ok {
		t.Fatal("test oracle input")
	}
	modulus := new(big.Int).Lsh(big.NewInt(1), 71)
	want := new(big.Int).Rem(source, modulus)
	want.Rsh(want, 12)
	text, err := manager.String(&result)
	if err != nil || text != want.String() {
		t.Fatalf("extract = %q, want %s, err %v", text, want, err)
	}
	if p.byShift[71] == nil || len(p.byShift) != 1 {
		t.Fatalf("extract did not borrow 2^(high+1): %v", p.byShift)
	}
	childText, err := manager.String(&child)
	if err != nil || childText != input {
		t.Fatal("borrowed child changed", childText, err)
	}

	if err := manager.Del(&result); err != nil {
		t.Fatal(err)
	}
	if err := manager.Del(&child); err != nil {
		t.Fatal(err)
	}
	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
	if budget.Used() != 0 {
		t.Fatal("extract owner leak", budget.Used())
	}
}

func TestBVEvaluatorSliceRejectsUnreachableShapesAndEndedPowerOwner(t *testing.T) {
	manager, _ := powersManager(t, 1<<28)
	p, err := newPowers(manager)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := newBVEvaluatorSlice(manager, p)
	if err != nil {
		t.Fatal(err)
	}
	var a, out numeral.Int
	if err := evaluator.concat([]*numeral.Int{&a}, []uint32{1}, &out); !errors.Is(err, errEvaluatorArity) {
		t.Fatal("concat arity", err)
	}
	if err := evaluator.extract(&a, 3, 4, &out); !errors.Is(err, errEvaluatorWidth) {
		t.Fatal("extract range", err)
	}
	if err := evaluator.extract(&a, math.MaxUint32, 0, &out); !errors.Is(err, errEvaluatorWidth) {
		t.Fatal("extract high wrap", err)
	}
	if err := p.closeChecked(); err != nil {
		t.Fatal(err)
	}
	if err := evaluator.extract(&a, 0, 0, &out); !errors.Is(err, errEvaluatorOwner) {
		t.Fatal("ended power owner", err)
	}
	if err := manager.CloseChecked(); err != nil {
		t.Fatal(err)
	}
}
