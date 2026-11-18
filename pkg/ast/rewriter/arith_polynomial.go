package rewriter

// Source: src/ast/rewriter/poly_rewriter.h;
// src/ast/rewriter/poly_rewriter_def.h.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

const arithRelationEq int32 = -1

// ErrArithPolynomialMonomialBlowup is mk_nflat_mul_core's
// throw rewriter_exception("sum of monomials blowup").
var ErrArithPolynomialMonomialBlowup error = newRewriterException("sum of monomials blowup")

const (
	arithDivideExact = iota
	arithDivideFloor
	arithDivideCeil
)

func (r *ArithRewriter) integerTerm(h ast.ASTHandle) (bool, error) {
	s, err := r.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	family, err := r.manager.GetFamilyID(s)
	if err != nil || family != r.family {
		return false, err
	}
	kind, err := r.manager.GetDeclKind(s)
	return err == nil && kind == 1, err
}

// numeralForTerm is mk_numeral(value) under set_curr_sort(term's sort).
func (r *ArithRewriter) numeralForTerm(value *unumeral.Rational, term ast.ASTHandle) (ast.ASTHandle, error) {
	integer, err := r.integerTerm(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.makeNumeral(value, integer)
}

// numeralForTermInt is mk_numeral(numeral(v)) under set_curr_sort.
func (r *ArithRewriter) numeralForTermInt(v int32, term ast.ASTHandle) (ast.ASTHandle, error) {
	integer, err := r.integerTerm(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.makeNumeralInt(v, integer)
}

func (r *ArithRewriter) makeAdd(args []ast.ASTHandle, term ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return r.numeralForTermInt(0, term)
	case 1:
		return args[0], nil
	default:
		return r.app(arithOpAdd, args...)
	}
}

func (r *ArithRewriter) makeMul(args []ast.ASTHandle, term ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return r.numeralForTermInt(1, term)
	case 1:
		return args[0], nil
	default:
		return r.app(arithOpMul, args...)
	}
}

// powerBody is the arithmetic specialization of get_power_body. Only an
// integer exponent greater than one participates in adjacent-factor folding.
func (r *ArithRewriter) powerBody(h ast.ASTHandle, exponent *unumeral.Rational) (ast.ASTHandle, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTApp {
		_, args, power, err := r.operation(h, arithOpPower)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if power && len(args) == 2 {
			_, numeral, e := r.numeral(args[1], exponent)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if numeral && exponent.IsInt() {
				greater, e := r.q().Compare(exponent, arithQOne())
				if e != nil {
					return ast.ASTHandle{}, e
				}
				if greater > 0 {
					return args[0], nil
				}
			}
		}
	}
	return h, r.q().SetInt32(exponent, 1)
}

func (r *ArithRewriter) sameArguments(args ast.AppArguments, values []ast.ASTHandle) (bool, error) {
	if uint32(len(values)) != args.Count() {
		return false, nil
	}
	for i := range values {
		h, err := args.At(uint32(i))
		if err != nil {
			return false, err
		}
		if h != values[i] {
			return false, nil
		}
	}
	return true, nil
}

func (r *ArithRewriter) monomialOrdinal(h ast.ASTHandle) (int64, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil {
		return 0, err
	}
	if kind == ast.ASTVar {
		id, err := r.manager.GetASTID(h)
		return int64(id), err
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil {
		return 0, err
	}
	if family != r.family {
		id, err := r.manager.GetASTID(h)
		return int64(id), err
	}
	if _, factors, product, err := r.operation(h, arithOpMul); err != nil {
		return 0, err
	} else if product {
		if len(factors) != 0 {
			if numeral, err := r.isNumeral(factors[0]); err != nil {
				return 0, err
			} else if numeral && len(factors) > 1 {
				id, err := r.manager.GetASTID(factors[1])
				return int64(id), err
			}
		}
		id, err := r.manager.GetASTID(h)
		return int64(id), err
	}
	if numeral, err := r.isNumeral(h); err != nil {
		return 0, err
	} else if numeral {
		return -1, nil
	}
	if r.mulToPower && !r.expandPower {
		if _, args, power, err := r.operation(h, arithOpPower); err != nil {
			return 0, err
		} else if power && len(args) == 2 {
			// is_numeral(arg, k) && k > 1
			exponent := unumeral.NewRational()
			_, numeral, err := r.numeral(args[1], &exponent)
			greater := 0
			if err == nil && numeral {
				greater, err = r.q().Compare(&exponent, arithQOne())
			}
			if err = errors.Join(err, arithQDel(r.q(), &exponent)); err != nil {
				return 0, err
			}
			if greater > 0 {
				id, err := r.manager.GetASTID(args[0])
				return int64(id), err
			}
		}
	}
	id, err := r.manager.GetASTID(h)
	return int64(id), err
}

func (r *ArithRewriter) monomialLess(left, right ast.ASTHandle) (bool, error) {
	if !r.arithIneqLHS {
		return r.manager.ASTLess(left, right)
	}
	leftOrdinal, err := r.monomialOrdinal(left)
	if err != nil {
		return false, err
	}
	rightOrdinal, err := r.monomialOrdinal(right)
	return leftOrdinal < rightOrdinal, err
}

func (r *ArithRewriter) hoistEqualCoefficients(terms []ast.ASTHandle, term ast.ASTHandle) (_ []ast.ASTHandle, _ bool, err error) {
	if !r.hoistCoeff {
		return terms, false, nil
	}
	q := r.q()
	// Each term's coefficient and power product, read once.
	n := len(terms)
	coefficients := make([]unumeral.Rational, n)
	products := make([]ast.ASTHandle, n)
	for i := range coefficients {
		coefficients[i] = unumeral.NewRational()
	}
	defer func() {
		for i := range coefficients {
			err = errors.Join(err, arithQDel(q, &coefficients[i]))
		}
	}()
	for i, item := range terms {
		if products[i], err = r.monomial(item, &coefficients[i]); err != nil {
			return nil, false, err
		}
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	var sortErr error
	sort.SliceStable(order, func(a, b int) bool {
		i, j := order[a], order[b]
		leftMul, rightMul := products[i] != terms[i], products[j] != terms[j]
		if leftMul != rightMul {
			return !leftMul
		}
		if !leftMul {
			leftID, err := r.manager.GetASTID(terms[i])
			if err != nil {
				sortErr = err
				return false
			}
			rightID, err := r.manager.GetASTID(terms[j])
			if err != nil {
				sortErr = err
				return false
			}
			return leftID < rightID
		}
		comparison, err := q.Compare(&coefficients[i], &coefficients[j])
		if err != nil {
			sortErr = err
			return false
		}
		if comparison != 0 {
			return comparison < 0
		}
		leftID, err := r.manager.GetASTID(products[i])
		if err != nil {
			sortErr = err
			return false
		}
		rightID, err := r.manager.GetASTID(products[j])
		if err != nil {
			sortErr = err
			return false
		}
		return leftID < rightID
	})
	if sortErr != nil {
		return nil, false, sortErr
	}
	// grouped: a monomial c*p with c != 1; a group is every grouped term
	// whose coefficient equals the first one's.
	grouped := func(i int) bool { return products[i] != terms[i] && !coefficients[i].IsOne() }
	consumed := make([]bool, n)
	result := make([]ast.ASTHandle, 0, n)
	changed := false
	for at, i := range order {
		if consumed[at] {
			continue
		}
		if !grouped(i) {
			result = append(result, terms[i])
			continue
		}
		members := []int{at}
		for later := at + 1; later < n; later++ {
			j := order[later]
			if consumed[later] || !grouped(j) {
				continue
			}
			same, e := q.Equal(&coefficients[i], &coefficients[j])
			if e != nil {
				return nil, false, e
			}
			if same {
				members = append(members, later)
			}
		}
		if len(members) < 2 {
			result = append(result, terms[i])
			continue
		}
		parts := make([]ast.ASTHandle, len(members))
		for k, member := range members {
			consumed[member] = true
			parts[k] = products[order[member]]
		}
		sum, err := r.makeAdd(parts, term)
		if err != nil {
			return nil, false, err
		}
		hoisted, err := r.coefficientProduct(&coefficients[i], sum, term)
		if err != nil {
			return nil, false, err
		}
		result = append(result, hoisted)
		changed = true
	}
	return result, changed, nil
}

// toBuffer is the TO_BUFFER macro of poly_rewriter_def.h: slot i is replaced
// by the first argument of a nested application of kind and the remaining
// arguments are appended, until no slot holds such an application.
func (r *ArithRewriter) toBuffer(kind int32, e ast.ASTHandle) ([]ast.ASTHandle, error) {
	buffer := []ast.ASTHandle{e}
	for i := 0; i < len(buffer); {
		_, children, matched, err := r.operation(buffer[i], kind)
		if err != nil {
			return nil, err
		}
		if !matched || len(children) == 0 {
			i++
			continue
		}
		buffer[i] = children[0]
		buffer = append(buffer, children[1:]...)
	}
	return buffer, nil
}

// mergeMuls is poly_rewriter::merge_muls.
func (r *ArithRewriter) mergeMuls(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	m1, err := r.toBuffer(arithOpMul, x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m2, err := r.toBuffer(arithOpMul, y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	k := 0
	for i := 0; i < len(m1); i++ {
		x = m1[i]
		found := false
		j := k
		for ; j < len(m2); j++ {
			if m2[j] == x {
				found = true
				break
			}
		}
		if found {
			m1[i], m1[k] = m1[k], m1[i]
			m2[j], m2[k] = m2[k], m2[j]
			k++
		}
	}
	left, err := r.makeMul(append([]ast.ASTHandle(nil), m1[k:]...), x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := r.makeMul(append([]ast.ASTHandle(nil), m2[k:]...), x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if k == len(m1) {
		m1 = append(m1, ast.ASTHandle{})
	}
	m1[k], err = r.makeAdd([]ast.ASTHandle{left, right}, x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.makeMul(append([]ast.ASTHandle(nil), m1[:k+1]...), x)
}

// hoistCommonMultiplication is poly_rewriter::hoist_multiplication(som).
func (r *ArithRewriter) hoistCommonMultiplication(som ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if !r.hoistMul {
		return ast.ASTHandle{}, false, nil
	}
	adds, err := r.toBuffer(arithOpAdd, som)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	valid := make([]bool, len(adds))
	for i := range valid {
		valid[i] = true
	}
	mulMap := make(map[ast.ASTHandle]int)
	changed := false
	for k := 0; k < len(adds); k++ {
		muls, err := r.toBuffer(arithOpMul, adds[k])
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		for _, e := range muls {
			if numeral, err := r.isNumeral(e); err != nil {
				return ast.ASTHandle{}, false, err
			} else if numeral {
				continue
			}
			if j, found := mulMap[e]; found && valid[j] && j != k {
				term := adds[k]
				adds[j], err = r.mergeMuls(adds[j], adds[k])
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				adds[k], err = r.numeralForTermInt(0, term)
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				valid[j] = false
				valid[k] = false
				changed = true
				break
			}
			mulMap[e] = k
		}
	}
	if !changed {
		return ast.ASTHandle{}, false, nil
	}
	result, err := r.makeAdd(adds, som)
	return result, err == nil, err
}

func (r *ArithRewriter) flattenArguments(args ast.AppArguments, kind int32, recursive bool) ([]ast.ASTHandle, bool, error) {
	values := make([]ast.ASTHandle, 0, args.Count())
	changed := false
	for i := uint32(0); i < args.Count(); i++ {
		h, err := args.At(i)
		if err != nil {
			return nil, false, err
		}
		_, children, matched, err := r.operation(h, kind)
		if err != nil {
			return nil, false, err
		}
		if !r.flat || !matched {
			values = append(values, h)
			continue
		}
		changed = true
		if !recursive {
			values = append(values, children...)
			continue
		}
		stack := append([]ast.ASTHandle(nil), children...)
		for len(stack) != 0 {
			current := stack[0]
			stack = stack[1:]
			_, nested, yes, e := r.operation(current, kind)
			if e != nil {
				return nil, false, e
			}
			if yes {
				stack = append(append([]ast.ASTHandle(nil), nested...), stack...)
			} else {
				values = append(values, current)
			}
		}
	}
	return values, changed, nil
}

// monomial is get_power_product(t, a): coefficient receives a (one when t
// is not (* a p) with a numeral a) and the power product is returned.
func (r *ArithRewriter) monomial(h ast.ASTHandle, coefficient *unumeral.Rational) (ast.ASTHandle, error) {
	_, args, matched, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if matched && len(args) == 2 {
		_, numeral, e := r.numeral(args[0], coefficient)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if numeral {
			return args[1], nil
		}
	}
	return h, r.q().SetInt32(coefficient, 1)
}

// powerProduct is get_power_product(t): the coefficient is not read.
func (r *ArithRewriter) powerProduct(h ast.ASTHandle) (ast.ASTHandle, error) {
	_, args, matched, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if matched && len(args) == 2 {
		numeral, e := r.isNumeral(args[0])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if numeral {
			return args[1], nil
		}
	}
	return h, nil
}

// arithQOne is a fresh rational(1), for a constant argument.
func arithQOne() *unumeral.Rational {
	return &unumeral.Rational{Num: unumeral.NewInt(1), Den: unumeral.NewInt(1)}
}

func (r *ArithRewriter) coefficientProduct(coefficient *unumeral.Rational, powerProduct, term ast.ASTHandle) (ast.ASTHandle, error) {
	if coefficient.IsOne() {
		return powerProduct, nil
	}
	numeral, err := r.numeralForTerm(coefficient, term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.app(arithOpMul, numeral, powerProduct)
}

func (r *ArithRewriter) negateMonomial(h ast.ASTHandle) (_ ast.ASTHandle, err error) {
	_, factors, product, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if product && len(factors) != 0 {
		// numeral a
		coefficient := unumeral.NewRational()
		defer func() { err = errors.Join(err, arithQDel(r.q(), &coefficient)) }()
		_, numeral, err := r.numeral(factors[0], &coefficient)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if numeral {
			result := make([]ast.ASTHandle, 0, len(factors))
			if !coefficient.IsMinusOne() {
				if err = r.q().Neg(&coefficient); err != nil {
					return ast.ASTHandle{}, err
				}
				negative, err := r.numeralForTerm(&coefficient, h)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				result = append(result, negative)
			}
			result = append(result, factors[1:]...)
			return r.makeMul(result, h)
		}
	}
	minusOne, err := r.numeralForTermInt(-1, h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.makeMul([]ast.ASTHandle{minusOne, h}, h)
}

func (r *ArithRewriter) negativePolynomial(h ast.ASTHandle) (_ ast.ASTHandle, _ bool, err error) {
	// numeral a
	coefficient := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(r.q(), &coefficient)) }()
	_, factors, product, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if product && len(factors) != 0 {
		_, numeral, err := r.numeral(factors[0], &coefficient)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if numeral && coefficient.IsNeg() {
			result, err := r.negateMonomial(h)
			return result, err == nil, err
		}
	}
	_, terms, sum, err := r.operation(h, arithOpAdd)
	if err != nil || !sum || len(terms) == 0 {
		return ast.ASTHandle{}, false, err
	}
	_, firstFactors, firstProduct, err := r.operation(terms[0], arithOpMul)
	if err != nil || !firstProduct || len(firstFactors) == 0 {
		return ast.ASTHandle{}, false, err
	}
	_, numeral, err := r.numeral(firstFactors[0], &coefficient)
	if err != nil || !numeral || !coefficient.IsNeg() {
		return ast.ASTHandle{}, false, err
	}
	negated := make([]ast.ASTHandle, len(terms))
	for i, item := range terms {
		negated[i], err = r.negateMonomial(item)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	result, err := r.makeAdd(negated, h)
	return result, err == nil, err
}

func (r *ArithRewriter) rewriteAdd(args ast.AppArguments, out ast.ASTRef) (_ RewriteStatus, err error) {
	if args.Count() == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	if args.Count() == 1 {
		h, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	values, flattened, err := r.flattenArguments(args, arithOpAdd, false)
	if err != nil {
		return RewriteFailed, err
	}
	first, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	// mk_nflat_add_core: an ordered sum without repeated power products is
	// already normal when it has no numeral, or only a nonzero leading one
	// (BR_FAILED). mk_flat_add_core then builds the flat application.
	if !r.hoistMul && !r.hoistCoeff {
		normal, err := r.addAlreadyNormal(values)
		if err != nil {
			return RewriteFailed, err
		}
		if normal {
			if !flattened {
				return RewriteFailed, nil
			}
			h, err := r.app(arithOpAdd, values...)
			if err != nil {
				return RewriteFailed, err
			}
			return RewriteDone, r.manager.SetRef(out, h)
		}
	}
	q := r.q()
	// numeral c(0); coeffs[pos] for each power product; a: the scratch.
	constant, a := unumeral.NewRational(), unumeral.NewRational()
	order := make([]ast.ASTHandle, 0, len(values))
	var coefficients []unumeral.Rational
	defer func() {
		for i := range coefficients {
			err = errors.Join(err, arithQDel(q, &coefficients[i]))
		}
		err = errors.Join(err, arithQDel2(q, &a, &constant))
	}()
	position := make(map[ast.ASTHandle]int)
	counts := make([]int, 0, len(values))
	for _, value := range values {
		_, numeral, e := r.numeral(value, &a)
		if e != nil {
			return RewriteFailed, e
		}
		if numeral {
			if e = q.Add(&constant, &a, &constant); e != nil {
				return RewriteFailed, e
			}
			continue
		}
		product, e := r.monomial(value, &a)
		if e != nil {
			return RewriteFailed, e
		}
		pos, seen := position[product]
		if !seen {
			pos = len(order)
			position[product] = pos
			order = append(order, product)
			coefficients = append(coefficients, unumeral.NewRational())
			counts = append(counts, 0)
		}
		counts[pos]++
		if e = q.Add(&coefficients[pos], &a, &coefficients[pos]); e != nil {
			return RewriteFailed, e
		}
	}
	resultArgs := make([]ast.ASTHandle, 0, len(order)+1)
	if !constant.IsZero() {
		h, e := r.numeralForTerm(&constant, first)
		if e != nil {
			return RewriteFailed, e
		}
		resultArgs = append(resultArgs, h)
	}
	duplicate := false
	for pos, product := range order {
		duplicate = duplicate || counts[pos] > 1
		if coefficients[pos].IsZero() {
			continue
		}
		h, e := r.coefficientProduct(&coefficients[pos], product, first)
		if e != nil {
			return RewriteFailed, e
		}
		resultArgs = append(resultArgs, h)
	}
	if r.sortSums && len(resultArgs) > 1 {
		start := 0
		if !constant.IsZero() {
			start = 1
		}
		var cause error
		sort.SliceStable(resultArgs[start:], func(i, j int) bool {
			less, e := r.monomialLess(resultArgs[start+i], resultArgs[start+j])
			if cause == nil {
				cause = e
			}
			return e == nil && less
		})
		if cause != nil {
			return RewriteFailed, cause
		}
	}
	coefficientHoisted := false
	resultArgs, coefficientHoisted, err = r.hoistEqualCoefficients(resultArgs, first)
	if err != nil {
		return RewriteFailed, err
	}
	result, err := r.makeAdd(resultArgs, first)
	if err != nil {
		return RewriteFailed, err
	}
	if hoistedSum, hoisted, e := r.hoistCommonMultiplication(result); e != nil {
		return RewriteFailed, e
	} else if hoisted {
		return RewriteFull, r.manager.SetRef(out, hoistedSum)
	}
	same, err := r.sameArguments(args, resultArgs)
	if err != nil {
		return RewriteFailed, err
	}
	if !flattened && !duplicate && !coefficientHoisted && same {
		return RewriteFailed, nil
	}
	return RewriteDone, r.manager.SetRef(out, result)
}

func (r *ArithRewriter) rewriteSub(args ast.AppArguments, out ast.ASTRef) (status RewriteStatus, result error) {
	if args.Count() == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	first, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	if args.Count() == 1 {
		return RewriteDone, r.manager.SetRef(out, first)
	}
	// expr_ref minus_one: held while the result is built, released on
	// return.
	minusOne, err := r.numeralForTermInt(-1, first)
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.IncRef(minusOne); err != nil {
		return RewriteFailed, err
	}
	defer func() { result = errors.Join(result, r.manager.DecRef(minusOne)) }()
	values := []ast.ASTHandle{first}
	for i := uint32(1); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return RewriteFailed, e
		}
		// is_zero(args[i])
		if zero, e := r.zero(h); e != nil {
			return RewriteFailed, e
		} else if zero {
			continue
		}
		negative, e := r.app(arithOpMul, minusOne, h)
		if e != nil {
			return RewriteFailed, e
		}
		values = append(values, negative)
	}
	sum, err := r.makeAdd(values, first)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, r.manager.SetRef(out, sum)
}

func (r *ArithRewriter) monomials(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	_, args, add, err := r.operation(h, arithOpAdd)
	if err != nil {
		return nil, err
	}
	if add {
		return args, nil
	}
	return []ast.ASTHandle{h}, nil
}

func (r *ArithRewriter) cancelMonomials(left, right ast.ASTHandle, move bool) (_, _ ast.ASTHandle, _ bool, err error) {
	sides := [2][]ast.ASTHandle{}
	sides[0], err = r.monomials(left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	sides[1], err = r.monomials(right)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	q := r.q()
	// numeral c(0); a: the scratch; coeffs of the shared power products.
	constant, a := unumeral.NewRational(), unumeral.NewRational()
	var sharedValues []unumeral.Rational
	defer func() {
		for i := range sharedValues {
			err = errors.Join(err, arithQDel(q, &sharedValues[i]))
		}
		err = errors.Join(err, arithQDel2(q, &a, &constant))
	}()
	constantCount := 0
	leftProducts := make(map[ast.ASTHandle]bool)
	shared := make(map[ast.ASTHandle]bool)
	for _, term := range sides[0] {
		_, numeral, e := r.numeral(term, &a)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if numeral {
			if e = q.Add(&constant, &a, &constant); e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
			constantCount++
			continue
		}
		product, e := r.powerProduct(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		leftProducts[product] = true
	}
	if move && constantCount == 0 {
		if numeral, e := r.isNumeral(right); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		} else if numeral {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
		}
	}
	for _, term := range sides[1] {
		_, numeral, e := r.numeral(term, &a)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if numeral {
			if e = q.Sub(&constant, &a, &constant); e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
			constantCount++
			continue
		}
		product, e := r.powerProduct(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if leftProducts[product] {
			shared[product] = true
		}
	}
	if len(shared) == 0 && constantCount <= 1 {
		rightNumeral, e := r.isNumeral(right)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if move && rightNumeral || !move && (constantCount == 0 || rightNumeral) {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
		}
	}
	sharedCoefficients := make(map[ast.ASTHandle]int)
	for side := range sides {
		for _, term := range sides[side] {
			if numeral, e := r.isNumeral(term); e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, e
			} else if numeral {
				continue
			}
			product, e := r.monomial(term, &a)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
			if !shared[product] {
				continue
			}
			pos, seen := sharedCoefficients[product]
			if !seen {
				pos = len(sharedValues)
				sharedCoefficients[product] = pos
				sharedValues = append(sharedValues, unumeral.NewRational())
			}
			if side == 0 {
				e = q.Add(&sharedValues[pos], &a, &sharedValues[pos])
			} else {
				e = q.Sub(&sharedValues[pos], &a, &sharedValues[pos])
			}
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
		}
	}
	lhs := make([]ast.ASTHandle, 0, len(sides[0])+len(sides[1]))
	rhs := make([]ast.ASTHandle, 0, len(sides[1]))
	seen := make(map[ast.ASTHandle]bool)
	for _, term := range sides[0] {
		if numeral, e := r.isNumeral(term); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		} else if numeral {
			continue
		}
		product, e := r.powerProduct(term)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if !shared[product] {
			lhs = append(lhs, term)
			continue
		}
		if seen[product] {
			continue
		}
		seen[product] = true
		coefficient := &sharedValues[sharedCoefficients[product]]
		if !coefficient.IsZero() {
			h, e := r.coefficientProduct(coefficient, product, left)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
			lhs = append(lhs, h)
		}
	}
	for _, term := range sides[1] {
		if numeral, e := r.isNumeral(term); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		} else if numeral {
			continue
		}
		product, e := r.monomial(term, &a)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if shared[product] {
			continue
		}
		if !move {
			rhs = append(rhs, term)
			continue
		}
		if e = q.Neg(&a); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		h, e := r.coefficientProduct(&a, product, left)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		lhs = append(lhs, h)
	}
	if move && r.sortSums && len(lhs) > 1 {
		// std::sort(new_lhs_monomials.begin() + 1, ..., mon_lt): the
		// coefficient slot is filled afterwards.
		var cause error
		sort.SliceStable(lhs, func(i, j int) bool {
			less, e := r.monomialLess(lhs[i], lhs[j])
			if cause == nil {
				cause = e
			}
			return e == nil && less
		})
		if cause != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, cause
		}
	}
	constantAtRight := move || len(rhs) == 0 || len(lhs) != 0
	if constantAtRight {
		if err = q.Neg(&constant); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
	}
	if !constant.IsZero() || constantAtRight && len(rhs) == 0 || !constantAtRight && len(lhs) == 0 {
		h, e := r.numeralForTerm(&constant, left)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if constantAtRight {
			rhs = append([]ast.ASTHandle{h}, rhs...)
		} else {
			lhs = append([]ast.ASTHandle{h}, lhs...)
		}
	}
	newLeft, err := r.makeAdd(lhs, left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	newRight, err := r.makeAdd(rhs, left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return newLeft, newRight, newLeft != left || newRight != right, nil
}

func invertArithmeticRelation(kind int32) int32 {
	if kind == arithOpLE {
		return arithOpGE
	}
	if kind == arithOpGE {
		return arithOpLE
	}
	return kind
}

func (r *ArithRewriter) rewriteBound(left, right ast.ASTHandle, kind int32, out ast.ASTRef) (_ bool, err error) {
	_, _, add, err := r.operation(left, arithOpAdd)
	if err != nil {
		return false, err
	}
	_, _, mod, err := r.operation(left, arithOpMod)
	if err != nil {
		return false, err
	}
	q := r.q()
	// numeral b, c
	bound, coefficient := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(q, &coefficient, &bound)) }()
	_, rightNumeral, err := r.numeral(right, &bound)
	if err != nil {
		return false, err
	}
	if !add && !mod && rightNumeral {
		product, e := r.monomial(left, &coefficient)
		if e != nil {
			return false, e
		}
		relation := kind
		changed := false
		if coefficient.IsNeg() {
			if e = q.Neg(&coefficient); e == nil {
				e = q.Neg(&bound)
			}
			if e != nil {
				return false, e
			}
			relation = invertArithmeticRelation(relation)
			changed = true
		}
		if !coefficient.IsOne() {
			if e = q.Div(&bound, &coefficient, &bound); e != nil {
				return false, e
			}
			changed = true
		}
		if changed {
			integer, e := r.integerTerm(left)
			if e != nil {
				return false, e
			}
			if integer && !bound.IsInt() {
				switch relation {
				case arithOpLE:
					e = q.Floor(&bound, &bound)
				case arithOpGE:
					e = q.Ceil(&bound, &bound)
				default:
					return true, r.setBool(out, false)
				}
				if e != nil {
					return false, e
				}
			}
			value, e := r.makeNumeral(&bound, integer)
			if e != nil {
				return false, e
			}
			var result ast.ASTHandle
			if relation == arithRelationEq {
				result, e = r.manager.MkEq(product, value)
			} else {
				result, e = r.app(relation, product, value)
			}
			if e != nil {
				return false, e
			}
			return true, r.manager.SetRef(out, result)
		}
	}
	if !rightNumeral {
		return false, nil
	}
	_, modArgs, mod, err := r.operation(left, arithOpMod)
	if err != nil || !mod || len(modArgs) != 2 {
		return false, err
	}
	// coefficient now holds the modulus
	integer, numeral, err := r.numeral(modArgs[1], &coefficient)
	if err != nil || !numeral || !integer || coefficient.IsZero() {
		return false, err
	}
	truth, decided := false, false
	if bound.IsNeg() {
		decided = true
		truth = kind == arithOpGE
	} else if bound.IsZero() && kind == arithOpGE {
		decided, truth = true, true
	} else {
		if err = q.Abs(&coefficient); err != nil {
			return false, err
		}
		cmp, e := q.Compare(&bound, &coefficient)
		if e != nil {
			return false, e
		}
		if bound.IsPos() && cmp >= 0 {
			decided = true
			truth = kind == arithOpLE
		} else if kind == arithOpLE {
			// b + 1 == abs(n)
			if e = q.Inc(&bound); e != nil {
				return false, e
			}
			if same, e := q.Equal(&bound, &coefficient); e != nil {
				return false, e
			} else if same {
				decided, truth = true, true
			}
		}
	}
	if !decided {
		return false, nil
	}
	return true, r.setBool(out, truth)
}

func (r *ArithRewriter) zero(h ast.ASTHandle) (bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	return r.manager.ArithmeticNumeralIsZero(h)
}

func (r *ArithRewriter) reducePowerFactors(h ast.ASTHandle, equality bool) (_ ast.ASTHandle, _ bool, err error) {
	_, factors, product, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !product {
		factors = []ast.ASTHandle{h}
	}
	q := r.q()
	// rational k
	exponent := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(q, &exponent)) }()
	changed := false
	result := make([]ast.ASTHandle, len(factors))
	for i, factor := range factors {
		result[i] = factor
		_, powerArgs, power, e := r.operation(factor, arithOpPower)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if !power || len(powerArgs) != 2 {
			continue
		}
		_, numeral, e := r.numeral(powerArgs[1], &exponent)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		threshold := int64(2)
		if equality {
			threshold = 1
		}
		if !numeral || !exponent.IsInt() {
			continue
		}
		small, e := q.IsInt64(&exponent)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if !small {
			continue
		}
		k, e := q.Int64(&exponent)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if k <= threshold {
			continue
		}
		changed = true
		if equality || k%2 != 0 {
			result[i] = powerArgs[0]
			continue
		}
		two, e := r.numeralForTermInt(2, factor)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		result[i], e = r.app(arithOpPower, powerArgs[0], two)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
	}
	if !changed {
		return h, false, nil
	}
	reduced, err := r.makeMul(result, h)
	return reduced, true, err
}

func (r *ArithRewriter) reduceComparisonPowers(left, right ast.ASTHandle, equality bool) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	leftZero, err := r.zero(left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	rightZero, err := r.zero(right)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if !leftZero && !rightZero {
		return left, right, false, nil
	}
	newLeft, leftChanged, err := r.reducePowerFactors(left, equality)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	newRight, rightChanged, err := r.reducePowerFactors(right, equality)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return newLeft, newRight, leftChanged || rightChanged, nil
}

func (r *ArithRewriter) eliminateToRealVariable(h ast.ASTHandle) (_ ast.ASTHandle, _ bool, err error) {
	// rational r
	value := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(r.q(), &value)) }()
	_, numeral, err := r.numeral(h, &value)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if numeral {
		if !value.IsInt() {
			return ast.ASTHandle{}, false, nil
		}
		result, e := r.makeNumeral(&value, true)
		return result, e == nil, e
	}
	if yes, inner, e := r.unary(h, arithOpToReal); e != nil {
		return ast.ASTHandle{}, false, e
	} else if yes {
		return inner, true, nil
	}
	return ast.ASTHandle{}, false, nil
}

func (r *ArithRewriter) eliminateToRealMonomial(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	_, factors, product, err := r.operation(h, arithOpMul)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !product {
		return r.eliminateToRealVariable(h)
	}
	converted := make([]ast.ASTHandle, len(factors))
	for i, factor := range factors {
		var ok bool
		converted[i], ok, err = r.eliminateToRealVariable(factor)
		if err != nil || !ok {
			// expr_ref_buffer new_vars releases the factors converted so
			// far (AUDIT.md FUZZ_A-F8).
			return ast.ASTHandle{}, false, errors.Join(err, r.discardConverted(converted[:i]))
		}
	}
	result, err := r.makeMul(converted, converted[0])
	return result, err == nil, err
}

func (r *ArithRewriter) eliminateToRealPolynomial(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	_, monomials, sum, err := r.operation(h, arithOpAdd)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !sum {
		return r.eliminateToRealMonomial(h)
	}
	converted := make([]ast.ASTHandle, len(monomials))
	for i, monomial := range monomials {
		var ok bool
		converted[i], ok, err = r.eliminateToRealMonomial(monomial)
		if err != nil || !ok {
			// expr_ref_buffer new_monomials releases the monomials
			// converted so far (AUDIT.md FUZZ_A-F8).
			return ast.ASTHandle{}, false, errors.Join(err, r.discardConverted(converted[:i]))
		}
	}
	result, err := r.makeAdd(converted, converted[0])
	return result, err == nil, err
}

// discardConverted is the destructor of an expr_ref_buffer holding hs:
// each entry loses the buffer's reference, front to back.
func (r *ArithRewriter) discardConverted(hs []ast.ASTHandle) (err error) {
	refs := make([]ast.ASTRef, 0, len(hs))
	for _, h := range hs {
		ref, e := r.manager.NewRef(h)
		if e != nil {
			err = errors.Join(err, e)
			continue
		}
		refs = append(refs, ref)
	}
	for i := range refs {
		err = errors.Join(err, r.manager.DelRef(&refs[i]))
	}
	return err
}

func (r *ArithRewriter) eliminateToRealComparison(left, right ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	integer, err := r.integerTerm(left)
	if err != nil || integer {
		return left, right, false, err
	}
	newLeft, leftOK, err := r.eliminateToRealPolynomial(left)
	if err != nil || !leftOK {
		return left, right, false, err
	}
	newRight, rightOK, err := r.eliminateToRealPolynomial(right)
	if err != nil || !rightOK {
		// new_new_arg1 keeps the converted left side until
		// mk_le_ge_eq_core returns; the caller holds it (AUDIT.md FUZZ_A-F8).
		return newLeft, right, false, err
	}
	return newLeft, newRight, true, nil
}

// polynomialCoefficientGCD is get_gcd's accumulation over h's monomials:
// gcd holds the gcd of the absolute integer coefficients seen so far.
func (r *ArithRewriter) polynomialCoefficientGCD(h ast.ASTHandle, gcd *unumeral.Rational, first *bool, constants *int) (err error) {
	terms, err := r.monomials(h)
	if err != nil {
		return err
	}
	q := r.q()
	// numeral a
	value := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(q, &value)) }()
	for _, term := range terms {
		_, numeral, e := r.numeral(term, &value)
		if e != nil {
			return e
		}
		if numeral {
			if !value.IsZero() {
				*constants++
			}
			continue
		}
		if _, e = r.monomial(term, &value); e != nil {
			return e
		}
		if !value.IsInt() {
			return ast.ErrASTDomain
		}
		if e = q.Abs(&value); e != nil {
			return e
		}
		if *first {
			e = q.Copy(gcd, &value)
			*first = false
		} else {
			e = q.GCD(gcd, &value, gcd)
		}
		if e != nil {
			return e
		}
		if gcd.IsOne() {
			return nil
		}
	}
	return nil
}

func (r *ArithRewriter) dividePolynomial(h ast.ASTHandle, divisor *unumeral.Rational, treatment int) (_ ast.ASTHandle, _ bool, err error) {
	terms, err := r.monomials(h)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	q := r.q()
	// numeral a
	value := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(q, &value)) }()
	result := make([]ast.ASTHandle, 0, len(terms))
	for _, term := range terms {
		_, numeral, e := r.numeral(term, &value)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if numeral {
			if e = q.Div(&value, divisor, &value); e != nil {
				return ast.ASTHandle{}, false, e
			}
			if !value.IsInt() {
				switch treatment {
				case arithDivideFloor:
					e = q.Floor(&value, &value)
				case arithDivideCeil:
					e = q.Ceil(&value, &value)
				default:
					return ast.ASTHandle{}, false, nil
				}
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
			}
			if !value.IsZero() {
				n, e := r.numeralForTerm(&value, h)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				result = append(result, n)
			}
			continue
		}
		product, e := r.monomial(term, &value)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if e = q.Div(&value, divisor, &value); e != nil {
			return ast.ASTHandle{}, false, e
		}
		if !value.IsInt() {
			return ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		if !value.IsZero() {
			monomial, e := r.coefficientProduct(&value, product, h)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			result = append(result, monomial)
		}
	}
	quotient, err := r.makeAdd(result, h)
	return quotient, err == nil, err
}

// setLeft is the caller's new_arg1 assignment: div_polynomial(arg1, ...)
// writes new_arg1, releasing its previous value, before arg2 is divided.
func (r *ArithRewriter) reducePolynomialGCD(left, right ast.ASTHandle, relation int32, setLeft func(ast.ASTHandle) error) (_, _ ast.ASTHandle, _, _ bool, err error) {
	integer, err := r.integerTerm(left)
	if err != nil || !integer {
		return left, right, false, false, err
	}
	// rational g
	gcd := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(r.q(), &gcd)) }()
	first, constants := true, 0
	if err = r.polynomialCoefficientGCD(left, &gcd, &first, &constants); err != nil {
		return left, right, false, false, err
	}
	if (first || !gcd.IsOne()) && constants <= 1 {
		if err = r.polynomialCoefficientGCD(right, &gcd, &first, &constants); err != nil {
			return left, right, false, false, err
		}
	}
	if first || gcd.IsOne() || constants > 1 {
		return left, right, false, false, nil
	}
	leftTreatment, rightTreatment := arithDivideExact, arithDivideExact
	if relation == arithOpLE {
		leftTreatment, rightTreatment = arithDivideCeil, arithDivideFloor
	} else if relation == arithOpGE {
		leftTreatment, rightTreatment = arithDivideFloor, arithDivideCeil
	}
	newLeft, possible, err := r.dividePolynomial(left, &gcd, leftTreatment)
	if err != nil || !possible {
		return left, right, false, !possible, err
	}
	// new_arg1 is an expr_ref local of mk_le_ge_eq_core: it holds the
	// quotient from here, and dies when the rewrite returns.
	if err = setLeft(newLeft); err != nil {
		return left, right, false, false, err
	}
	newRight, possible, err := r.dividePolynomial(right, &gcd, rightTreatment)
	if err != nil || !possible {
		return left, right, false, !possible, err
	}
	return newLeft, newRight, true, false, nil
}

// addAlreadyNormal is mk_nflat_add_core's !has_multiple && ordered test
// followed by its two BR_FAILED cases.
func (r *ArithRewriter) addAlreadyNormal(values []ast.ASTHandle) (bool, error) {
	numCoeffs := 0
	firstNonzeroNumeral := false
	ordered := true
	var prev ast.ASTHandle
	seen := make(map[ast.ASTHandle]bool, len(values))
	for i, value := range values {
		numeral, err := r.isNumeral(value)
		if err != nil {
			return false, err
		}
		product := value
		if numeral {
			numCoeffs++
			ordered = !r.sortSums || i == 0
			if i == 0 {
				// is_numeral(arg, a) && !a.is_zero()
				zero, err := r.zero(value)
				if err != nil {
					return false, err
				}
				firstNonzeroNumeral = !zero
			}
		} else {
			if r.sortSums && ordered {
				if !prev.IsNull() {
					less, err := r.monomialLess(value, prev)
					if err != nil {
						return false, err
					}
					if less {
						ordered = false
					}
				}
				prev = value
			}
			pp, err := r.powerProduct(value)
			if err != nil {
				return false, err
			}
			product = pp
		}
		if seen[product] {
			return false, nil // has_multiple
		}
		seen[product] = true
	}
	if !ordered {
		return false, nil
	}
	return numCoeffs == 0 || numCoeffs == 1 && firstNonzeroNumeral, nil
}
