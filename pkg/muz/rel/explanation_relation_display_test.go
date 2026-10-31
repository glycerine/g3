package rel_test

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestExplanationRelationFormulaDisplayAndForeignUnion(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	sort, err := m.MkDatalogRuleSort()
	must(err)
	name, err := m.Symbols.Intern("explanation-a")
	must(err)
	a, err := m.MkConst(name, sort)
	must(err)
	name, err = m.Symbols.Intern("explanation-b")
	must(err)
	b, err := m.MkConst(name, sort)
	must(err)
	r, err := rel.NewExplanationRelation(m, []ast.ASTHandle{sort, sort})
	must(err)
	defer func() { must(r.Close()) }()
	var output bytes.Buffer
	must(r.Display(&output))
	if output.String() != "<empty explanation relation>\n" {
		t.Fatal("empty display", output.String())
	}
	must(r.AddFact([]ast.ASTHandle{a, b}))
	output.Reset()
	must(r.Display(&output))
	if output.String() != "explanation-a, explanation-a\n" {
		t.Fatal("source repeated-first display", output.String())
	}
	formulaRef, err := r.ToFormula()
	must(err)
	formula, err := m.RefGet(formulaRef)
	must(err)
	variable, err := m.MkVar(0, sort)
	must(err)
	want, err := m.MkEq(variable, a)
	must(err)
	if formula != want {
		t.Fatal("explanation formula", formula, want)
	}
	must(m.DelRef(&formulaRef))
	must(r.Reset())
	formulaRef, err = r.ToFormula()
	must(err)
	must(m.DelRef(&formulaRef))
	delta, err := rel.NewExplanationRelation(m, []ast.ASTHandle{sort})
	must(err)
	defer func() { must(delta.Close()) }()
	must(r.UnionForeign(true, delta))
	empty, err := r.Empty()
	must(err)
	if !empty {
		t.Fatal("empty foreign source changed relation")
	}
	must(r.UnionForeign(false, delta))
	for _, relation := range []*rel.ExplanationRelation{r, delta} {
		empty, err = relation.Empty()
		must(err)
		defined, err := relation.NoUndefined()
		must(err)
		if empty || defined {
			t.Fatal("foreign union must create undefined row", empty, defined)
		}
	}
	output.Reset()
	must(r.Display(&output))
	if output.String() != "<undefined>, <undefined>\n" {
		t.Fatal("undefined display", output.String())
	}
}
