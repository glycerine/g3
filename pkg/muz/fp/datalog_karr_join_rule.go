package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type karrBinaryRule struct {
	left, right ast.ASTHandle
	join        karrJoinPlan
	head        karrHeadPlan
}

func (a *karrDatalogActivation) compileBinaryRule(rule *base.Rule) (ast.ASTHandle, karrBinaryRule, error) {
	var none ast.ASTHandle
	var out karrBinaryRule
	n, err := rule.TailSize()
	if err != nil {
		return none, out, err
	}
	if n != 2 {
		return none, out, ErrKarrDatalogFragment
	}
	n, err = rule.UninterpretedTailSize()
	if err != nil {
		return none, out, err
	}
	if n != 2 {
		return none, out, ErrKarrDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return none, out, err
	}
	left, negated, err := rule.Tail(0)
	if err != nil {
		return none, out, err
	}
	if negated {
		return none, out, ErrKarrDatalogFragment
	}
	right, negated, err := rule.Tail(1)
	if err != nil {
		return none, out, err
	}
	if negated {
		return none, out, ErrKarrDatalogFragment
	}
	hd, err := a.manager.GetDecl(head)
	if err != nil {
		return none, out, err
	}
	out.left, err = a.manager.GetDecl(left)
	if err != nil {
		return none, out, err
	}
	out.right, err = a.manager.GetDecl(right)
	if err != nil {
		return none, out, err
	}
	if a.seeds[hd] == nil || a.seeds[out.left] == nil || a.seeds[out.right] == nil {
		return none, out, ErrKarrDatalogFragment
	}
	out.join, err = a.compileKarrJoin(head, left, right)
	if err != nil {
		return none, out, err
	}
	// The native compiler traverses an internal map when emitting local
	// duplicate filters. Their order can change Karr's generator ordering;
	// admit at most one until that observable ordering can be preserved.
	if len(out.join.identical) > 1 {
		return none, out, ErrKarrDatalogFragment
	}
	out.head, err = a.compileKarrHead(head, out.join.expressions)
	if err != nil {
		return none, out, err
	}
	return hd, out, nil
}

func (a *karrDatalogActivation) applyBinaryRule(r karrBinaryRule, left, right *rel.KarrRelation) (*rel.KarrRelation, error) {
	result, err := r.join.apply(left, right)
	if err != nil {
		return nil, err
	}
	if err = a.checkpoint(); err != nil {
		return nil, err
	}
	result, err = r.head.apply(result)
	if err != nil {
		return nil, err
	}
	if err = a.checkpoint(); err != nil {
		return nil, err
	}
	return result, nil
}
