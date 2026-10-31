package rel

// Source: dl_mk_similarity_compressor.cpp operator()/process_class and
// dl_mk_similarity_compressor.h. This first integration uses finite tables.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// FiniteSimilarityCompressor is the finite-table specialization of the
// similarity transformation. It borrows its DatalogContext and relation
// manager. Unsupported fact domains return an error rather than being skipped.
type FiniteSimilarityCompressor struct {
	self      *FiniteSimilarityCompressor
	context   *base.DatalogContext
	relations *RelationManager
	threshold uint32
	closed    bool
}

func NewFiniteSimilarityCompressor(context *base.DatalogContext, relations *RelationManager, threshold uint32) (*FiniteSimilarityCompressor, error) {
	if context == nil || relations == nil || threshold <= 1 {
		return nil, ast.ErrASTDomain
	}
	m, e := context.Manager()
	if e != nil {
		return nil, e
	}
	if e = relations.open(); e != nil {
		return nil, e
	}
	if relations.manager != m {
		return nil, ast.ErrASTDomain
	}
	p := &FiniteSimilarityCompressor{context: context, relations: relations, threshold: threshold}
	p.self = p
	return p, nil
}
func (p *FiniteSimilarityCompressor) open() error {
	if p == nil || p.self != p || p.closed {
		return ast.ErrASTHandle
	}
	if _, e := p.context.Manager(); e != nil {
		return e
	}
	return p.relations.open()
}
func (p *FiniteSimilarityCompressor) Priority() uint32            { return 5000 }
func (p *FiniteSimilarityCompressor) CanDestratifyNegation() bool { return false }
func (p *FiniteSimilarityCompressor) Cancel()                     {}
func (p *FiniteSimilarityCompressor) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	p.closed = true
	return nil
}

// Transform returns a new rule set only when duplicates are removed or a
// class is compressed. Side effects on generated predicates and relations
// occur in the same class order as the source.
func (p *FiniteSimilarityCompressor) Transform(source *base.RuleSet) (result *base.RuleSet, err error) {
	if err = p.open(); err != nil {
		return nil, err
	}
	sourceRM, e := source.RuleManager()
	if e != nil {
		return nil, e
	}
	rm, e := p.context.RuleManager()
	if e != nil {
		return nil, e
	}
	if sourceRM != rm {
		return nil, ast.ErrASTDomain
	}
	n, e := source.Len()
	if e != nil {
		return nil, e
	}
	var rules []*base.Rule
	defer func() {
		for i := len(rules) - 1; i >= 0; i-- {
			err = errors.Join(err, rules[i].Close())
		}
		if err != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	for i := 0; i < n; i++ {
		r, e := source.Rule(i)
		if e != nil {
			return nil, e
		}
		rules = append(rules, r)
	}
	classes, changed, e := similarityClasses(source, rules, p.threshold)
	if e != nil {
		return nil, e
	}
	for _, class := range classes {
		if class.merge {
			changed = true
			break
		}
	}
	if !changed {
		return nil, nil
	}
	result, e = base.NewRuleSet(rm)
	if e != nil {
		return nil, e
	}
	for _, class := range classes {
		if class.merge {
			_, merged, e := similarityMergeFinite(p.context, p.relations, class)
			if e != nil {
				return result, e
			}
			e = result.Add(merged)
			e = errors.Join(e, merged.Close())
			if e != nil {
				return result, e
			}
		} else {
			for _, r := range class.rules {
				if e = result.Add(r); e != nil {
					return result, e
				}
			}
		}
	}
	if e = result.InheritPredicates(source); e != nil {
		return result, e
	}
	return result, nil
}
