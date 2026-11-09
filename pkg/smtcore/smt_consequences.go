package smtcore

// Source: src/smt/smt_consequences.cpp:27-428, context::get_consequences and
// its helpers (antecedent2fml, extract_fixed_consequences, justify,
// delete_unfixed, extract_fixed_eqs, mk_diseq), reached through
// smt_solver::get_consequences_core.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// smtIndexSet is index_set (uint_set) of Boolean variables, iterated in
// increasing order.
type smtIndexSet []int32

func (s smtIndexSet) union(other smtIndexSet) smtIndexSet {
	if len(other) == 0 {
		return s
	}
	merged := make(smtIndexSet, 0, len(s)+len(other))
	merged = append(merged, s...)
	merged = append(merged, other...)
	sort.Slice(merged, func(i, j int) bool { return merged[i] < merged[j] })
	out := merged[:0]
	for i, v := range merged {
		if i == 0 || v != merged[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func (s smtIndexSet) contains(v int32) bool {
	i := sort.Search(len(s), func(i int) bool { return s[i] >= v })
	return i < len(s) && s[i] == v
}

// smtConsequences holds the context fields get_consequences uses:
// m_antecedents, m_var2val, m_var2orig and m_assumption2orig.
type smtConsequences struct {
	k               *SMTKernel
	m               *ast.ASTManager
	antecedents     map[int32]smtIndexSet
	var2val         seqObjHashtable
	var2orig        map[ast.ASTHandle]ast.ASTHandle
	assumption2orig map[ast.ASTHandle]ast.ASTHandle
	refs            []ast.ASTRef // expr_ref locals and the trail
	conseq          []ast.ASTHandle
	unfixed         []ast.ASTHandle
}

func (c *smtConsequences) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := c.m.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, ref)
	return h, nil
}

func (c *smtConsequences) release() error {
	var err error
	for i := len(c.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, c.m.DelRef(&c.refs[i]))
	}
	c.refs = nil
	return err
}

func (c *smtConsequences) find(v int32) smtIndexSet { return c.antecedents[v] }

// antecedent2fml builds the conjunction of the assumption literals in vars.
func (c *smtConsequences) antecedent2fml(vars smtIndexSet) (ast.ASTHandle, error) {
	s := c.k.internalizer
	premises := make([]ast.ASTHandle, 0, len(vars))
	for _, v := range vars {
		e := s.boolExpr[v]
		if orig, ok := c.assumption2orig[e]; ok {
			e = orig
		}
		value, err := s.Assignment(NewSMTLiteral(v, false))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if value == lbool.False {
			if e, err = c.hold(c.m.MkNot(e)); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		premises = append(premises, e)
	}
	return c.hold(c.m.MkAndArray(premises))
}

func (c *smtConsequences) justify(lit SMTLiteral, s smtIndexSet) (smtIndexSet, error) {
	state := c.k.internalizer
	js := state.booleanData[lit.Var()].Justification()
	switch js.Kind() {
	case SMTBooleanJustificationClause:
		if cls, ok := js.Clause(); ok && cls != nil {
			for _, lit2 := range cls.Literals() {
				if lit2.Var() != lit.Var() {
					s = s.union(c.find(lit2.Var()))
				}
			}
		}
	case SMTBooleanJustificationBinaryClause:
		if other, ok := js.Literal(); ok {
			s = s.union(c.find(other.Var()))
		}
	case SMTBooleanJustificationAxiom:
	case SMTBooleanJustificationObject:
		if j, ok := js.Justification(); ok && j != nil {
			for _, l := range state.traceJustificationLiterals(j) {
				s = s.union(c.find(l.Var()))
			}
		}
	}
	return s, nil
}

func (c *smtConsequences) value(k ast.ASTHandle) ast.ASTHandle {
	i, err := c.var2val.find(k)
	if err != nil || i < 0 {
		return ast.ASTHandle{}
	}
	return c.var2val.table[i].value
}

func (c *smtConsequences) extractFixedLiteral(lit SMTLiteral, assumptions smtIndexSet) error {
	if lit == SMTTrueLiteral {
		return nil
	}
	m := c.m
	e := c.k.internalizer.boolExpr[lit.Var()]
	var s smtIndexSet
	if assumptions.contains(lit.Var()) {
		s = smtIndexSet{lit.Var()}
	} else {
		var err error
		if s, err = c.justify(lit, s); err != nil {
			return err
		}
	}
	c.antecedents[lit.Var()] = s
	found := false
	var fml ast.ASTHandle
	var err error
	if ok, _ := c.var2val.contains(e); ok {
		found = true
		if err = c.var2val.remove(e); err != nil {
			return err
		}
		e = c.var2orig[e]
		fml = e
		if lit.Sign() {
			if fml, err = c.hold(m.MkNot(e)); err != nil {
				return err
			}
		}
	} else if e1, e2, isEq := c.eqArgs(e); !lit.Sign() && isEq {
		v1, _ := m.IsValue(e1)
		v2, _ := m.IsValue(e2)
		if ok, _ := c.var2val.contains(e2); ok && v1 {
			found = true
			if err = c.var2val.remove(e2); err != nil {
				return err
			}
			e2 = c.var2orig[e2]
			e1, e2 = e2, e1
			if fml, err = c.hold(m.MkEq(e1, e2)); err != nil {
				return err
			}
		} else if ok, _ := c.var2val.contains(e1); ok && v2 {
			found = true
			if err = c.var2val.remove(e1); err != nil {
				return err
			}
			e1 = c.var2orig[e1]
			if fml, err = c.hold(m.MkEq(e1, e2)); err != nil {
				return err
			}
		}
	} else if !lit.Sign() {
		if arg, ctor, ok := c.recognizer(e); ok {
			if contained, _ := c.var2val.contains(arg); contained {
				found = true
				if fml, err = c.hold(m.MkEq(arg, ctor)); err != nil {
					return err
				}
				if err = c.var2val.remove(arg); err != nil {
					return err
				}
			}
		}
	}
	if found {
		ante, err := c.antecedent2fml(s)
		if err != nil {
			return err
		}
		if fml, err = c.hold(m.MkImplies(ante, fml)); err != nil {
			return err
		}
		c.conseq = append(c.conseq, fml)
	}
	return nil
}

func (c *smtConsequences) eqArgs(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
	op, basic, err := c.k.internalizer.basicOperation(e)
	if err != nil || !basic || op != ast.ASTOpEq {
		return ast.ASTHandle{}, ast.ASTHandle{}, false
	}
	e1, err1 := c.m.GetArg(e, 0)
	e2, err2 := c.m.GetArg(e, 1)
	if err1 != nil || err2 != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false
	}
	return e1, e2, true
}

// recognizer returns the argument and constructor constant of a datatype
// recognizer application.
func (c *smtConsequences) recognizer(e ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
	kind, err := c.m.GetKind(e)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ASTHandle{}, false
	}
	decl, err := c.m.GetDecl(e)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false
	}
	ctorDecl, ok, err := c.m.DatatypeRecognizerConstructor(decl)
	if err != nil || !ok {
		return ast.ASTHandle{}, ast.ASTHandle{}, false
	}
	arg, err := c.m.GetArg(e, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false
	}
	h, err := c.hold(c.m.MkApp(ctorDecl, nil))
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false
	}
	return arg, h, true
}

// popToSearchLevel is pop_to_search_lvl.
func (c *smtConsequences) popToSearchLevel(searchLevel uint32) error {
	s := c.k.internalizer
	if s.scopeLevel > searchLevel {
		return s.PopScopes(s.scopeLevel - searchLevel)
	}
	return nil
}

func (c *smtConsequences) extractFixed(start *int, searchLevel uint32, assumptions smtIndexSet) error {
	if err := c.popToSearchLevel(searchLevel); err != nil {
		return err
	}
	lits := c.k.internalizer.assignedLiterals
	sz := len(lits)
	for i := *start; i < sz; i++ {
		if err := c.extractFixedLiteral(c.k.internalizer.assignedLiterals[i], assumptions); err != nil {
			return err
		}
	}
	*start = sz
	return nil
}

func (c *smtConsequences) isBool(e ast.ASTHandle) bool {
	b, err := c.k.internalizer.isBoolean(e)
	return err == nil && b
}

// mkDiseq is context::mk_diseq.
func (c *smtConsequences) mkDiseq(e, val ast.ASTHandle) (SMTLiteral, error) {
	s := c.k.internalizer
	if c.isBool(e) {
		if v, found := s.boolVar.lookup(e); found {
			return NewSMTLiteral(v, c.m.IsTrue(val)), nil
		}
		if err := s.Internalize(e, false); err != nil {
			return SMTNullLiteral, err
		}
		v, _ := s.boolVar.lookup(e)
		return NewSMTLiteral(v, !c.m.IsTrue(val)), nil
	}
	eq, err := c.hold(s.mkEqualityAtom(e, val))
	if err != nil {
		return SMTNullLiteral, err
	}
	if err = s.Internalize(eq, false); err != nil {
		return SMTNullLiteral, err
	}
	v, _ := s.boolVar.lookup(eq)
	return NewSMTLiteral(v, true), nil
}

// forcePhase is context::force_phase(v, phase).
func (c *smtConsequences) forcePhase(v int32, phase bool) {
	data := &c.k.internalizer.booleanData[v]
	data.PhaseAvailable = true
	data.Phase = phase
}

// deleteUnfixed is context::delete_unfixed.
func (c *smtConsequences) deleteUnfixed() (int, error) {
	s := c.k.internalizer
	var toDelete []ast.ASTHandle
	for _, entry := range c.var2val.entries() {
		k, v := entry.e, entry.value
		if c.isBool(k) {
			variable, found := s.boolVar.lookup(k)
			value := lbool.Undef
			if found {
				var err error
				if value, err = s.Assignment(NewSMTLiteral(variable, false)); err != nil {
					return 0, err
				}
			}
			switch value {
			case lbool.True:
				if c.m.IsFalse(v) {
					toDelete = append(toDelete, k)
				} else {
					c.forcePhase(variable, false)
				}
			case lbool.False:
				if c.m.IsTrue(v) {
					toDelete = append(toDelete, k)
				} else {
					c.forcePhase(variable, true)
				}
			default:
				toDelete = append(toDelete, k)
			}
			continue
		}
		if node := s.context.enodes.get(k); node != nil {
			distinct, err := c.m.AreDistinct(v, node.root.owner)
			if err != nil {
				return 0, err
			}
			if distinct {
				toDelete = append(toDelete, k)
				continue
			}
		}
		lit, err := c.mkDiseq(k, v)
		if err != nil {
			return 0, err
		}
		value, err := s.Assignment(lit)
		if err != nil {
			return 0, err
		}
		if value == lbool.True {
			toDelete = append(toDelete, k)
		}
	}
	for _, k := range toDelete {
		if err := c.var2val.remove(k); err != nil {
			return 0, err
		}
		c.unfixed = append(c.unfixed, k)
	}
	return len(toDelete), nil
}

// extractFixedEqs is context::extract_fixed_eqs.
func (c *smtConsequences) extractFixedEqs() (int, error) {
	s := c.k.internalizer
	var toDelete []ast.ASTHandle
	for _, entry := range c.var2val.entries() {
		k, v := entry.e, entry.value
		if c.isBool(k) {
			continue
		}
		nk, nv := s.context.enodes.get(k), s.context.enodes.get(v)
		if nk == nil || nv == nil || nk.root != nv.root {
			continue
		}
		e := NewSMTAntecedentExpander()
		e.literalOf = (&SMTConflictResolver{state: s}).enodeLiteral
		literals, err := e.ExpandEquality(nv, nk)
		if err != nil {
			return 0, err
		}
		var set smtIndexSet
		for _, l := range literals {
			set = set.union(c.find(l.Var()))
		}
		fml, err := c.hold(c.m.MkEq(c.var2orig[k], v))
		if err != nil {
			return 0, err
		}
		ante, err := c.antecedent2fml(set)
		if err != nil {
			return 0, err
		}
		if fml, err = c.hold(c.m.MkImplies(ante, fml)); err != nil {
			return 0, err
		}
		c.conseq = append(c.conseq, fml)
		toDelete = append(toDelete, k)
		clause := make([]SMTLiteral, 0, len(literals)+1)
		for _, l := range literals {
			clause = append(clause, l.Negated())
		}
		lit, err := c.mkDiseq(k, v)
		if err != nil {
			return 0, err
		}
		clause = append(clause, lit)
		if err = s.addClause(clause, false, ast.ASTHandle{}); err != nil {
			return 0, err
		}
	}
	for _, k := range toDelete {
		if err := c.var2val.remove(k); err != nil {
			return 0, err
		}
	}
	return len(toDelete), nil
}

// clearConflict is m_conflict = null_b_justification; m_not_l = null_literal.
func (c *smtConsequences) clearConflict() {
	s := c.k.internalizer
	s.inconsistent = false
	s.conflict = NullSMTBooleanJustification()
	s.conflictLiteral = SMTNullLiteral
}

// resumeSearch continues the latest search for get_consequences, which calls
// bounded_search after check without init_search and outside search(), so
// m_searching is false.
func (k *SMTKernel) resumeSearch(searchLevel uint32, assumptions bool) (*smtSearchSession, error) {
	last := k.lastSearch
	if last == nil || k.caseSplitQueue == nil {
		return nil, ast.ErrASTDomain
	}
	session := &smtSearchSession{queue: k.caseSplitQueue, baseLevel: searchLevel, conflictBase: last.conflictBase, restarts: last.restarts}
	timer, err := stopwatch.NewTimer(k.timerClock)
	if err != nil {
		return nil, err
	}
	session.timer = timer
	resolver, err := NewSMTConflictResolver(k.internalizer)
	if err != nil {
		return nil, errors.Join(err, timer.Close())
	}
	session.resolver = resolver
	if err = k.internalizer.beginSearchPersistence(searchLevel, k.smtState); err != nil {
		return nil, errors.Join(err, resolver.Close(), timer.Close())
	}
	k.internalizer.searching = false
	if assumptions {
		k.internalizer.searchUserBaseLevel--
	}
	return session, nil
}

// GetConsequences is context::get_consequences. Consequences are borrowed
// from the returned refs, which the caller releases.
func (k *SMTKernel) GetConsequences(assumptions0, vars0 []ast.ASTHandle, evaluate func(ast.ASTHandle) (ast.ASTRef, error)) (status lbool.LBool, conseq []ast.ASTRef, resultErr error) {
	m := k.manager
	c := &smtConsequences{k: k, m: m, antecedents: map[int32]smtIndexSet{0: nil}, var2val: newSeqObjHashtable(m),
		var2orig: map[ast.ASTHandle]ast.ASTHandle{}, assumption2orig: map[ast.ASTHandle]ast.ASTHandle{}}
	defer func() {
		if resultErr == nil {
			for _, f := range c.conseq {
				ref, err := m.NewRef(f)
				if err != nil {
					resultErr = err
					break
				}
				conseq = append(conseq, ref)
			}
		}
		resultErr = errors.Join(resultErr, c.release())
	}()
	if err := k.popToBaseLevel(); err != nil {
		return lbool.Undef, nil, err
	}
	pushed := false
	pop := func() error {
		if pushed {
			return k.Pop(1)
		}
		return nil
	}
	fresh := func(prefix string, e ast.ASTHandle) (ast.ASTHandle, error) {
		if !pushed {
			pushed = true
			if err := k.Push(); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		sort, err := m.GetSort(e)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		cst, err := c.hold(m.MkFreshConst(prefix, sort))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		eq, err := c.hold(m.MkEq(cst, e))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return cst, k.Assert(eq, ast.ASTHandle{})
	}
	vars := make([]ast.ASTHandle, 0, len(vars0))
	for _, v := range vars0 {
		if ok, err := smtIsUninterpretedConstant(m, v); err != nil {
			return lbool.Undef, nil, err
		} else if ok {
			vars = append(vars, v)
			c.var2orig[v] = v
			continue
		}
		cst, err := fresh("v", v)
		if err != nil {
			return lbool.Undef, nil, err
		}
		vars = append(vars, cst)
		c.var2orig[cst] = v
	}
	assumptions := make([]ast.ASTHandle, 0, len(assumptions0))
	for _, a := range assumptions0 {
		if ok, err := smtIsUninterpretedConstant(m, a); err != nil {
			return lbool.Undef, nil, err
		} else if ok {
			assumptions = append(assumptions, a)
			c.assumption2orig[a] = a
			continue
		}
		cst, err := fresh("a", a)
		if err != nil {
			return lbool.Undef, nil, err
		}
		assumptions = append(assumptions, cst)
		c.assumption2orig[cst] = a
	}
	isSat, err := k.CheckAssumptions(assumptions)
	if err != nil {
		return lbool.Undef, nil, err
	}
	if isSat != lbool.True {
		return isSat, nil, pop()
	}
	if k.lastSearch == nil {
		return lbool.Undef, nil, ast.ErrASTDomain
	}
	searchLevel := k.lastSearch.baseLevel
	var assumptionSet smtIndexSet
	for _, a := range assumptions {
		lit, err := k.internalizer.Literal(a)
		if err != nil {
			return lbool.Undef, nil, err
		}
		assumptionSet = assumptionSet.union(smtIndexSet{lit.Var()})
	}
	for _, v := range vars {
		ref, err := evaluate(v)
		if err != nil {
			return lbool.Undef, nil, err
		}
		val, err := m.RefGet(ref)
		if err == nil {
			_, err = c.hold(val, nil)
		}
		err = errors.Join(err, m.DelRef(&ref))
		if err != nil {
			return lbool.Undef, nil, err
		}
		isValue, err := m.IsValue(val)
		if err != nil {
			return lbool.Undef, nil, err
		}
		if isValue {
			if err = c.var2val.insertValue(v, val); err != nil {
				return lbool.Undef, nil, err
			}
		} else {
			c.unfixed = append(c.unfixed, v)
		}
	}
	numUnits := 0
	if err = c.extractFixed(&numUnits, searchLevel, assumptionSet); err != nil {
		return lbool.Undef, nil, err
	}
	session, err := k.resumeSearch(searchLevel, len(assumptions) != 0)
	if err != nil {
		return lbool.Undef, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, k.endSearch(session)) }()
	const chunkSize = 100
	for c.var2val.size != 0 {
		numVars := 0
		for _, entry := range c.var2val.entries() {
			if numVars >= chunkSize {
				break
			}
			if canceled, err := m.Canceled(); err != nil {
				return lbool.Undef, nil, err
			} else if canceled {
				return lbool.Undef, nil, pop()
			}
			lit, err := c.mkDiseq(entry.e, entry.value)
			if err != nil {
				return lbool.Undef, nil, err
			}
			if err = k.internalizer.markRelevant(k.internalizer.boolExpr[lit.Var()]); err != nil {
				return lbool.Undef, nil, err
			}
			value, err := k.internalizer.Assignment(lit)
			if err != nil {
				return lbool.Undef, nil, err
			}
			if value != lbool.Undef {
				continue
			}
			numVars++
			if err = k.internalizer.PushScope(); err != nil {
				return lbool.Undef, nil, err
			}
			if err = k.internalizer.assignDecisionLiteral(lit); err != nil {
				return lbool.Undef, nil, err
			}
			consistent, err := k.internalizer.PropagateCore()
			if err != nil {
				return lbool.Undef, nil, err
			}
			if !consistent {
				resolved, err := k.resolveSearchConflict(session)
				if err != nil {
					return lbool.Undef, nil, err
				}
				if !resolved || k.internalizer.inconsistent {
					c.clearConflict()
				}
			}
		}
		var isSat lbool.LBool
		for {
			status, restartDue, err := k.boundedSearch(session)
			if err != nil {
				return lbool.Undef, nil, err
			}
			if restartDue {
				// inc_limits
				if err = session.restarts.advance(k.smtState); err != nil {
					return lbool.Undef, nil, err
				}
				continue
			}
			if status == lbool.Undef {
				return status, nil, pop()
			}
			isSat = status
			break
		}
		if isSat == lbool.False {
			c.clearConflict()
		}
		if isSat == lbool.True {
			if _, err = c.deleteUnfixed(); err != nil {
				return lbool.Undef, nil, err
			}
		}
		if err = c.extractFixed(&numUnits, searchLevel, assumptionSet); err != nil {
			return lbool.Undef, nil, err
		}
		if _, err = c.extractFixedEqs(); err != nil {
			return lbool.Undef, nil, err
		}
	}
	return lbool.True, nil, pop()
}

// smtIsUninterpretedConstant is is_uninterp_const.
func smtIsUninterpretedConstant(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, err
	}
	decl, err := m.GetDecl(h)
	if err != nil {
		return false, err
	}
	family, err := m.GetFamilyID(decl)
	return family == ast.NullFamilyID, err
}
