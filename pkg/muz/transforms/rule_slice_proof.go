package transforms

// Source: src/muz/transforms/dl_mk_slice.cpp, slice_proof_converter's
// original-rule map and asserted-proof translation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// SliceProofConverter owns the rule and formula mappings created by slicing.
// ConvertAsserted handles assertion leaves; Convert also handles supported
// hyper-resolution nodes, including quantified conclusions.
type SliceProofConverter struct {
	manager   *ast.ASTManager
	rules     *base.RuleManager
	originals []*base.Rule
	formulas  []ast.ASTRef
	byFormula map[ast.ASTHandle]sliceOriginalRule
}

type sliceOriginalRule struct {
	rule      *base.Rule
	variables []uint32
}

func newSliceProofConverter(manager *ast.ASTManager, rules *base.RuleManager) *SliceProofConverter {
	return &SliceProofConverter{manager: manager, rules: rules, byFormula: make(map[ast.ASTHandle]sliceOriginalRule)}
}

func (c *SliceProofConverter) insert(original, sliced *base.Rule, variables []uint32) (resultErr error) {
	if c == nil || c.manager == nil || original == nil || sliced == nil {
		return ast.ErrASTHandle
	}
	formula, err := sliced.ToFormula()
	if err != nil {
		return err
	}
	defer func() {
		if formula != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&formula))
		}
	}()
	fact, err := c.manager.RefGet(formula)
	if err != nil {
		return err
	}
	owner, err := original.Retain()
	if err != nil {
		return err
	}
	c.originals = append(c.originals, owner)
	c.formulas = append(c.formulas, formula)
	formula = ast.ASTRef{}
	c.byFormula[fact] = sliceOriginalRule{rule: owner, variables: append([]uint32(nil), variables...)}
	return nil
}

// OriginalVariables maps each sliced rule variable index to its original index.
// The returned slice is independent of the converter's internal mapping.
func (c *SliceProofConverter) OriginalVariables(proof ast.ASTHandle) ([]uint32, bool, error) {
	if c == nil || c.manager == nil {
		return nil, false, ast.ErrASTHandle
	}
	var fact ast.ASTHandle
	matched, err := c.manager.MatchProofAsserted(proof, &fact)
	if err != nil || !matched {
		return nil, false, err
	}
	entry, found := c.byFormula[fact]
	if !found {
		return nil, false, nil
	}
	return append([]uint32(nil), entry.variables...), true, nil
}

// ConvertAsserted maps an asserted sliced rule to the original rule's proof.
// The returned reference is owned by the caller when matched is true.
func (c *SliceProofConverter) ConvertAsserted(proof ast.ASTHandle) (result ast.ASTRef, matched bool, err error) {
	if c == nil || c.manager == nil {
		return result, false, ast.ErrASTHandle
	}
	var fact ast.ASTHandle
	matched, err = c.manager.MatchProofAsserted(proof, &fact)
	if err != nil || !matched {
		return result, false, err
	}
	entry, found := c.byFormula[fact]
	if !found {
		return result, false, nil
	}
	original, found, err := entry.rule.Proof()
	if err != nil {
		return result, false, err
	}
	if !found {
		return result, false, ast.ErrASTDomain
	}
	result, err = c.manager.NewRef(original)
	return result, err == nil, err
}

func (c *SliceProofConverter) Close() (result error) {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	for i := len(c.formulas) - 1; i >= 0; i-- {
		result = errors.Join(result, c.manager.DelRef(&c.formulas[i]))
	}
	for i := len(c.originals) - 1; i >= 0; i-- {
		result = errors.Join(result, c.originals[i].Close())
	}
	c.manager = nil
	c.rules = nil
	c.originals = nil
	c.formulas = nil
	c.byFormula = nil
	return result
}
