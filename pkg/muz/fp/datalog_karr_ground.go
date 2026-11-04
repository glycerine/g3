package fp

// Source: dl_compiler.cpp make_assembling_code builds no-tail ground heads
// from unary singleton relations, joining successive constants; a nullary
// head uses a full zero-column relation.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type karrGroundRule struct{ values []*big.Rat }

func (a *karrDatalogActivation) compileGroundRule(rule *base.Rule) (ast.ASTHandle, karrGroundRule, error) {
	var none ast.ASTHandle
	var out karrGroundRule
	n, err := rule.TailSize()
	if err != nil {
		return none, out, err
	}
	if n != 0 {
		return none, out, ErrKarrDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return none, out, err
	}
	decl, err := a.manager.GetDecl(head)
	if err != nil {
		return none, out, err
	}
	if a.seeds[decl] == nil {
		return none, out, ErrKarrDatalogFragment
	}
	arity, err := a.manager.GetNumArgs(head)
	if err != nil {
		return none, out, err
	}
	for i := uint32(0); i < arity; i++ {
		arg, err := a.manager.GetArg(head, i)
		if err != nil {
			return none, out, err
		}
		value, _, matched, err := a.manager.ArithmeticNumeralText(arg)
		if err != nil {
			return none, out, err
		}
		if !matched {
			return none, out, ErrKarrDatalogFragment
		}
		num, ok := new(big.Rat).SetString(value)
		if !ok {
			return none, out, ast.ErrASTDomain
		}
		out.values = append(out.values, num)
	}
	return decl, out, nil
}

func (r karrGroundRule) apply(checkpoint func() bool) (*rel.KarrRelation, error) {
	if len(r.values) == 0 {
		return rel.NewKarrFull(0, checkpoint), nil
	}
	var result *rel.KarrRelation
	for _, value := range r.values {
		singleton := rel.NewKarrEmpty(1, checkpoint)
		if err := singleton.AddFact([]*big.Rat{value}); err != nil {
			return nil, err
		}
		if result == nil {
			result = singleton
			continue
		}
		var err error
		result, err = rel.KarrJoin(result, singleton, nil, nil)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
