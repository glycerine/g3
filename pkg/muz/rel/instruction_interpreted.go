package rel

// Source: src/muz/rel/dl_instruction.cpp instr_filter_interpreted.

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type interpretedFilterInstruction struct {
	accounting      *base.AccountedObject
	manager         *ast.ASTManager
	relations       *RelationManager
	rewriter        *astrewriter.TheoryRewriter
	register        uint32
	conditions      []ast.ASTRef
	variableColumns map[uint32]int
	variableSorts   map[uint32]ast.ASTHandle
	numerals        *groundNumeralCache
	closed          bool
}

// NewFilterInterpretedInstruction retains conditions and copies the concrete
// variable-to-column binding. The manager, relation manager, and rewriter are
// borrowed until the instruction is closed.
func NewFilterInterpretedInstruction(manager *ast.ASTManager, relations *RelationManager, rewriter *astrewriter.TheoryRewriter, register uint32, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) (Instruction, error) {
	return newInterpretedFilterInstruction(manager, relations, rewriter, register, conditions, columns, sorts)
}

func newInterpretedFilterInstruction(manager *ast.ASTManager, relations *RelationManager, rewriter *astrewriter.TheoryRewriter, register uint32, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) (*interpretedFilterInstruction, error) {
	if manager == nil || relations == nil || rewriter == nil || len(conditions) == 0 {
		return nil, ErrInstruction
	}
	i := &interpretedFilterInstruction{
		accounting: base.NewAccountedObject(), manager: manager, relations: relations,
		rewriter: rewriter, register: register, variableColumns: make(map[uint32]int, len(columns)),
		variableSorts: make(map[uint32]ast.ASTHandle, len(sorts)),
		numerals:      newGroundNumeralCache(relations),
	}
	for variable, column := range columns {
		i.variableColumns[variable] = column
	}
	for _, variable := range sortedRelVariables(sorts) {
		sort := sorts[variable]
		i.variableSorts[variable] = sort
	}
	for _, condition := range conditions {
		reference, err := manager.NewRef(condition)
		if err != nil {
			return nil, errors.Join(err, i.Close())
		}
		i.conditions = append(i.conditions, reference)
	}
	return i, nil
}

func (i *interpretedFilterInstruction) open() error {
	if i == nil || i.closed || i.accounting == nil || i.manager == nil || i.relations == nil || i.rewriter == nil {
		return ErrInstruction
	}
	return nil
}

func (i *interpretedFilterInstruction) accepts(fact Fact) (accepted bool, resultErr error) {
	if len(i.conditions) == 1 {
		condition, err := i.manager.RefGet(i.conditions[0])
		if err != nil {
			return false, err
		}
		filter, matched, err := i.relations.notEqualFilter(condition)
		if err != nil {
			return false, err
		}
		if matched {
			return filter.keeps(i.variableColumns, fact)
		}
	}
	var maximum uint32
	for _, variable := range sortedRelVariables(i.variableSorts) {
		if variable > maximum {
			maximum = variable
		}
	}
	bindings := make([]ast.ASTHandle, 0)
	if len(i.variableSorts) != 0 {
		bindings = make([]ast.ASTHandle, int(maximum)+1)
	}
	references := make([]ast.ASTRef, 0, len(i.variableSorts)+1)
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, i.manager.DelRef(&references[index]))
		}
	}()
	for _, variable := range sortedRelVariables(i.variableSorts) {
		sort := i.variableSorts[variable]
		column, found := i.variableColumns[variable]
		if !found || column < 0 || column >= len(fact) {
			return false, ErrInstruction
		}
		handle, cached, err := i.numerals.numeral(sort, fact[column])
		if err != nil {
			return false, err
		}
		if !cached {
			reference, err := i.relations.DecodeValue(sort, fact[column])
			if err != nil {
				return false, err
			}
			references = append(references, reference)
			if handle, err = i.manager.RefGet(reference); err != nil {
				return false, err
			}
		}
		bindings[len(bindings)-int(variable)-1] = handle
	}
	for _, condition := range i.conditions {
		handle, err := i.manager.RefGet(condition)
		if err != nil {
			return false, err
		}
		out, err := i.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return false, err
		}
		references = append(references, out)
		if err = i.relations.GroundCondition(i.rewriter, handle, bindings, out); err != nil {
			return false, err
		}
		rewritten, err := i.manager.RefGet(out)
		if err != nil {
			return false, err
		}
		if i.manager.IsFalse(rewritten) {
			return false, nil
		}
		// should_remove removes a fact only when the ground condition
		// rewrites to false; any other result keeps it.
	}
	return true, nil
}

func (i *interpretedFilterInstruction) conditionHandles() ([]ast.ASTHandle, error) {
	conditions := make([]ast.ASTHandle, len(i.conditions))
	for index := range i.conditions {
		handle, err := i.manager.RefGet(i.conditions[index])
		if err != nil {
			return nil, err
		}
		conditions[index] = handle
	}
	return conditions, nil
}

func (i *interpretedFilterInstruction) relationSorts(table FiniteTable) ([]ast.ASTHandle, error) {
	signature, err := table.Signature()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, signature.Len())
	for _, variable := range sortedRelVariables(i.variableSorts) {
		sort := i.variableSorts[variable]
		column, found := i.variableColumns[variable]
		if !found || column < 0 || column >= len(result) {
			return nil, ErrInstruction
		}
		result[column] = sort
	}
	return result, nil
}

func (i *interpretedFilterInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if err := context.open(); err != nil {
		return false, err
	}
	// if (!ctx.reg(m_reg)) return true; log_verbose(ctx);
	if registerPresent(context, i.register) {
		if err := logVerbose(context, i); err != nil {
			return false, err
		}
	}
	if hasExplanationInstructionRegister(context, i.register) {
		return i.performExplanationFilter(context)
	}
	table, err := context.FiniteRegister(i.register)
	if err != nil || table == nil {
		return true, err
	}
	context.stats.Filter++
	conditions, err := i.conditionHandles()
	if err != nil {
		return false, err
	}
	if r, ok := AsRelation(table); ok {
		if len(conditions) != 1 {
			return false, ErrInstruction
		}
		if err = relationFilterInterpreted(r, conditions[0]); err != nil {
			return false, err
		}
		return i.emptyAfterFilter(context, table)
	}
	switch concrete := table.(type) {
	case *LazySparseTable:
		if err = concrete.filterInterpretedCompiled(i.relations, i.rewriter, conditions, i.variableColumns, i.variableSorts); err != nil {
			return false, err
		}
		return i.emptyAfterFilter(context, table)
	case *FiniteCheckedPair:
		relationSorts, sortErr := i.relationSorts(table)
		if sortErr != nil {
			return false, sortErr
		}
		if err = concrete.FilterInterpreted(i.relations, relationSorts, i.rewriter, conditions, i.variableColumns, i.variableSorts); err != nil {
			return false, err
		}
		return i.emptyAfterFilter(context, table)
	}
	facts, err := table.Facts()
	if err != nil {
		return false, err
	}
	kept := make([]Fact, 0, len(facts))
	removed := make([]Fact, 0)
	for _, fact := range facts {
		accepted, x := i.accepts(fact)
		if x != nil {
			return false, x
		}
		if accepted {
			kept = append(kept, fact)
		} else {
			removed = append(removed, fact)
		}
	}
	// relation_manager::auxiliary_table_filter_fn::operator() collects the
	// rows should_remove reports and hands them to r.remove_facts, which
	// for the default table plugins is table_base::remove_facts:
	// remove_fact per row in iteration order.
	handled, err := RemoveFiniteTableFacts(table, removed)
	if err != nil {
		return false, err
	}
	if handled {
		return i.emptyAfterFilter(context, table)
	}
	if err = table.Reset(); err != nil {
		return false, err
	}
	for _, fact := range kept {
		if _, err = table.Add(fact); err != nil {
			return false, err
		}
	}
	if len(kept) == 0 {
		return true, context.MakeEmpty(i.register)
	}
	return true, nil
}

// emptyAfterFilter is instr_filter_interpreted::perform's
// if (r.fast_empty()) ctx.make_empty(m_reg); table_relation::fast_empty is
// the table's exact emptiness and check_relation forwards it.
func (i *interpretedFilterInstruction) emptyAfterFilter(context *ExecutionContext, table FiniteTable) (bool, error) {
	empty, err := finiteFastEmpty(table)
	if err != nil {
		return false, err
	}
	if empty {
		return true, context.MakeEmpty(i.register)
	}
	return true, nil
}

func (i *interpretedFilterInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	return context.SetRegisterAnnotation(i.register, "filter_interpreted")
}

func (i *interpretedFilterInstruction) DisplayHead(_ *ExecutionContext, out io.Writer) error {
	if err := i.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	condition, err := i.conditionText()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "filter_interpreted %d using %s", i.register, condition)
	return err
}

// conditionText is mk_pp(m_cond): the finite compiler passes one condition;
// the explanation compiler's several are shown one after another.
func (i *interpretedFilterInstruction) conditionText() (string, error) {
	parts := make([]string, 0, len(i.conditions))
	for _, reference := range i.conditions {
		condition, err := i.manager.RefGet(reference)
		if err != nil {
			return "", err
		}
		text, err := relMkPP(i.manager, condition)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " "), nil
}

func (i *interpretedFilterInstruction) Accounting() *base.AccountedObject { return i.accounting }

func (i *interpretedFilterInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	return i.accounting.ProcessCosts()
}

func (i *interpretedFilterInstruction) CollectStatistics(out *statistics.Statistics) error {
	if err := i.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	cost, err := i.accounting.TotalCosts()
	if err != nil {
		return err
	}
	out.UpdateUint("instruction", cost.Instructions)
	out.UpdateUint("instruction-time", cost.Milliseconds)
	return nil
}

func (i *interpretedFilterInstruction) Close() (result error) {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	i.closed = true
	result = i.numerals.close()
	i.numerals = nil
	for index := len(i.conditions) - 1; index >= 0; index-- {
		result = errors.Join(result, i.manager.DelRef(&i.conditions[index]))
	}
	i.conditions = nil
	result = errors.Join(result, i.accounting.Close())
	return result
}

// RemoveFiniteTableFacts is table_base::remove_facts on the default table
// plugins (sparse, hashtable, bitvector), whose mk_filter_*_fn return
// nullptr so the relation manager's auxiliary_table_filter_fn removes the
// rejected rows in place. It reports false for tables without that path.
func RemoveFiniteTableFacts(table FiniteTable, facts []Fact) (bool, error) {
	switch concrete := table.(type) {
	case *SparseTable:
		_, err := concrete.RemoveFacts(facts)
		return true, err
	case *HashTable:
		return true, concrete.RemoveFacts(facts)
	case *BitvectorTable:
		return true, concrete.RemoveFacts(facts)
	}
	return false, nil
}
