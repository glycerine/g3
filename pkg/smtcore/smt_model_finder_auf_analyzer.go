package smtcore

// Source: src/smt/smt_model_finder.cpp quantifier_analyzer formula/term visits.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// smtModelFinderQInfo is a quantifier_info's m_qinfo_vect: the analyzer
// builds it once at register_quantifier, and it lives (with the terms its
// qinfos hold) until the quantifier is popped.
type smtModelFinderQInfo struct {
	fvars    []smtModelFinderFVar
	literals []smtModelFinderAUFLiteral
	selects  []smtModelFinderSelectVar
	refs     []ast.ASTRef
}

func (q *smtModelFinderQInfo) close(m *ast.ASTManager) (result error) {
	for i := range q.refs {
		result = errors.Join(result, m.DelRef(&q.refs[i]))
	}
	q.refs = nil
	return result
}

func (f *SMTModelFinder) analyzeQInfos(quantifier ast.ASTHandle) error {
	collector := newSMTModelFinderAUFGraph(f)
	err := collector.collectFVars(quantifier)
	info := &smtModelFinderQInfo{
		fvars: collector.fvars, literals: collector.literals, selects: collector.selects, refs: collector.refs,
	}
	if err != nil {
		return errors.Join(err, info.close(f.manager))
	}
	f.qinfos[quantifier] = info
	return nil
}

// addQInfo feeds one quantifier's stored qinfos to the auf_solver graph.
func (g *smtModelFinderAUFGraph) addQInfo(quantifier ast.ASTHandle) error {
	info := g.finder.qinfos[quantifier]
	if info == nil {
		return ast.ErrASTDomain
	}
	for _, fvar := range info.fvars {
		g.appendFVar(fvar)
	}
	for _, literal := range info.literals {
		g.appendLiteral(literal)
	}
	g.selects = append(g.selects, info.selects...)
	return nil
}

type smtModelFinderAnalysisKey struct {
	quantifier, expression ast.ASTHandle
}

func (g *smtModelFinderAUFGraph) appendFVar(info smtModelFinderFVar) {
	for _, prior := range g.fvars {
		if prior == info {
			return
		}
	}
	g.fvars = append(g.fvars, info)
}

func (g *smtModelFinderAUFGraph) appendLiteral(info smtModelFinderAUFLiteral) {
	if info.kind == aufEqualVariables || info.kind == aufDisequalVariables || info.kind == aufOrderedVariables || info.kind == aufSignedOrderedVariables {
		if info.first > info.second {
			info.first, info.second = info.second, info.first
		}
	}
	for _, prior := range g.literals {
		if prior == info {
			return
		}
	}
	g.literals = append(g.literals, info)
}

func (g *smtModelFinderAUFGraph) collectTerm(quantifier, term ast.ASTHandle) error {
	m := g.finder.manager
	ground, err := m.IsGround(term)
	if err != nil || ground {
		return err
	}
	key := smtModelFinderAnalysisKey{quantifier, term}
	if g.analysisSeen[key]&4 != 0 {
		return nil
	}
	g.analysisSeen[key] |= 4
	sort, err := m.GetSort(term)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	if sort == boolean {
		if err := g.collectLiterals(quantifier, term, false); err != nil {
			return err
		}
		return g.collectLiterals(quantifier, term, true)
	}
	kind, err := m.GetKind(term)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		return nil
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	return g.analyzeApplication(quantifier, term)
}

func (g *smtModelFinderAUFGraph) analyzeApplication(quantifier, term ast.ASTHandle) error {
	m := g.finder.manager
	family, err := m.GetFamilyID(term)
	if err != nil {
		return err
	}
	function, err := m.GetDecl(term)
	if err != nil {
		return err
	}
	count, err := m.GetNumArgs(term)
	if err != nil {
		return err
	}
	if family != ast.NullFamilyID {
		classifier := smtModelFinderAUFClassifier{finder: g.finder}
		selected, err := classifier.isAUFSelect(term)
		if err != nil {
			return err
		}
		if selected {
			if err := g.collectSelectVars(quantifier, term); err != nil {
				return err
			}
			array, err := m.GetArg(term, 0)
			if err != nil {
				return err
			}
			return g.collectTerm(quantifier, array)
		}
	}
	for position := uint32(0); position < count; position++ {
		argument, err := m.GetArg(term, position)
		if err != nil {
			return err
		}
		if family == ast.NullFamilyID {
			kind, err := m.GetKind(argument)
			if err != nil {
				return err
			}
			if kind == ast.ASTVar {
				variable, err := m.GetVarIndex(argument)
				if err != nil {
					return err
				}
				g.appendFVar(smtModelFinderFVar{quantifier: quantifier, function: function, position: position, variable: variable})
				continue
			}
			before := len(g.fvars)
			if err := g.collectOffset(quantifier, function, position, argument); err != nil {
				return err
			}
			if len(g.fvars) > before {
				continue
			}
		}
		if err := g.collectTerm(quantifier, argument); err != nil {
			return err
		}
	}
	return nil
}
