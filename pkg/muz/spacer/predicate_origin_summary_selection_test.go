package spacer

// Source: spacer_context.cpp pred_transformer::get_origin_summary.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestOriginSummaryUsesSelectedFactAndAuxiliaries(t *testing.T) {
	for _, chosen := range []int{1, 2} {
		t.Run(fmt.Sprintf("fact-%d", chosen), func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			p, m := f.context.transformers[f.query], f.context.manager
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			formulas := make([]ast.ASTHandle, 3)
			aux := make([]ast.ASTHandle, 3)
			for i := 1; i <= 2; i++ {
				formulas[i], err = m.MkFreshConst("summary-fact", boolean)
				depthMust(t, err)
				aux[i], err = m.MkFreshConst("summary-aux", boolean)
				depthMust(t, err)
				fact, err := NewReachFact(m, f.rule, formulas[i], []ast.ASTHandle{aux[i]}, false)
				depthMust(t, err)
				added, err := p.AddReachFact(fact)
				depthMust(t, err)
				depthMust(t, fact.Close())
				if !added {
					t.Fatal("fact not added")
				}
			}
			md, err := model.NewModel(m)
			depthMust(t, err)
			defer func() { depthMust(t, md.Close()) }()
			yes, err := m.MkTrue()
			depthMust(t, err)
			no, err := m.MkFalse()
			depthMust(t, err)
			for i, ref := range p.reachCaseVariables {
				h, err := m.RefGet(ref)
				depthMust(t, err)
				shifted, err := p.owner.FormulaNewToOld(h, 2, false)
				depthMust(t, err)
				h, err = m.RefGet(shifted)
				depthMust(t, err)
				d, err := m.GetDecl(h)
				depthMust(t, err)
				value := yes
				if i == chosen {
					value = no
				}
				depthMust(t, md.Core().RegisterConstant(d, value))
				depthMust(t, m.DelRef(&shifted))
			}
			shifted, err := p.owner.FormulaNewToOld(formulas[chosen], 2, false)
			depthMust(t, err)
			defer func() { depthMust(t, m.DelRef(&shifted)) }()
			expected, err := m.RefGet(shifted)
			depthMust(t, err)
			d, err := m.GetDecl(expected)
			depthMust(t, err)
			depthMust(t, md.Core().RegisterConstant(d, yes))
			before := m.Stats().LiveReferences
			summary, auxiliaries, err := p.OriginSummary(md.Core(), 0, 2, true)
			depthMust(t, err)
			actual, err := m.RefGet(summary)
			depthMust(t, err)
			if actual != expected || len(auxiliaries) != 1 || auxiliaries[0] != aux[chosen] {
				t.Fatalf("summary %v, auxiliaries %v; want %v, [%v]", actual, auxiliaries, expected, aux[chosen])
			}
			depthMust(t, m.DelRef(&summary))
			if got := m.Stats().LiveReferences; got != before {
				t.Fatalf("summary leaked refs: %d vs %d", got, before)
			}
		})
	}
}
