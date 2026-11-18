package bit_blaster

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// bbSmall is the small integer v as a rational without numeral storage.
func bbSmall(v int64) *unumeral.Rational {
	return &unumeral.Rational{Num: unumeral.NewInt(int32(v)), Den: unumeral.NewInt(1)}
}

// bbNumeral is Engine.Numeral as an int64.
func bbNumeral(t *testing.T, e *Engine, bits []ast.ASTHandle) (int64, bool) {
	t.Helper()
	q := e.numerals()
	r := unumeral.NewRational()
	ok, err := e.Numeral(bits, &r)
	var v int64
	if err == nil && ok {
		v, err = q.Int64(&r)
	}
	if err = errors.Join(err, q.Del(&r)); err != nil {
		t.Fatal(err)
	}
	return v, ok
}
