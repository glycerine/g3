package rel

// Source: src/muz/rel/dl_check_table.cpp query-reached check_table operations.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// FiniteCheckedPair owns independently instantiated concrete primary and
// checker tables and compares their observable results after every operation.
type FiniteCheckedPair struct {
	primary FiniteTable
	checker FiniteTable
	closed  bool
	// plugin is the check_table_plugin the table belongs to (nil for the
	// checked scope's pairs).
	plugin *FiniteCheckedFactory
	// deferredMkEmpty: made for a predicate whose relation the source has
	// not created yet; check_table_plugin::mk_empty's verbose line comes
	// when get_relation creates it (completeCreation).
	deferredMkEmpty bool
}

// completeCreation prints the deferred mk_empty line.
func (p *FiniteCheckedPair) completeCreation() error {
	if !p.deferredMkEmpty {
		return nil
	}
	p.deferredMkEmpty = false
	return checkTableVerbose("mk_empty")
}

// FiniteCheckedFactory is the concrete check_table plugin selection used for
// compiler temporaries during one configured checked execution.
type FiniteCheckedFactory struct {
	primaryKind string
	checkerKind string
	lazy        *LazySparseFiniteFactory
	// relationFavourite: the second check_table plugin, whose
	// table_relation plugin is the favourite relation plugin.
	relationFavourite bool
	// manager is the relation manager the plugin is registered with: its
	// sparse_table_plugin makes the sparse sides, its appropriate plugin
	// the sides a wrapped plugin cannot handle.
	manager *RelationManager
}

// NewFiniteCheckedFactory is a check_table plugin of the manager m: its
// sparse tables are m's sparse_table_plugin's.
func (m *RelationManager) NewFiniteCheckedFactory(primaryKind, checkerKind string, lazy *LazySparseFiniteFactory) (*FiniteCheckedFactory, error) {
	f, err := NewFiniteCheckedFactoryWithLazy(primaryKind, checkerKind, lazy)
	if err != nil {
		return nil, err
	}
	f.manager = m
	return f, nil
}

func NewFiniteCheckedFactory(primaryKind, checkerKind string) (*FiniteCheckedFactory, error) {
	return NewFiniteCheckedFactoryWithLazy(primaryKind, checkerKind, nil)
}

// NewFiniteCheckedFactoryWithLazy binds the context-owned lazy_sparse plugin
// used by either concrete side. The factory borrows lazy and never closes it.
func NewFiniteCheckedFactoryWithLazy(primaryKind, checkerKind string, lazy *LazySparseFiniteFactory) (*FiniteCheckedFactory, error) {
	valid := func(kind string) bool {
		switch kind {
		case "hashtable", "tr_hashtable", "sparse", "tr_sparse", "bitvector", "tr_bitvector":
			return true
		case "lazy_sparse", "tr_lazy_sparse":
			return lazy != nil && lazy.open() == nil
		default:
			return false
		}
	}
	if !valid(primaryKind) || !valid(checkerKind) {
		return nil, ErrCheckTable
	}
	return &FiniteCheckedFactory{primaryKind: primaryKind, checkerKind: checkerKind, lazy: lazy}, nil
}

func checkedKindHandles(kind string, signature Signature) bool {
	return checkedKindHandlesFunctional(kind, signature, 0)
}

// checkedKindHandlesFunctional is the wrapped table plugin's
// can_handle_signature of a signature with functional columns: hashtable
// and bitvector (table_plugin's default) reject functional columns, sparse
// and lazy_sparse the empty signature.
func checkedKindHandlesFunctional(kind string, signature Signature, functional int) bool {
	switch kind {
	case "hashtable", "tr_hashtable":
		return functional == 0
	case "sparse", "tr_sparse", "lazy_sparse", "tr_lazy_sparse":
		return signature.Len() != 0
	case "bitvector", "tr_bitvector":
		return BitvectorCanHandleSignature(signature, functional)
	default:
		return false
	}
}

// CanHandleSignatureFunctional is check_table_plugin::can_handle_signature
// of a signature with functional columns.
func (f *FiniteCheckedFactory) CanHandleSignatureFunctional(signature Signature, functional int) bool {
	return f != nil && checkedKindHandlesFunctional(f.primaryKind, signature, functional) && checkedKindHandlesFunctional(f.checkerKind, signature, functional)
}

// CanHandleSignature mirrors check_table_plugin::can_handle_signature: both
// selected concrete plugins must accept the signature.
func (f *FiniteCheckedFactory) CanHandleSignature(signature Signature) bool {
	return f != nil && checkedKindHandles(f.primaryKind, signature) && checkedKindHandles(f.checkerKind, signature)
}

// AllowsUncheckedNullaryFallback reports whether source plugin selection must
// bypass this checked favourite for the empty signature.
func (f *FiniteCheckedFactory) AllowsUncheckedNullaryFallback() bool {
	if f == nil {
		return false
	}
	signature, err := NewSignature(nil)
	return err == nil && !f.CanHandleSignature(signature)
}

func (f *FiniteCheckedFactory) table(kind string, signature Signature, functional int) (FiniteTable, error) {
	if !checkedKindHandles(kind, signature) {
		return nil, ErrCheckTable
	}
	switch kind {
	case "hashtable", "tr_hashtable":
		return NewHashTable(signature), nil
	case "sparse", "tr_sparse":
		return f.manager.sparsePlugin().MkEmpty(signature, functional)
	case "bitvector", "tr_bitvector":
		if functional != 0 {
			return nil, ErrCheckTable
		}
		// The manager's bitvector_table_plugin: a result it cannot
		// handle goes to the manager's sparse_table_plugin.
		return newBitvectorTableIn(f.manager.sparsePlugin(), signature)
	case "lazy_sparse", "tr_lazy_sparse":
		if f.lazy == nil {
			return nil, ErrCheckTable
		}
		if functional == 0 {
			return f.lazy.New(signature)
		}
		return f.lazy.NewFunctional(signature, functional)
	default:
		return nil, fmt.Errorf("%w: finite checker plugin %s", ErrTable, kind)
	}
}

func (f *FiniteCheckedFactory) New(signature Signature) (*FiniteCheckedPair, error) {
	return f.NewFunctional(signature, 0)
}

// NewFunctional is check_table_plugin::mk_empty(s) of a signature with
// functional columns.
func (f *FiniteCheckedFactory) NewFunctional(signature Signature, functional int) (*FiniteCheckedPair, error) {
	if f.manager != nil && (f.manager.reservingPlaceholder || f.manager.deferRelationCreation) {
		pair, err := f.newPair(signature, functional)
		if err != nil {
			return nil, err
		}
		pair.deferredMkEmpty = true
		return pair, nil
	}
	if err := checkTableVerbose("mk_empty"); err != nil {
		return nil, err
	}
	return f.newPair(signature, functional)
}

// newPair is check_table_plugin::mk_empty without its verbose line: the
// checker's table, then the checked plugin's.
func (f *FiniteCheckedFactory) newPair(signature Signature, functional int) (*FiniteCheckedPair, error) {
	if !f.CanHandleSignatureFunctional(signature, functional) {
		return nil, ErrCheckTable
	}
	checker, err := f.table(f.checkerKind, signature, functional)
	if err != nil {
		return nil, err
	}
	primary, err := f.table(f.primaryKind, signature, functional)
	if err != nil {
		return nil, errors.Join(err, checker.Close())
	}
	pair, err := adoptFiniteCheckedPair(primary, checker)
	if err != nil {
		return nil, err
	}
	pair.plugin = f
	return pair, nil
}

// sameKind is check_kind of two check tables: tables of the same
// check_table plugin (the checked scope's pairs have none).
func (p *FiniteCheckedPair) sameKind(o *FiniteCheckedPair) bool {
	return p.plugin == o.plugin
}

// RemoveFact is check_table::remove_fact.
func (p *FiniteCheckedPair) RemoveFact(fact Fact) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := checkTableVerbose("remove_fact"); err != nil {
		return err
	}
	if err := checkTableRemoveFact(p.primary, fact); err != nil {
		return err
	}
	if err := checkTableRemoveFact(p.checker, fact); err != nil {
		return err
	}
	return p.wellFormed()
}

// adopt is alloc(check_table, get(t).get_plugin(), ..., tocheck, checker)
// of an operation on p.
func (p *FiniteCheckedPair) adopt(primary, checker FiniteTable) (*FiniteCheckedPair, error) {
	result, err := adoptFiniteCheckedPair(primary, checker)
	if err != nil {
		return nil, err
	}
	result.plugin = p.plugin
	return result, nil
}

func adoptFiniteCheckedPair(primary, checker FiniteTable) (_ *FiniteCheckedPair, resultErr error) {
	if primary == nil || checker == nil {
		return nil, ErrCheckTable
	}
	result := &FiniteCheckedPair{primary: primary, checker: checker}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	primarySignature, err := primary.Signature()
	if err != nil {
		return nil, err
	}
	checkerSignature, err := checker.Signature()
	if err != nil || !primarySignature.equal(checkerSignature) {
		return nil, errors.Join(err, ErrCheckTable)
	}
	if err = result.wellFormed(); err != nil {
		return nil, err
	}
	return result, nil
}

func newFiniteTableLike(template FiniteTable, signature Signature) (FiniteTable, error) {
	switch concrete := template.(type) {
	case *HashTable:
		return NewHashTable(signature), nil
	case *SparseTable:
		if signature.Len() == 0 {
			return NewHashTable(signature), nil
		}
		// The template's plugin's mk_empty.
		return concrete.plugin.MkEmpty(signature, 0)
	case *LazySparseTable:
		return lazySparseInstructionResult(concrete, signature, 0)
	case *BitvectorTable:
		if BitvectorCanHandleSignature(signature, 0) {
			return concrete.emptyLike(signature)
		}
		// get_appropriate_plugin: sparse for every nonnullary signature.
		return concrete.sparse.MkEmpty(signature, 0)
	default:
		return nil, fmt.Errorf("%w: unsupported checked component %T", ErrCheckTable, template)
	}
}

func (p *FiniteCheckedPair) open() error {
	if p == nil || p.closed || p.primary == nil || p.checker == nil {
		return ErrCheckTable
	}
	return nil
}

func (p *FiniteCheckedPair) verify(objective string) error {
	if err := p.open(); err != nil {
		return err
	}
	a, err := p.primary.Facts()
	if err != nil {
		return err
	}
	b, err := p.checker.Facts()
	if err != nil {
		return err
	}
	if len(a) != len(b) {
		return fmt.Errorf("%w: %s row counts differ", ErrCheckTable, objective)
	}
	for _, fact := range a {
		found, err := p.checker.Contains(fact)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: %s checker misses %v", ErrCheckTable, objective, fact)
		}
	}
	for _, fact := range b {
		found, err := p.primary.Contains(fact)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: %s primary misses %v", ErrCheckTable, objective, fact)
		}
	}
	return nil
}

// wellFormed is check_table::well_formed: every row of the checked table
// is contained in the checker (contains_fact, timed by sparse_table's
// verbose_action("contains_fact", 2)), then every row of the checker in
// the checked table.
func (p *FiniteCheckedPair) wellFormed() error {
	if err := p.open(); err != nil {
		return err
	}
	a, err := p.primary.Facts()
	if err != nil {
		return err
	}
	for _, fact := range a {
		found, err := checkTableContainsFact(p.checker, fact)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("%w: checker misses %v", ErrCheckTable, fact)
		}
	}
	b, err := p.checker.Facts()
	if err != nil {
		return err
	}
	for _, fact := range b {
		found, err := checkTableContainsFact(p.primary, fact)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("%w: checked table misses %v", ErrCheckTable, fact)
		}
	}
	return nil
}

// checkTableContainsFact is table_base::contains_fact of a check table's
// component: sparse_table's (verbose), a lazy table's evaluated table's,
// or the table's own.
func checkTableContainsFact(t FiniteTable, fact Fact) (bool, error) {
	return tableContainsFact(t, fact)
}

// tableContainsFact is the virtual table_base::contains_fact: sparse_table's
// (timed by verbose_action("contains_fact", 2)), lazy_table's (its
// evaluated table's), check_table's (the checker's), else the table's own.
func tableContainsFact(t FiniteTable, fact Fact) (bool, error) {
	switch concrete := t.(type) {
	case *FiniteCheckedPair:
		if concrete.plugin != nil {
			if err := concrete.open(); err != nil {
				return false, err
			}
			return tableContainsFact(concrete.checker, fact)
		}
	case *SparseTable:
		return concrete.ContainsFact(fact)
	case *LazySparseTable:
		if concrete.root != nil && concrete.root.signature.Len() != 0 {
			table, err := concrete.eval()
			if err != nil {
				return false, err
			}
			return table.ContainsFact(fact)
		}
	}
	return t.Contains(fact)
}

func (p *FiniteCheckedPair) Signature() (Signature, error) {
	if err := p.open(); err != nil {
		return Signature{}, err
	}
	return p.primary.Signature()
}

// Facts iterates check_table::begin, the checked table's rows.
func (p *FiniteCheckedPair) Facts() ([]Fact, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return p.primary.Facts()
}
func (p *FiniteCheckedPair) Add(fact Fact) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if err := checkTableVerbose("add_fact"); err != nil {
		return false, err
	}
	a, err := p.primary.Add(fact)
	if err != nil {
		return false, err
	}
	b, err := p.checker.Add(fact)
	if err != nil {
		return false, err
	}
	if a != b {
		return false, fmt.Errorf("%w: insertion results differ", ErrCheckTable)
	}
	return a, p.wellFormed()
}
func (p *FiniteCheckedPair) Contains(fact Fact) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	a, err := p.primary.Contains(fact)
	if err != nil {
		return false, err
	}
	b, err := p.checker.Contains(fact)
	if err != nil {
		return false, err
	}
	if a != b {
		return false, fmt.Errorf("%w: membership results differ", ErrCheckTable)
	}
	return a, nil
}

// Len counts the checked table's rows; like check_table::empty it
// evaluates the checked table, then the checker, and fails when one is
// empty and the other is not.
func (p *FiniteCheckedPair) Len() (int, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	a, err := p.primary.Len()
	if err != nil {
		return 0, err
	}
	b, err := p.checker.Len()
	if err != nil {
		return 0, err
	}
	if (a == 0) != (b == 0) {
		return 0, fmt.Errorf("%w: emptiness differs", ErrCheckTable)
	}
	return a, nil
}
func (p *FiniteCheckedPair) SizeEstimateRows() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	// Source check_table forwards metadata without iterating either side.
	// In particular, lazy_table's estimate remains 1 after materialization.
	switch primary := p.primary.(type) {
	case *HashTable:
		return primary.SizeEstimateRows()
	case *BitvectorTable:
		return primary.SizeEstimateRows()
	case *SparseTable:
		return primary.SizeEstimateRows()
	case *LazySparseTable:
		return primary.SizeEstimateRows()
	default:
		return 0, ErrCheckTable
	}
}
func (p *FiniteCheckedPair) SizeEstimateBytes() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	switch primary := p.primary.(type) {
	case *HashTable:
		return primary.SizeEstimateBytes()
	case *BitvectorTable:
		return primary.SizeEstimateBytes()
	case *SparseTable:
		return primary.SizeEstimateBytes()
	case *LazySparseTable:
		return primary.SizeEstimateBytes()
	default:
		return 0, ErrCheckTable
	}
}
func (p *FiniteCheckedPair) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	if p.plugin != nil {
		// table_base::reset: remove_facts of the rows, check_table::
		// remove_fact of each.
		facts, err := p.primary.Facts()
		if err != nil {
			return err
		}
		for _, fact := range facts {
			if err = p.RemoveFact(fact); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.Join(p.primary.Reset(), p.checker.Reset())
}

// EmptyLike creates empty primary and checker instances of the same concrete
// kinds. It is the checked equivalent of table_plugin::mk_empty.
func (p *FiniteCheckedPair) EmptyLike() (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if err := checkTableVerbose("mk_empty"); err != nil {
		return nil, err
	}
	primary, err := emptyFiniteLike(p.primary)
	if err != nil {
		return nil, err
	}
	checker, err := emptyFiniteLike(p.checker)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return p.adopt(primary, checker)
}

// Complement implements the live finite complement boundary used by checked
// full construction. Wider complements remain compiler-decomposed.
func (p *FiniteCheckedPair) Complement() (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	signature, err := p.Signature()
	if err != nil {
		return nil, err
	}
	if signature.Len() > 1 {
		return nil, fmt.Errorf("%w: complement requires compiler decomposition", ErrCheckTable)
	}
	result, err := p.EmptyLike()
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			_ = result.Close()
		}
	}()
	if signature.Len() == 0 {
		found, err := p.Contains(Fact{})
		if err != nil {
			return nil, err
		}
		if !found {
			if _, err = result.Add(Fact{}); err != nil {
				return nil, err
			}
		}
	} else {
		domain, err := signature.Domain(0)
		if err != nil {
			return nil, err
		}
		for value := uint64(0); value < domain; value++ {
			fact := Fact{Element(value)}
			found, err := p.Contains(fact)
			if err != nil {
				return nil, err
			}
			if !found {
				if _, err = result.Add(fact); err != nil {
					return nil, err
				}
			}
		}
	}
	failed = false
	return result, nil
}

// FullLike creates a checked full relation of the same concrete pair kinds.
func (p *FiniteCheckedPair) FullLike() (*FiniteCheckedPair, error) {
	empty, err := p.EmptyLike()
	if err != nil {
		return nil, err
	}
	full, fullErr := empty.Complement()
	closeErr := empty.Close()
	if fullErr != nil || closeErr != nil {
		if full != nil {
			closeErr = errors.Join(closeErr, full.Close())
		}
		return nil, errors.Join(fullErr, closeErr)
	}
	return full, nil
}

func (p *FiniteCheckedPair) Join(other *FiniteCheckedPair, left, right []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil || other == nil || other.open() != nil {
		return nil, ErrCheckTable
	}
	if err := checkTableVerbose("operator()"); err != nil {
		return nil, err
	}
	primary, err := finiteTableJoin(p.primary, other.primary, left, right)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableJoin(p.checker, other.checker, left, right)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return p.adopt(primary, checker)
}

func (p *FiniteCheckedPair) JoinProject(other *FiniteCheckedPair, left, right, removed []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil || other == nil || other.open() != nil {
		return nil, ErrCheckTable
	}
	// join_project_fn: the manager's mk_join_project_fn of each side.
	var m *RelationManager
	if p.plugin != nil {
		m = p.plugin.manager
	}
	primary, err := finiteTableJoinProjectIn(m, p.primary, other.primary, left, right, removed)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableJoinProjectIn(m, p.checker, other.checker, left, right, removed)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return p.adopt(primary, checker)
}

func (p *FiniteCheckedPair) Project(removed []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	// project_fn: the manager's mk_project_fn of each side.
	var m *RelationManager
	if p.plugin != nil {
		m = p.plugin.manager
	}
	checker, err := finiteTableProjectIn(m, p.checker, removed)
	if err != nil {
		return nil, err
	}
	primary, err := finiteTableProjectIn(m, p.primary, removed)
	if err != nil {
		return nil, errors.Join(err, checker.Close())
	}
	return p.adopt(primary, checker)
}

func (p *FiniteCheckedPair) RenameCycle(cycle []int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if err := checkTableVerbose("operator()"); err != nil {
		return nil, err
	}
	checker, err := finiteTableRename(p.checker, cycle)
	if err != nil {
		return nil, err
	}
	primary, err := finiteTableRename(p.primary, cycle)
	if err != nil {
		return nil, errors.Join(err, checker.Close())
	}
	return p.adopt(primary, checker)
}

func (p *FiniteCheckedPair) SelectEqualAndProject(value Element, column int) (*FiniteCheckedPair, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	checker, err := finiteTableSelectProject(p.checker, value, column)
	if err != nil {
		return nil, err
	}
	primary, err := finiteTableSelectProject(p.primary, value, column)
	if err != nil {
		return nil, errors.Join(err, checker.Close())
	}
	return p.adopt(primary, checker)
}

func (p *FiniteCheckedPair) FilterEqual(value Element, column int) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := finiteTableFilterEqual(p.checker, value, column); err != nil {
		return err
	}
	if err := finiteTableFilterEqual(p.primary, value, column); err != nil {
		return err
	}
	return p.wellFormed()
}

func (p *FiniteCheckedPair) FilterIdentical(columns []int) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := finiteTableFilterIdentical(p.checker, columns); err != nil {
		return err
	}
	if err := finiteTableFilterIdentical(p.primary, columns); err != nil {
		return err
	}
	return p.wellFormed()
}

func (p *FiniteCheckedPair) FilterByNegation(negative *FiniteCheckedPair, targetColumns, negativeColumns []int) error {
	if err := p.open(); err != nil || negative == nil || negative.open() != nil {
		return ErrCheckTable
	}
	if err := checkTableVerbose("operator()"); err != nil {
		return err
	}
	if err := finiteTableFilterNegation(p.checker, negative.checker, targetColumns, negativeColumns); err != nil {
		return err
	}
	if err := finiteTableFilterNegation(p.primary, negative.primary, targetColumns, negativeColumns); err != nil {
		return err
	}
	return p.wellFormed()
}

func filterCheckedPairInterpreted(table FiniteTable, manager *RelationManager, relationSorts []ast.ASTHandle, theory *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	switch concrete := table.(type) {
	case *HashTable:
		relation := &TableRelation{manager: manager, sorts: append([]ast.ASTHandle(nil), relationSorts...), table: concrete}
		return relation.FilterInterpreted(theory, conditions, columns, sorts)
	case *SparseTable:
		return filterSparseInterpretedBound(concrete, manager, theory, conditions, columns, sorts)
	case *BitvectorTable:
		return filterBitvectorInterpreted(concrete, manager, theory, conditions, columns, sorts)
	case *LazySparseTable:
		return concrete.filterInterpretedCompiled(manager, theory, conditions, columns, sorts)
	default:
		return ErrCheckTable
	}
}

// FilterInterpreted executes the source checker first and selected primary
// second, then verifies the concrete results remain extensionally equal.
func (p *FiniteCheckedPair) FilterInterpreted(manager *RelationManager, relationSorts []ast.ASTHandle, theory *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := filterCheckedPairInterpreted(p.checker, manager, relationSorts, theory, conditions, columns, sorts); err != nil {
		return err
	}
	if err := filterCheckedPairInterpreted(p.primary, manager, relationSorts, theory, conditions, columns, sorts); err != nil {
		return err
	}
	return p.wellFormed()
}

// UnionFrom executes the reached check_table union semantics and accumulates
// only newly inserted rows in delta. Existing delta rows are preserved.
func (p *FiniteCheckedPair) UnionFrom(source, delta *FiniteCheckedPair) error {
	return p.unionFromConcrete(source, delta)
}

func (p *FiniteCheckedPair) CloneFinite() (FiniteTable, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if err := checkTableVerbose("clone"); err != nil {
		return nil, err
	}
	primary, err := p.primary.CloneFinite()
	if err != nil {
		return nil, err
	}
	checker, err := p.checker.CloneFinite()
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	result := &FiniteCheckedPair{primary: primary, checker: checker, plugin: p.plugin}
	if err = result.wellFormed(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}
func (p *FiniteCheckedPair) Close() error {
	if p == nil {
		return ErrCheckTable
	}
	if p.closed {
		return nil
	}
	// Source destruction releases the primary before the checker. A lazy
	// side may refuse AST-condition cleanup; keep that owner for retry.
	if p.primary != nil {
		if err := p.primary.Close(); err != nil {
			return err
		}
		p.primary = nil
	}
	if p.checker != nil {
		if err := p.checker.Close(); err != nil {
			return err
		}
		p.checker = nil
	}
	p.closed = true
	return nil
}

func (p *FiniteCheckedPair) ConcreteKinds() (primary, checker string, err error) {
	if err = p.open(); err != nil {
		return "", "", err
	}
	kind := func(table FiniteTable) string {
		switch table.(type) {
		case *HashTable:
			return "hashtable"
		case *SparseTable:
			return "sparse"
		case *BitvectorTable:
			return "bitvector"
		case *LazySparseTable:
			return "lazy_sparse"
		default:
			return fmt.Sprintf("%T", table)
		}
	}
	return kind(p.primary), kind(p.checker), nil
}

var _ FiniteTable = (*FiniteCheckedPair)(nil)

// CloneCheckTableForContext copies a check table into g3's query clone of a
// datalog context. The source's query runs on the same rel_context, so the
// copy is not a check_table::clone and prints nothing: its tables are
// copied by cloneTable (into the copy's own lazy engine), and it belongs to
// the corresponding check_table plugin of the copy's relation manager.
func CloneCheckTableForContext(p *FiniteCheckedPair, target *RelationManager, cloneTable func(FiniteTable) (FiniteTable, error)) (FiniteTable, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	plugin := p.plugin
	if plugin != nil && target != nil && target.defaultChecked != nil {
		if plugin.relationFavourite {
			plugin = target.defaultCheckedRelation
		} else {
			plugin = target.defaultChecked
		}
	}
	primary, err := cloneTable(p.primary)
	if err != nil {
		return nil, err
	}
	checker, err := cloneTable(p.checker)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return &FiniteCheckedPair{primary: primary, checker: checker, plugin: plugin, deferredMkEmpty: p.deferredMkEmpty}, nil
}

// checkTableRemoveFact is remove_fact of a check table's table
// (lazy_table::remove_fact: eval()->remove_fact).
func checkTableRemoveFact(t FiniteTable, fact Fact) error {
	if lazy, ok := t.(*LazySparseTable); ok {
		return lazy.RemoveFacts([]Fact{fact})
	}
	handled, err := RemoveFiniteTableFacts(t, []Fact{fact})
	if err == nil && !handled {
		err = ErrCheckTable
	}
	return err
}
