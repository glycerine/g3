package smtcore

// Source: src/smt/smt_model_finder.cpp:2738-2813,2848-2864.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// hintSearchState is the scoped satisfied/residue portion of hint_solver.
// Quantifiers and function declarations are borrowed from its AST manager.
// The surrounding macro indexes and greedy search remain in the model finder.
type hintSearchState struct {
	manager            *ast.ASTManager
	residue, satisfied *scopedQuantifierSet
	// definitions is m_fs (obj_map<func_decl, expr*>): Value[0] is the def.
	definitions           *ast.ObjMap
	infos                 map[ast.ASTHandle]*hintQuantifierInfo
	forbidden, candidates *hintSet
	qByFunction           map[ast.ASTHandle]*hintSet
	qByDefinition         map[hintFunctionDefinition]*hintSet
	defsByFunction        map[ast.ASTHandle]*hintSet
	trace                 *diagnostic.TraceTags
}

func newHintSearchState(m *ast.ASTManager) *hintSearchState {
	s := &hintSearchState{
		manager:        m,
		residue:        newScopedQuantifierSet(m, nil),
		definitions:    ast.NewObjMap(m),
		infos:          make(map[ast.ASTHandle]*hintQuantifierInfo),
		forbidden:      newHintSet(m),
		candidates:     newHintSet(m),
		qByFunction:    make(map[ast.ASTHandle]*hintSet),
		qByDefinition:  make(map[hintFunctionDefinition]*hintSet),
		defsByFunction: make(map[ast.ASTHandle]*hintSet),
	}
	s.satisfied = newScopedQuantifierSet(m, func(q ast.ASTHandle, _ bool) {
		// hint_solver::ev_handler runs before each satisfying-set trail edit.
		if info := s.infos[q]; info != nil {
			info.theOne = ast.ASTHandle{}
		}
	})
	return s
}

func (s *hintSearchState) pushScope() {
	s.satisfied.pushScope()
	s.residue.pushScope()
}

func (s *hintSearchState) popScope() error {
	// Source greedy pops m_satisfied before m_residue. The first pop clears
	// the chosen function for each undone satisfied-set insertion.
	if e := s.satisfied.popScope(); e != nil {
		return e
	}
	return s.residue.popScope()
}

// updateSatisfiedResidue follows hint_solver::update_satisfied_residue using
// its already-built Q_f and Q_{f=def} indexes. The caller holds the greedy
// search scope, as the source does.
func (s *hintSearchState) updateSatisfiedResidue(f, def ast.ASTHandle) (bool, error) {
	if s == nil || s.residue == nil || s.satisfied == nil || len(s.satisfied.scopes) == 0 || len(s.residue.scopes) == 0 {
		return false, ast.ErrASTDomain
	}
	qF, qFDef := s.qByFunction[f], s.qByDefinition[hintFunctionDefinition{f, def}]
	if qF == nil || qFDef == nil {
		return false, ast.ErrASTDomain
	}
	useful := false
	for _, q := range qFDef.keys() {
		if s.satisfied.contains(q) {
			continue
		}
		info, e := s.info(q)
		if e != nil {
			return false, e
		}
		useful = true
		if e = s.residue.erase(q); e != nil {
			return false, e
		}
		if e = s.satisfied.insert(q); e != nil {
			return false, e
		}
		// This is quantifier_info::set_the_one(f); the event handler above
		// clears it when the insertion is undone.
		info.theOne = f
	}
	if !useful {
		return false, nil
	}
	for _, q := range qF.keys() {
		if !s.satisfied.contains(q) {
			if e := s.residue.insert(q); e != nil {
				return false, e
			}
		}
	}
	return true, nil
}
