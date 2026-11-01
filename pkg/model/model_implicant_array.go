package model

// Source: src/model/model_implicant.cpp:468-616,875-917.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (i *ModelImplicant) isArray(h ast.ASTHandle) (bool, error) {
	s, err := i.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	family, err := i.manager.GetFamilyID(s)
	if err != nil {
		return false, err
	}
	arrayFamily, err := i.manager.ArrayFamilyID()
	return family == arrayFamily, err
}

func (i *ModelImplicant) extractArray(h ast.ASTHandle) (frame *arrayExpansion, ok bool, err error) {
	f := &arrayExpansion{active: true}
	if err = i.traceArrayExtraction(480, "", h); err != nil {
		return f, false, err
	}
	f.elseCase, err = i.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return f, false, err
	}
	// Source model_implicant extraction normalizes as-array components in
	// both explicit store rows and function graph rows.
	for {
		stored, x := i.evaluator.cfg.arrayOp(h, ast.ASTArrayOpStore)
		if x != nil {
			return f, false, x
		}
		if !stored {
			break
		}
		args, x := i.manager.AppArgumentView(h)
		if x != nil {
			return f, false, x
		}
		tail, x := args.Range(1, args.Count()-1)
		if x != nil {
			return f, false, x
		}
		if x = appendArrayArguments(i.manager, &f.row, tail); x != nil {
			return f, false, x
		}
		if x = i.normalizeArrayRow(f.row); x != nil {
			return f, false, x
		}
		f.publishRow()
		h, x = args.At(0)
		if x != nil {
			return f, false, x
		}
	}
	constant, err := i.evaluator.cfg.arrayOp(h, ast.ASTArrayOpConst)
	if err != nil {
		return f, false, err
	}
	if constant {
		fallback, err := i.manager.GetArg(h, 0)
		if err != nil {
			return f, false, err
		}
		err = i.manager.SetRef(f.elseCase, fallback)
		return f, err == nil, err
	}
	backed, err := i.evaluator.cfg.arrayOp(h, ast.ASTArrayOpAsArray)
	if err != nil {
		return f, false, err
	}
	if !backed {
		return f, false, i.traceArrayExtraction(529, "no translation: ", h)
	}
	declaration, err := i.manager.AsArrayFunction(h)
	if err != nil {
		return f, false, err
	}
	graph, err := i.model.Core().FunctionInterpretation(declaration)
	if err != nil || graph == nil {
		return f, false, err
	}
	for _, entry := range graph.entries {
		if err = appendArrayArguments(i.manager, &f.row, ast.SliceArguments(entry.args)); err != nil {
			return f, false, err
		}
		value, err := entry.Result()
		if err != nil {
			return f, false, err
		}
		if err = appendArrayRoot(i.manager, &f.row, value); err != nil {
			return f, false, err
		}
		for _, term := range f.row {
			ground, err := i.manager.IsGround(term)
			if err != nil {
				return f, false, err
			}
			if !ground {
				return f, false, i.traceArrayExtraction(506, "could not extract array interpretation: ", h, term)
			}
		}
		if err = i.normalizeArrayRow(f.row); err != nil {
			return f, false, err
		}
		f.publishRow()
	}
	fallback, err := graph.Else()
	if err != nil {
		return f, false, err
	}
	if fallback.IsNull() {
		return f, false, i.traceArrayExtraction(515, "no else case ", h)
	}
	ground, err := i.manager.IsGround(fallback)
	if err != nil {
		return f, false, err
	}
	if !ground {
		return f, false, i.traceArrayExtraction(519, "non-ground else case ", h, fallback)
	}
	if err = i.manager.SetRef(f.elseCase, fallback); err != nil {
		return f, false, err
	}
	backed, err = i.evaluator.cfg.arrayOp(fallback, ast.ASTArrayOpAsArray)
	if err != nil {
		return f, false, err
	}
	if backed {
		normalized, err := i.evalCompletedExpression(fallback)
		if err != nil {
			return f, false, err
		}
		value, err := i.manager.RefGet(normalized)
		if err == nil {
			err = i.manager.SetRef(f.elseCase, value)
		}
		err = errors.Join(err, i.manager.DelRef(&normalized))
		if err != nil {
			return f, false, err
		}
	}
	fallback, err = i.manager.RefGet(f.elseCase)
	if err == nil {
		err = i.traceArrayExtraction(526, "else case: ", fallback)
	}
	return f, err == nil, err
}

func (i *ModelImplicant) evaluateRef(h ast.ASTHandle) (ast.ASTRef, error) {
	return i.evaluator.EvaluateValue(h)
}

func (i *ModelImplicant) valuesDiffer(left, right ast.ASTHandle) (bool, error) {
	leftValue, err := i.manager.IsValue(left)
	if err != nil {
		return false, err
	}
	rightValue, err := i.manager.IsValue(right)
	if err != nil {
		return false, err
	}
	return leftValue && rightValue && left != right, nil
}

func (i *ModelImplicant) evalArrayEq(h, left, right ast.ASTHandle) (err error) {
	leftRef, err := i.evaluateRef(left)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, i.manager.DelRef(&leftRef)) }()
	rightRef, err := i.evaluateRef(right)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, i.manager.DelRef(&rightRef)) }()
	leftValue, err := i.manager.RefGet(leftRef)
	if err != nil {
		return err
	}
	rightValue, err := i.manager.RefGet(rightRef)
	if err != nil {
		return err
	}
	if leftValue == rightValue {
		return i.setState(h, implicantTrue)
	}
	arraySort, err := i.manager.GetSort(left)
	if err != nil {
		return err
	}
	rangeSort, err := i.manager.ArrayRange(arraySort)
	if err != nil {
		return err
	}
	rangeInfinite, err := i.manager.SortIsInfinite(rangeSort)
	if err != nil {
		return err
	}
	rangeVeryBig, err := i.manager.SortIsVeryBig(rangeSort)
	if err != nil {
		return err
	}
	arrayInfinite, err := i.manager.SortIsInfinite(arraySort)
	if err != nil {
		return err
	}
	arrayVeryBig, err := i.manager.SortIsVeryBig(arraySort)
	if err != nil {
		return err
	}
	if !rangeInfinite && !rangeVeryBig && !arrayInfinite && !arrayVeryBig {
		return i.setState(h, implicantX)
	}
	first, firstOK, err := i.extractArray(leftValue)
	if first != nil {
		defer func() { err = errors.Join(err, first.close(i.manager)) }()
	}
	if err != nil {
		return err
	}
	if !firstOK {
		return i.setState(h, implicantX)
	}
	second, secondOK, err := i.extractArray(rightValue)
	if second != nil {
		defer func() { err = errors.Join(err, second.close(i.manager)) }()
	}
	if err != nil {
		return err
	}
	if !secondOK {
		return i.setState(h, implicantX)
	}
	else1, err := i.manager.RefGet(first.elseCase)
	if err != nil {
		return err
	}
	else2, err := i.manager.RefGet(second.elseCase)
	if err != nil {
		return err
	}
	if else1 != else2 {
		different, err := i.valuesDiffer(else1, else2)
		if err != nil {
			return err
		}
		if different {
			return i.setState(h, implicantFalse)
		}
		nested, err := i.isArray(else1)
		if err != nil {
			return err
		}
		if nested {
			return i.evalArrayEq(h, else1, else2)
		}
		return i.setState(h, implicantX)
	}
	rows := make([][]ast.ASTHandle, 0, len(first.stores)+len(second.stores))
	rows = append(rows, first.stores...)
	rows = append(rows, second.stores...)
	arrayFamily, err := i.manager.ArrayFamilyID()
	if err != nil {
		return err
	}
	for _, row := range rows {
		if len(row) < 2 {
			return ast.ErrASTDomain
		}
		args1 := make([]ast.ASTHandle, 1, len(row))
		args1[0] = leftValue
		args1 = append(args1, row[:len(row)-1]...)
		args2 := make([]ast.ASTHandle, 1, len(row))
		args2[0] = rightValue
		args2 = append(args2, row[:len(row)-1]...)
		select1, err := i.manager.MkArrayAppArguments(arrayFamily, ast.ASTArrayOpSelect, ast.SliceArguments(args1))
		if err != nil {
			return err
		}
		select2, err := i.manager.MkArrayAppArguments(arrayFamily, ast.ASTArrayOpSelect, ast.SliceArguments(args2))
		if err != nil {
			return err
		}
		value1Ref, err := i.evaluateRef(select1)
		if err != nil {
			return err
		}
		value2Ref, err := i.evaluateRef(select2)
		if err != nil {
			_ = i.manager.DelRef(&value1Ref)
			return err
		}
		value1, firstErr := i.manager.RefGet(value1Ref)
		value2, secondErr := i.manager.RefGet(value2Ref)
		if err = errors.Join(firstErr, secondErr); err != nil {
			return errors.Join(err, i.manager.DelRef(&value2Ref), i.manager.DelRef(&value1Ref))
		}
		cleanupValues := func() error {
			return errors.Join(i.manager.DelRef(&value2Ref), i.manager.DelRef(&value1Ref))
		}
		if value1 == value2 {
			if err = cleanupValues(); err != nil {
				return err
			}
			continue
		}
		different, err := i.valuesDiffer(value1, value2)
		if err != nil {
			return errors.Join(err, cleanupValues())
		}
		if different {
			return errors.Join(i.setState(h, implicantFalse), cleanupValues())
		}
		nested, err := i.isArray(value1)
		if err != nil {
			return errors.Join(err, cleanupValues())
		}
		if nested {
			recursive := i.evalArrayEq(h, value1, value2)
			if err = errors.Join(recursive, cleanupValues()); err != nil {
				return err
			}
			if i.isTrue(h) {
				// The source recursively reuses e and then continues. Clear its
				// provisional true mark until every selected index is checked.
				delete(i.states, h)
				continue
			}
			return nil
		}
		if err = cleanupValues(); err != nil {
			return err
		}
		return i.setState(h, implicantX)
	}
	return i.setState(h, implicantTrue)
}

// Source: model_implicant::eval(model_ref&, expr*), model_implicant.cpp:893-909.
func (i *ModelImplicant) normalizeArrayValue(expression ast.ASTHandle, result ast.ASTRef) (err error) {
	array, err := i.isArray(expression)
	if err != nil || !array {
		return err
	}
	value, err := i.manager.RefGet(result)
	if err != nil {
		return err
	}
	frame, ok, err := i.extractArray(value)
	if frame != nil {
		defer func() { err = errors.Join(err, frame.close(i.manager)) }()
	}
	if err != nil || !ok {
		return err
	}
	fallback, err := i.manager.RefGet(frame.elseCase)
	if err != nil {
		return err
	}
	// Only the innermost suffix is redundant. Default-valued stores above
	// a non-default row can override it and must keep their source order.
	for len(frame.stores) != 0 {
		last := len(frame.stores) - 1
		row := frame.stores[last]
		if row[len(row)-1] != fallback {
			break
		}
		if err = frame.removeRow(i.manager, last); err != nil {
			return err
		}
	}
	sort, err := i.manager.GetSort(expression)
	if err != nil {
		return err
	}
	base, err := i.manager.MkConstArray(sort, fallback)
	if err != nil {
		return err
	}
	if err = i.manager.SetRef(result, base); err != nil {
		return err
	}
	family, err := i.manager.ArrayFamilyID()
	if err != nil {
		return err
	}
	for n := len(frame.stores) - 1; n >= 0; n-- {
		current, err := i.manager.RefGet(result)
		if err != nil {
			return err
		}
		args := append([]ast.ASTHandle{current}, frame.stores[n]...)
		stored, err := i.manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, args)
		if err != nil {
			return err
		}
		if err = i.manager.SetRef(result, stored); err != nil {
			return err
		}
	}
	return nil
}

// evalCompletedExpression shares the active evaluator without re-entering
// setupModel/reset while recursively normalizing array-valued row components.
func (i *ModelImplicant) evalCompletedExpression(expression ast.ASTHandle) (result ast.ASTRef, err error) {
	previousCompletion := i.evaluator.cfg.completion
	defer func() {
		err = errors.Join(err, i.evaluator.SetModelCompletion(previousCompletion))
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, i.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	if err = i.evaluator.SetModelCompletion(true); err != nil {
		return result, err
	}
	result, err = i.evaluator.EvaluateValue(expression)
	if err == nil {
		err = i.normalizeArrayValue(expression, result)
	}
	return result, err
}

// Source model_implicant::eval_exprs, model_implicant.cpp:468-476.
func (i *ModelImplicant) normalizeArrayRow(row []ast.ASTHandle) error {
	for n, term := range row {
		backed, err := i.evaluator.cfg.arrayOp(term, ast.ASTArrayOpAsArray)
		if err != nil {
			return err
		}
		if !backed {
			continue
		}
		value, err := i.evalCompletedExpression(term)
		if err != nil {
			return err
		}
		normalized, err := i.manager.RefGet(value)
		if err == nil {
			err = i.manager.IncRef(normalized)
		}
		if err != nil {
			return errors.Join(err, i.manager.DelRef(&value))
		}
		consumed, err := i.manager.DecRefConsumed(term)
		if consumed {
			row[n] = normalized
		} else {
			err = errors.Join(err, i.manager.DecRef(normalized))
		}
		err = errors.Join(err, i.manager.DelRef(&value))
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	return nil
}
