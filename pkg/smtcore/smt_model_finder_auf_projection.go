package smtcore

// Source: src/smt/smt_model_finder.cpp get_instantiation_set_values,
// sort_values and mk_mono_proj.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// smtModelFinderProjectionValue is a value with its numeral_lt /
// signed_bv_lt key: the is_numeral rational (signed values normalized by
// bv_util::norm), held until the sort is done.
type smtModelFinderProjectionValue struct {
	value     ast.ASTHandle
	numeral   numeral.Rational
	isNumeral bool
	id        uint32
}

func (g *smtModelFinderAUFGraph) instantiationValues(node *smtModelFinderAUFNode) ([]ast.ASTHandle, error) {
	seen := make(map[ast.ASTHandle]bool)
	var values []ast.ASTHandle
	for _, entry := range node.root().set.entries() {
		term := entry.Key
		value, err := g.eval(term)
		if err != nil {
			return nil, err
		}
		if !value.IsNull() && !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	return values, nil
}

func (g *smtModelFinderAUFGraph) sortedProjectionValues(node *smtModelFinderAUFNode) (_ []ast.ASTHandle, result error) {
	values, err := g.instantiationValues(node)
	if err != nil {
		return nil, err
	}
	m := g.finder.manager
	bv, err := m.IsBitVectorSort(node.sort)
	if err != nil {
		return nil, err
	}
	var width uint32
	if bv && node.root().signed {
		width, err = m.BitVectorSize(node.sort)
		if err != nil {
			return nil, err
		}
	}
	q := m.NumeralService().Manager
	keys := make([]smtModelFinderProjectionValue, len(values))
	defer func() {
		for i := range keys {
			result = smtErrJoin(result, smtQDel(q, &keys[i].numeral))
		}
	}()
	for i := range keys {
		keys[i].numeral = smtQZero()
	}
	// bv_util::norm(v, sz, true): v >= 2^(sz-1) becomes v - 2^sz.
	signBit, modulus := smtQZero(), smtQZero()
	defer func() { result = smtErrJoin(result, smtQDel(q, &signBit), smtQDel(q, &modulus)) }()
	signed := bv && node.root().signed
	if signed {
		two := smtQInt(2)
		if err = q.Power(&two, width, &modulus); err != nil {
			return nil, err
		}
		if err = q.Div(&modulus, &two, &signBit); err != nil {
			return nil, err
		}
	}
	for i, value := range values {
		id, err := m.GetASTID(value)
		if err != nil {
			return nil, err
		}
		key := &keys[i]
		key.value, key.id = value, id
		if bv {
			_, key.isNumeral, err = m.BitVectorNumeralRational(value, &key.numeral)
		} else {
			_, key.isNumeral, err = m.ArithmeticNumeralRational(value, &key.numeral)
		}
		if err != nil {
			return nil, err
		}
		if key.isNumeral && signed {
			c, err := q.Compare(&key.numeral, &signBit)
			if err != nil {
				return nil, err
			}
			if c >= 0 {
				if err = q.Sub(&key.numeral, &modulus, &key.numeral); err != nil {
					return nil, err
				}
			}
		}
	}
	var cmpErr error
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].isNumeral && keys[j].isNumeral {
			c, err := q.Compare(&keys[i].numeral, &keys[j].numeral)
			if err != nil && cmpErr == nil {
				cmpErr = err
			}
			return c < 0
		}
		return keys[i].id < keys[j].id
	})
	if cmpErr != nil {
		return nil, cmpErr
	}
	for i := range keys {
		values[i] = keys[i].value
	}
	return values, nil
}

func (g *smtModelFinderAUFGraph) makeMonotoneProjection(node *smtModelFinderAUFNode) (result error) {
	if node == nil || node.parent != nil || !node.mono || node.set == nil || g.model == nil {
		return ast.ErrASTDomain
	}
	if err := g.addNodeMonotoneExceptions(node); err != nil {
		return err
	}
	values, err := g.sortedProjectionValues(node)
	if err != nil || len(values) == 0 {
		return err
	}
	m := g.finder.manager
	bv, err := m.IsBitVectorSort(node.sort)
	if err != nil {
		return err
	}
	// func_decl * p = m.mk_fresh_func_decl(1, &s, s) precedes var.
	function, err := m.MkFreshFuncDecl(symbol.Symbol{}, symbol.Symbol{}, []ast.ASTHandle{node.sort}, node.sort)
	if err != nil {
		return err
	}
	// expr_ref var: released when mk_mono_proj returns.
	variable, err := m.MkVar(0, node.sort)
	if err != nil {
		return err
	}
	if err = m.IncRef(variable); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DecRef(variable)) }()
	definition := values[len(values)-1]
	for i := len(values) - 1; i >= 1; i-- {
		var condition ast.ASTHandle
		if bv {
			if node.signed {
				condition, err = m.MkBitVectorSLE(values[i], variable)
			} else {
				condition, err = m.MkBitVectorULE(values[i], variable)
			}
			if err == nil {
				condition, err = m.MkNot(condition)
			}
		} else {
			family, e := m.ArithmeticFamilyID()
			if e != nil {
				return e
			}
			condition, err = m.MkFamilyApp(family, 4, nil, []ast.ASTHandle{variable, values[i]})
		}
		if err != nil {
			return err
		}
		definition, err = m.MkIte(condition, values[i-1], definition)
		if err != nil {
			return err
		}
	}
	interpretation, err := model.NewFunctionInterpretation(m, 1)
	if err != nil {
		return err
	}
	if err := interpretation.SetElse(definition); err != nil {
		return errors.Join(err, interpretation.Close())
	}
	if err := g.model.RegisterAuxFunction(function, interpretation); err != nil {
		return errors.Join(err, interpretation.Close())
	}
	return node.setProjection(function)
}
