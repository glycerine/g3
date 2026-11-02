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
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrProofWitness = errors.New("duality: original rule witness reconstruction failed")

type proofWitnessCall struct {
	manager      *ast.ASTManager
	refs         []ast.ASTRef
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

func (c *proofWitnessCall) close() error {
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

func (g *RPFP) recoverProofWitness(factory solver.Factory, quantified, clause ast.ASTHandle, positions [][]uint32, resolved []ast.ASTProofPosition, premises []ast.ASTHandle, conclusion ast.ASTHandle) (out []ast.ASTRef, result error) {
	if factory == nil {
		return nil, ErrWrapperOwner
	}
	m := g.context.manager
	c := &proofWitnessCall{manager: m}
	var values []ast.ASTRef
	defer func() {
		pending := false
		if err := c.close(); err != nil {
			pending = true
			result = errors.Join(result, err)
		}
		if result != nil {
			for i := range values {
				result = errors.Join(result, m.DelRef(&values[i]))
				if values[i] != (ast.ASTRef{}) {
					c.refs = append(c.refs, values[i])
					pending = true
				}
			}
			out = nil
		}
		if pending {
			g.pendingProofWitnesses = append(g.pendingProofWitnesses, c)
		}
	}()
	n, err := m.GetNumDeclarations(quantified)
	if err != nil {
		return nil, err
	}
	skolem := make([]ast.ASTHandle, n)
	for j := uint32(0); j < n; j++ {
		sort, err := m.GetDeclarationSort(quantified, j)
		if err != nil {
			return nil, err
		}
		skolem[n-1-j], err = c.pin(m.MkFreshConst("@proof", sort))
		if err != nil {
			return nil, err
		}
	}
	c.substitution, err = rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return nil, err
	}
	instantiated, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	c.refs = append(c.refs, instantiated)
	if err := c.substitution.Substitute(clause, skolem, instantiated); err != nil {
		return nil, err
	}
	inst, err := m.RefGet(instantiated)
	if err != nil {
		return nil, err
	}
	body, head := ast.ASTHandle{}, inst
	family, err := m.GetFamilyID(inst)
	if err != nil {
		return nil, err
	}
	op, err := m.GetDeclKind(inst)
	if err != nil {
		return nil, err
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpImplies {
		body, err = m.GetArg(inst, 0)
		if err != nil {
			return nil, err
		}
		head, err = m.GetArg(inst, 1)
		if err != nil {
			return nil, err
		}
	} else {
		body, err = m.MkTrue()
		if err != nil {
			return nil, err
		}
	}
	atoms := []ast.ASTHandle{body}
	family, err = m.GetFamilyID(body)
	if err != nil {
		return nil, err
	}
	op, err = m.GetDeclKind(body)
	if err != nil {
		return nil, err
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpAnd {
		count, err := m.GetNumArgs(body)
		if err != nil {
			return nil, err
		}
		atoms = make([]ast.ASTHandle, count)
		for i := range atoms {
			atoms[i], err = m.GetArg(body, uint32(i))
			if err != nil {
				return nil, err
			}
		}
	}
	truth, err := m.MkTrue()
	if err != nil {
		return nil, err
	}
	falsity, err := m.MkFalse()
	if err != nil {
		return nil, err
	}
	replacements := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, group := range positions {
		for _, position := range group {
			if position == 0 || int(position) > len(atoms) {
				return nil, ErrProofWitness
			}
			replacements[atoms[position-1]] = falsity
		}
	}
	var constraints []ast.ASTHandle
	match := func(application, fact ast.ASTHandle) error {
		d1, err := m.GetDecl(application)
		if err != nil {
			return err
		}
		d2, err := m.GetDecl(fact)
		if err != nil {
			return err
		}
		if d1 != d2 {
			return ErrProofWitness
		}
		n, err := m.GetNumArgs(application)
		if err != nil {
			return err
		}
		for j := uint32(0); j < n; j++ {
			a, err := m.GetArg(application, j)
			if err != nil {
				return err
			}
			b, err := m.GetArg(fact, j)
			if err != nil {
				return err
			}
			eq, err := c.pin(m.MkEq(a, b))
			if err != nil {
				return err
			}
			constraints = append(constraints, eq)
		}
		return nil
	}
	for i, position := range resolved {
		if position.First == 0 || int(position.First) > len(atoms) || i+1 >= len(premises) {
			return nil, ErrProofWitness
		}
		atom := atoms[position.First-1]
		fact, err := m.GetProofFact(premises[i+1])
		if err != nil {
			return nil, err
		}
		if err := match(atom, fact); err != nil {
			return nil, err
		}
		replacements[atom] = truth
	}
	if err := match(head, conclusion); err != nil {
		return nil, err
	}
	condition, err := substituteExpression(g.context, body, replacements)
	if err != nil {
		return nil, err
	}
	c.refs = append(c.refs, condition)
	constraint, err := m.RefGet(condition)
	if err != nil {
		return nil, err
	}
	constraints = append(constraints, constraint)
	c.native, err = factory.New(m, nil, false, true, false, "")
	if err != nil {
		return nil, err
	}
	for _, constraint := range constraints {
		if err := c.native.Assert(constraint); err != nil {
			return nil, err
		}
	}
	status, err := c.native.CheckSat(nil)
	if err != nil {
		return nil, err
	}
	if status != lbool.True {
		return nil, ErrProofWitness
	}
	c.model, err = c.native.AcquireModel()
	if err != nil {
		return nil, err
	}
	if c.model == nil {
		return nil, ErrProofWitness
	}
	c.evaluator, err = model.NewModelEvaluator(c.model, g.evaluatorConfig)
	if err != nil {
		return nil, err
	}
	if err := c.evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	for _, constant := range skolem {
		value, err := c.evaluator.EvaluateValue(constant)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
