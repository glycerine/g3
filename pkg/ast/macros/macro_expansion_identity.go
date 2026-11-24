package macros

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// A macro finder can have thousands of pending formulas with only a few
// occurrences of the declarations it replaces. Examine small terms before
// allocating a general rewriter and its caches. All state ends with the call.
const maxMacroProbeVisits = 32

type macroIdentityProbe struct {
	owner     *Manager
	remaining int
	seen      [maxMacroProbeVisits]ast.ASTHandle
	used      int
	frames    uint32
}

func (m *Manager) chargeUnchangedExpansion(term ast.ASTHandle) bool {
	manager := m.manager
	if ast.ASTSourceDebug || ast.ASTValidateHandles || diagnostic.TraceCompiled() ||
		manager.TraceStream != nil || manager.Budget().InjectionArmed() ||
		manager.NumeralService().RuntimeOwner().AllocationLimited() {
		return false
	}
	proofs, err := manager.ProofsEnabled()
	if err != nil || proofs {
		return false
	}
	p := macroIdentityProbe{owner: m, remaining: maxMacroProbeVisits}
	if !p.visit(term, 0) {
		return false
	}
	// Rewrite checks once on entry. Each frame finishes once, and starting
	// each non-root frame suspends its parent for one additional iteration.
	steps := max(uint32(1), 2*p.frames)
	return manager.ResourceLimit().TryIncOffset(steps)
}

func (p *macroIdentityProbe) visit(term ast.ASTHandle, scopeStart int) bool {
	if p.remaining == 0 {
		return false
	}
	p.remaining--
	m := p.owner.manager
	h, err := m.NodeHeader(term)
	if err != nil {
		return false
	}
	switch h.Kind {
	case ast.ASTVar:
		return true
	case ast.ASTApp:
		if _, found := p.owner.byDeclaration[h.Decl]; found {
			return false
		}
		if h.NumArgs == 0 {
			return true
		}
		// The general traversal can fold an ITE even without a macro.
		if h.Family == m.BasicFamilyID() && h.DeclKind == ast.ASTOpIte {
			return false
		}
	case ast.ASTQuantifier:
	default:
		return false
	}
	if h.RefCount > 1 {
		for _, prev := range p.seen[scopeStart:p.used] {
			if term == prev {
				return true
			}
		}
		// At most one insertion per visit. Handles are borrowed from term.
		p.seen[p.used] = term
		p.used++
	}
	p.frames++
	if h.Kind == ast.ASTQuantifier {
		body, err := m.GetQuantifierBody(term)
		// The traversal gives each binder its own cache and leaves patterns
		// alone. Drop the body's cache entries when exiting the binder.
		start := p.used
		ok := err == nil && p.visit(body, start)
		p.used = start
		return ok
	}
	for i := uint32(0); i < h.NumArgs; i++ {
		arg, err := m.GetArg(term, i)
		if err != nil || !p.visit(arg, scopeStart) {
			return false
		}
	}
	return true
}
