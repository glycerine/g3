package opt

// Source: src/opt/maxsmt.cpp find_mutexes/process_mutex new_soft, an
// obj_map<expr, rational>: init_local adds the soft constraints in its
// table order.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type softWeightMap struct {
	table  *ast.ObjMap
	values []numeral.Rational // indexed by the entry's Num; owned
	q      *numeral.RationalManager
}

func newSoftWeightMap(m *ast.ASTManager) *softWeightMap {
	return &softWeightMap{table: ast.NewObjMap(m), q: m.NumeralService().Manager}
}

// release is the obj_map<expr, rational> destructor.
func (w *softWeightMap) release() (err error) {
	for i := range w.values {
		err = errors.Join(err, w.q.Del(&w.values[i]))
	}
	w.values = nil
	return err
}

// find is new_soft.find(e); nil when absent.
func (w *softWeightMap) find(e ast.ASTHandle) (*numeral.Rational, error) {
	n, found, err := w.table.FindNum(e)
	if err != nil || !found {
		return nil, err
	}
	return &w.values[n], nil
}

// insert is new_soft.insert(e, weight): it replaces an existing value.
func (w *softWeightMap) insert(e ast.ASTHandle, weight *numeral.Rational) error {
	n, found, err := w.table.FindNum(e)
	if err != nil {
		return err
	}
	if found {
		return w.q.Copy(&w.values[n], weight)
	}
	value := numeral.NewRational()
	if err = w.q.Copy(&value, weight); err != nil {
		return err
	}
	w.values = append(w.values, value)
	return w.table.InsertNum(e, uint64(len(w.values)-1))
}

func (w *softWeightMap) remove(e ast.ASTHandle) error { return w.table.Erase(e) }

// entries iterates the map in table order; the caller owns the copies.
func (w *softWeightMap) entries() ([]maxResWeightedTerm, error) {
	out := make([]maxResWeightedTerm, 0, w.table.Len())
	for _, entry := range w.table.Entries() {
		term := maxResWeightedTerm{expression: entry.Key, weight: numeral.NewRational()}
		if err := w.q.Copy(&term.weight, &w.values[entry.Num]); err != nil {
			return out, errors.Join(err, releaseWeightedTerms(w.q, out))
		}
		out = append(out, term)
	}
	return out, nil
}

func releaseWeightedTerms(q *numeral.RationalManager, terms []maxResWeightedTerm) (err error) {
	for i := range terms {
		err = errors.Join(err, q.Del(&terms[i].weight))
	}
	return err
}
