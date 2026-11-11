package smtcore

// Source: src/smt/smt_model_finder.cpp auf_solver node indexes and f_var.

import (
	"errors"
	uhash "github.com/glycerine/g3/pkg/util/hash"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

type smtModelFinderAUFKey struct {
	owner ast.ASTHandle
	index uint32
}

type smtModelFinderFVar struct {
	quantifier, function ast.ASTHandle
	position, variable   uint32
	offset               ast.ASTHandle
}

type smtModelFinderAUFGraph struct {
	finder    *SMTModelFinder
	nodes     []*smtModelFinderAUFNode
	uvars     map[smtModelFinderAUFKey]*smtModelFinderAUFNode
	arguments map[smtModelFinderAUFKey]*smtModelFinderAUFNode
	// argumentLayout is m_A_f_is's layout: key2node, a map keyed by
	// (func_decl, index) under pair_hash.
	argumentLayout *ast.ObjMap
	fvars          []smtModelFinderFVar
	literals       []smtModelFinderAUFLiteral
	refs           []ast.ASTRef
	model          *protomodel.ProtoModel
	selects        []smtModelFinderSelectVar
	witnesses      map[ast.ASTHandle]ast.ASTHandle
	analysisSeen   map[smtModelFinderAnalysisKey]uint8
	// qorder is quantifier_info::m_qinfo_vect's order across kinds.
	qorder []smtModelFinderQInfoRef
	// ftodo and ttodo are quantifier_analyzer's m_ftodo and m_ttodo.
	ftodo []smtModelFinderFormulaEntry
	ttodo []ast.ASTHandle
}

type smtModelFinderQInfoKind uint8

const (
	qinfoFVar smtModelFinderQInfoKind = iota
	qinfoLiteral
	qinfoSelect
)

type smtModelFinderQInfoRef struct {
	kind  smtModelFinderQInfoKind
	index int
}

func newSMTModelFinderAUFGraph(finder *SMTModelFinder) *smtModelFinderAUFGraph {
	return &smtModelFinderAUFGraph{
		finder: finder, uvars: make(map[smtModelFinderAUFKey]*smtModelFinderAUFNode),
		arguments:    make(map[smtModelFinderAUFKey]*smtModelFinderAUFNode),
		analysisSeen: make(map[smtModelFinderAnalysisKey]uint8),
	}
}

func (g *smtModelFinderAUFGraph) node(index map[smtModelFinderAUFKey]*smtModelFinderAUFNode, key smtModelFinderAUFKey, sort ast.ASTHandle) (*smtModelFinderAUFNode, error) {
	if node := index[key]; node != nil {
		if node.sort != sort {
			return nil, ast.ErrASTDomain
		}
		return node, nil
	}
	node, err := newSMTModelFinderAUFNode(uint32(len(g.nodes)), sort)
	if err != nil {
		return nil, err
	}
	index[key] = node
	g.nodes = append(g.nodes, node)
	return node, nil
}

// Variable indexes are de Bruijn indexes; the manager stores binder sorts in
// declaration order, as does the public model-finder instantiation-set API.
func (g *smtModelFinderAUFGraph) uvar(quantifier ast.ASTHandle, variable uint32) (*smtModelFinderAUFNode, error) {
	flat := g.finder.flatQuantifier(quantifier)
	count, err := g.finder.manager.GetNumDeclarations(flat)
	if err != nil || variable >= count {
		return nil, errors.Join(err, ast.ErrASTDomain)
	}
	sort, err := g.finder.manager.GetDeclarationSort(flat, count-variable-1)
	if err != nil {
		return nil, err
	}
	return g.node(g.uvars, smtModelFinderAUFKey{quantifier, variable}, sort)
}

func (g *smtModelFinderAUFGraph) argument(function ast.ASTHandle, position uint32) (*smtModelFinderAUFNode, error) {
	sort, err := g.finder.manager.GetDomain(function, position)
	if err != nil {
		return nil, err
	}
	key := smtModelFinderAUFKey{function, position}
	if g.arguments[key] == nil {
		if g.argumentLayout == nil {
			g.argumentLayout = ast.NewObjMap(g.finder.manager)
		}
		h, err := g.finder.manager.GetHash(function)
		if err != nil {
			return nil, err
		}
		g.argumentLayout.InsertPairIfNotThere(function, uint64(position), uhash.Combine(h, position))
	}
	return g.node(g.arguments, key, sort)
}

func (g *smtModelFinderAUFGraph) collectFVars(quantifier ast.ASTHandle) error {
	body, err := g.finder.manager.GetQuantifierBody(g.finder.flatQuantifier(quantifier))
	if err != nil {
		return err
	}
	// quantifier_analyzer::operator(): reset_cache, then a clause's
	// literals directly or the formula positively, then both stacks.
	for key := range g.analysisSeen {
		if key.quantifier == quantifier {
			delete(g.analysisSeen, key)
		}
	}
	g.ftodo, g.ttodo = g.ftodo[:0], g.ttodo[:0]
	clause, err := g.finder.manager.IsClause(body)
	if err != nil {
		return err
	}
	if clause {
		err = g.processClause(quantifier, body)
	} else {
		err = g.visitFormula(quantifier, body, false)
	}
	if err != nil {
		return err
	}
	for len(g.ftodo) != 0 || len(g.ttodo) != 0 {
		if err := g.processFormulasOnStack(quantifier); err != nil {
			return err
		}
		if err := g.processTermsOnStack(quantifier); err != nil {
			return err
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) build(quantifiers []ast.ASTHandle) error {
	for _, quantifier := range quantifiers {
		if err := g.finder.checkpoint(); err != nil {
			return err
		}
		count, err := g.finder.manager.GetNumDeclarations(g.finder.flatQuantifier(quantifier))
		if err != nil {
			return err
		}
		for variable := uint32(0); variable < count; variable++ {
			if _, err := g.uvar(quantifier, variable); err != nil {
				return err
			}
		}
		if err := g.addQInfo(quantifier); err != nil {
			return err
		}
	}
	// All equalities must be processed before allocating root sets.
	for _, info := range g.fvars {
		if err := g.finder.checkpoint(); err != nil {
			return err
		}
		argument, err := g.argument(info.function, info.position)
		if err != nil {
			return err
		}
		variable, err := g.uvar(info.quantifier, info.variable)
		if err != nil {
			return err
		}
		if info.offset.IsNull() {
			if err = argument.merge(variable); err != nil {
				return err
			}
		}
	}
	if err := g.processLiterals(); err != nil {
		return err
	}
	if err := g.processSelectVars(); err != nil {
		return err
	}
	for _, node := range g.nodes {
		if node.parent == nil {
			if err := node.makeInstantiationSet(g.finder.manager); err != nil {
				return err
			}
		}
	}
	// quantifier_info::populate_inst_sets: every qinfo in analysis order,
	// then the second pass (f_var_plus_offset copies) in the same order.
	for _, quantifier := range quantifiers {
		for _, ref := range g.qorder {
			var err error
			switch ref.kind {
			case qinfoFVar:
				if g.fvars[ref.index].quantifier == quantifier {
					err = g.populateFunctionArgument(g.fvars[ref.index])
				}
			case qinfoLiteral:
				if g.literals[ref.index].quantifier == quantifier {
					err = g.populateLiteral(g.literals[ref.index])
				}
			case qinfoSelect:
				if g.selects[ref.index].quantifier == quantifier {
					err = g.populateSelectVar(g.selects[ref.index])
				}
			}
			if err != nil {
				return err
			}
		}
		for _, ref := range g.qorder {
			if ref.kind == qinfoFVar && g.fvars[ref.index].quantifier == quantifier {
				if err := g.populateOffset(g.fvars[ref.index]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// populateFunctionArgument is f_var::populate_inst_sets (and
// f_var_plus_offset's first pass) for one qinfo.
func (g *smtModelFinderAUFGraph) populateFunctionArgument(info smtModelFinderFVar) error {
	context := g.finder.state.context
	node := g.arguments[smtModelFinderAUFKey{info.function, info.position}]
	if !info.offset.IsNull() {
		variable := g.uvars[smtModelFinderAUFKey{info.quantifier, info.variable}].root()
		argument := node.root()
		if argument != variable && (argument.mono || variable.mono) {
			argument.mono, variable.mono = true, true
		}
	}
	for _, application := range context.byDeclaration[info.function] {
		if application == nil || !g.finder.state.isRelevant(application.owner) {
			continue
		}
		if info.position >= uint32(len(application.arguments)) {
			return ast.ErrASTDomain
		}
		// Source f_var explicitly requires the original argument enode.
		// Its class representative can hide necessary MBQI instances.
		argument := application.arguments[info.position]
		if !info.offset.IsNull() && node.root() == g.uvars[smtModelFinderAUFKey{info.quantifier, info.variable}].root() {
			// A fresh arith_rewriter per relevant enode, as in the source.
			arith, err := g.localArithRewriter()
			if err != nil {
				return err
			}
			err = g.insertShifted(arith, node, argument.owner, info.offset, argument.Generation(), false)
			if err = errors.Join(err, arith.Close()); err != nil {
				return err
			}
			continue
		}
		if _, err := node.insert(argument.owner, argument.Generation()); err != nil {
			return err
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) publishInstantiationSets() error {
	for _, key := range smtSortedAUFKeys(g.uvars) {
		node := g.uvars[key]
		count, err := g.finder.manager.GetNumDeclarations(g.finder.flatQuantifier(key.owner))
		if err != nil {
			return err
		}
		originalCount, err := g.finder.manager.GetNumDeclarations(key.owner)
		if err != nil {
			return err
		}
		if count-key.index-1 >= originalCount {
			continue
		}
		set := node.root().set
		for _, entry := range set.entries() {
			term, generation := entry.Key, uint32(entry.Num)
			if _, err := g.finder.InsertInstantiationTerm(key.owner, count-key.index-1, term, generation); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) close() (result error) {
	for _, node := range g.nodes {
		result = errors.Join(result, node.close())
	}
	for i := range g.refs {
		result = errors.Join(result, g.finder.manager.DelRef(&g.refs[i]))
	}
	g.refs = nil
	return result
}

func (f *SMTModelFinder) populateAUFInstantiationSets(quantifiers []ast.ASTHandle, model *protomodel.ProtoModel) (result error) {
	graph := newSMTModelFinderAUFGraph(f)
	graph.model = model
	defer func() { result = errors.Join(result, graph.close()) }()
	if err := graph.build(quantifiers); err != nil {
		return err
	}
	if err := graph.extendModel(); err != nil {
		return err
	}
	return graph.publishInstantiationSets()
}
