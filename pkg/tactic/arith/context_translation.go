package arith

// Source: translate methods for the arithmetic finite-domain preprocessing tactics.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TranslateContextLeaf(value tactic.Tactic, m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, services model.ModelEvaluatorConfig) (result tactic.Tactic, matched bool, resultErr error) {
	if t, ok := value.(*Eq2BVTactic); ok {
		if err := t.open(); err != nil {
			return nil, true, err
		}
		next, err := NewEq2BVTactic(m, c, g)
		if err != nil {
			return nil, true, err
		}
		return next, true, nil
	}
	var source *uparams.ParamsContext
	var params *uparams.ParamsRef
	switch t := value.(type) {
	case *LIA2PBTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *Card2BVTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *PB2BVTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	default:
		return nil, false, nil
	}
	p, err := source.TranslateTo(params, c)
	if err != nil {
		return nil, true, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && result != nil {
			if e := result.IncRef(); e == nil {
				resultErr = errors.Join(resultErr, result.DecRef())
			} else {
				resultErr = errors.Join(resultErr, e)
			}
			result = nil
		}
	}()
	switch t := value.(type) {
	case *LIA2PBTactic:
		next, e := NewLIA2PBTactic(m, c, g, &p, services)
		err = e
		if err == nil {
			next.trace = t.trace
			next.bounds.trace = t.trace
			result = next
		}
	case *Card2BVTactic:
		next, e := NewCard2BVTactic(m, c, g, &p)
		err = e
		if err == nil {
			result = next
		}
	case *PB2BVTactic:
		next, e := NewPB2BVTactic(m, c, g, &p, services)
		err = e
		if err == nil {
			next.trace = t.trace
			next.bounds.trace = t.trace
			result = next
		}
	}
	return result, true, err
}
