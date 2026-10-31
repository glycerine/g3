package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestDegreeShiftModelReconstruction(t *testing.T) {
	base, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, false, "model-x")
	root := fixDLApp(t, m, 18, x, fixDLNumber(t, m, false, "2"))
	plan, err := degreeShiftPrepare(m, []ast.ASTHandle{root}, false)
	equationMust(t, err)
	defer func() { equationMust(t, plan.Close()) }()
	fresh := mustFreshFor(t, m, plan, x)
	freshDecl, err := m.GetDecl(fresh)
	equationMust(t, err)
	xDecl, err := m.GetDecl(x)
	equationMust(t, err)
	var out tactic.TacticResult
	equationMust(t, out.Prepare(m))
	defer func() { equationMust(t, out.Close()) }()
	equationMust(t, degreeShiftPublishModel(plan, base.services, &out))
	if out.ModelConverter == nil {
		t.Fatal("missing model converter")
	}
	md, err := model.NewModel(m)
	equationMust(t, err)
	equationMust(t, md.CoreField.RegisterConstant(freshDecl, fixDLNumber(t, m, false, "4")))
	var ref tactic.ModelRef
	equationMust(t, ref.Set(&md.CoreField))
	defer func() { equationMust(t, ref.Close()) }()
	equationMust(t, out.ModelConverter.ApplyModel(&ref, 0))
	converted, err := ref.Get()
	equationMust(t, err)
	value, err := converted.ConstantInterpretation(xDecl)
	equationMust(t, err)
	if value == (ast.ASTHandle{}) {
		t.Fatal("original variable was not reconstructed")
	}
	text, _, numeral, err := m.ArithmeticNumeralText(value)
	equationMust(t, err)
	if !numeral || text != "2" {
		t.Fatalf("square root of 4 reconstructed as %q (numeral %t)", text, numeral)
	}
	present, err := converted.HasInterpretation(freshDecl)
	equationMust(t, err)
	if present {
		t.Fatal("fresh variable was not filtered")
	}
}
