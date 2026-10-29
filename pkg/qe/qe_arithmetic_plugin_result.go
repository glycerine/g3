package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticPluginBranch struct {
	Result      ast.ASTRef
	Definition  ast.ASTRef
	Coefficient *big.Rat
	Selection   ArithmeticBranchSelection
}

func (b *ArithmeticPluginBranch) close(m *ast.ASTManager) error {
	if b == nil {
		return nil
	}
	var definitionErr error
	if b.Definition != (ast.ASTRef{}) {
		definitionErr = m.DelRef(&b.Definition)
	}
	return errors.Join(definitionErr, m.DelRef(&b.Result))
}

func (p *ArithmeticPlugin) storeBranch(key ArithmeticBranchKey, result, definition ast.ASTRef, selection ArithmeticBranchSelection) error {
	if old := p.branches[key]; old != nil {
		if err := old.close(p.manager); err != nil {
			return err
		}
	}
	p.branches[key] = &ArithmeticPluginBranch{Result: result, Definition: definition, Selection: selection, Coefficient: big.NewRat(1, 1)}
	return nil
}

func (p *ArithmeticPlugin) branch(variable ast.ASTHandle, formula ast.ASTHandle, value *big.Rat) (*ArithmeticPluginBranch, error) {
	branch, err := arithmeticBranchUnsigned(value)
	if err != nil {
		return nil, err
	}
	key := ArithmeticBranchKey{Formula: formula, Variable: variable, Branch: branch}
	b := p.branches[key]
	if b == nil {
		return nil, ast.ErrASTDomain
	}
	return b, nil
}

func arithmeticBranchUnsigned(value *big.Rat) (uint32, error) {
	if value == nil || !value.IsInt() || value.Sign() < 0 || !value.Num().IsUint64() {
		return 0, ast.ErrASTDomain
	}
	v := value.Num().Uint64()
	if v > uint64(^uint32(0)) {
		return 0, ast.ErrASTDomain
	}
	return uint32(v), nil
}
