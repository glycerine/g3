package arith

// Source: src/tactic/arith/fm_tactic.cpp:180-244.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c *FMModelConverter) ApplyModel(out *tactic.ModelRef, _ uint32) (err error) {
	if e := c.open(); e != nil {
		return e
	}
	md, e := out.Get()
	if e != nil {
		return e
	}
	if md == nil || md.Manager() != c.manager {
		return ast.ErrASTHandle
	}
	if e = c.traceModel(md, 184, true); e != nil {
		return e
	}
	ev, e := model.NewModelEvaluator(md, c.services)
	if e != nil {
		if ev != nil {
			e = errors.Join(e, ev.Close())
		}
		return e
	}
	defer func() { err = errors.Join(err, ev.Close()) }()
	if e = ev.SetModelCompletion(true); e != nil {
		return e
	}
	ref, e := c.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, c.manager.DelRef(&ref)) }()
	// rational lower, upper, val, witness;
	q := c.manager.NumeralService().Manager
	lower, upper, value, witness := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&lower), q.Del(&upper), q.Del(&value), q.Del(&witness)) }()
	for i := len(c.variables) - 1; i >= 0; i-- {
		if e = c.traceVariable(c.variables[i], ast.ASTHandle{}); e != nil {
			return e
		}
		hasLower, hasUpper := false, false
		for _, clause := range c.clauses[i] {
			if !c.manager.ResourceLimit().Inc() {
				return tactic.NewTacticException(c.manager.ResourceLimit().CancelMessage())
			}
			kind, e := c.processBound(c.variables[i], clause, ev, ref, &value)
			if e != nil {
				return e
			}
			if e = c.traceBound(kind, &value, clause); e != nil {
				return e
			}
			switch kind {
			case fmLowerBound:
				better := !hasLower
				if hasLower {
					cmp, e := q.Compare(&value, &lower)
					if e != nil {
						return e
					}
					better = cmp > 0
				}
				if better {
					hasLower = true
					if e = q.Copy(&lower, &value); e != nil {
						return e
					}
				}
			case fmUpperBound:
				better := !hasUpper
				if hasUpper {
					cmp, e := q.Compare(&value, &upper)
					if e != nil {
						return e
					}
					better = cmp < 0
				}
				if better {
					hasUpper = true
					if e = q.Copy(&upper, &value); e != nil {
						return e
					}
				}
			}
		}
		sort, e := c.manager.GetRange(c.variables[i])
		if e != nil {
			return e
		}
		kind, e := c.manager.GetDeclKind(sort)
		if e != nil {
			return e
		}
		if kind == 1 {
			switch {
			case hasLower:
				e = q.Copy(&witness, &lower)
			case hasUpper:
				e = q.Copy(&witness, &upper)
			default:
				e = q.SetInt32(&witness, 0)
			}
		} else {
			switch {
			case hasLower && hasUpper:
				// (lower + upper) / rational(2)
				e = q.Add(&lower, &upper, &witness)
				if e == nil {
					e = q.SetInt32(&value, 2)
				}
				if e == nil {
					e = q.Div(&witness, &value, &witness)
				}
			case hasLower:
				e = errors.Join(q.Copy(&witness, &lower), q.Inc(&witness))
			case hasUpper:
				e = errors.Join(q.Copy(&witness, &upper), q.Dec(&witness))
			default:
				e = q.SetInt32(&witness, 0)
			}
		}
		if e != nil {
			return e
		}
		numeral, e := c.manager.ArithmeticNumeralFromRational(&witness, kind == 1)
		if e != nil {
			return e
		}
		if e = c.traceVariable(c.variables[i], numeral); e != nil {
			return e
		}
		if e = md.RegisterConstant(c.variables[i], numeral); e != nil {
			return e
		}
	}
	return c.traceModel(md, 243, false)
}
