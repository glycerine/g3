package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCommandContextModelCompletionOptionCompletesUserDeclarations(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_UF"))
	contextMust(t, c.SetBoolOption("produce-models", true))
	m, err := c.Manager()
	contextMust(t, err)
	u, err := c.DeclareSort("CompletionU")
	contextMust(t, err)
	constant, err := c.DeclareFunction("completion-constant", nil, u)
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	function, err := c.DeclareFunction("completion-function", []ast.ASTHandle{u}, boolean)
	contextMust(t, err)
	contextMust(t, c.config.Globals.SourceSetName("model.completion", "true"))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("empty QF_UF check", status)
	}
	md, err := c.Model()
	contextMust(t, err)
	defer func() { contextMust(t, md.DecRef()) }()
	hasSort, err := md.Derived.HasUninterpretedSort(u)
	contextMust(t, err)
	hasConstant, err := md.HasConstantInterpretation(constant)
	contextMust(t, err)
	hasFunction, err := md.HasInterpretation(function)
	contextMust(t, err)
	if !hasSort || !hasConstant || !hasFunction {
		t.Fatal("completion omitted declared model values", hasSort, hasConstant, hasFunction)
	}
	interpretation, err := md.FunctionInterpretation(function)
	contextMust(t, err)
	partial, err := interpretation.IsPartial()
	contextMust(t, err)
	if partial {
		t.Fatal("completion left declared function partial")
	}
}

func TestCommandContextModelCompletionSkipsUnavailableModel(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_UF"))
	contextMust(t, c.config.Globals.SourceSetName("model.completion", "true"))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("empty QF_UF check", status)
	}
}

func TestCommandContextModelValidationEnabledForSatisfyingGroundAssertions(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_UF"))
	contextMust(t, c.SetBoolOption("produce-models", true))
	contextMust(t, c.SetBoolOption("model_validate", true))
	m, err := c.Manager()
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	constant, err := c.DeclareFunction("validated-constant", nil, boolean)
	contextMust(t, err)
	term, err := c.MakeApp("validated-constant", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.Assert(term, ""))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("satisfying ground assertion", status)
	}
	if _, err := m.GetKind(constant); err != nil {
		t.Fatal("validated declaration expired", err)
	}
}

func TestCommandContextModelValidationRejectsFalseGroundAssertion(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_UF"))
	contextMust(t, c.SetBoolOption("produce-models", true))
	contextMust(t, c.SetBoolOption("model_validate", true))
	m, err := c.Manager()
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	declaration, err := c.DeclareFunction("invalid-model-constant", nil, boolean)
	contextMust(t, err)
	assertion, err := c.MakeApp("invalid-model-constant", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.Assert(assertion, ""))
	md, err := model.NewModelCore(m)
	contextMust(t, err)
	falsity, err := m.MkFalse()
	contextMust(t, err)
	contextMust(t, md.RegisterConstant(declaration, falsity))
	result, err := solver.NewSimpleCheckSatResult(m)
	contextMust(t, err)
	contextMust(t, result.SetStatus(lbool.True))
	contextMust(t, result.SetModel(md))
	c.tacticResult = result
	c.status, c.checked = lbool.True, true
	if err = c.validateModel(); err == nil || err.Error() != "an invalid model was generated" {
		t.Fatal("invalid ground assertion validation", err)
	}
}
