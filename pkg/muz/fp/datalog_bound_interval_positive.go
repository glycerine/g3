package fp

// Source: dl_compiler.cpp lowers a positive rule by cloning one tail or joining
// two tails, applying one compiler-produced interpreted condition, projecting
// unused columns, and renaming the remaining columns into head order. This
// typed slice covers aligned exact-pair tails, including repeated variables
// in a single positive tail.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func ensureBoundIntervalConditionVariables(manager *ast.ASTManager, condition ast.ASTHandle, variables map[uint32]uint32, product *rel.IntervalBoundProduct, sorts []ast.ASTHandle) (*rel.IntervalBoundProduct, []ast.ASTHandle, error) {
	if variables == nil {
		return nil, nil, ErrBoundIntervalDatalogFragment
	}
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return nil, nil, err
	}
	if err = used.Collect(condition); err != nil {
		return nil, nil, err
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sortHandle := used.Contains(variable)
		if sortHandle.IsNull() {
			continue
		}
		if _, found := variables[variable]; found {
			continue
		}
		column := uint32(len(sorts))
		product, err = product.AppendFullColumn(manager, sorts, sortHandle)
		if err != nil {
			return nil, nil, err
		}
		sorts = append(sorts, sortHandle)
		variables[variable] = column
	}
	return product, sorts, nil
}

func (a *boundIntervalDatalogActivation) remapBoundIntervalCondition(condition ast.ASTHandle, variables map[uint32]uint32, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	used, err := ast.NewASTUsedVars(a.manager)
	if err != nil {
		return result, err
	}
	if err = used.Collect(condition); err != nil {
		return result, err
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		if used.Contains(variable).IsNull() {
			continue
		}
		if _, found := variables[variable]; !found {
			return result, ErrBoundIntervalDatalogFragment
		}
	}
	max := uint32(0)
	for variable := range variables {
		if variable >= max {
			max = variable + 1
		}
	}
	bindings := make([]ast.ASTHandle, max)
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, a.manager.DelRef(&roots[i]))
		}
	}()
	for variable, column := range variables {
		value, cause := a.manager.MkVar(column, sorts[column])
		if cause != nil {
			return result, cause
		}
		root, cause := a.manager.NewRef(value)
		if cause != nil {
			return result, cause
		}
		roots = append(roots, root)
		bindings[variable] = value
	}
	substitution, err := astrewriter.NewVarSubstitution(a.manager, false)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	result, err = a.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = substitution.Substitute(condition, bindings, result); err != nil {
		resultErr = errors.Join(err, a.manager.DelRef(&result))
		result = ast.ASTRef{}
	}
	return result, resultErr
}

func (a *boundIntervalDatalogActivation) positiveRuleHead(rule *base.Rule) (head ast.ASTHandle, formula ast.ASTRef, resultErr error) {
	formula, err := rule.ToFormula()
	if err != nil {
		return head, formula, err
	}
	head, err = a.manager.RefGet(formula)
	if err != nil {
		return head, formula, err
	}
	if kind, cause := a.manager.GetKind(head); cause != nil {
		return head, formula, cause
	} else if kind == ast.ASTQuantifier {
		head, err = a.manager.GetQuantifierBody(head)
		if err != nil {
			return head, formula, err
		}
	}
	if op, cause := a.manager.GetDeclKind(head); cause != nil {
		return head, formula, cause
	} else if op == ast.ASTOpImplies {
		head, err = a.manager.GetArg(head, 1)
	}
	return head, formula, err
}

type boundIntervalPositiveTail struct {
	declaration ast.ASTHandle
	product     *rel.IntervalBoundProduct
	sorts       []ast.ASTHandle
	variables   []uint32
	owned       bool // independent product; another clone would lose equality classes
}

func (a *boundIntervalDatalogActivation) parsePositiveTail(rule *base.Rule, index int, products map[ast.ASTHandle]*rel.IntervalBoundProduct, allowRepeated bool) (*boundIntervalPositiveTail, error) {
	tail, negated, err := rule.Tail(index)
	if err != nil || negated {
		return nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	declaration, err := a.manager.GetDecl(tail)
	if err != nil || products[declaration] == nil || a.entries[declaration] == nil {
		return nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	sorts := append([]ast.ASTHandle(nil), a.entries[declaration].sorts...)
	arguments, err := a.manager.GetNumArgs(tail)
	if err != nil || int(arguments) != len(sorts) {
		return nil, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	type tailConstant struct {
		column uint32
		value  *big.Rat
	}
	variables := make([]uint32, 0, arguments)
	constants := make([]tailConstant, 0, arguments)
	seen := make(map[uint32]bool, arguments)
	for column := uint32(0); column < arguments; column++ {
		argument, cause := a.manager.GetArg(tail, column)
		if cause != nil {
			return nil, cause
		}
		kind, cause := a.manager.GetKind(argument)
		if cause != nil {
			return nil, cause
		}
		if kind != ast.ASTVar {
			actualSort, sortErr := a.manager.GetSort(argument)
			text, _, numeral, numeralErr := a.manager.ArithmeticNumeralText(argument)
			cause = errors.Join(sortErr, numeralErr)
			value, valid := new(big.Rat).SetString(text)
			if cause != nil || !numeral || !valid || actualSort != sorts[column] {
				return nil, errors.Join(cause, ErrBoundIntervalDatalogFragment)
			}
			constants = append(constants, tailConstant{column: column, value: value})
			continue
		}
		variable, cause := a.manager.GetVarIndex(argument)
		if cause != nil {
			return nil, cause
		}
		if seen[variable] && !allowRepeated {
			return nil, ErrBoundIntervalDatalogFragment
		}
		seen[variable] = true
		variables = append(variables, variable)
	}
	product := products[declaration]
	sort.Slice(constants, func(i, j int) bool { return constants[i].column > constants[j].column })
	for _, constant := range constants {
		product, err = product.SelectEqualAndProject(constant.column, constant.value)
		if err != nil {
			return nil, err
		}
		column := int(constant.column)
		sorts = append(sorts[:column], sorts[column+1:]...)
	}
	owned := len(constants) != 0
	if allowRepeated {
		// The source filters each occurrence class after constant columns
		// have been selected/projected. Keep every variable column until
		// interpreted filtering and head assembly: bound projection can
		// discard referenced bounds, so eagerly removing duplicates is not
		// equivalent to the compiler's final projection.
		columns := make(map[uint32][]uint32, len(variables))
		var order []uint32
		for column, variable := range variables {
			if len(columns[variable]) == 0 {
				order = append(order, variable)
			}
			columns[variable] = append(columns[variable], uint32(column))
		}
		for _, variable := range order {
			occurrences := columns[variable]
			if len(occurrences) < 2 {
				continue
			}
			if !owned {
				product, err = product.Clone()
				if err != nil {
					return nil, err
				}
				owned = true
			}
			for _, column := range occurrences[1:] {
				if err = product.FilterIdentical(occurrences[0], column); err != nil {
					return nil, err
				}
			}
		}
	}
	return &boundIntervalPositiveTail{declaration: declaration, product: product, sorts: sorts, variables: variables, owned: owned}, nil
}

func joinBoundIntervalPositiveTails(left, right *boundIntervalPositiveTail) (*rel.IntervalBoundProduct, []ast.ASTHandle, map[uint32]uint32, error) {
	if left == nil || right == nil {
		return nil, nil, nil, ErrBoundIntervalDatalogFragment
	}
	leftVariables := make(map[uint32]uint32, len(left.variables))
	variables := make(map[uint32]uint32, len(left.variables)+len(right.variables))
	for column, variable := range left.variables {
		leftVariables[variable] = uint32(column)
		variables[variable] = uint32(column)
	}
	type sharedVariable struct {
		variable uint32
		left     uint32
		right    uint32
	}
	shared := make([]sharedVariable, 0, len(right.variables))
	removed := make([]uint32, 0, len(right.variables))
	workingSorts := append([]ast.ASTHandle(nil), left.sorts...)
	for column, variable := range right.variables {
		if leftColumn, found := leftVariables[variable]; found {
			if left.sorts[leftColumn] != right.sorts[column] {
				return nil, nil, nil, ErrBoundIntervalDatalogFragment
			}
			shared = append(shared, sharedVariable{variable: variable, left: leftColumn, right: uint32(column)})
			removed = append(removed, uint32(len(left.sorts)+column))
			continue
		}
		variables[variable] = uint32(len(workingSorts))
		workingSorts = append(workingSorts, right.sorts[column])
	}
	sort.Slice(shared, func(i, j int) bool { return shared[i].variable < shared[j].variable })
	leftColumns := make([]uint32, len(shared))
	rightColumns := make([]uint32, len(shared))
	for i := range shared {
		leftColumns[i], rightColumns[i] = shared[i].left, shared[i].right
	}
	working, err := rel.JoinProjectIntervalBoundProducts(left.product, right.product, leftColumns, rightColumns, removed)
	if err != nil {
		return nil, nil, nil, err
	}
	return working, workingSorts, variables, nil
}

func (a *boundIntervalDatalogActivation) evaluatePositiveRule(rule *base.Rule, products map[ast.ASTHandle]*rel.IntervalBoundProduct) (changed bool, resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
	positive, err := rule.PositiveTailSize()
	if err != nil || positive < 1 || positive > 2 {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil || uninterpreted != positive {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	tails := make([]*boundIntervalPositiveTail, positive)
	for i := 0; i < positive; i++ {
		tails[i], err = a.parsePositiveTail(rule, i, products, positive == 1)
		if err != nil {
			return false, err
		}
	}
	var working *rel.IntervalBoundProduct
	var workingSorts []ast.ASTHandle
	var variables map[uint32]uint32
	var occurrences map[uint32][]uint32
	repeatedTail := false
	if positive == 1 {
		working = tails[0].product
		if !tails[0].owned {
			working, err = working.Clone()
			if err != nil {
				return false, err
			}
		}
		workingSorts = append([]ast.ASTHandle(nil), tails[0].sorts...)
		variables = make(map[uint32]uint32, len(tails[0].variables))
		occurrences = make(map[uint32][]uint32, len(tails[0].variables))
		for column, variable := range tails[0].variables {
			variables[variable] = uint32(column)
			repeatedTail = repeatedTail || len(occurrences[variable]) != 0
			occurrences[variable] = append(occurrences[variable], uint32(column))
		}
	} else {
		working, workingSorts, variables, err = joinBoundIntervalPositiveTails(tails[0], tails[1])
		if err != nil {
			return false, err
		}
	}
	tailSize, err := rule.TailSize()
	if err != nil || tailSize-uninterpreted > 1 {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	var condition ast.ASTRef
	if tailSize > uninterpreted {
		raw, conditionNegated, cause := rule.Tail(uninterpreted)
		if cause != nil || conditionNegated {
			return false, errors.Join(cause, ErrBoundIntervalDatalogFragment)
		}
		working, workingSorts, cause = ensureBoundIntervalConditionVariables(a.manager, raw, variables, working, workingSorts)
		if cause != nil {
			return false, cause
		}
		condition, cause = a.remapBoundIntervalCondition(raw, variables, workingSorts)
		if cause != nil {
			return false, cause
		}
		defer func() { resultErr = errors.Join(resultErr, a.manager.DelRef(&condition)) }()
	}
	head, formula, err := a.positiveRuleHead(rule)
	if formula != (ast.ASTRef{}) {
		defer func() { resultErr = errors.Join(resultErr, a.manager.DelRef(&formula)) }()
	}
	if err != nil {
		return false, err
	}
	headDeclaration, err := a.manager.GetDecl(head)
	if err != nil || products[headDeclaration] == nil {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	headCount, err := a.manager.GetNumArgs(head)
	if err != nil || int(headCount) != len(a.entries[headDeclaration].sorts) {
		return false, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	conditionApplied := false
	if repeatedTail && condition != (ast.ASTRef{}) {
		// Source filters before assembling head columns, and fuses only the
		// projection of whole variables absent from the head. Unused repeated
		// occurrences of a head variable remain through this filter: the
		// fused fallback clones and intentionally loses equality classes.
		headVariables := make(map[uint32]bool)
		for i := uint32(0); i < headCount; i++ {
			argument, cause := a.manager.GetArg(head, i)
			if cause != nil {
				return false, cause
			}
			kind, cause := a.manager.GetKind(argument)
			if cause != nil {
				return false, cause
			}
			if kind == ast.ASTVar {
				variable, cause := a.manager.GetVarIndex(argument)
				if cause != nil {
					return false, cause
				}
				headVariables[variable] = true
			}
		}
		var removed []uint32
		for variable, column := range variables {
			if len(occurrences[variable]) == 0 {
				occurrences[variable] = []uint32{column}
			}
			if !headVariables[variable] {
				removed = append(removed, occurrences[variable]...)
			}
		}
		sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })
		conditionHandle, cause := a.manager.RefGet(condition)
		if cause != nil {
			return false, cause
		}
		if len(removed) == 0 {
			err = working.FilterInterpreted(a.manager, conditionHandle)
		} else {
			working, err = working.FilterInterpretedAndProject(a.manager, conditionHandle, removed)
		}
		if err != nil {
			return false, err
		}
		if len(removed) != 0 {
			remapped := make([]uint32, len(workingSorts))
			keptSorts := make([]ast.ASTHandle, 0, len(workingSorts)-len(removed))
			for column, sortHandle := range workingSorts {
				index := sort.Search(len(removed), func(i int) bool { return removed[i] >= uint32(column) })
				if index < len(removed) && removed[index] == uint32(column) {
					continue
				}
				remapped[column] = uint32(len(keptSorts))
				keptSorts = append(keptSorts, sortHandle)
			}
			for variable, columns := range occurrences {
				if !headVariables[variable] {
					delete(variables, variable)
					delete(occurrences, variable)
					continue
				}
				for i, column := range columns {
					columns[i] = remapped[column]
				}
				variables[variable] = columns[len(columns)-1]
			}
			workingSorts = keptSorts
		}
		conditionApplied = true
	}
	desired := make([]uint32, headCount)
	usedColumns := make(map[uint32]bool, headCount)
	headSorts := a.entries[headDeclaration].sorts
	for i := uint32(0); i < headCount; i++ {
		argument, cause := a.manager.GetArg(head, i)
		if cause != nil {
			return false, cause
		}
		kind, cause := a.manager.GetKind(argument)
		if cause != nil {
			return false, cause
		}
		var column uint32
		if kind == ast.ASTVar {
			variable, variableErr := a.manager.GetVarIndex(argument)
			if variableErr != nil {
				return false, variableErr
			}
			var found bool
			column, found = variables[variable]
			if columns := occurrences[variable]; len(columns) != 0 {
				// compiler head assembly consumes available occurrences from
				// the back before synthesizing any duplicated column.
				column = columns[len(columns)-1]
				if len(columns) > 1 {
					occurrences[variable] = columns[:len(columns)-1]
				}
			}
			if !found {
				column = uint32(len(workingSorts))
				working, cause = working.AppendFullColumn(a.manager, workingSorts, headSorts[i])
				if cause != nil {
					return false, cause
				}
				workingSorts = append(workingSorts, headSorts[i])
				variables[variable] = column
			}
		} else {
			valueSort, sortErr := a.manager.GetSort(argument)
			_, _, numeral, numeralErr := a.manager.ArithmeticNumeralText(argument)
			cause = errors.Join(sortErr, numeralErr)
			if cause != nil || !numeral || valueSort != headSorts[i] {
				return false, errors.Join(cause, ErrBoundIntervalDatalogFragment)
			}
			column = uint32(len(workingSorts))
			working, cause = working.AppendConstantColumn(a.manager, workingSorts, headSorts[i], argument)
			if cause != nil {
				return false, cause
			}
			workingSorts = append(workingSorts, headSorts[i])
		}
		if usedColumns[column] {
			working, cause = working.DuplicateColumn(a.manager, workingSorts, column)
			if cause != nil {
				return false, cause
			}
			workingSorts = append(workingSorts, workingSorts[column])
			column = uint32(len(workingSorts) - 1)
		}
		usedColumns[column], desired[i] = true, column
	}
	selected := append([]uint32(nil), desired...)
	sort.Slice(selected, func(i, j int) bool { return selected[i] < selected[j] })
	removed := make([]uint32, 0, len(workingSorts)-len(selected))
	for column, at := uint32(0), 0; int(column) < len(workingSorts); column++ {
		if at < len(selected) && selected[at] == column {
			at++
		} else {
			removed = append(removed, column)
		}
	}
	if condition != (ast.ASTRef{}) && !conditionApplied {
		conditionHandle, cause := a.manager.RefGet(condition)
		if cause != nil {
			return false, cause
		}
		if len(removed) != 0 {
			working, err = working.FilterInterpretedAndProject(a.manager, conditionHandle, removed)
		} else {
			err = working.FilterInterpreted(a.manager, conditionHandle)
		}
	} else if len(removed) != 0 {
		working, err = working.Project(removed)
	}
	if err != nil {
		return false, err
	}
	current := append([]uint32(nil), selected...)
	for destination, want := range desired {
		position := destination
		for position < len(current) && current[position] != want {
			position++
		}
		if position == len(current) {
			return false, ErrBoundIntervalDatalogFragment
		}
		if position != destination {
			working, err = working.Rename([]uint32{uint32(position), uint32(destination)})
			if err != nil {
				return false, err
			}
			current[position], current[destination] = current[destination], current[position]
		}
	}
	return products[headDeclaration].Union(working, nil, false)
}
