package smtcore

// Source: src/smt/smt_model_checker.cpp add_instance, restart_eh,
// assert_new_instances and reset_new_instances.

import (
	"errors"
	"slices"

	"github.com/glycerine/g3/pkg/ast"
)

type smtModelCheckerInstance struct {
	quantifier ast.ASTHandle
	bindings   []ast.ASTHandle
	generation uint32
}

func (c *SMTModelChecker) enqueueCounterexample(quantifier ast.ASTHandle, values []ast.ASTHandle, fromInverse bool, generation uint32) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	count, err := c.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return false, err
	}
	if uint32(len(values)) < count {
		return false, ast.ErrASTDomain
	}
	bindings := append([]ast.ASTHandle(nil), values[:count]...)
	for i, value := range bindings {
		if term := c.valueTerms[value]; !fromInverse && !term.IsNull() {
			value = term
		}
		private, err := containsPrivateModelTerm(c.manager, value, true)
		if err != nil {
			return false, err
		}
		if private {
			c.exhausted = true
			return false, nil
		}
		kind, err := c.manager.GetKind(value)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return false, ast.ErrASTDomain
		}
		bindings[i] = value
	}
	quantifierGeneration, err := c.registry.Generation(quantifier)
	if err != nil {
		return false, err
	}
	if quantifierGeneration > generation {
		generation = quantifierGeneration
	}
	return c.pinInstance(quantifier, bindings, generation)
}

// Both callers transfer a private slice of bindings. Keep their references
// together in capture order, as model_checker::m_pinned_exprs does.
func (c *SMTModelChecker) pinInstance(quantifier ast.ASTHandle, bindings []ast.ASTHandle, generation uint32) (bool, error) {
	if c.pinsBusy {
		return false, ast.ErrASTPending
	}
	c.pinsBusy = true
	defer func() { c.pinsBusy = false }()
	start := len(c.pins)
	c.pins = slices.Grow(c.pins, len(bindings)+1)
	for _, expression := range bindings {
		ref, err := c.manager.NewRef(expression)
		if err != nil {
			return false, errors.Join(err, c.releasePinsFrom(start))
		}
		c.pins = append(c.pins, ref)
	}
	ref, err := c.manager.NewRef(quantifier)
	if err != nil {
		return false, errors.Join(err, c.releasePinsFrom(start))
	}
	c.pins = append(c.pins, ref)
	c.newInstances = append(c.newInstances, smtModelCheckerInstance{quantifier: quantifier, bindings: bindings, generation: generation})
	return true, nil
}

// A refusal can leave the current reference live. A destructor can also
// consume it and report an error. Keep the remaining owners available to retry.
func (c *SMTModelChecker) releasePinsFrom(start int) error {
	for i := start; i < len(c.pins); i++ {
		if err := c.manager.DelRef(&c.pins[i]); err != nil {
			if c.pins[i] == (ast.ASTRef{}) {
				i++
			}
			remaining := copy(c.pins[start:], c.pins[i:])
			c.pins = c.pins[:start+remaining]
			return err
		}
	}
	c.pins = c.pins[:start]
	return nil
}

func (c *SMTModelChecker) HasNewInstances() bool {
	return c != nil && !c.closed && len(c.newInstances) != 0
}

func (c *SMTModelChecker) ResetNewInstances() error {
	if err := c.open(); err != nil {
		return err
	}
	if c.pinsBusy {
		return ast.ErrASTPending
	}
	c.pinsBusy = true
	defer func() { c.pinsBusy = false }()
	c.newInstances = nil
	return c.releasePinsFrom(0)
}

// Restart publishes pinned instances after the context has returned to its
// search base. Quantifiers removed by backtracking are skipped, and bindings
// whose enodes disappeared are internalized again at the captured generation.
func (c *SMTModelChecker) Restart() (result error) {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.reportRestart(); err != nil {
		return err
	}
	previousGeneration := c.state.generation
	defer func() { c.state.generation = previousGeneration }()
	for _, instance := range c.newInstances {
		if _, present := c.state.BoolVar(instance.quantifier); !present {
			continue
		}
		if err := c.state.SetGeneration(instance.generation); err != nil {
			return err
		}
		bindings := make([]*SMTEnode, len(instance.bindings))
		for i, expression := range instance.bindings {
			if c.state.context.enodes.get(expression) == nil {
				if err := c.state.Internalize(expression, false); err != nil {
					return err
				}
			}
			node := c.state.context.enodes.get(expression)
			if node == nil {
				return ast.ErrASTDomain
			}
			bindings[i] = node
		}
		if _, err := c.registry.AddInstance(instance.quantifier, ast.ASTHandle{}, bindings, instance.generation, instance.generation, instance.generation); err != nil {
			return err
		}
	}
	return c.ResetNewInstances()
}
