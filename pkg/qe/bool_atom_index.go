package qe

// Source: src/qe/qe_bool_plugin.cpp atom_set consumers.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// BoolAtomIndex owns the positive and negative atom roots supplied by the
// enclosing NNF context. Duplicate atoms retain only one source-set entry.
type BoolAtomIndex struct {
	self               *BoolAtomIndex
	manager            *ast.ASTManager
	positive, negative map[ast.ASTHandle]ast.ASTRef
	// order lists the held atoms in insertion order (positive, then
	// negative); the source atom_set holds raw pointers, so the Go-only
	// references are released deterministically in this order.
	order     []ast.ASTHandle
	orderSets []map[ast.ASTHandle]ast.ASTRef
	closed    bool
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
	if err = index.insert(index.negative, negative); err != nil {
		return nil, err
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
		ref, err := i.manager.NewRef(atom)
		if err != nil {
			return err
		}
		target[atom] = ref
		i.order = append(i.order, atom)
		i.orderSets = append(i.orderSets, target)
	}
	return nil
}

func (i *BoolAtomIndex) Close() (result error) {
	if i == nil || i.self != i {
		return ast.ErrASTHandle
	}
	if i.closed {
		return nil
	}
	for k, atom := range i.order {
		set := i.orderSets[k]
		ref, held := set[atom]
		if !held {
			continue
		}
		result = errors.Join(result, i.manager.DelRef(&ref))
		if result == nil {
			delete(set, atom)
		}
	}
	if result == nil {
		i.closed = true
	}
	return result
}
