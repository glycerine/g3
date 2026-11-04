package spacer

// Source: spacer_context.cpp context::mk_reach_fact.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestReachFactProjectsSelectedPredecessorAuxiliaries(t *testing.T) {
	for _, dnf := range []bool{false, true} {
		t.Run(fmt.Sprintf("dnf=%t", dnf), func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			name, err := m.Symbols.Intern("reach-projection-root")
			depthMust(t, err)
			rootDecl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			depthMust(t, err)
			depthMust(t, f.manager.RegisterPredicate(rootDecl))
			depthMust(t, f.manager.RegisterPredicate(f.other))
			rootHead, err := m.MkApp(rootDecl, nil)
			depthMust(t, err)
			firstHead, err := f.rule.Head()
			depthMust(t, err)
			secondHead, err := m.MkApp(f.other, nil)
			depthMust(t, err)
			rootRule, err := f.manager.NewRule(rootHead, []ast.ASTHandle{firstHead, secondHead}, nil, "reach-projection-root")
			depthMust(t, err)
			defer func() { depthMust(t, rootRule.Close()) }()
			secondRule, err := f.manager.NewRule(secondHead, nil, nil, "reach-projection-second")
			depthMust(t, err)
			defer func() { depthMust(t, secondRule.Close()) }()
			depthMust(t, f.rules.Add(rootRule))
			depthMust(t, f.rules.Add(secondRule))
			depthMust(t, c.UpdateRules(f.rules))
			depthMust(t, c.SetQuery(rootDecl))
			truth, err := m.MkTrue()
			depthMust(t, err)
			falsity, err := m.MkFalse()
			depthMust(t, err)
			md, err := model.NewModel(m)
			depthMust(t, err)
			defer func() { depthMust(t, md.Close()) }()
			children := []*PredicateTransformer{c.transformers[f.query], c.transformers[f.other]}
			rules := []*base.Rule{f.rule, secondRule}
			selected := make([]*reachFactState, 2)
			var originAux []ast.ASTHandle
			for i, child := range children {
				prefix, err := m.MkFreshConst("reach-projection-aux", boolean)
				depthMust(t, err)
				prefixDecl, err := m.GetDecl(prefix)
				depthMust(t, err)
				newDecl, err := child.owner.NewPredicate(prefixDecl)
				depthMust(t, err)
				a, err := m.MkApp(newDecl, nil)
				depthMust(t, err)
				fact, err := NewReachFact(m, rules[i], a, []ast.ASTHandle{a}, false)
				depthMust(t, err)
				selected[i] = fact.state
				added, err := child.AddReachFact(fact)
				depthMust(t, err)
				depthMust(t, fact.Close())
				if !added || len(child.reachCaseVariables) != 2 {
					t.Fatal("expected initial and computed facts")
				}
				for j, ref := range child.reachCaseVariables {
					h, err := m.RefGet(ref)
					depthMust(t, err)
					shifted, err := child.owner.FormulaNewToOld(h, uint32(i), false)
					depthMust(t, err)
					h, err = m.RefGet(shifted)
					depthMust(t, err)
					decl, err := m.GetDecl(h)
					depthMust(t, err)
					value := truth
					if j == 1 {
						value = falsity
					}
					depthMust(t, md.Core().RegisterConstant(decl, value))
					depthMust(t, m.DelRef(&shifted))
				}
				shifted, err := child.owner.FormulaNewToOld(a, uint32(i), false)
				depthMust(t, err)
				defer func() { depthMust(t, m.DelRef(&shifted)) }()
				h, err := m.RefGet(shifted)
				depthMust(t, err)
				originAux = append(originAux, h)
				decl, err := m.GetDecl(h)
				depthMust(t, err)
				depthMust(t, md.Core().RegisterConstant(decl, truth))
			}
			root := c.transformers[rootDecl]
			obligation, err := root.NewProofObligation(nil, 0, 1, truth, nil)
			depthMust(t, err)
			result, err := root.MakeReachFact(obligation, rootRule, md.Core(), true, dnf)
			depthMust(t, err)
			formula, err := result.Fact()
			depthMust(t, err)
			for _, a := range originAux {
				occurs, err := m.OccursExpr(a, formula)
				depthMust(t, err)
				if occurs {
					t.Fatal("predecessor auxiliary survived projection")
				}
			}
			justifications, err := result.Justifications()
			depthMust(t, err)
			if len(justifications) != 2 || justifications[0].state != selected[0] || justifications[1].state != selected[1] {
				t.Fatal("wrong selected justifications or order")
			}
			depthMust(t, result.Close())
			for _, j := range justifications {
				_, err := j.Fact()
				depthMust(t, err)
				depthMust(t, j.Close())
			}
		})
	}
}
