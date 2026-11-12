package opt

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type ObjectivePurifier struct {
	manager *ast.ASTManager
	arith   ast.FamilyID
	filter  *tactic.FilterModelConverter
	refs    []ast.ASTRef
	// objectiveRefs and hardRefs are from_fmls' m_objectives and
	// m_hard_constraints; refs holds the fresh constants, which the
	// source keeps only through those two.
	objectiveRefs []purifiedObjectiveRefs
	hardRefs      []ast.ASTRef
	// pendingHard are the goal's hard formulas from_fmls has not reached.
	pendingHard []ast.ASTHandle
	nextHard    int
	stats       ObjectivePurificationStats
	set         PurifiedObjectiveSet
	closed      bool
	// detached: the filter (fm) was handed to m_model_converter, whose
	// reference releases it.
	detached bool
}

func NewObjectivePurifier(manager *ast.ASTManager) (*ObjectivePurifier, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	arith, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	filter, err := tactic.NewFilterModel(manager)
	if err != nil {
		return nil, err
	}
	return &ObjectivePurifier{manager: manager, arith: arith, filter: filter}, nil
}

func (p *ObjectivePurifier) open() error {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	if p.filter == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (p *ObjectivePurifier) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := p.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.refs = append(p.refs, ref)
	return h, nil
}

func (p *ObjectivePurifier) Statistics() (ObjectivePurificationStats, error) {
	if err := p.open(); err != nil {
		return ObjectivePurificationStats{}, err
	}
	return p.stats, nil
}

func (p *ObjectivePurifier) Close() (result error) {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.filter != nil && p.detached {
		p.filter = nil
	} else if p.filter != nil {
		result = p.filter.Close()
		if result == nil {
			p.filter = nil
		}
	}
	release := func(refs []ast.ASTRef) {
		for i := range refs {
			if result != nil || refs[i] == (ast.ASTRef{}) {
				continue
			}
			result = p.manager.DelRef(&refs[i])
		}
	}
	release(p.refs)
	// import_scoped_state: m_objectives.reset() destroys each objective
	// front to back (m_terms, front to back, before m_term), then
	// m_hard_constraints.reset() releases front to back.
	for i := range p.objectiveRefs {
		release(p.objectiveRefs[i].terms)
		for j := len(p.objectiveRefs[i].term) - 1; j >= 0; j-- {
			release(p.objectiveRefs[i].term[j : j+1])
		}
	}
	release(p.hardRefs)
	if result == nil {
		p.refs, p.objectiveRefs, p.hardRefs = nil, nil, nil
		p.closed = true
	}
	return result
}

// ReleaseHardConstraints is opt::context::pop's m_hard_constraints.reset():
// from_fmls' hard constraints are released front to back while the
// objectives (m_objectives) stay until import_scoped_state.
func (p *ObjectivePurifier) ReleaseHardConstraints() (result error) {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	for i := range p.hardRefs {
		if p.hardRefs[i] == (ast.ASTRef{}) {
			continue
		}
		if result = p.manager.DelRef(&p.hardRefs[i]); result != nil {
			return result
		}
	}
	p.hardRefs = nil
	return nil
}

// purifiedObjectiveRefs is one from_fmls objective: m_terms (soft terms)
// and the term references (m_term).
type purifiedObjectiveRefs struct {
	terms []ast.ASTRef
	term  []ast.ASTRef
}

func (p *ObjectivePurifier) holdInto(list *[]ast.ASTRef, h ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := p.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*list = append(*list, ref)
	return h, nil
}

// holdTerm holds h as the current objective's term.
func (p *ObjectivePurifier) holdTerm(h ast.ASTHandle) (ast.ASTHandle, error) {
	if len(p.objectiveRefs) == 0 {
		return p.hold(h)
	}
	return p.holdInto(&p.objectiveRefs[len(p.objectiveRefs)-1].term, h)
}

// DetachFilter hands one reference to the filter model converter (fm) to
// the caller: purify's `m_model_converter = concat(m_model_converter, fm)`.
// It returns nil when purification introduced no fresh declaration.
func (p *ObjectivePurifier) DetachFilter() (tactic.ModelConverter, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if p.stats.FreshDeclarations == 0 {
		return nil, nil
	}
	if err := p.filter.IncRef(); err != nil {
		return nil, err
	}
	p.detached = true
	return p.filter, nil
}
