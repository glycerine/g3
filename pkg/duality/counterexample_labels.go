package duality

// Source: duality_rpfp.cpp, RPFP::GetLabelsRec/GetLabels.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// GetLabels selects source labels along the model's first satisfying Boolean
// branch. Symbols are borrowed from the graph's symbol table.
func (g *RPFP) GetLabels(edge *Edge) (labels []usymbol.Symbol, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if edge == nil || edge.closed || edge.Owner != g {
		return nil, ErrWrapperOwner
	}
	if edge.Map == nil {
		return nil, nil
	}
	source := edge.Map
	if source.closed {
		return nil, ErrWrapperOwner
	}
	if source.labeled == (ast.ASTRef{}) {
		return nil, nil
	}
	h, err := g.context.manager.RefGet(source.labeled)
	if err != nil {
		return nil, err
	}
	if h == (ast.ASTHandle{}) {
		return nil, nil
	}
	term, err := NewExpr(g.context, h)
	if err != nil {
		return nil, err
	}
	defer func() {
		result = errors.Join(result, term.Close())
		if result != nil {
			labels = nil
		}
	}()
	local, err := g.Localize(edge, term)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, local.Close()) }()
	root, err := local.Raw()
	if err != nil {
		return nil, err
	}
	return g.labelsForFormula(root)
}

func (g *RPFP) labelsForFormula(root ast.ASTHandle) (labels []usymbol.Symbol, result error) {
	m := g.context.manager
	memo := make(map[ast.ASTHandle]int)
	done := [2]map[ast.ASTHandle]bool{make(map[ast.ASTHandle]bool), make(map[ast.ASTHandle]bool)}
	var visit func(ast.ASTHandle, bool) error
	visit = func(h ast.ASTHandle, truth bool) error {
		polarity := 0
		if truth {
			polarity = 1
		}
		if done[polarity][h] {
			return nil
		}
		done[polarity][h] = true
		kind, err := m.GetKind(h)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			return nil
		}
		family, err := m.GetFamilyID(h)
		if err != nil {
			return err
		}
		if family == m.BasicFamilyID() {
			op, err := m.GetDeclKind(h)
			if err != nil {
				return err
			}
			n, err := m.GetNumArgs(h)
			if err != nil {
				return err
			}
			arg := func(i uint32) (ast.ASTHandle, error) { return m.GetArg(h, i) }
			switch op {
			case ast.ASTOpNot:
				a, err := arg(0)
				if err != nil {
					return err
				}
				return visit(a, !truth)
			case ast.ASTOpIff:
				a, err := arg(0)
				if err != nil {
					return err
				}
				b, err := g.subtermTruth(memo, a)
				if err != nil {
					return err
				}
				if b == 2 {
					return nil
				}
				a, err = arg(1)
				if err != nil {
					return err
				}
				return visit(a, (b == 1) == truth)
			case ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpImplies:
				// Implication follows the source order: consequent, then negated antecedent.
				for i := uint32(0); i < n; i++ {
					index := i
					negated := false
					if op == ast.ASTOpImplies {
						index = 1 - i
						negated = i == 1
					}
					a, err := arg(index)
					if err != nil {
						return err
					}
					all := (op == ast.ASTOpAnd && truth) || (op != ast.ASTOpAnd && !truth)
					childTruth := truth != negated
					if !all {
						b, err := g.subtermTruth(memo, a)
						if err != nil {
							return err
						}
						if (childTruth && b != 1) || (!childTruth && b != 0) {
							continue
						}
					}
					if err := visit(a, childTruth); err != nil {
						return err
					}
					if !all {
						return nil
					}
				}
				return nil
			}
		}
		var positive bool
		var names []usymbol.Symbol
		labeled, err := m.IsLabel(h, &positive, &names)
		if err != nil {
			return err
		}
		if labeled {
			a, err := m.GetArg(h, 0)
			if err != nil {
				return err
			}
			if err := visit(a, truth); err != nil {
				return err
			}
			if positive == truth {
				labels = append(labels, names...)
			}
		}
		return nil
	}
	result = visit(root, true)
	if result != nil {
		labels = nil
	}
	return
}
