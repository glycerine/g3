package spacer

// Source: spacer_context.cpp context::mk_reach_fact applies spacer_elim_aux
// to rule-local variables, independently of implicant extraction.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestReachFactRuleAuxiliaryProjectionPolicy(t *testing.T) {
	for _, eliminate := range []bool{false, true} {
		for _, dnf := range []bool{false, true} {
			t.Run(fmt.Sprintf("eliminate=%t/dnf=%t", eliminate, dnf), func(t *testing.T) {
				f := newContextAnswerFixture(t, false)
				c, m := f.context, f.context.manager
				boolean, err := m.MkBoolSort()
				depthMust(t, err)
				variable, err := m.MkVar(0, boolean)
				depthMust(t, err)
				constraint, err := m.MkNot(variable)
				depthMust(t, err)
				head, err := f.rule.Head()
				depthMust(t, err)
				rule, err := f.manager.NewRule(head, []ast.ASTHandle{constraint}, nil, "rule-aux-projection")
				depthMust(t, err)
				defer func() { depthMust(t, rule.Close()) }()
				set, err := base.NewRuleSet(f.manager)
				depthMust(t, err)
				defer func() { depthMust(t, set.Close()) }()
				depthMust(t, set.Add(rule))
				depthMust(t, c.UpdateRules(set))
				p := c.transformers[f.query]
				aux, err := p.RuleAuxiliaries(rule)
				depthMust(t, err)
				if len(aux) != 1 {
					t.Fatalf("rule auxiliary count = %d, want 1", len(aux))
				}
				truth, err := m.MkTrue()
				depthMust(t, err)
				md, err := model.NewModel(m)
				depthMust(t, err)
				defer func() { depthMust(t, md.Close()) }()
				decl, err := m.GetDecl(aux[0])
				depthMust(t, err)
				falsity, err := m.MkFalse()
				depthMust(t, err)
				depthMust(t, md.Core().RegisterConstant(decl, falsity))
				obligation, err := p.NewProofObligation(nil, 0, 1, truth, nil)
				depthMust(t, err)
				result, err := p.MakeReachFact(obligation, rule, md.Core(), eliminate, dnf)
				depthMust(t, err)
				defer func() { depthMust(t, result.Close()) }()
				formula, err := result.Fact()
				depthMust(t, err)
				retained, err := result.Auxiliaries()
				depthMust(t, err)
				occurs, err := m.OccursExpr(aux[0], formula)
				depthMust(t, err)
				if eliminate {
					if len(retained) != 0 || occurs || !m.IsTrue(formula) {
						t.Fatalf("eliminated fact %v; auxiliaries %v, occurs %t", formula, retained, occurs)
					}
				} else if len(retained) != 1 || retained[0] != aux[0] || !occurs {
					t.Fatalf("retained fact %v; auxiliaries %v, occurs %t", formula, retained, occurs)
				}
			})
		}
	}
}
