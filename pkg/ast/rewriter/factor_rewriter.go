package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp;
// src/ast/rewriter/factor_rewriter.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

const (
	factorOpLE     int32 = 2
	factorOpGE     int32 = 3
	factorOpLT     int32 = 4
	factorOpGT     int32 = 5
	factorOpAdd    int32 = 6
	factorOpSub    int32 = 7
	factorOpUMinus int32 = 8
	factorOpMul    int32 = 9
)

type factorTerm struct {
	expr     ast.ASTHandle
	positive bool
}

type factorPower struct {
	expr  ast.ASTHandle
	count uint32
}

type factorCall struct {
	owner   *FactorRewriter
	refs    []ast.ASTRef
	adds    []factorTerm
	muls    [][]ast.ASTHandle
	factors []ast.ASTHandle
	powers  []factorPower
}

func (c *factorCall) close() (result error) {
	for i := len(c.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, c.owner.manager.DelRef(&c.refs[i]))
	}
	return result
}

func (c *factorCall) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return h, err
}

func (c *factorCall) app(h ast.ASTHandle, op int32) (bool, error) {
	k, err := c.owner.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := c.owner.manager.GetFamilyID(h)
	if err != nil || f != c.owner.arithFamily {
		return false, err
	}
	d, err := c.owner.manager.GetDeclKind(h)
	return d == op, err
}

func (c *factorCall) args(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := c.owner.manager.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	r := make([]ast.ASTHandle, n)
	for i := range r {
		r[i], err = c.owner.manager.GetArg(h, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (c *factorCall) numeral(h ast.ASTHandle) (text string, integer, matched bool, err error) {
	return c.owner.manager.ArithmeticNumeralText(h)
}

func (c *factorCall) numeralFor(expr ast.ASTHandle, value int64) (ast.ASTHandle, error) {
	h, err := c.numeralRaw(expr, value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(h)
}

// numeralRaw is a().mk_numeral(value, sort(expr)) as a raw expr*: the caller
// decides who owns it.
func (c *factorCall) numeralRaw(expr ast.ASTHandle, value int64) (ast.ASTHandle, error) {
	sort, err := c.owner.manager.GetSort(expr)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := c.owner.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	service := c.owner.manager.NumeralService()
	q, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer service.Del(&q)
	if err = service.SetInt64(&q, value); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.owner.manager.ArithmeticNumeral(&q, kind == 1)
}

func (c *factorCall) negateNumeral(h ast.ASTHandle) (ast.ASTHandle, error) {
	text, integer, matched, err := c.numeral(h)
	if err != nil || !matched {
		return ast.ASTHandle{}, err
	}
	s := c.owner.manager.NumeralService()
	q, err := s.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer s.Del(&q)
	if err = s.SetString(&q, text); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.NegAssign(&q); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.owner.manager.ArithmeticNumeral(&q, integer)
}

func (c *factorCall) mkBasic(makeNode func() (ast.ASTHandle, error)) (ast.ASTHandle, error) {
	h, err := makeNode()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(h)
}

func (c *factorCall) makeAdds(lhs, rhs ast.ASTHandle) error {
	c.adds = append(c.adds[:0], factorTerm{lhs, true}, factorTerm{rhs, false})
	for i := 0; i < len(c.adds); {
		term := &c.adds[i]
		if err := c.traceAddVisit(i, term.expr); err != nil {
			return err
		}
		isAdd, err := c.app(term.expr, factorOpAdd)
		if err != nil {
			return err
		}
		isSub, err := c.app(term.expr, factorOpSub)
		if err != nil {
			return err
		}
		isNeg, err := c.app(term.expr, factorOpUMinus)
		if err != nil {
			return err
		}
		switch {
		case isAdd || isSub:
			children, err := c.args(term.expr)
			if err != nil {
				return err
			}
			if len(children) == 0 {
				i++
				continue
			}
			sign := term.positive
			term.expr = children[0]
			for j := 1; j < len(children); j++ {
				childSign := sign
				if isSub {
					childSign = !sign
				}
				c.adds = append(c.adds, factorTerm{children[j], childSign})
			}
		case isNeg:
			child, err := c.owner.manager.GetArg(term.expr, 0)
			if err != nil {
				return err
			}
			term.expr, term.positive = child, !term.positive
		default:
			text, _, numeral, err := c.numeral(term.expr)
			if err != nil {
				return err
			}
			if numeral && text == "0" {
				c.adds[i] = c.adds[len(c.adds)-1]
				c.adds = c.adds[:len(c.adds)-1]
			} else {
				i++
			}
		}
	}
	return c.traceAdds()
}

func (c *factorCall) expandMuls(row []ast.ASTHandle) ([]ast.ASTHandle, error) {
	for i := 0; i < len(row); {
		mul, err := c.app(row[i], factorOpMul)
		if err != nil {
			return nil, err
		}
		if !mul {
			i++
			continue
		}
		children, err := c.args(row[i])
		if err != nil {
			return nil, err
		}
		if len(children) == 0 {
			i++
			continue
		}
		row[i] = children[0]
		row = append(row, children[1:]...)
	}
	return row, nil
}

func (c *factorCall) makeMuls() error {
	c.muls = c.muls[:0]
	for i := 0; i < len(c.adds); {
		row, err := c.expandMuls([]ast.ASTHandle{c.adds[i].expr})
		if err != nil {
			return err
		}
		if len(row) == 0 {
			c.adds = append(c.adds[:i], c.adds[i+1:]...)
			continue
		}
		c.muls = append(c.muls, row)
		i++
	}
	return c.traceMuls()
}

// collectPowers is factor_rewriter::collect_powers: m_powers is the
// rewriter's obj_map<expr, unsigned>, reset and refilled, and its
// consumers walk it in table order.
func (c *factorCall) collectPowers() error {
	powers := c.owner.powers
	powers.Reset()
	for _, h := range c.factors {
		entry, err := powers.InsertIfNotThere(h)
		if err != nil {
			return err
		}
		entry.Num++
	}
	c.powers = c.powers[:0]
	for _, entry := range powers.Entries() {
		c.powers = append(c.powers, factorPower{entry.Key, uint32(entry.Num)})
	}
	return nil
}

func removeFactor(row []ast.ASTHandle, i int) []ast.ASTHandle {
	return append(row[:i], row[i+1:]...)
}

// resetFactors is m_factors.reset(): the member expr_ref_vector releases its
// references front to back. m_factors outlives the call, as in C++, until
// the next extract_factors or the rewriter's destruction.
func (r *FactorRewriter) resetFactors() (result error) {
	for i := range r.factorRefs {
		result = errors.Join(result, r.manager.DelRef(&r.factorRefs[i]))
	}
	r.factorRefs = r.factorRefs[:0]
	return result
}

func (c *factorCall) pushFactor(h ast.ASTHandle) error {
	ref, err := c.owner.manager.NewRef(h)
	if err != nil {
		return err
	}
	c.owner.factorRefs = append(c.owner.factorRefs, ref)
	c.factors = append(c.factors, h)
	return nil
}

func (c *factorCall) extractFactors() (_ bool, result error) {
	c.factors = c.factors[:0]
	if err := c.owner.resetFactors(); err != nil {
		return false, err
	}
	if len(c.muls) == 1 {
		if len(c.muls[0]) <= 1 {
			return false, nil
		}
		for _, h := range c.muls[0] {
			if err := c.pushFactor(h); err != nil {
				return false, err
			}
		}
		if !c.adds[0].positive {
			found := false
			for i, h := range c.factors {
				_, _, numeral, err := c.numeral(h)
				if err != nil {
					return false, err
				}
				if numeral {
					negated, err := c.negateNumeral(h)
					if err != nil {
						return false, err
					}
					if err = c.owner.manager.SetRef(c.owner.factorRefs[i], negated); err != nil {
						return false, err
					}
					c.factors[i] = negated
					found = true
					break
				}
			}
			if !found {
				minusOne, err := c.numeralRaw(c.muls[0][0], -1)
				if err != nil {
					return false, err
				}
				if err = c.pushFactor(minusOne); err != nil {
					return false, err
				}
			}
		}
		if err := c.collectPowers(); err != nil {
			return false, err
		}
		return true, nil
	}
	for i := 0; i < len(c.muls[0]); {
		candidate := c.muls[0][i]
		positions := make([]int, len(c.muls))
		positions[0] = i
		ok := true
		for j := 1; j < len(c.muls); j++ {
			positions[j] = -1
			for k, h := range c.muls[j] {
				if h == candidate {
					positions[j] = k
					break
				}
			}
			if positions[j] < 0 {
				ok = false
				break
			}
		}
		if !ok {
			i++
			continue
		}
		if err := c.pushFactor(candidate); err != nil {
			return false, err
		}
		for j, p := range positions {
			c.muls[j] = removeFactor(c.muls[j], p)
		}
	}
	if len(c.factors) == 0 {
		return false, nil
	}
	// trail is the local expr_ref_vector: it releases its references front
	// to back when extract_factors returns.
	trail := make([]ast.ASTHandle, 0, len(c.adds))
	trailRefs := make([]ast.ASTRef, 0, len(c.adds))
	defer func() {
		for i := range trailRefs {
			result = errors.Join(result, c.owner.manager.DelRef(&trailRefs[i]))
		}
	}()
	for i, term := range c.adds {
		var h ast.ASTHandle
		var err error
		switch len(c.muls[i]) {
		case 0:
			h, err = c.numeralRaw(c.factors[0], 1)
		case 1:
			h = c.muls[i][0]
		default:
			h, err = c.owner.manager.MkFamilyApp(c.owner.arithFamily, factorOpMul, nil, c.muls[i])
		}
		if err != nil {
			return false, err
		}
		if !term.positive {
			h, err = c.owner.manager.MkFamilyApp(c.owner.arithFamily, factorOpUMinus, nil, []ast.ASTHandle{h})
			if err != nil {
				return false, err
			}
		}
		ref, err := c.owner.manager.NewRef(h)
		if err != nil {
			return false, err
		}
		trailRefs = append(trailRefs, ref)
		trail = append(trail, h)
	}
	if len(trail) == 1 {
		if err := c.pushFactor(trail[0]); err != nil {
			return false, err
		}
	} else if len(trail) > 1 {
		h, err := c.owner.manager.MkFamilyApp(c.owner.arithFamily, factorOpAdd, nil, trail)
		if err != nil {
			return false, err
		}
		if err = c.pushFactor(h); err != nil {
			return false, err
		}
	}
	if err := c.traceFactors(); err != nil {
		return false, err
	}
	if err := c.collectPowers(); err != nil {
		return false, err
	}
	return true, nil
}

// isNegative is mk_is_negative(result, eqs). Its expr_ref locals neg0, neg,
// pos0, pos, tmp and zero release their references, in reverse declaration
// order, when it returns; the unused pos0 chain dies there. The reference
// GCC build evaluates mk_or(mk_and(a, b), mk_and(c, d)) right to left, so
// the second conjunction is built first.
func (c *factorCall) isNegative() (_ ast.ASTHandle, _ []ast.ASTHandle, result error) {
	m := c.owner.manager
	const (
		neg0 = iota
		neg
		pos0
		pos
		tmp
		zero
		localCount
	)
	var locals [localCount]ast.ASTRef
	created := 0
	defer func() {
		for i := created - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&locals[i]))
		}
	}()
	for i := range locals {
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		locals[i] = ref
		created++
	}
	get := func(i int) ast.ASTHandle {
		h, _ := m.RefGet(locals[i])
		return h
	}
	set := func(i int, h ast.ASTHandle, err error) error {
		if err != nil {
			return err
		}
		return m.SetRef(locals[i], h)
	}
	h, err := c.numeralRaw(c.powers[0].expr, 0)
	if err = set(zero, h, err); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	z := get(zero)
	h, err = m.MkTrue()
	if err = set(pos0, h, err); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	h, err = m.MkFalse()
	if err = set(neg0, h, err); err != nil {
		return ast.ASTHandle{}, nil, err
	}
	eqs := make([]ast.ASTHandle, 0, len(c.powers))
	for _, power := range c.powers {
		eq, err := c.mkBasic(func() (ast.ASTHandle, error) { return m.MkEq(z, power.expr) })
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		eqs = append(eqs, eq)
		if power.count&1 == 0 {
			continue
		}
		h, err = m.MkFamilyApp(c.owner.arithFamily, factorOpLT, nil, []ast.ASTHandle{z, power.expr})
		if err = set(pos, h, err); err != nil {
			return ast.ASTHandle{}, nil, err
		}
		h, err = m.MkFamilyApp(c.owner.arithFamily, factorOpLT, nil, []ast.ASTHandle{power.expr, z})
		if err = set(neg, h, err); err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if m.IsFalse(get(neg0)) {
			if err = set(neg0, get(neg), nil); err != nil {
				return ast.ASTHandle{}, nil, err
			}
			if err = set(pos0, get(pos), nil); err != nil {
				return ast.ASTHandle{}, nil, err
			}
			continue
		}
		// tmp = or(and(pos, pos0), and(neg, neg0))
		nn, err := m.MkAnd(get(neg), get(neg0))
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		pp, err := m.MkAnd(get(pos), get(pos0))
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		h, err = m.MkOr(pp, nn)
		if err = set(tmp, h, err); err != nil {
			return ast.ASTHandle{}, nil, err
		}
		// neg0 = or(and(neg, pos0), and(pos, neg0))
		pn, err := m.MkAnd(get(pos), get(neg0))
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		np, err := m.MkAnd(get(neg), get(pos0))
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		h, err = m.MkOr(np, pn)
		if err = set(neg0, h, err); err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if err = set(pos0, get(tmp), nil); err != nil {
			return ast.ASTHandle{}, nil, err
		}
	}
	// result = neg0: the caller's expr_ref takes its reference before the
	// locals die.
	r, err := c.hold(get(neg0))
	return r, eqs, err
}

func (c *factorCall) rewriteRelation(op int32, lhs, rhs ast.ASTHandle) (ast.ASTHandle, RewriteStatus, error) {
	if err := c.makeAdds(lhs, rhs); err != nil {
		return ast.ASTHandle{}, RewriteFailed, err
	}
	if err := c.makeMuls(); err != nil {
		return ast.ASTHandle{}, RewriteFailed, err
	}
	if len(c.muls) == 0 {
		if op == factorOpLT {
			h, err := c.owner.manager.MkFalse()
			return h, RewriteDone, err
		}
		h, err := c.owner.manager.MkTrue()
		return h, RewriteDone, err
	}
	ok, err := c.extractFactors()
	if err != nil || !ok {
		if err == nil {
			err = c.traceRelationRefusal(op, lhs, rhs)
		}
		return ast.ASTHandle{}, RewriteFailed, err
	}
	if op == -1 {
		eqs := make([]ast.ASTHandle, 0, len(c.powers))
		for _, power := range c.powers {
			zero, err := c.numeralFor(power.expr, 0)
			if err != nil {
				return ast.ASTHandle{}, RewriteFailed, err
			}
			eq, err := c.owner.manager.MkEq(power.expr, zero)
			if err != nil {
				return ast.ASTHandle{}, RewriteFailed, err
			}
			if _, err = c.hold(eq); err != nil {
				return ast.ASTHandle{}, RewriteFailed, err
			}
			eqs = append(eqs, eq)
		}
		// ast_manager::mk_or(n, args) builds an or even for one argument.
		h, err := c.owner.manager.MkOrArgs(eqs)
		return h, RewriteDone, err
	}
	negative, eqs, err := c.isNegative()
	if err != nil {
		return ast.ASTHandle{}, RewriteFailed, err
	}
	if op == factorOpLE {
		eqs = append(eqs, negative)
		h, err := c.owner.manager.MkOrArgs(eqs)
		if err == nil {
			err = c.traceLessEqualResult(lhs, rhs, h)
		}
		return h, RewriteDone, err
	}
	for i, eq := range eqs {
		eqs[i], err = c.owner.manager.MkNot(eq)
		if err != nil {
			return ast.ASTHandle{}, RewriteFailed, err
		}
		if _, err = c.hold(eqs[i]); err != nil {
			return ast.ASTHandle{}, RewriteFailed, err
		}
	}
	eqs = append(eqs, negative)
	h, err := c.owner.manager.MkAndArgs(eqs)
	if err == nil {
		err = c.traceStrictResult(h)
	}
	return h, RewriteDone, err
}

type factorRewriterConfiguration struct {
	DefaultConfiguration
	owner *FactorRewriter
}

func (c *factorRewriterConfiguration) RewritePatterns() bool                 { return false }
func (c *factorRewriterConfiguration) FlatAssoc(ast.ASTHandle) (bool, error) { return false, nil }
func (c *factorRewriterConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (RewriteStatus, error) {
	if err := c.owner.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return RewriteFailed, err
	}
	return c.owner.RewriteApp(decl, args, out)
}

// FactorRewriter ports factor_rewriter and its retained no-proof traversal.
type FactorRewriter struct {
	self        *FactorRewriter
	manager     *ast.ASTManager
	arithFamily ast.FamilyID
	config      factorRewriterConfiguration
	traversal   *Traversal
	trace       *diagnostic.TraceTags
	// powers is the member m_powers (obj_map<expr, unsigned>).
	powers *ast.ObjMap
	// factorRefs owns the member m_factors (expr_ref_vector).
	factorRefs []ast.ASTRef
	closed     bool
}

func NewFactorRewriter(m *ast.ASTManager) (*FactorRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	r := &FactorRewriter{manager: m, arithFamily: family, powers: ast.NewObjMap(m)}
	r.self, r.config.owner = r, r
	r.traversal, err = NewTraversal(m, &r.config)
	if err != nil {
		if r.traversal != nil {
			err = errors.Join(err, r.traversal.Close())
		}
		return r, err
	}
	return r, nil
}

func (r *FactorRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	return nil
}

// RewriteApp applies the source factor_rewriter core to a declaration and arguments.
func (r *FactorRewriter) RewriteApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (status RewriteStatus, result error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	op, err := r.manager.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if family == r.manager.BasicFamilyID() && op == ast.ASTOpEq {
		op = -1
	} else if family != r.arithFamily || op < factorOpLE || op > factorOpGT {
		return RewriteFailed, nil
	}
	if ast.ASTSourceDebug && args.Count() != 2 {
		return RewriteFailed, ast.ErrASTDomain
	}
	lhs, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	rhs, err := args.At(1)
	if err != nil {
		return RewriteFailed, err
	}
	if op == -1 {
		sort, err := r.manager.GetSort(lhs)
		if err != nil {
			return RewriteFailed, err
		}
		f, err := r.manager.GetFamilyID(sort)
		if err != nil || f != r.arithFamily {
			return RewriteFailed, err
		}
	}
	if op == factorOpGE {
		lhs, rhs, op = rhs, lhs, factorOpLE
	} else if op == factorOpGT {
		lhs, rhs, op = rhs, lhs, factorOpLT
	}
	call := factorCall{owner: r}
	defer func() { result = errors.Join(result, call.close()) }()
	h, status, err := call.rewriteRelation(op, lhs, rhs)
	if err != nil || status == RewriteFailed {
		return status, err
	}
	return status, r.manager.SetRef(out, h)
}

// Rewrite factors supported relations throughout root.
func (r *FactorRewriter) Rewrite(root ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Rewrite(root, out)
}

func (r *FactorRewriter) Close() error {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
	}
	if err := r.resetFactors(); err != nil {
		return err
	}
	if r.powers != nil {
		r.powers.Finalize()
	}
	r.closed = true
	return nil
}
