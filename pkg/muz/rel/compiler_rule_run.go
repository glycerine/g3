package rel

// Source: src/muz/rel/dl_compiler.cpp compiler::compile_rule_evaluation_run,
// add_unbound_columns_for_negation, make_assembling_code and the make_*
// register helpers they call.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// compilerUMap is a u_map<V> (z3 core_hashtable keyed by u_hash(k) == k:
// initial capacity 8, linear probing, deleted markers, growth when
// (size+deleted)*4 > capacity*3 before an insert). Only the slot layout is
// kept here; the values live in a Go map beside it.
type compilerUMap struct {
	slots   []compilerUMapSlot
	size    int
	deleted int
}

type compilerUMapSlot struct {
	state uint8 // 0 free, 1 deleted, 2 used
	key   uint32
}

const compilerUMapSmallCapacity = 64

func newCompilerUMap() *compilerUMap {
	return &compilerUMap{slots: make([]compilerUMapSlot, 8)}
}

func (t *compilerUMap) moveInto(target []compilerUMapSlot) {
	mask := uint32(len(target) - 1)
	for _, s := range t.slots {
		if s.state != 2 {
			continue
		}
		index := s.key & mask
		for target[index].state != 0 {
			index = (index + 1) & mask
		}
		target[index] = s
	}
}

// insert is insert_if_not_there2: it reports whether the key was new.
func (t *compilerUMap) insert(key uint32) bool {
	if (t.size+t.deleted)<<2 > len(t.slots)*3 {
		grown := make([]compilerUMapSlot, len(t.slots)<<1)
		t.moveInto(grown)
		t.slots = grown
		t.deleted = 0
	}
	mask := uint32(len(t.slots) - 1)
	begin := key & mask
	del := -1
	for step := 0; step < len(t.slots); step++ {
		index := int((begin + uint32(step)) & mask)
		switch t.slots[index].state {
		case 2:
			if t.slots[index].key == key {
				return false
			}
		case 0:
			if del >= 0 {
				index = del
				t.deleted--
			}
			t.slots[index] = compilerUMapSlot{state: 2, key: key}
			t.size++
			return true
		default:
			del = index
		}
	}
	// No free slot: the load factor check above keeps one available.
	return false
}

func (t *compilerUMap) remove(key uint32) {
	mask := uint32(len(t.slots) - 1)
	begin := key & mask
	for step := 0; step < len(t.slots); step++ {
		index := int((begin + uint32(step)) & mask)
		switch t.slots[index].state {
		case 2:
			if t.slots[index].key == key {
				t.slots[index].state = 1
				t.deleted++
				t.size--
				if t.deleted > t.size && t.deleted > compilerUMapSmallCapacity {
					// remove_deleted_entries rebuilds at the same capacity.
					fresh := make([]compilerUMapSlot, len(t.slots))
					t.moveInto(fresh)
					t.slots = fresh
					t.deleted = 0
				}
				return
			}
		case 0:
			return
		}
	}
}

// keys lists the used slots in table order (the map's iterator).
func (t *compilerUMap) keys() []uint32 {
	result := make([]uint32, 0, t.size)
	for _, s := range t.slots {
		if s.state == 2 {
			result = append(result, s.key)
		}
	}
	return result
}

// int2ints is compiler::int2ints (u_map<unsigned_vector>).
type compilerInt2Ints struct {
	layout *compilerUMap
	values map[uint32][]int
}

func newCompilerInt2Ints() *compilerInt2Ints {
	return &compilerInt2Ints{layout: newCompilerUMap(), values: make(map[uint32][]int)}
}

func (m *compilerInt2Ints) push(key uint32, column int) {
	m.layout.insert(key)
	m.values[key] = append(m.values[key], column)
}

func (m *compilerInt2Ints) find(key uint32) ([]int, bool) {
	value, found := m.values[key]
	return value, found
}

func (m *compilerInt2Ints) remove(key uint32) {
	if _, found := m.values[key]; !found {
		return
	}
	m.layout.remove(key)
	delete(m.values, key)
}

// getRegister is compiler::get_register.
func (c *finiteCompiler) getRegister(signature Signature, reuse bool, r uint32) uint32 {
	if !reuse {
		return c.fresh(signature)
	}
	c.signatures[r] = Signature{domains: signature.Domains()}
	return r
}

func joinedSignature(first, second Signature) (Signature, error) {
	return NewSignature(append(first.Domains(), second.domains...))
}

// makeJoin is compiler::make_join.
func (c *finiteCompiler) makeJoin(t1, t2 uint32, cols1, cols2 []int, reuseT1 bool, rule *base.Rule) (uint32, error) {
	signature, err := joinedSignature(c.signatures[t1], c.signatures[t2])
	if err != nil {
		return 0, err
	}
	result := c.getRegister(signature, reuseT1, t1)
	return result, c.emit(NewJoinInstruction(t1, t2, cols1, cols2, result), rule)
}

// makeJoinProject is compiler::make_join_project.
func (c *finiteCompiler) makeJoinProject(t1, t2 uint32, cols1, cols2, removed []int, reuseT1 bool, rule *base.Rule) (uint32, error) {
	auxiliary, err := joinedSignature(c.signatures[t1], c.signatures[t2])
	if err != nil {
		return 0, err
	}
	signature, err := projectedSignature(auxiliary, removed)
	if err != nil {
		return 0, err
	}
	result := c.getRegister(signature, reuseT1, t1)
	return result, c.emit(NewJoinProjectInstruction(t1, t2, cols1, cols2, removed, result), rule)
}

// makeSelectEqualAndProject is compiler::make_select_equal_and_project.
func (c *finiteCompiler) makeSelectEqualAndProject(source uint32, value Element, column int, reuse bool, rule *base.Rule) (uint32, error) {
	signature, err := projectedSignature(c.signatures[source], []int{column})
	if err != nil {
		return 0, err
	}
	result := c.getRegister(signature, reuse, source)
	return result, c.emit(NewSelectEqualProjectInstruction(source, value, column, result), rule)
}

// makeClone is compiler::make_clone.
func (c *finiteCompiler) makeClone(source uint32, rule *base.Rule) (uint32, error) {
	result := c.fresh(c.signatures[source])
	return result, c.emit(NewCloneInstruction(source, result), rule)
}

// makeProjection is compiler::make_projection.
func (c *finiteCompiler) makeProjection(source uint32, removed []int, reuse bool, rule *base.Rule) (uint32, error) {
	signature, err := projectedSignature(c.signatures[source], removed)
	if err != nil {
		return 0, err
	}
	result := c.getRegister(signature, reuse, source)
	return result, c.emit(NewProjectionInstruction(source, removed, result), rule)
}

// makeRename is compiler::make_rename.
func (c *finiteCompiler) makeRename(source uint32, cycle []int, reuse bool, rule *base.Rule) (uint32, error) {
	signature, err := renamedSignature(c.signatures[source], cycle)
	if err != nil {
		return 0, err
	}
	result := c.getRegister(signature, reuse, source)
	return result, c.emit(NewRenameInstruction(source, cycle, result), rule)
}

// makeDeallocNonVoid is compiler::make_dealloc_non_void.
func (c *finiteCompiler) makeDeallocNonVoid(r uint32, rule *base.Rule) error {
	if r == VoidRegister {
		return nil
	}
	return c.emit(NewDeallocateInstruction(r), rule)
}

// makeAddConstantColumn is compiler::make_add_constant_column.
func (c *finiteCompiler) makeAddConstantColumn(rule *base.Rule, headPred ast.ASTHandle, source uint32, sort, constant ast.ASTHandle, dealloc *bool) (uint32, error) {
	domain, err := c.relations.sortDomain(sort)
	if err != nil {
		return 0, err
	}
	value, err := c.relations.EncodeValue(sort, constant)
	if err != nil {
		return 0, err
	}
	singleton, err := c.constantRegister(rule, headPred, sort, domain, value)
	if err != nil {
		return 0, err
	}
	if source == VoidRegister {
		return singleton, nil
	}
	result, err := c.makeJoin(source, singleton, nil, nil, *dealloc, rule)
	*dealloc = true
	return result, err
}

// makeAddUnboundColumn is compiler::make_add_unbound_column.
func (c *finiteCompiler) makeAddUnboundColumn(rule *base.Rule, column int, pred ast.ASTHandle, source uint32, sort ast.ASTHandle, dealloc *bool) (uint32, error) {
	if err := unsafeRuleColumnVerbose(c.manager, rule, column); err != nil {
		return 0, err
	}
	domain, err := c.relations.sortDomain(sort)
	if err != nil {
		return 0, err
	}
	total, err := c.totalRegister(rule, pred, sort, domain)
	if err != nil {
		return 0, err
	}
	if source == VoidRegister {
		return total, nil
	}
	result, err := c.makeJoin(source, total, nil, nil, *dealloc, rule)
	*dealloc = true
	return result, err
}

// makeDuplicateColumn is compiler::make_duplicate_column.
func (c *finiteCompiler) makeDuplicateColumn(source uint32, column int, reuse bool, rule *base.Rule) (uint32, error) {
	count := c.signatures[source].Len()
	single := source
	if count != 1 {
		removed := make([]int, 0, count-1)
		for index := 0; index < count; index++ {
			if index != column {
				removed = append(removed, index)
			}
		}
		var err error
		if single, err = c.makeProjection(source, removed, false, rule); err != nil {
			return 0, err
		}
	}
	result, err := c.makeJoin(source, single, []int{column}, []int{0}, reuse, rule)
	if err != nil {
		return 0, err
	}
	if count != 1 {
		if err = c.makeDeallocNonVoid(single, rule); err != nil {
			return 0, err
		}
	}
	return result, nil
}

type assemblingColumnKind uint8

const (
	ackBoundVar assemblingColumnKind = iota
	ackUnboundVar
	ackConstant
)

// assemblingColumnInfo is compiler::assembling_column_info.
type assemblingColumnInfo struct {
	domain       ast.ASTHandle
	kind         assemblingColumnKind
	sourceColumn int
	varIndex     uint32
	constant     ast.ASTHandle
}

// makeAssemblingCode is compiler::make_assembling_code.
func (c *finiteCompiler) makeAssemblingCode(rule *base.Rule, headPred ast.ASTHandle, source uint32, acis0 []assemblingColumnInfo, dealloc *bool) (uint32, error) {
	columnCount := len(acis0)
	current := source
	sourceColumnCount := 0
	if current != VoidRegister {
		sourceColumnCount = c.signatures[current].Len()
	}
	acis := append([]assemblingColumnInfo(nil), acis0...)
	handledUnbound := make(map[uint32]int)

	//first remove unused source columns
	referenced := make(map[int]bool)
	for i := 0; i < columnCount; i++ {
		if acis[i].kind == ackBoundVar {
			referenced[acis[i].sourceColumn] = true
		}
	}
	//if an ACK_BOUND_VAR pointed to column i, after projection it will point to
	//i-new_src_col_offset[i] due to removal of some of earlier columns
	offsets := make([]int, 0, sourceColumnCount)
	removed := make([]int, 0)
	for i := 0; i < sourceColumnCount; i++ {
		if !referenced[i] {
			removed = append(removed, i)
		}
		offsets = append(offsets, len(removed))
	}
	var err error
	if len(removed) != 0 {
		if current, err = c.makeProjection(current, removed, *dealloc, rule); err != nil {
			return 0, err
		}
		*dealloc = true
		//update ACK_BOUND_VAR references
		for i := 0; i < columnCount; i++ {
			if acis[i].kind == ackBoundVar {
				column := acis[i].sourceColumn
				acis[i].sourceColumn = column - offsets[column]
			}
		}
	}

	currentWidth := func() int {
		if current == VoidRegister {
			return 0
		}
		return c.signatures[current].Len()
	}
	//convert all result columns into bound variables by extending the source table
	for i := 0; i < columnCount; i++ {
		if acis[i].kind == ackBoundVar {
			continue
		}
		boundColumn, handled := 0, false
		if acis[i].kind == ackUnboundVar {
			boundColumn, handled = handledUnbound[acis[i].varIndex]
		}
		if !handled {
			boundColumn = currentWidth()
			if acis[i].kind == ackConstant {
				current, err = c.makeAddConstantColumn(rule, headPred, current, acis[i].domain, acis[i].constant, dealloc)
			} else {
				current, err = c.makeAddUnboundColumn(rule, i, headPred, current, acis[i].domain, dealloc)
				handledUnbound[acis[i].varIndex] = boundColumn
			}
			if err != nil {
				return 0, err
			}
		}
		acis[i].kind = ackBoundVar
		acis[i].sourceColumn = boundColumn
	}

	//duplicate needed source columns
	used := make(map[int]bool)
	for i := 0; i < columnCount; i++ {
		column := acis[i].sourceColumn
		if !used[column] {
			used[column] = true
			continue
		}
		if current, err = c.makeDuplicateColumn(current, column, *dealloc, rule); err != nil {
			return 0, err
		}
		*dealloc = true
		acis[i].sourceColumn = currentWidth() - 1
	}

	//reorder source columns to match target
	if currentWidth() != columnCount {
		return 0, ErrCompilerUnsupported
	}
	for i := 0; i < columnCount; i++ {
		if acis[i].sourceColumn == i {
			continue
		}
		permutation := make([]int, 0, 2)
		next := i
		for {
			permutation = append(permutation, next)
			previous := next
			next = acis[previous].sourceColumn
			if next < i || next >= columnCount || len(permutation) > columnCount {
				return 0, ErrCompilerUnsupported
			}
			acis[previous].sourceColumn = previous
			if next == i {
				break
			}
		}
		if current, err = c.makeRename(current, permutation, *dealloc, rule); err != nil {
			return 0, err
		}
		*dealloc = true
	}

	if current == VoidRegister {
		if current, err = c.emptyTotalRegister(rule, headPred); err != nil {
			return 0, err
		}
		*dealloc = false
	}
	return current, nil
}

// ruleRunExprs is the single_res_expr vector with the references the
// expr_ref_vector holds for the variables the compiler creates.
type ruleRunExprs struct {
	m     *ast.ASTManager
	exprs []ast.ASTHandle
	refs  []ast.ASTRef
}

func (e *ruleRunExprs) push(handle ast.ASTHandle) {
	e.exprs = append(e.exprs, handle)
}

func (e *ruleRunExprs) pushOwned(handle ast.ASTHandle) error {
	reference, err := e.m.NewRef(handle)
	if err != nil {
		return err
	}
	e.refs = append(e.refs, reference)
	e.exprs = append(e.exprs, handle)
	return nil
}

func (e *ruleRunExprs) release() (result error) {
	for index := len(e.refs) - 1; index >= 0; index-- {
		result = errors.Join(result, e.m.DelRef(&e.refs[index]))
	}
	e.refs = nil
	return result
}

// addUnboundColumnsForNegation is compiler::add_unbound_columns_for_negation.
func (c *finiteCompiler) addUnboundColumnsForNegation(rule finiteCompiledRule, pred ast.ASTHandle, singleRes *uint32, singleResExpr *ruleRunExprs, dealloc *bool) error {
	// no negated predicates
	if len(rule.negative) == 0 {
		return nil
	}
	// populate negative variables:
	negKeys := make([]uint32, 0)
	negVars := make(map[uint32]ast.ASTHandle)
	for _, negTail := range rule.negative {
		count, err := c.manager.GetNumArgs(negTail)
		if err != nil {
			return err
		}
		for j := uint32(0); j < count; j++ {
			e, err := c.manager.GetArg(negTail, j)
			if err != nil {
				return err
			}
			kind, err := c.manager.GetKind(e)
			if err != nil {
				return err
			}
			if kind != ast.ASTVar {
				continue
			}
			idx, err := c.manager.GetVarIndex(e)
			if err != nil {
				return err
			}
			// neg_vars.insert(idx, e) overwrites the value of a present key.
			negKeys = append(negKeys, idx)
			negVars[idx] = e
		}
	}
	// populate positive variables:
	posVars := make(map[uint32]bool)
	for _, e := range singleResExpr.exprs {
		kind, err := c.manager.GetKind(e)
		if err != nil {
			return err
		}
		if kind == ast.ASTVar {
			idx, err := c.manager.GetVarIndex(e)
			if err != nil {
				return err
			}
			posVars[idx] = true
		}
	}
	// add negative variables that are not in positive
	for _, v := range uMapOrder(negKeys) {
		e := negVars[v]
		if posVars[v] {
			continue
		}
		singleResExpr.push(e)
		sort, err := c.manager.GetSort(e)
		if err != nil {
			return err
		}
		if *singleRes, err = c.makeAddUnboundColumn(rule.rule, int(v), pred, *singleRes, sort, dealloc); err != nil {
			return err
		}
	}
	return nil
}

// varSorts lists the sort of every single_res_expr entry: the relation
// sorts of the register those expressions describe.
func (c *finiteCompiler) exprSorts(exprs []ast.ASTHandle) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(exprs))
	for index, e := range exprs {
		sort, err := c.manager.GetSort(e)
		if err != nil {
			return nil, err
		}
		result[index] = sort
	}
	return result, nil
}

// localIndexesForProjection is compiler::get_local_indexes_for_projection
// for a rule with two positive tails.
func (c *finiteCompiler) localIndexesForProjection(rule finiteCompiledRule) ([]int, error) {
	counter := make(map[uint32]int)
	countVars := func(root ast.ASTHandle, coefficient int) error {
		count, err := c.manager.GetNumArgs(root)
		if err != nil {
			return err
		}
		for index := uint32(0); index < count; index++ {
			argument, err := c.manager.GetArg(root, index)
			if err != nil {
				return err
			}
			kind, err := c.manager.GetKind(argument)
			if err != nil {
				return err
			}
			if kind != ast.ASTVar {
				continue
			}
			variable, err := c.manager.GetVarIndex(argument)
			if err != nil {
				return err
			}
			counter[variable] += coefficient
		}
		return nil
	}
	// leave one column copy per var in the head (avoids later duplication)
	if err := countVars(rule.head, -1); err != nil {
		return nil, err
	}
	// take interp & neg preds into account (at least 1 column copy if referenced)
	later := make(map[uint32]ast.ASTHandle)
	for _, terms := range [][]ast.ASTHandle{rule.negative, rule.interpreted} {
		for _, term := range terms {
			if err := c.conditionVariables(term, later); err != nil {
				return nil, err
			}
		}
	}
	for variable := range later {
		if counter[variable] == 0 {
			counter[variable] = -1
		}
	}
	if err := countVars(rule.tail, 1); err != nil {
		return nil, err
	}
	if err := countVars(rule.tail2, 1); err != nil {
		return nil, err
	}
	result := make([]int, 0)
	offset := 0
	for _, tail := range []ast.ASTHandle{rule.tail, rule.tail2} {
		count, err := c.manager.GetNumArgs(tail)
		if err != nil {
			return nil, err
		}
		for index := uint32(0); index < count; index++ {
			argument, err := c.manager.GetArg(tail, index)
			if err != nil {
				return nil, err
			}
			kind, err := c.manager.GetKind(argument)
			if err != nil {
				return nil, err
			}
			if kind != ast.ASTVar {
				continue
			}
			variable, err := c.manager.GetVarIndex(argument)
			if err != nil {
				return nil, err
			}
			if counter[variable] > 0 {
				counter[variable]--
				result = append(result, int(index)+offset)
			}
		}
		offset += int(count)
	}
	return result, nil
}

// variableIntersection is variable_intersection::populate over two argument
// lists: every pair of positions holding the same variable, first list
// outermost.
func (c *finiteCompiler) variableIntersection(first, second []ast.ASTHandle) ([]int, []int, error) {
	var cols1, cols2 []int
	for i1, e1 := range first {
		k1, err := c.manager.GetKind(e1)
		if err != nil {
			return nil, nil, err
		}
		if k1 != ast.ASTVar {
			continue
		}
		v1, err := c.manager.GetVarIndex(e1)
		if err != nil {
			return nil, nil, err
		}
		for i2, e2 := range second {
			k2, err := c.manager.GetKind(e2)
			if err != nil {
				return nil, nil, err
			}
			if k2 != ast.ASTVar {
				continue
			}
			v2, err := c.manager.GetVarIndex(e2)
			if err != nil {
				return nil, nil, err
			}
			if v1 == v2 {
				cols1 = append(cols1, i1)
				cols2 = append(cols2, i2)
			}
		}
	}
	return cols1, cols2, nil
}

func (c *finiteCompiler) appArgs(application ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := c.manager.GetNumArgs(application)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, count)
	for index := uint32(0); index < count; index++ {
		if result[index], err = c.manager.GetArg(application, index); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// compileRuleEvaluationRun is compiler::compile_rule_evaluation_run.
// tailRegs holds a register for every positive tail.
func (c *finiteCompiler) compileRuleEvaluationRun(rule finiteCompiledRule, headReg uint32, tailRegs []uint32, deltaReg uint32) (resultErr error) {
	m := c.manager
	r := rule.rule
	headPred, err := m.GetDecl(rule.head)
	if err != nil {
		return err
	}
	headArgs, err := c.appArgs(rule.head)
	if err != nil {
		return err
	}
	ptLen := len(tailRegs)

	var singleRes uint32
	singleResExpr := &ruleRunExprs{m: m}
	defer func() { resultErr = errors.Join(resultErr, singleResExpr.release()) }()

	//used to save on filter_identical instructions where the check is already done
	//by the join operation
	secondTailArgOfs := 0

	// whether to dealloc the previous result
	dealloc := true

	switch ptLen {
	case 2:
		t1Reg, t2Reg := tailRegs[0], tailRegs[1]
		a1, err := c.appArgs(rule.tail)
		if err != nil {
			return err
		}
		a2, err := c.appArgs(rule.tail2)
		if err != nil {
			return err
		}
		cols1, cols2, err := c.variableIntersection(a1, a2)
		if err != nil {
			return err
		}
		removedCols, err := c.localIndexesForProjection(rule)
		if err != nil {
			return err
		}
		if len(removedCols) == 0 {
			singleRes, err = c.makeJoin(t1Reg, t2Reg, cols1, cols2, false, r)
		} else {
			singleRes, err = c.makeJoinProject(t1Reg, t2Reg, cols1, cols2, removedCols, false, r)
		}
		if err != nil {
			return err
		}
		remIndex := 0
		for i := range a1 {
			if remIndex < len(removedCols) && removedCols[remIndex] == i {
				remIndex++
				continue
			}
			singleResExpr.push(a1[i])
		}
		secondTailArgOfs = len(singleResExpr.exprs)
		for i := range a2 {
			if remIndex < len(removedCols) && removedCols[remIndex] == i+len(a1) {
				remIndex++
				continue
			}
			singleResExpr.push(a2[i])
		}
	case 1:
		a, err := c.appArgs(rule.tail)
		if err != nil {
			return err
		}
		tailPred, err := m.GetDecl(rule.tail)
		if err != nil {
			return err
		}
		sorts, _, err := c.relations.PredicateSignature(tailPred)
		if err != nil {
			return err
		}
		singleRes = tailRegs[0]
		dealloc = false
		for i, arg := range a {
			kind, err := m.GetKind(arg)
			if err != nil {
				return err
			}
			if kind != ast.ASTVar {
				//argument is a constant
				value, err := c.relations.EncodeValue(sorts[i], arg)
				if err != nil {
					return err
				}
				if singleRes, err = c.makeSelectEqualAndProject(singleRes, value, len(singleResExpr.exprs), dealloc, r); err != nil {
					return err
				}
				dealloc = true
			} else {
				singleResExpr.push(arg)
			}
		}
	default:
		//single_res register should never be used in this case
		singleRes = VoidRegister
		dealloc = false
	}

	if err = c.addUnboundColumnsForNegation(rule, headPred, &singleRes, singleResExpr, &dealloc); err != nil {
		return err
	}

	varIndexes := newCompilerInt2Ints()
	filteredRes := singleRes

	//enforce equality to constants
	for i, e := range singleResExpr.exprs {
		kind, err := m.GetKind(e)
		if err != nil {
			return err
		}
		if kind != ast.ASTVar {
			sort, err := m.GetSort(e)
			if err != nil {
				return err
			}
			value, err := c.relations.EncodeValue(sort, e)
			if err != nil {
				return err
			}
			if !dealloc {
				if filteredRes, err = c.makeClone(filteredRes, r); err != nil {
					return err
				}
			}
			if err = c.emit(NewFilterEqualInstruction(filteredRes, value, i), r); err != nil {
				return err
			}
			dealloc = true
		} else {
			varNum, err := m.GetVarIndex(e)
			if err != nil {
				return err
			}
			varIndexes.push(varNum, i)
		}
	}

	//enforce equality of columns
	for _, key := range varIndexes.layout.keys() {
		indexes := varIndexes.values[key]
		if len(indexes) == 1 {
			continue
		}
		if ptLen == 2 && indexes[0] < secondTailArgOfs && indexes[len(indexes)-1] >= secondTailArgOfs {
			//If variable appears in multiple tails, the identicity will already be enforced by join.
			continue
		}
		if !dealloc {
			if filteredRes, err = c.makeClone(filteredRes, r); err != nil {
				return err
			}
		}
		if err = c.emit(NewFilterIdenticalInstruction(filteredRes, append([]int(nil), indexes...)), r); err != nil {
			return err
		}
		dealloc = true
	}

	// add unbounded columns for interpreted filter
	tail := rule.interpreted
	var binding []ast.ASTHandle
	var bindingRefs []ast.ASTRef
	defer func() {
		for index := len(bindingRefs) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, m.DelRef(&bindingRefs[index]))
		}
	}()
	freeVars, err := astrewriter.NewExprFreeVars(m)
	if err != nil {
		return err
	}
	if len(tail) != 0 {
		filterCond, err := m.MkAndArray(tail)
		if err != nil {
			return err
		}
		filterRef, err := m.NewRef(filterCond)
		if err != nil {
			return err
		}
		err = freeVars.Collect(filterCond)
		err = errors.Join(err, m.DelRef(&filterRef))
		if err != nil {
			return err
		}
		size := freeVars.Size()
		// create binding
		binding = make([]ast.ASTHandle, size+1)
		for v := uint32(0); v < size; v++ {
			if !freeVars.Contains(v) {
				continue
			}
			var srcCol int
			if entry, found := varIndexes.find(v); found {
				srcCol = entry[len(entry)-1]
			} else {
				// we have an unbound variable, so we add an unbound column for it
				unboundSort, err := freeVars.Get(v)
				if err != nil {
					return err
				}
				if filteredRes, err = c.makeAddUnboundColumn(r, 0, headPred, filteredRes, unboundSort, &dealloc); err != nil {
					return err
				}
				srcCol = len(singleResExpr.exprs)
				variable, err := m.MkVar(v, unboundSort)
				if err != nil {
					return err
				}
				if err = singleResExpr.pushOwned(variable); err != nil {
					return err
				}
				varIndexes.push(v, srcCol)
			}
			varSort, err := m.GetSort(singleResExpr.exprs[srcCol])
			if err != nil {
				return err
			}
			column, err := m.MkVar(uint32(srcCol), varSort)
			if err != nil {
				return err
			}
			reference, err := m.NewRef(column)
			if err != nil {
				return err
			}
			bindingRefs = append(bindingRefs, reference)
			binding[size-v] = column
		}
	}

	// add at least one column for the negative filter
	if len(rule.negative) != 0 && filteredRes == VoidRegister {
		if filteredRes, err = c.emptyTotalRegister(r, headPred); err != nil {
			return err
		}
	}

	//enforce negative predicates
	for _, negTail := range rule.negative {
		negPred, err := m.GetDecl(negTail)
		if err != nil {
			return err
		}
		negArgs, err := c.appArgs(negTail)
		if err != nil {
			return err
		}
		tCols, negCols, err := c.variableIntersection(singleResExpr.exprs, negArgs)
		if err != nil {
			return err
		}
		negSorts, _, err := c.relations.PredicateSignature(negPred)
		if err != nil {
			return err
		}
		for i, e := range negArgs {
			kind, err := m.GetKind(e)
			if err != nil {
				return err
			}
			if kind == ast.ASTVar {
				continue
			}
			if filteredRes, err = c.makeAddConstantColumn(r, headPred, filteredRes, negSorts[i], e, &dealloc); err != nil {
				return err
			}
			tCols = append(tCols, len(singleResExpr.exprs))
			negCols = append(negCols, i)
			singleResExpr.push(e)
		}
		negReg := c.predicateRegs[negPred]
		if !dealloc {
			if filteredRes, err = c.makeClone(filteredRes, r); err != nil {
				return err
			}
		}
		if err = c.emit(NewFilterByNegationInstruction(filteredRes, negReg, tCols, negCols), r); err != nil {
			return err
		}
		dealloc = true
	}

	// enforce interpreted tail predicates
	if len(tail) != 0 {
		filterCond, err := m.MkAndArray(tail)
		if err != nil {
			return err
		}
		filterRef, err := m.NewRef(filterCond)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, m.DelRef(&filterRef)) }()

		// check if there are any columns to remove
		relationSorts, err := c.exprSorts(singleResExpr.exprs)
		if err != nil {
			return err
		}
		removeColumns := make([]int, 0)
		{
			varIdxToRemove := make([]uint32, 0)
			if err = freeVars.Collect(rule.head); err != nil {
				return err
			}
			for _, varIdx := range varIndexes.layout.keys() {
				if !freeVars.Contains(varIdx) {
					removeColumns = append(removeColumns, varIndexes.values[varIdx]...)
					varIdxToRemove = append(varIdxToRemove, varIdx)
				}
			}
			for _, varIdx := range varIdxToRemove {
				varIndexes.remove(varIdx)
			}
			// update column idx for after projection state
			if len(removeColumns) != 0 {
				offsets := make([]int, len(singleResExpr.exprs))
				for _, removed := range removeColumns {
					for column := removed; column < len(offsets); column++ {
						offsets[column]++
					}
				}
				for _, key := range varIndexes.layout.keys() {
					columns := varIndexes.values[key]
					for i := range columns {
						columns[i] -= offsets[columns[i]]
					}
				}
			}
		}

		renamedRef, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, m.DelRef(&renamedRef)) }()
		if c.relations.varSubst == nil {
			return ErrCompilerUnsupported
		}
		err = c.relations.varSubst.Substitute(filterCond, binding, renamedRef)
		err = errors.Join(err, c.relations.varSubst.Reset())
		if err != nil {
			return err
		}
		renamed, err := m.RefGet(renamedRef)
		if err != nil {
			return err
		}
		columns, sorts, err := c.renamedConditionColumns(renamed)
		if err != nil {
			return err
		}
		if len(removeColumns) == 0 {
			if !dealloc {
				if filteredRes, err = c.makeClone(filteredRes, r); err != nil {
					return err
				}
			}
			instruction, err := NewFilterInterpretedInstruction(m, c.relations, c.guard, filteredRes, []ast.ASTHandle{renamed}, columns, sorts)
			if err != nil {
				return err
			}
			if err = c.emit(instruction, r); err != nil {
				return err
			}
		} else {
			sort.Ints(removeColumns)
			projected, err := projectedSignature(c.signatures[filteredRes], removeColumns)
			if err != nil {
				return err
			}
			result := c.getRegister(projected, dealloc, filteredRes)
			instruction, err := NewFilterInterpretedAndProjectInstruction(m, c.relations, c.guard, filteredRes, []ast.ASTHandle{renamed}, columns, sorts, relationSorts, removeColumns, result)
			if err != nil {
				return err
			}
			if err = c.emit(instruction, r); err != nil {
				return err
			}
			filteredRes = result
		}
		dealloc = true
	}

	{
		//put together the columns of head relation
		headSorts, _, err := c.relations.PredicateSignature(headPred)
		if err != nil {
			return err
		}
		headAcis := make([]assemblingColumnInfo, 0, len(headArgs))
		for i, exp := range headArgs {
			aci := assemblingColumnInfo{domain: headSorts[i]}
			kind, err := m.GetKind(exp)
			if err != nil {
				return err
			}
			if kind == ast.ASTVar {
				varNum, err := m.GetVarIndex(exp)
				if err != nil {
					return err
				}
				if bindingIndexes, found := varIndexes.find(varNum); found {
					aci.kind = ackBoundVar
					aci.sourceColumn = bindingIndexes[len(bindingIndexes)-1]
					if len(bindingIndexes) > 1 {
						//if possible, we do not want multiple head columns
						//point to a single column in the intermediate table,
						//since then we would have to duplicate the column
						//(and remove columns we did not point to at all)
						varIndexes.values[varNum] = bindingIndexes[:len(bindingIndexes)-1]
					}
				} else {
					aci.kind = ackUnboundVar
					aci.varIndex = varNum
				}
			} else {
				aci.kind = ackConstant
				aci.constant = exp
			}
			headAcis = append(headAcis, aci)
		}

		newHeadReg, err := c.makeAssemblingCode(r, headPred, filteredRes, headAcis, &dealloc)
		if err != nil {
			return err
		}
		if !c.signatures[newHeadReg].equal(c.signatures[headReg]) {
			return ErrCompilerUnsupported
		}
		//update the head relation
		if err = c.emit(NewUnionInstruction(newHeadReg, headReg, deltaReg), r); err != nil {
			return err
		}
		if dealloc {
			if err = c.makeDeallocNonVoid(newHeadReg, r); err != nil {
				return err
			}
		}
	}
	return nil
}
