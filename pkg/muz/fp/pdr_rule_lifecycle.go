package fp

// Source: the PDR dl_interface check_reset/update_rules lifecycle in
// src/muz/pdr/pdr_dl_interface.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (e *PropositionalEngine) advanceRuleRevision() {
	// Rule changes cannot hide an earlier background or parameter change.
	if e.pdrRuleRevision == e.revision {
		e.pdrRuleRevision++
	}
	e.revision++
}

func (e *PropositionalEngine) canReusePDRSourceRules(query ast.ASTHandle) (bool, error) {
	if e.pdr == nil || e.pdrSourceRules == nil || e.pdrRuleRevision != e.revision || e.pdrQuery != query {
		return false, nil
	}
	// Source check_reset compares rules before query preprocessing.
	missing, err := firstUncoveredHornRule(e.pdrSourceRules, e.set)
	if err != nil {
		return false, err
	}
	if missing >= 0 {
		oldCount, err := e.pdrSourceRules.Len()
		if err == nil && oldCount > 0 {
			err = e.tracePDRFreshRule(missing)
		}
		return false, err
	}
	return true, nil
}
