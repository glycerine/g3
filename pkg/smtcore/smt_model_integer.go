package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// integerCounterexample checks an evaluated one-variable body over the
// unbounded integers. An auxiliary solver supplied with its own assertions
// or search limit must handle the complete check itself.
func (c *SMTModelChecker) integerCounterexample(model *protomodel.ProtoModel, quantifier, flat, body ast.ASTHandle, remaining *uint32) (lbool.LBool, error) {
	m, aux := c.manager, c.auxiliaryContext
	if aux.manager != m || aux.ScopeLevelCore() != 1 || len(aux.tracked) != 0 || len(aux.kernel.assertions) != 0 ||
		aux.kernel.smtState.MaxConflicts != ^uint32(0) || c.trace || m.TraceStream != nil ||
		m.Budget().InjectionArmed() || m.ResourceLimit().Limit != 0 || m.ResourceLimit().CancelFlag() {
		return lbool.Undef, nil
	}
	sort, err := m.GetDeclarationSort(flat, 0)
	if err != nil {
		return lbool.Undef, err
	}
	kind, err := m.GetDeclKind(sort)
	if err != nil || kind != smtArithIntSort {
		return lbool.Undef, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return lbool.Undef, err
	}
	arith, err := m.ArithmeticFamilyID()
	if err != nil || family != arith {
		return lbool.Undef, err
	}
	if flat != quantifier {
		n, err := m.GetNumDeclarations(quantifier)
		if err != nil || n != 1 {
			return lbool.Undef, err
		}
		originalSort, err := m.GetDeclarationSort(quantifier, 0)
		if err != nil || originalSort != sort {
			return lbool.Undef, err
		}
	}
	finite, err := model.IsFinite(sort)
	if err != nil || finite {
		return lbool.Undef, err
	}
	p := compileIntegerRegions(m, arith, sort, body)
	if p == nil || len(p.ops) < 8 {
		return lbool.Undef, nil
	}
	if c.modelFinder != nil {
		found, err := c.integerInverseWitnesses(quantifier, p, remaining)
		if err != nil {
			return lbool.Undef, err
		}
		if found {
			return lbool.True, nil
		}
	}
	status, witness := p.counterexample(m)
	if status != lbool.True {
		return status, nil
	}
	h, err := m.ArithmeticNumeralInt64(witness, true)
	if err != nil {
		return lbool.Undef, err
	}
	inserted, err := c.addCounterexample(quantifier, []ast.ASTHandle{h})
	if inserted {
		*remaining--
	}
	return lbool.True, err
}

// Prefer existing ground terms, including their original instantiation
// generations, before introducing a new integer witness.
func (c *SMTModelChecker) integerInverseWitnesses(q ast.ASTHandle, p *integerRegions, remaining *uint32) (bool, error) {
	terms, err := c.modelFinder.InstantiationValues(q, 0)
	if err != nil || len(terms) > integerRegionWork/(len(p.ops)+len(p.args)) {
		return false, err
	}
	found := false
	for _, value := range terms {
		if c.manager.ResourceLimit().CancelFlag() {
			break
		}
		x, ok, err := c.manager.ArithmeticSmallInteger(value)
		if err != nil {
			return found, err
		}
		if !ok {
			continue
		}
		truth, _, ok := p.evaluate(int64(x))
		if !ok || truth {
			continue
		}
		term, generation, ok, err := c.modelFinder.Inverse(q, 0, value)
		if err != nil {
			return found, err
		}
		if !ok {
			continue
		}
		inserted, err := c.enqueueCounterexample(q, []ast.ASTHandle{term}, true, generation)
		if err != nil {
			return found, err
		}
		if inserted {
			found = true
			*remaining--
			if *remaining == 0 {
				break
			}
		}
	}
	return found, nil
}
