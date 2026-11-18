package fp

// Source: src/tactic/horn_subsume_model_converter.cpp Horn-clause
// normalization and reverse model reconstruction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/cooperate"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// normalizeInlineModelRule is horn_subsume_model_converter::mk_horn(head,
// body, pred, body_res); rewrite is the converter's m_rewrite.
func (e *PropositionalEngine) normalizeInlineModelRule(rule transforms.InlineModelRule, rewrite *astrewriter.TheoryRewriter) (declaration ast.ASTHandle, result ast.ASTRef, resultErr error) {
	head, err := e.manager.RefGet(rule.Head)
	if err != nil {
		return declaration, result, err
	}
	body, err := e.manager.RefGet(rule.Body)
	if err != nil {
		return declaration, result, err
	}
	declaration, err = e.manager.GetDecl(head)
	if err != nil {
		return declaration, result, err
	}
	if family, getErr := e.manager.GetFamilyID(declaration); getErr != nil || family != ast.NullFamilyID {
		if getErr == nil {
			getErr = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, result, getErr
	}
	free, err := astrewriter.NewExprFreeVars(e.manager)
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if err = free.Collect(head); err == nil {
		err = free.Accumulate(body)
	}
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if err = free.SetDefaultSort(boolean); err != nil {
		return ast.ASTHandle{}, result, err
	}
	freeCount := free.Size()
	arity, err := e.manager.GetNumArgs(head)
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if arity == 0 && freeCount == 0 {
		result, err = e.manager.NewRef(body)
		return declaration, result, err
	}

	// Source mk_horn owns the formal variables, substitution bindings,
	// conjunction and result throughout normalization. Keep those concrete
	// scopes alive even when constructing the substitution engine fails.
	attempt := &contextFormulaAttempt{manager: e.manager}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			attempt.references = append(attempt.references, result)
			result = ast.ASTRef{}
		}
		resultErr = attempt.finish(resultErr)
		if resultErr != nil && result != (ast.ASTRef{}) {
			attempt.references = append(attempt.references, result)
			result = ast.ASTRef{}
		}
	}()
	reserve := func() (ast.ASTRef, error) {
		ref, err := e.manager.NewRef(ast.ASTHandle{})
		if ref != (ast.ASTRef{}) {
			attempt.references = append(attempt.references, ref)
		}
		return ref, err
	}
	conjuncts := []ast.ASTHandle{body}
	bindings := make([]ast.ASTHandle, freeCount)
	// subst is non-empty once a head argument is a variable.
	substituted := false
	for index := uint32(0); index < arity; index++ {
		argument, getErr := e.manager.GetArg(head, index)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		sort, getErr := e.manager.GetSort(argument)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		owned, getErr := reserve()
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		formal, getErr := e.manager.MkVar(freeCount+index, sort)
		if !formal.IsNull() {
			getErr = errors.Join(getErr, e.manager.SetRef(owned, formal))
		}
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		kind, getErr := e.manager.GetKind(argument)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		if kind == ast.ASTVar {
			variable, getErr := e.manager.GetVarIndex(argument)
			if getErr != nil {
				return ast.ASTHandle{}, result, getErr
			}
			if uint64(variable) >= uint64(len(bindings)) {
				return ast.ASTHandle{}, result, ast.ErrASTDomain
			}
			substituted = true
			if bindings[variable].IsNull() {
				bindings[variable] = formal
				continue
			}
			argument = bindings[variable]
		}
		owned, getErr = reserve()
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		equality, getErr := e.manager.MkEq(formal, argument)
		if !equality.IsNull() {
			getErr = errors.Join(getErr, e.manager.SetRef(owned, equality))
		}
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		conjuncts = append(conjuncts, equality)
	}
	owned, err := reserve()
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	formula, err := e.manager.MkAndArray(conjuncts)
	if !formula.IsNull() {
		err = errors.Join(err, e.manager.SetRef(owned, formula))
	}
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if substituted {
		attempt.substitution, err = astrewriter.NewVarSubstitution(e.manager, false)
		if err != nil {
			return ast.ASTHandle{}, result, err
		}
		rewritten, makeErr := reserve()
		if makeErr != nil {
			return ast.ASTHandle{}, result, makeErr
		}
		if makeErr = attempt.substitution.Substitute(formula, bindings, rewritten); makeErr != nil {
			return ast.ASTHandle{}, result, makeErr
		}
		formula, makeErr = e.manager.RefGet(rewritten)
		if makeErr != nil {
			return ast.ASTHandle{}, result, makeErr
		}
	}
	result, err = e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if freeCount != 0 {
		if err = free.Reverse(); err != nil {
			return ast.ASTHandle{}, result, err
		}
		sorts := free.Sorts()
		names := make([]usymbol.Symbol, freeCount)
		for i := range names {
			names[len(names)-i-1] = usymbol.Numerical(uint32(i))
		}
		formula, err = e.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: sorts, Names: names, Body: formula})
	}
	if !formula.IsNull() {
		err = errors.Join(err, e.manager.SetRef(result, formula))
	}
	if err != nil || freeCount == 0 {
		return declaration, result, err
	}
	// m_rewrite(body_res): th_rewriter::operator()(expr_ref&) assigns the
	// rewritten term back (term = result).
	if rewrite == nil {
		return ast.ASTHandle{}, result, ErrPropositionalEngine
	}
	rewritten, err := reserve()
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if err = rewrite.Rewrite(formula, rewritten); err != nil {
		return ast.ASTHandle{}, result, errors.Join(err, rewrite.Reset())
	}
	formula, err = e.manager.RefGet(rewritten)
	if err == nil {
		err = e.manager.SetRef(result, formula)
	}
	return declaration, result, err
}

func (e *PropositionalEngine) addInlineDefaultInterpretations(destination *model.ModelCore, expression ast.ASTHandle) error {
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return err
	}
	falsity, err := e.manager.MkFalse()
	if err != nil {
		return err
	}
	visited := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{expression}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if visited[current] {
			continue
		}
		visited[current] = true
		kind, getErr := e.manager.GetKind(current)
		if getErr != nil {
			return getErr
		}
		switch kind {
		case ast.ASTVar:
			continue
		case ast.ASTQuantifier:
			body, getErr := e.manager.GetQuantifierBody(current)
			if getErr != nil {
				return getErr
			}
			stack = append(stack, body)
			continue
		case ast.ASTApp:
		default:
			return ast.ErrASTDomain
		}
		declaration, getErr := e.manager.GetDecl(current)
		if getErr != nil {
			return getErr
		}
		family, getErr := e.manager.GetFamilyID(declaration)
		if getErr != nil {
			return getErr
		}
		sort, getErr := e.manager.GetSort(current)
		if getErr != nil {
			return getErr
		}
		arity, getErr := e.manager.GetArity(declaration)
		if getErr != nil {
			return getErr
		}
		if family == ast.NullFamilyID && sort == boolean {
			if arity == 0 {
				present, presentErr := destination.HasConstantInterpretation(declaration)
				if presentErr != nil {
					return presentErr
				}
				if !present {
					if presentErr = destination.RegisterConstant(declaration, falsity); presentErr != nil {
						return presentErr
					}
				}
			} else {
				interpretation, presentErr := destination.FunctionInterpretation(declaration)
				if presentErr != nil {
					return presentErr
				}
				if interpretation == nil {
					interpretation, presentErr = model.NewFunctionInterpretation(e.manager, arity)
					if presentErr == nil {
						presentErr = interpretation.SetElse(falsity)
					}
					if presentErr == nil {
						presentErr = destination.RegisterFunction(declaration, interpretation)
						interpretation = nil
					}
					if interpretation != nil {
						presentErr = errors.Join(presentErr, interpretation.Close())
					}
					if presentErr != nil {
						return presentErr
					}
				}
			}
		}
		arguments, getErr := e.manager.GetNumArgs(current)
		if getErr != nil {
			return getErr
		}
		for index := uint32(0); index < arguments; index++ {
			argument, argumentErr := e.manager.GetArg(current, index)
			if argumentErr != nil {
				return argumentErr
			}
			stack = append(stack, argument)
		}
	}
	return nil
}

// applyHornInlineModelRules is horn_subsume_model_converter::operator():
// mk_horn turns every delayed clause, in order, into m_funcs/m_bodies, the
// delayed clauses go, and then each body, last first, is evaluated into the
// model.
func (e *PropositionalEngine) applyHornInlineModelRules(destination *model.Model, converter *hornInlineConverter) (resultErr error) {
	if converter.empty() {
		return nil
	}
	if e.params == nil || e.globals == nil {
		return ErrPropositionalEngine
	}
	cooperation, err := cooperate.NewCooperation(e.manager.Budget())
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, cooperation.Close()) }()
	configuration := model.ModelEvaluatorConfig{
		Context:     e.params,
		Globals:     e.globals,
		Memory:      e.manager.NumeralService().RuntimeOwner(),
		Cooperation: cooperation,
	}
	rewrite, err := converter.rewriter(e)
	if err != nil {
		return err
	}
	for converter.normalized < len(converter.delayed) {
		declaration, normalized, err := e.normalizeInlineModelRule(converter.delayed[converter.normalized], rewrite)
		if err != nil {
			return err
		}
		// insert(pred, body_res): m_funcs and m_bodies keep both.
		predicate, err := e.manager.NewRef(declaration)
		if err != nil {
			return errors.Join(err, e.manager.DelRef(&normalized))
		}
		converter.funcs = append(converter.funcs, predicate)
		converter.bodies = append(converter.bodies, normalized)
		converter.normalized++
	}
	if len(converter.delayed) != 0 {
		// m_delay_head.reset(); m_delay_body.reset().
		for _, field := range []func(*transforms.InlineModelRule) *ast.ASTRef{
			func(r *transforms.InlineModelRule) *ast.ASTRef { return &r.Head },
			func(r *transforms.InlineModelRule) *ast.ASTRef { return &r.Body },
		} {
			for i := range converter.delayed {
				if ref := field(&converter.delayed[i]); *ref != (ast.ASTRef{}) {
					if err := e.manager.DelRef(ref); err != nil {
						return err
					}
				}
			}
		}
		converter.delayed, converter.normalized = nil, 0
	}
	for index := len(converter.funcs) - 1; index >= 0; index-- {
		if err := e.applyHornInlineBody(destination, configuration, rewrite, converter.funcs[index], converter.bodies[index]); err != nil {
			return err
		}
	}
	return nil
}

// applyHornInlineBody is one iteration of operator()'s m_funcs loop. Its
// expr_ref body is the single owner the evaluated body, the disjunction
// with the previous interpretation, and the rewritten term pass through:
// each assignment releases the term it replaces.
func (e *PropositionalEngine) applyHornInlineBody(destination *model.Model, configuration model.ModelEvaluatorConfig, rewrite *astrewriter.TheoryRewriter, funcRef, bodyRef ast.ASTRef) (resultErr error) {
	declaration, err := e.manager.RefGet(funcRef)
	if err != nil {
		return err
	}
	normalized, err := e.manager.RefGet(bodyRef)
	if err != nil {
		return err
	}
	// expr_ref body(m_bodies[i]); add_default_false_interpretation(body, mr)
	body, err := e.manager.NewRef(normalized)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&body)) }()
	if err = e.addInlineDefaultInterpretations(destination.Core(), normalized); err != nil {
		return err
	}
	// expr_ref tmp(body); mr->eval(tmp, body)
	evaluator, err := model.NewModelEvaluator(destination.Core(), configuration)
	if err == nil {
		err = evaluator.Evaluate(normalized, body)
	}
	if evaluator != nil {
		err = errors.Join(err, evaluator.Close())
	}
	if err != nil {
		return err
	}
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return err
	}
	// body = m.mk_or(e, body)
	disjoin := func(prior ast.ASTHandle) error {
		value, err := e.manager.RefGet(body)
		if err != nil {
			return err
		}
		value, err = e.manager.MkOr(prior, value)
		if err != nil {
			return err
		}
		return e.manager.SetRef(body, value)
	}
	// m_rewrite(body)
	rewriteBody := func() (ast.ASTHandle, error) {
		value, err := e.manager.RefGet(body)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		rewritten, err := e.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = rewrite.Rewrite(value, rewritten); err != nil {
			// Source's converter member keeps an interrupted traversal.
			// The Go error-return boundary permits a new model export, so
			// discard that attempt before returning to the caller. The
			// converter continues to own its rewriter if resetting is
			// refused.
			return ast.ASTHandle{}, errors.Join(err, rewrite.Reset(), e.manager.DelRef(&rewritten))
		}
		value, err = e.manager.RefGet(rewritten)
		if err == nil {
			err = e.manager.SetRef(body, value)
		}
		return value, errors.Join(err, e.manager.DelRef(&rewritten))
	}
	if arity == 0 {
		present, err := destination.Core().HasConstantInterpretation(declaration)
		if err != nil {
			return err
		}
		if present {
			prior, err := destination.Core().ConstantInterpretation(declaration)
			if err == nil {
				err = disjoin(prior)
			}
			if err != nil {
				return err
			}
		}
		value, err := rewriteBody()
		if err != nil {
			return err
		}
		return destination.Core().RegisterConstant(declaration, value)
	}
	interpretation, err := destination.Core().FunctionInterpretation(declaration)
	if err != nil {
		return err
	}
	if interpretation != nil {
		prior, err := interpretation.Else()
		if err == nil && !prior.IsNull() {
			err = disjoin(prior)
		}
		if err != nil {
			return err
		}
	}
	value, err := rewriteBody()
	if err != nil {
		return err
	}
	created := false
	if interpretation == nil {
		interpretation, err = model.NewFunctionInterpretation(e.manager, arity)
		if err != nil {
			return err
		}
		created = true
	}
	err = interpretation.SetElse(value)
	if err == nil && created {
		err = destination.Core().RegisterFunction(declaration, interpretation)
		if err == nil {
			interpretation = nil
		}
	}
	if err != nil && created && interpretation != nil {
		err = errors.Join(err, interpretation.Close())
	}
	return err
}

func (e *PropositionalEngine) hornInlineEliminated(conversions []hornModelConversion, declaration ast.ASTHandle) (bool, error) {
	for _, conversion := range conversions {
		if conversion.kind != hornConvertInline {
			continue
		}
		if conversion.inline == nil {
			continue
		}
		for _, predicate := range conversion.inline.funcs {
			headDeclaration, err := e.manager.RefGet(predicate)
			if err != nil {
				return false, err
			}
			if headDeclaration == declaration {
				return true, nil
			}
		}
		for _, rule := range conversion.inline.delayed {
			head, err := e.manager.RefGet(rule.Head)
			if err != nil {
				return false, err
			}
			headDeclaration, err := e.manager.GetDecl(head)
			if err != nil {
				return false, err
			}
			if headDeclaration == declaration {
				return true, nil
			}
		}
	}
	return false, nil
}
