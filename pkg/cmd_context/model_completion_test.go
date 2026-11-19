package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
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

// cmd_context::check_sat calls display_sat_result before complete_model
// and validate_model, so "sat" is printed before a long validation and
// before an "invalid model" error. The display callback here asserts a
// ground formula the model falsifies: validation fails only if it runs
// after the display.
func TestCheckSatDisplaysResultBeforeModelValidation(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_UF"))
	contextMust(t, c.SetBoolOption("produce-models", true))
	contextMust(t, c.SetBoolOption("model_validate", true))
	m, err := c.Manager()
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	_, err = c.DeclareFunction("display-order-constant", nil, boolean)
	contextMust(t, err)
	term, err := c.MakeApp("display-order-constant", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.Assert(term, ""))
	var shown []lbool.LBool
	status, err := c.CheckSatDisplay(nil, func(status lbool.LBool) error {
		shown = append(shown, status)
		negated, err := m.MkNot(term)
		if err != nil {
			return err
		}
		// Add the row directly: Assert would reset the check-sat result
		// (assert_expr does too), and validate_model needs its model.
		ref, err := m.NewRef(negated)
		if err != nil {
			return err
		}
		c.assertions = append(c.assertions, assertion{expression: negated, ref: ref})
		return nil
	})
	if len(shown) != 1 || shown[0] != lbool.True || status != lbool.True {
		t.Fatal("display before validation", shown, status)
	}
	if err == nil || err.Error() != "an invalid model was generated" {
		t.Fatal("validation after display", err)
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

// AUDIT.md cmd_context_01-F15: contains_underspecified_op_proc runs over
// for_each_expr, which visits a quantifier's body (and patterns), so a
// store inside a ground quantified assertion exempts it from validation.
func TestValidateModelScansQuantifierBodies(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	boolean, err := m.MkBoolSort()
	contextMust(t, err)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	contextMust(t, err)
	_, err = c.DeclareFunction("scan-array", nil, arraySort)
	contextMust(t, err)
	a, err := c.MakeApp("scan-array", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	v, err := m.MkVar(0, boolean)
	contextMust(t, err)
	store, err := c.MakeApp("store", []ast.ASTHandle{a, v, v}, nil, ast.ASTHandle{})
	contextMust(t, err)
	read, err := c.MakeApp("select", []ast.ASTHandle{store, v}, nil, ast.ASTHandle{})
	contextMust(t, err)
	name, err := m.Symbols.Intern("scan-x")
	contextMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{name}, Body: read, Weight: 1})
	contextMust(t, err)
	found, err := c.hasUnderSpecifiedModelOps(q)
	contextMust(t, err)
	if !found {
		t.Fatal("the store in the quantifier body was not found")
	}
}
