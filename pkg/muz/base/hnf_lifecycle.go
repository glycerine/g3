package base

// Source: src/muz/base/hnf.cpp imp::set_name/reset/get_fresh_predicates;
// src/muz/base/dl_rule.cpp rule_manager::mk_rule_core.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (n *HornNormalizer) open() error {
	if n == nil || n.manager == nil || n.closed {
		return ErrHornNormalizationOwner
	}
	return nil
}

// SetName selects the prefix for future definitions. A null symbol restores P.
func (n *HornNormalizer) SetName(name usymbol.Symbol) error {
	if err := n.open(); err != nil {
		return err
	}
	if name.IsNull() {
		var err error
		name, err = n.manager.Symbols.Intern("P")
		if err != nil {
			return err
		}
	}
	n.prefix = name
	return nil
}

// Reset clears source memo/cache roots without changing the name, hoister or
// current body. The next processing step clears that body. Previously returned
// results and fresh-predicate snapshots own their references.
func (n *HornNormalizer) Reset() error {
	if err := n.open(); err != nil {
		return err
	}
	err := errors.Join(closeASTRefs(n.manager, n.workRefs), closeASTRefs(n.manager, n.freshPredicates))
	n.workRefs = nil
	n.freshPredicates = nil
	return err
}

// FreshPredicates copies the source declaration vector into owning references.
// The caller releases every reference; an already-Horn invocation leaves this
// vector unchanged until Reset or a subsequent non-Horn invocation.
func (n *HornNormalizer) FreshPredicates() (result []ast.ASTRef, resultErr error) {
	if err := n.open(); err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(n.manager, result))
			result = nil
		}
	}()
	for _, ref := range n.freshPredicates {
		declaration, err := n.manager.RefGet(ref)
		if err != nil {
			return result, err
		}
		copyRef, err := n.manager.NewRef(declaration)
		if err != nil {
			return result, err
		}
		result = append(result, copyRef)
	}
	return result, nil
}

func (n *HornNormalizer) Close() error {
	if n == nil || n.manager == nil {
		return ErrHornNormalizationOwner
	}
	if n.closed {
		return nil
	}
	if err := n.Reset(); err != nil {
		return err
	}
	if err := n.beginHornStep(); err != nil {
		return err
	}
	if err := n.hoister.Close(); err != nil {
		return err
	}
	n.closed = true
	return nil
}

func (m *RuleManager) prepareHornNormalizer(name string) (*HornNormalizer, error) {
	if err := m.normalizer.Reset(); err != nil {
		return nil, err
	}
	var prefix usymbol.Symbol
	if name != "" {
		var err error
		prefix, err = m.manager.Symbols.Intern(name)
		if err != nil {
			return nil, err
		}
	}
	if err := m.normalizer.SetName(prefix); err != nil {
		return nil, err
	}
	return m.normalizer, nil
}

func (m *RuleManager) registerHornPredicates() (resultErr error) {
	declarations, err := m.normalizer.FreshPredicates()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(m.manager, declarations)) }()
	for _, ref := range declarations {
		declaration, err := m.manager.RefGet(ref)
		if err != nil {
			return err
		}
		if err = m.RegisterPredicate(declaration); err != nil {
			return err
		}
	}
	return nil
}

// Source m_refs owns memoized expressions/proofs, while m_body survives reset
// and is cleared at the start of the next mk_horn call or during destruction.
func (n *HornNormalizer) rememberHornRoot(h ast.ASTHandle) error {
	return n.pin(&n.workRefs, h)
}

func (n *HornNormalizer) beginHornStep() error {
	err := closeASTRefs(n.manager, n.bodyRefs)
	n.bodyRefs = nil
	return err
}

func (n *HornNormalizer) storeHornBody(body []ast.ASTHandle) error {
	for _, h := range body {
		if err := n.pin(&n.bodyRefs, h); err != nil {
			return err
		}
	}
	return nil
}

func (n *HornNormalizer) replaceHornBody(index int, body ast.ASTHandle) error {
	return n.manager.SetRef(n.bodyRefs[index], body)
}
