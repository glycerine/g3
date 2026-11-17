package rel

// Source: src/muz/rel/dl_external_relation.cpp/.h and the fixedpoint callback
// fallback in src/api/api_datalog.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// ExternalRelationCallbacks are the concrete fixedpoint reduction callbacks.
// The plugin keeps callback declarations, arguments, and reduction results
// alive until it closes. Returning a null handle requests the source fallback.
type ExternalRelationCallbacks struct {
	Reduce       func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error)
	ReduceAssign func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error
}

// ExternalRelationPlugin owns the shared callback configuration for relations
// created by one source external relation plugin.
type ExternalRelationPlugin struct {
	manager   *RelationManager
	ast       *ast.ASTManager
	callbacks ExternalRelationCallbacks
	trail     []ast.ASTRef
	users     int
	foreign   int
	autoClose bool
	closed    bool
}

func NewExternalRelationPlugin(manager *RelationManager, callbacks ExternalRelationCallbacks) (*ExternalRelationPlugin, error) {
	if manager == nil || manager.open() != nil {
		return nil, ErrTableRelation
	}
	return &ExternalRelationPlugin{manager: manager, ast: manager.manager, callbacks: callbacks}, nil
}

// SetCallbacks replaces the context-level reduction callbacks used by every
// relation created through this plugin, matching fixedpoint_context setters.
func (plugin *ExternalRelationPlugin) SetCallbacks(callbacks ExternalRelationCallbacks) error {
	if plugin == nil || plugin.closed || plugin.manager == nil || plugin.manager.open() != nil {
		return ErrTableRelation
	}
	plugin.callbacks = callbacks
	return nil
}

// ExternalRelation owns its relation expression and cached accessor declarations.
// The relation manager and callback functions are borrowed.
type ExternalRelation struct {
	plugin                           *ExternalRelationPlugin
	manager                          *RelationManager
	ast                              *ast.ASTManager
	sorts                            []ast.ASTHandle
	relation                         ast.ASTRef
	selectDecl, storeDecl, emptyDecl ast.ASTRef
	closed                           bool
	// pending: a placeholder whose mk_empty has not run yet.
	pending bool
	// foreign: made for the API user (NewForeign), not the relation manager.
	foreign bool
}

// NewExternalRelation creates a fresh external relation and invokes the empty
// relation assignment callback, if present.
func NewExternalRelation(manager *RelationManager, sorts []ast.ASTHandle, callbacks ExternalRelationCallbacks) (_ *ExternalRelation, resultErr error) {
	plugin, err := NewExternalRelationPlugin(manager, callbacks)
	if err != nil {
		return nil, err
	}
	plugin.autoClose = true
	return plugin.NewEmpty(sorts)
}

// NewEmpty creates a relation belonging to this callback plugin.
func (plugin *ExternalRelationPlugin) NewEmpty(sorts []ast.ASTHandle) (result *ExternalRelation, resultErr error) {
	if plugin == nil || plugin.closed || plugin.manager == nil || plugin.manager.open() != nil {
		return nil, ErrTableRelation
	}
	r := &ExternalRelation{plugin: plugin, manager: plugin.manager, ast: plugin.manager.manager, sorts: append([]ast.ASTHandle(nil), sorts...), pending: true}
	plugin.users++
	if plugin.manager.deferRelationCreation {
		// A placeholder: get_relation creates it (completeCreation).
		return r, nil
	}
	if err := r.completeCreation(); err != nil {
		return nil, errors.Join(err, r.Close())
	}
	return r, nil
}

// completeCreation is external_relation_plugin::mk_empty's body: a fresh
// constant "T" of the relation sort, assigned by OP_RA_EMPTY.
func (r *ExternalRelation) completeCreation() (resultErr error) {
	if !r.pending {
		return nil
	}
	r.pending = false
	plugin := r.plugin
	m := r.ast
	relationSort, err := m.MkDatalogRelationSort(r.sorts)
	if err != nil {
		return err
	}
	sortRef, err := m.NewRef(relationSort)
	if err != nil {
		return err
	}
	owner := &externalRelationConstructionOwner{manager: m, temps: []ast.ASTRef{sortRef}}
	defer func() { resultErr = owner.dispose(resultErr) }()
	expression, err := m.MkFreshConst("T", relationSort)
	if err != nil {
		return err
	}
	if r.relation, err = m.NewRef(expression); err != nil {
		return err
	}
	// func_decl_ref empty_decl(m.mk_func_decl(fid, OP_RA_EMPTY, 1,
	// &param, 0, nullptr)).
	fid, err := m.RegisterDatalogPlugin()
	if err != nil {
		return err
	}
	param, err := m.NewParameterAST(relationSort)
	if err != nil {
		return err
	}
	declaration, err := m.MkFamilyFuncDecl(fid, ast.DatalogOpEmpty, []ast.ASTParameter{param}, nil, ast.ASTHandle{})
	err = errors.Join(err, m.DelParameter(&param))
	if err != nil {
		return err
	}
	if declaration == (ast.ASTHandle{}) {
		return ast.ErrASTDomain
	}
	emptyRef, err := m.NewRef(declaration)
	if err != nil {
		return err
	}
	owner.temps = append(owner.temps, emptyRef)
	return plugin.reduceAssign(declaration, nil, []ast.ASTHandle{expression})
}

func (r *ExternalRelation) open() error {
	if r == nil || r.closed || r.plugin == nil || r.plugin.closed || r.ast == nil || r.manager == nil || r.manager.open() != nil || r.relation == (ast.ASTRef{}) && !r.pending {
		return ErrTableRelation
	}
	if r.pending {
		return r.completeCreation()
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
	switch op {
	case ast.DatalogOpStore, ast.DatalogOpSelect, ast.DatalogOpIsEmpty:
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	// fn = m.mk_func_decl(fid, k, 0, nullptr, args.size(), args.c_ptr()):
	// the decl of the arguments' sorts, without an application.
	fid, err := r.ast.RegisterDatalogPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, len(args))
	for i, a := range args {
		if domain[i], err = r.ast.GetSort(a); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	declaration, err := r.ast.MkFamilyFuncDecl(fid, op, nil, domain, ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if declaration == (ast.ASTHandle{}) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	*cache, err = r.ast.NewRef(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return declaration, nil
}

func (r *ExternalRelation) reduce(declaration ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
	return r.plugin.reduce(declaration, args)
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
	return r.plugin.reduceAssign(declaration, args, args[:1])
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
func (r *ExternalRelation) Clone() (result *ExternalRelation, resultErr error) {
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
	result = &ExternalRelation{plugin: r.plugin, manager: r.manager, ast: r.ast, sorts: append([]ast.ASTHandle(nil), r.sorts...), relation: freshRef}
	r.plugin.users++
	owner := &externalRelationConstructionOwner{manager: r.ast, result: result}
	defer func() {
		resultErr = owner.dispose(resultErr)
		if resultErr != nil {
			result = nil
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
	owner.temps = append(owner.temps, appRef)
	declaration, err := r.ast.GetDecl(app)
	if err != nil {
		return nil, err
	}
	if err = r.plugin.reduceAssign(declaration, []ast.ASTHandle{expression}, []ast.ASTHandle{fresh}); err != nil {
		return nil, err
	}
	return result, nil
}

// Complement invokes the source non-destructive complement reduction.
func (r *ExternalRelation) Complement() (result *ExternalRelation, resultErr error) {
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
	owner := &externalRelationConstructionOwner{manager: r.ast, temps: []ast.ASTRef{appRef}}
	defer func() {
		resultErr = owner.dispose(resultErr)
		if resultErr != nil {
			result = nil
		}
	}()
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
	result = &ExternalRelation{plugin: r.plugin, manager: r.manager, ast: r.ast, sorts: append([]ast.ASTHandle(nil), r.sorts...), relation: ref}
	r.plugin.users++
	owner.result = result
	return result, nil
}

func (r *ExternalRelation) Close() (result error) {
	if r == nil || r.closed && r.plugin == nil {
		return nil
	}
	if !r.closed {
		for _, ref := range []*ast.ASTRef{&r.emptyDecl, &r.storeDecl, &r.selectDecl, &r.relation} {
			if *ref != (ast.ASTRef{}) {
				result = errors.Join(result, r.ast.DelRef(ref))
			}
		}
		if r.emptyDecl != (ast.ASTRef{}) || r.storeDecl != (ast.ASTRef{}) || r.selectDecl != (ast.ASTRef{}) || r.relation != (ast.ASTRef{}) {
			return result
		}
		r.closed = true
		r.plugin.users--
		if r.foreign {
			r.plugin.foreign--
		}
	}
	if r.plugin.autoClose && r.plugin.users == 0 {
		if err := r.plugin.Close(); err != nil {
			return errors.Join(result, err)
		}
	}
	r.plugin, r.manager, r.ast, r.sorts = nil, nil, nil, nil
	return result
}
