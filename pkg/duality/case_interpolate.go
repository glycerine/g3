package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrCaseInterpolationBad = errors.New("duality case interpolation: Bad")
var ErrCaseInterpolationReallyBad = errors.New("duality case interpolation: ReallyBad")

// CaseInterpolator borrows the search's graph, services and background theory.
// Its source TEST_BAD schedule persists across calls on this engine instance.
// Source: duality_rpfp.cpp:2967-3131.
type CaseInterpolator struct {
	Graph              *RPFP
	Auxiliary          solver.Solver
	Extensional        bool
	Simplifier         *interp.Manager
	Quantifiers        *qe.Lite
	Theory             []*Expr
	badCount, badLimit int
}

func (c *CaseInterpolator) Interpolate(root, node *Node, config *interp.RunConfig) (result error) {
	if c == nil || c.Auxiliary == nil || c.Simplifier == nil {
		return ErrWrapperOwner
	}
	g := c.Graph
	if err := g.solverReady(); err != nil {
		return err
	}
	if node == nil || node.closed || node.Owner != g || node.Outgoing == nil {
		return ErrWrapperOwner
	}
	m := g.context.manager
	if c.badLimit == 0 {
		c.badLimit = 1
	}
	aux, err := NewImplicantSolver(g, c.Auxiliary, c.Extensional)
	if err != nil {
		return err
	}
	core, err := NewProofCoreAccumulator(g.context)
	if err != nil {
		return errors.Join(err, aux.Close())
	}
	graphScope := false
	var pins []*Expr
	var facts []*Transformer
	defer func() {
		if graphScope {
			result = errors.Join(result, g.Pop(1))
		}
		for len(aux.limits) > 0 {
			if err := aux.Pop(); err != nil {
				result = errors.Join(result, err)
				break
			}
		}
		result = errors.Join(result, aux.Close(), core.Close())
		for _, term := range pins {
			result = errors.Join(result, term.Close())
		}
		for _, fact := range facts {
			result = errors.Join(result, fact.Close())
		}
	}()
	own := func(term *Expr, err error) (*Expr, error) {
		if term != nil {
			pins = append(pins, term)
		}
		return term, err
	}
	fromRef := func(ref ast.ASTRef, err error) (*Expr, error) {
		if err != nil {
			return nil, err
		}
		h, err := m.RefGet(ref)
		if err != nil {
			return nil, errors.Join(err, m.DelRef(&ref))
		}
		term, createErr := own(NewExpr(g.context, h))
		return term, errors.Join(createErr, m.DelRef(&ref))
	}
	simplifyAnnotation := func() error {
		formula, err := own(node.Annotation.Formula())
		if err != nil {
			return err
		}
		h, err := formula.Raw()
		if err != nil {
			return err
		}
		simplified, err := fromRef(c.Simplifier.Simplify(h))
		if err != nil {
			return err
		}
		h, err = simplified.Raw()
		if err != nil {
			return err
		}
		return m.SetRef(node.Annotation.formula, h)
	}
	needed := make(map[ast.ASTHandle]*Expr)
	for _, axiom := range c.Theory {
		if err := axiom.open(); err != nil {
			return err
		}
		if axiom.value.object.context != g.context {
			return ErrContextMismatch
		}
		h, err := axiom.Raw()
		if err != nil {
			return err
		}
		needed[h] = axiom
	}
	if err = aux.Push(); err != nil {
		return err
	}
	if err = aux.AddEdge(node.Outgoing); err != nil {
		return err
	}
	if err = node.Annotation.SetEmpty(); err != nil {
		return err
	}
	initial := ast.ASTHandle{}
	if node.Outgoing.dual != (ast.ASTRef{}) {
		initial, err = m.RefGet(node.Outgoing.dual)
		if err != nil {
			return err
		}
	}
	if err = core.includeHandle(initial); err != nil {
		return err
	}
	previousAnnotation, err := own(g.context.BoolVal(false))
	if err != nil {
		return err
	}
	previousImplicant := previousAnnotation
	repeated := 0
	axiomsAdded := false
	for {
		if err = aux.Push(); err != nil {
			return err
		}
		annotation, err := own(g.GetAnnotation(node))
		if err != nil {
			return err
		}
		excluded, err := own(annotation.Not())
		if err != nil {
			return err
		}
		old, err := node.Annotation.Clone()
		if err != nil {
			return err
		}
		facts = append(facts, old)
		if err = aux.Add(excluded); err != nil {
			return err
		}
		status, err := aux.Check()
		if err != nil {
			return err
		}
		if err = aux.Pop(); err != nil {
			return err
		}
		if status == lbool.False {
			break
		}
		if err = g.Push(); err != nil {
			return err
		}
		graphScope = true
		implicant, err := own(aux.GetImplicant())
		if err != nil {
			return err
		}
		ih, err := implicant.Raw()
		if err != nil {
			return err
		}
		previous, err := previousImplicant.Raw()
		if err != nil {
			return err
		}
		if ih == previous {
			repeated++
		}
		previousImplicant = implicant
		if err = g.ConstrainEdgeLocalized(node.Outgoing, implicant); err != nil {
			return err
		}
		if err = g.ConstrainEdgeLocalized(node.Outgoing, excluded); err != nil {
			return err
		}
		status, err = g.Check(root, nil, nil)
		if err != nil {
			return err
		}
		if status != lbool.False {
			return ErrCaseInterpolationReallyBad
		}
		proof, err := g.solver.Proof()
		if err != nil {
			return err
		}
		assumptions, err := interp.ProofAssumptions(c.Simplifier, proof)
		if err != nil {
			return err
		}
		var usedAxioms []*Expr
		for _, h := range assumptions {
			if err = core.includeHandle(h); err != nil {
				return err
			}
			if axiom, found := needed[h]; found {
				usedAxioms = append(usedAxioms, axiom)
				delete(needed, h)
			}
		}
		if _, err = g.SolveSingleNode(root, node, c.Simplifier, config); err != nil {
			return err
		}
		learned, err := own(g.GetAnnotation(node))
		if err != nil {
			return err
		}
		md, err := aux.AcquireModel()
		if err != nil {
			return err
		}
		if err = g.replaceDualModel(md, false); err != nil {
			return err
		}
		strengthened, cases, err := g.StrengthenFormulaByCaseSplitting(learned, c.Simplifier, c.Quantifiers)
		if strengthened != nil {
			pins = append(pins, strengthened)
		}
		pins = append(pins, cases...)
		if err != nil {
			return err
		}
		if err = g.SetAnnotation(node, strengthened); err != nil {
			return err
		}
		if err = simplifyAnnotation(); err != nil {
			return err
		}
		for _, axiom := range usedAxioms {
			if err = aux.Add(axiom); err != nil {
				return err
			}
		}
		// Preserve the active source TEST_BAD schedule without process globals.
		if c.badCount >= c.badLimit {
			c.badCount = 0
			c.badLimit *= 2
			return ErrCaseInterpolationBad
		}
		c.badCount++
		empty, err := node.Annotation.IsEmpty()
		if err != nil {
			return err
		}
		current, err := own(node.Annotation.Formula())
		if err != nil {
			return err
		}
		ch, err := current.Raw()
		if err != nil {
			return err
		}
		ph, err := previousAnnotation.Raw()
		if err != nil {
			return err
		}
		if empty || ch == ph || (repeated > 0 && !axiomsAdded) || repeated >= 10 {
			if axiomsAdded {
				return ErrCaseInterpolationReallyBad
			}
			for _, axiom := range c.Theory {
				if err = aux.Add(axiom); err != nil {
					return err
				}
			}
			axiomsAdded = true
		}
		graphScope = false
		if err = g.Pop(1); err != nil {
			return err
		}
		previousAnnotation = current
		if err = node.Annotation.UnionWith(old); err != nil {
			return err
		}
	}
	if err = core.Install(g); err != nil {
		return err
	}
	return aux.Pop()
}
