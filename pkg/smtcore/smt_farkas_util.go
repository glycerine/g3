package smtcore

// Source: src/smt/smt_farkas_util.cpp and src/smt/smt_farkas_util.h.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// SMTFarkasUtil combines arithmetic premises using Farkas coefficients. The
// parameter services are borrowed (extract_consequence builds its own
// th_rewriter from them); the utility owns each premise retained by Add until
// Reset or Close.
type SMTFarkasUtil struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	// q is the AST manager's rational manager; coefficients and
	// normalizeFactor are its rationals (vector<rational> m_coeffs,
	// rational m_normalize_factor).
	q               *numeral.RationalManager
	inequalities    []ast.ASTHandle
	coefficients    []numeral.Rational
	normalizeFactor numeral.Rational
	splitLiterals   bool
	closed          bool
}

func NewSMTFarkasUtil(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (result *SMTFarkasUtil, err error) {
	if m == nil || context == nil || globals == nil || context.Symbols() != m.Symbols || globals.Context != context {
		return nil, ast.ErrASTDomain
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	numerals := m.NumeralService()
	if numerals == nil {
		return nil, ast.ErrASTHandle
	}
	result = &SMTFarkasUtil{manager: m, family: family, context: context, globals: globals, q: numerals.Manager, normalizeFactor: smtQInt(1)}
	return result, nil
}

// newExtractRewriter is extract_consequence's th_rewriter rw(m) with
// gcd_rounding set.
func (f *SMTFarkasUtil) newExtractRewriter() (_ *rewriter.TheoryRewriter, err error) {
	local, err := f.context.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.context.Del(&local)) }()
	name := "gcd_rounding"
	if err = f.context.SetBoolName(&local, &name, true); err != nil {
		return nil, err
	}
	rw, err := rewriter.NewTheoryRewriter(f.manager, f.context, f.globals, &local)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return nil, err
	}
	return rw, nil
}

func (f *SMTFarkasUtil) open() error {
	if f == nil || f.closed || f.manager == nil || f.context == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (f *SMTFarkasUtil) Close() (result error) {
	if f == nil {
		return ast.ErrASTHandle
	}
	if f.closed {
		return nil
	}
	if result = f.Reset(); result != nil {
		return result
	}
	if result = smtQDel(f.q, &f.normalizeFactor); result != nil {
		return result
	}
	f.closed = true
	return nil
}

func (f *SMTFarkasUtil) Reset() (result error) {
	if err := f.open(); err != nil {
		return err
	}
	for len(f.inequalities) != 0 {
		index := len(f.inequalities) - 1
		consumed, err := f.manager.DecRefConsumed(f.inequalities[index])
		if consumed {
			f.inequalities = f.inequalities[:index]
			if e := smtQDel(f.q, &f.coefficients[index]); e != nil && err == nil {
				err = e
			}
			f.coefficients = f.coefficients[:index]
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	return nil
}

func (f *SMTFarkasUtil) SetSplitLiterals(enabled bool) error {
	if err := f.open(); err != nil {
		return err
	}
	f.splitLiterals = enabled
	return nil
}

// NormalizeFactor is get_normalize_factor(): the factor, owned by the
// utility, stays valid until the next Get or Close.
func (f *SMTFarkasUtil) NormalizeFactor() (*numeral.Rational, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	return &f.normalizeFactor, nil
}

func (f *SMTFarkasUtil) operation(expression ast.ASTHandle) (int32, ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, err := f.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	family, err := f.manager.GetFamilyID(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	op, err := f.manager.GetDeclKind(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	valid := family == f.family && (op == smtArithOpLE || op == smtArithOpGE || op == smtArithOpLT || op == smtArithOpGT) ||
		family == f.manager.BasicFamilyID() && op == ast.ASTOpEq
	if !valid {
		return op, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	count, err := f.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return op, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	left, err := f.manager.GetArg(expression, 0)
	if err != nil {
		return op, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err := f.manager.GetArg(expression, 1)
	return op, left, right, err == nil, err
}

func (f *SMTFarkasUtil) isInteger(expression ast.ASTHandle) (bool, error) {
	_, left, _, matched, err := f.operation(expression)
	if err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return false, err
	}
	sort, err := f.manager.GetSort(left)
	if err != nil {
		return false, err
	}
	family, err := f.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	kind, err := f.manager.GetDeclKind(sort)
	if err != nil {
		return false, err
	}
	if family != f.family || kind != 0 && kind != 1 {
		return false, ast.ErrASTDomain
	}
	return kind == 1, nil
}

func (f *SMTFarkasUtil) app(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return f.manager.MkFamilyApp(f.family, op, nil, args)
}

func (f *SMTFarkasUtil) one() (ast.ASTHandle, error) {
	one := smtQInt(1)
	return f.manager.ArithmeticNumeralFromRational(&one, true)
}

func (f *SMTFarkasUtil) fixSign(positive bool, constraint ast.ASTHandle) (ast.ASTHandle, error) {
	op, left, right, matched, err := f.operation(constraint)
	if err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	integer, err := f.isInteger(constraint)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := f.manager.GetFamilyID(constraint)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family == f.manager.BasicFamilyID() {
		if positive {
			return constraint, nil
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if integer && positive && (op == smtArithOpLT || op == smtArithOpGT) {
		if op == smtArithOpGT {
			left, right = right, left
		}
		one, err := f.one()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		left, err = f.app(smtArithOpAdd, left, one)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return f.app(smtArithOpLE, left, right)
	}
	if integer && !positive && (op == smtArithOpLE || op == smtArithOpGE) {
		if op == smtArithOpGE {
			left, right = right, left
		}
		one, err := f.one()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		right, err = f.app(smtArithOpAdd, right, one)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return f.app(smtArithOpGE, left, right)
	}
	if positive {
		return constraint, nil
	}
	complement := map[int32]int32{smtArithOpLE: smtArithOpGT, smtArithOpLT: smtArithOpGE, smtArithOpGE: smtArithOpLT, smtArithOpGT: smtArithOpLE}
	result, found := complement[op]
	if !found {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return f.app(result, left, right)
}

// AddBigRat is Add for a coefficient held as a big.Rat (the muz learners'
// boundary until they hold rationals).
func (f *SMTFarkasUtil) AddBigRat(coefficient *big.Rat, constraint ast.ASTHandle) (err error) {
	if err := f.open(); err != nil || coefficient == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	c := smtQZero()
	if err = f.q.SetBigRat(&c, coefficient); err == nil {
		err = f.Add(&c, constraint)
	}
	if e := smtQDel(f.q, &c); e != nil {
		return errors.Join(err, e)
	}
	return err
}

// Add is add(coef, c); the utility keeps a copy of coefficient.
func (f *SMTFarkasUtil) Add(coefficient *numeral.Rational, constraint ast.ASTHandle) error {
	if err := f.open(); err != nil || coefficient == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	positive := true
	for {
		negated, child, err := f.manager.IsNot(constraint)
		if err != nil {
			return err
		}
		if !negated {
			break
		}
		positive = !positive
		constraint = child
	}
	zero, err := f.q.IsZero(coefficient)
	if err != nil {
		return err
	}
	if zero || f.manager.IsTrue(constraint) {
		return nil
	}
	constraint, err = f.fixSign(positive, constraint)
	if err != nil {
		return err
	}
	c, err := smtQNew(f.q, coefficient)
	if err != nil {
		return err
	}
	if err = f.manager.IncRef(constraint); err != nil {
		return errors.Join(err, smtQDel(f.q, &c))
	}
	f.inequalities = append(f.inequalities, constraint)
	f.coefficients = append(f.coefficients, c)
	return nil
}

// normalizeCoefficients is normalize_coeffs: l is the lcm of the
// coefficients' denominators; each coefficient is multiplied by l unless l
// is one.
func (f *SMTFarkasUtil) normalizeCoefficients() (err error) {
	q := f.q
	l, d := smtQInt(1), smtQZero()
	defer func() { err = errors.Join(err, smtQDel(q, &l), smtQDel(q, &d)) }()
	for i := range f.coefficients {
		if err = q.DenominatorRational(&f.coefficients[i], &d); err != nil {
			return err
		}
		if err = q.LCM(&l, &d, &l); err != nil {
			return err
		}
	}
	if err = q.Copy(&f.normalizeFactor, &l); err != nil {
		return err
	}
	one, err := q.IsOne(&l)
	if err != nil || one {
		return err
	}
	for i := range f.coefficients {
		if err = q.Mul(&f.coefficients[i], &l, &f.coefficients[i]); err != nil {
			return err
		}
	}
	return nil
}

func (f *SMTFarkasUtil) addMultiple(sum ast.ASTHandle, coefficient *numeral.Rational, term ast.ASTHandle) (ast.ASTHandle, error) {
	part := term
	one, err := f.q.IsOne(coefficient)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !one {
		integer, err := f.termIsInteger(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		isInt, err := f.q.IsInt(coefficient)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		numeral, err := f.manager.ArithmeticNumeralFromRational(coefficient, isInt && integer)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		part, err = f.app(smtArithOpMul, numeral, term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return f.app(smtArithOpAdd, sum, part)
}

func (f *SMTFarkasUtil) termIsInteger(term ast.ASTHandle) (bool, error) {
	sort, err := f.manager.GetSort(term)
	if err != nil {
		return false, err
	}
	family, err := f.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	kind, err := f.manager.GetDeclKind(sort)
	return family == f.family && kind == 1, err
}

func (f *SMTFarkasUtil) extractConsequence(low, high int, out ast.ASTRef) (err error) {
	integer, err := f.isInteger(f.inequalities[0])
	if err != nil {
		return err
	}
	zeroValue, negated := smtQZero(), smtQZero()
	defer func() { err = errors.Join(err, smtQDel(f.q, &negated)) }()
	zero, err := f.manager.ArithmeticNumeralFromRational(&zeroValue, integer)
	if err != nil {
		return err
	}
	sum := zero
	strict, equalityOnly := false, true
	for index := low; index < high; index++ {
		op, left, right, matched, err := f.operation(f.inequalities[index])
		if err != nil || !matched {
			if err == nil {
				err = ast.ErrASTDomain
			}
			return err
		}
		if op == smtArithOpGE || op == smtArithOpGT {
			left, right = right, left
		}
		coefficient := &f.coefficients[index]
		sum, err = f.addMultiple(sum, coefficient, left)
		if err != nil {
			return err
		}
		if err = f.q.Copy(&negated, coefficient); err != nil {
			return err
		}
		if err = f.q.Neg(&negated); err != nil {
			return err
		}
		sum, err = f.addMultiple(sum, &negated, right)
		if err != nil {
			return err
		}
		family, err := f.manager.GetFamilyID(f.inequalities[index])
		if err != nil {
			return err
		}
		if family != f.manager.BasicFamilyID() {
			equalityOnly = false
		}
		if op == smtArithOpLT || op == smtArithOpGT {
			strict = true
		}
	}
	integer, err = f.termIsInteger(sum)
	if err != nil {
		return err
	}
	zero, err = f.manager.ArithmeticNumeralFromRational(&zeroValue, integer)
	if err != nil {
		return err
	}
	var consequence ast.ASTHandle
	if equalityOnly {
		consequence, err = f.manager.MkEq(sum, zero)
	} else if strict {
		consequence, err = f.app(smtArithOpLT, sum, zero)
	} else {
		consequence, err = f.app(smtArithOpLE, sum, zero)
	}
	if err != nil {
		return err
	}
	consequence, err = f.manager.MkNot(consequence)
	if err != nil {
		return err
	}
	// The locals die in reverse: pr, then rw, then res.
	res, err := f.manager.NewRef(consequence)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.manager.DelRef(&res)) }()
	rw, err := f.newExtractRewriter()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	pr, err := f.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.manager.DelRef(&pr)) }()
	temporary, err := f.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.manager.DelRef(&temporary)) }()
	// rw(res, result, pr): pr stays null when proofs are disabled.
	proofs, err := f.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		err = rw.RewriteWithProof(consequence, temporary, pr)
	} else {
		err = rw.Rewrite(consequence, temporary)
	}
	if err != nil {
		return err
	}
	rewritten, err := f.manager.RefGet(temporary)
	if err != nil {
		return err
	}
	rewritten, err = f.fixDifferenceLogic(rewritten)
	if err != nil {
		return err
	}
	return f.manager.SetRef(out, rewritten)
}

func (f *SMTFarkasUtil) fixDifferenceLogic(expression ast.ASTHandle) (ast.ASTHandle, error) {
	negated, child, err := f.manager.IsNot(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if negated {
		child, err = f.fixDifferenceLogic(child)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return f.manager.MkNot(child)
	}
	_, left, right, matched, err := f.operation(expression)
	if err != nil || !matched {
		return expression, err
	}
	family, err := f.manager.GetFamilyID(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := f.manager.GetDeclKind(left)
	if err != nil || family != f.family || op != smtArithOpAdd {
		return expression, err
	}
	count, err := f.manager.GetNumArgs(left)
	if err != nil || count != 2 {
		return expression, err
	}
	first, err := f.manager.GetArg(left, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstFamily, err := f.manager.GetFamilyID(first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstOp, err := f.manager.GetDeclKind(first)
	if err != nil || firstFamily != f.family || firstOp != smtArithOpMul {
		return expression, err
	}
	second, err := f.manager.GetArg(left, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err = f.app(smtArithOpAdd, second, first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := f.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return f.manager.MkApp(declaration, []ast.ASTHandle{left, right})
}

func (f *SMTFarkasUtil) processTerm(expression ast.ASTHandle, parent, size map[uint32]uint32) (uint32, error) {
	root, err := f.manager.GetASTID(expression)
	if err != nil {
		return 0, err
	}
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{expression}
	for len(stack) != 0 {
		last := len(stack) - 1
		term := stack[last]
		stack = stack[:last]
		if seen[term] {
			continue
		}
		seen[term] = true
		kind, err := f.manager.GetKind(term)
		if err != nil {
			return 0, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := f.manager.GetFamilyID(term)
		if err != nil {
			return 0, err
		}
		if family == ast.NullFamilyID {
			id, err := f.manager.GetASTID(term)
			if err != nil {
				return 0, err
			}
			f.union(root, id, parent, size)
		}
		count, err := f.manager.GetNumArgs(term)
		if err != nil {
			return 0, err
		}
		for index := uint32(0); index < count; index++ {
			child, err := f.manager.GetArg(term, index)
			if err != nil {
				return 0, err
			}
			stack = append(stack, child)
		}
	}
	return root, nil
}

func (f *SMTFarkasUtil) find(index uint32, parent, size map[uint32]uint32) uint32 {
	if _, found := parent[index]; !found {
		parent[index], size[index] = index, 1
	}
	for parent[index] != index {
		index = parent[index]
	}
	return index
}

func (f *SMTFarkasUtil) union(left, right uint32, parent, size map[uint32]uint32) {
	left, right = f.find(left, parent, size), f.find(right, parent, size)
	if left == right {
		return
	}
	if size[left] > size[right] {
		left, right = right, left
	}
	parent[left] = right
	size[right] += size[left]
}

func (f *SMTFarkasUtil) partition() ([]int, error) {
	parent, size := make(map[uint32]uint32), make(map[uint32]uint32)
	representatives := make([]uint32, len(f.inequalities))
	for index, inequality := range f.inequalities {
		var err error
		representatives[index], err = f.processTerm(inequality, parent, size)
		if err != nil {
			return nil, err
		}
	}
	var ends []int
	for head := 0; head < len(f.inequalities); {
		representative := f.find(representatives[head], parent, size)
		tail := head
		for index := head + 1; index < len(f.inequalities); index++ {
			if f.find(representatives[index], parent, size) != representative {
				continue
			}
			tail++
			if tail != index {
				representatives[tail], representatives[index] = representatives[index], representatives[tail]
				f.inequalities[tail], f.inequalities[index] = f.inequalities[index], f.inequalities[tail]
				f.coefficients[tail], f.coefficients[index] = f.coefficients[index], f.coefficients[tail]
			}
		}
		head = tail + 1
		ends = append(ends, head)
	}
	return ends, nil
}

// Get stores the complement of the weighted premises in out.
// GetPDRConstraint is pdr::farkas_learner::constr::get: it always takes
// the full linear combination first, then rebuilds res as the ast_util
// mk_or of the variable-disjoint partition's consequences (the first res is
// released by that assignment, before lits).
func (f *SMTFarkasUtil) GetPDRConstraint(out ast.ASTRef) (err error) {
	if err := f.open(); err != nil {
		return err
	}
	if err := f.q.SetInt64(&f.normalizeFactor, 1); err != nil {
		return err
	}
	if len(f.coefficients) == 0 {
		value, err := f.manager.MkBoolVal(false)
		if err != nil {
			return err
		}
		return f.manager.SetRef(out, value)
	}
	integer, err := f.isInteger(f.inequalities[0])
	if err != nil {
		return err
	}
	if integer {
		if err = f.normalizeCoefficients(); err != nil {
			return err
		}
	}
	if err = f.extractConsequence(0, len(f.coefficients), out); err != nil {
		return err
	}
	ends, err := f.partition()
	if err != nil {
		return err
	}
	literals := make([]ast.ASTHandle, 0, len(ends))
	refs := make([]ast.ASTRef, 0, len(ends))
	defer func() {
		for index := range refs {
			err = errors.Join(err, f.manager.DelRef(&refs[index]))
		}
	}()
	low := 0
	for _, high := range ends {
		ref, err := f.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		if err = f.extractConsequence(low, high, ref); err != nil {
			return err
		}
		literal, err := f.manager.RefGet(ref)
		if err != nil {
			return err
		}
		literals = append(literals, literal)
		low = high
	}
	disjunction, err := f.manager.MkOrArray(literals)
	if err != nil {
		return err
	}
	if err = f.manager.SetRef(out, disjunction); err != nil {
		return err
	}
	// IF_VERBOSE(2, if (lits.size() > 1) "combined lemma: " << mk_pp(res, m))
	// (AUDIT.md FUZZ-F5; g3trace builds only).
	if diagnostic.VerboseProgress && len(literals) > 1 {
		return progressVerbose(2, "combined lemma: "+smtPPText(f.manager, disjunction)+"\n")
	}
	return nil
}

func (f *SMTFarkasUtil) Get(out ast.ASTRef) (err error) {
	if err := f.open(); err != nil {
		return err
	}
	if err := f.q.SetInt64(&f.normalizeFactor, 1); err != nil {
		return err
	}
	if len(f.coefficients) == 0 {
		value, err := f.manager.MkBoolVal(false)
		if err != nil {
			return err
		}
		return f.manager.SetRef(out, value)
	}
	integer, err := f.isInteger(f.inequalities[0])
	if err != nil {
		return err
	}
	if integer {
		if err = f.normalizeCoefficients(); err != nil {
			return err
		}
	}
	if !f.splitLiterals {
		return f.extractConsequence(0, len(f.coefficients), out)
	}
	ends, err := f.partition()
	if err != nil {
		return err
	}
	literals := make([]ast.ASTHandle, 0, len(ends))
	refs := make([]ast.ASTRef, 0, len(ends))
	defer func() {
		for index := range refs {
			err = errors.Join(err, f.manager.DelRef(&refs[index]))
		}
	}()
	low := 0
	for _, high := range ends {
		ref, err := f.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		if err = f.extractConsequence(low, high, ref); err != nil {
			return err
		}
		literal, err := f.manager.RefGet(ref)
		if err != nil {
			return err
		}
		literals = append(literals, literal)
		low = high
	}
	disjunction, err := f.manager.MkOrArgs(literals)
	if err != nil {
		return err
	}
	if err = f.manager.SetRef(out, disjunction); err != nil {
		return err
	}
	// IF_VERBOSE(2, if (lits.size() > 1) "combined lemma: " << mk_pp(res, m))
	// (AUDIT.md FUZZ-F5; g3trace builds only).
	if diagnostic.VerboseProgress && len(literals) > 1 {
		return progressVerbose(2, "combined lemma: "+smtPPText(f.manager, disjunction)+"\n")
	}
	return nil
}
