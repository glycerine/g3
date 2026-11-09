package base

// Source: src/muz/base/bind_variables.cpp and bind_variables.h.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var ErrBindVariablesOwner = errors.New("muz/base: invalid or closed variable binder")

// VariableBinder owns the constant applications declared as implicit Datalog
// variables and abstracts them into a forall or exists prefix on demand.
type VariableBinder struct {
	manager    *ast.ASTManager
	variables  []ast.ASTHandle
	references []ast.ASTRef
	closing    bool
	closed     bool
}

func NewVariableBinder(manager *ast.ASTManager) (*VariableBinder, error) {
	if manager == nil {
		return nil, ErrBindVariablesOwner
	}
	if _, err := manager.ProofsEnabled(); err != nil {
		return nil, err
	}
	return &VariableBinder{manager: manager}, nil
}

func (b *VariableBinder) open() error {
	if b == nil || b.closed || b.closing || b.manager == nil {
		return ErrBindVariablesOwner
	}
	return nil
}

// AddVariable retains a zero-arity constant application. Re-adding a variable
// is a no-op, matching the source map's single bound index per application.
func (b *VariableBinder) AddVariable(variable ast.ASTHandle) error {
	if err := b.open(); err != nil {
		return err
	}
	kind, err := b.manager.GetKind(variable)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return fmt.Errorf("muz/base: implicit variable must be a constant application")
	}
	count, err := b.manager.GetNumArgs(variable)
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("muz/base: implicit variable must be zero-arity")
	}
	for _, current := range b.variables {
		if current == variable {
			return nil
		}
	}
	reference, err := b.manager.NewRef(variable)
	if err != nil {
		return err
	}
	b.variables = append(b.variables, variable)
	b.references = append(b.references, reference)
	return nil
}

// Bind abstracts registered constants and returns one owning expression
// reference. With no registered variables it returns an owning reference to
// the original expression.
func (b *VariableBinder) Bind(expression ast.ASTHandle, forall bool) (result ast.ASTRef, resultErr error) {
	if err := b.open(); err != nil {
		return result, err
	}
	root, err := b.manager.NewRef(expression)
	if err != nil {
		return result, err
	}
	defer func() {
		if e := b.manager.DelRef(&root); e != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("muz/base: release binding input: %w", e))
		}
	}()
	result, err = b.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	failed := true
	defer func() {
		if failed || resultErr != nil {
			if e := b.manager.DelRef(&result); e != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("muz/base: release failed binding result: %w", e))
			}
		}
	}()
	if len(b.variables) == 0 {
		err = b.manager.SetRef(result, expression)
	} else {
		err = b.bindAbstract(expression, forall, result)
	}
	if err != nil {
		return result, err
	}
	failed = false
	return result, nil
}

// bindState is one bind_variables::operator() call: m_var2bound's values,
// m_bound, m_names, m_pinned and the shared m_todo stack.
type bindState struct {
	var2bound map[ast.ASTHandle]ast.ASTHandle
	bound     []ast.ASTHandle
	names     []symbol.Symbol
	pinned    []ast.ASTRef
	todo      []ast.ASTHandle
}

// reserve allocates the m_pinned slot before its AST exists, so a refused
// allocation never leaves a fresh node without an owner.
func (st *bindState) reserve(m *ast.ASTManager) (ast.ASTRef, error) {
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ref, err
	}
	st.pinned = append(st.pinned, ref)
	return ref, nil
}

// pinned stores a freshly created AST in its reserved slot.
func pinned(m *ast.ASTManager, slot ast.ASTRef) func(ast.ASTHandle, error) (ast.ASTHandle, error) {
	return func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if !h.IsNull() {
			err = errors.Join(err, m.SetRef(slot, h))
		}
		return h, err
	}
}

// bindAbstract is bind_variables::operator(): abstract with first-encounter
// bound indices, then wrap the reversed bound sorts and names in one
// quantifier. m_pinned releases forward after the quantifier exists.
func (b *VariableBinder) bindAbstract(expression ast.ASTHandle, forall bool, result ast.ASTRef) (resultErr error) {
	m := b.manager
	st := &bindState{var2bound: make(map[ast.ASTHandle]ast.ASTHandle, len(b.variables))}
	for _, variable := range b.variables {
		st.var2bound[variable] = ast.ASTHandle{}
	}
	defer func() {
		for i := range st.pinned {
			resultErr = errors.Join(resultErr, m.DelRef(&st.pinned[i]))
		}
	}()
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	abstracted, err := b.abstract(st, expression, cache, 0)
	if err != nil {
		return err
	}
	if len(st.names) == 0 {
		return m.SetRef(result, abstracted)
	}
	count := len(st.names)
	sorts := make([]ast.ASTHandle, count)
	names := make([]symbol.Symbol, count)
	for i := 0; i < count; i++ {
		sorts[count-1-i] = st.bound[i]
		names[count-1-i] = st.names[i]
	}
	_, err = pinned(m, result)(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: abstracted}))
	return err
}

// abstract is bind_variables::abstract over the shared todo stack.
func (b *VariableBinder) abstract(st *bindState, term ast.ASTHandle, cache map[ast.ASTHandle]ast.ASTHandle, scope uint32) (ast.ASTHandle, error) {
	m := b.manager
	base := len(st.todo)
	st.todo = append(st.todo, term)
	var args []ast.ASTHandle
	for len(st.todo) > base {
		e := st.todo[len(st.todo)-1]
		if _, ok := cache[e]; ok {
			st.todo = st.todo[:len(st.todo)-1]
			continue
		}
		kind, err := m.GetKind(e)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		switch kind {
		case ast.ASTVar:
			cache[e] = e
			st.todo = st.todo[:len(st.todo)-1]
		case ast.ASTApp:
			if v, registered := st.var2bound[e]; registered {
				if v.IsNull() {
					// Allocate a bound index.
					sort, err := m.GetSort(e)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					slot, err := st.reserve(m)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					if v, err = pinned(m, slot)(m.MkVar(uint32(len(st.names)), sort)); err != nil {
						return ast.ASTHandle{}, err
					}
					declaration, err := m.GetDecl(e)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					name, err := m.GetASTName(declaration)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					st.names = append(st.names, name)
					st.bound = append(st.bound, sort)
					st.var2bound[e] = v
				}
				if scope == 0 {
					cache[e] = v
				} else {
					index, err := m.GetVarIndex(v)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					sort, err := m.GetSort(v)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					slot, err := st.reserve(m)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					shifted, err := pinned(m, slot)(m.MkVar(scope+index, sort))
					if err != nil {
						return ast.ASTHandle{}, err
					}
					cache[e] = shifted
				}
				st.todo = st.todo[:len(st.todo)-1]
				break
			}
			count, err := m.GetNumArgs(e)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			allVisited, someDiff := true, false
			args = args[:0]
			for i := uint32(0); i < count; i++ {
				arg, err := m.GetArg(e, i)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if mapped, ok := cache[arg]; !ok {
					st.todo = append(st.todo, arg)
					allVisited = false
				} else if allVisited {
					args = append(args, mapped)
					if mapped != arg {
						someDiff = true
					}
				}
			}
			if allVisited {
				rebuilt := e
				if someDiff {
					declaration, err := m.GetDecl(e)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					slot, err := st.reserve(m)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					if rebuilt, err = pinned(m, slot)(m.MkApp(declaration, args)); err != nil {
						return ast.ASTHandle{}, err
					}
				}
				cache[e] = rebuilt
				st.todo = st.todo[:len(st.todo)-1]
			}
		case ast.ASTQuantifier:
			declarations, err := m.GetNumDeclarations(e)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			inner := scope + declarations
			nested := make(map[ast.ASTHandle]ast.ASTHandle)
			patternCount, err := m.GetNumPatterns(e)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			patternsHeld := int(patternCount)
			// patterns (expr_ref_buffer) and result1 are held until the
			// update; result1 dies first, then the buffer releases forward.
			var held []ast.ASTRef
			release := func(err error) error {
				if n := len(held); n != 0 && held[n-1] != (ast.ASTRef{}) && n > patternsHeld {
					err = errors.Join(err, m.DelRef(&held[n-1]))
					held = held[:n-1]
				}
				for i := range held {
					err = errors.Join(err, m.DelRef(&held[i]))
				}
				return err
			}
			patterns := make([]ast.ASTHandle, 0, patternCount)
			for i := uint32(0); i < patternCount; i++ {
				pattern, err := m.GetPattern(e, i)
				if err != nil {
					return ast.ASTHandle{}, release(err)
				}
				abstracted, err := b.abstract(st, pattern, nested, inner)
				if err != nil {
					return ast.ASTHandle{}, release(err)
				}
				ref, err := m.NewRef(abstracted)
				if err != nil {
					return ast.ASTHandle{}, release(err)
				}
				held = append(held, ref)
				patterns = append(patterns, abstracted)
			}
			body, err := m.GetQuantifierBody(e)
			if err != nil {
				return ast.ASTHandle{}, release(err)
			}
			body, err = b.abstract(st, body, nested, inner)
			if err != nil {
				return ast.ASTHandle{}, release(err)
			}
			ref, err := m.NewRef(body)
			if err != nil {
				return ast.ASTHandle{}, release(err)
			}
			held = append(held, ref)
			slot, err := st.reserve(m)
			if err != nil {
				return ast.ASTHandle{}, release(err)
			}
			updated, err := pinned(m, slot)(m.UpdateQuantifierPatterns(e, patterns, body))
			if err = release(err); err != nil {
				return ast.ASTHandle{}, err
			}
			cache[e] = updated
			st.todo = st.todo[:len(st.todo)-1]
		default:
			return ast.ASTHandle{}, fmt.Errorf("muz/base: unexpected AST kind %v while binding variables", kind)
		}
	}
	return cache[term], nil
}

func (b *VariableBinder) Close() (result error) {
	if b == nil || b.manager == nil {
		return ErrBindVariablesOwner
	}
	if b.closed {
		return nil
	}
	b.closing = true
	for len(b.references) != 0 {
		last := len(b.references) - 1
		var err error
		if b.references[last] != (ast.ASTRef{}) {
			err = b.manager.DelRef(&b.references[last])
		}
		if b.references[last] == (ast.ASTRef{}) {
			// Native destruction may consume a reference before reporting an
			// error. Never replay that decrement when Close is retried.
			b.references = b.references[:last]
			b.variables = b.variables[:last]
		}
		if err != nil {
			return err
		}
	}
	b.references = nil
	b.variables = nil
	b.closing = false
	b.closed = true
	return nil
}
