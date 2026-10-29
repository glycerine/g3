package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticCooperRuntime struct {
	plugin   *ArithmeticPlugin
	variable ast.ASTHandle
	common   *big.Int
	residue  ast.ASTHandle
	fresh    []ast.ASTRef
}

func (p *ArithmeticPlugin) newCooperRuntime(variable ast.ASTHandle, processor *ArithmeticBoundsProcessor) (*ArithmeticCooperRuntime, bool, error) {
	all := append(processor.Divisibilities(false), processor.Divisibilities(true)...)
	if len(all) == 0 {
		return nil, false, nil
	}
	common, err := cooperLCM(all)
	if err != nil {
		return nil, false, err
	}
	if common.Cmp(big.NewInt(1)) == 0 {
		return nil, false, nil
	}
	sort, err := p.manager.GetSort(variable)
	if err != nil {
		return nil, false, err
	}
	z, err := p.manager.MkFreshConst("qe-cooper-z", sort)
	if err != nil {
		return nil, false, err
	}
	ref, err := p.manager.NewRef(z)
	if err != nil {
		return nil, false, err
	}
	runtime := &ArithmeticCooperRuntime{plugin: p, variable: variable, common: common, residue: z, fresh: []ast.ASTRef{ref}}
	return runtime, true, nil
}
func (r *ArithmeticCooperRuntime) publish() error {
	r.plugin.pendingFresh = append(r.plugin.pendingFresh, r.fresh...)
	r.fresh = nil
	return nil
}
func (r *ArithmeticCooperRuntime) close() (result error) {
	if r == nil {
		return nil
	}
	for i := len(r.fresh) - 1; i >= 0; i-- {
		result = errors.Join(result, r.plugin.manager.DelRef(&r.fresh[i]))
	}
	r.fresh = nil
	return result
}
