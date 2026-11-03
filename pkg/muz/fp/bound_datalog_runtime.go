package fp

// Source: dl_bound_relation.cpp bound_relation::add_fact and rel_context's
// imprecise relation query status. Positive Horn rules are lowered by the
// companion bound_datalog_positive implementation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

var ErrBoundDatalogFragment = errors.New("muz/fp: unsupported arithmetic bound Datalog fragment")

type boundDatalogEntry struct {
	sorts []ast.ASTHandle
	seed  *rel.BoundRelation
	root  ast.ASTRef
}

// boundDatalogRuntime owns declaration pins and seed facts, and publishes a
// separate saturated snapshot only after the complete supported program succeeds.
type boundDatalogRuntime struct {
	manager           *ast.ASTManager
	rules             *base.RuleSet
	entries           map[ast.ASTHandle]*boundDatalogEntry
	order             []ast.ASTHandle
	evaluated         map[ast.ASTHandle]*rel.BoundRelation
	similarity        *rel.BoundSimilarityProgram
	similarityRuntime *boundDatalogRuntime
	similarityCleanup memory.ConstructionFailure
	pending           []ast.ASTRef
	closed            bool
}

type boundDatalogDeclaration struct {
	declaration ast.ASTHandle
	sorts       []ast.ASTHandle
}

// boundDatalogCleanupReceipt preserves a partially constructed runtime when
// native AST cleanup refuses. Cleanup may be retried without recovering an
// otherwise unreachable owner from a joined error.
type boundDatalogCleanupReceipt struct {
	original error
	cleanup  error
	owner    *memory.Runtime
	runtime  *boundDatalogRuntime
}

func (e *boundDatalogCleanupReceipt) Error() string {
	return errors.Join(e.original, e.cleanup).Error()
}
func (e *boundDatalogCleanupReceipt) Unwrap() error                 { return errors.Join(e.original, e.cleanup) }
func (e *boundDatalogCleanupReceipt) RuntimeOwner() *memory.Runtime { return e.owner }
func (e *boundDatalogCleanupReceipt) OriginalCause() error          { return e.original }
func (e *boundDatalogCleanupReceipt) CleanupCause() error           { return e.cleanup }
func (e *boundDatalogCleanupReceipt) Cleanup() error {
	if e == nil || e.runtime == nil {
		return nil
	}
	e.cleanup = e.runtime.close()
	if e.cleanup == nil {
		e.runtime = nil
	}
	return e.cleanup
}

// boundDatalogDeclarations validates the complete declaration list without
// acquiring declaration roots or changing an existing runtime. This is the
// source plugin's signature gate, including its vacuously supported nullary
// relations.
func boundDatalogDeclarations(manager *ast.ASTManager, declarations []ast.ASTHandle) ([]boundDatalogDeclaration, bool, error) {
	if manager == nil || len(declarations) == 0 {
		return nil, false, nil
	}
	plugin, err := rel.NewBoundRelationPlugin(manager)
	if err != nil {
		return nil, false, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, false, err
	}
	seen := make(map[ast.ASTHandle]bool, len(declarations))
	validated := make([]boundDatalogDeclaration, 0, len(declarations))
	for _, declaration := range declarations {
		kind, err := manager.GetKind(declaration)
		if err != nil {
			return nil, false, err
		}
		if kind != ast.ASTFuncDecl || seen[declaration] {
			return nil, false, ErrBoundDatalogFragment
		}
		seen[declaration] = true
		rangeSort, err := manager.GetRange(declaration)
		if err != nil {
			return nil, false, err
		}
		if rangeSort != boolean {
			return nil, false, nil
		}
		arity, err := manager.GetArity(declaration)
		if err != nil {
			return nil, false, err
		}
		sorts := make([]ast.ASTHandle, arity)
		for i := uint32(0); i < arity; i++ {
			sorts[i], err = manager.GetDomain(declaration, i)
			if err != nil {
				return nil, false, err
			}
		}
		capable, err := plugin.CanHandleSignature(sorts)
		if err != nil {
			return nil, false, err
		}
		if !capable {
			return nil, false, nil
		}
		validated = append(validated, boundDatalogDeclaration{declaration: declaration, sorts: sorts})
	}
	return validated, true, nil
}

// newBoundDatalogActivation distinguishes an unsupported signature from an
// error after the bound backend has been selected.
func newBoundDatalogActivation(manager *ast.ASTManager, rules *base.RuleSet, declarations []ast.ASTHandle) (*boundDatalogRuntime, bool, error) {
	_, selected, err := boundDatalogDeclarations(manager, declarations)
	if err != nil || !selected {
		return nil, selected, err
	}
	runtime, err := newBoundDatalogRuntime(manager, rules, declarations)
	return runtime, true, err
}

func newBoundDatalogRuntime(manager *ast.ASTManager, rules *base.RuleSet, declarations []ast.ASTHandle) (_ *boundDatalogRuntime, resultErr error) {
	if rules == nil {
		return nil, ErrBoundDatalogFragment
	}
	validated, selected, err := boundDatalogDeclarations(manager, declarations)
	if err != nil {
		return nil, err
	}
	if !selected {
		return nil, ErrBoundDatalogFragment
	}
	r := &boundDatalogRuntime{manager: manager, rules: rules, entries: make(map[ast.ASTHandle]*boundDatalogEntry, len(declarations))}
	defer func() {
		if resultErr != nil {
			if cleanup := r.close(); cleanup != nil {
				receipt := &boundDatalogCleanupReceipt{original: resultErr, cleanup: cleanup, runtime: r}
				if service := r.manager.NumeralService(); service != nil {
					receipt.owner = service.RuntimeOwner()
				}
				var failure memory.ConstructionFailure
				if errors.As(resultErr, &failure) || errors.As(cleanup, &failure) {
					if owner := failure.RuntimeOwner(); owner != nil {
						receipt.owner = owner
					}
				}
				resultErr = receipt
			}
		}
	}()
	for _, item := range validated {
		declaration, sorts := item.declaration, item.sorts
		root, err := manager.NewRef(declaration)
		if err != nil {
			return nil, err
		}
		r.entries[declaration] = &boundDatalogEntry{sorts: sorts, seed: rel.NewBoundRelation(uint32(len(sorts)), true), root: root}
		r.order = append(r.order, declaration)
	}
	return r, nil
}

// syncPredicates monotonically registers late predicates and synthetic
// nullary query heads. The complete list is checked before roots, seeds, or a
// published result are changed.
func (r *boundDatalogRuntime) syncPredicates(declarations []ast.ASTHandle) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.retirePending(); err != nil {
		return err
	}
	validated, selected, err := boundDatalogDeclarations(r.manager, declarations)
	if err != nil {
		return err
	}
	if !selected {
		return ErrBoundDatalogFragment
	}
	current := make(map[ast.ASTHandle]bool, len(validated))
	for _, item := range validated {
		current[item.declaration] = true
	}
	for _, declaration := range r.order {
		if !current[declaration] {
			return ErrBoundDatalogFragment
		}
	}
	additions := make([]boundDatalogDeclaration, 0, len(validated)-len(r.entries))
	for _, item := range validated {
		if r.entries[item.declaration] == nil {
			additions = append(additions, item)
		}
	}
	roots := make([]ast.ASTRef, 0, len(additions))
	defer func() {
		if resultErr == nil {
			return
		}
		r.pending = append(r.pending, roots...)
		resultErr = errors.Join(resultErr, r.retirePending())
	}()
	for _, item := range additions {
		root, err := r.manager.NewRef(item.declaration)
		if err != nil {
			return err
		}
		roots = append(roots, root)
	}
	for i, item := range additions {
		r.entries[item.declaration] = &boundDatalogEntry{
			sorts: item.sorts,
			seed:  rel.NewBoundRelation(uint32(len(item.sorts)), true),
			root:  roots[i],
		}
		r.order = append(r.order, item.declaration)
	}
	roots = nil
	if len(additions) != 0 {
		r.evaluated = nil
	}
	return nil
}

// clone copies seed abstractions while borrowing a destination clone's rule
// set. Published saturation state is deliberately not shared.
func (r *boundDatalogRuntime) clone(rules *base.RuleSet) (_ *boundDatalogRuntime, resultErr error) {
	if err := r.open(); err != nil || rules == nil {
		if err == nil {
			err = ErrBoundDatalogFragment
		}
		return nil, err
	}
	copyRuntime, err := newBoundDatalogRuntime(r.manager, rules, r.order)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, copyRuntime.close())
		}
	}()
	for declaration, entry := range r.entries {
		copyRuntime.entries[declaration].seed = entry.seed.Clone()
	}
	return copyRuntime, nil
}

func (r *boundDatalogRuntime) open() error {
	if r == nil || r.closed || r.manager == nil || r.rules == nil || r.entries == nil {
		return ErrBoundDatalogFragment
	}
	return nil
}

// retirePending keeps every staged declaration owner attached to the runtime
// until native deletion succeeds, so a refused rollback remains retryable.
func (r *boundDatalogRuntime) retirePending() (result error) {
	if r == nil || r.manager == nil {
		return ErrBoundDatalogFragment
	}
	for i := len(r.pending) - 1; i >= 0; i-- {
		if r.pending[i] != (ast.ASTRef{}) {
			result = errors.Join(result, r.manager.DelRef(&r.pending[i]))
		}
	}
	kept := r.pending[:0]
	for _, root := range r.pending {
		if root != (ast.ASTRef{}) {
			kept = append(kept, root)
		}
	}
	r.pending = kept
	return result
}

func (r *boundDatalogRuntime) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	entry := r.entries[declaration]
	if entry == nil || len(values) != len(entry.sorts) {
		return ErrBoundDatalogFragment
	}
	for i, value := range values {
		sort, err := r.manager.GetSort(value)
		if err != nil {
			return err
		}
		if sort != entry.sorts[i] {
			return ast.ErrASTDomain
		}
	}
	if err := entry.seed.AddFact(values); err != nil {
		return err
	}
	r.evaluated = nil
	return nil
}

func (r *boundDatalogRuntime) query(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := r.open(); err != nil {
		return lbool.Undef, err
	}
	r.evaluated = nil
	if err := r.discardSimilarity(); err != nil {
		return lbool.Undef, err
	}
	if err := r.saturatePositive(true); err != nil {
		return lbool.Undef, err
	}
	if len(declarations) == 0 {
		return lbool.True, nil
	}
	for _, declaration := range declarations {
		if r.evaluated[declaration] == nil {
			return lbool.Undef, fmt.Errorf("%w: undeclared query predicate", ErrBoundDatalogFragment)
		}
	}
	for _, declaration := range declarations {
		if !r.evaluated[declaration].Empty() {
			return lbool.Undef, nil
		}
	}
	return lbool.False, nil
}

func (r *boundDatalogRuntime) relation(declaration ast.ASTHandle) (*rel.BoundRelation, error) {
	if err := r.open(); err != nil || r.evaluated == nil || r.evaluated[declaration] == nil {
		if err == nil {
			err = ErrBoundDatalogFragment
		}
		return nil, err
	}
	return r.evaluated[declaration].Clone(), nil
}

func (r *boundDatalogRuntime) relationFormula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := r.open(); err != nil || r.evaluated == nil || r.evaluated[declaration] == nil {
		if err == nil {
			err = ErrBoundDatalogFragment
		}
		return ast.ASTRef{}, err
	}
	return r.evaluated[declaration].ToFormula(r.manager, r.entries[declaration].sorts)
}

func (r *boundDatalogRuntime) reset() error {
	if err := r.open(); err != nil {
		return err
	}
	r.evaluated = nil
	if err := r.discardSimilarity(); err != nil {
		return err
	}
	for _, entry := range r.entries {
		entry.seed = rel.NewBoundRelation(uint32(len(entry.sorts)), true)
	}
	r.evaluated = nil
	return nil
}

func (r *boundDatalogRuntime) close() (result error) {
	if r == nil || r.closed {
		return nil
	}
	if err := r.discardSimilarity(); err != nil {
		return err
	}
	result = errors.Join(result, r.retirePending())
	for i := len(r.order) - 1; i >= 0; i-- {
		entry := r.entries[r.order[i]]
		if entry != nil && entry.root != (ast.ASTRef{}) {
			result = errors.Join(result, r.manager.DelRef(&entry.root))
		}
	}
	if result != nil {
		return result
	}
	r.closed = true
	r.manager, r.rules, r.entries, r.order, r.evaluated, r.pending = nil, nil, nil, nil, nil, nil
	return nil
}
