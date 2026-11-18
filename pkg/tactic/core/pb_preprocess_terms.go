package core

// Source: src/tactic/core/pb_preprocess_tactic.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type pbPreprocessTerm struct {
	expression  ast.ASTHandle
	coefficient *unumeral.Rational // borrowed from a pbPreprocessHeld
}

type pbPreprocessConstraint struct {
	terms []pbPreprocessTerm
	bound *unumeral.Rational // borrowed from a pbPreprocessHeld
}

// pbPreprocessHeld owns the rationals (of the manager's
// NumeralService().Manager) behind the constraints a pass parses: z3's
// vector<rational> coeffs and rational k locals.
type pbPreprocessHeld struct {
	values []*unumeral.Rational
}

func (t *PBPreprocessTactic) heldValue(held *pbPreprocessHeld) *unumeral.Rational {
	v := new(unumeral.Rational)
	*v = unumeral.NewRational()
	held.values = append(held.values, v)
	return v
}

func (t *PBPreprocessTactic) heldOne(held *pbPreprocessHeld) (*unumeral.Rational, error) {
	v := t.heldValue(held)
	return v, t.manager.NumeralService().Manager.SetInt32(v, 1)
}

func (t *PBPreprocessTactic) releaseHeld(held *pbPreprocessHeld) (err error) {
	q := t.manager.NumeralService().Manager
	for _, v := range held.values {
		err = errors.Join(err, q.Del(v))
	}
	held.values = nil
	return err
}

// rational is pb.get_k(value) or pb.get_coeff(value, *coefficient) into a
// rational of held.
func (t *PBPreprocessTactic) rational(value ast.ASTHandle, coefficient *uint32, held *pbPreprocessHeld) (_ *unumeral.Rational, result error) {
	var owned = func() (v unumeral.OwnedRational, e error) {
		if coefficient == nil {
			return t.manager.PseudoBooleanBound(value)
		}
		return t.manager.PseudoBooleanCoefficient(value, *coefficient)
	}
	v, e := owned()
	if e != nil {
		return nil, e
	}
	defer func() { result = errors.Join(result, t.manager.NumeralService().Del(&v)) }()
	out := t.heldValue(held)
	return out, t.manager.NumeralService().Manager.Copy(out, &v.Raw)
}

func (t *PBPreprocessTactic) negated(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	m := t.manager
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return ast.ASTHandle{}, false, e
	}
	f, e := m.GetFamilyID(h)
	if e != nil || f != m.BasicFamilyID() {
		return ast.ASTHandle{}, false, e
	}
	op, e := m.GetDeclKind(h)
	if e != nil || op != ast.ASTOpNot {
		return ast.ASTHandle{}, false, e
	}
	a, e := m.GetArg(h, 0)
	return a, e == nil, e
}

func (t *PBPreprocessTactic) isConstant(h ast.ASTHandle) (bool, error) {
	m := t.manager
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	n, e := m.GetNumArgs(h)
	if e != nil || n != 0 {
		return false, e
	}
	f, e := m.GetFamilyID(h)
	return e == nil && f == ast.NullFamilyID, e
}

// unit is the constraint 1*h >= 1.
func (t *PBPreprocessTactic) unit(h ast.ASTHandle, held *pbPreprocessHeld) (*pbPreprocessConstraint, error) {
	c, e := t.heldOne(held)
	if e != nil {
		return nil, e
	}
	k, e := t.heldOne(held)
	if e != nil {
		return nil, e
	}
	return &pbPreprocessConstraint{terms: []pbPreprocessTerm{{h, c}}, bound: k}, nil
}

// parse reads h as a pseudo-Boolean constraint whose rationals held owns.
func (t *PBPreprocessTactic) parse(h ast.ASTHandle, held *pbPreprocessHeld) (*pbPreprocessConstraint, error) {
	m := t.manager
	if yes, e := t.isConstant(h); e != nil {
		return nil, e
	} else if yes {
		return t.unit(h, held)
	}
	if child, yes, e := t.negated(h); e != nil {
		return nil, e
	} else if yes {
		if constant, x := t.isConstant(child); x != nil || !constant {
			return nil, x
		}
		return t.unit(h, held)
	}
	ge, e := m.IsPseudoBooleanGE(h)
	if e != nil {
		return nil, e
	}
	if ge {
		bound, x := t.rational(h, nil, held)
		if x != nil {
			return nil, x
		}
		n, x := m.GetNumArgs(h)
		if x != nil {
			return nil, x
		}
		result := &pbPreprocessConstraint{bound: bound}
		for i := uint32(0); i < n; i++ {
			a, y := m.GetArg(h, i)
			if y != nil {
				return nil, y
			}
			if ok, y := t.pureLiteral(a); y != nil || !ok {
				return nil, y
			}
			c, y := t.rational(h, &i, held)
			if y != nil {
				return nil, y
			}
			result.terms = append(result.terms, pbPreprocessTerm{a, c})
		}
		return result, nil
	}
	f, e := m.GetFamilyID(h)
	if e != nil {
		return nil, e
	}
	op, e := m.GetDeclKind(h)
	if e != nil {
		return nil, e
	}
	if f != m.BasicFamilyID() || op != ast.ASTOpOr {
		return nil, nil
	}
	n, e := m.GetNumArgs(h)
	if e != nil {
		return nil, e
	}
	one, e := t.heldOne(held)
	if e != nil {
		return nil, e
	}
	result := &pbPreprocessConstraint{bound: one}
	for i := uint32(0); i < n; i++ {
		a, x := m.GetArg(h, i)
		if x != nil {
			return nil, x
		}
		if ok, x := t.pureLiteral(a); x != nil || !ok {
			return nil, x
		}
		c, x := t.heldOne(held)
		if x != nil {
			return nil, x
		}
		result.terms = append(result.terms, pbPreprocessTerm{a, c})
	}
	return result, nil
}

// pureLiteral is the per-argument test of pure_args: strip one not, then
// accept an uninterpreted constant, true or false (so (not false) is pure).
func (t *PBPreprocessTactic) pureLiteral(h ast.ASTHandle) (bool, error) {
	child, yes, e := t.negated(h)
	if e != nil {
		return false, e
	}
	if yes {
		h = child
	}
	if c, e := t.isConstant(h); e != nil || c {
		return c, e
	}
	return t.manager.IsTrue(h) || t.manager.IsFalse(h), nil
}
