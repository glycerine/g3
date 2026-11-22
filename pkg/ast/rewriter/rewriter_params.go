package rewriter

// Source: src/ast/rewriter/rewriter_params.hpp:5-37.

import uparams "github.com/glycerine/g3/pkg/util/params"

// RewriterParams is the concrete generated parameter facade. It borrows the
// caller's local parameters and owns the constructor-time rewriter snapshot.
type RewriterParams struct {
	self              *RewriterParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

// Each setting supplies both the generated facade and the small-list reader.
type rewriterBoolSetting struct {
	name         string
	defaultValue bool
}
type rewriterUIntSetting struct {
	name         string
	defaultValue uint32
}

func (s rewriterBoolSetting) read(p *uparams.ScalarParams) bool {
	return p.Bool(s.name, s.defaultValue)
}
func (s rewriterUIntSetting) read(p *uparams.ScalarParams) uint32 {
	return p.UInt(s.name, s.defaultValue)
}

var (
	settingFlat                        = rewriterBoolSetting{"flat", true}
	settingSumOfMonomials              = rewriterBoolSetting{"som", false}
	settingSumOfMonomialsBlowup        = rewriterUIntSetting{"som_blowup", ^uint32(0)}
	settingHoistMultiplication         = rewriterBoolSetting{"hoist_mul", false}
	settingHoistCoefficients           = rewriterBoolSetting{"hoist_cmul", false}
	settingMaxMemory                   = rewriterUIntSetting{"max_memory", ^uint32(0)}
	settingMaxSteps                    = rewriterUIntSetting{"max_steps", ^uint32(0)}
	settingPullCheapITE                = rewriterBoolSetting{"pull_cheap_ite", false}
	settingCacheAll                    = rewriterBoolSetting{"cache_all", false}
	settingPushITEArith                = rewriterBoolSetting{"push_ite_arith", false}
	settingPushITEBV                   = rewriterBoolSetting{"push_ite_bv", false}
	settingIgnorePatternsOnGroundQBody = rewriterBoolSetting{"ignore_patterns_on_ground_qbody", true}
	settingRewritePatterns             = rewriterBoolSetting{"rewrite_patterns", false}
)

func NewRewriterParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*RewriterParams, error) {
	if c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	p := &RewriterParams{context: c}
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

func (p *RewriterParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *RewriterParams) boolean(key string, d bool) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, d)
}

func (p *RewriterParams) unsigned(key string, d uint32) (uint32, error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, d)
}

func (p *RewriterParams) MaxMemory() (uint32, error) {
	return p.unsigned(settingMaxMemory.name, settingMaxMemory.defaultValue)
}
func (p *RewriterParams) MaxSteps() (uint32, error) {
	return p.unsigned(settingMaxSteps.name, settingMaxSteps.defaultValue)
}
func (p *RewriterParams) Flat() (bool, error) {
	return p.boolean(settingFlat.name, settingFlat.defaultValue)
}
func (p *RewriterParams) SumOfMonomials() (bool, error) {
	return p.boolean(settingSumOfMonomials.name, settingSumOfMonomials.defaultValue)
}
func (p *RewriterParams) SumOfMonomialsBlowup() (uint32, error) {
	return p.unsigned(settingSumOfMonomialsBlowup.name, settingSumOfMonomialsBlowup.defaultValue)
}
func (p *RewriterParams) HoistMultiplication() (bool, error) {
	return p.boolean(settingHoistMultiplication.name, settingHoistMultiplication.defaultValue)
}
func (p *RewriterParams) HoistCoefficients() (bool, error) {
	return p.boolean(settingHoistCoefficients.name, settingHoistCoefficients.defaultValue)
}
func (p *RewriterParams) PushITEArith() (bool, error) {
	return p.boolean(settingPushITEArith.name, settingPushITEArith.defaultValue)
}
func (p *RewriterParams) PushITEBV() (bool, error) {
	return p.boolean(settingPushITEBV.name, settingPushITEBV.defaultValue)
}
func (p *RewriterParams) PullCheapITE() (bool, error) {
	return p.boolean(settingPullCheapITE.name, settingPullCheapITE.defaultValue)
}
func (p *RewriterParams) BVIneqConsistencyTestMax() (uint32, error) {
	return p.unsigned("bv_ineq_consistency_test_max", 0)
}
func (p *RewriterParams) CacheAll() (bool, error) {
	return p.boolean(settingCacheAll.name, settingCacheAll.defaultValue)
}
func (p *RewriterParams) RewritePatterns() (bool, error) {
	return p.boolean(settingRewritePatterns.name, settingRewritePatterns.defaultValue)
}
func (p *RewriterParams) IgnorePatternsOnGroundQBody() (bool, error) {
	return p.boolean(settingIgnorePatternsOnGroundQBody.name, settingIgnorePatternsOnGroundQBody.defaultValue)
}

func (p *RewriterParams) Close() error {
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
