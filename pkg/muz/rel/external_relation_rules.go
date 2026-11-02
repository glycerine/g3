package rel

// Source: src/muz/rel/dl_compiler.cpp fact and single-tail rule compilation,
// specialized for src/muz/rel/dl_external_relation.cpp operations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type externalFactRule struct {
	head, tail ast.ASTHandle
	values     []ast.ASTHandle
	removed    []uint32
	cycles     [][]uint32
}

func classifyExternalFactRule(manager *ast.ASTManager, rule *base.Rule) (externalFactRule, error) {
	var result externalFactRule
	head, err := rule.Head()
	if err != nil {
		return result, err
	}
	result.head, err = manager.GetDecl(head)
	if err != nil {
		return result, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return result, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return result, err
	}
	tails, err := rule.TailSize()
	if err != nil || positive < 0 || positive > 1 || uninterpreted != positive || tails != positive {
		return result, errors.Join(err, ErrCompilerUnsupported)
	}
	headCount, err := manager.GetNumArgs(head)
	if err != nil {
		return result, err
	}
	headArgs := make([]ast.ASTHandle, headCount)
	for index := uint32(0); index < headCount; index++ {
		headArgs[index], err = manager.GetArg(head, index)
		if err != nil {
			return result, err
		}
	}
	if positive == 0 {
		for _, value := range headArgs {
			valid, cause := manager.IsValue(value)
			if cause != nil || !valid {
				return result, errors.Join(cause, ErrCompilerUnsupported)
			}
		}
		result.values = headArgs
		return result, nil
	}
	tail, negated, err := rule.Tail(0)
	if err != nil || negated {
		return result, errors.Join(err, ErrCompilerUnsupported)
	}
	result.tail, err = manager.GetDecl(tail)
	if err != nil {
		return result, err
	}
	tailCount, err := manager.GetNumArgs(tail)
	if err != nil {
		return result, err
	}
	positions := make(map[ast.ASTHandle]uint32, tailCount)
	for index := uint32(0); index < tailCount; index++ {
		argument, cause := manager.GetArg(tail, index)
		if cause != nil {
			return result, cause
		}
		kind, cause := manager.GetKind(argument)
		if cause != nil || kind != ast.ASTVar {
			return result, errors.Join(cause, ErrCompilerUnsupported)
		}
		if _, found := positions[argument]; found {
			return result, ErrCompilerUnsupported
		}
		positions[argument] = index
	}
	used := make([]bool, tailCount)
	headSeen := make(map[ast.ASTHandle]bool, len(headArgs))
	headPositions := make([]uint32, len(headArgs))
	for index, argument := range headArgs {
		position, found := positions[argument]
		if !found || headSeen[argument] {
			return result, ErrCompilerUnsupported
		}
		headSeen[argument] = true
		used[position] = true
		headPositions[index] = position
	}
	compact := make(map[uint32]uint32, len(headArgs))
	next := uint32(0)
	for index := range used {
		if !used[index] {
			result.removed = append(result.removed, uint32(index))
			continue
		}
		compact[uint32(index)] = next
		next++
	}
	// external_relation::rename uses a cycle c where new[c[i]] receives
	// old[c[i+1]]. Decompose the desired destination-to-source permutation.
	permutation := make([]uint32, len(headPositions))
	for destination, original := range headPositions {
		permutation[destination] = compact[original]
	}
	visited := make([]bool, len(permutation))
	for start := range permutation {
		if visited[start] || permutation[start] == uint32(start) {
			visited[start] = true
			continue
		}
		cycle := make([]uint32, 0, len(permutation))
		current := uint32(start)
		for !visited[current] {
			visited[current] = true
			cycle = append(cycle, current)
			current = permutation[current]
		}
		if current != uint32(start) {
			return result, ErrCompilerUnsupported
		}
		result.cycles = append(result.cycles, cycle)
	}
	return result, nil
}

// ExecuteExternalFactRules runs the concrete source-reached external relation
// slice: ground facts and single positive-tail variable projections whose head
// columns preserve source order. It validates the whole set before mutation.
func ExecuteExternalFactRules(plugin *ExternalRelationPlugin, set *base.RuleSet) (resultErr error) {
	if plugin == nil || plugin.closed || plugin.ast == nil || set == nil {
		return ErrCompilerUnsupported
	}
	count, err := set.Len()
	if err != nil {
		return err
	}
	compiled := make([]externalFactRule, 0, count)
	for index := 0; index < count; index++ {
		rule, cause := set.Rule(index)
		if cause != nil {
			return cause
		}
		entry, cause := classifyExternalFactRule(plugin.ast, rule)
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return cause
		}
		compiled = append(compiled, entry)
	}
	boolean, err := plugin.ast.MkBoolSort()
	if err != nil {
		return err
	}
	validated := make(map[ast.ASTHandle]bool)
	for _, entry := range compiled {
		for _, predicate := range []ast.ASTHandle{entry.head, entry.tail} {
			if predicate.IsNull() || validated[predicate] {
				continue
			}
			if _, _, err = plugin.manager.PredicateSignature(predicate); err != nil {
				return err
			}
			rangeSort, cause := plugin.ast.GetRange(predicate)
			if cause != nil || rangeSort != boolean {
				return errors.Join(cause, ErrCompilerUnsupported)
			}
			validated[predicate] = true
		}
	}
	for _, entry := range compiled {
		if !entry.tail.IsNull() {
			continue
		}
		target, cause := plugin.PredicateRelation(entry.head)
		if cause != nil {
			return cause
		}
		if cause = target.AddFact(entry.values); cause != nil {
			return cause
		}
	}
	// Without a callback-level changed flag, at most N monotone passes are
	// sufficient for every acyclic chain and for propagation around a cycle.
	for pass := 0; pass < len(compiled); pass++ {
		for _, entry := range compiled {
			if entry.tail.IsNull() {
				continue
			}
			source, cause := plugin.PredicateRelation(entry.tail)
			if cause != nil {
				return cause
			}
			value, cause := source.Clone()
			if cause != nil {
				return cause
			}
			if len(entry.removed) != 0 {
				projected, projectErr := value.Project(entry.removed)
				projectErr = errors.Join(projectErr, value.Close())
				if projectErr != nil {
					if projected != nil {
						projectErr = errors.Join(projectErr, projected.Close())
					}
					return projectErr
				}
				value = projected
			}
			for _, cycle := range entry.cycles {
				renamed, renameErr := value.Rename(cycle)
				renameErr = errors.Join(renameErr, value.Close())
				if renameErr != nil {
					if renamed != nil {
						renameErr = errors.Join(renameErr, renamed.Close())
					}
					return renameErr
				}
				value = renamed
			}
			target, cause := plugin.PredicateRelation(entry.head)
			if cause == nil {
				cause = target.Union(value, nil, false)
			}
			cause = errors.Join(cause, value.Close())
			if cause != nil {
				return cause
			}
		}
	}
	return nil
}
