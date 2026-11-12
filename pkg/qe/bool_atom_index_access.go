package qe

import "github.com/glycerine/g3/pkg/ast"

func (i *BoolAtomIndex) Positive(atom ast.ASTHandle) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	_, found := i.positive[atom]
	return found, nil
}

func (i *BoolAtomIndex) Negative(atom ast.ASTHandle) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	_, found := i.negative[atom]
	return found, nil
}

func (i *BoolAtomIndex) positiveAtoms() map[ast.ASTHandle]ast.ASTRef { return i.positive }
func (i *BoolAtomIndex) negativeAtoms() map[ast.ASTHandle]ast.ASTRef { return i.negative }

// positiveTable and negativeTable list the atom_set entries in
// obj_hashtable iteration order: a fresh table filled in insertion order.
func (i *BoolAtomIndex) positiveTable() ([]ast.ASTHandle, error) {
	return i.tableOrder(i.positive, true)
}
func (i *BoolAtomIndex) negativeTable() ([]ast.ASTHandle, error) {
	return i.tableOrder(i.negative, false)
}

func (i *BoolAtomIndex) tableOrder(set map[ast.ASTHandle]ast.ASTRef, positive bool) ([]ast.ASTHandle, error) {
	table := ast.NewObjMap(i.manager)
	for k, atom := range i.order {
		if k >= len(i.orderPositive) || i.orderPositive[k] != positive {
			continue
		}
		if _, held := set[atom]; !held {
			continue
		}
		if err := table.Insert(atom, [2]ast.ASTHandle{}); err != nil {
			return nil, err
		}
	}
	entries := table.Entries()
	out := make([]ast.ASTHandle, len(entries))
	for k, entry := range entries {
		out[k] = entry.Key
	}
	return out, nil
}
