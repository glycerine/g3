package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// ArithmeticPluginBranch is the m_subst branch_formula entry: the branch
// result, the x_t substitution (coefficient and term, nil without Cooper),
// and the selection get_def reads when subst asks for a definition.
type ArithmeticPluginBranch struct {
	Result      ast.ASTRef
	Coefficient *big.Rat
	Term        ast.ASTRef
	Selection   ArithmeticBranchSelection
	Fresh       []ast.ASTRef
}

// substitution rebuilds x_t from the cached coefficient and term.
func (b *ArithmeticPluginBranch) substitution(m *ast.ASTManager) (*arithmeticBranchSubstitution, error) {
	if b.Term == (ast.ASTRef{}) {
		return nil, nil
	}
	term, err := m.RefGet(b.Term)
	if err != nil {
		return nil, err
	}
	return &arithmeticBranchSubstitution{coefficient: new(big.Rat).Set(b.Coefficient), term: term}, nil
}

// close releases the fresh-variable copies; Result and Term are owned by
// the plugin trail (add_cache).
func (b *ArithmeticPluginBranch) close(m *ast.ASTManager) error {
	if b == nil {
		return nil
	}
	var definitionErr error
	for i := len(b.Fresh) - 1; i >= 0; i-- {
		definitionErr = errors.Join(definitionErr, m.DelRef(&b.Fresh[i]))
	}
	b.Fresh = nil
	return definitionErr
}

func (p *ArithmeticPlugin) storeBranch(key ArithmeticBranchKey, result ast.ASTRef, substitution *arithmeticBranchSubstitution, selection ArithmeticBranchSelection, fresh []ast.ASTRef) (err error) {
	var copies []ast.ASTRef
	var term ast.ASTRef
	defer func() {
		if err != nil {
			for i := len(copies) - 1; i >= 0; i-- {
				err = errors.Join(err, p.manager.DelRef(&copies[i]))
			}
			if term != (ast.ASTRef{}) {
				err = errors.Join(err, p.manager.DelRef(&term))
			}
		}
	}()
	coefficient := big.NewRat(1, 1)
	if substitution != nil {
		coefficient = new(big.Rat).Set(substitution.coefficient)
		if term, err = p.manager.NewRef(substitution.term); err != nil {
			return err
		}
	}
	for _, ref := range fresh {
		h, cause := p.manager.RefGet(ref)
		if cause != nil {
			return cause
		}
		copy, cause := p.manager.NewRef(h)
		if cause != nil {
			return cause
		}
		copies = append(copies, copy)
	}
	// add_cache: m_trail.push_back(x), (fml), (result), (term) and
	// m_subst.insert, which replaces an equal entry (the trail keeps the old
	// one's expressions).
	if err = p.pushTrail(key.Variable, key.Formula); err != nil {
		return err
	}
	p.trail = append(p.trail, result)
	if term != (ast.ASTRef{}) {
		p.trail = append(p.trail, term)
	}
	branch := &ArithmeticPluginBranch{Result: result, Coefficient: coefficient, Term: term, Selection: selection, Fresh: copies}
	p.branches[key] = branch
	p.branchOrder = append(p.branchOrder, branch)
	return nil
}

// replayBranch republishes the cached fresh variables without re-emitting the
// branch constraints, as the source get_cache path does.
func (p *ArithmeticPlugin) replayBranch(branch *ArithmeticPluginBranch) (err error) {
	var copies []ast.ASTRef
	defer func() {
		if err != nil {
			for i := len(copies) - 1; i >= 0; i-- {
				err = errors.Join(err, p.manager.DelRef(&copies[i]))
			}
		}
	}()
	for _, ref := range branch.Fresh {
		h, cause := p.manager.RefGet(ref)
		if cause != nil {
			return cause
		}
		copy, cause := p.manager.NewRef(h)
		if cause != nil {
			return cause
		}
		copies = append(copies, copy)
	}
	p.pendingFresh = append(p.pendingFresh, copies...)
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
