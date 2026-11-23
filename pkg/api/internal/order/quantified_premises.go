package order

import (
	"maps"
	"slices"

	"github.com/glycerine/g3/pkg/ast"
)

// QuantifiedPremises stores the input DAG up to the last permanent assertion.
// Grounding and Boolean compilation still see the constants in the current goal.
type QuantifiedPremises struct {
	manager            *ast.ASTManager
	base               []ast.ASTHandle
	roots              []uint32
	input              []projectionNode
	children           []uint32
	sourceIDs, symbols map[ast.ASTHandle]uint32
	work, nextPoll     int
}

func (g *groundProjection) readInputs(assertions []ast.ASTHandle, ids []uint32) error {
	for i, h := range assertions {
		id, err := g.read(h, 0)
		if err != nil {
			return err
		}
		if !g.input[id].boolean {
			return inapplicable
		}
		ids[i] = id
	}
	return nil
}

func (p *QuantifiedPremises) readBase(g *groundProjection, base []ast.ASTHandle, ids []uint32) error {
	if p.manager == g.manager && slices.Equal(p.base, base) {
		// Count the original parse work against each goal's budget. Check for
		// cancellation even when a hit skips every poll in the parsed prefix.
		if p.work > g.compiler.ceiling || g.manager.ResourceLimit().CancelFlag() {
			return errBitLimit
		}
		g.compiler.work, g.compiler.nextPoll = p.work, p.nextPoll
		copy(ids, p.roots)
		g.input, g.children = p.input, p.children
	} else {
		*p = QuantifiedPremises{}
		if err := g.readInputs(base, ids); err != nil {
			return err
		}
		// A very wide input can have many repeated edges despite the existing
		// 12,000-node parse limit. Keep that additional storage query-local.
		if len(g.children) > 25_000 {
			return nil
		}
		p.manager, p.base, p.roots = g.manager, slices.Clone(base), slices.Clone(ids)
		p.input, p.children = g.input, g.children
		p.sourceIDs, p.symbols = g.sourceIDs, g.symbols
		p.work, p.nextPoll = g.compiler.work, g.compiler.nextPoll
	}
	// The goal appends nodes and edges after the cached slices and introduces
	// its own source and declaration IDs. Only the premise IDs survive a call.
	g.sourceIDs, g.symbols = maps.Clone(p.sourceIDs), maps.Clone(p.symbols)
	return nil
}
