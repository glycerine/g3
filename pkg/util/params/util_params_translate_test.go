package params

import (
	"math"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestParamsTranslateToIndependentServices(t *testing.T) {
	source, sourceSymbols, sourceNumbers := paramsTestOwner(t)
	target, targetSymbols, targetNumbers := paramsTestOwner(t)
	original, err := source.NewValue()
	paramsMust(t, err)
	defer func() {
		if original.Owner != 0 {
			paramsMust(t, source.Del(&original))
		}
	}()
	flag, amount, ratio, textKey, emptyKey, symbolKey, numKey := "flag", "amount", "ratio", "text", "empty", "symbol", "rational"
	paramsMust(t, source.SetBoolName(&original, &flag, true))
	paramsMust(t, source.SetUIntName(&original, &amount, 42))
	paramsMust(t, source.SetDoubleName(&original, &ratio, math.Copysign(0, -1)))
	text := "hello"
	paramsMust(t, source.SetStringName(&original, &textKey, &text))
	paramsMust(t, source.SetStringName(&original, &emptyKey, nil))
	sym, err := sourceSymbols.Intern("choice")
	paramsMust(t, err)
	paramsMust(t, source.SetSymbolName(&original, &symbolKey, sym))
	rational, err := sourceNumbers.NewValue()
	paramsMust(t, err)
	paramsMust(t, sourceNumbers.SetString(&rational, "123456789012345678901/79"))
	paramsMust(t, source.SetRatName(&original, &numKey, &rational))
	paramsMust(t, sourceNumbers.Del(&rational))
	paramsMust(t, source.SetUInt(&original, usymbol.Numerical(17), 23))
	copy, err := source.TranslateTo(&original, target)
	paramsMust(t, err)
	defer func() { paramsMust(t, target.Del(&copy)) }()
	paramsMust(t, source.SetBoolName(&original, &flag, false))
	paramsMust(t, source.Del(&original))
	b, err := target.GetBoolName(&copy, &flag, false)
	paramsMust(t, err)
	if !b {
		t.Fatal("boolean changed")
	}
	u, err := target.GetUIntName(&copy, &amount, 0)
	paramsMust(t, err)
	if u != 42 {
		t.Fatal("uint", u)
	}
	d, err := target.GetDoubleName(&copy, &ratio, 1)
	paramsMust(t, err)
	if d != 0 || !math.Signbit(d) {
		t.Fatal("double", d)
	}
	got, present, err := target.GetStringName(&copy, &textKey, nil)
	paramsMust(t, err)
	if !present || got != "hello" {
		t.Fatal("string", got, present)
	}
	_, present, err = target.GetStringName(&copy, &emptyKey, nil)
	paramsMust(t, err)
	if present {
		t.Fatal("null string changed")
	}
	translated, err := target.GetSymbolName(&copy, &symbolKey, usymbol.Symbol{})
	paramsMust(t, err)
	spelling, err := targetSymbols.String(translated)
	paramsMust(t, err)
	if spelling != "choice" {
		t.Fatal("symbol", spelling)
	}
	r, err := target.GetRatName(&copy, &numKey, nil)
	paramsMust(t, err)
	got, err = targetNumbers.String(&r)
	paramsMust(t, err)
	paramsMust(t, targetNumbers.Del(&r))
	if got != "123456789012345678901/79" {
		t.Fatal("rational", got)
	}
	u, err = target.GetUInt(&copy, usymbol.Numerical(17), 0)
	paramsMust(t, err)
	if u != 23 {
		t.Fatal("numeric key", u)
	}
}

func TestParamsTranslateFailureLeavesSourceUsable(t *testing.T) {
	source, _, _ := paramsTestOwner(t)
	target, _, _ := paramsTestOwner(t)
	value, err := source.NewValue()
	paramsMust(t, err)
	defer func() { paramsMust(t, source.Del(&value)) }()
	key := "translation-failure-key"
	paramsMust(t, source.SetUIntName(&value, &key, 19))
	target.Budget().FailAfter(0)
	translated, err := source.TranslateTo(&value, target)
	target.Budget().FailAfter(-1)
	if err == nil || translated != (ParamsRef{}) {
		t.Fatal("expected clean failure", translated, err)
	}
	n, err := source.GetUIntName(&value, &key, 0)
	paramsMust(t, err)
	if n != 19 {
		t.Fatal("source changed", n)
	}
	translated, err = source.TranslateTo(&value, target)
	paramsMust(t, err)
	defer func() { paramsMust(t, target.Del(&translated)) }()
	n, err = target.GetUIntName(&translated, &key, 0)
	paramsMust(t, err)
	if n != 19 {
		t.Fatal("retry", n)
	}
	var absent *ParamsContext
	if _, err = absent.TranslateTo(&value, target); err != ErrParamsOwner {
		t.Fatal("nil source", err)
	}
}
