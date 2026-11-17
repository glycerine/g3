package smtcore

// Source: src/smt/theory_lra.cpp:1994-2142, retained fixed-bound equalities.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/util/numeral"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lp"
)

// smtLRAConstraintBound is constraint_bound (constraint index, rational);
// value is owned by its vector or trail entry.
type smtLRAConstraintBound struct {
	constraint uint32
	value      numeral.Rational
}
type smtLRATermBoundChange struct {
	index    uint32
	lower    bool
	previous smtLRAConstraintBound
}
type smtLRAFixedKey struct {
	value   string
	integer bool
}

func (t *SMTLRATheory) fixedKey(v SMTTheoryVar, value *numeral.Rational) (smtLRAFixedKey, error) {
	n, err := t.Enode(v)
	if err != nil {
		return smtLRAFixedKey{}, err
	}
	sort, err := t.state.manager.GetSort(n.owner)
	if err != nil {
		return smtLRAFixedKey{}, err
	}
	kind, err := t.state.manager.GetDeclKind(sort)
	if err != nil {
		return smtLRAFixedKey{}, err
	}
	text := t.lraText(value)
	return smtLRAFixedKey{text, kind == 1}, t.takeQErr(nil)
}

func (t *SMTLRATheory) setTermBound(index, witness uint32, value *numeral.Rational, lower bool) error {
	if !t.numeric.solver.IsTerm(index) {
		return nil
	}
	index -= lp.LARTermStartIndex
	vec := &t.upperTerms
	if lower {
		vec = &t.lowerTerms
	}
	for len(*vec) <= int(index) {
		*vec = append(*vec, smtLRAConstraintBound{constraint: smtLRANoIndex, value: smtQZero()})
	}
	previous := &(*vec)[index]
	cmp := t.lraCmp(&previous.value, value)
	if err := t.takeQErr(nil); err != nil {
		return err
	}
	if previous.constraint != smtLRANoIndex && (lower && cmp >= 0 || !lower && cmp <= 0) {
		return nil
	}
	// The trail takes the previous value; the vector entry gets a copy of
	// value.
	change := smtLRATermBoundChange{index: index, lower: lower}
	change.previous = *previous
	t.termBoundTrail = append(t.termBoundTrail, change)
	next := smtLRAConstraintBound{constraint: witness, value: smtQZero()}
	(*vec)[index] = next
	return t.numerals.Copy(&(*vec)[index].value, value)
}

func (t *SMTLRATheory) hasFixedBound(index uint32, value *numeral.Rational, lower bool) (uint32, bool, error) {
	if !t.numeric.solver.IsTerm(index) {
		return t.numeric.hasRationalBound(index, lower, value)
	}
	v := t.variableForLPIndex(index)
	if err := t.emitLRA("has_bound", 2057, func(out io.Writer) error { return smtWriteTheoryText(out, fmt.Sprintf("%d %d\n", index, v)) }); err != nil {
		return smtLRANoIndex, false, err
	}
	if v != SMTNullTheoryVar {
		n, err := t.Enode(v)
		if err != nil {
			return smtLRANoIndex, false, err
		}
		actual := smtQZero()
		number, err := t.numeral(n.owner, &actual)
		equal := number && err == nil && t.lraCmp(&actual, value) == 0
		if e := smtQDel(t.numerals, &actual); err == nil {
			err = e
		}
		if err = t.takeQErr(err); err != nil {
			return smtLRANoIndex, false, err
		}
		if equal {
			return smtLRANoIndex, true, nil
		}
	}
	index -= lp.LARTermStartIndex
	vec := t.upperTerms
	if lower {
		vec = t.lowerTerms
	}
	if uint64(index) >= uint64(len(vec)) {
		return smtLRANoIndex, false, nil
	}
	b := &vec[index]
	found := b.constraint != smtLRANoIndex && t.lraCmp(&b.value, value) == 0
	return b.constraint, found, t.takeQErr(nil)
}

func (t *SMTLRATheory) propagateFixedBounds(b *smtLRABound, witness uint32, truth bool) error {
	if !t.config.PropagateEqs || t.conflictCount >= t.config.PropagationThreshold || !truth || witness == smtLRANoIndex {
		return nil
	}
	index, ok := t.numeric.index(b.variable)
	if !ok {
		return ast.ErrASTDomain
	}
	lower := !b.upper
	if err := t.setTermBound(index, witness, &b.value, lower); err != nil {
		return err
	}
	_, fixed, err := t.hasFixedBound(index, &b.value, !lower)
	if err != nil || !fixed {
		return err
	}
	return t.fixedVariable(b.variable, &b.value)
}

func (t *SMTLRATheory) fixedVariable(v SMTTheoryVar, value *numeral.Rational) error {
	key, err := t.fixedKey(v, value)
	if err != nil {
		return err
	}
	other, found := t.fixedVariables[key]
	if !found || other < 0 || int(other) >= t.NumVariables() {
		t.fixedVariables[key] = v
		return nil
	}
	left, err := t.Enode(v)
	if err != nil {
		return err
	}
	right, err := t.Enode(other)
	if err != nil {
		return err
	}
	if left.Root() == right.Root() {
		t.fixedVariables[key] = v
		return nil
	}
	vi, err := t.numeric.getVarIndex(v)
	if err != nil {
		return err
	}
	wi, err := t.numeric.getVarIndex(other)
	if err != nil {
		return err
	}
	if err = t.emitLRA("fixed_var_eh", 2098, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "fixed: "); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, left.owner); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " "); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, right.owner); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " "+t.lraText(value)+" "); err != nil {
			return err
		}
		_, found, err := t.hasFixedBound(wi, value, true)
		if err != nil {
			return err
		}
		flag := 0
		if found {
			flag = 1
		}
		return smtWriteTheoryText(out, fmt.Sprintf("%d\n", flag))
	}); err != nil {
		return err
	}
	lo2, fixed, err := t.hasFixedBound(wi, value, true)
	if err != nil || !fixed {
		return err
	}
	hi2, fixed, err := t.hasFixedBound(wi, value, false)
	if err != nil || !fixed {
		return err
	}
	lo1, fixed, err := t.hasFixedBound(vi, value, true)
	if err != nil {
		return err
	}
	if !fixed {
		return ast.ErrASTDomain
	}
	hi1, fixed, err := t.hasFixedBound(vi, value, false)
	if err != nil {
		return err
	}
	if !fixed {
		return ast.ErrASTDomain
	}
	hook := smtLRABoundPropagation{owner: t}
	for _, witness := range []uint32{lo1, hi1, lo2, hi2} {
		if err = hook.Consume(nil, witness); err != nil {
			return err
		}
	}
	t.fixedEqualities++
	owner, err := NewSMTExtTheoryEqualityPropagationJustification(t.state.manager, t.FamilyID(), hook.core, nil, left, right, nil)
	if err != nil {
		return err
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	if err = t.emitLRA("fixed_var_eh", 2116, func(out io.Writer) error {
		for _, literal := range hook.core {
			if err := t.state.DisplayDetailedLiteral(out, literal); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
		if err := smtWriteTheoryText(out, " ==> "); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, left.owner); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " = "); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, right.owner); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	if err := t.checkValidation(smtLRAValidateEquality, hook.core, SMTFalseLiteral, left, right); err != nil {
		return err
	}
	return t.state.assignEquality(left, right, NewSMTEqualityJustification(owner))
}
