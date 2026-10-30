package api

import "testing"

type rcfFixture struct {
	t       *testing.T
	context *SMTContext
	values  []*RCFNumber
}

func newRCFFixture(t *testing.T) *rcfFixture {
	t.Helper()
	c, err := NewSMTContext()
	if err != nil {
		t.Fatal(err)
	}
	return &rcfFixture{t: t, context: c}
}

func (f *rcfFixture) keep(value *RCFNumber, err error) *RCFNumber {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
	f.values = append(f.values, value)
	return value
}

func (f *rcfFixture) rational(text string) *RCFNumber { return f.keep(f.context.RCFRational(text)) }

func (f *rcfFixture) text(value *RCFNumber) string {
	f.t.Helper()
	text, err := f.context.RCFString(value)
	if err != nil {
		f.t.Fatal(err)
	}
	return text
}

func (f *rcfFixture) close() {
	f.t.Helper()
	for index := len(f.values) - 1; index >= 0; index-- {
		if err := f.values[index].Close(); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := f.context.Close(); err != nil {
		f.t.Fatal(err)
	}
}
