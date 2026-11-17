package rel

// Source: src/muz/rel/dl_instruction.cpp unary-singleton, total-relation,
// and signature-assertion instructions.

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type constructInstruction struct {
	accounting *base.AccountedObject
	kind       string
	signature  Signature
	value      Element
	astValue   ast.ASTHandle
	target     uint32
	closed     bool
	relations  *RelationManager

	finiteSort    ast.ASTRef
	astValueRoot  ast.ASTRef
	finiteManager *ast.ASTManager
	pendingFinite FiniteTable
	closing       bool
	// predicateName is the func_decl* p that instr_mk_total passes to
	// mk_full_relation; hasPredicate is false for a null p.
	predicateName string
	hasPredicate  bool
	// displaySorts is m_sig's relation sorts for display_head_impl
	// (to_nice_string); borrowed from the compiled predicate signature.
	displaySorts   []ast.ASTHandle
	displayManager *ast.ASTManager
	// predicate is the instruction's func_decl* pred, whose requested kind
	// selects the plugin of mk_empty_relation / mk_full_relation.
	predicate ast.ASTHandle
}

func NewUnarySingletonInstruction(domain uint64, value Element, target uint32) (Instruction, error) {
	signature, err := NewSignature([]uint64{domain})
	if err != nil || uint64(value) >= domain {
		return nil, errors.Join(ErrInstruction, err)
	}
	return &constructInstruction{
		accounting: base.NewAccountedObject(), kind: "mk_unary_singleton",
		signature: signature, value: value, target: target,
	}, nil
}

func NewTotalInstruction(signature Signature, target uint32) Instruction {
	return &constructInstruction{
		accounting: base.NewAccountedObject(), kind: "mk_total",
		signature: Signature{domains: signature.Domains()}, target: target,
	}
}

// NewTotalInstructionForPredicate is instruction::mk_total(sig, pred, reg):
// the predicate name is reported by table_base::complement's large-table
// warning.
func NewTotalInstructionForPredicate(signature Signature, target uint32, predicateName string) Instruction {
	return &constructInstruction{
		accounting: base.NewAccountedObject(), kind: "mk_total",
		signature: Signature{domains: signature.Domains()}, target: target,
		predicateName: predicateName, hasPredicate: true,
	}
}

// NewTotalInstructionForPredicateKind is NewTotalInstructionForPredicate
// whose table is created with the predicate's requested plugin.
func NewTotalInstructionForPredicateKind(signature Signature, target uint32, predicate ast.ASTHandle, predicateName string) Instruction {
	return &constructInstruction{
		accounting: base.NewAccountedObject(), kind: "mk_total",
		signature: Signature{domains: signature.Domains()}, target: target,
		predicateName: predicateName, hasPredicate: true, predicate: predicate,
	}
}

// NewUnarySingletonInstructionForPredicate is
// instruction::mk_unary_singleton(m, pred, s, val, reg): the singleton
// relation is mk_empty_relation(sig, pred).
func NewUnarySingletonInstructionForPredicate(domain uint64, value Element, target uint32, predicate ast.ASTHandle) (Instruction, error) {
	instruction, err := NewUnarySingletonInstruction(domain, value, target)
	if err != nil {
		return nil, err
	}
	instruction.(*constructInstruction).predicate = predicate
	return instruction, nil
}

// warnLargeTotalTable is the warning of table_base::complement (dl_base.cpp),
// reached from table_plugin::mk_full for a one-column signature whose
// domain exceeds 1<<18, before the rows are enumerated.
func warnLargeTotalTable(signature Signature, predicateName string, hasPredicate bool) error {
	if signature.Len() != 1 {
		return nil
	}
	upperBound := signature.domains[0]
	if upperBound <= 1<<18 {
		return nil
	}
	if hasPredicate {
		return diagnostic.BorrowProcessWarningService().Warningf("%s", fmt.Sprintf("creating large table of size %d for relation %s", upperBound, predicateName))
	}
	return diagnostic.BorrowProcessWarningService().Warningf("%s", fmt.Sprintf("creating large table of size %d", upperBound))
}

func NewAssertSignatureInstruction(signature Signature, target uint32) Instruction {
	return &constructInstruction{
		accounting: base.NewAccountedObject(), kind: "assert_signature",
		signature: Signature{domains: signature.Domains()}, target: target,
	}
}

func (i *constructInstruction) open() error {
	if i == nil || i.closed || i.closing || i.accounting == nil {
		return ErrInstruction
	}
	return nil
}

func (i *constructInstruction) clearPendingFinite() error {
	if i.pendingFinite == nil {
		return nil
	}
	if err := i.pendingFinite.Close(); err != nil {
		return err
	}
	i.pendingFinite = nil
	return nil
}

func (i *constructInstruction) disposeUninstalledFinite(table FiniteTable, cause error) error {
	if table == nil {
		return cause
	}
	if err := table.Close(); err != nil {
		i.pendingFinite = table
		return errors.Join(cause, err)
	}
	return cause
}

func fillTotalTable(table FiniteTable, signature Signature) error {
	fact := make(Fact, signature.Len())
	for {
		if _, err := table.Add(fact); err != nil {
			return err
		}
		column := len(fact) - 1
		for column >= 0 {
			fact[column]++
			if uint64(fact[column]) < signature.domains[column] {
				break
			}
			fact[column] = 0
			column--
		}
		if column < 0 {
			return nil
		}
	}
}

func constructFiniteTable(context *ExecutionContext, signature Signature, predicate ast.ASTHandle) (FiniteTable, error) {
	if context != nil && context.relations != nil {
		if !predicate.IsNull() {
			return context.relations.newRequestedFiniteTable(predicate, signature)
		}
		return context.relations.newFavouriteFiniteTable(signature)
	}
	return NewHashTable(signature), nil
}

// relationSorts is m_sig of the instruction when the compiler supplied it.
func (i *constructInstruction) relationSorts() ([]ast.ASTHandle, bool) {
	if len(i.displaySorts) != i.signature.Len() {
		return nil, false
	}
	return i.displaySorts, true
}

// constructRelation is rmanager.mk_empty_relation(m_sig, m_pred).
func (i *constructInstruction) constructRelation(context *ExecutionContext, _ bool) (FiniteTable, error) {
	if context != nil && context.relations != nil {
		if sorts, ok := i.relationSorts(); ok {
			return context.relations.MkEmptyRelation(sorts, i.predicate)
		}
	}
	return constructFiniteTable(context, i.signature, i.predicate)
}

// fullRelation is rmanager.mk_full_relation(m_sig, m_pred) when a
// relation (not table_relation) plugin is selected for the signature.
func (i *constructInstruction) fullRelation(context *ExecutionContext) (FiniteTable, bool, error) {
	if context == nil || context.relations == nil {
		return nil, false, nil
	}
	sorts, ok := i.relationSorts()
	if !ok {
		return nil, false, nil
	}
	r, ok, err := context.relations.MkFullRelation(sorts, i.predicate)
	if err != nil || !ok {
		return nil, false, err
	}
	return r, true, nil
}

func (i *constructInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if err := logVerbose(context, i); err != nil {
		return false, err
	}
	if err := i.clearPendingFinite(); err != nil {
		return false, err
	}
	switch i.kind {
	case "mk_unary_singleton":
		context.stats.UnarySingleton++
		table, err := i.constructRelation(context, false)
		if err != nil {
			return false, err
		}
		if _, err := table.Add(Fact{i.value}); err != nil {
			return false, errors.Join(err, table.Close())
		}
		if err := context.SetFiniteRegister(i.target, table); err != nil {
			return false, i.disposeUninstalledFinite(table, err)
		}
		return true, nil
	case "mk_total":
		context.stats.Total++
		if full, ok, err := i.fullRelation(context); err != nil {
			return false, err
		} else if ok {
			if err := context.SetFiniteRegister(i.target, full); err != nil {
				return false, i.disposeUninstalledFinite(full, err)
			}
			return true, nil
		}
		var table FiniteTable
		var err error
		if context != nil && context.relations != nil {
			table, err = context.relations.newFullTable(i.predicate, i.signature)
		} else {
			table, err = constructFiniteTable(context, i.signature, i.predicate)
		}
		if err != nil {
			return false, err
		}
		// table_plugin::mk_full builds the full table as the complement of an
		// empty aux = mk_empty(s) and then calls aux->deallocate(); for the
		// sparse plugin that is sparse_table_plugin::recycle. aux is made
		// before the complement's table: the two empty tables trade their
		// pooled storage.
		var aux *SparseTable
		sparse, ok := table.(*SparseTable)
		if lazy, isLazy := table.(*LazySparseTable); isLazy && lazy.root != nil && lazy.root.kind == LazyTableBase {
			// The lazy plugin's aux is a lazy table over a sparse table:
			// lazy_table::complement complements that table, and aux's
			// deallocation recycles it.
			sparse, ok = lazy.root.table.(*SparseTable)
		}
		if ok {
			aux, err = sparse.plugin.MkEmpty(sparse.signature, sparse.functional)
			if err != nil {
				return false, errors.Join(err, table.Close())
			}
			sparse.swapStorage(aux)
		}
		closeAux := func() error {
			if aux == nil {
				return nil
			}
			return aux.Close()
		}
		if err := warnLargeTotalTable(i.signature, i.predicateName, i.hasPredicate); err != nil {
			return false, errors.Join(err, closeAux(), table.Close())
		}
		if err := fillTotalTable(table, i.signature); err != nil {
			return false, errors.Join(err, closeAux(), table.Close())
		}
		if err := closeAux(); err != nil {
			return false, errors.Join(err, table.Close())
		}
		if err := context.SetFiniteRegister(i.target, table); err != nil {
			return false, i.disposeUninstalledFinite(table, err)
		}
		return true, nil
	case "assert_signature":
		table, err := context.FiniteRegister(i.target)
		if err != nil || table == nil {
			return err == nil, err
		}
		actual, err := table.Signature()
		if err != nil {
			return false, err
		}
		if !actual.equal(i.signature) {
			return false, fmt.Errorf("%w: register signature assertion failed", ErrInstruction)
		}
		return true, nil
	default:
		return false, ErrInstruction
	}
}

func (i *constructInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	if _, found, err := context.RegisterAnnotation(i.target); err != nil || found {
		return err
	}
	return context.SetRegisterAnnotation(i.target, i.kind)
}

// SetDisplaySorts records the relation sorts that display_head_impl prints
// for mk_unary_singleton and mk_total.
func (i *constructInstruction) SetDisplaySorts(manager *ast.ASTManager, sorts []ast.ASTHandle) {
	i.displayManager, i.displaySorts = manager, append([]ast.ASTHandle(nil), sorts...)
}

// sortText is relation_manager::to_nice_string(sort): mk_pp(s).
func (i *constructInstruction) sortText(index int) (string, error) {
	if i.displayManager == nil || index >= len(i.displaySorts) {
		return fmt.Sprint(i.signature.domains[index]), nil
	}
	return i.displayManager.SMT2PPSortString(i.displaySorts[index])
}

func (i *constructInstruction) DisplayHead(context *ExecutionContext, out io.Writer) error {
	if err := i.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	switch i.kind {
	case "mk_unary_singleton":
		sort, err := i.sortText(0)
		if err != nil {
			return err
		}
		var element ast.ASTHandle
		if len(i.displaySorts) != 0 {
			element = i.displaySorts[0]
		}
		value, err := niceElementText(context, element, i.value)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "mk_unary_singleton into %d sort:%s val:%s", i.target, sort, value)
		return err
	case "mk_total":
		// to_nice_string(m_sig) << " " << m_pred->get_name().
		parts := make([]string, len(i.signature.domains))
		for index := range parts {
			text, err := i.sortText(index)
			if err != nil {
				return err
			}
			parts[index] = text
		}
		_, err := fmt.Fprintf(out, "mk_total into %d sort:[%s] %s", i.target, strings.Join(parts, ","), i.predicateName)
		return err
	case "assert_signature":
		_, err := fmt.Fprintf(out, "assert_signature of %d signature:%v", i.target, i.signature.domains)
		return err
	default:
		return ErrInstruction
	}
}

func (i *constructInstruction) Accounting() *base.AccountedObject { return i.accounting }

func (i *constructInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	return i.accounting.ProcessCosts()
}

func (i *constructInstruction) CollectStatistics(out *statistics.Statistics) error {
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

func (i *constructInstruction) Close() error {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	i.closing = true
	if err := i.clearPendingFinite(); err != nil {
		return err
	}
	if i.finiteSort != (ast.ASTRef{}) {
		if err := i.finiteManager.DelRef(&i.finiteSort); err != nil {
			return err
		}
	}
	if i.astValueRoot != (ast.ASTRef{}) {
		if err := i.finiteManager.DelRef(&i.astValueRoot); err != nil {
			return err
		}
	}
	if err := i.accounting.Close(); err != nil {
		return err
	}
	i.closed = true
	return nil
}
