package rewriter

// Source: src/ast/rewriter/arith_rewriter_params.hpp:5-42.

import uparams "github.com/glycerine/g3/pkg/util/params"

// ArithRewriterParams is the concrete generated parameter facade. It borrows
// the local params_ref and owns the constructor-time rewriter module snapshot.
type ArithRewriterParams struct {
	self              *ArithRewriterParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

var (
	settingArithLHS                 = rewriterBoolSetting{"arith_lhs", false}
	settingArithIneqLHS             = rewriterBoolSetting{"arith_ineq_lhs", false}
	settingGCDRounding              = rewriterBoolSetting{"gcd_rounding", false}
	settingElimToReal               = rewriterBoolSetting{"elim_to_real", false}
	settingPushToReal               = rewriterBoolSetting{"push_to_real", true}
	settingAlgebraicNumberEvaluator = rewriterBoolSetting{"algebraic_number_evaluator", true}
	settingExpandPower              = rewriterBoolSetting{"expand_power", false}
	settingMulToPower               = rewriterBoolSetting{"mul_to_power", false}
	settingElimRem                  = rewriterBoolSetting{"elim_rem", false}
	settingExpandTan                = rewriterBoolSetting{"expand_tan", false}
	settingEq2Ineq                  = rewriterBoolSetting{"eq2ineq", false}
	settingSortSums                 = rewriterBoolSetting{"sort_sums", false}
	settingMaxDegree                = rewriterUIntSetting{"max_degree", 64}
)

func NewArithRewriterParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*ArithRewriterParams, error) {
	if c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	p := &ArithRewriterParams{context: c}
	p.self = p
	if local == nil {
		p.empty = c.EmptyValue()
		local = &p.empty
	}
	if e := c.Check(local); e != nil {
		return nil, e
	}
	p.local = local
	var e error
	p.global, e = g.SourceGetModuleName("rewriter")
	if e != nil {
		return nil, e
	}
	return p, nil
}

func (p *ArithRewriterParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *ArithRewriterParams) boolean(key string, d bool) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, d)
}

func (p *ArithRewriterParams) AlgebraicNumberEvaluator() (bool, error) {
	return p.boolean(settingAlgebraicNumberEvaluator.name, settingAlgebraicNumberEvaluator.defaultValue)
}
func (p *ArithRewriterParams) MulToPower() (bool, error) {
	return p.boolean(settingMulToPower.name, settingMulToPower.defaultValue)
}
func (p *ArithRewriterParams) ExpandPower() (bool, error) {
	return p.boolean(settingExpandPower.name, settingExpandPower.defaultValue)
}
func (p *ArithRewriterParams) ExpandTan() (bool, error) {
	return p.boolean(settingExpandTan.name, settingExpandTan.defaultValue)
}
func (p *ArithRewriterParams) SortSums() (bool, error) {
	return p.boolean(settingSortSums.name, settingSortSums.defaultValue)
}
func (p *ArithRewriterParams) GCDRounding() (bool, error) {
	return p.boolean(settingGCDRounding.name, settingGCDRounding.defaultValue)
}
func (p *ArithRewriterParams) ArithLHS() (bool, error) {
	return p.boolean(settingArithLHS.name, settingArithLHS.defaultValue)
}
func (p *ArithRewriterParams) ArithIneqLHS() (bool, error) {
	return p.boolean(settingArithIneqLHS.name, settingArithIneqLHS.defaultValue)
}
func (p *ArithRewriterParams) ElimToReal() (bool, error) {
	return p.boolean(settingElimToReal.name, settingElimToReal.defaultValue)
}
func (p *ArithRewriterParams) PushToReal() (bool, error) {
	return p.boolean(settingPushToReal.name, settingPushToReal.defaultValue)
}
func (p *ArithRewriterParams) Eq2Ineq() (bool, error) {
	return p.boolean(settingEq2Ineq.name, settingEq2Ineq.defaultValue)
}
func (p *ArithRewriterParams) ElimRem() (bool, error) {
	return p.boolean(settingElimRem.name, settingElimRem.defaultValue)
}

func (p *ArithRewriterParams) MaxDegree() (uint32, error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	key := "max_degree"
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, 64)
}

func (p *ArithRewriterParams) Close() error {
	if p == nil || p.self != p {
		return uparams.ErrParamsOwner
	}
	if p.closed {
		return nil
	}
	if p.destructorStarted {
		if e := p.context.CompleteDecrement(&p.global, p.decrement); e != nil {
			return e
		}
	} else {
		p.destructorStarted = true
		if e := p.context.DelTracked(&p.global, &p.decrement); e != nil {
			return e
		}
	}
	p.closed = true
	return nil
}
