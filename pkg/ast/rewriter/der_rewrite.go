package rewriter

// Source: src/ast/rewriter/der.cpp:100-206,317-461;
// src/ast/rewriter/der.h:120-181.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *DER) substituteTerm(c *derCall, term ast.ASTHandle, bindings []ast.ASTHandle, scratch ast.ASTRef) (ast.ASTHandle, error) {
	if err := d.substitute.Substitute(term, bindings, scratch); err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := d.manager.RefGet(scratch)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(value)
}

func (d *DER) applySubstitution(c *derCall, q ast.ASTHandle, literals []ast.ASTHandle, positions []int32, definitions []ast.ASTHandle, order []uint32) (result ast.ASTHandle, err error) {
	bindings := make([]ast.ASTHandle, len(definitions))
	scratch, err := d.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, d.manager.DelRef(&scratch)) }()
	for _, index := range order {
		current, x := d.substituteTerm(c, definitions[index], bindings, scratch)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		bindings[len(bindings)-int(index)-1] = current
	}
	args := make([]ast.ASTHandle, 0, len(literals))
	for i := range literals {
		index := positions[i]
		if index >= 0 && definitions[index] != (ast.ASTHandle{}) {
			continue
		}
		args = append(args, literals[i])
	}
	// Source: t = (sz == 1) ? m_new_args[0] : m_manager.mk_or(sz, ...); the
	// raw mk_or keeps a nullary or.
	var reducedBody ast.ASTHandle
	if len(args) == 1 {
		reducedBody = args[0]
	} else {
		reducedBody, err = d.manager.MkOrArgs(args)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	reducedBody, err = d.substituteTerm(c, reducedBody, bindings, scratch)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	patternsN, err := d.manager.GetNumPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	patterns := make([]ast.ASTHandle, patternsN)
	for i := range patterns {
		old, x := d.manager.GetPattern(q, uint32(i))
		if x != nil {
			return ast.ASTHandle{}, x
		}
		patterns[i], x = d.substituteTerm(c, old, bindings, scratch)
		if x != nil {
			return ast.ASTHandle{}, x
		}
	}
	noPatternsN, err := d.manager.GetNumNoPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	noPatterns := make([]ast.ASTHandle, noPatternsN)
	for i := range noPatterns {
		old, x := d.manager.GetNoPattern(q, uint32(i))
		if x != nil {
			return ast.ASTHandle{}, x
		}
		noPatterns[i], x = d.substituteTerm(c, old, bindings, scratch)
		if x != nil {
			return ast.ASTHandle{}, x
		}
	}
	return d.manager.UpdateQuantifierAnnotations(q, patterns, noPatterns, reducedBody)
}

// reduceOnce is der::reduce1. Only universal quantifiers are reduced. A
// top-level or contributes its direct arguments; any other body is reduced
// only by the unit rule.
func (d *DER) reduceOnce(c *derCall, q ast.ASTHandle) (ast.ASTHandle, error) {
	forall, err := d.manager.IsForall(q)
	if err != nil {
		return q, err
	}
	if !forall {
		return q, nil
	}
	body, err := d.manager.GetQuantifierBody(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declarations, err := d.manager.GetNumDeclarations(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	isOr, err := d.isBasic(body, ast.ASTOpOr)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if isOr {
		count, err := d.manager.GetNumArgs(body)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		literals := make([]ast.ASTHandle, count)
		positions := make([]int32, count)
		for i := range literals {
			literals[i], err = d.manager.GetArg(body, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			positions[i] = -1
		}
		definitions := make([]ast.ASTHandle, declarations)
		variables := make([]ast.ASTHandle, declarations)
		matches := uint32(0)
		// Find all disequalities.
		for i, arg := range literals {
			variable, index, definition, matched, x := d.isVarDiseq(c, arg, declarations)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			if !matched {
				continue
			}
			if definitions[index] == (ast.ASTHandle{}) {
				definitions[index] = definition
				variables[index] = variable
				positions[i] = int32(index)
				matches++
			}
		}
		if matches == 0 {
			if err = d.traceNoCandidate(q); err != nil {
				return ast.ASTHandle{}, err
			}
			return q, nil
		}
		if err = d.traceDefinitions(definitions); err != nil {
			return ast.ASTHandle{}, err
		}
		order, err := d.sortVariables(variables, definitions)
		if err != nil {
			return q, err
		}
		if err = d.traceEliminationOrder(order); err != nil {
			return ast.ASTHandle{}, err
		}
		if len(order) == 0 {
			return q, nil
		}
		return d.applySubstitution(c, q, literals, positions, definitions, order)
	}
	// get_elimination_order/top-sort checks for cycles, but it is not
	// invoked for unit clauses, so the occurs check is done here.
	variable, _, definition, matched, err := d.isVarDiseq(c, body, declarations)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if matched {
		occurs, x := d.manager.OccursExpr(variable, definition)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if !occurs {
			return d.manager.MkFalse()
		}
	}
	return q, nil
}

// ReduceQuantifier is der::operator(): it applies reduce1 to a fixed point
// and then, if something was reduced and the result is still a universal
// quantifier, removes declarations made unused by substitution.
func (d *DER) ReduceQuantifier(q ast.ASTHandle, out, proof ast.ASTRef) (result error) {
	if err := d.open(); err != nil {
		return err
	}
	if kind, err := d.manager.GetKind(q); err != nil {
		return err
	} else if kind != ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	call := derCall{owner: d}
	defer func() { result = errors.Join(result, call.close()) }()
	if err := d.traceQuantifier(q); err != nil {
		return err
	}
	proofs, err := d.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	var pr ast.ASTHandle
	current := q
	reduced := false
	for {
		next, err := d.reduceOnce(&call, current)
		if err != nil {
			return err
		}
		if next == current {
			break
		}
		reduced = true
		if next, err = call.hold(next); err != nil {
			return err
		}
		if proofs {
			step, err := d.manager.MkProofDER(current, next)
			if err != nil {
				return err
			}
			if step, err = call.hold(step); err != nil {
				return err
			}
			if pr, err = d.manager.MkProofTransitivity(pr, step); err != nil {
				return err
			}
			if pr, err = call.hold(pr); err != nil {
				return err
			}
		}
		current = next
		kind, err := d.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind != ast.ASTQuantifier {
			break
		}
	}
	// Eliminate variables that have become unused.
	if reduced {
		forall := false
		kind, err := d.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind == ast.ASTQuantifier {
			if forall, err = d.manager.IsForall(current); err != nil {
				return err
			}
		}
		if forall {
			eliminated, err := ElimUnusedVarsUFBV(d.manager, current)
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, d.manager.DelRef(&eliminated)) }()
			r, err := d.manager.RefGet(eliminated)
			if err != nil {
				return err
			}
			if proofs {
				p1, err := d.manager.MkProofElimUnusedVars(current, r)
				if err != nil {
					return err
				}
				if p1, err = call.hold(p1); err != nil {
					return err
				}
				if pr, err = d.manager.MkProofTransitivity(pr, p1); err != nil {
					return err
				}
				if pr, err = call.hold(pr); err != nil {
					return err
				}
			}
			current = r
		}
	}
	if err := d.manager.SetRef(out, current); err != nil {
		return err
	}
	if !proofs {
		return nil
	}
	if pr.IsNull() {
		return d.manager.ResetRef(proof)
	}
	return d.manager.SetRef(proof, pr)
}

type DERConfiguration struct {
	DefaultConfiguration
	self   *DERConfiguration
	core   *DER
	closed bool
}

func NewDERConfiguration(m *ast.ASTManager) (*DERConfiguration, error) {
	d, err := NewDER(m)
	if err != nil {
		return nil, err
	}
	c := &DERConfiguration{core: d}
	c.self = c
	c.defaults.manager = m
	return c, nil
}

func (c *DERConfiguration) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (result bool, err error) {
	if c == nil || c.self != c || c.closed || c.core == nil {
		return false, ast.ErrASTHandle
	}
	updated, err := c.core.manager.UpdateQuantifierArguments(q, body, patterns, noPatterns)
	if err != nil {
		return false, err
	}
	ref, err := c.core.manager.NewRef(updated)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, c.core.manager.DelRef(&ref)) }()
	return true, c.core.ReduceQuantifier(updated, out, proof)
}

func (c *DERConfiguration) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	c.closed = true
	return c.core.Close()
}

type DERRewriter struct {
	self      *DERRewriter
	manager   *ast.ASTManager
	config    *DERConfiguration
	traversal *Traversal
	closed    bool
}

func NewDERRewriter(m *ast.ASTManager) (*DERRewriter, error) {
	c, err := NewDERConfiguration(m)
	if err != nil {
		return nil, err
	}
	r := &DERRewriter{manager: m, config: c}
	r.self = r
	r.traversal, err = NewTraversal(m, c)
	return r, err
}

func (r *DERRewriter) Rewrite(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if r == nil || r.self != r || r.closed || r.traversal == nil {
		return ast.ErrASTHandle
	}
	if err := r.traversal.Rewrite(root, out); err != nil {
		return err
	}
	proofs, err := r.manager.ProofsEnabled()
	if err != nil || !proofs {
		return err
	}
	value, err := r.manager.RefGet(out)
	if err != nil {
		return err
	}
	var p ast.ASTHandle
	if value == root {
		p, err = r.manager.MkProofReflexivity(root)
	} else {
		p, err = r.manager.MkProofRewrite(root, value)
	}
	if err != nil {
		return err
	}
	return r.manager.SetRef(proof, p)
}

func (r *DERRewriter) Reset() error {
	if r == nil || r.self != r || r.closed || r.traversal == nil {
		return ast.ErrASTHandle
	}
	return r.traversal.Reset()
}

func (r *DERRewriter) Cleanup() error {
	if r == nil || r.self != r || r.closed || r.traversal == nil {
		return ast.ErrASTHandle
	}
	if err := r.traversal.Cleanup(); err != nil {
		return err
	}
	if r.config == nil || r.config.core == nil || r.config.core.substitute == nil {
		return ast.ErrASTHandle
	}
	return r.config.core.substitute.reducer.engine.Cleanup()
}

func (r *DERRewriter) Close() (result error) {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		result = errors.Join(result, r.traversal.Close())
	}
	if r.config != nil {
		result = errors.Join(result, r.config.Close())
	}
	r.closed = true
	return result
}
