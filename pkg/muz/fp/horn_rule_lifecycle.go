package fp

// Source: pdr_dl_interface::check_reset and
// spacer_dl_interface::check_reset transformed-rule retention.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// hornRuleSetSubsumes reports whether every new transformed rule is covered
// by an old transformed rule. Only then may learned frame properties survive.
func hornRuleSetSubsumes(oldSet, newSet *base.RuleSet) (_ bool, resultErr error) {
	if oldSet == nil || newSet == nil {
		return false, nil
	}
	oldCount, err := oldSet.Len()
	if err != nil {
		return false, err
	}
	newCount, err := newSet.Len()
	if err != nil {
		return false, err
	}
	for newIndex := 0; newIndex < newCount; newIndex++ {
		candidate, err := newSet.Rule(newIndex)
		if err != nil {
			return false, err
		}
		covered := false
		for oldIndex := 0; oldIndex < oldCount && !covered; oldIndex++ {
			oldRule, oldErr := oldSet.Rule(oldIndex)
			if oldErr == nil {
				covered, oldErr = base.BasicRuleSubsumes(oldRule, candidate)
			}
			if oldRule != nil {
				oldErr = errors.Join(oldErr, oldRule.Close())
			}
			if oldErr != nil {
				return false, errors.Join(oldErr, candidate.Close())
			}
		}
		if err = candidate.Close(); err != nil {
			return false, err
		}
		if !covered {
			return false, nil
		}
	}
	return true, nil
}

func snapshotHornRules(set *base.RuleSet) (*base.RuleSet, error) {
	if set == nil {
		return nil, ast.ErrASTDomain
	}
	rules, err := set.RuleManager()
	if err != nil {
		return nil, err
	}
	snapshot, err := base.NewRuleSet(rules)
	if err != nil {
		return nil, err
	}
	if err = snapshot.AddRules(set); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	if err = snapshot.InheritPredicates(set); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	return snapshot, nil
}
