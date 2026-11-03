package rel

// Source: dl_mk_similarity_compressor.cpp merge_class/operator, using the
// registered interval_relation plugin for arithmetic auxiliary facts.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// IntervalSimilarityProgram owns query-local rules and arithmetic auxiliary
// relations. Its private rule manager keeps generated predicates out of the
// original program's registration state.
type IntervalSimilarityProgram struct {
	manager     *ast.ASTManager
	rules       *base.RuleManager
	set         *base.RuleSet
	sourceRules []*base.Rule
	pending     []*base.Rule
	refs        []ast.ASTRef
	auxiliary   []ast.ASTHandle
	seeds       map[ast.ASTHandle]*IntervalRelation
	origins     map[ast.ASTHandle]ast.ASTHandle
	modified    bool
	closed      bool
}

func NewIntervalSimilarityProgram(manager *ast.ASTManager, source *base.RuleSet, threshold uint32, outputs []ast.ASTHandle) (_ *IntervalSimilarityProgram, resultErr error) {
	if manager == nil || source == nil || threshold <= 1 {
		return nil, ast.ErrASTDomain
	}
	p := &IntervalSimilarityProgram{manager: manager, seeds: make(map[ast.ASTHandle]*IntervalRelation), origins: make(map[ast.ASTHandle]ast.ASTHandle)}
	defer func() {
		if resultErr != nil {
			resultErr = finishIntervalSimilarityConstruction(resultErr, p)
		}
	}()
	var err error
	p.rules, err = base.NewRuleManager(manager)
	if err != nil {
		return nil, err
	}
	p.set, err = base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		rule, err := source.Rule(index)
		if err != nil {
			return nil, err
		}
		p.sourceRules = append(p.sourceRules, rule)
		owner, err := rule.ASTManager()
		if err != nil || owner != manager {
			return nil, errors.Join(err, ast.ErrASTDomain)
		}
		head, err := rule.HeadDeclaration()
		if err != nil {
			return nil, err
		}
		if err = p.rules.RegisterPredicate(head); err != nil {
			return nil, err
		}
		tails, err := rule.UninterpretedTailSize()
		if err != nil {
			return nil, err
		}
		for column := 0; column < tails; column++ {
			declaration, err := rule.TailDeclaration(column)
			if err != nil {
				return nil, err
			}
			if err = p.rules.RegisterPredicate(declaration); err != nil {
				return nil, err
			}
		}
	}
	protected := make(map[ast.ASTHandle]bool, len(outputs))
	for _, output := range outputs {
		protected[output] = true
		if err = p.rules.RegisterPredicate(output); err != nil {
			return nil, err
		}
		if err = p.set.SetOutputPredicate(output); err != nil {
			return nil, err
		}
	}
	originalOutputs, err := source.OutputPredicates()
	if err != nil {
		return nil, err
	}
	for _, output := range originalOutputs {
		if err = p.rules.RegisterPredicate(output); err != nil {
			return nil, err
		}
		if err = p.set.SetOutputPredicate(output); err != nil {
			return nil, err
		}
	}
	classes, modified, err := similarityClassesExcept(source, p.sourceRules, threshold, protected)
	if err != nil {
		return nil, err
	}
	p.modified = modified
	for _, class := range classes {
		p.modified = p.modified || class.merge
	}
	if !p.modified {
		// The source returns nullptr when no transformation occurred. Its
		// comparator order must not affect subsequent interval widening.
		for _, rule := range p.sourceRules {
			if err = p.set.Add(rule); err != nil {
				return nil, err
			}
		}
		return p, nil
	}
	for _, class := range classes {
		if !class.merge {
			for _, rule := range class.rules {
				if err = p.set.Add(rule); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err = p.merge(class); err != nil {
			return nil, err
		}
		p.modified = true
	}
	return p, nil
}

func (p *IntervalSimilarityProgram) merge(class similarityClass) error {
	domains, rows, err := similarityOrphanRows(class.rules, class.constants)
	if err != nil {
		return err
	}
	if !CanHandleIntervalSorts(p.manager, domains) {
		return ast.ErrASTDomain
	}
	seed := NewIntervalEmpty(uint32(len(domains)))
	for _, row := range rows {
		if err = seed.AddFactExpressions(p.manager, domains, row); err != nil {
			return err
		}
	}
	head, err := class.rules[0].HeadDeclaration()
	if err != nil {
		return err
	}
	prefix, err := p.manager.GetASTName(head)
	if err != nil {
		return err
	}
	suffix, err := p.manager.Symbols.Intern(fmt.Sprintf("sc_%d", len(class.constants)))
	if err != nil {
		return err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return err
	}
	auxiliary, err := p.manager.MkFreshFuncDecl(prefix, suffix, domains, boolean)
	if err != nil {
		return err
	}
	ref, err := p.manager.NewRef(auxiliary)
	if err != nil {
		return err
	}
	p.refs = append(p.refs, ref)
	if err = p.rules.RegisterPredicate(auxiliary); err != nil {
		return err
	}
	p.auxiliary = append(p.auxiliary, auxiliary)
	p.seeds[auxiliary], p.origins[auxiliary] = seed, head
	merged, err := similarityRewriteRule(p.rules, class.rules[0], class.constants, auxiliary)
	if err != nil {
		return err
	}
	p.pending = append(p.pending, merged)
	if err = p.set.Add(merged); err != nil {
		return err
	}
	if err = merged.Close(); err != nil {
		return err
	}
	p.pending = p.pending[:len(p.pending)-1]
	return nil
}

func (p *IntervalSimilarityProgram) Rules() (*base.RuleSet, error) {
	if p == nil || p.closed || p.set == nil {
		return nil, ast.ErrASTHandle
	}
	return p.set, nil
}

func (p *IntervalSimilarityProgram) AuxiliaryPredicates() []ast.ASTHandle {
	if p == nil || p.closed {
		return nil
	}
	return append([]ast.ASTHandle(nil), p.auxiliary...)
}

// AuxiliaryRelation returns a borrowed numeric seed; evaluators clone it.
func (p *IntervalSimilarityProgram) AuxiliaryRelation(predicate ast.ASTHandle) (*IntervalRelation, bool) {
	if p == nil || p.closed {
		return nil, false
	}
	relation, found := p.seeds[predicate]
	return relation, found
}

func (p *IntervalSimilarityProgram) Modified() bool { return p != nil && !p.closed && p.modified }

func (p *IntervalSimilarityProgram) Close() error {
	if p == nil || p.closed {
		return nil
	}
	for len(p.pending) != 0 {
		last := len(p.pending) - 1
		if err := p.pending[last].Close(); err != nil {
			return err
		}
		p.pending = p.pending[:last]
	}
	if p.set != nil {
		if err := p.set.Close(); err != nil {
			return err
		}
		p.set = nil
	}
	for len(p.sourceRules) != 0 {
		last := len(p.sourceRules) - 1
		if err := p.sourceRules[last].Close(); err != nil {
			return err
		}
		p.sourceRules = p.sourceRules[:last]
	}
	if p.rules != nil {
		if err := p.rules.Close(); err != nil {
			return err
		}
		p.rules = nil
	}
	for len(p.refs) != 0 {
		last := len(p.refs) - 1
		err := p.manager.DelRef(&p.refs[last])
		if p.refs[last] == (ast.ASTRef{}) {
			p.refs = p.refs[:last]
		}
		if err != nil {
			return err
		}
	}
	p.seeds, p.origins, p.auxiliary, p.closed = nil, nil, nil, true
	return nil
}
