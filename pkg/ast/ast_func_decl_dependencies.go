package ast

// Source: src/ast/func_decl_dependencies.cpp;
// src/ast/func_decl_dependencies.h.

import "errors"

// FuncDeclSet is func_decl_set (obj_hashtable<func_decl>): membership plus
// the source table's iteration order. Handles are borrowed until Insert
// gives a set to FuncDeclDependencies.
type FuncDeclSet struct{ table *ObjMap }

func NewFuncDeclSet(m *ASTManager) *FuncDeclSet { return &FuncDeclSet{table: NewObjMap(m)} }

// Insert is m_set.insert(d).
func (s *FuncDeclSet) Insert(d ASTHandle) error { return s.table.Insert(d, [2]ASTHandle{}) }

// Contains is m_set.contains(d).
func (s *FuncDeclSet) Contains(d ASTHandle) bool {
	if s == nil {
		return false
	}
	found, err := s.table.Contains(d)
	return err == nil && found
}

func (s *FuncDeclSet) Len() int {
	if s == nil {
		return 0
	}
	return s.table.Len()
}

// Keys lists the members in iteration order.
func (s *FuncDeclSet) Keys() []ASTHandle {
	if s == nil {
		return nil
	}
	entries := s.table.Entries()
	keys := make([]ASTHandle, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}
	return keys
}

// CollectFuncDecls is collect_func_decls: for_each_expr visits each node
// after its children (left to right), and each uninterpreted application
// with arguments inserts its declaration. With nonGroundOnly, ground
// applications are skipped; traversal still descends through them.
func (m *ASTManager) CollectFuncDecls(root ASTHandle, out *FuncDeclSet, nonGroundOnly bool) error {
	if out == nil {
		return ErrASTHandle
	}
	visited := make(map[ASTHandle]struct{})
	type frame struct {
		n        ASTHandle
		children []ASTHandle
		next     int
	}
	children := func(n ASTHandle) ([]ASTHandle, ASTKind, error) {
		kind, err := m.GetKind(n)
		if err != nil {
			return nil, kind, err
		}
		switch kind {
		case ASTApp:
			count, err := m.GetNumArgs(n)
			if err != nil {
				return nil, kind, err
			}
			result := make([]ASTHandle, count)
			for i := range result {
				if result[i], err = m.GetArg(n, uint32(i)); err != nil {
					return nil, kind, err
				}
			}
			return result, kind, nil
		case ASTQuantifier:
			// get_child order: body, patterns, no-patterns.
			body, err := m.GetQuantifierBody(n)
			if err != nil {
				return nil, kind, err
			}
			result := []ASTHandle{body}
			patterns, err := m.GetNumPatterns(n)
			if err != nil {
				return nil, kind, err
			}
			for i := uint32(0); i < patterns; i++ {
				p, err := m.GetPattern(n, i)
				if err != nil {
					return nil, kind, err
				}
				result = append(result, p)
			}
			noPatterns, err := m.GetNumNoPatterns(n)
			if err != nil {
				return nil, kind, err
			}
			for i := uint32(0); i < noPatterns; i++ {
				p, err := m.GetNoPattern(n, i)
				if err != nil {
					return nil, kind, err
				}
				result = append(result, p)
			}
			return result, kind, nil
		case ASTVar:
			return nil, kind, nil
		}
		return nil, kind, ErrASTDomain
	}
	visit := func(n ASTHandle, kind ASTKind) error {
		if kind != ASTApp {
			return nil
		}
		count, err := m.GetNumArgs(n)
		if err != nil || count == 0 {
			return err
		}
		if nonGroundOnly {
			ground, err := m.IsGround(n)
			if err != nil || ground {
				return err
			}
		}
		decl, err := m.GetDecl(n)
		if err != nil {
			return err
		}
		family, err := m.GetFamilyID(decl)
		if err != nil || family != NullFamilyID {
			return err
		}
		return out.Insert(decl)
	}
	rootChildren, _, err := children(root)
	if err != nil {
		return err
	}
	stack := []frame{{n: root, children: rootChildren}}
	for len(stack) != 0 {
		top := &stack[len(stack)-1]
		if top.next < len(top.children) {
			child := top.children[top.next]
			top.next++
			if _, seen := visited[child]; seen {
				continue
			}
			grand, _, err := children(child)
			if err != nil {
				return err
			}
			stack = append(stack, frame{n: child, children: grand})
			continue
		}
		n := top.n
		stack = stack[:len(stack)-1]
		if _, seen := visited[n]; seen {
			continue
		}
		visited[n] = struct{}{}
		kind, err := m.GetKind(n)
		if err != nil {
			return err
		}
		if err = visit(n, kind); err != nil {
			return err
		}
	}
	return nil
}

type funcDeclDependencyEntry struct {
	key     ASTRef
	deps    *FuncDeclSet
	depRefs []ASTRef // in the set's table order (inc_ref/dec_ref(m, *s))
	nextRef int
}

// FuncDeclDependencies owns pins for every successfully inserted declaration
// and its dependency set. Native maps replace the source object maps and sets.
type FuncDeclDependencies struct {
	self    *FuncDeclDependencies
	manager *ASTManager
	deps    map[ASTHandle]*funcDeclDependencyEntry
	order   []ASTHandle
	// layout is m_deps' obj_map layout; reset walks it.
	layout *ObjMap
	closed bool
}

func NewFuncDeclDependencies(m *ASTManager) (*FuncDeclDependencies, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	d := &FuncDeclDependencies{manager: m, deps: make(map[ASTHandle]*funcDeclDependencyEntry), layout: NewObjMap(m)}
	d.self = d
	return d, nil
}

func (d *FuncDeclDependencies) open() error {
	if d == nil || d.self != d || d.manager == nil || d.closed {
		return ErrASTHandle
	}
	_, err := d.manager.ProofsEnabled()
	return err
}

func (d *FuncDeclDependencies) Collect(root ASTHandle, out *FuncDeclSet) error {
	if err := d.open(); err != nil {
		return err
	}
	return d.manager.CollectFuncDecls(root, out, false)
}

func (d *FuncDeclDependencies) CollectNonGround(root ASTHandle, out *FuncDeclSet) error {
	if err := d.open(); err != nil {
		return err
	}
	return d.manager.CollectFuncDecls(root, out, true)
}

func (d *FuncDeclDependencies) createsCycle(root ASTHandle, incoming *FuncDeclSet) bool {
	const (
		open uint8 = iota
		active
		closed
	)
	colors := make(map[ASTHandle]uint8)
	var visit func(ASTHandle) bool
	visit = func(n ASTHandle) bool {
		switch colors[n] {
		case active:
			return true
		case closed:
			return false
		}
		colors[n] = active
		var children *FuncDeclSet
		if n == root {
			children = incoming
		} else if entry := d.deps[n]; entry != nil {
			children = entry.deps
		}
		for _, child := range children.Keys() {
			if visit(child) {
				return true
			}
		}
		colors[n] = closed
		return false
	}
	return visit(root)
}

func (d *FuncDeclDependencies) validateDecl(h ASTHandle) error {
	kind, err := d.manager.GetKind(h)
	if err != nil {
		return err
	}
	if kind != ASTFuncDecl {
		return ErrASTDomain
	}
	return nil
}

// Insert returns false for a duplicate key or a dependency cycle.
func (d *FuncDeclDependencies) Insert(decl ASTHandle, dependencies *FuncDeclSet) (bool, error) {
	if err := d.open(); err != nil {
		return false, err
	}
	if dependencies == nil {
		return false, ErrASTHandle
	}
	if _, found := d.deps[decl]; found {
		return false, nil
	}
	if err := d.validateDecl(decl); err != nil {
		return false, err
	}
	keys := dependencies.Keys()
	for _, dependency := range keys {
		if err := d.validateDecl(dependency); err != nil {
			return false, err
		}
	}
	// m_deps.insert(f, s), then the cycle check erases it again.
	if err := d.layout.Insert(decl, [2]ASTHandle{}); err != nil {
		return false, err
	}
	if d.createsCycle(decl, dependencies) {
		return false, d.layout.Erase(decl)
	}
	entry := &funcDeclDependencyEntry{deps: dependencies}
	key, err := d.manager.NewRef(decl)
	if err != nil {
		return false, errors.Join(err, d.layout.Erase(decl))
	}
	entry.key = key
	// inc_ref(m_manager, *s): the set in table order.
	for _, dependency := range keys {
		ref, e := d.manager.NewRef(dependency)
		if e != nil {
			for i := len(entry.depRefs) - 1; i >= 0; i-- {
				_ = d.manager.DelRef(&entry.depRefs[i])
			}
			_ = d.manager.DelRef(&entry.key)
			return false, errors.Join(e, d.layout.Erase(decl))
		}
		entry.depRefs = append(entry.depRefs, ref)
	}
	d.deps[decl] = entry
	d.order = append(d.order, decl)
	return true, nil
}

func (d *FuncDeclDependencies) Contains(decl ASTHandle) bool {
	if d == nil || d.self != d || d.closed {
		return false
	}
	_, found := d.deps[decl]
	return found
}

// GetDependencies returns a copy of decl's dependency set (same order).
func (d *FuncDeclDependencies) GetDependencies(decl ASTHandle) *FuncDeclSet {
	if d == nil || d.self != d || d.closed {
		return nil
	}
	entry := d.deps[decl]
	if entry == nil {
		return nil
	}
	result := NewFuncDeclSet(d.manager)
	for _, h := range entry.deps.Keys() {
		_ = result.Insert(h)
	}
	return result
}

func (d *FuncDeclDependencies) releaseEntry(entry *funcDeclDependencyEntry) error {
	if entry.key != (ASTRef{}) {
		if err := d.manager.DelRef(&entry.key); err != nil {
			return err
		}
	}
	for entry.nextRef < len(entry.depRefs) {
		if err := d.manager.DelRef(&entry.depRefs[entry.nextRef]); err != nil {
			return err
		}
		entry.nextRef++
	}
	return nil
}

func (d *FuncDeclDependencies) Erase(decl ASTHandle) error {
	if err := d.open(); err != nil {
		return err
	}
	entry := d.deps[decl]
	if entry == nil {
		return nil
	}
	if entry.key != (ASTRef{}) {
		if err := d.layout.Erase(decl); err != nil {
			return err
		}
	}
	if err := d.releaseEntry(entry); err != nil {
		return err
	}
	delete(d.deps, decl)
	for i, h := range d.order {
		if h == decl {
			copy(d.order[i:], d.order[i+1:])
			d.order = d.order[:len(d.order)-1]
			break
		}
	}
	return nil
}

func (d *FuncDeclDependencies) Reset() error {
	if err := d.open(); err != nil {
		return err
	}
	// reset: m_deps in table order.
	for _, slot := range d.layout.Entries() {
		if err := d.Erase(slot.Key); err != nil {
			return err
		}
	}
	for len(d.order) != 0 {
		if err := d.Erase(d.order[0]); err != nil {
			return err
		}
	}
	return nil
}

func (d *FuncDeclDependencies) Close() error {
	if d == nil || d.self != d {
		return ErrASTHandle
	}
	if d.closed {
		return nil
	}
	if err := d.Reset(); err != nil {
		return err
	}
	d.closed = true
	return nil
}
