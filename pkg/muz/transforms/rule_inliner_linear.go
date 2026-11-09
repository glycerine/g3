package transforms

// Source: src/muz/transforms/dl_mk_rule_inliner.cpp mk_rule_inliner::
// inline_linear, add_rule, del_rule and the visitor over m_head_index and
// m_tail_index (straight-line relation inlining).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/muz/base"
)

// inlinerVisitor is mk_rule_inliner::visitor: it collects the rule
// positions of every indexed expression the tree unifies with, stopping once
// more than one is known.
type inlinerVisitor struct {
	subst     *substitution.Substitution
	unifiers  []int
	canRemove []bool
	canExpand []bool
	// positions is m_positions: lookups only, keyed by indexed expressions
	// that the pinned rules keep alive.
	positions map[ast.ASTHandle][]int
}

func (v *inlinerVisitor) TreeSubstitution() *substitution.Substitution { return v.subst }

func (v *inlinerVisitor) VisitTreeExpression(e ast.ASTHandle) (bool, error) {
	v.unifiers = append(v.unifiers, v.positions[e]...)
	return len(v.unifiers) <= 1, nil
}

func (v *inlinerVisitor) resetSize(size int) {
	v.unifiers = v.unifiers[:0]
	v.canRemove = make([]bool, size)
	v.canExpand = make([]bool, size)
	for i := 0; i < size; i++ {
		v.canRemove[i], v.canExpand[i] = true, true
	}
	v.positions = make(map[ast.ASTHandle][]int)
}

func (v *inlinerVisitor) addPosition(e ast.ASTHandle, j int) {
	v.positions[e] = append(v.positions[e], j)
}

func (v *inlinerVisitor) delPosition(e ast.ASTHandle, j int) {
	list := v.positions[e]
	for k, value := range list {
		if value == j {
			v.positions[e] = append(list[:k], list[k+1:]...)
			return
		}
	}
}

// linearIndex holds the inliner members inline_linear uses: m_head_index,
// m_tail_index, m_subst, both visitors, and m_pinned's rules.
type linearIndex struct {
	head, tail  *substitution.SubstitutionTree
	subst       *substitution.Substitution
	headVisitor inlinerVisitor
	tailVisitor inlinerVisitor
	pinned      []*base.Rule
}

func (p *RuleInlinerPlugin) linearMembers() (*linearIndex, error) {
	if p.linearState != nil {
		return p.linearState, nil
	}
	state := &linearIndex{}
	var err error
	if state.head, err = substitution.NewSubstitutionTree(p.manager); err != nil {
		return nil, err
	}
	if state.tail, err = substitution.NewSubstitutionTree(p.manager); err != nil {
		return nil, errors.Join(err, state.head.Close())
	}
	if state.subst, err = substitution.New(p.manager); err != nil {
		return nil, errors.Join(err, state.tail.Close(), state.head.Close())
	}
	state.headVisitor.subst = state.subst
	state.tailVisitor.subst = state.subst
	p.linearState = state
	return state, nil
}

// closeIndexes destroys the visitors, m_subst, m_tail_index and
// m_head_index (reverse declaration order).
func (s *linearIndex) closeIndexes() error {
	err := s.subst.Close()
	return errors.Join(err, s.tail.Close(), s.head.Close())
}

// closePinned releases m_pinned (declared first, destroyed last) forward.
func (s *linearIndex) closePinned() (err error) {
	for _, rule := range s.pinned {
		err = errors.Join(err, rule.Close())
	}
	s.pinned = nil
	return err
}

// ruleMaxVar is rule_counter::get_max_var: the largest free variable index,
// or 0 without variables.
func ruleMaxVar(m *ast.ASTManager, expressions ...ast.ASTHandle) (uint32, error) {
	used, err := ast.NewASTUsedVars(m)
	if err != nil {
		return 0, err
	}
	for _, e := range expressions {
		if err = used.Process(e); err != nil {
			return 0, err
		}
	}
	if count := used.MaxFoundVarIndexPlusOne(); count != 0 {
		return count - 1, nil
	}
	return 0, nil
}

// addLinearRule is mk_rule_inliner::add_rule.
func (p *RuleInlinerPlugin) addLinearRule(s *linearIndex, rule *base.Rule, i int) error {
	head, err := rule.Head()
	if err != nil {
		return err
	}
	headDeclaration, err := rule.HeadDeclaration()
	if err != nil {
		return err
	}
	s.headVisitor.addPosition(head, i)
	if err = s.head.Insert(head); err != nil {
		return err
	}
	pinned, err := rule.Retain()
	if err != nil {
		return err
	}
	s.pinned = append(s.pinned, pinned)
	// Output predicates and predicates with facts (p.protected) stay.
	if p.protected[headDeclaration] {
		s.headVisitor.canRemove[i] = false
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	for j := 0; j < uninterpreted; j++ {
		tail, _, err := rule.Tail(j)
		if err != nil {
			return err
		}
		s.tailVisitor.addPosition(tail, i)
		if err = s.tail.Insert(tail); err != nil {
			return err
		}
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return err
	}
	canExpand := uninterpreted == 1 && positive == 1
	if canExpand {
		tailDeclaration, err := rule.TailDeclaration(0)
		if err != nil {
			return err
		}
		canExpand = !p.protected[tailDeclaration]
	}
	s.headVisitor.canExpand[i] = canExpand
	return nil
}

// delLinearRule is mk_rule_inliner::del_rule: only the positions go; the
// expressions stay in the indexes.
func delLinearRule(s *linearIndex, rule *base.Rule, i int) error {
	head, err := rule.Head()
	if err != nil {
		return err
	}
	s.headVisitor.delPosition(head, i)
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	for j := 0; j < uninterpreted; j++ {
		tail, _, err := rule.Tail(j)
		if err != nil {
			return err
		}
		s.tailVisitor.delPosition(tail, i)
	}
	return nil
}

func ruleMaxRuleVar(m *ast.ASTManager, rule *base.Rule) (uint32, error) {
	head, err := rule.Head()
	if err != nil {
		return 0, err
	}
	size, err := rule.TailSize()
	if err != nil {
		return 0, err
	}
	expressions := []ast.ASTHandle{head}
	for j := 0; j < size; j++ {
		tail, _, err := rule.Tail(j)
		if err != nil {
			return 0, err
		}
		expressions = append(expressions, tail)
	}
	return ruleMaxVar(m, expressions...)
}

// inlineLinear is mk_rule_inliner::inline_linear: inline predicates that
// are not join points, found through the head and tail unification indexes.
func (p *RuleInlinerPlugin) inlineLinear(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	size, err := source.Len()
	if err != nil {
		return nil, err
	}
	s, err := p.linearMembers()
	if err != nil {
		return nil, err
	}
	s.headVisitor.resetSize(size)
	s.tailVisitor.resetSize(size)
	if err = s.head.Reset(); err != nil {
		return nil, err
	}
	if err = s.tail.Reset(); err != nil {
		return nil, err
	}
	// acc is a rule_ref_vector local: released forward on return.
	acc := make([]*base.Rule, size)
	defer func() {
		for _, rule := range acc {
			if rule != nil {
				resultErr = errors.Join(resultErr, rule.Close())
			}
		}
	}()
	for i := 0; i < size; i++ {
		if acc[i], err = source.Rule(i); err != nil {
			return nil, err
		}
	}
	for i := 0; i < size; i++ {
		if err = p.addLinearRule(s, acc[i], i); err != nil {
			return nil, err
		}
	}
	var maxVar uint32
	for i := 0; i < size; i++ {
		head, err := acc[i].Head()
		if err != nil {
			return nil, err
		}
		expressions := []ast.ASTHandle{head}
		uninterpreted, err := acc[i].UninterpretedTailSize()
		if err != nil {
			return nil, err
		}
		for j := 0; j < uninterpreted; j++ {
			tail, _, err := acc[i].Tail(j)
			if err != nil {
				return nil, err
			}
			expressions = append(expressions, tail)
		}
		for _, e := range expressions {
			v, err := ruleMaxVar(p.manager, e)
			if err != nil {
				return nil, err
			}
			if v > maxVar {
				maxVar = v
			}
		}
	}
	if err = s.subst.Reset(); err != nil {
		return nil, err
	}
	if err = s.subst.ReserveVariables(maxVar + 1); err != nil {
		return nil, err
	}
	tailRegisters, err := s.tail.ApproxRegisters()
	if err != nil {
		return nil, err
	}
	headRegisters, err := s.head.ApproxRegisters()
	if err != nil {
		return nil, err
	}
	offsets := tailRegisters
	if 2+headRegisters > offsets {
		offsets = 2 + headRegisters
	}
	if err = s.subst.ReserveOffsets(offsets); err != nil {
		return nil, err
	}
	valid := make([]bool, size)
	for i := range valid {
		valid[i] = true
	}
	canRemove, canExpand := s.headVisitor.canRemove, s.headVisitor.canExpand
	doneSomething := false
	for i := 0; i < size; i++ {
		for valid[i] && canExpand[i] {
			r := acc[i]
			tail, _, err := r.Tail(0)
			if err != nil {
				return nil, err
			}
			s.headVisitor.unifiers = s.headVisitor.unifiers[:0]
			if err = s.head.Unify(tail, &s.headVisitor); err != nil {
				return nil, err
			}
			if len(s.headVisitor.unifiers) != 1 {
				break
			}
			j := s.headVisitor.unifiers[0]
			if !canRemove[j] || !valid[j] || i == j {
				break
			}
			r2 := acc[j]
			// The head of r2 must unify with this single body position.
			head2, err := r2.Head()
			if err != nil {
				return nil, err
			}
			s.tailVisitor.unifiers = s.tailVisitor.unifiers[:0]
			if err = s.tail.Unify(head2, &s.tailVisitor); err != nil {
				return nil, err
			}
			tailUnifiers := len(s.tailVisitor.unifiers)
			if !p.branching && tailUnifiers != 1 {
				break
			}
			resolved, err := p.resolve(r, 0, r2)
			if err != nil {
				return nil, err
			}
			if resolved == nil {
				break
			}
			doneSomething = true
			if err = delLinearRule(s, r, i); err != nil {
				return nil, errors.Join(err, resolved.Close())
			}
			if err = p.addLinearRule(s, resolved, i); err != nil {
				return nil, errors.Join(err, resolved.Close())
			}
			if err = acc[i].Close(); err != nil {
				return nil, errors.Join(err, resolved.Close())
			}
			acc[i] = resolved
			canExpand[i] = canExpand[j]
			if tailUnifiers == 1 {
				valid[j] = false
				// datalog::del_rule(m_mc, *r2, true).
				if err = p.captureDeletedModelRule(r2, true); err != nil {
					return nil, err
				}
				if err = delLinearRule(s, r2, j); err != nil {
					return nil, err
				}
			}
			v, err := ruleMaxRuleVar(p.manager, resolved)
			if err != nil {
				return nil, err
			}
			if v > maxVar {
				maxVar = v
			}
			if err = s.subst.ReserveVariables(maxVar + 1); err != nil {
				return nil, err
			}
		}
	}
	if !doneSomething {
		return nil, nil
	}
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	for i := 0; i < size; i++ {
		if valid[i] {
			if err = result.Add(acc[i]); err != nil {
				return nil, err
			}
		}
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	keep = true
	return result, nil
}
