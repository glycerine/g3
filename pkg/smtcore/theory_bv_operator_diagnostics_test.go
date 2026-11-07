package smtcore

// Source: src/smt/theory_bv.cpp:687-855,917-980,1052-1053.

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSMTBVOperatorSourceAssertionSites(t *testing.T) {
	if !diagnostic.DebugBuild {
		return
	}
	t.Run("shape", func(t *testing.T) {
		for _, test := range []struct {
			name      string
			operation int32
			count     uint32
			line      int
			condition string
		}{
			{"unary", 32, 2, 690, "n->get_num_args() == 1"},
			{"binary", 17, 1, 703, "n->get_num_args() == 2"},
			{"ac", 4, 1, 719, "n->get_num_args() >= 2"},
			{"subtract", 5, 1, 767, "n->get_num_args() == 2"},
			{"parametric", 38, 2, 807, "n->get_num_args() == 1"},
			{"extract", 40, 2, 842, "n->get_num_args() == 1"},
		} {
			t.Run(test.name, func(t *testing.T) {
				_, report := newLRASourceAssertionContext(t)
				_, _, theory := bvOccurrenceFixture(t)
				err := theory.assertTermOperatorShape(ast.ASTHandle{}, test.operation, test.count)
				bvInvariantFailure(t, err, report, test.line, test.condition)
			})
		}
	})

	t.Run("fresh", func(t *testing.T) {
		for _, test := range []struct {
			name      string
			operation int32
			count     uint32
			line      int
		}{
			{"unary", 32, 1, 689},
			{"binary", 17, 2, 702},
			{"ac", 4, 2, 718},
			{"subtract", 5, 2, 766},
			{"parametric", 38, 1, 806},
		} {
			t.Run(test.name, func(t *testing.T) {
				_, report := newLRASourceAssertionContext(t)
				_, _, theory := bvOccurrenceFixture(t)
				expression := ast.ASTHandle{}
				theory.state.context.enodes.set(expression, &SMTEnode{})
				err := theory.assertTermOperatorShape(expression, test.operation, test.count)
				bvInvariantFailure(t, err, report, test.line, "!get_context().e_internalized(n)")
			})
		}
	})

	t.Run("width-and-bounds", func(t *testing.T) {
		for _, test := range []struct {
			name      string
			line      int
			condition string
			invoke    func() error
		}{
			{"binary", 710, "arg1_bits.size() == arg2_bits.size()", func() error {
				return assertBVOperatorWidth(710, "arg1_bits.size() == arg2_bits.size()", make([]ast.ASTHandle, 1), make([]ast.ASTHandle, 2))
			}},
			{"ac", 733, "arg_bits.size() == bits.size()", func() error {
				return assertBVOperatorWidth(733, "arg_bits.size() == bits.size()", make([]ast.ASTHandle, 2), make([]ast.ASTHandle, 1))
			}},
			{"subtract", 774, "arg1_bits.size() == arg2_bits.size()", func() error {
				return assertBVOperatorWidth(774, "arg1_bits.size() == arg2_bits.size()", make([]ast.ASTHandle, 1), make([]ast.ASTHandle, 2))
			}},
			{"extract", 849, "start <= end", func() error { return assertBVExtractBounds(2, 1) }},
		} {
			t.Run(test.name, func(t *testing.T) {
				_, report := newLRASourceAssertionContext(t)
				bvInvariantFailure(t, test.invoke(), report, test.line, test.condition)
			})
		}
	})

	t.Run("atom-arity", func(t *testing.T) {
		for _, test := range []struct {
			name      string
			operation int32
			line      int
		}{
			{"overflow", 52, 919},
			{"comparison", 22, 954},
		} {
			t.Run(test.name, func(t *testing.T) {
				_, report := newLRASourceAssertionContext(t)
				err := assertBVAtomArity(test.operation, 1)
				bvInvariantFailure(t, err, report, test.line, "n->get_num_args() == 2")
			})
		}
	})
}

func TestSMTBVUnsupportedOperatorSourceUnreachableAndTrace(t *testing.T) {
	if !diagnostic.DebugBuild {
		return
	}
	t.Run("term", func(t *testing.T) {
		_, report := newLRASourceAssertionContext(t)
		m, state, theory := bvOccurrenceFixture(t)
		p := smtRelevancyBool(t, m, "operator-unreachable-p")
		word, _ := bvNativeWord(t, m, state, theory, p)
		term, err := m.MkFamilyApp(theory.FamilyID(), 3, nil, []ast.ASTHandle{word.owner})
		astTestMust(t, err)
		var output bytes.Buffer
		tags := diagnostic.NewTraceTagsWithWriter(&output)
		tags.Enable("bv_op")
		theory.trace = tags
		_, err = theory.InternalizeTerm(term)
		if err == nil {
			t.Fatal("unsupported BV term continued after source unreachable")
		}
		data, readErr := os.ReadFile(report)
		astTestMust(t, readErr)
		if !strings.Contains(string(data), "File: src/smt/theory_bv.cpp\nLine: 912\nUNREACHABLE CODE WAS REACHED.\n") {
			t.Fatal("wrong unsupported-term source site", string(data))
		}
		if tags.ShouldTrace("bv_op") {
			if !strings.Contains(output.String(), "-------- [bv_op] internalize_term src/smt/theory_bv.cpp:911 ---------\nunsupported operator: (bvneg") {
				t.Fatal("missing unsupported operator source trace", output.String())
			}
		} else if output.Len() != 0 {
			t.Fatal("compiled-out unsupported operator trace executed", output.String())
		}
	})

	t.Run("atom", func(t *testing.T) {
		_, report := newLRASourceAssertionContext(t)
		m, state, theory := bvOccurrenceFixture(t)
		p := smtRelevancyBool(t, m, "atom-unreachable-p")
		q := smtRelevancyBool(t, m, "atom-unreachable-q")
		left, _ := bvNativeWord(t, m, state, theory, p)
		right, _ := bvNativeWord(t, m, state, theory, q)
		atom, err := m.MkFamilyApp(theory.FamilyID(), 24, nil, []ast.ASTHandle{left.owner, right.owner})
		astTestMust(t, err)
		_, err = theory.InternalizeAtom(atom, true)
		if err == nil {
			t.Fatal("unsupported BV atom continued after source unreachable")
		}
		data, readErr := os.ReadFile(report)
		astTestMust(t, readErr)
		if !strings.Contains(string(data), "File: src/smt/theory_bv.cpp\nLine: 1067\nUNREACHABLE CODE WAS REACHED.\n") {
			t.Fatal("wrong unsupported-atom source site", string(data))
		}
	})
}
