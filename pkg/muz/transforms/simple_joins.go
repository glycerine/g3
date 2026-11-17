package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp and .h: mk_simple_joins and its
// join_planner, which rewrites rules to at most two positive predicate tails.

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SimpleJoinsPlugin struct {
	manager   *ast.ASTManager
	rules     *base.RuleManager
	relations *rel.RelationManager
	priority  uint32
	fresh     []ast.ASTHandle
	origins   map[ast.ASTHandle]ast.ASTHandle
	// keyPins is join_planner::m_pinned for get_key's normalized pair terms
	// and apply_binary_rule's new literals: the planner is local to
	// mk_simple_joins::operator(), so they are released (front to back)
	// when Transform returns.
	keyPins []ast.ASTRef
	// proofTrace is m_context.generate_proof_trace(), which gates
	// rule_manager::mk_rule_asserted_proof for the introduced rules.
	proofTrace bool
	closed     bool
	closing    bool
}

// SetGenerateProofTrace makes the introduced rules carry asserted proofs,
// as mk_rule_asserted_proof does when the context generates proof traces.
func (p *SimpleJoinsPlugin) SetGenerateProofTrace(enabled bool) {
	p.proofTrace = enabled
}

func NewSimpleJoinsPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32, relations ...*rel.RelationManager) *SimpleJoinsPlugin {
	if priority == 0 {
		priority = 1000
	}
	plugin := &SimpleJoinsPlugin{manager: manager, rules: rules, priority: priority}
	if len(relations) > 0 {
		plugin.relations = relations[0]
	}
	return plugin
}

func (p *SimpleJoinsPlugin) Priority() uint32            { return p.priority }
func (p *SimpleJoinsPlugin) CanDestratifyNegation() bool { return false }
func (p *SimpleJoinsPlugin) Cancel()                     {}

func (p *SimpleJoinsPlugin) FreshPredicates() []ast.ASTHandle {
	if p == nil || p.closed || p.closing {
		return nil
	}
	return append([]ast.ASTHandle(nil), p.fresh...)
}

// FreshPredicateOrigin returns the parent head passed to
// context::mk_fresh_head_predicate by join_planner::join_pair. Both
// declarations are borrowed from the rule manager.
func (p *SimpleJoinsPlugin) FreshPredicateOrigin(predicate ast.ASTHandle) (ast.ASTHandle, bool) {
	if p == nil || p.closed || p.closing {
		return ast.ASTHandle{}, false
	}
	origin, found := p.origins[predicate]
	return origin, found
}

// variables collects the variables of root with their sorts.
func (p *SimpleJoinsPlugin) variables(root ast.ASTHandle, result map[uint32]ast.ASTHandle) error {
	kind, err := p.manager.GetKind(root)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		index, x := p.manager.GetVarIndex(root)
		if x != nil {
			return x
		}
		result[index], x = p.manager.GetSort(root)
		return x
	}
	if kind != ast.ASTApp {
		return nil
	}
	count, err := p.manager.GetNumArgs(root)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		child, x := p.manager.GetArg(root, index)
		if x != nil {
			return x
		}
		if x = p.variables(child, result); x != nil {
			return x
		}
	}
	return nil
}

func (p *SimpleJoinsPlugin) pin(handle ast.ASTHandle) error {
	reference, err := p.manager.NewRef(handle)
	if err == nil {
		p.keyPins = append(p.keyPins, reference)
	}
	return err
}

func (p *SimpleJoinsPlugin) releaseKeyPins() error {
	for len(p.keyPins) != 0 {
		err := p.manager.DelRef(&p.keyPins[0])
		if p.keyPins[0] == (ast.ASTRef{}) {
			p.keyPins = p.keyPins[1:]
		}
		if err != nil {
			return err
		}
	}
	p.keyPins = nil
	return nil
}

func (p *SimpleJoinsPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closing = true
	if err := p.releaseKeyPins(); err != nil {
		return err
	}
	p.fresh = nil
	p.origins = nil
	p.closed = true
	return nil
}

// SourceTypeName is typeid(datalog::mk_simple_joins).name() for the IF_VERBOSE
// transform lines of rule_transformer::operator().
func (p *SimpleJoinsPlugin) SourceTypeName() string {
	return base.DatalogPluginTypeName("mk_simple_joins")
}

// Transform is mk_simple_joins::operator(): rs_aux_copy answers stratum
// queries for the local join_planner.
func (p *SimpleJoinsPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.closing || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	auxCopy, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, auxCopy.Close()) }()
	if err = auxCopy.AddRules(source); err != nil {
		return nil, err
	}
	if _, err = auxCopy.Finalize(); err != nil {
		return nil, err
	}
	planner, err := newJoinPlanner(p, auxCopy)
	if err != nil {
		return nil, err
	}
	// ~join_planner runs before rs_aux_copy's destructor.
	defer func() { resultErr = errors.Join(resultErr, planner.close(), p.releaseKeyPins()) }()
	return planner.run(source)
}

// appPair is join_planner::app_pair.
type appPair struct {
	first, second ast.ASTHandle
}

// pairInfo is join_planner::pair_info.
type pairInfo struct {
	totalCost float32
	// consumers is the number of rules longer than two that contain this
	// pair.
	consumers       uint32
	stratified      bool
	srcStratum      uint32
	allNonlocalVars map[uint32]struct{}
	rules           []int
}

func newPairInfo() *pairInfo {
	return &pairInfo{stratified: true, allNonlocalVars: make(map[uint32]struct{})}
}

func (i *pairInfo) canBeJoined() bool { return i.consumers > 0 }

func (i *pairInfo) getCost() float32 {
	amortized := i.totalCost / float32(i.consumers)
	if i.stratified {
		if amortized > 0 {
			return amortized * (1 / 16.0)
		}
		return amortized * 16.0
	}
	return amortized
}

// addRule is pair_info::add_rule.
func (i *pairInfo) addRule(pl *joinPlanner, t1, t2 ast.ASTHandle, r int, nonLocalVarsNormalized, nonLocalVars map[uint32]struct{}) error {
	if len(i.rules) == 0 {
		cost, err := pl.computeCost(t1, t2, nonLocalVars)
		if err != nil {
			return err
		}
		i.totalCost = cost
		s1, err := pl.getStratumOf(t1)
		if err != nil {
			return err
		}
		s2, err := pl.getStratumOf(t2)
		if err != nil {
			return err
		}
		i.srcStratum = max(s1, s2)
	}
	i.rules = append(i.rules, r)
	if len(pl.rulesContent[r]) > 2 {
		i.consumers++
	}
	if i.stratified {
		headStratum, err := pl.getStratum(pl.heads[r])
		if err != nil {
			return err
		}
		if headStratum == i.srcStratum {
			i.stratified = false
		}
	}
	for v := range nonLocalVarsNormalized {
		i.allNonlocalVars[v] = struct{}{}
	}
	return nil
}

// removeRule is pair_info::remove_rule: true when no rules remain.
func (i *pairInfo) removeRule(r int, originalLength int) (bool, error) {
	found := false
	for index, candidate := range i.rules {
		if candidate == r {
			last := len(i.rules) - 1
			i.rules[index], i.rules[last] = i.rules[last], i.rules[index]
			i.rules = i.rules[:last]
			found = true
			break
		}
	}
	if !found {
		return false, fmt.Errorf("muz/transforms: join pair does not hold the removed rule")
	}
	if originalLength > 2 {
		i.consumers--
	}
	return len(i.rules) == 0, nil
}

// joinPlanner is datalog::join_planner. Rules are identified by their index
// in the source rule set; m_rules_content is iterated in that order.
type joinPlanner struct {
	p        *SimpleJoinsPlugin
	m        *ast.ASTManager
	varSubst *astrewriter.VarSubstitution
	auxCopy  *base.RuleSet
	// costs is m_costs: costOrder reproduces the core_hashtable slot order
	// of the pair map; costInfo holds each pair_info.
	costOrder    *ast.ObjMap
	costHash     map[appPair]uint32
	costInfo     map[appPair]*pairInfo
	sourceRules  []*base.Rule
	heads        []ast.ASTHandle
	rulesContent [][]ast.ASTHandle
	introduced   []*base.Rule
	modified     bool
}

func newJoinPlanner(p *SimpleJoinsPlugin, auxCopy *base.RuleSet) (*joinPlanner, error) {
	// m_var_subst is the context's var_subst (std_order).
	varSubst, err := astrewriter.NewVarSubstitution(p.manager, true)
	if err != nil {
		if varSubst != nil {
			err = errors.Join(err, varSubst.Close())
		}
		return nil, err
	}
	return &joinPlanner{p: p, m: p.manager, varSubst: varSubst, auxCopy: auxCopy, costOrder: ast.NewObjMap(p.manager), costHash: make(map[appPair]uint32), costInfo: make(map[appPair]*pairInfo)}, nil
}

func (pl *joinPlanner) close() (result error) {
	for i := len(pl.introduced) - 1; i >= 0; i-- {
		if pl.introduced[i] != nil {
			result = errors.Join(result, pl.introduced[i].Close())
		}
	}
	pl.introduced = nil
	for _, r := range pl.sourceRules {
		result = errors.Join(result, r.Close())
	}
	pl.sourceRules = nil
	if pl.varSubst != nil {
		result = errors.Join(result, pl.varSubst.Close())
		pl.varSubst = nil
	}
	return result
}

func (pl *joinPlanner) argVar(t ast.ASTHandle, index uint32) (uint32, ast.ASTHandle, error) {
	argument, err := pl.m.GetArg(t, index)
	if err != nil {
		return 0, ast.ASTHandle{}, err
	}
	kind, err := pl.m.GetKind(argument)
	if err != nil {
		return 0, ast.ASTHandle{}, err
	}
	if kind != ast.ASTVar {
		// The planner runs after mk_filter_rules: pair literals hold only
		// variables.
		return 0, ast.ASTHandle{}, fmt.Errorf("muz/transforms: join planner expects variable arguments: %w", ast.ErrASTDomain)
	}
	index2, err := pl.m.GetVarIndex(argument)
	return index2, argument, err
}

// getNormalizerInto is get_normalizer(app*, next_var, result).
func (pl *joinPlanner) getNormalizerInto(t ast.ASTHandle, nextVar *uint32, result []ast.ASTHandle) error {
	resOfs := uint32(len(result) - 1)
	n, err := pl.m.GetNumArgs(t)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		varIdx, v, err := pl.argVar(t, i)
		if err != nil {
			return err
		}
		if result[resOfs-varIdx].IsNull() {
			s, err := pl.m.GetSort(v)
			if err != nil {
				return err
			}
			if result[resOfs-varIdx], err = pl.m.MkVar(*nextVar, s); err != nil {
				return err
			}
			*nextVar++
		}
	}
	return nil
}

// getNormalizer is get_normalizer(t1, t2, result).
func (pl *joinPlanner) getNormalizer(t1, t2 ast.ASTHandle) ([]ast.ASTHandle, error) {
	n1, err := pl.m.GetNumArgs(t1)
	if err != nil {
		return nil, err
	}
	n2, err := pl.m.GetNumArgs(t2)
	if err != nil {
		return nil, err
	}
	if n1 == 0 && n2 == 0 {
		return nil, nil //nothing to normalize
	}
	vars := make(map[uint32]ast.ASTHandle)
	if err = pl.p.variables(t1, vars); err != nil {
		return nil, err
	}
	if err = pl.p.variables(t2, vars); err != nil {
		return nil, err
	}
	maxVarIdx := uint32(0)
	for varIdx := range vars {
		if varIdx > maxVarIdx {
			maxVarIdx = varIdx
		}
	}
	d1, err := pl.m.GetDecl(t1)
	if err != nil {
		return nil, err
	}
	d2, err := pl.m.GetDecl(t2)
	if err != nil {
		return nil, err
	}
	if d1 != d2 {
		id1, err := pl.m.GetASTID(d1)
		if err != nil {
			return nil, err
		}
		id2, err := pl.m.GetASTID(d2)
		if err != nil {
			return nil, err
		}
		if id1 < id2 {
			t1, t2 = t2, t1
		}
	} else {
		norm1 := make([]int, maxVarIdx+1)
		norm2 := make([]int, maxVarIdx+1)
		for i := range norm1 {
			norm1[i], norm2[i] = -1, -1
		}
		for i := uint32(0); i < n1; i++ {
			v1Idx, v1, err := pl.argVar(t1, i)
			if err != nil {
				return nil, err
			}
			v2Idx, v2, err := pl.argVar(t2, i)
			if err != nil {
				return nil, err
			}
			s1, err := pl.m.GetSort(v1)
			if err != nil {
				return nil, err
			}
			s2, err := pl.m.GetSort(v2)
			if err != nil {
				return nil, err
			}
			if s1 != s2 {
				//different sorts mean we can distinguish the two terms
				id1, err := pl.m.GetASTID(s1)
				if err != nil {
					return nil, err
				}
				id2, err := pl.m.GetASTID(s2)
				if err != nil {
					return nil, err
				}
				if id1 < id2 {
					t1, t2 = t2, t1
				}
				break
			}
			if norm2[v1Idx] != norm1[v2Idx] {
				//now we can distinguish the two terms
				if norm2[v1Idx] < norm1[v2Idx] {
					t1, t2 = t2, t1
				}
				break
			}
			norm1[v1Idx] = int(i)
			norm2[v2Idx] = int(i)
		}
		//if we did not exit the loop prematurely, the two terms are indistinguishable,
		//so the order should not matter
	}
	result := make([]ast.ASTHandle, maxVarIdx+1)
	nextVar := uint32(0)
	if err = pl.getNormalizerInto(t1, &nextVar, result); err != nil {
		return nil, err
	}
	if err = pl.getNormalizerInto(t2, &nextVar, result); err != nil {
		return nil, err
	}
	return result, nil
}

// substitute is m_var_subst(t, subst.size(), subst.c_ptr(), result); the
// result is pinned in m_pinned by the caller's use.
func (pl *joinPlanner) substitute(t ast.ASTHandle, subst []ast.ASTHandle) (ast.ASTHandle, error) {
	out, err := pl.m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = pl.varSubst.Substitute(t, subst, out); err != nil {
		return ast.ASTHandle{}, errors.Join(err, pl.m.DelRef(&out))
	}
	pl.p.keyPins = append(pl.p.keyPins, out)
	return pl.m.RefGet(out)
}

// getKey is join_planner::get_key.
// The normalized terms are pinned (m_pinned) after the swap, smaller id
// first.
func (pl *joinPlanner) getKey(t1, t2 ast.ASTHandle) (_ appPair, resultErr error) {
	normSubst, err := pl.getNormalizer(t1, t2)
	if err != nil {
		return appPair{}, err
	}
	refs := make([]ast.ASTRef, 0, 2)
	defer func() {
		for i := range refs {
			if refs[i] != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, pl.m.DelRef(&refs[i]))
			}
		}
	}()
	handles := make([]ast.ASTHandle, 0, 2)
	for _, t := range []ast.ASTHandle{t1, t2} {
		out, err := pl.m.NewRef(ast.ASTHandle{})
		if err != nil {
			return appPair{}, err
		}
		refs = append(refs, out)
		if err = pl.varSubst.Substitute(t, normSubst, out); err != nil {
			return appPair{}, err
		}
		h, err := pl.m.RefGet(out)
		if err != nil {
			return appPair{}, err
		}
		handles = append(handles, h)
	}
	t1n, t2n := handles[0], handles[1]
	id1, err := pl.m.GetASTID(t1n)
	if err != nil {
		return appPair{}, err
	}
	id2, err := pl.m.GetASTID(t2n)
	if err != nil {
		return appPair{}, err
	}
	if id1 > id2 {
		t1n, t2n = t2n, t1n
		refs[0], refs[1] = refs[1], refs[0]
	}
	// m_pinned.push_back(t1n); m_pinned.push_back(t2n): the pins take over
	// the substitution results' references.
	pl.p.keyPins = append(pl.p.keyPins, refs...)
	refs = nil
	return appPair{first: t1n, second: t2n}, nil
}

// insertCost is m_costs.insert_if_not_there2(key, nullptr).
func (pl *joinPlanner) insertCost(key appPair) (*pairInfo, error) {
	if info := pl.costInfo[key]; info != nil {
		return info, nil
	}
	hash, err := pl.costOrder.PairHash(key.first, key.second)
	if err != nil {
		return nil, err
	}
	pl.costOrder.InsertPairKeys(key.first, key.second, hash)
	pl.costHash[key] = hash
	info := newPairInfo()
	pl.costInfo[key] = info
	return info, nil
}

func (pl *joinPlanner) eraseCost(key appPair) {
	pl.costOrder.ErasePairKeys(key.first, key.second, pl.costHash[key])
	delete(pl.costHash, key)
	delete(pl.costInfo, key)
}

func sortedVarSet(set map[uint32]struct{}) []uint32 {
	result := make([]uint32, 0, len(set))
	for v := range set {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// registerPair adds rule r among rules interested in predicate pair t1, t2.
func (pl *joinPlanner) registerPair(t1, t2 ast.ASTHandle, r int, nonLocalVars map[uint32]struct{}) error {
	key, err := pl.getKey(t1, t2)
	if err != nil {
		return err
	}
	inf, err := pl.insertCost(key)
	if err != nil {
		return err
	}
	normalizer, err := pl.getNormalizer(t1, t2)
	if err != nil {
		return err
	}
	normOfs := len(normalizer) - 1
	normalizedVars := make(map[uint32]struct{})
	for v := range nonLocalVars {
		normVar, err := pl.m.GetVarIndex(normalizer[normOfs-int(v)])
		if err != nil {
			return err
		}
		normalizedVars[normVar] = struct{}{}
	}
	if err = inf.addRule(pl, t1, t2, r, normalizedVars, nonLocalVars); err != nil {
		return err
	}
	return pl.traceRegisterPair(t1, t2, r, nonLocalVars, inf)
}

func (pl *joinPlanner) traceRegisterPair(t1, t2 ast.ASTHandle, r int, nonLocalVars map[uint32]struct{}, inf *pairInfo) error {
	return pl.m.EmitTrace("dl", "register_pair", "src/muz/rel/dl_mk_simple_joins.cpp", 290, func(w io.Writer) error {
		return pl.p.writeRegisterPairTrace(w, t1, t2, sortedVarSet(nonLocalVars), pl.sourceRules[r], inf)
	})
}

func (pl *joinPlanner) removeRuleFromPair(key appPair, r int, originalLen int) error {
	inf := pl.costInfo[key]
	if inf == nil {
		return nil
	}
	empty, err := inf.removeRule(r, originalLen)
	if err != nil {
		return err
	}
	if empty {
		pl.eraseCost(key)
	}
	return nil
}

// varCounter is var_counter (a counter over variable indices).
type varCounter map[uint32]int

// countVars is var_counter::count_vars(app, coef): every free variable of
// each argument is counted once for that argument.
func (pl *joinPlanner) countVars(counter varCounter, t ast.ASTHandle, coef int) error {
	n, err := pl.m.GetNumArgs(t)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		argument, err := pl.m.GetArg(t, i)
		if err != nil {
			return err
		}
		vars := make(map[uint32]ast.ASTHandle)
		if err = pl.p.variables(argument, vars); err != nil {
			return err
		}
		for v := range vars {
			counter[v] += coef
		}
	}
	return nil
}

func (counter varCounter) collectPositive() map[uint32]struct{} {
	result := make(map[uint32]struct{})
	for v, count := range counter {
		if count > 0 {
			result[v] = struct{}{}
		}
	}
	return result
}

func (pl *joinPlanner) collectVars(t ast.ASTHandle) (map[uint32]struct{}, error) {
	vars := make(map[uint32]ast.ASTHandle)
	if err := pl.p.variables(t, vars); err != nil {
		return nil, err
	}
	result := make(map[uint32]struct{}, len(vars))
	for v := range vars {
		result[v] = struct{}{}
	}
	return result, nil
}

// nonLocalVars is the counter dance shared by register_rule and
// replace_edges: the variables of t1 and t2 that still occur elsewhere.
func (pl *joinPlanner) nonLocalVars(counter varCounter, t1Vars map[uint32]struct{}, t2 ast.ASTHandle) (map[uint32]struct{}, error) {
	if err := pl.countVars(counter, t2, -1); err != nil {
		return nil, err
	}
	scopeVars, err := pl.collectVars(t2)
	if err != nil {
		return nil, err
	}
	for v := range t1Vars {
		scopeVars[v] = struct{}{}
	}
	nonLocal := counter.collectPositive()
	if err = pl.countVars(counter, t2, 1); err != nil {
		return nil, err
	}
	for v := range nonLocal {
		if _, found := scopeVars[v]; !found {
			delete(nonLocal, v)
		}
	}
	return nonLocal, nil
}

// registerRule is join_planner::register_rule.
func (pl *joinPlanner) registerRule(r int) error {
	rule := pl.sourceRules[r]
	counter := make(varCounter)
	// counter.count_rule_vars(r, 1)
	head, err := rule.Head()
	if err != nil {
		return err
	}
	if err = pl.countVars(counter, head, 1); err != nil {
		return err
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return err
	}
	for i := 0; i < tailSize; i++ {
		tail, _, err := rule.Tail(i)
		if err != nil {
			return err
		}
		if err = pl.countVars(counter, tail, 1); err != nil {
			return err
		}
	}
	posTailSize, err := rule.PositiveTailSize()
	if err != nil {
		return err
	}
	content := make([]ast.ASTHandle, 0, posTailSize)
	for i := 0; i < posTailSize; i++ {
		tail, _, err := rule.Tail(i)
		if err != nil {
			return err
		}
		content = append(content, tail)
	}
	pl.rulesContent[r] = content
	for i := 0; i+1 < posTailSize; i++ {
		t1 := content[i]
		t1Vars, err := pl.collectVars(t1)
		if err != nil {
			return err
		}
		if err = pl.countVars(counter, t1, -1); err != nil { //temporarily remove t1 variables from counter
			return err
		}
		for j := i + 1; j < posTailSize; j++ {
			t2 := content[j]
			nonLocal, err := pl.nonLocalVars(counter, t1Vars, t2)
			if err != nil {
				return err
			}
			if err = pl.registerPair(t1, t2, r, nonLocal); err != nil {
				return err
			}
		}
		if err = pl.countVars(counter, t1, 1); err != nil { //restore t1 variables in counter
			return err
		}
	}
	return nil
}

// extractArgumentInfo is extract_argument_info.
func (pl *joinPlanner) extractArgumentInfo(varIdx uint32, t ast.ASTHandle, args, domain *[]ast.ASTHandle) (bool, error) {
	n, err := pl.m.GetNumArgs(t)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		idx, v, err := pl.argVar(t, i)
		if err != nil {
			return false, err
		}
		if idx == varIdx {
			s, err := pl.m.GetSort(v)
			if err != nil {
				return false, err
			}
			*args = append(*args, v)
			*domain = append(*domain, s)
			return true, nil
		}
	}
	return false, nil
}

// joinPair is join_planner::join_pair.
func (pl *joinPlanner) joinPair(pairKey appPair) error {
	t1, t2 := pairKey.first, pairKey.second
	inf := pl.costInfo[pairKey]
	if inf == nil || len(inf.rules) == 0 {
		return fmt.Errorf("muz/transforms: selected join pair has no rules")
	}
	var args, domain []ast.ASTHandle
	for _, varIdx := range sortedVarSet(inf.allNonlocalVars) {
		found, err := pl.extractArgumentInfo(varIdx, t1, &args, &domain)
		if err != nil {
			return err
		}
		if !found {
			if found, err = pl.extractArgumentInfo(varIdx, t2, &args, &domain); err != nil {
				return err
			}
		}
		if !found {
			return fmt.Errorf("muz/transforms: join output variable %d is not in the pair", varIdx)
		}
	}
	oneParent := inf.rules[len(inf.rules)-1]
	parentHead, err := pl.sourceRules[oneParent].HeadDeclaration()
	if err != nil {
		return err
	}
	headName, err := pl.m.GetASTName(parentHead)
	if err != nil {
		return err
	}
	parentName, err := pl.m.Symbols.String(headName)
	if err != nil {
		return err
	}
	if len(inf.rules) > 1 {
		parentName = fmt.Sprintf("%s_and_%d", parentName, len(inf.rules)-1)
	}
	prefix, err := pl.m.Symbols.Intern(parentName)
	if err != nil {
		return err
	}
	suffix, err := pl.m.Symbols.Intern("split")
	if err != nil {
		return err
	}
	boolean, err := pl.m.MkBoolSort()
	if err != nil {
		return err
	}
	// m_context.mk_fresh_head_predicate(prefix, "split", arity, domain, parent_head)
	decl, err := pl.m.MkFreshFuncDecl(prefix, suffix, domain, boolean)
	if err != nil {
		return err
	}
	if err = pl.p.rules.RegisterPredicate(decl); err != nil {
		return err
	}
	pl.p.fresh = append(pl.p.fresh, decl)
	if pl.p.origins == nil {
		pl.p.origins = make(map[ast.ASTHandle]ast.ASTHandle)
	}
	pl.p.origins[decl] = parentHead
	head, err := pl.m.MkApp(decl, args)
	if err != nil {
		return err
	}
	if err = pl.p.pin(head); err != nil {
		return err
	}
	newRule, err := pl.p.rules.NewRule(head, []ast.ASTHandle{t1, t2}, nil, "")
	if err != nil {
		return err
	}
	//TODO: update accounting so that it can handle multiple parents
	if err = newRule.SetAccountingParentRule(pl.sourceRules[oneParent]); err != nil {
		return errors.Join(err, newRule.Close())
	}
	pl.introduced = append(pl.introduced, newRule)
	//here we copy the inf.m_rules vector because inf.m_rules will get changed
	//in the iteration. Also we use hashtable instead of vector because we do
	//not want to process one rule twice.
	processed := make(map[int]bool)
	rules := append([]int(nil), inf.rules...)
	for _, r := range rules {
		if !processed[r] {
			if err = pl.applyBinaryRule(r, pairKey, head); err != nil {
				return err
			}
			processed[r] = true
		}
	}
	return nil
}

func containsHandle(list []ast.ASTHandle, h ast.ASTHandle) bool {
	for _, x := range list {
		if x == h {
			return true
		}
	}
	return false
}

// replaceEdges is join_planner::replace_edges.
func (pl *joinPlanner) replaceEdges(r int, removedTails, addedTails0, ruleContent []ast.ASTHandle) error {
	length := len(ruleContent)
	originalLen := length + len(removedTails) - len(addedTails0)
	addedTails := append([]ast.ASTHandle(nil), addedTails0...) //we need a copy since we'll be modifying it
	rtSz := len(removedTails)
	//remove edges between removed tails
	for i := 0; i < rtSz; i++ {
		for j := i + 1; j < rtSz; j++ {
			pairKey, err := pl.getKey(removedTails[i], removedTails[j])
			if err != nil {
				return err
			}
			if err = pl.removeRuleFromPair(pairKey, r, originalLen); err != nil {
				return err
			}
		}
	}
	//remove edges between surviving tails and removed tails
	for i := 0; i < length; i++ {
		if containsHandle(addedTails, ruleContent[i]) {
			continue
		}
		for ri := 0; ri < rtSz; ri++ {
			pairKey, err := pl.getKey(ruleContent[i], removedTails[ri])
			if err != nil {
				return err
			}
			if err = pl.removeRuleFromPair(pairKey, r, originalLen); err != nil {
				return err
			}
		}
	}
	if length == 1 {
		return nil
	}
	rule := pl.sourceRules[r]
	head, err := rule.Head()
	if err != nil {
		return err
	}
	counter := make(varCounter)
	if err = pl.countVars(counter, head, 1); err != nil {
		return err
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return err
	}
	posTailSize, err := rule.PositiveTailSize()
	if err != nil {
		return err
	}
	for i := posTailSize; i < tailSize; i++ {
		tail, _, err := rule.Tail(i)
		if err != nil {
			return err
		}
		if err = pl.countVars(counter, tail, 1); err != nil {
			return err
		}
	}
	for i := 0; i < length; i++ {
		if err = pl.countVars(counter, ruleContent[i], 1); err != nil {
			return err
		}
	}
	//add edges that contain added tails
	for len(addedTails) != 0 {
		aTail := addedTails[len(addedTails)-1] //added tail
		aTailVars, err := pl.collectVars(aTail)
		if err != nil {
			return err
		}
		if err = pl.countVars(counter, aTail, -1); err != nil { //temporarily remove a_tail variables from counter
			return err
		}
		for i := 0; i < length; i++ {
			oTail := ruleContent[i] //other tail
			if containsHandle(addedTails, oTail) {
				//this avoids adding edges between new tails twice
				continue
			}
			nonLocal, err := pl.nonLocalVars(counter, aTailVars, oTail)
			if err != nil {
				return err
			}
			if err = pl.registerPair(oTail, aTail, r, nonLocal); err != nil {
				return err
			}
		}
		if err = pl.countVars(counter, aTail, 1); err != nil { //restore t1 variables in counter
			return err
		}
		addedTails = addedTails[:len(addedTails)-1]
	}
	return nil
}

// reverseRenaming is reverse_renaming (src/muz/base/dl_util.cpp).
func (pl *joinPlanner) reverseRenaming(src []ast.ASTHandle) ([]ast.ASTHandle, error) {
	srcSz := len(src)
	srcOfs := srcSz - 1
	maxVarIdx := uint32(0)
	for i := 0; i < srcSz; i++ {
		if src[i].IsNull() {
			continue
		}
		varIdx, err := pl.m.GetVarIndex(src[i])
		if err != nil {
			return nil, err
		}
		if varIdx > maxVarIdx {
			maxVarIdx = varIdx
		}
	}
	tgtSz := int(maxVarIdx) + 1
	tgtOfs := tgtSz - 1
	tgt := make([]ast.ASTHandle, tgtSz)
	for i := 0; i < srcSz; i++ {
		e := src[srcOfs-i]
		if e.IsNull() {
			continue
		}
		varIdx, err := pl.m.GetVarIndex(e)
		if err != nil {
			return nil, err
		}
		s, err := pl.m.GetSort(e)
		if err != nil {
			return nil, err
		}
		if tgt[tgtOfs-int(varIdx)], err = pl.m.MkVar(uint32(i), s); err != nil {
			return nil, err
		}
	}
	return tgt, nil
}

// applyBinaryRule is join_planner::apply_binary_rule.
func (pl *joinPlanner) applyBinaryRule(r int, pairKey appPair, tNew ast.ASTHandle) error {
	t1, t2 := pairKey.first, pairKey.second
	ruleContent := pl.rulesContent[r]
	length := len(ruleContent)
	if length == 1 {
		return nil
	}
	t1Pred, err := pl.m.GetDecl(t1)
	if err != nil {
		return err
	}
	t2Pred, err := pl.m.GetDecl(t2)
	if err != nil {
		return err
	}
	var removedTails, addedTails []ast.ASTHandle
	for i1 := 0; i1 < length; i1++ {
		rt1 := ruleContent[i1]
		rt1Pred, err := pl.m.GetDecl(rt1)
		if err != nil {
			return err
		}
		if rt1Pred != t1Pred {
			continue
		}
		i2start := 0
		if t1Pred == t2Pred {
			i2start = i1 + 1
		}
		for i2 := i2start; i2 < length; i2++ {
			rt2 := ruleContent[i2]
			if i1 == i2 {
				continue
			}
			rt2Pred, err := pl.m.GetDecl(rt2)
			if err != nil {
				return err
			}
			if rt2Pred != t2Pred {
				continue
			}
			key, err := pl.getKey(rt1, rt2)
			if err != nil {
				return err
			}
			if key != pairKey {
				continue
			}
			normalizer, err := pl.getNormalizer(rt1, rt2)
			if err != nil {
				return err
			}
			denormalizer, err := pl.reverseRenaming(normalizer)
			if err != nil {
				return err
			}
			newLit, err := pl.substitute(tNew, denormalizer)
			if err != nil {
				return err
			}
			ruleContent[i1] = newLit
			ruleContent[i2] = ruleContent[len(ruleContent)-1]
			ruleContent = ruleContent[:len(ruleContent)-1]
			length-- //here the bound of both loops changes!!!
			removedTails = append(removedTails, rt1, rt2)
			addedTails = append(addedTails, newLit)
			//this exits the inner loop, the outer one continues in case there will
			//be other matches
			break
		}
	}
	pl.rulesContent[r] = ruleContent
	if len(removedTails) == 0 || len(addedTails) == 0 {
		return fmt.Errorf("muz/transforms: join pair matched no tails of a registered rule")
	}
	pl.modified = true
	return pl.replaceEdges(r, removedTails, addedTails, ruleContent)
}

// getDomainSize is get_domain_size: context::get_sort_size_estimate.
func (pl *joinPlanner) getDomainSize(pred ast.ASTHandle, argIndex uint32) (float32, error) {
	s, err := pl.m.GetDomain(pred, argIndex)
	if err != nil {
		return 0, err
	}
	return pl.p.domainCost(s), nil
}

func (pl *joinPlanner) getStratum(pred ast.ASTHandle) (uint32, error) {
	return pl.auxCopy.PredicateStratum(pred)
}

func (pl *joinPlanner) getStratumOf(t ast.ASTHandle) (uint32, error) {
	pred, err := pl.m.GetDecl(t)
	if err != nil {
		return 0, err
	}
	return pl.getStratum(pred)
}

// computeCost is join_planner::compute_cost.
func (pl *joinPlanner) computeCost(t1, t2 ast.ASTHandle, nonLocalVars map[uint32]struct{}) (float32, error) {
	return pl.p.computeCost(t1, t2, nonLocalVars)
}

// pickBestPair is join_planner::pick_best_pair over m_costs in table order.
func (pl *joinPlanner) pickBestPair() (appPair, bool) {
	var best appPair
	found := false
	var bestCost float32
	for _, entry := range pl.costOrder.Entries() {
		key := appPair{first: entry.Key, second: entry.Value[0]}
		inf := pl.costInfo[key]
		if inf == nil || !inf.canBeJoined() {
			continue
		}
		c := inf.getCost()
		if !found || c < bestCost {
			found = true
			bestCost = c
			best = key
		}
	}
	return best, found
}

// run is join_planner::run.
func (pl *joinPlanner) run(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	numRules, err := source.Len()
	if err != nil {
		return nil, err
	}
	pl.rulesContent = make([][]ast.ASTHandle, numRules)
	pl.heads = make([]ast.ASTHandle, numRules)
	for i := 0; i < numRules; i++ {
		rule, err := source.Rule(i)
		if err != nil {
			return nil, err
		}
		pl.sourceRules = append(pl.sourceRules, rule)
		if pl.heads[i], err = rule.HeadDeclaration(); err != nil {
			return nil, err
		}
	}
	for i := 0; i < numRules; i++ {
		if err = pl.registerRule(i); err != nil {
			return nil, err
		}
	}
	for {
		selected, found := pl.pickBestPair()
		if !found {
			break
		}
		if err = pl.joinPair(selected); err != nil {
			return nil, err
		}
	}
	if !pl.modified {
		return nil, nil
	}
	result, err := base.NewRuleSet(pl.p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	// m_rules_content is iterated in source rule order.
	for r, origR := range pl.sourceRules {
		content := pl.rulesContent[r]
		posTailSize, err := origR.PositiveTailSize()
		if err != nil {
			return nil, err
		}
		if len(content) == posTailSize {
			//rule did not change
			if err = result.Add(origR); err != nil {
				return nil, err
			}
			continue
		}
		tail := append([]ast.ASTHandle(nil), content...)
		negs := make([]bool, len(tail))
		orLen, err := origR.TailSize()
		if err != nil {
			return nil, err
		}
		for i := posTailSize; i < orLen; i++ {
			t, neg, err := origR.Tail(i)
			if err != nil {
				return nil, err
			}
			tail = append(tail, t)
			negs = append(negs, neg)
		}
		head, err := origR.Head()
		if err != nil {
			return nil, err
		}
		name, err := origR.NameSymbol()
		if err != nil {
			return nil, err
		}
		newRule, x := pl.p.rules.NewRuleWithSymbol(head, tail, negs, name)
		if x == nil {
			x = newRule.SetAccountingParentRule(origR)
		}
		if x == nil {
			x = pl.p.rules.MkRuleRewriteProof(origR, newRule)
		}
		if x == nil {
			x = result.Add(newRule)
		}
		if newRule != nil {
			x = errors.Join(x, newRule.Close())
		}
		if x != nil {
			return nil, x
		}
	}
	for len(pl.introduced) != 0 {
		last := pl.introduced[len(pl.introduced)-1]
		x := result.Add(last)
		if x == nil && pl.p.proofTrace {
			x = pl.p.rules.MkRuleAssertedProof(last)
		}
		x = errors.Join(x, last.Close())
		pl.introduced = pl.introduced[:len(pl.introduced)-1]
		if x != nil {
			return nil, x
		}
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	keep = true
	return result, nil
}

// estimatedPairCost evaluates compute_cost for tails[leftIndex] and
// tails[rightIndex] of a rule whose remaining literals are outside and the
// other tails.
func (p *SimpleJoinsPlugin) estimatedPairCost(tails []ast.ASTHandle, outside []ast.ASTHandle, leftIndex, rightIndex int) (float32, error) {
	other := make([]ast.ASTHandle, 0, len(outside)+len(tails)-2)
	other = append(other, outside...)
	for index, tail := range tails {
		if index != leftIndex && index != rightIndex {
			other = append(other, tail)
		}
	}
	live, err := p.variableSorts(other)
	if err != nil {
		return 0, err
	}
	scope, err := p.variableSorts([]ast.ASTHandle{tails[leftIndex], tails[rightIndex]})
	if err != nil {
		return 0, err
	}
	nonLocal := make(map[uint32]struct{})
	for v := range live {
		if _, found := scope[v]; found {
			nonLocal[v] = struct{}{}
		}
	}
	return p.computeCost(tails[leftIndex], tails[rightIndex], nonLocal)
}

// usymbolUnused keeps the symbol import for the rule-name helpers.
var _ usymbol.Symbol
