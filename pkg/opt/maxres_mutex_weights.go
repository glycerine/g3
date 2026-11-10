package opt

// Source: src/opt/maxsmt.cpp find_mutexes/process_mutex new_soft, an
// obj_map<expr, rational>: init_local adds the soft constraints in its
// table order.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type softWeightMap struct {
	table  *ast.ObjMap
	values []big.Rat // indexed by the entry's Num
}

func newSoftWeightMap(m *ast.ASTManager) *softWeightMap {
	return &softWeightMap{table: ast.NewObjMap(m)}
}

// find is new_soft.find(e); nil when absent.
func (w *softWeightMap) find(e ast.ASTHandle) (*big.Rat, error) {
	n, found, err := w.table.FindNum(e)
	if err != nil || !found {
		return nil, err
	}
	return &w.values[n], nil
}

// insert is new_soft.insert(e, weight): it replaces an existing value.
func (w *softWeightMap) insert(e ast.ASTHandle, weight *big.Rat) error {
	n, found, err := w.table.FindNum(e)
	if err != nil {
		return err
	}
	if found {
		w.values[n].Set(weight)
		return nil
	}
	w.values = append(w.values, *new(big.Rat).Set(weight))
	return w.table.InsertNum(e, uint64(len(w.values)-1))
}

func (w *softWeightMap) remove(e ast.ASTHandle) error { return w.table.Erase(e) }

// entries iterates the map in table order.
func (w *softWeightMap) entries() []maxResWeightedTerm {
	out := make([]maxResWeightedTerm, 0, w.table.Len())
	for _, entry := range w.table.Entries() {
		term := maxResWeightedTerm{expression: entry.Key}
		term.weight.Set(&w.values[entry.Num])
		out = append(out, term)
	}
	return out
}
