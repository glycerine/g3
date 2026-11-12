package cmd_context

// Source: src/cmd_context/cmd_context.cpp complete_model and validate_model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// completeModel applies the command-context model.completion option to the
// current successful check result. ModelCore owns the added interpretations.
func (c *Context) completeModel() (resultErr error) {
	// if (!is_model_available() || model.completion != "true") return;
	available, err := c.IsModelAvailable()
	if err != nil || !available {
		return err
	}
	enabled, err := c.config.Globals.SourceGetValueName("model.completion")
	if err != nil || enabled != "true" {
		return err
	}
	md, err := c.availableCheckModel()
	if err != nil {
		return err
	}
	if md == nil {
		return nil
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	derived := md.Derived
	if derived == nil {
		return ast.ErrASTHandle
	}

	// Source m().get_some_value(s) runs with no some_value_proc installed:
	// it never consults the model universe, so an uninterpreted sort yields
	// mk_model_value(0, s) even when the universe lacks that element.
	// Both declaration loops walk their dictionaries in table order.
	for _, entry := range c.psortDictionary().entries() {
		if entry.psort == nil || !entry.psort.userDecl {
			continue
		}
		var parameters []ast.ASTHandle
		if constructor, found := c.sortConstructors[entry.name]; found {
			boolean, err := c.manager.MkBoolSort()
			if err != nil {
				return err
			}
			parameters = make([]ast.ASTHandle, constructor.arity)
			for i := range parameters {
				parameters[i] = boolean
			}
		}
		sort, err := c.InstantiateSort(entry.name, parameters)
		if err != nil {
			return err
		}
		present, err := derived.HasUninterpretedSort(sort)
		if err != nil || present {
			if err != nil {
				return err
			}
			continue
		}
		value, err := c.manager.GetSomeValue(sort)
		if err != nil {
			return err
		}
		if err = derived.RegisterUniverse(sort, []ast.ASTHandle{value}); err != nil {
			return err
		}
	}

	count := md.NumFunctionDeclarations()
	for i := uint32(0); i < count; i++ {
		declaration, err := md.Function(i)
		if err != nil {
			return err
		}
		interpretation, err := md.FunctionInterpretation(declaration)
		if err != nil || interpretation == nil {
			if err != nil {
				return err
			}
			continue
		}
		partial, err := interpretation.IsPartial()
		if err != nil || !partial {
			if err != nil {
				return err
			}
			continue
		}
		rangeSort, err := c.manager.GetRange(declaration)
		if err != nil {
			return err
		}
		value, err := c.manager.GetSomeValue(rangeSort)
		if err != nil {
			return err
		}
		if err = interpretation.SetElse(value); err != nil {
			return err
		}
	}

	for _, entry := range c.funcDictionary().entries() {
		if entry.funcs == nil {
			continue
		}
		for _, function := range entry.funcs.entries() {
			if err := c.completeFunction(md, function); err != nil {
				return err
			}
		}
	}
	return nil
}

// completeFunction is one complete_model m_func_decls entry.
func (c *Context) completeFunction(md *model.ModelCore, function ast.ASTHandle) error {
	present, err := md.HasInterpretation(function)
	if err != nil || present {
		return err
	}
	rangeSort, err := c.manager.GetRange(function)
	if err != nil {
		return err
	}
	value, err := c.manager.GetSomeValue(rangeSort)
	if err != nil {
		return err
	}
	arity, err := c.manager.GetArity(function)
	if err != nil {
		return err
	}
	if arity == 0 {
		return md.RegisterConstant(function, value)
	}
	interpretation, err := model.NewFunctionInterpretation(c.manager, arity)
	if err != nil {
		return err
	}
	if err = interpretation.SetElse(value); err != nil {
		return errors.Join(err, interpretation.Close())
	}
	if err = md.RegisterFunction(function, interpretation); err != nil {
		return errors.Join(err, interpretation.Close())
	}
	return nil
}

// validateModel checks ground assertions against a successful solver model
// when the native model_validate option is enabled. Array expressions and
// datatype accessors are exempt because their model evaluation is incomplete.
func (c *Context) validateModel() (resultErr error) {
	enabled, err := c.config.Globals.SourceGetValueName("model_validate")
	if err != nil || enabled != "true" || !c.checked || c.status != lbool.True {
		return err
	}
	if available, err := c.IsModelAvailable(); err != nil || !available {
		return err
	}
	md, err := c.availableCheckModel()
	if err != nil {
		return err
	}
	if md == nil {
		return nil
	}
	defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	evaluator, err := model.NewModelEvaluator(md, c.config.ModelEvaluator)
	if evaluator != nil {
		defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	}
	if err != nil {
		return err
	}
	if err = evaluator.SetModelCompletion(true); err != nil {
		return err
	}
	if err = evaluator.SetExpandArrayEqualities(false); err != nil {
		return err
	}
	invalid := false
	for _, assertion := range c.assertions {
		ground, err := c.manager.IsGround(assertion.expression)
		if err != nil || !ground {
			if err != nil {
				return err
			}
			continue
		}
		value, err := evaluator.EvaluateValue(assertion.expression)
		if err != nil {
			return err
		}
		h, err := c.manager.RefGet(value)
		if err == nil {
			quantified, e := c.manager.HasQuantifiers(h)
			if e != nil {
				err = e
			} else if !quantified {
				underSpecified, e := c.hasUnderSpecifiedModelOps(assertion.expression)
				if e == nil && !underSpecified {
					underSpecified, e = c.hasUnderSpecifiedModelOps(h)
				}
				if e != nil {
					err = e
				} else if !underSpecified && !c.manager.IsTrue(h) {
					invalid = true
				}
			}
		}
		err = errors.Join(err, c.manager.DelRef(&value))
		if err != nil {
			return err
		}
	}
	if invalid {
		return errors.New("an invalid model was generated")
	}
	return nil
}

// IsModelAvailable is cmd_context::is_model_available: it retrieves the
// check-sat result's model (running the solvers' model conversions) and
// drops it again. Source commands call it before their own get_model.
func (c *Context) IsModelAvailable() (bool, error) {
	md, err := c.availableCheckModel()
	if err != nil || md == nil {
		return false, err
	}
	return true, md.DecRef()
}

func (c *Context) availableCheckModel() (*model.ModelCore, error) {
	state, err := c.parameters.State()
	if err != nil {
		return nil, err
	}
	if !state.Model || !c.checked || c.status != lbool.True && c.status != lbool.Undef {
		return nil, nil
	}
	md, err := c.checkSatResult().AcquireModel()
	if err != nil || md == nil {
		return md, err
	}
	return c.fixRetrievedModel(md)
}

func (c *Context) hasUnderSpecifiedModelOps(root ast.ASTHandle) (bool, error) {
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{root}
	arrayFamily, err := c.manager.ArrayFamilyID()
	if err != nil {
		return false, err
	}
	dtFamily, err := c.manager.DatatypeFamilyID()
	if err != nil {
		return false, err
	}
	for len(stack) != 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		kind, err := c.manager.GetKind(n)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := c.manager.GetFamilyID(n)
		if err != nil {
			return false, err
		}
		declKind, err := c.manager.GetDeclKind(n)
		if err != nil {
			return false, err
		}
		if family == dtFamily && declKind == ast.ASTDatatypeAccessor || family == arrayFamily && (declKind == ast.ASTArrayOpAsArray || declKind == ast.ASTArrayOpStore || declKind == ast.ASTArrayOpMap || declKind == ast.ASTArrayOpConst) {
			return true, nil
		}
		count, err := c.manager.GetNumArgs(n)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < count; i++ {
			arg, err := c.manager.GetArg(n, i)
			if err != nil {
				return false, err
			}
			stack = append(stack, arg)
		}
	}
	return false, nil
}
