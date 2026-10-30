package spacer

// Source: src/muz/spacer/spacer_context.cpp pred_transformer::init_rules.

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestSpacerTaggedSystemKeepsSourceImplicationsAndTagOrder(t *testing.T) {
	for _, tc := range []struct {
		count     int
		recursive bool
	}{{1, false}, {2, false}, {1, true}, {2, true}} {
		count := tc.count
		t.Run(strconv.Itoa(count)+map[bool]string{false: "/initial", true: "/recursive"}[tc.recursive], func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			m := f.context.manager
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(f.manager.RegisterPredicate(f.other))
			pt, err := NewRulePredicateTransformer(f.context.owner, f.other, PropSolverConfig{})
			must(err)
			defer func() { must(pt.Close()) }()
			boolean, err := m.MkBoolSort()
			must(err)
			head, err := m.MkApp(f.other, nil)
			must(err)
			var rules []*base.Rule
			defer func() {
				for _, rule := range rules {
					must(rule.Close())
				}
			}()
			var conditions []ast.ASTHandle
			for i := 0; i < count; i++ {
				condition, err := m.MkFreshConst("tagged-system-condition", boolean)
				must(err)
				tail := []ast.ASTHandle{condition}
				if tc.recursive && i == count-1 {
					tail = []ast.ASTHandle{head, condition}
				}
				rule, err := f.manager.NewRule(head, tail, nil, "tagged-system-"+strconv.Itoa(i))
				must(err)
				rules = append(rules, rule)
				conditions = append(conditions, condition)
			}
			must(pt.InitializeRules(map[ast.ASTHandle]*PredicateTransformer{f.other: pt}, rules))
			negativeExtension, err := m.RefGet(pt.extendLiteral)
			must(err)
			_, extension, err := m.IsNot(negativeExtension)
			must(err)
			var tags, guarded []ast.ASTHandle
			for i, rule := range rules {
				tag, err := pt.RuleTag(rule)
				must(err)
				tags = append(tags, tag)
				implication, err := m.MkImplies(tag, conditions[i])
				must(err)
				guarded = append(guarded, implication)
			}
			var selection []ast.ASTHandle
			if count == 1 {
				selection = []ast.ASTHandle{tags[0], extension}
			} else {
				selection = append([]ast.ASTHandle{extension}, tags...)
			}
			choice, err := m.MkOrArray(selection)
			must(err)
			guarded = append(guarded, choice)
			expected, err := m.MkAndArray(guarded)
			must(err)
			actual, err := pt.Transition()
			must(err)
			if actual != expected {
				t.Fatal("tagged transition changed source implication shape or tag/extension order", actual, expected)
			}
			initial, err := pt.InitialState()
			must(err)
			expectedInitial, err := m.MkTrue()
			must(err)
			if tc.recursive {
				expectedInitial, err = m.MkNot(tags[count-1])
				must(err)
			}
			if initial != expectedInitial {
				t.Fatal("initial condition must disable only the recursive rule tag", initial, expectedInitial)
			}
			allInitial, err := pt.AllInitial()
			must(err)
			if allInitial == tc.recursive {
				t.Fatal("all-initial classification", allInitial, tc.recursive)
			}
		})
	}
}
