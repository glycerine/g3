package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Each verification batch can retain one private AST manager between plans.
// Rotating the context bounds the lifetime of its symbols and datatype tables.
const maxPlansPerContext = 16

type proofPlanContext struct {
	enabled bool
	parent  *ast.ASTManager
	aux     *orderProofState
	uses    int
}

func (c *proofPlanContext) reusable(parent *ast.ASTManager) bool {
	return c.enabled && !ast.ASTSourceDebug && !diagnostic.TraceCompiled() &&
		parent.TraceStream == nil && !parent.Budget().InjectionArmed() &&
		!parent.NumeralService().RuntimeOwner().AllocationLimited()
}

func (c *proofPlanContext) acquire(parent *ast.ASTManager, source *APISimpleSolver, reuse bool) (*orderProofState, error) {
	if c.aux != nil && (!reuse || c.parent != parent || c.uses >= maxPlansPerContext) {
		if err := c.close(); err != nil {
			return nil, err
		}
	}
	if c.aux == nil {
		c.parent, c.aux = parent, &orderProofState{source: source}
		search := orderSearch
		if reuse {
			search = eMatchingSearch
		}
		if err := c.aux.start(parent, search); err != nil {
			return nil, errors.Join(err, c.close())
		}
	}
	c.uses++
	return c.aux, nil
}

func (c *proofPlanContext) close() error {
	if c.aux != nil {
		if err := c.aux.release(); err != nil {
			// Retry destruction before any later plan uses the context.
			c.uses = maxPlansPerContext
			return err
		}
	}
	c.parent, c.aux, c.uses = nil, nil, 0
	return nil
}
