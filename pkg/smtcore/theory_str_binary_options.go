package smtcore

// Source: src/smt/theory_str.cpp binary_search_case_split,
// binary_search_length_test and gen_len_val_options_for_free_var dispatch;
// theory_str.h binary_search_info and binary_search_trail.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtStringBinaryBounds is binary_search_info: lowerBound, midPoint,
// upperBound and windowSize are rationals of the AST manager's rational
// manager. A stored window owns them; it is freed when its
// binary_search_len_tester_info entry is erased.
type smtStringBinaryBounds struct{ lower, midpoint, upper, window numeral.Rational }

func newSMTStringBinaryBounds() *smtStringBinaryBounds {
	return &smtStringBinaryBounds{lower: smtQZero(), midpoint: smtQZero(), upper: smtQZero(), window: smtQZero()}
}

func (b *smtStringBinaryBounds) del(q *numeral.RationalManager) error {
	return smtErrJoin(smtQDel(q, &b.lower), smtQDel(q, &b.midpoint), smtQDel(q, &b.upper), smtQDel(q, &b.window))
}

type smtStringBinaryTrail struct {
	level        uint32
	word, tester ast.ASTHandle
}

// calculateMidpoint is calculate_midpoint():
// midPoint = floor(lowerBound + ((upperBound - lowerBound) / rational(2))).
func (b *smtStringBinaryBounds) calculateMidpoint(q *numeral.RationalManager) error {
	tmp, two := smtQZero(), smtQInt(2)
	err := q.Sub(&b.upper, &b.lower, &tmp)
	if err == nil {
		err = q.Div(&tmp, &two, &tmp)
	}
	if err == nil {
		err = q.Add(&b.lower, &tmp, &tmp)
	}
	if err == nil {
		err = q.Floor(&tmp, &b.midpoint)
	}
	return smtErrJoin(err, smtQDel(q, &tmp))
}

func (t *SMTStringTheory) recordStringBinaryTest(word, tester ast.ASTHandle, bounds *smtStringBinaryBounds) {
	t.testers.binaryStacks[word] = append(t.testers.binaryStacks[word], tester)
	t.testers.binaryBounds[tester] = bounds
	t.testers.binaryTrail = append(t.testers.binaryTrail, smtStringBinaryTrail{level: t.level, word: word, tester: tester})
}

func (t *SMTStringTheory) popStringBinaryTests() {
	for len(t.testers.binaryTrail) > 0 {
		last := len(t.testers.binaryTrail) - 1
		entry := t.testers.binaryTrail[last]
		if entry.level <= t.level {
			break
		}
		// insert_obj_map::undo erases; it does not restore an old value.
		if bounds := t.testers.binaryBounds[entry.tester]; bounds != nil {
			t.keepQ(bounds.del(t.q()))
		}
		delete(t.testers.binaryBounds, entry.tester)
		stack := t.testers.binaryStacks[entry.word]
		if len(stack) > 0 {
			t.testers.binaryStacks[entry.word] = stack[:len(stack)-1]
		}
		t.testers.binaryTrail[last] = smtStringBinaryTrail{}
		t.testers.binaryTrail = t.testers.binaryTrail[:last]
	}
}

func (t *SMTStringTheory) generateStringBinaryCaseSplit(word, tester ast.ASTHandle, bounds *smtStringBinaryBounds) (output ast.ASTRef, result error) {
	if bounds == nil {
		return output, ast.ErrASTDomain
	}
	m := t.state.manager
	q := t.q()
	// rational N = midPoint, N_minus_one = N - 1, N_plus_one = N + 1
	n, nMinusOne, nPlusOne := smtQZero(), smtQZero(), smtQZero()
	defer t.qdel(&nPlusOne, &result)
	defer t.qdel(&nMinusOne, &result)
	defer t.qdel(&n, &result)
	if err := q.Copy(&n, &bounds.midpoint); err != nil {
		return output, err
	}
	if err := q.Copy(&nMinusOne, &n); err != nil {
		return output, err
	}
	if err := q.Dec(&nMinusOne); err != nil {
		return output, err
	}
	if err := q.Copy(&nPlusOne, &n); err != nil {
		return output, err
	}
	if err := q.Inc(&nPlusOne); err != nil {
		return output, err
	}
	midpointText, err := q.String(&n)
	if err != nil {
		return output, err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	length, err := pin(m.MkSequenceLength(word))
	if err != nil {
		return output, err
	}
	var choices, conjuncts []ast.ASTHandle
	for _, label := range []string{"less", "more", midpointText} {
		text, err := pin(t.stringLiteralChars(label))
		if err != nil {
			return output, err
		}
		choice, err := pin(t.state.mkEqualityAtom(tester, text))
		if err != nil {
			return output, err
		}
		choices = append(choices, choice)
		value := &n
		if label == "less" {
			value = &nMinusOne
		} else if label == "more" {
			value = &nPlusOne
		}
		number, err := pin(m.ArithmeticNumeralFromRational(value, true))
		if err != nil {
			return output, err
		}
		var relation ast.ASTHandle
		switch label {
		case "less":
			relation, err = pin(m.MkArithmeticLE(length, number))
		case "more":
			relation, err = pin(m.MkArithmeticGE(length, number))
		default:
			relation, err = pin(t.state.mkEqualityAtom(length, number))
		}
		if err != nil {
			return output, err
		}
		equivalence, err := pin(t.state.mkEqualityAtom(choice, relation))
		if err != nil {
			return output, err
		}
		conjuncts = append(conjuncts, equivalence)
	}
	disjunction, err := pin(m.MkOrArray(choices))
	if err != nil {
		return output, err
	}
	conjuncts = append(conjuncts, disjunction)
	// The native case_split_literals vector is unused, but its explicit
	// Boolean internalizations and literal lookups remain live side effects.
	for _, choice := range choices {
		if _, exists := t.state.BoolVar(choice); !exists {
			if err = t.state.Internalize(choice, false); err != nil {
				return output, err
			}
		}
		if _, err = t.state.Literal(choice); err != nil {
			return output, err
		}
	}
	formula, err := pin(m.MkAndArray(conjuncts))
	if err != nil {
		return output, err
	}
	return m.NewRef(formula)
}

func (t *SMTStringTheory) generateStringBinaryLengthOptions(word, callback ast.ASTHandle, callbackValue string) (output ast.ASTRef, result error) {
	m := t.state.manager
	stack := t.testers.binaryStacks[word]
	var tester ast.ASTHandle
	var bounds *smtStringBinaryBounds
	if len(stack) > 0 {
		last := stack[len(stack)-1]
		value, assigned, err := t.eqcStringValue(last)
		if err != nil {
			return output, err
		}
		var text string
		if !assigned {
			if callback != last {
				return output, ast.ErrASTDomain
			}
			text = callbackValue
		} else {
			text, _, err = m.SequenceStringValue(value)
			if err != nil {
				return output, err
			}
		}
		previous := t.testers.binaryBounds[last]
		if previous == nil {
			return output, ast.ErrASTDomain
		}
		if text != "more" && text != "less" {
			if previous.midpoint.IsNeg() {
				locals := smtStringLengthPremises{theory: t}
				defer func() { result = errors.Join(result, locals.close()) }()
				length, err := m.MkSequenceLength(word)
				if err != nil {
					return output, err
				}
				if err = locals.add(length); err != nil {
					return output, err
				}
				zero, err := m.ArithmeticIntegerNumeralText("0")
				if err != nil {
					return output, err
				}
				formula, err := m.MkArithmeticGE(length, zero)
				if err != nil {
					return output, err
				}
				return m.NewRef(formula)
			}
			return t.generateStringFreeValueOptions(word, last, text, ast.ASTHandle{}, "")
		}
		// binary_search_info newBounds = lastBounds (copies), then updated.
		q := t.q()
		bounds = newSMTStringBinaryBounds()
		recorded := false
		defer func() {
			if !recorded {
				result = smtErrJoin(result, bounds.del(q))
			}
		}()
		for _, pair := range [][2]*numeral.Rational{{&bounds.lower, &previous.lower}, {&bounds.midpoint, &previous.midpoint}, {&bounds.upper, &previous.upper}, {&bounds.window, &previous.window}} {
			if err := q.Copy(pair[0], pair[1]); err != nil {
				return output, err
			}
		}
		one, two := smtQInt(1), smtQInt(2)
		cache := t.testers.binaryLow
		if text == "more" {
			cache = t.testers.binaryHigh
			atWindow, err := q.Equal(&previous.midpoint, &previous.upper)
			if err != nil {
				return output, err
			}
			if atWindow {
				if atWindow, err = q.Equal(&previous.upper, &previous.window); err != nil {
					return output, err
				}
			}
			if atWindow {
				if err = q.Add(&previous.window, &one, &bounds.lower); err != nil {
					return output, err
				}
				if err = q.Mul(&previous.window, &two, &bounds.window); err != nil {
					return output, err
				}
				if err = q.Copy(&bounds.upper, &bounds.window); err != nil {
					return output, err
				}
			} else if err = q.Add(&previous.midpoint, &one, &bounds.lower); err != nil {
				return output, err
			}
		} else if err := q.Sub(&previous.midpoint, &one, &bounds.upper); err != nil {
			return output, err
		}
		if err := bounds.calculateMidpoint(q); err != nil {
			return output, err
		}
		tester = cache[last]
		if tester.IsNull() {
			fits, err := q.IsInt32(&bounds.midpoint)
			if err != nil {
				return output, err
			}
			if !fits {
				return output, ast.ErrASTDomain
			}
			mid, err := q.Int32(&bounds.midpoint)
			if err != nil {
				return output, err
			}
			tester, err = t.freshStringLengthTester(word, mid)
			if err != nil {
				return output, err
			}
			cache[last] = tester
		}
		if err = t.refreshStringVariable(tester); err != nil {
			return output, err
		}
		recorded = true
		t.recordStringBinaryTest(word, tester, bounds)
	} else {
		t.testers.binaryStacks[word] = nil
		upper := uint32(64)
		if t.parameterOwner != nil {
			options, err := t.stringParameterState()
			if err != nil {
				return output, err
			}
			upper = options.BinarySearchInitialUpperBound
		}
		// binary_search_info(0, 0, upper, upper)
		q := t.q()
		bounds = newSMTStringBinaryBounds()
		recorded := false
		defer func() {
			if !recorded {
				result = smtErrJoin(result, bounds.del(q))
			}
		}()
		if err := q.SetUint64(&bounds.upper, uint64(upper)); err != nil {
			return output, err
		}
		if err := q.SetUint64(&bounds.window, uint64(upper)); err != nil {
			return output, err
		}
		if err := bounds.calculateMidpoint(q); err != nil {
			return output, err
		}
		tester = t.testers.binaryStart[word]
		if tester.IsNull() {
			// A uint32 initial bound has an int32-representable midpoint.
			mid, err := q.Int64(&bounds.midpoint)
			if err != nil {
				return output, err
			}
			tester, err = t.freshStringLengthTester(word, int32(mid))
			if err != nil {
				return output, err
			}
			t.testers.binaryStart[word] = tester
		}
		if err := t.refreshStringVariable(tester); err != nil {
			return output, err
		}
		// rational freeVarLen; if (get_len_value(freeVar, freeVarLen)) ...
		known := smtQZero()
		defer t.qdel(&known, &result)
		ok, err := t.lengthValue(word, &known)
		if err != nil {
			return output, err
		}
		if ok {
			two := smtQInt(2)
			if err = q.Copy(&bounds.midpoint, &known); err != nil {
				return output, err
			}
			if err = q.Mul(&known, &two, &bounds.upper); err != nil {
				return output, err
			}
			if err = q.Copy(&bounds.window, &bounds.upper); err != nil {
				return output, err
			}
		}
		recorded = true
		t.recordStringBinaryTest(word, tester, bounds)
	}
	root, err := t.generateStringBinaryCaseSplit(word, tester, bounds)
	if err != nil {
		return output, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&root)) }()
	formula, err := m.RefGet(root)
	if err != nil {
		return output, err
	}
	if err = t.retain(formula); err != nil {
		return output, err
	}
	return m.NewRef(formula)
}

func (t *SMTStringTheory) generateStringLengthValueOptions(word, callback ast.ASTHandle, callbackValue string) (ast.ASTRef, error) {
	if t.parameterOwner != nil {
		options, err := t.stringParameterState()
		if err != nil {
			return ast.ASTRef{}, err
		}
		if options.UseBinarySearch {
			return t.generateStringBinaryLengthOptions(word, callback, callbackValue)
		}
	}
	return t.generateStringSequentialOptions(word, callback, callbackValue)
}
