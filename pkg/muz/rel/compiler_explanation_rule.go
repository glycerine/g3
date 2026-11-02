package rel

// Source: dl_compiler.cpp compile_rule's total-column, interpreted-filter,
// head projection/renaming and union/store lowering, specialized to the
// explanation rules produced by dl_mk_explanations.cpp get_e_rule.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

type explanationTailConstant struct {
	column int
	value  Element
}

type explanationFiniteColumn struct {
	sort      ast.ASTHandle
	value     Element
	singleton bool
}

type explanationRuleLayout struct {
	head, tail, tail2 ast.ASTHandle
	joinLeft          []int
	joinRight         []int
	columns           map[uint32]int
	sorts             map[uint32]ast.ASTHandle
	conditions        []ast.ASTHandle
	removed           []int
	permutation       []int
	appendHead        bool
	tailConstants     []explanationTailConstant
	appendFinite      []explanationFiniteColumn
	identical         [][]int
	duplicates        []int
	finiteWidth       int
	explanationWidth  int
}

func planExplanationRule(relations *RelationManager, rule *base.Rule) (plan explanationRuleLayout, resultErr error) {
	if err := relations.open(); err != nil {
		return plan, err
	}
	owner, err := rule.ASTManager()
	if err != nil {
		return plan, err
	}
	if owner != relations.manager {
		return plan, ErrCompilerUnsupported
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return plan, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return plan, err
	}
	if positive > 2 || positive != uninterpreted {
		return plan, ErrCompilerUnsupported
	}
	m := relations.manager
	var tails []ast.ASTHandle
	var tailSignatures [][]ast.ASTHandle
	var tailSorts []ast.ASTHandle
	for index := 0; index < positive; index++ {
		tail, negative, err := rule.Tail(index)
		if err != nil {
			return plan, err
		}
		if negative {
			return plan, ErrCompilerUnsupported
		}
		predicate, err := m.GetDecl(tail)
		if err != nil {
			return plan, err
		}
		sorts, explanations, err := relations.explanationPredicateSignature(predicate)
		if err != nil {
			return plan, errors.Join(ErrCompilerUnsupported, err)
		}
		if explanations != 1 {
			return plan, ErrCompilerUnsupported
		}
		if index == 0 {
			plan.tail = predicate
		} else {
			plan.tail2 = predicate
		}
		tails = append(tails, tail)
		tailSignatures = append(tailSignatures, sorts)
		tailSorts = append(tailSorts, sorts...)
	}
	head, err := rule.Head()
	if err != nil {
		return plan, err
	}
	plan.head, err = m.GetDecl(head)
	if err != nil {
		return plan, err
	}
	explained, err := relations.predicateUsesExplanationColumns(plan.head)
	if err != nil {
		return plan, err
	}
	var headSorts []ast.ASTHandle
	headExplanations := 0
	if explained {
		headSorts, headExplanations, err = relations.explanationPredicateSignature(plan.head)
	} else {
		headSorts, _, err = relations.PredicateSignature(plan.head)
	}
	if err != nil {
		return plan, errors.Join(ErrCompilerUnsupported, err)
	}
	if headExplanations > 1 {
		return plan, ErrCompilerUnsupported
	}
	plan.columns = make(map[uint32]int)
	plan.sorts = make(map[uint32]ast.ASTHandle)
	variable := func(application ast.ASTHandle, column int) (uint32, ast.ASTHandle, error) {
		value, err := m.GetArg(application, uint32(column))
		if err != nil {
			return 0, ast.ASTHandle{}, err
		}
		kind, err := m.GetKind(value)
		if err != nil {
			return 0, ast.ASTHandle{}, err
		}
		if kind != ast.ASTVar {
			return 0, ast.ASTHandle{}, ErrCompilerUnsupported
		}
		index, err := m.GetVarIndex(value)
		if err != nil {
			return 0, ast.ASTHandle{}, err
		}
		sort, err := m.GetSort(value)
		return index, sort, err
	}
	tailVariables := make(map[uint32][]int)
	finiteOffset := 0
	for tailIndex, tail := range tails {
		finite := len(tailSignatures[tailIndex])
		for local := 0; local <= finite; local++ {
			column := finiteOffset + local
			if local == finite {
				// Concrete products group all finite columns before both
				// explanation rows, preserving the original tail order.
				column = len(tailSorts) + tailIndex
			}
			argument, err := m.GetArg(tail, uint32(local))
			if err != nil {
				return plan, err
			}
			kind, err := m.GetKind(argument)
			if err != nil {
				return plan, err
			}
			if kind != ast.ASTVar {
				if local == finite {
					return plan, ErrCompilerUnsupported
				}
				value, err := relations.EncodeValue(tailSorts[column], argument)
				if err != nil {
					return plan, err
				}
				plan.tailConstants = append(plan.tailConstants, explanationTailConstant{column: column, value: value})
				continue
			}
			index, sort, err := variable(tail, local)
			if err != nil {
				return plan, err
			}
			if previous, exists := plan.columns[index]; exists {
				if column >= len(tailSorts) || previous >= len(tailSorts) || plan.sorts[index] != sort {
					return plan, ErrCompilerUnsupported
				}
			} else {
				plan.columns[index], plan.sorts[index] = column, sort
			}
			if tailIndex == 1 && local < finite {
				// Source variable_intersection adds every matching pair.
				for _, left := range tailVariables[index] {
					if left < finiteOffset {
						plan.joinLeft = append(plan.joinLeft, left)
						plan.joinRight = append(plan.joinRight, local)
					}
				}
			}
			tailVariables[index] = append(tailVariables[index], column)
		}
		finiteOffset += finite
	}
	for _, columns := range tailVariables {
		if len(columns) > 1 {
			if positive == 2 && columns[0] < len(tailSignatures[0]) && columns[len(columns)-1] >= len(tailSignatures[0]) {
				// Cross-tail identity is already enforced by all join pairs.
				continue
			}
			plan.identical = append(plan.identical, columns)
		}
	}
	sort.Slice(plan.identical, func(i, j int) bool { return plan.identical[i][0] < plan.identical[j][0] })
	plan.finiteWidth = len(tailSorts)
	plan.explanationWidth = positive
	var headColumns []int
	available := make(map[uint32][]int, len(tailVariables))
	for variable, columns := range tailVariables {
		available[variable] = append([]int(nil), columns...)
	}
	keep := make(map[int]bool)
	for column, headSort := range headSorts {
		argument, err := m.GetArg(head, uint32(column))
		if err != nil {
			return plan, err
		}
		kind, err := m.GetKind(argument)
		if err != nil {
			return plan, err
		}
		if kind != ast.ASTVar {
			value, err := relations.EncodeValue(headSort, argument)
			if err != nil {
				return plan, err
			}
			from := len(tailSorts) + len(plan.appendFinite)
			plan.appendFinite = append(plan.appendFinite, explanationFiniteColumn{sort: headSort, value: value, singleton: true})
			headColumns = append(headColumns, from)
			continue
		}
		index, actual, err := variable(head, column)
		if err != nil {
			return plan, err
		}
		if actual != headSort {
			return plan, ErrCompilerUnsupported
		}
		from, bound := plan.columns[index]
		if !bound {
			from = len(tailSorts) + len(plan.appendFinite)
			plan.appendFinite = append(plan.appendFinite, explanationFiniteColumn{sort: headSort})
			plan.columns[index], plan.sorts[index] = from, headSort
			available[index] = []int{from}
		} else if plan.sorts[index] != headSort {
			return plan, ErrCompilerUnsupported
		}
		if columns := available[index]; len(columns) != 0 {
			from = columns[0]
			available[index] = columns[1:]
		} else {
			plan.duplicates = append(plan.duplicates, from)
			// More finite columns may follow this repeated head argument.
			// Resolve duplicate positions after all full/singleton columns.
			from = -len(plan.duplicates)
		}
		headColumns = append(headColumns, from)
	}
	plan.finiteWidth += len(plan.appendFinite)
	for index, variableSort := range plan.sorts {
		ruleSort, err := m.IsDatalogRuleSort(variableSort)
		if err != nil {
			return plan, err
		}
		if ruleSort {
			plan.columns[index] += len(plan.appendFinite)
		}
	}
	for index, column := range headColumns {
		if column < 0 {
			column = plan.finiteWidth - column - 1
			headColumns[index] = column
		}
		keep[column] = true
	}
	width := plan.finiteWidth + plan.explanationWidth
	if headExplanations == 1 {
		index, sort, err := variable(head, len(headSorts))
		if err != nil {
			return plan, err
		}
		if _, exists := plan.columns[index]; exists {
			return plan, ErrCompilerUnsupported
		}
		plan.columns[index], plan.sorts[index] = width, sort
		plan.appendHead = true
		plan.explanationWidth++
	}
	width = plan.finiteWidth + len(plan.duplicates) + plan.explanationWidth
	if plan.appendHead {
		keep[width-1] = true
	}
	for column := 0; column < width; column++ {
		if !keep[column] {
			plan.removed = append(plan.removed, column)
		}
	}
	sorted := append([]int(nil), headColumns...)
	sort.Ints(sorted)
	positions := make(map[int]int, len(sorted))
	for index, column := range sorted {
		positions[column] = index
	}
	for _, column := range headColumns {
		plan.permutation = append(plan.permutation, positions[column])
	}
	tailCount, err := rule.TailSize()
	if err != nil {
		return plan, err
	}
	for index := uninterpreted; index < tailCount; index++ {
		condition, negated, err := rule.Tail(index)
		if err != nil {
			return plan, err
		}
		if negated {
			return plan, ErrCompilerUnsupported
		}
		used, err := ast.NewASTUsedVars(m)
		if err != nil {
			return plan, err
		}
		if err = used.Collect(condition); err != nil {
			return plan, err
		}
		var known uint32
		for index, sort := range plan.sorts {
			found := used.Contains(index)
			if !found.IsNull() {
				if found != sort {
					return plan, ErrCompilerUnsupported
				}
				known++
			}
		}
		if known != used.NumVars() {
			return plan, ErrCompilerUnsupported
		}
		plan.conditions = append(plan.conditions, condition)
	}
	return plan, nil
}

// CompileExplanationRuleWithGuard lowers a ground, one-tail or two-tail
// explained rule or its finite output bridge. Tail constants and repeated
// finite columns use filtering and duplicate-column lowering. Finite head
// constants and unbound variables use singleton/full columns. Rules with more
// than two positive tails and condition-only unbound variables are rejected
// before a program is published.
func CompileExplanationRuleWithGuard(relations *RelationManager, rule *base.Rule, guard *rewriter.TheoryRewriter) (result *InstructionBlock, resultErr error) {
	plan, err := planExplanationRule(relations, rule)
	if err != nil {
		return nil, err
	}
	if len(plan.conditions) != 0 && guard == nil {
		return nil, ErrCompilerUnsupported
	}
	result = NewInstructionBlock()
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	emit := func(instruction Instruction) error {
		if err := instruction.Accounting().SetAccountingParentRule(rule); err != nil {
			return errors.Join(err, instruction.Close())
		}
		if err := result.Append(instruction); err != nil {
			return errors.Join(err, instruction.Close())
		}
		return nil
	}
	next := uint32(0)
	fresh := func() uint32 { register := next; next++; return register }
	current := fresh()
	var load Instruction
	if plan.tail.IsNull() {
		// Source make_full_relation supplies the join identity when no
		// positive body predicate has established a current relation.
		load, err = NewExplanationTotalInstruction(relations, 0, current)
	} else {
		load, err = NewLoadInstruction(relations, plan.tail, current)
	}
	if err != nil {
		return result, err
	}
	if err = emit(load); err != nil {
		return result, err
	}
	if !plan.tail2.IsNull() {
		right := fresh()
		load, err = NewLoadInstruction(relations, plan.tail2, right)
		if err != nil {
			return result, err
		}
		if err = emit(load); err != nil {
			return result, err
		}
		joined := fresh()
		if err = emit(NewJoinInstruction(current, right, plan.joinLeft, plan.joinRight, joined)); err != nil {
			return result, err
		}
		if err = emit(NewDeallocateInstruction(current)); err != nil {
			return result, err
		}
		if err = emit(NewDeallocateInstruction(right)); err != nil {
			return result, err
		}
		current = joined
	}
	for _, constant := range plan.tailConstants {
		if err = emit(NewFilterEqualInstruction(current, constant.value, constant.column)); err != nil {
			return result, err
		}
	}
	for _, columns := range plan.identical {
		if err = emit(NewFilterIdenticalInstruction(current, columns)); err != nil {
			return result, err
		}
	}
	for _, column := range plan.appendFinite {
		unary := fresh()
		var instruction Instruction
		if column.singleton {
			instruction, err = NewExplanationFiniteSingletonInstruction(relations, column.sort, column.value, unary)
		} else {
			instruction, err = NewExplanationFiniteTotalInstruction(relations, column.sort, unary)
		}
		if err != nil {
			return result, err
		}
		if err = emit(instruction); err != nil {
			return result, err
		}
		joined := fresh()
		if err = emit(NewJoinInstruction(current, unary, nil, nil, joined)); err != nil {
			return result, err
		}
		if err = emit(NewDeallocateInstruction(current)); err != nil {
			return result, err
		}
		if err = emit(NewDeallocateInstruction(unary)); err != nil {
			return result, err
		}
		current = joined
	}
	if plan.appendHead {
		total := fresh()
		instruction, err := NewExplanationTotalInstruction(relations, 1, total)
		if err != nil {
			return result, err
		}
		if err = emit(instruction); err != nil {
			return result, err
		}
		joined := fresh()
		if err = emit(NewJoinInstruction(current, total, nil, nil, joined)); err != nil {
			return result, err
		}
		if err = emit(NewDeallocateInstruction(current)); err != nil {
			return result, err
		}
		if err = emit(NewDeallocateInstruction(total)); err != nil {
			return result, err
		}
		current = joined
	}
	if len(plan.conditions) != 0 {
		filter, err := NewFilterInterpretedInstruction(relations.manager, relations, guard, current, plan.conditions, plan.columns, plan.sorts)
		if err != nil {
			return result, err
		}
		if err = emit(filter); err != nil {
			return result, err
		}
	}
	finiteWidth := plan.finiteWidth
	for _, column := range plan.duplicates {
		// Source make_duplicate_column projects a unary column and joins it
		// back on equality, preserving the explanation suffix just once.
		var removed []int
		for index := 0; index < finiteWidth+plan.explanationWidth; index++ {
			if index != column {
				removed = append(removed, index)
			}
		}
		single := fresh()
		if err = emit(NewProjectionInstruction(current, removed, single)); err != nil {
			return result, err
		}
		joined := fresh()
		if err = emit(NewJoinInstruction(current, single, []int{column}, []int{0}, joined)); err != nil {
			return result, err
		}
		if err = emit(NewDeallocateInstruction(current)); err != nil {
			return result, err
		}
		if err = emit(NewDeallocateInstruction(single)); err != nil {
			return result, err
		}
		current = joined
		finiteWidth++
	}
	projected := fresh()
	if err = emit(NewProjectionInstruction(current, plan.removed, projected)); err != nil {
		return result, err
	}
	if err = emit(NewDeallocateInstruction(current)); err != nil {
		return result, err
	}
	current = projected
	visited := make([]bool, len(plan.permutation))
	for start := range visited {
		if visited[start] || plan.permutation[start] == start {
			continue
		}
		var cycle []int
		for index := start; !visited[index]; index = plan.permutation[index] {
			visited[index] = true
			cycle = append(cycle, index)
		}
		renamed := fresh()
		if err = emit(NewRenameInstruction(current, cycle, renamed)); err != nil {
			return result, err
		}
		if err = emit(NewDeallocateInstruction(current)); err != nil {
			return result, err
		}
		current = renamed
	}
	target := fresh()
	load, err = NewLoadInstruction(relations, plan.head, target)
	if err != nil {
		return result, err
	}
	if err = emit(load); err != nil {
		return result, err
	}
	if err = emit(NewUnionInstruction(current, target, VoidRegister)); err != nil {
		return result, err
	}
	if err = emit(NewDeallocateInstruction(current)); err != nil {
		return result, err
	}
	store, err := NewStoreInstruction(relations, plan.head, target)
	if err != nil {
		return result, err
	}
	if err = emit(store); err != nil {
		return result, err
	}
	return result, nil
}
