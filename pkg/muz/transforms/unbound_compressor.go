package transforms

// Source: src/muz/transforms/dl_mk_unbound_compressor.{h,cpp}.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type compressionKey struct {
	predicate ast.ASTHandle
	argument  uint32
}

type UnboundCompressorPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	enabled  bool
	priority uint32
	mapping  map[compressionKey]ast.ASTHandle
	// order lists mapping keys in insertion order (deterministic).
	order  []compressionKey
	closed bool
	// nonEmpty is rel_context::collect_non_empty_predicates.
	nonEmpty func() error
	// headCount is m_head_occurrence_ctr (kept across calls, as the
	// source's).
	headCount map[ast.ASTHandle]int
	// borrowLite and fixUnbound are fix_unbound_vars' services; lite is
	// the borrowed qe_lite.
	borrowLite func() (*qe.Lite, error)
	lite       *qe.Lite
	fixUnbound bool
	// addFact is m_context.add_fact.
	addFact func(head ast.ASTHandle) error
}

func NewUnboundCompressorPlugin(manager *ast.ASTManager, rules *base.RuleManager, enabled bool, priority uint32) *UnboundCompressorPlugin {
	if priority == 0 {
		priority = 500
	}
	return &UnboundCompressorPlugin{manager: manager, rules: rules, enabled: enabled, priority: priority, mapping: make(map[compressionKey]ast.ASTHandle)}
}

// SetNonEmptyCollector installs rel_context::collect_non_empty_predicates,
// which operator() calls once compress_unbound is on.
func (p *UnboundCompressorPlugin) SetNonEmptyCollector(collect func() error) {
	p.nonEmpty = collect
}

func (p *UnboundCompressorPlugin) Priority() uint32            { return p.priority }
func (p *UnboundCompressorPlugin) CanDestratifyNegation() bool { return false }
func (p *UnboundCompressorPlugin) Cancel()                     {}

func (p *UnboundCompressorPlugin) CompressedPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle)
	for key, declaration := range p.mapping {
		result[key.predicate] = declaration
	}
	return result
}

func (p *UnboundCompressorPlugin) headTask(source *base.RuleSet, rule *base.Rule) (compressionKey, bool, error) {
	declaration, err := rule.HeadDeclaration()
	if err != nil {
		return compressionKey{}, false, err
	}
	output, err := source.IsOutputPredicate(declaration)
	if err != nil || output {
		return compressionKey{}, false, err
	}
	head, err := rule.Head()
	if err != nil {
		return compressionKey{}, false, err
	}
	tailVariables, err := p.rules.TailVariables(rule)
	if err != nil {
		return compressionKey{}, false, err
	}
	count, err := p.manager.GetNumArgs(head)
	if err != nil {
		return compressionKey{}, false, err
	}
	occurrences := make(map[uint32]uint32)
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(head, index)
		if getErr != nil {
			return compressionKey{}, false, getErr
		}
		if kind, x := p.manager.GetKind(argument); x == nil && kind == ast.ASTVar {
			variable, x := p.manager.GetVarIndex(argument)
			if x != nil {
				return compressionKey{}, false, x
			}
			occurrences[variable]++
		}
	}
	for index := uint32(0); index < count; index++ {
		argument, getErr := p.manager.GetArg(head, index)
		if getErr != nil {
			return compressionKey{}, false, getErr
		}
		kind, getErr := p.manager.GetKind(argument)
		if getErr != nil || kind != ast.ASTVar {
			continue
		}
		variable, getErr := p.manager.GetVarIndex(argument)
		if getErr != nil {
			return compressionKey{}, false, getErr
		}
		if tailVariables[variable].IsNull() && occurrences[variable] == 1 {
			return compressionKey{predicate: declaration, argument: index}, true, nil
		}
	}
	return compressionKey{}, false, nil
}

func (p *UnboundCompressorPlugin) declare(key compressionKey) (ast.ASTHandle, error) {
	if declaration := p.mapping[key]; !declaration.IsNull() {
		return declaration, nil
	}
	arity, err := p.manager.GetArity(key.predicate)
	if err != nil || arity == 0 || key.argument >= arity {
		return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
	}
	domain := make([]ast.ASTHandle, 0, arity-1)
	for index := uint32(0); index < arity; index++ {
		if index == key.argument {
			continue
		}
		sort, getErr := p.manager.GetDomain(key.predicate, index)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		domain = append(domain, sort)
	}
	name, err := p.manager.GetASTName(key.predicate)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	suffixText := fmt.Sprintf("compr_arg_%d", key.argument)
	suffix, err := p.manager.Symbols.Intern(suffixText)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := p.manager.MkFreshFuncDecl(name, suffix, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.rules.RegisterPredicate(declaration); err != nil {
		return ast.ASTHandle{}, err
	}
	p.mapping[key] = declaration
	p.order = append(p.order, key)
	return declaration, nil
}

func (p *UnboundCompressorPlugin) compressedApplication(application ast.ASTHandle, key compressionKey) (ast.ASTHandle, error) {
	declaration := p.mapping[key]
	if declaration.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	count, err := p.manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0, count-1)
	for index := uint32(0); index < count; index++ {
		if index == key.argument {
			continue
		}
		argument, getErr := p.manager.GetArg(application, index)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		arguments = append(arguments, argument)
	}
	return p.manager.MkApp(declaration, arguments)
}

// unboundRun is one operator() call's state: m_rules, m_todo,
// m_in_progress and m_pinned (m_map is the plugin's mapping, m_modified
// modified).
type unboundRun struct {
	p          *UnboundCompressorPlugin
	source     *base.RuleSet
	rules      []*base.Rule
	todo       []compressionKey
	inProgress map[compressionKey]bool
	pinned     []ast.ASTRef
	modified   bool
}

func (u *unboundRun) pin(h ast.ASTHandle) error {
	ref, err := u.p.manager.NewRef(h)
	if err == nil {
		u.pinned = append(u.pinned, ref)
	}
	return err
}

// addTask is add_task: the compressed predicate, pinned with its parent.
func (u *unboundRun) addTask(key compressionKey) error {
	if !u.p.mapping[key].IsNull() {
		return nil //this task was already added
	}
	declaration, err := u.p.declare(key)
	if err != nil {
		return err
	}
	if err = u.pin(declaration); err != nil {
		return err
	}
	if err = u.pin(key.predicate); err != nil {
		return err
	}
	u.todo = append(u.todo, key)
	return nil
}

// detectTasks is detect_tasks: the first head argument that is a variable
// occurring once in the head and not in the tail.
func (u *unboundRun) detectTasks(ruleIndex int) error {
	key, found, err := u.p.headTask(u.source, u.rules[ruleIndex])
	if err != nil || !found {
		return err
	}
	return u.addTask(key)
}

// tryCompress is try_compress: l_undef (lbool.Undef) if nothing to
// compress, l_true if compressed, l_false if the rule became a fact and was
// removed.
func (u *unboundRun) tryCompress(ruleIndex int) (lbool.LBool, error) {
	p := u.p
	m := p.manager
	r := u.rules[ruleIndex]
	tailVariables, err := p.rules.TailVariables(r)
	if err != nil {
		return lbool.Undef, err
	}
	head, err := r.Head()
	if err != nil {
		return lbool.Undef, err
	}
	headDeclaration, err := m.GetDecl(head)
	if err != nil {
		return lbool.Undef, err
	}
	arity, err := m.GetNumArgs(head)
	if err != nil {
		return lbool.Undef, err
	}
	occurrences := make(map[uint32]uint32)
	variables := make([]int64, arity)
	for i := uint32(0); i < arity; i++ {
		variables[i] = -1
		argument, err := m.GetArg(head, i)
		if err != nil {
			return lbool.Undef, err
		}
		if kind, err := m.GetKind(argument); err != nil {
			return lbool.Undef, err
		} else if kind == ast.ASTVar {
			index, err := m.GetVarIndex(argument)
			if err != nil {
				return lbool.Undef, err
			}
			occurrences[index]++
			variables[i] = int64(index)
		}
	}
	argIndex := arity
	for i := uint32(0); i < arity; i++ {
		if variables[i] < 0 {
			continue
		}
		v := uint32(variables[i])
		if tailVariables[v].IsNull() && occurrences[v] == 1 && u.inProgress[compressionKey{predicate: headDeclaration, argument: i}] {
			argIndex = i
			break
		}
	}
	if argIndex == arity {
		//we didn't find anything to compress
		return lbool.Undef, nil
	}
	key := compressionKey{predicate: headDeclaration, argument: argIndex}
	chead, err := p.compressedApplication(head, key)
	if err != nil {
		return lbool.Undef, err
	}
	// app_ref chead
	cheadRef, err := m.NewRef(chead)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { _ = m.DelRef(&cheadRef) }()
	u.modified = true
	tails, err := r.TailSize()
	if err != nil {
		return lbool.Undef, err
	}
	fact := tails == 0
	if fact {
		count, err := m.GetNumArgs(chead)
		if err != nil {
			return lbool.Undef, err
		}
		for i := uint32(0); i < count && fact; i++ {
			argument, err := m.GetArg(chead, i)
			if err != nil {
				return lbool.Undef, err
			}
			if fact, err = m.IsValue(argument); err != nil {
				return lbool.Undef, err
			}
		}
	}
	if fact {
		// m_non_empty_rels.insert(cpred); m_context.add_fact(chead)
		if p.addFact == nil {
			return lbool.Undef, fmt.Errorf("muz/transforms: unbound compressor fact without a context")
		}
		if err = p.addFact(chead); err != nil {
			return lbool.Undef, err
		}
		p.headCount[headDeclaration]--
		//remove the rule that became fact by placing the last rule on its place
		last := len(u.rules) - 1
		removed := u.rules[ruleIndex]
		if ruleIndex < last {
			u.rules[ruleIndex] = u.rules[last]
		}
		u.rules = u.rules[:last]
		return lbool.False, removed.Close()
	}
	name, err := r.NameSymbol()
	if err != nil {
		return lbool.Undef, err
	}
	replacement, err := p.rules.ReplaceRuleHeadWithSymbol(r, chead, name)
	if err != nil {
		return lbool.Undef, err
	}
	if err = replacement.SetAccountingParentRule(r); err != nil {
		return lbool.Undef, errors.Join(err, replacement.Close())
	}
	p.headCount[headDeclaration]--
	u.rules[ruleIndex] = replacement
	p.headCount[key2decl(p, key)]++
	if err = r.Close(); err != nil {
		return lbool.Undef, err
	}
	return lbool.True, u.detectTasks(ruleIndex)
}

func key2decl(p *UnboundCompressorPlugin, key compressionKey) ast.ASTHandle { return p.mapping[key] }

// mkDecompressionRule is mk_decompression_rule.
func (u *unboundRun) mkDecompressionRule(r *base.Rule, tailIndex int, argIndex uint32) (_ *base.Rule, resultErr error) {
	p := u.p
	m := p.manager
	origDtail, _, err := r.Tail(tailIndex)
	if err != nil {
		return nil, err
	}
	declaration, err := m.GetDecl(origDtail)
	if err != nil {
		return nil, err
	}
	dtail, err := p.compressedApplication(origDtail, compressionKey{predicate: declaration, argument: argIndex})
	if err != nil {
		return nil, err
	}
	// app_ref dtail
	dtailRef, err := m.NewRef(dtail)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&dtailRef)) }()
	total, err := r.TailSize()
	if err != nil {
		return nil, err
	}
	tails := make([]ast.ASTHandle, 0, total+1)
	negated := make([]bool, 0, total+1)
	tailNegated := false
	for i := 0; i < total; i++ {
		tail, neg, err := r.Tail(i)
		if err != nil {
			return nil, err
		}
		negated = append(negated, neg)
		if i == tailIndex {
			tailNegated = neg
		}
		if i == tailIndex && !neg {
			tails = append(tails, dtail)
		} else {
			tails = append(tails, tail)
		}
	}
	// Accumulate negated filtered rule instead
	// of replacing the original predicate.
	if tailNegated {
		negated = append(negated, true)
		tails = append(tails, dtail)
	}
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	res, err := p.rules.NewRuleWithSymbol(head, tails, negated, usymbol.Symbol{})
	if err != nil {
		return nil, err
	}
	if err = res.SetAccountingParentRule(r); err != nil {
		return nil, errors.Join(err, res.Close())
	}
	// m_context.get_rule_manager().fix_unbound_vars(res, true)
	lite, err := p.fixLite()
	if err != nil {
		return nil, errors.Join(err, res.Close())
	}
	fixed, _, err := p.rules.FixUnboundVariables(res, lite, p.fixUnbound)
	if err != nil {
		return nil, errors.Join(err, res.Close())
	}
	if fixed != res {
		if err = res.Close(); err != nil {
			return nil, errors.Join(err, fixed.Close())
		}
	}
	return fixed, nil
}

// addDecompressionRule is add_decompression_rule.
func (u *unboundRun) addDecompressionRule(r *base.Rule, tailIndex int, argIndex uint32) error {
	replacement, err := u.mkDecompressionRule(r, tailIndex, argIndex)
	if err != nil {
		return err
	}
	index := len(u.rules)
	u.rules = append(u.rules, replacement)
	if err = u.p.rules.MkRuleRewriteProof(r, replacement); err != nil {
		return err
	}
	declaration, err := replacement.HeadDeclaration()
	if err != nil {
		return err
	}
	u.p.headCount[declaration]++
	if err = u.detectTasks(index); err != nil {
		return err
	}
	u.modified = true
	return nil
}

// replaceByDecompressionRule is replace_by_decompression_rule; the
// caller's reference keeps the replaced rule alive.
func (u *unboundRun) replaceByDecompressionRule(ruleIndex, tailIndex int, argIndex uint32) error {
	r := u.rules[ruleIndex]
	replacement, err := u.mkDecompressionRule(r, tailIndex, argIndex)
	if err != nil {
		return err
	}
	u.rules[ruleIndex] = replacement
	// we don't update the head occurrence counter because the head
	// predicate doesn't change
	if err = r.Close(); err != nil {
		return err
	}
	if err = u.detectTasks(ruleIndex); err != nil {
		return err
	}
	u.modified = true
	return nil
}

// inProgressIndices is add_in_progress_indices.
func (u *unboundRun) inProgressIndices(application ast.ASTHandle) ([]uint32, error) {
	m := u.p.manager
	declaration, err := m.GetDecl(application)
	if err != nil {
		return nil, err
	}
	count, err := m.GetNumArgs(application)
	if err != nil {
		return nil, err
	}
	var indices []uint32
	for i := uint32(0); i < count; i++ {
		if u.inProgress[compressionKey{predicate: declaration, argument: i}] {
			indices = append(indices, i)
		}
	}
	return indices, nil
}

// decompressRule is decompress_rule. can_remove_orig_rule conjoins
// arg_indices.empty(), false inside the loop over arg_indices.
func (u *unboundRun) decompressRule(r *base.Rule, argIndices []uint32, ruleIndex, tailIndex int) (bool, error) {
	_, negated, err := r.Tail(tailIndex)
	if err != nil {
		return false, err
	}
	replace := false
	for _, argIndex := range argIndices {
		if negated {
			replace = true
			if err = u.replaceByDecompressionRule(ruleIndex, tailIndex, argIndex); err != nil {
				return false, err
			}
			// arg_indices becomes stale after the original rule is replaced.
			break
		}
		if err = u.addDecompressionRule(r, tailIndex, argIndex); err != nil {
			return false, err
		}
	}
	return replace, nil
}

// addDecompressionRules is add_decompression_rules.
func (u *unboundRun) addDecompressionRules(ruleIndex int) (resultErr error) {
	// rule_ref r: updated when the rule is replaced.
	r, err := u.rules[ruleIndex].Retain()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.Close()) }()
	utailLen, err := r.UninterpretedTailSize()
	if err != nil {
		return err
	}
	for tailIndex := 0; tailIndex < utailLen; {
		tail, _, err := r.Tail(tailIndex)
		if err != nil {
			return err
		}
		indices, err := u.inProgressIndices(tail)
		if err != nil {
			return err
		}
		replace, err := u.decompressRule(r, indices, ruleIndex, tailIndex)
		if err != nil {
			return err
		}
		if replace {
			// update r with the new rule and examine its tail_index-th tail
			next, err := u.rules[ruleIndex].Retain()
			if err != nil {
				return err
			}
			if err = r.Close(); err != nil {
				return errors.Join(err, next.Close())
			}
			r = next
		} else {
			tailIndex++
		}
	}
	return nil
}

// release is reset(): m_rules front to back, then m_pinned.
func (u *unboundRun) release() (result error) {
	for i := range u.rules {
		result = errors.Join(result, u.rules[i].Close())
	}
	u.rules = nil
	for i := range u.pinned {
		result = errors.Join(result, u.p.manager.DelRef(&u.pinned[i]))
	}
	u.pinned = nil
	return result
}

// Transform is mk_unbound_compressor::operator().
func (p *UnboundCompressorPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	if !p.enabled {
		return nil, nil
	}
	if p.nonEmpty != nil {
		if err := p.nonEmpty(); err != nil {
			return nil, err
		}
	}
	p.mapping = make(map[compressionKey]ast.ASTHandle)
	p.order = nil
	if p.headCount == nil {
		p.headCount = make(map[ast.ASTHandle]int)
	}
	u := &unboundRun{p: p, source: source}
	defer func() { resultErr = errors.Join(resultErr, u.release()) }()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, err := source.Rule(i)
		if err != nil {
			return nil, err
		}
		u.rules = append(u.rules, rule)
		declaration, err := rule.HeadDeclaration()
		if err != nil {
			return nil, err
		}
		p.headCount[declaration]++
	}
	for i := 0; i < count; i++ {
		if err = u.detectTasks(i); err != nil {
			return nil, err
		}
	}
	for len(u.todo) != 0 {
		u.inProgress = make(map[compressionKey]bool)
		for len(u.todo) != 0 {
			last := len(u.todo) - 1
			u.inProgress[u.todo[last]] = true
			u.todo = u.todo[:last]
		}
		for ruleIndex := 0; ruleIndex < len(u.rules); {
			status, err := u.tryCompress(ruleIndex)
			if err != nil {
				return nil, err
			}
			if status == lbool.False {
				continue
			}
			//m_rules can grow here
			if err = u.addDecompressionRules(ruleIndex); err != nil {
				return nil, err
			}
			ruleIndex++
		}
	}
	if !u.modified {
		return nil, nil
	}
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	for _, rule := range u.rules {
		if err = result.Add(rule); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	// mk_fresh_head_predicate(..., pred): the compressed predicates' origins.
	for _, key := range p.order {
		declaration, held := p.mapping[key]
		if !held {
			continue
		}
		if err = result.InheritPredicate(source, key.predicate, declaration); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}

// fixLite is the rule manager's qe_lite for fix_unbound_vars.
func (p *UnboundCompressorPlugin) fixLite() (*qe.Lite, error) {
	if p.lite == nil && p.borrowLite != nil {
		lite, err := p.borrowLite()
		if err != nil {
			return nil, err
		}
		p.lite = lite
	}
	return p.lite, nil
}

// SetFixUnboundServices supplies rule_manager::fix_unbound_vars: the rule
// manager's qe_lite and xform.fix_unbound_vars.
func (p *UnboundCompressorPlugin) SetFixUnboundServices(lite func() (*qe.Lite, error), fix bool) {
	p.borrowLite, p.fixUnbound = lite, fix
}

// SetFactAdder supplies m_context.add_fact(head) for a compressed head
// that became a fact.
func (p *UnboundCompressorPlugin) SetFactAdder(add func(head ast.ASTHandle) error) {
	p.addFact = add
}

func (p *UnboundCompressorPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed, p.mapping = true, nil
	return nil
}

// SourceTypeName is typeid(datalog::mk_unbound_compressor).name() for the IF_VERBOSE
// transform lines of rule_transformer::operator().
func (p *UnboundCompressorPlugin) SourceTypeName() string {
	return base.DatalogPluginTypeName("mk_unbound_compressor")
}
