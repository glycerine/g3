package fp

// Source: dl_compiler.cpp joins two positive tail registers before head
// assembly. Product join matches their actual DOC and table components.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type udocProductPairPlan struct {
	left, right               ast.ASTHandle
	leftColumns, rightColumns []uint32
}

func (e *PropositionalEngine) planUDocProductPair(program *udocProductProgram, rule *base.Rule) (*udocProductRulePlan, error) {
	var declarations [2]ast.ASTHandle
	var arguments [2][]ast.ASTHandle
	ready := true
	for i := range declarations {
		tail, negated, err := rule.Tail(i)
		if err != nil {
			return nil, err
		}
		if negated {
			return nil, ErrUDocDatalogFragment
		}
		declarations[i], err = e.manager.GetDecl(tail)
		if err != nil {
			return nil, err
		}
		if !program.productHead(declarations[i]) {
			if program.rules[declarations[i]] != nil || program.docs[declarations[i]] || program.finite[declarations[i]] {
				return nil, ErrUDocDatalogFragment
			}
			ready = false
		}
		arguments[i], err = productArguments(e.manager, tail)
		if err != nil {
			return nil, err
		}
		seen := make(map[ast.ASTHandle]bool)
		for _, argument := range arguments[i] {
			kind, err := e.manager.GetKind(argument)
			if err != nil {
				return nil, err
			}
			if kind == ast.ASTVar {
				if seen[argument] {
					return nil, ErrUDocDatalogFragment
				}
				seen[argument] = true
				continue
			}
			value, err := e.manager.IsValue(argument)
			if err != nil {
				return nil, err
			}
			if !value {
				return nil, ErrUDocDatalogFragment
			}
		}
	}
	if !ready {
		return nil, nil
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	declaration, err := rule.HeadDeclaration()
	if err != nil {
		return nil, err
	}
	pair := &udocProductPairPlan{left: declarations[0], right: declarations[1]}
	for i, left := range arguments[0] {
		kind, err := e.manager.GetKind(left)
		if err != nil {
			return nil, err
		}
		// Source variable_intersection only matches variables. Identical
		// constants in different tails are separate post-join filters.
		if kind != ast.ASTVar {
			continue
		}
		for j, right := range arguments[1] {
			if left == right {
				pair.leftColumns = append(pair.leftColumns, uint32(i))
				pair.rightColumns = append(pair.rightColumns, uint32(j))
			}
		}
	}
	plan := &udocProductRulePlan{head: declaration, pair: pair}
	joined := append(append([]ast.ASTHandle(nil), arguments[0]...), arguments[1]...)
	if err = planUDocProductAssembly(e.manager, plan, joined, head, true); err != nil {
		return nil, err
	}
	// Same-tail repeats remain outside this slice. Every remaining repeated
	// variable crosses the two tails, so source joins already enforce its
	// identity and the compiler skips the later FilterIdentical operation.
	plan.identical = nil
	return plan, nil
}

func (p *udocProductContext) productPairInput(declaration ast.ASTHandle, finite map[ast.ASTHandle]rel.FiniteTable) (*rel.UDocTableProduct, bool, error) {
	if product := p.result[declaration]; product != nil {
		empty, err := product.Empty()
		return product, empty, err
	}
	if ordinary := finite[declaration]; ordinary != nil {
		rows, err := ordinary.Len()
		return nil, rows == 0, err
	}
	return nil, true, nil
}

func (p *udocProductContext) evaluateProductPair(pair *udocProductPairPlan, finite map[ast.ASTHandle]rel.FiniteTable) (*rel.UDocTableProduct, error) {
	left, leftEmpty, err := p.productPairInput(pair.left, finite)
	if err != nil {
		return nil, err
	}
	right, rightEmpty, err := p.productPairInput(pair.right, finite)
	if err != nil {
		return nil, err
	}
	// A source join with an absent/empty input register performs no join.
	if leftEmpty || rightEmpty {
		return nil, nil
	}
	// An earlier empty query did not infer product kind. A later API fact can
	// therefore populate ordinary finite storage. That is a real nonempty
	// input, but mixed table/product dispatch is outside this paired slice.
	if left == nil || right == nil {
		return nil, fmt.Errorf("%w: two-product join has a nonempty ordinary finite input", ErrUDocDatalogFragment)
	}
	return left.Join(right, pair.leftColumns, pair.rightColumns)
}
