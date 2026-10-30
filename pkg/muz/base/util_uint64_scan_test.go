package base_test

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestReadUint64BoundariesAndPointerContract(t *testing.T) {
	text := "18446744073709551615 rest"
	value, next, ok := base.ReadUint64(text, 0)
	if !ok || value != math.MaxUint64 || next != 20 {
		t.Fatal("maximum", value, next, ok)
	}
	value, next, ok = base.ReadUint64("18446744073709551616", 0)
	if ok || next != 19 || value != 1844674407370955161 {
		t.Fatal("overflow pointer", value, next, ok)
	}
	if value, next, ok = base.ReadUint64("x1", 0); ok || value != 0 || next != 0 {
		t.Fatal("non-digit", value, next, ok)
	}
}
