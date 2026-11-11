package rewriter

// Source: src/ast/rewriter/quant_hoist.cpp; src/ast/rewriter/quant_hoist.h.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type quantifierType uint8

const (
	quantForallPositive quantifierType = 0x10
	quantExistsPositive quantifierType = 0x20
	quantNonePositive   quantifierType = 0x40
	quantForallNegative quantifierType = 0x11
	quantExistsNegative quantifierType = 0x21
	quantNoneNegative   quantifierType = 0x41
)

type QuantifierHoister struct {
	self           *QuantifierHoister
	m              *ast.ASTManager
	bool           *boolRewriter
	defaultBoolean bool
	closed         bool
}

func NewQuantifierHoister(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*QuantifierHoister, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	rw, err := newBoolRewriter(m, c, g, p)
	if err != nil {
		if rw != nil {
			return nil, errors.Join(err, rw.Close())
		}
		return nil, err
	}
	h := &QuantifierHoister{m: m, bool: rw}
	h.self = h
	return h, nil
}

// NewDefaultQuantifierHoister captures source-default Boolean parameters from
// the AST manager's services. Plain AST managers use intrinsic gate defaults.
func NewDefaultQuantifierHoister(m *ast.ASTManager) (*QuantifierHoister, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	context, globals, err := m.RewriterParameterServices()
	if err != nil {
		return nil, err
	}
	if context != nil && globals != nil {
		// bool_rewriter(m) captures the current global module defaults at
		// construction, without a caller-local parameter overlay.
		return NewQuantifierHoister(m, context, globals, nil)
	}
	h := &QuantifierHoister{m: m, defaultBoolean: true}
	h.self = h
	return h, nil
}

// NewQuantifierHoisterWithoutRewrite supports clients that explicitly request
// quantifier movement without Boolean reduction. Rewrite requests still require
// a configured or default Boolean reducer.
func NewQuantifierHoisterWithoutRewrite(m *ast.ASTManager) (*QuantifierHoister, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	h := &QuantifierHoister{m: m}
	h.self = h
	return h, nil
}

func (h *QuantifierHoister) open() error {
	if h == nil || h.self != h || h.closed || h.m == nil {
		return ast.ErrASTHandle
	}
	return nil
}

type QuantifierHoistResult struct {
	self      *QuantifierHoistResult
	manager   *ast.ASTManager
	Variables []ast.ASTRef
	Formula   ast.ASTRef
	IsForall  bool
	closed    bool
}

func (r *QuantifierHoistResult) Close() error {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.Formula != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.Formula); err != nil {
			return err
		}
	}
	for len(r.Variables) != 0 {
		last := len(r.Variables) - 1
		if err := r.manager.DelRef(&r.Variables[last]); err != nil {
			return err
		}
		r.Variables = r.Variables[:last]
	}
	r.closed = true
	return nil
}

type quantifierHoistState struct {
	owner     *QuantifierHoister
	temporary []ast.ASTRef
	variables []ast.ASTRef
}

func (s *quantifierHoistState) scratch(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := s.owner.m.NewRef(h)
	if err == nil {
		s.temporary = append(s.temporary, r)
	}
	return r, err
}
func (s *quantifierHoistState) own(h ast.ASTHandle) (ast.ASTHandle, error) {
	_, err := s.scratch(h)
	return h, err
}
func (s *quantifierHoistState) cleanup() error {
	for len(s.temporary) != 0 {
		last := len(s.temporary) - 1
		if err := s.owner.m.DelRef(&s.temporary[last]); err != nil {
			return err
		}
		s.temporary = s.temporary[:last]
	}
	return nil
}
func (s *quantifierHoistState) cleanupVariables() error {
	for len(s.variables) != 0 {
		last := len(s.variables) - 1
		if err := s.owner.m.DelRef(&s.variables[last]); err != nil {
			return err
		}
		s.variables = s.variables[:last]
	}
	return nil
}

func negateQuantifierType(q *quantifierType) { *q ^= 1 }
func compatibleQuantifier(q quantifierType, forall bool) bool {
	switch q {
	case quantForallPositive:
		return forall
	case quantForallNegative:
		return !forall
	case quantExistsPositive:
		return !forall
	case quantExistsNegative:
		return forall
	case quantNonePositive, quantNoneNegative:
		return true
	}
	return false
}
func setQuantifierType(q *quantifierType, forall bool) error {
	switch *q {
	case quantForallPositive:
		if !forall {
			return ast.ErrASTDomain
		}
	case quantForallNegative:
		if forall {
			return ast.ErrASTDomain
		}
	case quantExistsPositive:
		if forall {
			return ast.ErrASTDomain
		}
	case quantExistsNegative:
		if !forall {
			return ast.ErrASTDomain
		}
	case quantNonePositive:
		if forall {
			*q = quantForallPositive
		} else {
			*q = quantExistsPositive
		}
	case quantNoneNegative:
		if forall {
			*q = quantExistsNegative
		} else {
			*q = quantForallNegative
		}
	default:
		return ast.ErrASTDomain
	}
	return nil
}

func (s *quantifierHoistState) isOp(h ast.ASTHandle, op int32) (bool, error) {
	k, err := s.owner.m.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := s.owner.m.GetFamilyID(h)
	if err != nil || f != 0 {
		return false, err
	}
	d, err := s.owner.m.GetDeclKind(h)
	return d == op, err
}
func (s *quantifierHoistState) args(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := s.owner.m.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	r := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		r[i], err = s.owner.m.GetArg(h, i)
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}
func (s *quantifierHoistState) construct(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return s.own(h)
}

// hold returns a node with a fresh owning reference: one expr_ref local of
// the source's pull_quantifier frame. The calling frame releases it where
// that local is destroyed.
func (s *quantifierHoistState) hold(h ast.ASTHandle, err error) (ast.ASTHandle, ast.ASTRef, error) {
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	ref, err := s.owner.m.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	return h, ref, nil
}

// drop releases one frame local; a zero reference was never assigned.
func (s *quantifierHoistState) drop(ref *ast.ASTRef) error {
	if *ref == (ast.ASTRef{}) {
		return nil
	}
	return s.owner.m.DelRef(ref)
}

func (s *quantifierHoistState) dropAll(refs ...*ast.ASTRef) error {
	for _, ref := range refs {
		if err := s.drop(ref); err != nil {
			return err
		}
	}
	return nil
}

func (s *quantifierHoistState) mkGate(op int32, args []ast.ASTHandle, rewrite bool) (ast.ASTHandle, ast.ASTRef, error) {
	if !rewrite {
		if op == ast.ASTOpAnd {
			return s.hold(s.owner.m.MkAndArgs(args))
		}
		return s.hold(s.owner.m.MkOrArgs(args))
	}
	out, err := s.owner.m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	switch {
	case s.owner.defaultBoolean:
		err = SimplifyBooleanGateDefault(s.owner.m, op, args, out)
	case op == ast.ASTOpAnd:
		err = s.owner.bool.mkAnd(args, out)
	default:
		err = s.owner.bool.mkOr(args, out)
	}
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, errors.Join(err, s.owner.m.DelRef(&out))
	}
	h, err := s.owner.m.RefGet(out)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, errors.Join(err, s.owner.m.DelRef(&out))
	}
	return h, out, nil
}

func (s *quantifierHoistState) mkAnd(args []ast.ASTHandle, rewrite bool) (ast.ASTHandle, ast.ASTRef, error) {
	return s.mkGate(ast.ASTOpAnd, args, rewrite)
}
func (s *quantifierHoistState) mkOr(args []ast.ASTHandle, rewrite bool) (ast.ASTHandle, ast.ASTRef, error) {
	return s.mkGate(ast.ASTOpOr, args, rewrite)
}

// extract is extract_quantifier: the fresh constants join vars for the whole
// hoist; the instantiated body is the quantifier case's tmp local.
func (s *quantifierHoistState) extract(q ast.ASTHandle, useFresh bool) (ast.ASTHandle, ast.ASTRef, error) {
	n, err := s.owner.m.GetNumDeclarations(q)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	for i := uint32(0); i < n; i++ {
		sort, e := s.owner.m.GetDeclarationSort(q, i)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, e
		}
		name, e := s.owner.m.GetDeclarationName(q, i)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, e
		}
		// Reserve the app_ref_vector entry before creating a fresh constant;
		// allocating its Go root afterward can strand the new application.
		ref, e := s.owner.m.NewRef(ast.ASTHandle{})
		if ref != (ast.ASTRef{}) {
			s.variables = append(s.variables, ref)
		}
		if e != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, e
		}
		var v ast.ASTHandle
		if useFresh {
			prefix, cause := s.owner.m.Symbols.String(name)
			if cause != nil {
				return ast.ASTHandle{}, ast.ASTRef{}, cause
			}
			v, e = s.owner.m.MkFreshConst(prefix, sort)
		} else {
			v, e = s.owner.m.MkConst(name, sort)
		}
		if !v.IsNull() {
			e = errors.Join(e, s.owner.m.SetRef(ref, v))
		}
		if e != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, e
		}
	}
	values := make([]ast.ASTHandle, n)
	base := len(s.variables) - int(n)
	for i := range values {
		values[i], err = s.owner.m.RefGet(s.variables[base+i])
		if err != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, err
		}
	}
	out, err := s.owner.m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	if err = InstantiateQuantifier(s.owner.m, q, values, out); err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, errors.Join(err, s.owner.m.DelRef(&out))
	}
	h, err := s.owner.m.RefGet(out)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, errors.Join(err, s.owner.m.DelRef(&out))
	}
	return h, out, nil
}

// pull is pull_quantifier(fml, qt, vars, result, ...). The result comes with
// its owning reference (the caller's expr_ref result), which the caller
// releases as its expr_ref; this frame's own expr_ref locals (tmp, args, the
// ite/iff temporaries) are released where the source destroys them, in its
// reverse declaration order.
func (s *quantifierHoistState) pull(fml ast.ASTHandle, qt *quantifierType, useFresh, rewrite bool) (result ast.ASTHandle, owner ast.ASTRef, resultErr error) {
	// result = fml: the caller's expr_ref holds every result, including an
	// unchanged input that may be one of the caller's own locals.
	has, err := s.owner.m.HasQuantifiers(fml)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	if !has {
		return s.hold(fml, nil)
	}
	kind, err := s.owner.m.GetKind(fml)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	var locals []*ast.ASTRef
	defer func() {
		if resultErr == nil {
			return
		}
		for i := len(locals) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, s.drop(locals[i]))
		}
		resultErr = errors.Join(resultErr, s.drop(&owner))
		result = ast.ASTHandle{}
	}()
	fail := func(e error) (ast.ASTHandle, ast.ASTRef, error) { return ast.ASTHandle{}, ast.ASTRef{}, e }
	if kind == ast.ASTQuantifier {
		forall, e := s.owner.m.IsForall(fml)
		if e != nil {
			return fail(e)
		}
		if !compatibleQuantifier(*qt, forall) {
			return s.hold(fml, nil)
		}
		if e = setQuantifierType(qt, forall); e != nil {
			return fail(e)
		}
		body, tmp, e := s.extract(fml, useFresh)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &tmp)
		result, owner, e = s.pull(body, qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		return result, owner, s.drop(&tmp)
	}
	if kind == ast.ASTVar {
		return s.hold(fml, nil)
	}
	if kind != ast.ASTApp {
		return fail(ast.ErrASTDomain)
	}
	args, err := s.args(fml)
	if err != nil {
		return fail(err)
	}
	isAnd, e := s.isOp(fml, ast.ASTOpAnd)
	if e != nil {
		return fail(e)
	}
	isOr, e := s.isOp(fml, ast.ASTOpOr)
	if e != nil {
		return fail(e)
	}
	if isAnd || isOr {
		out := make([]ast.ASTHandle, len(args))
		owners := make([]ast.ASTRef, len(args))
		for i := range args {
			out[i], owners[i], err = s.pull(args[i], qt, useFresh, rewrite)
			if err != nil {
				return fail(err)
			}
			locals = append(locals, &owners[i])
		}
		if isAnd {
			result, owner, err = s.mkAnd(out, rewrite)
		} else {
			result, owner, err = s.mkOr(out, rewrite)
		}
		if err != nil {
			return fail(err)
		}
		// tmp (the last element again), then args in element order.
		for i := range owners {
			if err = s.drop(&owners[i]); err != nil {
				return fail(err)
			}
		}
		return result, owner, nil
	}
	if yes, e := s.isOp(fml, ast.ASTOpNot); e != nil {
		return fail(e)
	} else if yes && len(args) == 1 {
		negateQuantifierType(qt)
		child, tmp, e := s.pull(args[0], qt, useFresh, rewrite)
		negateQuantifierType(qt)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &tmp)
		if result, owner, e = s.hold(s.owner.m.MkNot(child)); e != nil {
			return fail(e)
		}
		return result, owner, s.drop(&tmp)
	}
	if yes, e := s.isOp(fml, ast.ASTOpImplies); e != nil {
		return fail(e)
	} else if yes && len(args) == 2 {
		negateQuantifierType(qt)
		left, tmp, e := s.pull(args[0], qt, useFresh, rewrite)
		negateQuantifierType(qt)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &tmp)
		right, pulled, e := s.pull(args[1], qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &pulled)
		// result = m.mk_implies(tmp, result): the assignment releases the
		// pulled consequent; tmp goes with the frame.
		if result, owner, e = s.hold(s.owner.m.MkImplies(left, right)); e != nil {
			return fail(e)
		}
		return result, owner, s.dropAll(&pulled, &tmp)
	}
	if yes, e := s.isOp(fml, ast.ASTOpIte); e != nil {
		return fail(e)
	} else if yes && len(args) == 3 {
		// expr_ref tt1, tt2, tt3, ntt1, nt1: destroyed nt1, ntt1, tt3, tt2, tt1.
		thenPart, tt2, e := s.pull(args[1], qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &tt2)
		elsePart, tt3, e := s.pull(args[2], qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &tt3)
		hc, e := s.owner.m.HasQuantifiers(args[0])
		if e != nil {
			return fail(e)
		}
		if !hc {
			if result, owner, e = s.hold(s.owner.m.MkIte(args[0], thenPart, elsePart)); e != nil {
				return fail(e)
			}
			return result, owner, s.dropAll(&tt3, &tt2)
		}
		positive, tt1, e := s.pull(args[0], qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &tt1)
		neg, nt1, e := s.hold(s.owner.m.MkNot(args[0]))
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &nt1)
		negative, ntt1, e := s.pull(neg, qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &ntt1)
		// m.mk_and(m.mk_or(ntt1, tt2), m.mk_or(tt1, tt3)): the source build
		// evaluates the arguments right to left, so the second or is
		// created first; both are raw until the and holds them.
		b, bRef, e := s.mkOr([]ast.ASTHandle{positive, elsePart}, false)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &bRef)
		a, aRef, e := s.mkOr([]ast.ASTHandle{negative, thenPart}, false)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &aRef)
		if result, owner, e = s.mkAnd([]ast.ASTHandle{a, b}, false); e != nil {
			return fail(e)
		}
		return result, owner, s.dropAll(&aRef, &bRef, &nt1, &ntt1, &tt3, &tt2, &tt1)
	}
	iff, e := s.isOp(fml, ast.ASTOpIff)
	if e != nil {
		return fail(e)
	}
	eq, e := s.isOp(fml, ast.ASTOpEq)
	if e != nil {
		return fail(e)
	}
	boolEq := false
	if eq && len(args) == 2 {
		sort, x := s.owner.m.GetSort(args[0])
		if x != nil {
			return fail(x)
		}
		bs, x := s.owner.m.MkBoolSort()
		if x != nil {
			return fail(x)
		}
		boolEq = sort == bs
	}
	if (iff || boolEq) && len(args) == 2 {
		// expr_ref tt1, tt2, ntt1, ntt2, nt1, nt2: destroyed nt2, nt1, ntt2,
		// ntt1, tt2, tt1.
		p, tt1, e := s.pull(args[0], qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &tt1)
		q, tt2, e := s.pull(args[1], qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &tt2)
		n1, nt1, e := s.hold(s.owner.m.MkNot(args[0]))
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &nt1)
		n2, nt2, e := s.hold(s.owner.m.MkNot(args[1]))
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &nt2)
		np, ntt1, e := s.pull(n1, qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &ntt1)
		nq, ntt2, e := s.pull(n2, qt, useFresh, rewrite)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &ntt2)
		// m.mk_and(m.mk_or(ntt1, tt2), m.mk_or(ntt2, tt1)): right-to-left
		// argument evaluation creates the second or first.
		b, bRef, e := s.mkOr([]ast.ASTHandle{nq, p}, false)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &bRef)
		a, aRef, e := s.mkOr([]ast.ASTHandle{np, q}, false)
		if e != nil {
			return fail(e)
		}
		locals = append(locals, &aRef)
		if result, owner, e = s.mkAnd([]ast.ASTHandle{a, b}, false); e != nil {
			return fail(e)
		}
		return result, owner, s.dropAll(&aRef, &bRef, &nt2, &nt1, &ntt2, &ntt1, &tt2, &tt1)
	}
	// The formula contains a quantifier, but it is inaccessible.
	return s.hold(fml, nil)
}

func (h *QuantifierHoister) run(fml ast.ASTHandle, qt quantifierType, useFresh, rewrite bool) (result *QuantifierHoistResult, err error) {
	if err = h.open(); err != nil {
		return nil, err
	}
	if rewrite && h.bool == nil && !h.defaultBoolean {
		return nil, uparams.ErrParamsOwner
	}
	s := &quantifierHoistState{owner: h}
	defer func() {
		err = errors.Join(err, s.cleanup())
		if err != nil {
			err = errors.Join(err, s.cleanupVariables())
		}
	}()
	formula, ref, err := s.pull(fml, &qt, useFresh, rewrite)
	if err != nil {
		return nil, err
	}
	if ref == (ast.ASTRef{}) {
		if ref, err = h.m.NewRef(formula); err != nil {
			return nil, err
		}
	}
	result = &QuantifierHoistResult{manager: h.m, Variables: s.variables, Formula: ref, IsForall: qt == quantForallPositive}
	result.self = result
	s.variables = nil
	return result, nil
}

func (h *QuantifierHoister) formatTrace(hnd ast.ASTHandle) (value string, result error) {
	format, err := h.m.SMT2PPExprTree(hnd, nil)
	if err != nil {
		return "", err
	}
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: h.m.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if err = h.m.PrintFormatTransient(out, format, nil, nil, nil); err != nil {
		return "", err
	}
	return out.String()
}
func (h *QuantifierHoister) trace(before, after ast.ASTHandle, line int) error {
	tags := diagnostic.BorrowProcessTraceTags()
	return tags.EmitConditional("qe_verbose", "qe_verbose", "src/ast/rewriter/quant_hoist.cpp", line, func() bool { return true }, func(w io.Writer) error {
		a, err := h.formatTrace(before)
		if err != nil {
			return err
		}
		b, err := h.formatTrace(after)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, a+"\n"+b+"\n")
		return err
	})
}
func (h *QuantifierHoister) Hoist(fml ast.ASTHandle, useFresh, rewrite bool) (*QuantifierHoistResult, error) {
	r, err := h.run(fml, quantNonePositive, useFresh, rewrite)
	if err != nil {
		return nil, err
	}
	after, e := h.m.RefGet(r.Formula)
	if e == nil {
		e = h.trace(fml, after, 44)
	}
	if e != nil {
		return nil, errors.Join(e, r.Close())
	}
	return r, nil
}
func (h *QuantifierHoister) PullExists(fml ast.ASTHandle, useFresh, rewrite bool) (*QuantifierHoistResult, error) {
	r, err := h.run(fml, quantExistsPositive, useFresh, rewrite)
	if err != nil {
		return nil, err
	}
	after, e := h.m.RefGet(r.Formula)
	if e == nil {
		e = h.trace(fml, after, 55)
	}
	if e != nil {
		return nil, errors.Join(e, r.Close())
	}
	return r, nil
}
func (h *QuantifierHoister) PullQuantifier(forall bool, fml ast.ASTHandle, useFresh, rewrite bool) (*QuantifierHoistResult, error) {
	q := quantExistsPositive
	if forall {
		q = quantForallPositive
	}
	r, err := h.run(fml, q, useFresh, rewrite)
	if err != nil {
		return nil, err
	}
	after, e := h.m.RefGet(r.Formula)
	if e == nil {
		e = h.trace(fml, after, 65)
	}
	if e != nil {
		return nil, errors.Join(e, r.Close())
	}
	return r, nil
}

type QuantifierIndexedResult struct {
	*QuantifierHoistResult
	Index uint32
	Sorts []ast.ASTHandle
	Names []usymbol.Symbol
}

func (h *QuantifierHoister) PullQuantifierIndexed(forall bool, fml ast.ASTHandle, useFresh, rewrite bool) (result *QuantifierIndexedResult, err error) {
	if err = h.open(); err != nil {
		return nil, err
	}
	counter, e := NewVarCounter(h.m)
	if e != nil {
		return nil, e
	}
	index, e := counter.NextVar(fml)
	if e != nil {
		return nil, e
	}
	sorts := []ast.ASTHandle{}
	names := []usymbol.Symbol{}
	current := fml
	for {
		kind, e := h.m.GetKind(current)
		if e != nil {
			return nil, e
		}
		if kind != ast.ASTQuantifier {
			break
		}
		fa, e := h.m.IsForall(current)
		if e != nil {
			return nil, e
		}
		if fa != forall {
			break
		}
		n, e := h.m.GetNumDeclarations(current)
		if e != nil {
			return nil, e
		}
		index += n
		for i := uint32(0); i < n; i++ {
			name, x := h.m.GetDeclarationName(current, i)
			if x != nil {
				return nil, x
			}
			sort, x := h.m.GetDeclarationSort(current, i)
			if x != nil {
				return nil, x
			}
			names = append(names, name)
			sorts = append(sorts, sort)
		}
		current, e = h.m.GetQuantifierBody(current)
		if e != nil {
			return nil, e
		}
	}
	has, e := h.m.HasQuantifiers(current)
	if e != nil {
		return nil, e
	}
	if !has {
		ref, e := h.m.NewRef(current)
		if e != nil {
			return nil, e
		}
		base := &QuantifierHoistResult{manager: h.m, Formula: ref, IsForall: forall}
		base.self = base
		return &QuantifierIndexedResult{QuantifierHoistResult: base, Index: index, Sorts: sorts, Names: names}, nil
	}
	base, e := h.PullQuantifier(forall, current, useFresh, rewrite)
	if e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, base.Close())
		}
	}()
	if len(base.Variables) == 0 {
		return &QuantifierIndexedResult{QuantifierHoistResult: base, Index: index, Sorts: sorts, Names: names}, nil
	}
	rep, e := NewExprSafeReplace(h.m)
	if e != nil {
		return nil, e
	}
	defer func() { err = errors.Join(err, rep.Close()) }()
	boundNames := []usymbol.Symbol{}
	boundSorts := []ast.ASTHandle{}
	for _, vr := range base.Variables {
		v, x := h.m.RefGet(vr)
		if x != nil {
			return nil, x
		}
		decl, x := h.m.GetDecl(v)
		if x != nil {
			return nil, x
		}
		name, x := h.m.GetASTName(decl)
		if x != nil {
			return nil, x
		}
		sort, x := h.m.GetSort(v)
		if x != nil {
			return nil, x
		}
		boundNames = append(boundNames, name)
		boundSorts = append(boundSorts, sort)
		// ExprSafeReplace owns inserted substitutions. Its temporary root
		// also covers variable construction and a failed Insert allocation.
		root, x := rep.scratch()
		if x != nil {
			return nil, x
		}
		variable, x := h.m.MkVar(index, sort)
		if !variable.IsNull() {
			x = errors.Join(x, h.m.SetRef(root, variable))
		}
		if x != nil {
			return nil, x
		}
		index++
		if x = rep.Insert(v, variable); x != nil {
			return nil, x
		}
	}
	if e = rep.ApplyRef(base.Formula); e != nil {
		return nil, e
	}
	for i, j := 0, len(boundNames)-1; i < j; i, j = i+1, j-1 {
		boundNames[i], boundNames[j] = boundNames[j], boundNames[i]
		boundSorts[i], boundSorts[j] = boundSorts[j], boundSorts[i]
	}
	names = append(boundNames, names...)
	sorts = append(boundSorts, sorts...)
	return &QuantifierIndexedResult{QuantifierHoistResult: base, Index: index, Sorts: sorts, Names: names}, nil
}

func (h *QuantifierHoister) Close() error {
	if h == nil || h.self != h {
		return ast.ErrASTHandle
	}
	if h.closed {
		return nil
	}
	if h.bool != nil {
		if err := h.bool.Close(); err != nil {
			return err
		}
	}
	h.closed = true
	return nil
}
