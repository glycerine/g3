package qe

// Source: src/qe/qe_bool_plugin.cpp atom_set consumers.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// BoolAtomIndex is the source atom_set pair (obj_hashtable<app>): it
// borrows the positive and negative atoms of the enclosing NNF formula and
// takes no references, so an atom's reference count (and the rewriter's
// must_cache decisions that read it) matches the source. The map values are
// always the zero ASTRef. Duplicate atoms retain only one source-set entry.
type BoolAtomIndex struct {
	self               *BoolAtomIndex
	manager            *ast.ASTManager
	positive, negative map[ast.ASTHandle]ast.ASTRef
	// order lists the atoms in insertion order (positive, then negative).
	order     []ast.ASTHandle
	orderSets []map[ast.ASTHandle]ast.ASTRef
	// orderPositive marks the order entries of the positive set.
	orderPositive []bool
	closed        bool
}

func NewBoolAtomIndex(manager *ast.ASTManager, positive, negative []ast.ASTHandle) (result *BoolAtomIndex, err error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	index := &BoolAtomIndex{manager: manager, positive: make(map[ast.ASTHandle]ast.ASTRef), negative: make(map[ast.ASTHandle]ast.ASTRef)}
	index.self = index
	defer func() {
		if err != nil {
			err = errors.Join(err, index.Close())
			result = nil
		}
	}()
	if err = index.insert(index.positive, positive); err != nil {
		return nil, err
	}
	for len(index.orderPositive) < len(index.order) {
		index.orderPositive = append(index.orderPositive, true)
	}
	if err = index.insert(index.negative, negative); err != nil {
		return nil, err
	}
	for len(index.orderPositive) < len(index.order) {
		index.orderPositive = append(index.orderPositive, false)
	}
	return index, nil
}

func (i *BoolAtomIndex) open() error {
	if i == nil || i.self != i || i.manager == nil {
		return ast.ErrASTHandle
	}
	if i.closed {
		return arena.ErrClosed
	}
	return nil
}

func (i *BoolAtomIndex) insert(target map[ast.ASTHandle]ast.ASTRef, atoms []ast.ASTHandle) error {
	for _, atom := range atoms {
		if _, found := target[atom]; found {
			continue
		}
		if kind, err := i.manager.GetKind(atom); err != nil || kind != ast.ASTApp {
			return errors.Join(ast.ErrASTDomain, err)
		}
		target[atom] = ast.ASTRef{}
		i.order = append(i.order, atom)
		i.orderSets = append(i.orderSets, target)
	}
	return nil
}

func (i *BoolAtomIndex) Close() error {
	if i == nil || i.self != i {
		return ast.ErrASTHandle
	}
	if i.closed {
		return nil
	}
	for k, atom := range i.order {
		delete(i.orderSets[k], atom)
	}
	i.closed = true
	return nil
}
