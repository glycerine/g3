package ast

// Source: src/ast/ast_translation.cpp; src/ast/ast_translation.h.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type astTranslationParameter struct {
	kind     ASTParameterKind
	integer  int32
	ast      ASTHandle
	symbol   usymbol.Symbol
	rational unumeral.OwnedRational
	double   float64
	external ASTParameter
}

type astTranslationInfo struct {
	present bool
	family  FamilyID
	kind    int32
	params  []astTranslationParameter
	size    SortSize
	flags   ASTFuncFlags
	private bool
}

type astTranslationNode struct {
	kind             ASTKind
	refCount         uint32
	name             usymbol.Symbol
	info             astTranslationInfo
	index            uint32
	forall           bool
	declarationCount uint32
	patternCount     uint32
	noPatternCount   uint32
	declarationNames []usymbol.Symbol
	weight           int32
	qid, skolemID    usymbol.Symbol
}

// ASTTranslation translates manager-owned AST DAGs and retains the source
// cache policy: only nodes with more than one source reference remain pinned
// between calls.
type ASTTranslation struct {
	from, to *ASTManager
	cache    *ObjMap // obj_map<ast, ast*> m_cache, keyed in the source manager
	persist  bool
	closed   bool
}

func NewASTTranslation(from, to *ASTManager, copyPlugins bool) (*ASTTranslation, error) {
	return newASTTranslation(from, to, copyPlugins, true)
}

func newASTTranslation(from, to *ASTManager, copyPlugins, persist bool) (*ASTTranslation, error) {
	if from == nil || to == nil || from == to {
		return nil, ErrASTHandle
	}
	t := &ASTTranslation{from: from, to: to, cache: NewObjMap(from), persist: persist}
	if copyPlugins {
		if err := to.CopyFamiliesPlugins(from); err != nil {
			return nil, err
		}
		if err := t.copyDatatypeDefinitions(); err != nil {
			_ = t.Close()
			return nil, err
		}
	}
	return t, nil
}

func (t *ASTTranslation) From() *ASTManager { return t.from }
func (t *ASTTranslation) To() *ASTManager   { return t.to }

func (t *ASTTranslation) ResetCache() error {
	if t == nil || t.closed {
		return ErrASTHandle
	}
	var result error
	for _, entry := range t.cache.Entries() {
		result = errors.Join(result, t.from.DecRef(entry.Key), t.to.DecRef(entry.Value[0]))
	}
	t.cache.Reset()
	return result
}

func (t *ASTTranslation) Close() error {
	if t == nil || t.closed {
		return nil
	}
	err := t.ResetCache()
	t.closed = true
	t.from, t.to, t.cache = nil, nil, nil
	return err
}

func (t *ASTTranslation) Translate(source ASTHandle) (ASTHandle, error) {
	if t == nil || t.closed || t.from == nil || t.to == nil {
		return ASTHandle{}, ErrASTHandle
	}
	if source.IsNull() {
		return ASTHandle{}, nil
	}
	// A frame owns its suffix in the source-child and translated-result
	// stacks. After its last child, replace that suffix with the new parent.
	// Shared nodes use the persistent cache or a map local to this call.
	type frame struct {
		source                             ASTHandle
		node                               astTranslationNode
		firstChild, nextChild, firstResult int
	}
	var frames []frame
	var children, results []ASTHandle
	var local map[ASTHandle]ASTHandle
	if !t.persist {
		local = make(map[ASTHandle]ASTHandle)
	}
	next := source
	for {
		var cached ASTHandle
		if t.persist {
			value, found, err := t.cache.Find(next)
			if err != nil {
				return ASTHandle{}, err
			}
			if found {
				cached = value[0]
			}
		} else {
			cached = local[next]
		}
		if cached.IsNull() {
			start := len(children)
			node, err := t.snapshotNode(next, &children)
			if err != nil {
				return ASTHandle{}, err
			}
			frames = append(frames, frame{source: next, node: node, firstChild: start, nextChild: start, firstResult: len(results)})
		} else {
			results = append(results, cached)
		}
		for len(frames) > 0 {
			top := &frames[len(frames)-1]
			if top.nextChild < len(children) {
				next = children[top.nextChild]
				top.nextChild++
				break
			}
			result, err := t.buildNode(&top.node, results[top.firstResult:])
			if err != nil {
				return ASTHandle{}, err
			}
			if top.node.refCount > 1 {
				if t.persist {
					if err = t.keep(top.source, result); err != nil {
						return ASTHandle{}, err
					}
				} else {
					local[top.source] = result
				}
			}
			children = children[:top.firstChild]
			results = append(results[:top.firstResult], result)
			*top = frame{}
			frames = frames[:len(frames)-1]
		}
		if len(frames) == 0 {
			return results[0], nil
		}
	}
}

func (t *ASTTranslation) keep(source, result ASTHandle) error {
	if err := t.from.IncRef(source); err != nil {
		return err
	}
	if err := t.to.IncRef(result); err != nil {
		return errors.Join(err, t.from.DecRef(source))
	}
	if err := t.cache.Insert(source, [2]ASTHandle{result}); err != nil {
		return errors.Join(err, t.to.DecRef(result), t.from.DecRef(source))
	}
	return nil
}

// TranslateAST performs one translation with a nonpersistent cache.
func TranslateAST(source ASTHandle, from, to *ASTManager) (result ASTHandle, err error) {
	t, err := newASTTranslation(from, to, true, false)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, t.Close()) }()
	return t.Translate(source)
}

func (t *ASTTranslation) snapshotNode(h ASTHandle, children *[]ASTHandle) (astTranslationNode, error) {
	m := t.from
	if err := m.lock(); err != nil {
		return astTranslationNode{}, err
	}
	defer m.mu.Unlock()
	r, err := m.internal(h)
	if err != nil {
		return astTranslationNode{}, err
	}
	n := astTranslationNode{kind: m.nodeAt(r.slot).kind, refCount: m.nodeAt(r.slot).rc}

	appendChild := func(child astNodeRef) { *children = append(*children, m.external(child)) }
	switch n.kind {
	case ASTSort:
		v := m.sortAt(r.slot)
		n.name = v.name
		n.info, err = m.snapshotInfo(v.info.slot, v.info.generation)
		if err == nil {
			for _, p := range n.info.params {
				if p.kind == ASTParameterAST {
					*children = append(*children, p.ast)
				}
			}
		}
	case ASTFuncDecl:
		v := m.declAt(r.slot)
		n.name = v.name
		n.info, err = m.snapshotInfo(v.info.slot, v.info.generation)
		if err == nil {
			for _, p := range n.info.params {
				if p.kind == ASTParameterAST {
					*children = append(*children, p.ast)
				}
			}
			for i := uint32(0); i < v.arity; i++ {
				var child astNodeRef
				child, err = m.readInternal(v.domain, i)
				if err != nil {
					break
				}
				appendChild(child)
			}
			appendChild(v.rangeSort)
		}
	case ASTVar:
		v := m.varAt(r.slot)
		n.index = v.index
		appendChild(v.sort)
	case ASTApp:
		v := m.appAt(r.slot)
		appendChild(v.decl)
		for i := uint32(0); i < v.count && err == nil; i++ {
			var child astNodeRef
			child, err = m.appArg(v, i)
			if err == nil {
				appendChild(child)
			}
		}
	case ASTQuantifier:
		v := m.quantifierAt(r.slot)
		n.forall, n.declarationCount, n.patternCount, n.noPatternCount = v.forall, v.count, v.patternCount, v.noPatternCount
		n.weight, n.qid, n.skolemID = v.weight, v.qid, v.skolemID
		for i := uint32(0); i < v.count && err == nil; i++ {
			var child astNodeRef
			child, err = m.readInternal(v.sorts, i)
			if err == nil {
				appendChild(child)
			}
			if err == nil {
				var name usymbol.Symbol
				name, err = m.readSymbol(v.names, i)
				n.declarationNames = append(n.declarationNames, name)
			}
		}
		appendChild(v.body)
		for i := uint32(0); i < v.patternCount && err == nil; i++ {
			var child astNodeRef
			child, err = m.readInternal(v.patterns, i)
			if err == nil {
				appendChild(child)
			}
		}
		for i := uint32(0); i < v.noPatternCount && err == nil; i++ {
			var child astNodeRef
			child, err = m.readInternal(v.noPatterns, i)
			if err == nil {
				appendChild(child)
			}
		}
	default:
		err = ErrASTDomain
	}

	return n, err
}

func (m *ASTManager) snapshotInfo(slot, generation uint32) (astTranslationInfo, error) {
	if slot == 0 {
		return astTranslationInfo{}, nil
	}
	r, err := m.info(slot, generation)
	if err != nil {
		return astTranslationInfo{}, err
	}
	result := astTranslationInfo{present: true, family: r.family, kind: r.kind, size: r.size, flags: r.flags, private: r.private}
	for i := uint32(0); i < r.count; i++ {
		p, e := m.infoParameter(r, i)
		if e != nil {
			return astTranslationInfo{}, e
		}
		v, e := m.parameter(p)
		if e != nil {
			return astTranslationInfo{}, e
		}
		copy := astTranslationParameter{kind: v.Kind, integer: v.integer, ast: m.external(v.ast), symbol: v.symbol, rational: v.Rational, double: v.double}
		if v.Kind == ASTParameterExternal {
			copy.external = p
		}
		result.params = append(result.params, copy)
	}
	return result, nil
}

func (t *ASTTranslation) translateSymbol(s usymbol.Symbol) (usymbol.Symbol, error) {
	if s.IsNull() || s.IsNumerical() {
		return s, nil
	}
	text, err := t.from.Symbols.String(s)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	return t.to.Symbols.Intern(text)
}

func (t *ASTTranslation) buildParameters(info astTranslationInfo, translated []ASTHandle) ([]ASTParameter, error) {
	result := make([]ASTParameter, 0, len(info.params))
	fail := func(err error) ([]ASTParameter, error) {
		for i := len(result) - 1; i >= 0; i-- {
			err = errors.Join(err, t.to.DelParameter(&result[i]))
		}
		return nil, err
	}
	for _, source := range info.params {
		var p ASTParameter
		var err error
		switch source.kind {
		case ASTParameterInt:
			p, err = t.to.NewParameterInt(source.integer)
		case ASTParameterAST:
			if len(translated) == 0 {
				return fail(ErrASTDomain)
			}
			p, err = t.to.NewParameterAST(translated[0])
			translated = translated[1:]
		case ASTParameterSymbol:
			var s usymbol.Symbol
			s, err = t.translateSymbol(source.symbol)
			if err == nil {
				p, err = t.to.NewParameterSymbol(s)
			}
		case ASTParameterRational:
			var q unumeral.OwnedRational
			q, err = t.to.numerals.NewValue()
			if err == nil {
				err = t.to.numerals.CopyFrom(&q, t.from.numerals, &source.rational)
			}
			if err == nil {
				p, err = t.to.NewParameterRational(&q)
			}
			if q != (unumeral.OwnedRational{}) {
				err = errors.Join(err, t.to.numerals.Del(&q))
			}
		case ASTParameterDouble:
			p, err = t.to.NewParameterDouble(source.double)
		case ASTParameterExternal:
			if t.from.arithmeticPlugin != nil && info.family == t.from.arithmeticPlugin.family {
				p, err = t.from.TranslateArithmeticExternalParameter(source.external, t.to)
			} else if t.from.floatingPointPlugin != nil && info.family == t.from.floatingPointPlugin.family {
				p, err = t.from.TranslateFloatingPointExternalParameter(source.external, t.to)
			} else {
				err = ErrASTDomain
			}
		default:
			err = ErrASTDomain
		}
		if err != nil {
			return fail(err)
		}
		result = append(result, p)
	}
	return result, nil
}

// TranslateDependency linearizes the source dependency DAG, translates its
// unique expression leaves in source order, and rebuilds the target join.
func (t *ASTTranslation) TranslateDependency(source ASTDependency) (ASTDependency, error) {
	if t == nil || t.closed || t.from == nil || t.to == nil {
		return ASTDependency{}, ErrASTHandle
	}
	if source.IsNull() {
		return ASTDependency{}, nil
	}
	leaves, err := t.from.DependencyLinearize(source, nil)
	if err != nil {
		return ASTDependency{}, err
	}
	translated := make([]ASTHandle, len(leaves))
	for i, leaf := range leaves {
		translated[i], err = t.Translate(leaf)
		if err != nil {
			return ASTDependency{}, err
		}
	}
	progress, err := t.to.DependencyJoinValues(translated)
	if err != nil {
		return progress.Completed, err
	}
	return progress.Completed, nil
}

func (t *ASTTranslation) buildNode(n *astTranslationNode, translated []ASTHandle) (result ASTHandle, err error) {
	switch n.kind {
	case ASTVar:
		if len(translated) != 1 {
			return ASTHandle{}, ErrASTDomain
		}
		return t.to.MkVar(n.index, translated[0])
	case ASTApp:
		if len(translated) == 0 {
			return ASTHandle{}, ErrASTDomain
		}
		return t.to.MkApp(translated[0], translated[1:])
	case ASTSort, ASTFuncDecl:
		name, e := t.translateSymbol(n.name)
		if e != nil {
			return ASTHandle{}, e
		}
		if n.kind == ASTSort && !n.info.present {
			return t.to.MkUninterpretedSort(name, nil)
		}
		params, e := t.buildParameters(n.info, translated)
		if e != nil {
			return ASTHandle{}, e
		}
		defer func() {
			for i := len(params) - 1; i >= 0; i-- {
				err = errors.Join(err, t.to.DelParameter(&params[i]))
			}
		}()
		if n.kind == ASTSort {
			info, e := t.to.NewSortInfo(n.info.family, n.info.kind, params, n.info.size, n.info.private)
			if e != nil {
				return ASTHandle{}, e
			}
			defer func() { err = errors.Join(err, t.to.DelSortInfo(&info)) }()
			return t.to.MkSort(name, info)
		}
		extra := 0
		for _, p := range n.info.params {
			if p.kind == ASTParameterAST {
				extra++
			}
		}
		domainCount := len(translated) - extra - 1
		if domainCount < 0 {
			return ASTHandle{}, ErrASTDomain
		}
		domain := translated[extra : extra+domainCount]
		rangeSort := translated[len(translated)-1]
		if !n.info.present {
			return t.to.MkFuncDecl(name, domain, rangeSort, ASTFuncDeclInfo{})
		}
		info, e := t.to.NewFuncDeclInfo(n.info.family, n.info.kind, params, n.info.flags, n.info.private)
		if e != nil {
			return ASTHandle{}, e
		}
		defer func() { err = errors.Join(err, t.to.DelFuncDeclInfo(&info)) }()
		return t.to.MkFuncDeclWithInfo(name, domain, rangeSort, info)
	case ASTQuantifier:
		if uint64(n.declarationCount)+1+uint64(n.patternCount)+uint64(n.noPatternCount) != uint64(len(translated)) {
			return ASTHandle{}, ErrASTDomain
		}
		endSorts := int(n.declarationCount)
		endPatterns := endSorts + 1 + int(n.patternCount)
		spec := ASTQuantifierSpec{
			Forall: n.forall, Weight: n.weight, Sorts: translated[:endSorts],
			Body: translated[endSorts], Patterns: translated[endSorts+1 : endPatterns],
			NoPatterns: translated[endPatterns:], Names: make([]usymbol.Symbol, endSorts),
		}
		for i, name := range n.declarationNames {
			spec.Names[i], err = t.translateSymbol(name)
			if err != nil {
				return ASTHandle{}, err
			}
		}
		spec.QID, err = t.translateSymbol(n.qid)
		if err == nil {
			spec.SkolemID, err = t.translateSymbol(n.skolemID)
		}
		if err != nil {
			return ASTHandle{}, err
		}
		return t.to.MkQuantifier(spec)
	default:
		return ASTHandle{}, ErrASTDomain
	}
}
