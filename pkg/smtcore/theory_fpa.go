package smtcore

// Source: src/smt/theory_fpa.cpp and src/smt/theory_fpa.h floating-point
// reduction to bit-vectors inside the incremental SMT context.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	modelfpa "github.com/glycerine/g3/pkg/model/fpa"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

type smtFPAConversion struct {
	key   ast.ASTRef
	value ast.ASTRef
}

type smtFPAValueProcedure struct {
	manager      *ast.ASTManager
	dependencies []SMTModelValueDependency
	ebits        uint32
	sbits        uint32
	rounding     bool
}

func (p *smtFPAValueProcedure) Dependencies() []SMTModelValueDependency {
	return append([]SMTModelValueDependency(nil), p.dependencies...)
}

func (*smtFPAValueProcedure) IsFresh() bool { return false }

func (p *smtFPAValueProcedure) MakeValue(generator *SMTModelGenerator, values []ast.ASTHandle) (ast.ASTHandle, error) {
	if p == nil || p.manager == nil || generator == nil || len(values) != len(p.dependencies) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if err := p.traceInputs(generator, values); err != nil {
		return ast.ASTHandle{}, err
	}
	var value ast.ASTHandle
	var err error
	if p.rounding {
		value, err = p.manager.MkFPARoundingModeModelValue(values)
	} else {
		value, err = p.manager.MkFPAModelValue(p.ebits, p.sbits, values)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.traceResult(generator, values, value); err != nil {
		return ast.ASTHandle{}, err
	}
	return value, nil
}

// SMTFloatingPointTheory preserves FP and rounding-mode terms in the e-graph
// and defines every reached atom through the retained FPA-to-BV converter.
type SMTFloatingPointTheory struct {
	*SMTTheoryBase
	converter  *astfpa.FPA2BVConverter
	rewriter   *rewriter.FPA2BVRewriter
	thRewriter *rewriter.TheoryRewriter // m_th_rw with arith_lhs=true
	converted  map[ast.ASTHandle]ast.ASTHandle
	conversion []smtFPAConversion
	scopes     []int
}

func NewSMTFloatingPointTheory(family ast.FamilyID) (*SMTFloatingPointTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	return &SMTFloatingPointTheory{
		SMTTheoryBase: base,
		converted:     make(map[ast.ASTHandle]ast.ASTHandle),
	}, nil
}

func (t *SMTFloatingPointTheory) Initialize(state *SMTInternalizerState) (result error) {
	if t == nil || t.converter != nil || t.rewriter != nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	converter, err := rewriter.NewFPA2BVConverter(state.manager, state.paramsContext, state.globals)
	if err != nil {
		return fmt.Errorf("floating-point converter construction: %w", err)
	}
	t.converter = converter
	// fpa2bv_converter_wrapped overrides mk_const and mk_rm_const.
	converter.SetOverrides(astfpa.FPA2BVOverrides{MkConst: t.wrappedMkConst, MkRMConst: t.wrappedMkRMConst})
	rw, err := rewriter.NewFPA2BVRewriter(state.manager, converter)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return errors.Join(fmt.Errorf("floating-point rewriter construction: %w", err), converter.Close())
	}
	t.rewriter = rw
	// theory_fpa's constructor: p.set_bool("arith_lhs", true) on m_th_rw.
	if err = t.initializeTheoryRewriter(state); err != nil {
		if t.thRewriter != nil {
			err = errors.Join(err, t.thRewriter.Close())
		}
		err = errors.Join(err, t.rewriter.Close(), t.converter.Close())
		t.thRewriter, t.rewriter, t.converter = nil, nil, nil
		return fmt.Errorf("floating-point theory rewriter construction: %w", err)
	}
	return nil
}

func (t *SMTFloatingPointTheory) initializeTheoryRewriter(state *SMTInternalizerState) (result error) {
	local, err := state.paramsContext.NewValue()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, state.paramsContext.Del(&local)) }()
	name := "arith_lhs"
	if err = state.paramsContext.SetBoolName(&local, &name, true); err != nil {
		return err
	}
	t.thRewriter, err = rewriter.NewTheoryRewriter(state.manager, state.paramsContext, state.globals, &local)
	return err
}

func (t *SMTFloatingPointTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := state.manager.GetDeclKind(expression)
	return op, err == nil, err
}

// wrappedMkConst is fpa2bv_converter_wrapped::mk_const.
func (t *SMTFloatingPointTheory) wrappedMkConst(c *astfpa.FPA2BVConverter, f ast.ASTHandle, result ast.ASTRef) error {
	m := c.Manager()
	if r, ok, err := c.Const2BVFind(f); err != nil {
		return err
	} else if ok {
		return m.SetRef(result, r)
	}
	r, err := t.convertFloatingConstant(f)
	if err != nil {
		return err
	}
	if err = m.SetRef(result, r); err != nil {
		return err
	}
	return c.Const2BVInsert(f, r)
}

// wrappedMkRMConst is fpa2bv_converter_wrapped::mk_rm_const.
func (t *SMTFloatingPointTheory) wrappedMkRMConst(c *astfpa.FPA2BVConverter, f ast.ASTHandle, result ast.ASTRef) error {
	m := c.Manager()
	if r, ok, err := c.RMConst2BVFind(f); err != nil {
		return err
	} else if ok {
		return m.SetRef(result, r)
	}
	r, err := t.convertRoundingConstant(f)
	if err != nil {
		return err
	}
	if err = m.SetRef(result, r); err != nil {
		return err
	}
	return c.RMConst2BVInsert(f, r)
}

func (t *SMTFloatingPointTheory) convertFloatingConstant(declaration ast.ASTHandle) (_ ast.ASTHandle, retErr error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(46, "f->get_family_id() == null_family_id", func() (bool, error) {
		family, e := state.manager.GetFamilyID(declaration)
		return family == ast.NullFamilyID, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(47, "f->get_arity() == 0", func() (bool, error) {
		arity, e := state.manager.GetArity(declaration)
		return arity == 0, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	original, err := state.manager.MkApp(declaration, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	f := &smtFPAFrame{m: state.manager}
	defer f.release(&retErr)
	packed, err := t.wrap(f, original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := state.manager.GetSort(original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	width, err := state.manager.BitVectorSize(packed)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssertBool(58, "bv_sz == m_th.m_fpa_util.get_ebits(s) + sbits", width == ebits+sbits); err != nil {
		return ast.ASTHandle{}, err
	}
	if width != ebits+sbits {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	// mk_fp(extract(sgn), extract(exp), extract(sig)): GCC builds the
	// extracts right to left.
	significand, err := state.manager.MkBitVectorExtract(sbits-2, 0, packed)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := state.manager.MkBitVectorExtract(width-2, sbits-1, packed)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sign, err := state.manager.MkBitVectorExtract(width-1, width-1, packed)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := state.manager.MkFamilyApp(t.FamilyID(), ast.FPAOpFP, nil, []ast.ASTHandle{sign, exponent, significand})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(62, "m_th.m_fpa_util.is_float(result)", func() (bool, error) {
		resultSort, e := state.manager.GetSort(result)
		if e != nil {
			return false, e
		}
		_, _, matched, e := state.manager.FloatingPointSortShape(resultSort)
		return matched, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (t *SMTFloatingPointTheory) convertRoundingConstant(declaration ast.ASTHandle) (_ ast.ASTHandle, result error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(70, "f->get_family_id() == null_family_id", func() (bool, error) {
		family, e := state.manager.GetFamilyID(declaration)
		return family == ast.NullFamilyID, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(71, "f->get_arity() == 0", func() (bool, error) {
		arity, e := state.manager.GetArity(declaration)
		return arity == 0, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(77, "is_rm(f->get_range())", func() (bool, error) {
		rangeSort, e := state.manager.GetRange(declaration)
		if e != nil {
			return false, e
		}
		return state.manager.IsRoundingModeSort(rangeSort)
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	original, err := state.manager.MkApp(declaration, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	f := &smtFPAFrame{m: state.manager}
	defer f.release(&result)
	packed, err := t.wrap(f, original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.MkFamilyApp(t.FamilyID(), ast.FPAOpBV2RM, nil, []ast.ASTHandle{packed})
}

func (t *SMTFloatingPointTheory) assertConstraint(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil || state.manager.IsTrue(expression) {
		return err
	}
	if err = t.traceFPAExpression("t_fpa_detail", "assert_cnstr", 410, "asserting ", expression); err != nil {
		return err
	}
	if err = state.Internalize(expression, false); err != nil {
		return err
	}
	literal, err := state.Literal(expression)
	if err != nil {
		return err
	}
	// ctx.mark_as_relevant(lit) marks the literal's atom before mk_th_axiom.
	if literal.Var() != SMTTrueBoolVar {
		atom, atomErr := state.BoolExpression(literal.Var())
		if atomErr != nil {
			return atomErr
		}
		if err = state.markRelevant(atom); err != nil {
			return err
		}
	}
	return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal})
}

func (t *SMTFloatingPointTheory) ensureVariable(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		argument, argumentErr := state.manager.GetArg(expression, i)
		if argumentErr != nil {
			return false, argumentErr
		}
		if argumentErr = state.Internalize(argument, false); argumentErr != nil {
			return false, argumentErr
		}
	}
	node := state.context.enodes.get(expression)
	if node == nil {
		node, err = state.MkEnode(expression, false, false, true)
		if err != nil {
			return false, err
		}
	}
	if t.IsAttachedToVariable(node) {
		return false, nil
	}
	_, err = t.createFPAVariable(node)
	return err == nil, err
}

func (t *SMTFloatingPointTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	if err := t.traceFPAExpression("t_fpa_internalize", "internalize_term", 452, "internalizing term: ", expression); err != nil {
		return false, err
	}
	op, own, err := t.operation(expression)
	if err != nil || !own {
		return false, err
	}
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if err = fpaSourceAssert(453, "term->get_family_id() == get_family_id()", func() (bool, error) {
		family, e := state.manager.GetFamilyID(expression)
		return family == t.FamilyID(), e
	}); err != nil {
		return false, err
	}
	if err = fpaSourceAssertBool(454, "!get_context().e_internalized(term)", state.context.enodes.get(expression) == nil); err != nil {
		return false, err
	}
	created, err := t.ensureVariable(expression)
	if err != nil || !created {
		return true, err
	}
	switch op {
	case ast.FPAOpToFP, ast.FPAOpToUBV, ast.FPAOpToSBV, ast.FPAOpToReal, ast.FPAOpToIEEEBV:
		converted, conversionErr := t.convert(expression)
		if conversionErr != nil {
			return false, conversionErr
		}
		equality, conversionErr := state.manager.MkEq(expression, converted)
		if conversionErr == nil {
			conversionErr = t.assertConstraint(equality)
		}
		if conversionErr == nil {
			conversionErr = t.assertSideConditions()
		}
		return conversionErr == nil, conversionErr
	default:
		return true, nil
	}
}

func (t *SMTFloatingPointTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) (result error) {
	if node == nil {
		return ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	if err = t.traceFPAExpression("t_fpa", "apply_sort_cnstr", 494, "apply sort cnstr for: ", node.Owner()); err != nil {
		return err
	}
	_, _, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil {
		return err
	}
	ownerSort, err := state.manager.GetSort(node.Owner())
	if err != nil {
		return err
	}
	_, _, ownerFloating, err := state.manager.FloatingPointSortShape(ownerSort)
	if err != nil {
		return err
	}
	ownerRounding, err := state.manager.IsRoundingModeSort(ownerSort)
	if err != nil {
		return err
	}
	if err = fpaSourceAssert(495, "s->get_family_id() == get_family_id()", func() (bool, error) {
		family, e := state.manager.GetFamilyID(sort)
		return family == t.FamilyID(), e
	}); err != nil {
		return err
	}
	if err = fpaSourceAssertBool(496, "m_fpa_util.is_float(s) || m_fpa_util.is_rm(s)", floating || rounding); err != nil {
		return err
	}
	if err = fpaSourceAssertBool(497, "m_fpa_util.is_float(n->get_owner()) || m_fpa_util.is_rm(n->get_owner())", ownerFloating || ownerRounding); err != nil {
		return err
	}
	if err = fpaSourceAssertBool(498, "n->get_owner()->get_decl()->get_range() == s", ownerSort == sort); err != nil {
		return err
	}
	if !floating && !rounding || !ownerFloating && !ownerRounding || ownerSort != sort {
		return ast.ErrASTDomain
	}
	if t.IsAttachedToVariable(node) {
		return nil
	}
	if _, err = t.createFPAVariable(node); err != nil {
		return err
	}
	if rounding {
		owner := node.Owner()
		if op, own, operationErr := t.operation(owner); operationErr != nil {
			return operationErr
		} else if !own || op != ast.FPAOpBV2RM {
			// limit = mk_numeral(4, 3); valid = mk_ule(wrap(owner), limit).
			limit, limitErr := state.manager.BitVectorNumeralUint64(4, 3)
			if limitErr != nil {
				return limitErr
			}
			f := &smtFPAFrame{m: state.manager}
			defer f.release(&result)
			packed, wrapErr := t.wrap(f, owner)
			if wrapErr != nil {
				return wrapErr
			}
			valid, validErr := state.manager.MkBitVectorULE(packed, limit)
			if validErr != nil {
				return validErr
			}
			if err = t.assertConstraint(valid); err != nil {
				return err
			}
		}
	}
	if state.relevancy == nil || !state.relevancy.enabled {
		return t.Relevant(node.Owner())
	}
	return nil
}

func (t *SMTFloatingPointTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.scopes = append(t.scopes, len(t.conversion))
	return nil
}

func (t *SMTFloatingPointTheory) PopScopes(count uint32) (result error) {
	if count == 0 || count > uint32(len(t.scopes)) {
		return ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	level := len(t.scopes) - int(count)
	limit := t.scopes[level]
	for i := len(t.conversion) - 1; i >= limit; i-- {
		entry := &t.conversion[i]
		key, keyErr := state.manager.RefGet(entry.key)
		if keyErr == nil {
			delete(t.converted, key)
		}
		if keyErr != nil {
			result = errors.Join(result, fmt.Errorf("floating-point scoped conversion key: %w", keyErr))
		}
		if releaseErr := state.manager.DelRef(&entry.key); releaseErr != nil {
			result = errors.Join(result, fmt.Errorf("floating-point scoped conversion key release: %w", releaseErr))
		}
		if releaseErr := state.manager.DelRef(&entry.value); releaseErr != nil {
			result = errors.Join(result, fmt.Errorf("floating-point scoped conversion value release: %w", releaseErr))
		}
	}
	t.conversion = t.conversion[:limit]
	t.scopes = t.scopes[:level]
	if traceErr := t.traceFPAPop(count, len(t.scopes)); traceErr != nil {
		result = errors.Join(result, traceErr)
	}
	if baseErr := t.SMTTheoryBase.PopScopes(count); baseErr != nil {
		result = errors.Join(result, fmt.Errorf("floating-point base scope pop: %w", baseErr))
	}
	return result
}

func (t *SMTFloatingPointTheory) Close() (result error) {
	if t == nil || t.SMTTheoryBase == nil {
		return ast.ErrASTHandle
	}
	state, stateErr := t.State()
	if stateErr == nil {
		for i := range t.conversion {
			if err := state.manager.DelRef(&t.conversion[i].key); err != nil {
				result = errors.Join(result, fmt.Errorf("floating-point conversion key release: %w", err))
			}
			if err := state.manager.DelRef(&t.conversion[i].value); err != nil {
				result = errors.Join(result, fmt.Errorf("floating-point conversion value release: %w", err))
			}
		}
	} else {
		result = fmt.Errorf("floating-point state close: %w", stateErr)
	}
	if t.thRewriter != nil {
		if err := t.thRewriter.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("floating-point theory rewriter close: %w", err))
		}
	}
	if t.rewriter != nil {
		if err := t.rewriter.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("floating-point rewriter close: %w", err))
		}
	}
	if t.converter != nil {
		if err := t.converter.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("floating-point converter close: %w", err))
		}
	}
	t.converted = nil
	t.conversion = nil
	t.scopes = nil
	t.rewriter = nil
	t.thRewriter = nil
	t.converter = nil
	if err := fpaSourceAssertBool(118, "m_trail_stack.get_num_scopes() == 0", len(t.scopes) == 0); err != nil {
		result = errors.Join(result, err)
	}
	if err := fpaSourceAssertBool(119, "m_conversions.empty()", len(t.conversion) == 0); err != nil {
		result = errors.Join(result, err)
	}
	if err := fpaSourceAssertBool(120, "m_is_added_to_model.empty()", true); err != nil {
		result = errors.Join(result, err)
	}
	return result
}

func (t *SMTFloatingPointTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.manager != state.manager || generator.model == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	if err = t.traceFPAInitializeModel(generator); err != nil {
		return err
	}
	factory, err := protomodel.NewFloatingPointFactory(generator.manager, t.FamilyID())
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

func smtFPAEnodeDependency(node *SMTEnode) ([]SMTModelValueDependency, error) {
	dependency, err := NewSMTEnodeValueDependency(node)
	if err != nil {
		return nil, err
	}
	return []SMTModelValueDependency{dependency}, nil
}

func (t *SMTFloatingPointTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (_ SMTModelValueProcedure, result error) {
	if node == nil || generator == nil || generator.model == nil || !t.IsAttachedToVariable(node) {
		return nil, ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil || generator.manager != state.manager {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	owner := node.Owner()
	if err = fpaSourceAssertBool(739, "ctx.e_internalized(owner)", state.context.enodes.get(owner) != nil); err != nil {
		return nil, err
	}
	sort, err := state.manager.GetSort(owner)
	if err != nil {
		return nil, err
	}
	if err = t.traceFPAModelValue(generator, owner, sort); err != nil {
		return nil, err
	}
	if value, valueErr := state.manager.IsValue(owner); valueErr != nil {
		return nil, valueErr
	} else if value {
		return NewSMTExpressionValueProcedure(owner)
	}
	ebits, sbits, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return nil, err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil || (!floating && !rounding) {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	op, own, err := t.operation(owner)
	if err != nil {
		return nil, err
	}
	procedure := &smtFPAValueProcedure{manager: state.manager, ebits: ebits, sbits: sbits, rounding: rounding}
	if own && op == ast.FPAOpFP {
		if err = fpaSourceAssert(759, "to_app(owner)->get_num_args() == 3", func() (bool, error) {
			count, e := state.manager.GetNumArgs(owner)
			return count == 3, e
		}); err != nil {
			return nil, err
		}
		procedure.dependencies = make([]SMTModelValueDependency, 3)
		nodes := make([]*SMTEnode, 3)
		for i := range procedure.dependencies {
			argument, argumentErr := state.manager.GetArg(owner, uint32(i))
			if argumentErr != nil {
				return nil, argumentErr
			}
			argumentNode := state.context.enodes.get(argument)
			if argumentNode == nil {
				return nil, ast.ErrASTDomain
			}
			nodes[i] = argumentNode
			procedure.dependencies[i], argumentErr = NewSMTEnodeValueDependency(argumentNode)
			if argumentErr != nil {
				return nil, argumentErr
			}
		}
		if err = t.traceFPADependencies(generator, 770, nodes...); err != nil {
			return nil, err
		}
		if err = fpaSourceAssertBool(808, "res != 0", procedure != nil); err != nil {
			return nil, err
		}
		return procedure, nil
	}
	if own && op == ast.FPAOpBV2RM {
		if err = fpaSourceAssert(777, "to_app(owner)->get_num_args() == 1", func() (bool, error) {
			count, e := state.manager.GetNumArgs(owner)
			return count == 1, e
		}); err != nil {
			return nil, err
		}
		argument, argumentErr := state.manager.GetArg(owner, 0)
		if argumentErr != nil {
			return nil, argumentErr
		}
		argumentNode := state.context.enodes.get(argument)
		if argumentNode == nil {
			return nil, ast.ErrASTDomain
		}
		procedure.dependencies, err = smtFPAEnodeDependency(argumentNode)
		if err == nil {
			err = t.traceFPADependencies(generator, 782, argumentNode)
		}
		if err == nil {
			err = fpaSourceAssertBool(808, "res != 0", procedure != nil)
		}
		return procedure, err
	}
	f := &smtFPAFrame{m: state.manager}
	defer f.release(&result)
	wrapped, err := t.wrap(f, owner)
	if err != nil {
		return nil, err
	}
	if err = fpaSourceAssert(750, "m_bv_util.is_bv(wrapped)", func() (bool, error) {
		_, e := state.manager.BitVectorSize(wrapped)
		return e == nil, nil
	}); err != nil {
		return nil, err
	}
	if wrappedNode := state.context.enodes.get(wrapped); wrappedNode != nil {
		procedure.dependencies, err = smtFPAEnodeDependency(wrappedNode)
		if err == nil {
			err = t.traceFPADependencies(generator, 798, wrappedNode)
		}
		if err == nil {
			err = fpaSourceAssertBool(808, "res != 0", procedure != nil)
		}
		return procedure, err
	}
	if err = t.traceFPAMissingDependency(generator, wrapped, state.context.enodes.get(owner) != nil); err != nil {
		return nil, err
	}
	if rounding {
		fallback, fallbackErr := state.manager.MkFamilyApp(t.FamilyID(), ast.FPAOpRMTowardZero, nil, nil)
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		return NewSMTExpressionValueProcedure(fallback)
	}
	fallback, err := state.manager.MkFPAZeroOfSort(sort, false)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(fallback)
}

func (t *SMTFloatingPointTheory) FinalizeModel(generator *SMTModelGenerator) (result error) {
	state, err := t.State()
	if err != nil || generator == nil || generator.manager != state.manager || generator.model == nil || t.converter == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	converter, err := modelfpa.NewConverter(state.manager, t.converter)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, converter.Close()) }()
	target, err := model.NewModelCore(state.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, target.Close()) }()
	source := generator.model.Core()
	seen, err := converter.ConvertFunctions(source, target)
	if err != nil {
		return err
	}
	for declaration := range seen {
		if err = source.UnregisterDeclaration(declaration); err != nil {
			return err
		}
	}
	constantCount, err := target.NumConstants()
	if err != nil {
		return err
	}
	for index := uint32(0); index < constantCount; index++ {
		declaration, copyErr := target.Constant(index)
		if copyErr != nil {
			return copyErr
		}
		value, copyErr := target.ConstantInterpretation(declaration)
		if copyErr != nil {
			return copyErr
		}
		if copyErr = source.RegisterConstant(declaration, value); copyErr != nil {
			return copyErr
		}
	}
	functionCount := target.NumFunctionDeclarations()
	for index := uint32(0); index < functionCount; index++ {
		declaration, copyErr := target.Function(index)
		if copyErr != nil {
			return copyErr
		}
		interpretation, copyErr := target.FunctionInterpretation(declaration)
		if copyErr != nil || interpretation == nil {
			if copyErr != nil {
				return copyErr
			}
			return ast.ErrASTDomain
		}
		copy, copyErr := interpretation.Copy()
		if copyErr != nil {
			return copyErr
		}
		if copyErr = source.RegisterFunction(declaration, copy); copyErr != nil {
			_ = copy.Close()
			return copyErr
		}
	}
	return nil
}

var _ SMTTheoryInternalizer = (*SMTFloatingPointTheory)(nil)
var _ SMTModelValueTheory = (*SMTFloatingPointTheory)(nil)
var _ SMTModelFinalizer = (*SMTFloatingPointTheory)(nil)
