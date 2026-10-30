package fp

// Source: the PDR dl_interface check_reset/update_rules lifecycle in
// src/muz/pdr/pdr_dl_interface.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (e *PropositionalEngine) canReusePDRRules(prepared *base.RuleSet, query ast.ASTHandle) (bool, error) {
	if e.pdr == nil || e.pdrRules == nil || e.pdrRuleRevision != e.revision || e.pdrQuery != query {
		return false, nil
	}
	subsumes, err := hornRuleSetSubsumes(e.pdrRules, prepared)
	if err != nil || !subsumes {
		return false, err
	}
	return e.pdr.CanUpdateRules(prepared)
}
