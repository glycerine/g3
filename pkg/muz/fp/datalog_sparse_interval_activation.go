package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// sparseIntervalDatalogActivation owns seed storage and replaces its
// evaluator snapshot for each query. Engine selection remains in the caller.
type sparseIntervalDatalogActivation struct {
	manager    *ast.ASTManager
	rules      *base.RuleSet
	guard      *astrewriter.TheoryRewriter // borrowed
	store      *SparseIntervalDatalogStore
	evaluator  *SparseIntervalDatalogEvaluator
	predicates []ast.ASTHandle
	closed     bool
}

func newSparseIntervalDatalogActivation(manager *ast.ASTManager, rules *base.RuleSet, guard *astrewriter.TheoryRewriter, declarations []ast.ASTHandle) (_ *sparseIntervalDatalogActivation, selected bool, resultErr error) {
	if manager == nil || rules == nil || guard == nil || len(declarations) == 0 {
		return nil, false, nil
	}
	a := &sparseIntervalDatalogActivation{manager: manager, rules: rules, guard: guard, predicates: append([]ast.ASTHandle(nil), declarations...)}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, a.close())
		}
	}()
	var err error
	a.store, err = NewSparseIntervalDatalogStore(manager)
	if err != nil {
		return nil, true, err
	}
	seen := make(map[ast.ASTHandle]bool, len(declarations))
	for _, declaration := range declarations {
		if seen[declaration] {
			return nil, true, ErrSparseIntervalDatalogFragment
		}
		seen[declaration] = true
		if err = a.store.Register(declaration); err != nil {
			if errors.Is(err, rel.ErrSparseIntervalProduct) || errors.Is(err, ErrSparseIntervalDatalogFragment) {
				return nil, false, a.close()
			}
			return nil, true, err
		}
	}
	return a, true, nil
}

func (a *sparseIntervalDatalogActivation) open() error {
	if a == nil || a.closed || a.manager == nil || a.rules == nil || a.guard == nil || a.store == nil {
		return ErrSparseIntervalDatalogFragment
	}
	return nil
}

func (a *sparseIntervalDatalogActivation) discardEvaluator() error {
	if a.evaluator == nil {
		return nil
	}
	err := a.evaluator.Close()
	if err == nil {
		a.evaluator = nil
	}
	return err
}

func (a *sparseIntervalDatalogActivation) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := a.open(); err != nil {
		return err
	}
	if err := a.discardEvaluator(); err != nil {
		return err
	}
	return a.store.AddFact(declaration, values)
}

func (a *sparseIntervalDatalogActivation) syncPredicates(declarations []ast.ASTHandle) error {
	if err := a.open(); err != nil {
		return err
	}
	current := make(map[ast.ASTHandle]bool, len(declarations))
	for _, declaration := range declarations {
		if current[declaration] {
			return ErrSparseIntervalDatalogFragment
		}
		current[declaration] = true
	}
	for _, declaration := range a.predicates {
		if !current[declaration] {
			return ErrSparseIntervalDatalogFragment
		}
	}
	if err := a.discardEvaluator(); err != nil {
		return err
	}
	for _, declaration := range declarations {
		if a.store.entries[declaration] != nil {
			continue
		}
		if err := a.store.Register(declaration); err != nil {
			return errors.Join(err, ErrSparseIntervalDatalogFragment)
		}
		a.predicates = append(a.predicates, declaration)
	}
	return nil
}

func (a *sparseIntervalDatalogActivation) clone(rules *base.RuleSet, guard *astrewriter.TheoryRewriter) (_ *sparseIntervalDatalogActivation, resultErr error) {
	if err := a.open(); err != nil || rules == nil || guard == nil {
		if err == nil {
			err = ErrSparseIntervalDatalogFragment
		}
		return nil, err
	}
	copyActivation, selected, err := newSparseIntervalDatalogActivation(a.manager, rules, guard, a.predicates)
	if err != nil || !selected {
		return nil, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, copyActivation.close())
		}
	}()
	for declaration, source := range a.store.entries {
		clone, cause := source.relation.CloneTo(copyActivation.store.relations)
		if cause != nil {
			return nil, cause
		}
		destination := copyActivation.store.entries[declaration]
		if cause = destination.relation.Close(); cause != nil {
			return nil, errors.Join(cause, clone.Close())
		}
		destination.relation = clone
	}
	return copyActivation, nil
}

func (a *sparseIntervalDatalogActivation) query(declarations []ast.ASTHandle, widen bool) (lbool.LBool, error) {
	if err := a.open(); err != nil {
		return lbool.Undef, err
	}
	if err := a.discardEvaluator(); err != nil {
		return lbool.Undef, err
	}
	evaluator, err := a.store.Evaluator(a.rules, a.guard)
	if err != nil {
		return lbool.Undef, err
	}
	if err = evaluator.SaturatePositive(widen); err != nil {
		return lbool.Undef, errors.Join(err, evaluator.Close())
	}
	a.evaluator = evaluator
	return evaluator.Query(declarations)
}

func (a *sparseIntervalDatalogActivation) relation(declaration ast.ASTHandle) (*rel.SparseIntervalSieveProduct, error) {
	if err := a.open(); err != nil || a.evaluator == nil {
		if err == nil {
			err = ErrSparseIntervalDatalogFragment
		}
		return nil, err
	}
	return a.evaluator.Relation(declaration)
}

func (a *sparseIntervalDatalogActivation) relationFormula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := a.open(); err != nil || a.evaluator == nil {
		if err == nil {
			err = ErrSparseIntervalDatalogFragment
		}
		return ast.ASTRef{}, err
	}
	return a.evaluator.RelationFormula(declaration)
}

func (a *sparseIntervalDatalogActivation) reset() error {
	if err := a.open(); err != nil {
		return err
	}
	if err := a.discardEvaluator(); err != nil {
		return err
	}
	return a.store.Reset()
}

func (a *sparseIntervalDatalogActivation) close() (result error) {
	if a == nil || a.closed {
		return nil
	}
	result = errors.Join(result, a.discardEvaluator())
	if a.store != nil {
		result = errors.Join(result, a.store.Close())
	}
	if result != nil {
		return result
	}
	a.closed = true
	a.manager, a.rules, a.guard, a.store, a.predicates = nil, nil, nil, nil, nil
	return nil
}
