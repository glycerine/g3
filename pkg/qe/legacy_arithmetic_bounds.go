package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::find_max.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// evaluateRat evaluates expression to a numeral and writes its value into
// out (a rational of q()).
func (p *legacyArithmeticProjection) evaluateRat(evaluator *model.ModelEvaluator, expression ast.ASTHandle, out *unumeral.Rational) (resultErr error) {
	value, err := evaluator.EvaluateValue(expression)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, p.syntax.manager.DelRef(&value))
	}()
	handle, err := p.syntax.manager.RefGet(value)
	if err != nil {
		return err
	}
	numeral, err := p.syntax.numeral(handle, out)
	if err != nil {
		return err
	}
	if !numeral {
		return ast.ErrASTDomain
	}
	return nil
}

func (p *legacyArithmeticProjection) findMax(md *model.ModelCore, variable ast.ASTHandle, literals []*legacyLinearLiteral, positive bool) (_ int, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return -1, ast.ErrASTHandle
	}
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
		Context:     p.syntax.config.Context,
		Globals:     p.syntax.config.Globals,
		Memory:      p.syntax.manager.NumeralService().RuntimeOwner(),
		Cooperation: p.syntax.config.Cooperation,
		Trace:       p.syntax.config.Trace,
	})
	if err != nil {
		return -1, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, evaluator.Close())
	}()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return -1, err
	}
	q := p.q()
	variableValue, foundValue, value, abs, atModel := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() {
		resultErr = errors.Join(resultErr, q.Del(&variableValue), q.Del(&foundValue), q.Del(&value), q.Del(&abs), q.Del(&atModel))
	}()
	if err = p.evaluateRat(evaluator, variable, &variableValue); err != nil {
		return -1, err
	}
	found := -1
	foundStrict := false
	for index, literal := range literals {
		if literal == nil || literal.manager != p.syntax.manager || literal.equality {
			continue
		}
		sign, err := p.sign(&literal.coefficient)
		if err != nil {
			return -1, err
		}
		if sign == 0 || (sign > 0) != positive {
			continue
		}
		term, err := p.syntax.manager.RefGet(literal.term)
		if err != nil {
			return -1, err
		}
		if err = p.evaluateRat(evaluator, term, &value); err != nil {
			return -1, err
		}
		if err = errors.Join(q.Copy(&abs, &literal.coefficient), q.Abs(&abs)); err != nil {
			return -1, err
		}
		if err = q.Div(&value, &abs, &value); err != nil {
			return -1, err
		}
		if positive {
			err = q.Add(&value, &variableValue, &atModel)
		} else {
			err = q.Sub(&value, &variableValue, &atModel)
		}
		if err != nil {
			return -1, err
		}
		atSign, err := q.Sign(&atModel)
		if err != nil {
			return -1, err
		}
		trueInModel := atSign < 0 || (!literal.strict && atSign == 0)
		if !trueInModel {
			continue
		}
		better := found < 0
		if !better {
			c, err := q.Compare(&value, &foundValue)
			if err != nil {
				return -1, err
			}
			better = c > 0 || (c == 0 && !foundStrict && literal.strict)
		}
		if better {
			found = index
			if err = q.Copy(&foundValue, &value); err != nil {
				return -1, err
			}
			foundStrict = literal.strict
		}
	}
	if found < 0 {
		return -1, ast.ErrASTDomain
	}
	return found, nil
}
