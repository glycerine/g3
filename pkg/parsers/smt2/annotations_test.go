package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
)

func smt2AnnotatedDefinition(t *testing.T, input string) (*ast.ASTManager, ast.ASTHandle) {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var out bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(input), &out); err != nil {
		t.Fatal(err, out.String())
	}
	m, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.DelRef(&ref); err != nil {
			t.Fatal(err)
		}
	})
	if err = context.MakeAppInto("annotated", nil, nil, ast.ASTHandle{}, ref); err != nil {
		t.Fatal(err)
	}
	value, err := m.RefGet(ref)
	if err != nil {
		t.Fatal(err)
	}
	return m, value
}

func TestSMT2QuantifierAttributesPreserveMultiPatternsAndMetadata(t *testing.T) {
	m, value := smt2AnnotatedDefinition(t, `(declare-fun p (Int) Bool)
(declare-fun q (Int) Bool)
(declare-fun r (Int Int) Bool)
(define-fun annotated () Bool
 (forall ((x Int) (y Int))
  (! (or (p x) (q y)) :pattern ((p x) (q y)) :pattern ((r x y))
     :weight 5 :qid user-quantifier :skolemid user-skolem)))`)
	weight, err := m.GetQuantifierWeight(value)
	if err != nil || weight != 5 {
		t.Fatal("weight", weight, err)
	}
	qid, skid, err := m.GetQuantifierIDs(value)
	if err != nil {
		t.Fatal(err)
	}
	qidText, err := m.Symbols.String(qid)
	if err != nil {
		t.Fatal(err)
	}
	skidText, err := m.Symbols.String(skid)
	if err != nil || qidText != "user-quantifier" || skidText != "user-skolem" {
		t.Fatal("identifiers", qidText, skidText, err)
	}
	count, err := m.GetNumPatterns(value)
	if err != nil || count != 2 {
		t.Fatal("patterns", count, err)
	}
	pattern, err := m.GetPattern(value, 0)
	if err != nil {
		t.Fatal(err)
	}
	count, err = m.GetNumArgs(pattern)
	if err != nil || count != 2 {
		t.Fatal("multi-pattern terms", count, err)
	}
	for position, expectedIndex := range []uint32{1, 0} {
		term, err := m.GetArg(pattern, uint32(position))
		if err != nil {
			t.Fatal(err)
		}
		argument, err := m.GetArg(term, 0)
		if err != nil {
			t.Fatal(err)
		}
		index, err := m.GetVarIndex(argument)
		if err != nil || index != expectedIndex {
			t.Fatal("pattern binder index", position, index, expectedIndex, err)
		}
	}
}

func TestSMT2QuantifierPatternValidationAndParserOptions(t *testing.T) {
	for _, tc := range []struct {
		name, options, attributes string
		patterns                  uint32
	}{
		{"empty-ignored", "", ":pattern ()", 0},
		{"ground-ignored", "", ":pattern ((p 0))", 0},
		{"partial-coverage-ignored", "", ":pattern ((p x))", 0},
		{"invalid-filtered-before-valid", "", ":pattern ((p x)) :pattern ((p x) (p y))", 1},
		{"legacy-unary", "", ":pattern (r x y)", 1},
		{"user-patterns-ignored", "(set-option :parser.ignore_user_patterns true)", ":pattern ((unknown x y)) :no-pattern (unknown x)", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, value := smt2AnnotatedDefinition(t, tc.options+`(declare-fun p (Int) Bool)
(declare-fun r (Int Int) Bool)
(define-fun annotated () Bool (forall ((x Int) (y Int)) (! (r x y) `+tc.attributes+`)))`)
			count, err := m.GetNumPatterns(value)
			if err != nil || count != tc.patterns {
				t.Fatal("accepted patterns", count, tc.patterns, err)
			}
		})
	}
}

func TestSMT2NestedQuantifierMetadataAndNoPatternsStayLocal(t *testing.T) {
	m, outer := smt2AnnotatedDefinition(t, `(declare-fun r (Int Int) Bool)
(define-fun annotated () Bool
 (forall ((x Int))
  (! (exists ((y Int)) (! (r x y) :no-pattern (r x y) :weight 7 :qid inner))
     :weight 3 :qid outer)))`)
	inner, err := m.GetQuantifierBody(outer)
	if err != nil {
		t.Fatal(err)
	}
	for index, quantifier := range []ast.ASTHandle{outer, inner} {
		weight, err := m.GetQuantifierWeight(quantifier)
		if err != nil || weight != []int32{3, 7}[index] {
			t.Fatal("nested weight", index, weight, err)
		}
		qid, _, err := m.GetQuantifierIDs(quantifier)
		if err != nil {
			t.Fatal(err)
		}
		name, err := m.Symbols.String(qid)
		if err != nil || name != []string{"outer", "inner"}[index] {
			t.Fatal("nested identifier", index, name, err)
		}
		count, err := m.GetNumNoPatterns(quantifier)
		if err != nil || count != uint32(index) {
			t.Fatal("no-pattern escaped its quantifier", index, count, err)
		}
	}
	term, err := m.GetNoPattern(inner, 0)
	if err != nil {
		t.Fatal(err)
	}
	for position, expected := range []uint32{1, 0} {
		argument, err := m.GetArg(term, uint32(position))
		if err != nil {
			t.Fatal(err)
		}
		index, err := m.GetVarIndex(argument)
		if err != nil || index != expected {
			t.Fatal("nested no-pattern binder", index, expected, err)
		}
	}
}

func TestSMT2QuantifierAttributesRejectWrongScopesAndBadPatterns(t *testing.T) {
	for _, expression := range []string{
		"(! true :weight 3)",
		"(forall ((x Int)) (and true (! true :qid misplaced)))",
		"(forall ((x Int)) (let ((v true)) (! v :pattern ((p x)))))",
		"(forall ((x Int)) (! true :weight 4294967296))",
		"(forall ((x Int)) (! true :pattern ()))",
		"(forall ((x Int) (y Int)) (! true :pattern ((p x))))",
		"(forall ((x Int)) (! true :pattern ((and (p x) true))))",
		"(forall ((x Int)) 3)",
	} {
		t.Run(expression, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			})
			input := "(set-option :parser.ignore_bad_patterns false)(declare-fun p (Int) Bool)(assert " + expression + ")"
			var out bytes.Buffer
			if err := ParseCommands(context, strings.NewReader(input), &out); err == nil {
				t.Fatal("invalid quantifier attribute accepted")
			}
		})
	}
}

// AUDIT.md parsers_shell_01-F4: push_quant_frame rejects an empty
// sorted-variable list, so the assertion is dropped.
func TestSMT2EmptyQuantifierBinderRejected(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var out bytes.Buffer
	err = ParseShellCommands(context, strings.NewReader("(assert (exists () false))\n(check-sat)\n"), &out)
	const message = "line 1 column 20: invalid quantifier, list of sorted variables is empty"
	if err == nil || err.Error() != message {
		t.Fatal("first parse error", err)
	}
	if want := "(error \"" + message + "\")\nsat\n"; out.String() != want {
		t.Fatal("empty quantifier binder", out.String(), want)
	}
}
