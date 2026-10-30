package spacer

// Source: src/muz/spacer/spacer_context.cpp pred_transformer::init_rule
// excludes false transitions from m_rule2tag, which init_reach_facts iterates.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestSpacerInitialReachFactsExcludeFalseRules(t *testing.T) {
	for _, tc := range []struct {
		name   string
		bodies []bool
	}{
		{"all-false", []bool{false, false}},
		{"false-before-live", []bool{false, true}},
		{"false-after-live", []bool{true, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			set, err := base.NewRuleSet(f.manager)
			depthMust(t, err)
			defer func() { depthMust(t, set.Close()) }()
			head, err := f.rule.Head()
			depthMust(t, err)
			expected := 0
			for i, live := range tc.bodies {
				body, err := m.MkFalse()
				if live {
					body, err = m.MkTrue()
					expected++
				}
				depthMust(t, err)
				rule, err := f.manager.NewRule(head, []ast.ASTHandle{body}, nil, fmt.Sprintf("initial-fact-%d", i))
				depthMust(t, err)
				depthMust(t, set.Add(rule))
				depthMust(t, rule.Close())
			}
			depthMust(t, c.UpdateRules(set))
			p := c.transformers[f.query]
			facts, err := p.ReachFacts()
			depthMust(t, err)
			defer func() {
				for _, fact := range facts {
					depthMust(t, fact.Close())
				}
			}()
			if len(facts) != expected || int(p.initialReachFacts) != expected || len(p.reachCaseVariables) != expected {
				t.Fatalf("initial reach facts/cases = %d/%d/%d, want %d", len(facts), p.initialReachFacts, len(p.reachCaseVariables), expected)
			}
			has, err := p.HasReachFacts()
			depthMust(t, err)
			if has != (expected != 0) {
				t.Fatal("false transition advertised as a reach fact", has)
			}
			for _, fact := range facts {
				body, err := fact.Fact()
				depthMust(t, err)
				if m.IsFalse(body) {
					t.Fatal("false transition published as initial fact")
				}
			}
		})
	}
}
