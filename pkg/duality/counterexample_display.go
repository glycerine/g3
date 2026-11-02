package duality

// Source: muz/duality/duality_dl_interface.cpp, print_proof.
import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"io"
)

// WriteDerivation prints the active counterexample steps in source child-first
// order. The expression printer and solver factory are borrowed for this call.
// Quantified substitutions use reconstructed original-rule witnesses so values
// eliminated by preprocessing are not printed as arbitrary model defaults.
func (c *Counterexample) WriteDerivation(out io.Writer, writeExpr func(io.Writer, ast.ASTHandle) error, factory solver.Factory) (result error) {
	if err := c.open(); err != nil {
		return err
	}
	if c.tree == nil || c.root == nil || out == nil || writeExpr == nil {
		return ErrWrapperOwner
	}
	g := c.tree
	if err := g.open(); err != nil {
		return err
	}
	if err := g.cleanupProofWitnesses(); err != nil {
		return err
	}
	m := g.context.manager
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, scope.Close()) }()
	active := make(map[*Node]bool)
	var visit func(*Node) error
	visit = func(node *Node) (result error) {
		if node == nil || node.closed || node.Owner != g || node.Outgoing == nil || active[node] {
			return ErrWrapperOwner
		}
		active[node] = true
		defer delete(active, node)
		edge := node.Outgoing
		if edge.Map == nil || edge.Map.closed {
			return ErrWrapperOwner
		}
		used := make([]bool, len(edge.Children))
		for i, child := range edge.Children {
			empty, err := g.Empty(child)
			if err != nil {
				return err
			}
			used[i] = !empty
			if !empty {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		fact, err := g.EvalNode(node)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, fact.Close()) }()
		h, err := fact.Raw()
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "(step s!%d ", node.Number); err != nil {
			return err
		}
		if err = writeExpr(out, h); err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "\n rule!%d  (subst\n", edge.Map.Number); err != nil {
			return err
		}
		original, err := edge.Map.Owner.OriginalClause(edge.Map)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, original.Close()) }()
		clause, err := original.Raw()
		if err != nil {
			return err
		}
		kind, err := m.GetKind(clause)
		if err != nil {
			return err
		}
		if kind == ast.ASTQuantifier {
			forall, err := m.IsForall(clause)
			if err != nil {
				return err
			}
			if forall {
				proof, err := g.extractCounterexampleProof(node, make(map[*Node]bool), factory)
				if err != nil {
					return err
				}
				defer func() { result = errors.Join(result, m.DelRef(&proof)) }()
				ph, err := m.RefGet(proof)
				if err != nil {
					return err
				}
				substitutions, err := ast.NewASTProofHyperOutput(m)
				if err != nil {
					return err
				}
				defer func() { result = errors.Join(result, substitutions.Close()) }()
				conclusion, err := m.NewRef(ast.ASTHandle{})
				if err != nil {
					return err
				}
				defer func() { result = errors.Join(result, m.DelRef(&conclusion)) }()
				var premises []ast.ASTHandle
				defer func() {
					for _, p := range premises {
						result = errors.Join(result, m.DecRef(p))
					}
				}()
				matched, err := m.MatchProofHyperResolve(ph, &premises, conclusion, substitutions)
				if err != nil {
					return err
				}
				if !matched {
					return ErrProofWitness
				}
				values, err := substitutions.Substitution(0)
				if err != nil {
					return err
				}
				for i := range values {
					name, err := m.GetDeclarationName(clause, uint32(i))
					if err != nil {
						return err
					}
					sort, err := m.GetDeclarationSort(clause, uint32(i))
					if err != nil {
						return err
					}
					constant, err := m.MkConst(name, sort)
					if err != nil {
						return err
					}
					ref, err := m.NewRef(constant)
					if err != nil {
						return err
					}
					err = func() error {
						if _, err := io.WriteString(out, "    (= "); err != nil {
							return err
						}
						if err := writeExpr(out, constant); err != nil {
							return err
						}
						if _, err := io.WriteString(out, " "); err != nil {
							return err
						}
						if err := writeExpr(out, values[len(values)-1-i]); err != nil {
							return err
						}
						_, err := io.WriteString(out, ")\n")
						return err
					}()
					err = errors.Join(err, m.DelRef(&ref))
					if err != nil {
						return err
					}
				}
			}
		}
		if _, err := io.WriteString(out, "  )\n  (labels"); err != nil {
			return err
		}
		labels, err := g.GetLabels(edge)
		if err != nil {
			return err
		}
		for _, label := range labels {
			name, err := m.Symbols.String(label)
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, " %s", name); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, "  )\n  (ref "); err != nil {
			return err
		}
		for i, child := range edge.Children {
			if used[i] {
				_, err = fmt.Fprintf(out, " s!%d", child.Number)
			} else {
				_, err = io.WriteString(out, " true")
			}
			if err != nil {
				return err
			}
		}
		_, err = io.WriteString(out, "  ))\n")
		return err
	}
	if _, err := io.WriteString(out, "(derivation\n"); err != nil {
		return err
	}
	if err := visit(c.root); err != nil {
		return err
	}
	_, err = io.WriteString(out, ")\n")
	return err
}
