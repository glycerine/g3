package params

import (
	"strconv"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestScalarParamsFallbackAndIsolation(t *testing.T) {
	c, symbols, _ := paramsTestOwner(t)
	local, err := c.NewValue()
	paramsMust(t, err)
	defer c.Del(&local)
	global, err := c.NewValue()
	paramsMust(t, err)
	defer c.Del(&global)
	key := internSymbol(t, symbols, "shared")
	paramsMust(t, c.SetBool(&local, key, true))
	paramsMust(t, c.SetUInt(&global, key, 42))
	shadow := internSymbol(t, symbols, "shadow")
	paramsMust(t, c.SetUInt(&local, shadow, 2))
	paramsMust(t, c.SetUInt(&global, shadow, 3))
	var options ScalarParams
	if !c.ReadScalarParams(&options, &local, &global) {
		t.Fatal("expected a small snapshot")
	}
	if !options.Bool("shared", false) || options.UInt("shared\x00junk", 0) != 42 || options.UInt("shadow", 0) != 2 || !options.Bool("missing", true) {
		t.Fatal("scalar lookup lost kind, order or default")
	}
	paramsMust(t, c.SetUInt(&local, shadow, 9))
	if options.UInt("shadow", 0) != 2 {
		t.Fatal("a later write changed the snapshot")
	}
	for i := range 14 {
		paramsMust(t, c.SetBool(&global, internSymbol(t, symbols, "f"+strconv.Itoa(i)), true))
	}
	if c.ReadScalarParams(&options, &local, &global) || options.UInt("shadow", 0) != 0 {
		t.Fatal("partial snapshot of oversized fallback")
	}
	paramsMust(t, c.Reset(&global))
	paramsMust(t, c.SetUInt(&global, usymbol.Numerical(9), 8))
	if c.ReadScalarParams(&options, &local, &global) {
		t.Fatal("numerical key requires the ordinary named scan")
	}
}
