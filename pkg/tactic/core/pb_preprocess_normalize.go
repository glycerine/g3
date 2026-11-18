package core

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (t *PBPreprocessTactic) ownedRational(q *unumeral.Rational) (unumeral.OwnedRational, error) {
	v, e := t.manager.NumeralService().NewValue()
	if e != nil {
		return v, e
	}
	if e = t.manager.NumeralService().Manager.Copy(&v.Raw, q); e != nil {
		return v, errors.Join(e, t.manager.NumeralService().Del(&v))
	}
	return v, nil
}

func (t *PBPreprocessTactic) makeGE(terms []pbPreprocessTerm, bound *unumeral.Rational) (_ ast.ASTHandle, result error) {
	coefficients := make([]unumeral.OwnedRational, len(terms))
	args := make([]ast.ASTHandle, len(terms))
	created := 0
	defer func() {
		for i := created - 1; i >= 0; i-- {
			result = errors.Join(result, t.manager.NumeralService().Del(&coefficients[i]))
		}
	}()
	for i, term := range terms {
		var e error
		coefficients[i], e = t.ownedRational(term.coefficient)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		created++
		args[i] = term.expression
	}
	k, e := t.ownedRational(bound)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	defer func() { result = errors.Join(result, t.manager.NumeralService().Del(&k)) }()
	return t.manager.MkPseudoBooleanGE(coefficients, args, &k)
}

func (t *PBPreprocessTactic) negateLiteral(h ast.ASTHandle) (ast.ASTHandle, error) {
	if child, yes, e := t.negated(h); e != nil {
		return ast.ASTHandle{}, e
	} else if yes {
		return child, nil
	}
	return t.manager.MkNot(h)
}

func (t *PBPreprocessTactic) normalize(g *tactic.Goal) (resultErr error) {
	q := t.manager.NumeralService().Manager
	var held pbPreprocessHeld
	defer func() { resultErr = errors.Join(resultErr, t.releaseHeld(&held)) }()
	// C++ pb_preprocess_tactic::normalize: for (i = 0; !g->inconsistent() && i < g->size(); ++i).
	for i := uint32(0); ; i++ {
		inconsistent, e := g.Inconsistent()
		if e != nil {
			return e
		}
		if inconsistent {
			break
		}
		n, e := g.Size()
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		h, x := g.Form(i)
		if x != nil {
			return x
		}
		child, negated, x := t.negated(h)
		if x != nil {
			return x
		}
		if !negated {
			continue
		}
		ge, x := t.manager.IsPseudoBooleanGE(child)
		if x != nil {
			return x
		}
		if !ge {
			continue
		}
		// Read k, the coefficients and the arguments straight from the pb
		// app, for any arguments (C++ reads pb.get_k / pb.get_coeff and
		// negate(arg) without requiring literal arguments).
		if x = t.releaseHeld(&held); x != nil {
			return x
		}
		k, x := t.rational(child, nil, &held)
		if x != nil {
			return x
		}
		numArgs, x := t.manager.GetNumArgs(child)
		if x != nil {
			return x
		}
		sum := t.heldValue(&held)
		terms := make([]pbPreprocessTerm, numArgs)
		for j := uint32(0); j < numArgs; j++ {
			c, y := t.rational(child, &j, &held)
			if y != nil {
				return y
			}
			if y = q.Add(sum, c, sum); y != nil {
				return y
			}
			terms[j].coefficient = c
			a, y := t.manager.GetArg(child, j)
			if y != nil {
				return y
			}
			terms[j].expression, y = t.negateLiteral(a)
			if y != nil {
				return y
			}
		}
		bound := t.heldValue(&held)
		if x = errors.Join(q.Sub(sum, k, bound), q.Inc(bound)); x != nil {
			return x
		}
		next, x := t.makeGE(terms, bound)
		if x != nil {
			return x
		}
		pr, x := g.Proof(i)
		if x != nil {
			return x
		}
		dep, x := g.Dependency(i)
		if x != nil {
			return x
		}
		if x = g.Update(i, next, pr, dep); x != nil {
			return x
		}
	}
	return nil
}
