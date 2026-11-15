package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp, bmc::query nonlinear dispatch.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

const nonlinearBMCWarning = "WARNING: non-linear BMC is highly inefficient\n"

func warnNonlinearBMC() {
	enabled, err := diagnostic.ProcessVerboseEnabled(0)
	if err != nil || !enabled {
		return
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return
	}
	// ostream insertion failure only marks the source stream bad; it does not
	// change the fixedpoint result.
	_ = out.AppendCString(nonlinearBMCWarning)
}

// writeBMCContextRules is bmc::query's IF_VERBOSE(2,
// m_ctx.display_rules(verbose_stream())): the context's rules as they were
// before mk_query, so without the synthetic query!N rule g3 already holds.
func (e *PropositionalEngine) writeBMCContextRules(query ast.ASTHandle) (result error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || !enabled {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if !e.auxiliary[query] {
		return e.writeRuleSetDisplay(pdrVerboseRuleWriter{stream: stream}, e.set)
	}
	manager, err := e.set.RuleManager()
	if err != nil {
		return err
	}
	old, err := base.NewRuleSet(manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, old.Close()) }()
	if err = old.InheritPredicates(e.set); err != nil {
		return err
	}
	n, err := e.set.Len()
	if err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		rule, err := e.set.Rule(i)
		if err != nil {
			return err
		}
		head, err := rule.HeadDeclaration()
		if err != nil {
			return err
		}
		if head == query {
			continue
		}
		if err = old.Add(rule); err != nil {
			return err
		}
	}
	return e.writeRuleSetDisplay(pdrVerboseRuleWriter{stream: stream}, old)
}
