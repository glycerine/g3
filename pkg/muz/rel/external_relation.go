package rel

// Source: src/muz/rel/dl_external_relation.cpp/.h and the fixedpoint callback
// fallback in src/api/api_datalog.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// ExternalRelationCallbacks are the concrete fixedpoint reduction callbacks.
// Handles and slices are borrowed for the duration of each callback. Returning
// a null handle from Reduce requests the source's unreduced-application fallback.
type ExternalRelationCallbacks struct {
	Reduce       func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error)
	ReduceAssign func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error
}

// ExternalRelationPlugin owns the shared callback configuration for relations
// created by one source external relation plugin.
type ExternalRelationPlugin struct {
	manager   *RelationManager
	callbacks ExternalRelationCallbacks
}

func NewExternalRelationPlugin(manager *RelationManager, callbacks ExternalRelationCallbacks) (*ExternalRelationPlugin, error) {
	if manager == nil || manager.open() != nil {
		return nil, ErrTableRelation
	}
	return &ExternalRelationPlugin{manager: manager, callbacks: callbacks}, nil
}

// ExternalRelation owns its relation expression and cached accessor declarations.
// The relation manager and callback functions are borrowed.
type ExternalRelation struct {
	plugin                           *ExternalRelationPlugin
	manager                          *RelationManager
	ast                              *ast.ASTManager
	sorts                            []ast.ASTHandle
	callbacks                        ExternalRelationCallbacks
	relation                         ast.ASTRef
	selectDecl, storeDecl, emptyDecl ast.ASTRef
	closed                           bool
}

// NewExternalRelation creates a fresh external relation and invokes the empty
// relation assignment callback, if present.
func NewExternalRelation(manager *RelationManager, sorts []ast.ASTHandle, callbacks ExternalRelationCallbacks) (_ *ExternalRelation, resultErr error) {
	plugin, err := NewExternalRelationPlugin(manager, callbacks)
	if err != nil {
		return nil, err
	}
	return plugin.NewEmpty(sorts)
}

// NewEmpty creates a relation belonging to this callback plugin.
func (plugin *ExternalRelationPlugin) NewEmpty(sorts []ast.ASTHandle) (_ *ExternalRelation, resultErr error) {
	if plugin == nil || plugin.manager == nil || plugin.manager.open() != nil {
		return nil, ErrTableRelation
	}
	manager, callbacks := plugin.manager, plugin.callbacks
	m := manager.manager
	relationSort, err := m.MkDatalogRelationSort(sorts)
	if err != nil {
		return nil, err
	}
	sortRef, err := m.NewRef(relationSort)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&sortRef)) }()
	expression, err := m.MkFreshConst("T", relationSort)
	if err != nil {
		return nil, err
	}
	ref, err := m.NewRef(expression)
	if err != nil {
		return nil, err
	}
	r := &ExternalRelation{plugin: plugin, manager: manager, ast: m, sorts: append([]ast.ASTHandle(nil), sorts...), callbacks: callbacks, relation: ref}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, r.Close())
		}
	}()
	empty, err := m.MkDatalogEmpty(relationSort)
	if err != nil {
		return nil, err
	}
	emptyRef, err := m.NewRef(empty)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&emptyRef)) }()
	declaration, err := m.GetDecl(empty)
	if err != nil {
		return nil, err
	}
	if callbacks.ReduceAssign != nil {
		if err = callbacks.ReduceAssign(declaration, nil, []ast.ASTHandle{expression}); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *ExternalRelation) open() error {
	if r == nil || r.closed || r.ast == nil || r.manager == nil || r.manager.open() != nil || r.relation == (ast.ASTRef{}) {
		return ErrTableRelation
	}
	return nil
}

// Relation returns the borrowed external relation expression.
func (r *ExternalRelation) Relation() (ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return r.ast.RefGet(r.relation)
}

// ToFormula returns an owning reference to the external relation expression.
func (r *ExternalRelation) ToFormula() (ast.ASTRef, error) {
	expression, err := r.Relation()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return r.ast.NewRef(expression)
}

func (r *ExternalRelation) validateFact(values []ast.ASTHandle) error {
	if len(values) != len(r.sorts) {
		return ast.ErrASTDomain
	}
	for i, value := range values {
		sort, err := r.ast.GetSort(value)
		if err != nil {
			return err
		}
		if sort != r.sorts[i] {
			return ast.ErrASTDomain
		}
	}
	return nil
}

// accessor creates and retains the same per-relation declaration on first use.
func (r *ExternalRelation) accessor(cache *ast.ASTRef, op int32, args []ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	if *cache != (ast.ASTRef{}) {
		return r.ast.RefGet(*cache)
	}
	var app ast.ASTHandle
	var err error
	switch op {
	case ast.DatalogOpStore:
		app, err = r.ast.MkDatalogStore(args[0], args[1:]...)
	case ast.DatalogOpSelect:
		app, err = r.ast.MkDatalogSelect(args[0], args[1:]...)
	case ast.DatalogOpIsEmpty:
		app, err = r.ast.MkDatalogIsEmpty(args[0])
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	appRef, err := r.ast.NewRef(app)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&appRef)) }()
	declaration, err := r.ast.GetDecl(app)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*cache, err = r.ast.NewRef(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return declaration, nil
}

func (r *ExternalRelation) reduce(declaration ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
	if r.callbacks.Reduce != nil {
		result, err := r.callbacks.Reduce(declaration, args)
		if err != nil || result != (ast.ASTHandle{}) {
			return result, err
		}
	}
	return r.ast.MkApp(declaration, args)
}

func (r *ExternalRelation) AddFact(values []ast.ASTHandle) error {
	expression, err := r.Relation()
	if err != nil {
		return err
	}
	if err = r.validateFact(values); err != nil {
		return err
	}
	args := append([]ast.ASTHandle{expression}, values...)
	declaration, err := r.accessor(&r.storeDecl, ast.DatalogOpStore, args)
	if err != nil {
		return err
	}
	if r.callbacks.ReduceAssign != nil {
		return r.callbacks.ReduceAssign(declaration, args, args[:1])
	}
	return nil
}

func (r *ExternalRelation) ContainsFact(values []ast.ASTHandle) (found bool, resultErr error) {
	expression, err := r.Relation()
	if err != nil {
		return false, err
	}
	if err = r.validateFact(values); err != nil {
		return false, err
	}
	args := append([]ast.ASTHandle{expression}, values...)
	declaration, err := r.accessor(&r.selectDecl, ast.DatalogOpSelect, args)
	if err != nil {
		return false, err
	}
	result, err := r.reduce(declaration, args)
	if err != nil {
		return false, err
	}
	ref, err := r.ast.NewRef(result)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&ref)) }()
	return !r.ast.IsFalse(result), nil
}

func (r *ExternalRelation) Empty() (empty bool, resultErr error) {
	expression, err := r.Relation()
	if err != nil {
		return false, err
	}
	args := []ast.ASTHandle{expression}
	declaration, err := r.accessor(&r.emptyDecl, ast.DatalogOpIsEmpty, args)
	if err != nil {
		return false, err
	}
	result, err := r.reduce(declaration, args)
	if err != nil {
		return false, err
	}
	ref, err := r.ast.NewRef(result)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&ref)) }()
	return r.ast.IsTrue(result), nil
}

// Clone invokes the source clone assignment with a fresh relation expression.
func (r *ExternalRelation) Clone() (_ *ExternalRelation, resultErr error) {
	expression, err := r.Relation()
	if err != nil {
		return nil, err
	}
	sort, err := r.ast.GetSort(expression)
	if err != nil {
		return nil, err
	}
	fresh, err := r.ast.MkFreshConst("T", sort)
	if err != nil {
		return nil, err
	}
	freshRef, err := r.ast.NewRef(fresh)
	if err != nil {
		return nil, err
	}
	result := &ExternalRelation{plugin: r.plugin, manager: r.manager, ast: r.ast, sorts: append([]ast.ASTHandle(nil), r.sorts...), callbacks: r.callbacks, relation: freshRef}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	app, err := r.ast.MkDatalogClone(expression)
	if err != nil {
		return nil, err
	}
	appRef, err := r.ast.NewRef(app)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&appRef)) }()
	declaration, err := r.ast.GetDecl(app)
	if err != nil {
		return nil, err
	}
	if r.callbacks.ReduceAssign != nil {
		if err = r.callbacks.ReduceAssign(declaration, []ast.ASTHandle{expression}, []ast.ASTHandle{fresh}); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Complement invokes the source non-destructive complement reduction.
func (r *ExternalRelation) Complement() (_ *ExternalRelation, resultErr error) {
	expression, err := r.Relation()
	if err != nil {
		return nil, err
	}
	app, err := r.ast.MkDatalogComplement(expression)
	if err != nil {
		return nil, err
	}
	appRef, err := r.ast.NewRef(app)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.ast.DelRef(&appRef)) }()
	declaration, err := r.ast.GetDecl(app)
	if err != nil {
		return nil, err
	}
	value, err := r.reduce(declaration, []ast.ASTHandle{expression})
	if err != nil {
		return nil, err
	}
	ref, err := r.ast.NewRef(value)
	if err != nil {
		return nil, err
	}
	result := &ExternalRelation{plugin: r.plugin, manager: r.manager, ast: r.ast, sorts: append([]ast.ASTHandle(nil), r.sorts...), callbacks: r.callbacks, relation: ref}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	return result, nil
}

func (r *ExternalRelation) Close() (result error) {
	if r == nil || r.closed {
		return nil
	}
	for _, ref := range []*ast.ASTRef{&r.emptyDecl, &r.storeDecl, &r.selectDecl, &r.relation} {
		if *ref != (ast.ASTRef{}) {
			result = errors.Join(result, r.ast.DelRef(ref))
		}
	}
	if r.emptyDecl == (ast.ASTRef{}) && r.storeDecl == (ast.ASTRef{}) && r.selectDecl == (ast.ASTRef{}) && r.relation == (ast.ASTRef{}) {
		r.closed, r.plugin, r.manager, r.ast, r.sorts = true, nil, nil, nil, nil
	}
	return result
}
