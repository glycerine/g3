package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
	"io"
)

// WriteDerivation prints the reachable query's child-first rule steps.
func (b *Backend) WriteDerivation(out io.Writer, writeExpr func(io.Writer, ast.ASTHandle) error) error {
	if err := b.open(); err != nil {
		return err
	}
	e := b.queries
	if e.active == nil || e.status != lbool.True {
		return graph.ErrWrapperOwner
	}
	if len(e.active.background) > 0 || len(e.config.Theory) > 0 {
		return graph.ErrProofWitness
	}
	return e.active.engine.GetCounterexample().WriteDerivation(out, writeExpr, e.config.Factory)
}
