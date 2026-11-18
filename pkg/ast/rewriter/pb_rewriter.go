package rewriter

// Source: src/ast/rewriter/pb_rewriter.cpp:21-64,198-293;
// src/ast/rewriter/pb_rewriter.h:24-61;
// src/ast/rewriter/pb_rewriter_def.h:27-292.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type pbTerm struct {
	literal ast.ASTHandle
	coeff   *unumeral.Rational // held by the rewrite's pbNumbers
}

// pbNumbers holds the rationals (numeral = rational, of the AST manager's
// NumeralService().Manager) of one mk_app_core: its coefficient vector, k
// and temporaries. Operations keep the first error.
type pbNumbers struct {
	q      *unumeral.RationalManager
	values []*unumeral.Rational
	err    error
}

func newPBNumbers(m *ast.ASTManager) *pbNumbers {
	return &pbNumbers{q: m.NumeralService().Manager}
}

func (n *pbNumbers) fresh() *unumeral.Rational {
	v := new(unumeral.Rational)
	*v = unumeral.NewRational()
	n.values = append(n.values, v)
	return v
}

func (n *pbNumbers) fail(err error) {
	if n.err == nil && err != nil {
		n.err = err
	}
}

func (n *pbNumbers) release() error {
	err := n.err
	for _, v := range n.values {
		err = errors.Join(err, n.q.Del(v))
	}
	n.values = nil
	return err
}

func (n *pbNumbers) set(dst, src *unumeral.Rational) { n.fail(n.q.Copy(dst, src)) }
func (n *pbNumbers) setInt(dst *unumeral.Rational, v int64) {
	n.fail(n.q.SetInt64(dst, v))
}
func (n *pbNumbers) add(dst, a, b *unumeral.Rational) { n.fail(n.q.Add(a, b, dst)) }
func (n *pbNumbers) sub(dst, a, b *unumeral.Rational) { n.fail(n.q.Sub(a, b, dst)) }
func (n *pbNumbers) mul(dst, a, b *unumeral.Rational) { n.fail(n.q.Mul(a, b, dst)) }
func (n *pbNumbers) div(dst, a, b *unumeral.Rational) { n.fail(n.q.Div(a, b, dst)) }
func (n *pbNumbers) neg(dst *unumeral.Rational)       { n.fail(n.q.Neg(dst)) }
func (n *pbNumbers) cmp(a, b *unumeral.Rational) int {
	c, err := n.q.Compare(a, b)
	n.fail(err)
	return c
}

type pbTruth int8

const (
	pbFalse pbTruth = -1
	pbUndef pbTruth = 0
	pbTrue  pbTruth = 1
)

type PseudoBooleanRewriter struct {
	self    *PseudoBooleanRewriter
	manager *ast.ASTManager
	family  ast.FamilyID
	closed  bool
	// negations is mk_app_core's local pb_ast_rewriter_util::m_refs.
	negations []ast.ASTRef
}

func (r *PseudoBooleanRewriter) traceTags() *diagnostic.TraceTags {
	return r.manager.BorrowMemoryTraceTags()
}

func NewPseudoBooleanRewriter(m *ast.ASTManager) (*PseudoBooleanRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.RegisterPseudoBooleanPlugin()
	if err != nil {
		return nil, err
	}
	r := &PseudoBooleanRewriter{manager: m, family: family}
	r.self = r
	return r, nil
}

func (r *PseudoBooleanRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

// rational copies value into a rational held by n.
func (r *PseudoBooleanRewriter) rational(n *pbNumbers, value *unumeral.OwnedRational) (*unumeral.Rational, error) {
	out := n.fresh()
	return out, n.q.Copy(out, &value.Raw)
}

func (r *PseudoBooleanRewriter) bound(n *pbNumbers, decl ast.ASTHandle) (_ *unumeral.Rational, result error) {
	value, err := r.manager.PseudoBooleanBound(decl)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, r.manager.NumeralService().Del(&value)) }()
	return r.rational(n, &value)
}

func (r *PseudoBooleanRewriter) coefficient(n *pbNumbers, decl ast.ASTHandle, index uint32) (_ *unumeral.Rational, result error) {
	value, err := r.manager.PseudoBooleanCoefficient(decl, index)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, r.manager.NumeralService().Del(&value)) }()
	return r.rational(n, &value)
}

func (r *PseudoBooleanRewriter) negated(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil || family != r.manager.BasicFamilyID() {
		return ast.ASTHandle{}, false, err
	}
	op, err := r.manager.GetDeclKind(h)
	if err != nil || op != ast.ASTOpNot {
		return ast.ASTHandle{}, false, err
	}
	arg, err := r.manager.GetArg(h, 0)
	return arg, err == nil, err
}

func (r *PseudoBooleanRewriter) negate(h ast.ASTHandle) (ast.ASTHandle, error) {
	if r.manager.IsTrue(h) {
		return r.manager.MkFalse()
	}
	if r.manager.IsFalse(h) {
		return r.manager.MkTrue()
	}
	if arg, yes, err := r.negated(h); err != nil || yes {
		return arg, err
	}
	// m_refs.push_back(m.mk_not(e)); return m_refs.back();
	not, err := r.manager.MkNot(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := r.manager.NewRef(not)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r.negations = append(r.negations, ref)
	return not, nil
}

// releaseNegations is ~pb_ast_rewriter_util: m_refs released front to back.
func (r *PseudoBooleanRewriter) releaseNegations() error {
	var err error
	for i := range r.negations {
		err = errors.Join(err, r.manager.DelRef(&r.negations[i]))
	}
	r.negations = r.negations[:0]
	return err
}

func (r *PseudoBooleanRewriter) unique(n *pbNumbers, args []pbTerm, k *unumeral.Rational) ([]pbTerm, error) {
	for i := range args {
		if literal, negated, err := r.negated(args[i].literal); err != nil {
			return nil, err
		} else if negated {
			args[i].literal = literal
			n.sub(k, k, args[i].coeff)
			n.neg(args[i].coeff)
		}
	}
	for i := 0; i < len(args); i++ {
		if r.manager.IsTrue(args[i].literal) {
			n.sub(k, k, args[i].coeff)
			args[i] = args[len(args)-1]
			args = args[:len(args)-1]
			i--
		} else if r.manager.IsFalse(args[i].literal) {
			args[i] = args[len(args)-1]
			args = args[:len(args)-1]
			i--
		}
	}
	ids := make(map[ast.ASTHandle]uint32, len(args))
	for _, arg := range args {
		id, err := r.manager.GetASTID(arg.literal)
		if err != nil {
			return nil, err
		}
		ids[arg.literal] = id
	}
	sort.Slice(args, func(i, j int) bool { return ids[args[i].literal] < ids[args[j].literal] })
	if len(args) != 0 {
		out := args[:1]
		for _, arg := range args[1:] {
			last := &out[len(out)-1]
			if last.literal == arg.literal {
				n.add(last.coeff, last.coeff, arg.coeff)
			} else {
				out = append(out, arg)
			}
		}
		args = out
	}
	out := args[:0]
	for _, arg := range args {
		if !arg.coeff.IsZero() {
			out = append(out, arg)
		}
	}
	return out, n.err
}

func (r *PseudoBooleanRewriter) normalize(n *pbNumbers, args []pbTerm, k *unumeral.Rational, equality bool) ([]pbTerm, pbTruth, error) {
	q := n.q
	sum, c := n.fresh(), n.fresh()
	for i := range args {
		n.set(c, args[i].coeff)
		if c.IsNeg() {
			n.set(args[i].coeff, c)
			n.neg(args[i].coeff)
			literal, err := r.negate(args[i].literal)
			if err != nil {
				return nil, pbUndef, err
			}
			args[i].literal = literal
			n.sub(k, k, c)
		}
		n.add(sum, sum, args[i].coeff)
	}
	if n.err != nil {
		return nil, pbUndef, n.err
	}
	if !equality && !k.IsPos() {
		return args[:0], pbTrue, nil
	}
	if equality && k.IsZero() && len(args) == 0 {
		return args, pbTrue, nil
	}
	if n.cmp(sum, k) < 0 {
		return args[:0], pbFalse, n.err
	}
	if equality && n.cmp(k, sum) == 0 {
		for i := range args {
			n.setInt(args[i].coeff, 1)
		}
		n.setInt(k, int64(len(args)))
		return args, pbUndef, n.err
	}
	// d = lcm of the denominators of k and the coefficients
	lcm, den := n.fresh(), n.fresh()
	n.fail(q.DenominatorRational(k, lcm))
	for _, arg := range args {
		n.fail(q.DenominatorRational(arg.coeff, den))
		n.fail(q.LCM(lcm, den, lcm))
	}
	if n.err != nil {
		return nil, pbUndef, n.err
	}
	if !lcm.IsOne() {
		n.mul(k, k, lcm)
		for i := range args {
			n.mul(args[i].coeff, args[i].coeff, lcm)
		}
	}
	if equality {
		return args, pbUndef, n.err
	}
	n.setInt(sum, 0)
	for i := range args {
		if n.cmp(args[i].coeff, k) > 0 {
			n.set(args[i].coeff, k)
		}
		n.add(sum, sum, args[i].coeff)
	}
	if len(args) == 0 {
		return args, pbFalse, n.err
	}
	if n.cmp(sum, k) == 0 {
		for i := range args {
			n.setInt(args[i].coeff, 1)
		}
		n.setInt(k, int64(len(args)))
	}
	g := n.fresh()
	for _, arg := range args {
		if n.cmp(arg.coeff, k) == 0 {
			continue
		}
		if g.IsZero() {
			n.set(g, arg.coeff)
		} else if !g.IsOne() {
			n.fail(q.GCD(g, arg.coeff, g))
		}
	}
	if n.err != nil {
		return nil, pbUndef, n.err
	}
	if g.IsZero() {
		for i := range args {
			n.setInt(args[i].coeff, 1)
		}
		n.setInt(k, 1)
	} else if !g.IsOne() && g.IsPos() {
		// k_new = div(k, g), rounded up when g does not divide k
		kNew, rem := n.fresh(), n.fresh()
		n.fail(q.IDiv(k, g, kNew))
		n.fail(q.Rem(k, g, rem))
		if !rem.IsZero() {
			n.fail(q.Inc(kNew))
		}
		for i := range args {
			if n.cmp(args[i].coeff, k) == 0 {
				n.set(args[i].coeff, kNew)
			} else {
				n.fail(q.IDiv(args[i].coeff, g, args[i].coeff))
			}
		}
		n.set(k, kNew)
	}
	if !k.IsOne() {
		min, max := n.fresh(), n.fresh()
		n.set(min, args[0].coeff)
		n.set(max, args[0].coeff)
		for _, arg := range args[1:] {
			if n.cmp(arg.coeff, min) < 0 {
				n.set(min, arg.coeff)
			}
			if n.cmp(arg.coeff, max) > 0 {
				n.set(max, arg.coeff)
			}
		}
		n0, n1, n2 := n.fresh(), n.fresh(), n.fresh()
		n.div(n0, k, max)
		n.fail(q.Floor(n0, n1))
		n.div(n2, k, min)
		n.fail(q.Ceil(n2, n2))
		n.fail(q.Dec(n2))
		if n.cmp(n1, n2) == 0 && !n0.IsInt() {
			for i := range args {
				n.setInt(args[i].coeff, 1)
			}
			n.set(k, n1)
			n.fail(q.Inc(k))
		}
	}
	return args, pbUndef, n.err
}

func (r *PseudoBooleanRewriter) prune(n *pbNumbers, args []pbTerm, k *unumeral.Rational, equality bool) ([]pbTerm, error) {
	if equality {
		return args, nil
	}
	nlt := n.fresh()
	occurrences := 0
	for i := 0; n.cmp(nlt, k) < 0 && i < len(args); i++ {
		if n.cmp(args[i].coeff, k) < 0 {
			n.add(nlt, nlt, args[i].coeff)
			occurrences++
		}
	}
	if n.err != nil {
		return nil, n.err
	}
	if occurrences > 0 && n.cmp(nlt, k) < 0 {
		for i := 0; i < len(args); i++ {
			if n.cmp(args[i].coeff, k) < 0 {
				args[i] = args[len(args)-1]
				args = args[:len(args)-1]
				i--
			}
		}
		var err error
		args, err = r.unique(n, args, k)
		if err != nil {
			return nil, err
		}
		args, _, err = r.normalize(n, args, k, false)
		return args, err
	}
	return args, n.err
}

func (r *PseudoBooleanRewriter) ownedRational(q *unumeral.Rational) (unumeral.OwnedRational, error) {
	s := r.manager.NumeralService()
	v, err := s.NewValue()
	if err != nil {
		return v, err
	}
	if err = s.Manager.Copy(&v.Raw, q); err != nil {
		return v, errors.Join(err, s.Del(&v))
	}
	return v, nil
}

func (r *PseudoBooleanRewriter) build(args []pbTerm, k *unumeral.Rational, equality bool) (result ast.ASTHandle, err error) {
	handles := make([]ast.ASTHandle, len(args))
	coefficients := make([]unumeral.OwnedRational, len(args))
	for i := range args {
		handles[i] = args[i].literal
		coefficients[i], err = r.ownedRational(args[i].coeff)
		if err != nil {
			return result, err
		}
	}
	defer func() {
		for i := range coefficients {
			if coefficients[i] != (unumeral.OwnedRational{}) {
				err = errors.Join(err, r.manager.NumeralService().Del(&coefficients[i]))
			}
		}
	}()
	bound, err := r.ownedRational(k)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, r.manager.NumeralService().Del(&bound)) }()
	if equality {
		return r.manager.MkPseudoBooleanEQ(coefficients, handles, &bound)
	}
	return r.manager.MkPseudoBooleanGE(coefficients, handles, &bound)
}

func (r *PseudoBooleanRewriter) RewriteApp(decl ast.ASTHandle, arguments ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	status, err := r.rewriteApp(decl, arguments, out)
	// mk_app_core's pbu (and its m_refs) is destroyed on return, after the
	// result is held by the caller's expr_ref.
	if e := r.releaseNegations(); e != nil {
		return RewriteFailed, errors.Join(err, e)
	}
	return status, err
}

func (r *PseudoBooleanRewriter) rewriteApp(decl ast.ASTHandle, arguments ast.AppArguments, out ast.ASTRef) (_ RewriteStatus, resultErr error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	n := newPBNumbers(r.manager)
	defer func() { resultErr = errors.Join(resultErr, n.release()) }()
	aux, err := r.manager.IsPseudoBooleanAux(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if aux {
		return RewriteFailed, nil
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil || family != r.family {
		return RewriteFailed, err
	}
	kind, err := r.manager.GetDeclKind(decl)
	if err != nil || kind < ast.ASTPBOpAtMost || kind > ast.ASTPBOpEQ {
		return RewriteFailed, err
	}
	k, err := r.bound(n, decl)
	if err != nil {
		return RewriteFailed, err
	}
	args := make([]pbTerm, arguments.Count())
	for i := range args {
		args[i].literal, err = arguments.At(uint32(i))
		if err != nil {
			return RewriteFailed, err
		}
		args[i].coeff, err = r.coefficient(n, decl, uint32(i))
		if err != nil {
			return RewriteFailed, err
		}
	}
	if kind == ast.ASTPBOpAtMost || kind == ast.ASTPBOpLE {
		for i := range args {
			n.neg(args[i].coeff)
		}
		n.neg(k)
	}
	equality := kind == ast.ASTPBOpEQ
	args, err = r.unique(n, args, k)
	if err != nil {
		return RewriteFailed, err
	}
	var truth pbTruth
	args, truth, err = r.normalize(n, args, k, equality)
	if err != nil {
		return RewriteFailed, err
	}
	args, err = r.prune(n, args, k, equality)
	if err != nil {
		return RewriteFailed, err
	}
	var value ast.ASTHandle
	switch truth {
	case pbTrue:
		value, err = r.manager.MkTrue()
	case pbFalse:
		value, err = r.manager.MkFalse()
	default:
		allUnit := true
		for _, arg := range args {
			allUnit = allUnit && arg.coeff.IsOne()
		}
		if equality && len(args) == 0 {
			if k.IsZero() {
				value, err = r.manager.MkTrue()
			} else {
				value, err = r.manager.MkFalse()
			}
		} else if !equality && allUnit && k.IsOne() {
			handles := make([]ast.ASTHandle, len(args))
			for i := range args {
				handles[i] = args[i].literal
			}
			value, err = r.manager.MkOrArray(handles)
		} else if v, small := k.SmallInt32(); !equality && allUnit && small && int64(v) == int64(len(args)) {
			handles := make([]ast.ASTHandle, len(args))
			for i := range args {
				handles[i] = args[i].literal
			}
			value, err = r.manager.MkAndArray(handles)
		} else {
			value, err = r.build(args, k, equality)
		}
	}
	if err != nil {
		return RewriteFailed, err
	}
	if err = r.manager.SetRef(out, value); err != nil {
		return RewriteFailed, err
	}
	if err = r.traceRewrite(decl, arguments, value); err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, nil
}

func (r *PseudoBooleanRewriter) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	r.closed = true
	return nil
}
