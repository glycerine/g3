package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (e *IntervalDatalogEvaluator) relationFormula(relation *rel.IntervalRelation, sorts []ast.ASTHandle) (ast.ASTRef, ast.ASTHandle, error) {
	formula, err := relation.ToFormula(e.manager, sorts)
	if err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, err
	}
	handle, err := e.manager.RefGet(formula)
	if err != nil {
		return formula, ast.ASTHandle{}, errors.Join(err, e.manager.DelRef(&formula))
	}
	return formula, handle, nil
}

func (e *IntervalDatalogEvaluator) filterIdentical(relation *rel.IntervalRelation, sorts []ast.ASTHandle, columns []uint32) (resultErr error) {
	if e.formulaCheck == nil {
		return relation.FilterIdenticalColumns(columns)
	}
	beforeRoot, before, err := e.relationFormula(relation, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&beforeRoot)) }()
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&roots[i]))
		}
	}()
	var equalities []ast.ASTHandle
	for _, column := range columns[1:] {
		left, err := e.manager.MkVar(columns[0], sorts[columns[0]])
		if err != nil {
			return err
		}
		leftRoot, err := e.manager.NewRef(left)
		if err != nil {
			return err
		}
		roots = append(roots, leftRoot)
		right, err := e.manager.MkVar(column, sorts[column])
		if err != nil {
			return err
		}
		rightRoot, err := e.manager.NewRef(right)
		if err != nil {
			return err
		}
		roots = append(roots, rightRoot)
		equality, err := e.manager.MkEq(left, right)
		if err != nil {
			return err
		}
		equalityRoot, err := e.manager.NewRef(equality)
		if err != nil {
			return err
		}
		roots = append(roots, equalityRoot)
		equalities = append(equalities, equality)
	}
	condition, err := e.manager.MkAndArgs(equalities)
	if err != nil {
		return err
	}
	conditionRoot, err := e.manager.NewRef(condition)
	if err != nil {
		return err
	}
	roots = append(roots, conditionRoot)
	if err = relation.FilterIdenticalColumns(columns); err != nil {
		return err
	}
	return rel.VerifyIntervalRelationFilter(e.manager, sorts, *e.formulaCheck, before, condition, relation, true)
}

func (e *IntervalDatalogEvaluator) filterEqual(relation *rel.IntervalRelation, sorts []ast.ASTHandle, column uint32, value ast.ASTHandle) (resultErr error) {
	if e.formulaCheck == nil {
		return relation.FilterEqualExpression(e.manager, column, value)
	}
	beforeRoot, before, err := e.relationFormula(relation, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&beforeRoot)) }()
	variable, err := e.manager.MkVar(column, sorts[column])
	if err != nil {
		return err
	}
	variableRoot, err := e.manager.NewRef(variable)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&variableRoot)) }()
	condition, err := e.manager.MkEq(variable, value)
	if err != nil {
		return err
	}
	conditionRoot, err := e.manager.NewRef(condition)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&conditionRoot)) }()
	if err = relation.FilterEqualExpression(e.manager, column, value); err != nil {
		return err
	}
	return rel.VerifyIntervalRelationFilter(e.manager, sorts, *e.formulaCheck, before, condition, relation, false)
}

func (e *IntervalDatalogEvaluator) filterInterpreted(relation *rel.IntervalRelation, sorts []ast.ASTHandle, condition ast.ASTHandle) (resultErr error) {
	if e.formulaCheck == nil {
		return relation.FilterInterpreted(e.manager, condition)
	}
	beforeRoot, before, err := e.relationFormula(relation, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&beforeRoot)) }()
	if err = relation.FilterInterpreted(e.manager, condition); err != nil {
		return err
	}
	return rel.VerifyIntervalRelationFilter(e.manager, sorts, *e.formulaCheck, before, condition, relation, true)
}

func (e *IntervalDatalogEvaluator) join(left *rel.IntervalRelation, leftSorts []ast.ASTHandle, right *rel.IntervalRelation, rightSorts []ast.ASTHandle, leftColumns, rightColumns []uint32) (*rel.IntervalRelation, []ast.ASTHandle, error) {
	joined, err := rel.JoinIntervalRelations(left, right, leftColumns, rightColumns)
	if err != nil {
		return nil, nil, err
	}
	sorts := append(append([]ast.ASTHandle(nil), leftSorts...), rightSorts...)
	if e.formulaCheck != nil {
		if err = rel.VerifyIntervalRelationJoin(e.manager, leftSorts, rightSorts, *e.formulaCheck, left, right, leftColumns, rightColumns, joined); err != nil {
			return nil, nil, err
		}
	}
	return joined, sorts, nil
}

func (e *IntervalDatalogEvaluator) project(relation *rel.IntervalRelation, sorts []ast.ASTHandle, removed []uint32) (*rel.IntervalRelation, []ast.ASTHandle, error) {
	projected, err := relation.Project(removed)
	if err != nil {
		return nil, nil, err
	}
	resultSorts := make([]ast.ASTHandle, 0, len(sorts)-len(removed))
	removedIndex := 0
	for i, sort := range sorts {
		if removedIndex < len(removed) && removed[removedIndex] == uint32(i) {
			removedIndex++
		} else {
			resultSorts = append(resultSorts, sort)
		}
	}
	if e.formulaCheck == nil {
		return projected, resultSorts, nil
	}
	sourceRoot, source, err := e.relationFormula(relation, sorts)
	if err != nil {
		return nil, nil, err
	}
	defer e.manager.DelRef(&sourceRoot)
	removedColumns := make([]int, len(removed))
	for i, column := range removed {
		removedColumns[i] = int(column)
	}
	if err = rel.VerifyIntervalRelationProject(e.manager, sorts, resultSorts, *e.formulaCheck, source, removedColumns, projected); err != nil {
		return nil, nil, err
	}
	return projected, resultSorts, nil
}

func (e *IntervalDatalogEvaluator) rename(relation *rel.IntervalRelation, sorts []ast.ASTHandle, cycle []uint32) (*rel.IntervalRelation, []ast.ASTHandle, error) {
	renamed, err := relation.Rename(cycle)
	if err != nil {
		return nil, nil, err
	}
	resultSorts := append([]ast.ASTHandle(nil), sorts...)
	for i, column := range cycle {
		resultSorts[cycle[(i+1)%len(cycle)]] = sorts[column]
	}
	if e.formulaCheck == nil {
		return renamed, resultSorts, nil
	}
	sourceRoot, source, err := e.relationFormula(relation, sorts)
	if err != nil {
		return nil, nil, err
	}
	defer e.manager.DelRef(&sourceRoot)
	if err = rel.VerifyIntervalRelationRename(e.manager, sorts, resultSorts, *e.formulaCheck, source, cycle, renamed); err != nil {
		return nil, nil, err
	}
	return renamed, resultSorts, nil
}

func (e *IntervalDatalogEvaluator) union(target *rel.IntervalRelation, sorts []ast.ASTHandle, source *rel.IntervalRelation, widen bool) (bool, error) {
	if e.formulaCheck == nil {
		return target.Union(source, widen)
	}
	beforeRoot, before, err := e.relationFormula(target, sorts)
	if err != nil {
		return false, err
	}
	defer e.manager.DelRef(&beforeRoot)
	changed, err := target.Union(source, widen)
	if err != nil {
		return changed, err
	}
	return changed, rel.VerifyIntervalRelationUnion(e.manager, sorts, *e.formulaCheck, before, source, target)
}
