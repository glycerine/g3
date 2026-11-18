package duality

// Reconstruct original Horn-rule witnesses after FromClauses' QE-lite phase.
// A projected variable has no meaningful value in the counterexample model;
// solve its original rule against the already proved child/head arguments.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
)

var ErrProofWitness = errors.New("duality: original rule witness reconstruction failed")

type proofWitnessCall struct {
	manager      *ast.ASTManager
	proofScope   *ast.ScopedProofMode
	refs         []ast.ASTRef
	handles      []ast.ASTHandle
	terms        []*Expr
	substitution *rewriter.VarSubstitution
	native       solver.Solver
	model        *model.ModelCore
	evaluator    *model.ModelEvaluator
}

func (g *RPFP) cleanupProofWitnesses() error {
	for len(g.pendingProofWitnesses) > 0 {
		if err := g.pendingProofWitnesses[0].close(); err != nil {
			return err
		}
		g.pendingProofWitnesses = g.pendingProofWitnesses[1:]
	}
	return nil
}

func (g *RPFP) insertPendingProofWitness(index int, call *proofWitnessCall) {
	g.pendingProofWitnesses = append(g.pendingProofWitnesses, nil)
	copy(g.pendingProofWitnesses[index+1:], g.pendingProofWitnesses[index:])
	g.pendingProofWitnesses[index] = call
}

func (c *proofWitnessCall) close() error {
	if c.proofScope != nil {
		if err := c.proofScope.Close(); err != nil {
			return err
		}
		c.proofScope = nil
	}
	if c.evaluator != nil {
		if err := c.evaluator.Close(); err != nil {
			return err
		}
		c.evaluator = nil
	}
	if c.model != nil {
		consumed, err := c.model.DecRefConsumed()
		if consumed {
			c.model = nil
		}
		if err != nil {
			return err
		}
	}
	if c.native != nil {
		if err := c.native.Close(); err != nil {
			return err
		}
		c.native = nil
	}
	if c.substitution != nil {
		if err := c.substitution.Close(); err != nil {
			return err
		}
		c.substitution = nil
	}
	for len(c.terms) > 0 {
		i := len(c.terms) - 1
		err := c.terms[i].Close()
		if c.terms[i] == nil || c.terms[i].value == nil || c.terms[i].value.closed {
			c.terms = c.terms[:i]
		}
		if err != nil {
			return err
		}
	}
	if err := c.closeHandles(); err != nil {
		return err
	}
	for len(c.refs) > 0 {
		i := len(c.refs) - 1
		err := c.manager.DelRef(&c.refs[i])
		if c.refs[i] == (ast.ASTRef{}) {
			c.refs = c.refs[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *proofWitnessCall) closeHandles() error {
	for len(c.handles) > 0 {
		i := len(c.handles) - 1
		consumed, err := c.manager.DecRefConsumed(c.handles[i])
		if consumed {
			c.handles[i] = ast.ASTHandle{}
			c.handles = c.handles[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *proofWitnessCall) pin(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := c.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, ref)
	return h, nil
}
