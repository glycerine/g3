package smtcore

// Source: src/smt/smt_model_finder.cpp direct variable qinfo constraints.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtModelFinderAUFLiteralKind uint8

const (
	aufEqualVariables smtModelFinderAUFLiteralKind = iota
	aufDisequalVariables
	aufEqualGround
	aufDisequalGround
	aufOrderedVariables
	aufSignedOrderedVariables
	aufOrderedGround
)

type smtModelFinderAUFLiteral struct {
	kind       smtModelFinderAUFLiteralKind
	quantifier ast.ASTHandle
	first      uint32
	second     uint32
	term       ast.ASTHandle
}

// processLiteral is quantifier_analyzer::process_literal(atom, sign).
func (g *smtModelFinderAUFGraph) processLiteral(quantifier, atom ast.ASTHandle, negative bool) error {
	m := g.finder.manager
	kind, err := m.GetKind(atom)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		variable, err := m.GetVarIndex(atom)
		if err != nil {
			return err
		}
		// (not X) is X != true; X is X != false.
		var term ast.ASTHandle
		if negative {
			term, err = m.MkTrue()
		} else {
			term, err = m.MkFalse()
		}
		if err != nil {
			return err
		}
		g.appendLiteral(smtModelFinderAUFLiteral{kind: aufDisequalGround, quantifier: quantifier, first: variable, term: term})
		return nil
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	equality, err := g.isBasicEq(atom)
	if err != nil {
		return err
	}
	if equality {
		left, err := m.GetArg(atom, 0)
		if err != nil {
			return err
		}
		right, err := m.GetArg(atom, 1)
		if err != nil {
			return err
		}
		variable, bound, boundRef, _, matched, err := g.variableAndGround(left, right)
		if err != nil {
			return errors.Join(err, releaseOptionalRef(m, &boundRef))
		}
		if matched {
			if boundRef != (ast.ASTRef{}) {
				if bound, err = g.keep(boundRef); err != nil {
					return err
				}
			}
			row := smtModelFinderAUFLiteral{kind: aufEqualGround, quantifier: quantifier, first: variable, term: bound}
			if negative {
				row.kind = aufDisequalGround
			}
			g.appendLiteral(row)
			return nil
		}
		first, second, matched, err := g.variableAndVariable(left, right)
		if err != nil {
			return err
		}
		if matched {
			row := smtModelFinderAUFLiteral{kind: aufEqualVariables, quantifier: quantifier, first: first, second: second}
			if negative {
				row.kind = aufDisequalVariables
			}
			g.appendLiteral(row)
			return nil
		}
		return g.processApp(quantifier, atom)
	}
	before := len(g.literals)
	if err := g.collectComparison(quantifier, atom, negative); err != nil {
		return err
	}
	if len(g.literals) > before {
		return nil
	}
	return g.processApp(quantifier, atom)
}

// processLiteral is one literal qinfo's process_auf (x_eq_y, x_neq_y,
// x_leq_y, x_sleq_y, x_eq_t, x_neq_t, x_gle_t).
func (g *smtModelFinderAUFGraph) processLiteralAUF(info smtModelFinderAUFLiteral) error {
	first, err := g.uvar(info.quantifier, info.first)
	if err != nil {
		return err
	}
	switch info.kind {
	case aufEqualVariables, aufDisequalVariables, aufOrderedVariables, aufSignedOrderedVariables:
		second, err := g.uvar(info.quantifier, info.second)
		if err != nil {
			return err
		}
		if info.kind != aufEqualVariables {
			if err = first.merge(second); err != nil {
				return err
			}
			if info.kind == aufOrderedVariables || info.kind == aufSignedOrderedVariables {
				if err = first.setMono(); err != nil {
					return err
				}
			}
			if info.kind == aufSignedOrderedVariables {
				if err = first.setSigned(); err != nil {
					return err
				}
			}
		} else {
			if err = first.insertAvoid(second); err != nil {
				return err
			}
			if first != second {
				if err = second.insertAvoid(first); err != nil {
					return err
				}
			}
		}
	case aufEqualGround:
		if err = first.insertException(info.term); err != nil {
			return err
		}
	case aufOrderedGround:
		if err = first.setMono(); err != nil {
			return err
		}
	}
	return nil
}

// populateLiteral is one literal qinfo's populate_inst_sets.
func (g *smtModelFinderAUFGraph) populateLiteral(info smtModelFinderAUFLiteral) error {
	node := g.uvars[smtModelFinderAUFKey{info.quantifier, info.first}]
	switch info.kind {
	case aufDisequalGround, aufOrderedGround:
		if _, err := node.insert(info.term, 0); err != nil {
			return err
		}
	case aufEqualGround:
		uninterpreted, err := g.finder.manager.IsUninterpretedSort(node.sort)
		if err != nil {
			return err
		}
		if !uninterpreted {
			return nil
		}
		// For x=t on an uninterpreted sort, the source includes every
		// relevant context term of that sort (paper section 4.1).
		for _, enode := range g.finder.state.context.enodes.all() {
			if !g.finder.state.isRelevant(enode.owner) {
				continue
			}
			sort, err := g.finder.manager.GetSort(enode.owner)
			if err != nil {
				return err
			}
			if sort == node.sort {
				if _, err = node.insert(enode.owner, enode.Generation()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
