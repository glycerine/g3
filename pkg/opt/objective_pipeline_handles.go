package opt

import (
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ObjectivePipeline) refHandles(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		var err error
		result[i], err = p.manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// restore is m_adjust_value(value) into a caller-owned rational.
func (o *pipelineObjective) restore(q *numeral.RationalManager, value *numeral.Rational) (numeral.Rational, error) {
	result := numeral.NewRational()
	var err error
	if value != nil {
		err = q.Copy(&result, value)
	}
	if err == nil && o.negate {
		err = q.Neg(&result)
	}
	if err == nil {
		err = q.Add(&result, &o.offset, &result)
	}
	return result, err
}
