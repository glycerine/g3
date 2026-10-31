package smtcore

// Source: src/test/simple_parser.cpp.
import (
	"bytes"
	"testing"

	parserutil "github.com/glycerine/g3/pkg/parsers/util"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSimpleParserSourceExpressionsAndCostEvaluator(t *testing.T) {
	m := astTestManager(t)
	p, err := parserutil.NewCostParser(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	p.SetWarnings(nil)
	for _, name := range []string{"x", "y"} {
		_, err = p.AddCostVariable(name)
		astTestMust(t, err)
	}
	var warnings bytes.Buffer
	eval, err := NewSMTCostEvaluator(m, diagnostic.NewWarningService(&warnings))
	astTestMust(t, err)
	for _, tc := range []struct {
		input string
		ok    bool
		value float32
	}{
		{"(+ x (* y x))", true, 9}, {"(+ x (* y x) x)", true, 9},
		{"(+ x (* y x) x", false, 0}, {"(x)", false, 0}, {"(+ x))", true, 3}, {")x)", false, 0},
		{"(+ x (* 10 y) 2)", true, 23}, {"(ite (and (> x 3) (<= y 4))  2 10)", true, 10},
		{"(ite (or (> x 3) (<= y 4))  2 10)", true, 2},
	} {
		ref, ok, err := p.ParseString(tc.input)
		if err != nil || ok != tc.ok {
			t.Fatalf("%q: ok=%v err=%v", tc.input, ok, err)
		}
		if !ok {
			continue
		}
		h, err := m.RefGet(ref)
		astTestMust(t, err)
		if tc.input == "(+ x))" {
			astTestMust(t, m.DelRef(&ref))
			continue
		}
		value, err := eval.Eval(h, []float32{2, 3})
		astTestMust(t, err)
		if value != tc.value {
			t.Errorf("%q: got %v want %v", tc.input, value, tc.value)
		}
		astTestMust(t, m.DelRef(&ref))
	}
	if warnings.Len() != 0 {
		t.Fatal(warnings.String())
	}
}
