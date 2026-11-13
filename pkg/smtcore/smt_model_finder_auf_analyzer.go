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
	order    []smtModelFinderQInfoRef // m_qinfo_vect order across kinds
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
	collector.ngDecls = f.nonGround[quantifier]
	err := collector.collectFVars(quantifier)
	info := &smtModelFinderQInfo{
		fvars: collector.fvars, literals: collector.literals, selects: collector.selects, order: collector.qorder, refs: collector.refs,
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
	for _, ref := range info.order {
		switch ref.kind {
		case qinfoFVar:
			g.appendFVar(info.fvars[ref.index])
		case qinfoLiteral:
			g.appendLiteral(info.literals[ref.index])
		case qinfoSelect:
			g.appendSelect(info.selects[ref.index])
		}
	}
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
	g.qorder = append(g.qorder, smtModelFinderQInfoRef{qinfoFVar, len(g.fvars)})
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
	g.qorder = append(g.qorder, smtModelFinderQInfoRef{qinfoLiteral, len(g.literals)})
	g.literals = append(g.literals, info)
}

// appendSelect is insert_qinfo for a select_var.
func (g *smtModelFinderAUFGraph) appendSelect(info smtModelFinderSelectVar) {
	g.qorder = append(g.qorder, smtModelFinderQInfoRef{qinfoSelect, len(g.selects)})
	g.selects = append(g.selects, info)
}

// smtModelFinderFormulaEntry is quantifier_analyzer's entry: a formula
// and its polarity on m_ftodo.
type smtModelFinderFormulaEntry struct {
	formula  ast.ASTHandle
	negative bool
}

const (
	smtModelFinderPosCache uint8 = 1 // m_pos_cache (shared with visit_term)
	smtModelFinderNegCache uint8 = 2 // m_neg_cache
)

// visitFormula is quantifier_analyzer::visit_formula.
func (g *smtModelFinderAUFGraph) visitFormula(quantifier, n ast.ASTHandle, negative bool) error {
	ground, err := g.finder.manager.IsGround(n)
	if err != nil || ground {
		return err
	}
	flag := smtModelFinderPosCache
	if negative {
		flag = smtModelFinderNegCache
	}
	key := smtModelFinderAnalysisKey{quantifier, n}
	if g.analysisSeen[key]&flag == 0 {
		g.ftodo = append(g.ftodo, smtModelFinderFormulaEntry{n, negative})
		g.analysisSeen[key] |= flag
	}
	return nil
}

// visitTerm is quantifier_analyzer::visit_term. Terms are marked in the
// positive formula cache, so a Boolean term later visited as a positive
// formula is skipped there.
func (g *smtModelFinderAUFGraph) visitTerm(quantifier, n ast.ASTHandle) error {
	ground, err := g.finder.manager.IsGround(n)
	if err != nil || ground {
		return err
	}
	key := smtModelFinderAnalysisKey{quantifier, n}
	if g.analysisSeen[key]&smtModelFinderPosCache == 0 {
		g.ttodo = append(g.ttodo, n)
		g.analysisSeen[key] |= smtModelFinderPosCache
	}
	return nil
}

// processUApp is quantifier_analyzer::process_u_app.
func (g *smtModelFinderAUFGraph) processUApp(quantifier, t ast.ASTHandle) error {
	m := g.finder.manager
	function, err := m.GetDecl(t)
	if err != nil {
		return err
	}
	count, err := m.GetNumArgs(t)
	if err != nil {
		return err
	}
	for position := uint32(0); position < count; position++ {
		argument, err := m.GetArg(t, position)
		if err != nil {
			return err
		}
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
		if err := g.visitTerm(quantifier, argument); err != nil {
			return err
		}
	}
	return nil
}

// processIApp is quantifier_analyzer::process_i_app.
func (g *smtModelFinderAUFGraph) processIApp(quantifier, t ast.ASTHandle) error {
	m := g.finder.manager
	classifier := smtModelFinderAUFClassifier{finder: g.finder}
	selected, err := classifier.isAUFSelect(t)
	if err != nil {
		return err
	}
	if selected {
		array, err := m.GetArg(t, 0)
		if err != nil {
			return err
		}
		if err := g.visitTerm(quantifier, array); err != nil {
			return err
		}
		return g.collectSelectVars(quantifier, t)
	}
	count, err := m.GetNumArgs(t)
	if err != nil {
		return err
	}
	for position := uint32(0); position < count; position++ {
		argument, err := m.GetArg(t, position)
		if err != nil {
			return err
		}
		if err := g.visitTerm(quantifier, argument); err != nil {
			return err
		}
	}
	return nil
}

// processApp is quantifier_analyzer::process_app.
func (g *smtModelFinderAUFGraph) processApp(quantifier, t ast.ASTHandle) error {
	family, err := g.finder.manager.GetFamilyID(t)
	if err != nil {
		return err
	}
	// m_info->m_ng_decls.insert(t->get_decl()) for non-basic applications.
	if g.ngDecls != nil && family != g.finder.manager.BasicFamilyID() {
		declaration, err := g.finder.manager.GetDecl(t)
		if err != nil {
			return err
		}
		if err = g.ngDecls.Insert(declaration, [2]ast.ASTHandle{}); err != nil {
			return err
		}
	}
	if family == ast.NullFamilyID {
		return g.processUApp(quantifier, t)
	}
	return g.processIApp(quantifier, t)
}

// processIff is quantifier_analyzer::process_iff.
func (g *smtModelFinderAUFGraph) processIff(quantifier, n ast.ASTHandle) error {
	m := g.finder.manager
	for i := uint32(0); i < 2; i++ {
		arg, err := m.GetArg(n, i)
		if err != nil {
			return err
		}
		if err := g.visitFormula(quantifier, arg, false); err != nil {
			return err
		}
		if err := g.visitFormula(quantifier, arg, true); err != nil {
			return err
		}
	}
	return nil
}

// processFormulasOnStack is quantifier_analyzer::process_formulas_on_stack.
func (g *smtModelFinderAUFGraph) processFormulasOnStack(quantifier ast.ASTHandle) error {
	m := g.finder.manager
	for len(g.ftodo) != 0 {
		if err := g.finder.checkpoint(); err != nil {
			return err
		}
		e := g.ftodo[len(g.ftodo)-1]
		g.ftodo = g.ftodo[:len(g.ftodo)-1]
		curr, negative := e.formula, e.negative
		kind, err := m.GetKind(curr)
		if err != nil {
			return err
		}
		if kind == ast.ASTVar {
			if err := g.processLiteral(quantifier, curr, negative); err != nil {
				return err
			}
			continue
		}
		if kind != ast.ASTApp {
			// The quantifier is flat: no nested quantifiers.
			return ast.ErrASTDomain
		}
		family, err := m.GetFamilyID(curr)
		if err != nil {
			return err
		}
		if family != m.BasicFamilyID() {
			if err := g.processLiteral(quantifier, curr, negative); err != nil {
				return err
			}
			continue
		}
		op, err := m.GetDeclKind(curr)
		if err != nil {
			return err
		}
		switch op {
		case ast.ASTOpOr, ast.ASTOpAnd:
			n, err := m.GetNumArgs(curr)
			if err != nil {
				return err
			}
			for i := uint32(0); i < n; i++ {
				arg, err := m.GetArg(curr, i)
				if err != nil {
					return err
				}
				if err := g.visitFormula(quantifier, arg, negative); err != nil {
					return err
				}
			}
		case ast.ASTOpNot:
			arg, err := m.GetArg(curr, 0)
			if err != nil {
				return err
			}
			if err := g.visitFormula(quantifier, arg, !negative); err != nil {
				return err
			}
		case ast.ASTOpIte:
			c, err := m.GetArg(curr, 0)
			if err != nil {
				return err
			}
			th, err := m.GetArg(curr, 1)
			if err != nil {
				return err
			}
			el, err := m.GetArg(curr, 2)
			if err != nil {
				return err
			}
			for _, v := range []smtModelFinderFormulaEntry{{c, negative}, {c, !negative}, {th, negative}, {el, negative}} {
				if err := g.visitFormula(quantifier, v.formula, v.negative); err != nil {
					return err
				}
			}
		case ast.ASTOpIff:
			if err := g.processIff(quantifier, curr); err != nil {
				return err
			}
		case ast.ASTOpEq:
			arg, err := m.GetArg(curr, 0)
			if err != nil {
				return err
			}
			boolean, err := g.isBoolExpr(arg)
			if err != nil {
				return err
			}
			if boolean {
				err = g.processIff(quantifier, curr)
			} else {
				err = g.processLiteral(quantifier, curr, negative)
			}
			if err != nil {
				return err
			}
		default:
			if err := g.processLiteral(quantifier, curr, negative); err != nil {
				return err
			}
		}
	}
	return nil
}

// processTermsOnStack is quantifier_analyzer::process_terms_on_stack.
func (g *smtModelFinderAUFGraph) processTermsOnStack(quantifier ast.ASTHandle) error {
	m := g.finder.manager
	for len(g.ttodo) != 0 {
		curr := g.ttodo[len(g.ttodo)-1]
		g.ttodo = g.ttodo[:len(g.ttodo)-1]
		boolean, err := g.isBoolExpr(curr)
		if err != nil {
			return err
		}
		if boolean {
			// A formula nested in a term.
			if err := g.visitFormula(quantifier, curr, false); err != nil {
				return err
			}
			if err := g.visitFormula(quantifier, curr, true); err != nil {
				return err
			}
			continue
		}
		kind, err := m.GetKind(curr)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			if err := g.processApp(quantifier, curr); err != nil {
				return err
			}
		case ast.ASTVar:
			// An unexpected variable occurrence (m_is_auf = false).
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

// processClause is quantifier_analyzer::process_clause.
func (g *smtModelFinderAUFGraph) processClause(quantifier, cls ast.ASTHandle) error {
	m := g.finder.manager
	n, err := m.ClauseNumLiterals(cls)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		literal, err := m.ClauseLiteral(cls, i)
		if err != nil {
			return err
		}
		atom, negative, err := m.LiteralAtomSign(literal)
		if err != nil {
			return err
		}
		ground, err := m.IsGround(atom)
		if err != nil {
			return err
		}
		if !ground {
			if err := g.processLiteral(quantifier, atom, negative); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) isBoolExpr(e ast.ASTHandle) (bool, error) {
	m := g.finder.manager
	sort, err := m.GetSort(e)
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	return err == nil && sort == boolean, err
}

// isBasicEq is ast_manager::is_eq.
func (g *smtModelFinderAUFGraph) isBasicEq(e ast.ASTHandle) (bool, error) {
	m := g.finder.manager
	family, err := m.GetFamilyID(e)
	if err != nil || family != m.BasicFamilyID() {
		return false, err
	}
	op, err := m.GetDeclKind(e)
	return err == nil && op == ast.ASTOpEq, err
}
