package realclosure

import (
	"bytes"
	"testing"
)

func TestEvaluateRationalPolynomialIntervalHorner(t *testing.T) {
	f := newBoundedSignFixture(t, 1, 2)
	one := f.rational(1)
	result, err := NewBinaryInterval(f.bounds)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.evaluateRationalPolynomialInterval([]*realValue{one, one}, f.interval, result); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := result.Display(&out); err != nil || out.String() != "[2, 3]" {
		t.Fatal(out.String(), err)
	}
	if err := result.Close(); err != nil {
		t.Fatal(err)
	}
}
