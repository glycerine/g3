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
	return c.writeDerivation(out, writeExpr, factory, nil, nil)
}

func (c *Counterexample) writeDerivation(out io.Writer, writeExpr func(io.Writer, ast.ASTHandle) error, factory solver.Factory, locals map[ast.ASTHandle]bool, background []*Expr) (result error) {
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
		// print the label and the proved fact: "(step s!N (name args...)".
		// AUDIT.md FUZZ_F-F6: the source prints the node's name and the
		// evaluated edge.F.IndParams, not the evaluated node application.
		if _, err := fmt.Fprintf(out, "(step s!%d (", node.Number); err != nil {
			return err
		}
		nameHandle, err := m.RefGet(node.Name)
		if err != nil {
			return err
		}
		nameDecl, err := NewFuncDecl(g.context, nameHandle)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, nameDecl.Close()) }()
		nameSymbol, err := nameDecl.Name()
		if err != nil {
			return err
		}
		if err = nameSymbol.Write(out); err != nil {
			return err
		}
		for _, paramRef := range edge.F.individualParams {
			ph, err := m.RefGet(paramRef)
			if err != nil {
				return err
			}
			param, err := NewExpr(g.context, ph)
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, param.Close()) }()
			value, err := g.Eval(edge, param)
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, value.Close()) }()
			vh, err := value.Raw()
			if err != nil {
				return err
			}
			if _, err = io.WriteString(out, " "); err != nil {
				return err
			}
			if err = writeExpr(out, vh); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintf(out, ")\n rule!%d  (subst\n", edge.Map.Number); err != nil {
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
				// FORK FIX #63: the original rule's witness, else the
				// counterexample model's value of each skolem.
				witness, hasWitness, err := g.findStepWitness(factory, node, background)
				if err != nil {
					return err
				}
				defer func() {
					for i := range witness {
						result = errors.Join(result, m.DelRef(&witness[i]))
					}
				}()
				bound, err := m.GetNumDeclarations(clause)
				if err != nil {
					return err
				}
				for i := 0; i < int(bound); i++ {
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
					// expr val = has_witness ? witness[j] : tree->Eval(&edge, skolem)
					var value ast.ASTHandle
					var evaluated *Expr
					if hasWitness {
						value, err = m.RefGet(witness[i])
					} else {
						var skolem *Expr
						if skolem, err = NewExpr(g.context, constant); err == nil {
							evaluated, err = g.Eval(edge, skolem)
							err = errors.Join(err, skolem.Close())
							if err == nil {
								value, err = evaluated.Raw()
							}
						}
					}
					if err != nil {
						if evaluated != nil {
							err = errors.Join(err, evaluated.Close())
						}
						return errors.Join(err, m.DelRef(&ref))
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
						if err := writeExpr(out, value); err != nil {
							return err
						}
						_, err := io.WriteString(out, ")\n")
						return err
					}()
					if err == nil && locals != nil {
						localTerm, e := NewExpr(g.context, constant)
						if e == nil {
							localized, e := g.Localize(edge, localTerm)
							if e == nil {
								var raw ast.ASTHandle
								raw, e = localized.Raw()
								if e == nil {
									var decl ast.ASTHandle
									decl, e = m.GetDecl(raw)
									if e == nil {
										locals[decl] = true
									}
								}
								e = errors.Join(e, localized.Close())
							}
							e = errors.Join(e, localTerm.Close())
						}
						err = e
					}
					if evaluated != nil {
						err = errors.Join(err, evaluated.Close())
					}
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
