package transforms

// Source: src/muz/transforms/dl_mk_slice.cpp, translate_proof and
// translate_hyper_res. This stage reconstructs original rules by unification.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

type slicedProofResult struct {
	proof ast.ASTRef
	rule  *base.Rule
}

// Convert translates asserted leaves and supported hyper-resolution nodes to
// proofs over original rules. It returns an owned proof. As in the source
// converter, resolution uses rule unification. Substitutions on asserted rules
// use Horn conversion for compound predicate heads; substitutions on derived
// premises and resolution against a premise other than a positive rule head
// remain unsupported.
func (c *SliceProofConverter) Convert(proof ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || c.rules == nil {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
		}
	}()
	memo := make(map[ast.ASTHandle]slicedProofResult)
	defer func() {
		for _, v := range memo {
			if v.rule != nil {
				resultErr = errors.Join(resultErr, v.rule.Close())
			}
			if v.proof != (ast.ASTRef{}) {
				ref := v.proof
				resultErr = errors.Join(resultErr, m.DelRef(&ref))
			}
		}
	}()
	var translate func(ast.ASTHandle) (slicedProofResult, error)
	translate = func(node ast.ASTHandle) (value slicedProofResult, nodeErr error) {
		if v, ok := memo[node]; ok {
			return v, nil
		}
		defer func() {
			if nodeErr == nil {
				memo[node] = value
			} else {
				if value.rule != nil {
					nodeErr = errors.Join(nodeErr, value.rule.Close())
				}
				if value.proof != (ast.ASTRef{}) {
					nodeErr = errors.Join(nodeErr, m.DelRef(&value.proof))
				}
			}
		}()
		var fact ast.ASTHandle
		asserted, e := m.MatchProofAsserted(node, &fact)
		if e != nil {
			return value, e
		}
		if asserted {
			if original, found := c.byFormula[fact]; found {
				value.proof, _, e = c.ConvertAsserted(node)
				if e != nil {
					return value, e
				}
				value.rule, e = original.rule.Retain()
				return value, e
			}
		}
		isHyper, e := m.IsProofKind(node, ast.ASTPrHyperResolve)
		if e != nil {
			return value, e
		}
		if !isHyper {
			value.proof, e = m.NewRef(node)
			return value, e
		}
		conclusion, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return value, e
		}
		defer func() { nodeErr = errors.Join(nodeErr, m.DelRef(&conclusion)) }()
		output, e := ast.NewASTProofHyperOutput(m)
		if e != nil {
			return value, e
		}
		defer func() { nodeErr = errors.Join(nodeErr, output.Close()) }()
		var premises []ast.ASTHandle
		defer func() {
			for _, premise := range premises {
				nodeErr = errors.Join(nodeErr, m.DecRef(premise))
			}
		}()
		matched, e := m.MatchProofHyperResolve(node, &premises, conclusion, output)
		if e != nil || !matched || len(premises) == 0 {
			return value, errors.Join(ast.ErrASTDomain, e)
		}
		var instantiated []slicedProofResult
		defer func() {
			for _, temporary := range instantiated {
				nodeErr = errors.Join(nodeErr, temporary.rule.Close())
				ref := temporary.proof
				nodeErr = errors.Join(nodeErr, m.DelRef(&ref))
			}
		}()
		prepare := func(index int) (slicedProofResult, error) {
			original, cause := translate(premises[index])
			if cause != nil {
				return original, cause
			}
			substitution, cause := output.Substitution(uint32(index))
			if cause != nil || len(substitution) == 0 {
				return original, cause
			}
			variables, found, cause := c.OriginalVariables(premises[index])
			if cause != nil {
				return slicedProofResult{}, cause
			}
			if !found || original.rule == nil {
				return slicedProofResult{}, fmt.Errorf("slice proof: substitution for derived premise is not implemented")
			}
			rule, ref, cause := c.instantiateAsserted(original.rule, original.proof, variables, substitution)
			if cause != nil {
				return slicedProofResult{}, cause
			}
			value := slicedProofResult{proof: ref, rule: rule}
			instantiated = append(instantiated, value)
			return value, nil
		}
		first, e := prepare(0)
		if e != nil {
			return value, e
		}
		if first.rule == nil {
			return value, fmt.Errorf("slice proof: first premise has no original rule")
		}
		value.proof, e = m.CopyRef(first.proof)
		if e != nil {
			return value, e
		}
		value.rule, e = first.rule.Retain()
		if e != nil {
			return value, e
		}
		originalPositive, e := first.rule.PositiveTailSize()
		if e != nil {
			return value, e
		}
		removed := make(map[uint32]bool)
		for i := 1; i < len(premises); i++ {
			other, cause := prepare(i)
			if cause != nil {
				return value, cause
			}
			if other.rule == nil {
				return value, fmt.Errorf("slice proof: premise %d has no original rule", i)
			}
			position, cause := output.Position(uint32(i - 1))
			if cause != nil {
				return value, cause
			}
			if position.First == 0 || position.Second != 0 {
				return value, fmt.Errorf("slice proof: unsupported resolution positions")
			}
			originalIndex := position.First - 1
			if originalIndex >= uint32(originalPositive) || removed[originalIndex] {
				return value, fmt.Errorf("slice proof: invalid or repeated body position")
			}
			index := originalIndex
			for prior := range removed {
				if prior < originalIndex {
					index--
				}
			}
			removed[originalIndex] = true
			before, cause := value.rule.PositiveTailSize()
			if cause != nil {
				return value, cause
			}
			added, cause := other.rule.PositiveTailSize()
			if cause != nil {
				return value, cause
			}
			next, nextProof, cause := c.resolveProof(value.rule, value.proof, other.rule, other.proof, int(index))
			if cause != nil {
				return value, cause
			}
			after, cause := next.PositiveTailSize()
			if cause != nil || after != before-1+added {
				cause = errors.Join(cause, fmt.Errorf("slice proof: resolution merged body predicates"), next.Close(), m.DelRef(&nextProof))
				return value, cause
			}
			cause = errors.Join(value.rule.Close(), m.DelRef(&value.proof))
			value.rule, value.proof = next, nextProof
			if cause != nil {
				return value, cause
			}
		}
		return value, nil
	}
	converted, err := translate(proof)
	if err != nil {
		return result, err
	}
	return m.CopyRef(converted.proof)
}

func (c *SliceProofConverter) resolveProof(first *base.Rule, firstProof ast.ASTRef, second *base.Rule, secondProof ast.ASTRef, tailIndex int) (rule *base.Rule, proof ast.ASTRef, resultErr error) {
	m := c.manager
	defer func() {
		if resultErr != nil {
			if rule != nil {
				resultErr = errors.Join(resultErr, rule.Close())
				rule = nil
			}
			if proof != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(&proof))
			}
		}
	}()
	u, err := NewRuleUnifier(m)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.Close()) }()
	ok, err := u.UnifyRules(first, tailIndex, second)
	if err != nil || !ok {
		return nil, proof, errors.Join(ast.ErrASTDomain, err)
	}
	a, err := u.RuleSubstitution(first, true)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, a)) }()
	b, err := u.RuleSubstitution(second, false)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, b)) }()
	substitutions := make([][]ast.ASTHandle, 2)
	for index, refs := range [][]ast.ASTRef{a, b} {
		substitutions[index] = make([]ast.ASTHandle, len(refs))
		for j, ref := range refs {
			substitutions[index][j], err = m.RefGet(ref)
			if err != nil {
				return nil, proof, err
			}
		}
	}
	var renaming []ast.ASTRef
	rule, renaming, err = u.ResolveRawWithRenaming(c.rules, first, tailIndex, second)
	if err != nil {
		return nil, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, renaming)) }()
	if len(renaming) != 0 {
		mapping := make([]ast.ASTHandle, len(renaming))
		for i, ref := range renaming {
			if ref != (ast.ASTRef{}) {
				mapping[i], err = m.RefGet(ref)
				if err != nil {
					return rule, proof, err
				}
			}
		}
		sub, e := rewriter.NewVarSubstitution(m, false)
		if e != nil {
			if sub != nil {
				e = errors.Join(e, sub.Close())
			}
			return rule, proof, e
		}
		defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
		var rewritten []ast.ASTRef
		defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(m, rewritten)) }()
		for i := range substitutions {
			for j, term := range substitutions[i] {
				ref, e := m.NewRef(ast.ASTHandle{})
				if e != nil {
					return rule, proof, e
				}
				rewritten = append(rewritten, ref)
				if e = sub.Substitute(term, mapping, ref); e != nil {
					return rule, proof, e
				}
				substitutions[i][j], e = m.RefGet(ref)
				if e != nil {
					return rule, proof, e
				}
			}
		}
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return rule, proof, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&formula)) }()
	fact, err := m.RefGet(formula)
	if err != nil {
		return rule, proof, err
	}
	left, err := m.RefGet(firstProof)
	if err != nil {
		return rule, proof, err
	}
	right, err := m.RefGet(secondProof)
	if err != nil {
		return rule, proof, err
	}
	position := ast.ASTProofPosition{First: uint32(tailIndex + 1), Second: 0}
	node, err := m.MkProofHyperResolve([]ast.ASTHandle{left, right}, fact, []ast.ASTProofPosition{position}, substitutions)
	if err != nil {
		return rule, proof, err
	}
	proof, err = m.NewRef(node)
	return rule, proof, err
}
