package fp

// Source: src/muz/pdr/pdr_context.cpp model_search::get_proof_trace, which
// builds datalog::rule_unifier unif(dctx) over the datalog context's rule
// manager and services.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

// pdrProofRuleServices is the datalog::context seen by get_proof_trace.
type pdrProofRuleServices struct{ engine *PropositionalEngine }

// NewProofRuleUnifier is datalog::rule_unifier unif(dctx): the context's
// rule manager, fix_unbound_vars with its qe_lite, and an interpreted-tail
// simplifier sharing the context rewriter.
func (s pdrProofRuleServices) NewProofRuleUnifier() (*transforms.NormalizingRuleUnifier, error) {
	e := s.engine
	plugin := transforms.NewRuleInlinerPluginWithServices(e.manager, e.rules, []ast.ASTHandle{}, true, true, e.params, e.globals, 35000)
	if e.params != nil && e.globals != nil {
		normalization, err := e.ruleNormalizationParams()
		if err != nil {
			return nil, errors.Join(err, plugin.Close())
		}
		plugin.SetNormalizationParams(normalization)
		plugin.SetRuleManagerQELite(e.ruleManagerLite)
	}
	plugin.SetContextRewriter(e.contextRewriter())
	unifier, err := transforms.NewNormalizingRuleUnifier(plugin)
	if err != nil {
		return nil, errors.Join(err, plugin.Close())
	}
	return unifier, nil
}
