package opt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestWMaxAssignmentUsesIncompleteModel(t *testing.T) {
	m, fixture := pbSLSFixture(t)
	p := maxResTestBool(t, m, "wmax-model-p")
	q := maxResTestBool(t, m, "wmax-model-q")
	notQ, err := m.MkNot(q)
	if err != nil {
		t.Fatal(err)
	}
	root, err := m.NewRef(notQ)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := m.DelRef(&root); e != nil {
			t.Error(e)
		}
	}()
	md, err := model.NewModel(m)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := md.Close(); e != nil {
			t.Error(e)
		}
	}()
	decl, err := m.GetDecl(p)
	if err != nil {
		t.Fatal(err)
	}
	truth, err := m.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	if err = md.Core().RegisterConstant(decl, truth); err != nil {
		t.Fatal(err)
	}
	solver := &ArithmeticSolver{manager: m, config: smtcore.SMTSolverConfig{Params: fixture.Context, Globals: fixture.Globals, ModelEvaluator: fixture.Model}}
	result, err := wmaxModelAssignment(solver, md.Core(), []ast.ASTHandle{p, q, notQ})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 || !result[0] || result[1] || result[2] {
		t.Fatal("source model assignment", result)
	}
}
