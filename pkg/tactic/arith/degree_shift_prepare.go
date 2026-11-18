package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp, discard_non_candidates
// and prepare_substitution.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type degreeShiftEntry struct {
	original, fresh, proof ast.ASTRef
	// definition is (^ fresh 1/k), built by prepare_substitution for the
	// extension model converter when models are enabled.
	definition ast.ASTRef
	degree     unumeral.Rational // m_var2degree's k (owned)
}
type degreeShiftPlan struct {
	manager *ast.ASTManager
	entries []degreeShiftEntry
}

func (p *degreeShiftPlan) Close() error {
	if p == nil || p.manager == nil {
		return nil
	}
	q := p.manager.NumeralService().Manager
	for i := range p.entries {
		if err := q.Del(&p.entries[i].degree); err != nil {
			return err
		}
		for _, ref := range []*ast.ASTRef{&p.entries[i].proof, &p.entries[i].definition, &p.entries[i].fresh, &p.entries[i].original} {
			if *ref != (ast.ASTRef{}) {
				if err := p.manager.DelRef(ref); err != nil {
					return err
				}
			}
		}
	}
	p.entries = nil
	p.manager = nil
	return nil
}
func degreeShiftDefinitionProof(m *ast.ASTManager, original, fresh ast.ASTHandle, degree *unumeral.Rational) (result ast.ASTRef, err error) {
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	pin := func(h ast.ASTHandle) error {
		r, e := m.NewRef(h)
		if e == nil {
			refs = append(refs, r)
		}
		return e
	}
	exponent, err := m.ArithmeticNumeralFromRational(degree, false)
	if err != nil {
		return result, err
	}
	if err = pin(exponent); err != nil {
		return result, err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return result, err
	}
	power, err := m.MkFamilyApp(family, 18, nil, []ast.ASTHandle{original, exponent})
	if err != nil {
		return result, err
	}
	if err = pin(power); err != nil {
		return result, err
	}
	equation, err := m.MkEq(fresh, power)
	if err != nil {
		return result, err
	}
	if err = pin(equation); err != nil {
		return result, err
	}
	intro, err := m.MkProofDefIntro(equation)
	if err != nil {
		return result, err
	}
	if err = pin(intro); err != nil {
		return result, err
	}
	proof, err := m.MkProofApplyDef(fresh, power, intro)
	if err != nil {
		return result, err
	}
	return m.NewRef(proof)
}
func degreeShiftPrepare(m *ast.ASTManager, roots []ast.ASTHandle, proofs bool) (plan *degreeShiftPlan, err error) {
	return degreeShiftPrepareWithCheckpoint(m, roots, proofs, nil)
}

func degreeShiftPrepareWithCheckpoint(m *ast.ASTManager, roots []ast.ASTHandle, proofs bool, checkpoint func() error) (plan *degreeShiftPlan, err error) {
	return degreeShiftPrepareSubstitution(m, roots, proofs, false, checkpoint)
}

// degreeShiftModelDefinition is mk_power(fresh, rational(1)/k).
func degreeShiftModelDefinition(m *ast.ASTManager, fresh ast.ASTHandle, degree *unumeral.Rational) (ast.ASTRef, error) {
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTRef{}, err
	}
	// rational(1)/k
	q := m.NumeralService().Manager
	inverse := unumeral.NewRational()
	err = q.Inv(degree, &inverse)
	var exponent ast.ASTHandle
	if err == nil {
		exponent, err = m.ArithmeticNumeralFromRational(&inverse, false)
	}
	if err = errors.Join(err, q.Del(&inverse)); err != nil {
		return ast.ASTRef{}, err
	}
	exponentRef, err := m.NewRef(exponent)
	if err != nil {
		return ast.ASTRef{}, err
	}
	definition, err := m.MkFamilyApp(family, 18, nil, []ast.ASTHandle{fresh, exponent})
	var result ast.ASTRef
	if err == nil {
		result, err = m.NewRef(definition)
	}
	return result, errors.Join(err, m.DelRef(&exponentRef))
}

// degreeShiftPrepareSubstitution is prepare_substitution: per table entry it
// creates the fresh constant, then (models) the 1/k power for the extension
// model converter, then (proofs) the definition proof, all before rewriting.
func degreeShiftPrepareSubstitution(m *ast.ASTManager, roots []ast.ASTHandle, proofs, models bool, checkpoint func() error) (plan *degreeShiftPlan, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	degrees, order, err := degreeShiftCandidatesOrdered(m, roots, checkpoint)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, degreeShiftReleaseDegrees(m, degrees)) }()
	// prepare_substitution walks m_var2degree in table order.
	type item struct{ h ast.ASTHandle }
	items := make([]item, 0, len(order))
	for _, h := range order {
		items = append(items, item{h})
	}
	plan = &degreeShiftPlan{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, plan.Close())
			plan = nil
		}
	}()
	for _, entry := range items {
		original, e := m.NewRef(entry.h)
		if e != nil {
			return plan, e
		}
		plan.entries = append(plan.entries, degreeShiftEntry{original: original, degree: unumeral.NewRational()})
		current := &plan.entries[len(plan.entries)-1]
		if e = m.NumeralService().Manager.Copy(&current.degree, degrees[entry.h]); e != nil {
			return plan, e
		}
		sort, e := m.GetSort(entry.h)
		if e != nil {
			return plan, e
		}
		fresh, e := m.MkFreshConstNullable(nil, sort)
		if e != nil {
			return plan, e
		}
		current.fresh, e = m.NewRef(fresh)
		if e != nil {
			return plan, e
		}
		if models {
			current.definition, e = degreeShiftModelDefinition(m, fresh, &current.degree)
			if e != nil {
				return plan, e
			}
		}
		if proofs {
			current.proof, e = degreeShiftDefinitionProof(m, entry.h, fresh, &current.degree)
			if e != nil {
				return plan, e
			}
		}
	}
	return plan, nil
}
