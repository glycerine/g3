package tactic

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Bound the cost of an unsuccessful attempt before the ordinary QF_NRA portfolio.
// All limits affect witness discovery only.
const (
	minCandidateAssertions  = 32
	maxCandidateAssertions  = 1 << 17
	maxCandidateEdges       = 1 << 20
	maxCandidateDepth       = 128
	maxCandidateIntegerBits = 512
)

func (t *NLSATTactic) tryIntegerModel(goal *basetactic.Goal, result *basetactic.TacticResult) (bool, error) {
	m := t.manager
	n, err := goal.Size()
	if err != nil || n < minCandidateAssertions || n > maxCandidateAssertions || goal.IsDecided() {
		return false, err
	}
	if m.Limit.Limit != 0 || m.Limit.CancelFlag() || m.Budget().InjectionArmed() ||
		t.services.Memory.AllocationLimited() || m.TraceStream != nil || diagnostic.TraceCompiled() || ast.ASTSourceDebug {
		return false, nil
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil || proofs {
		return false, err
	}
	proofs, err = m.ProofsEnabled()
	if err != nil || proofs {
		return false, err
	}
	precision, err := goal.Precision()
	if err != nil || precision != basetactic.GoalPrecise {
		return false, err
	}
	verbose, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || verbose {
		return false, err
	}
	cooperating, err := t.services.Cooperation.Enabled()
	if err != nil || cooperating {
		return false, err
	}
	limited, err := t.integerModelResourceLimited()
	if err != nil || limited {
		return false, err
	}
	forms, err := goal.AppendFormulas(nil)
	if err != nil {
		return false, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	p := integerCandidate{manager: m, arithmetic: arithmetic, terms: make(map[ast.ASTHandle]candidateTerm)}
	p.comparisons = make([]candidateComparison, len(forms))
	for i, form := range forms {
		if i%64 == 0 && !m.Limit.Inc() {
			return false, nil
		}
		if _, ok, err := p.scan(form, 0); err != nil || !ok {
			return false, err
		}
		if err := p.constrain(form, &p.comparisons[i]); err != nil {
			return false, err
		}
	}
	if !p.nonlinear || len(p.variables) < 2 {
		return false, nil
	}
	// Seed the magnitude with a small non-unit value. If a mixed polynomial
	// needs a larger magnitude, try a point beyond its small coefficients.
	for attempt, seed := range [...]int64{2, p.largest + 1} {
		if attempt != 0 && (seed <= 2 || seed > 1<<16) {
			break
		}
		feasible, changed := p.assign(seed)
		if !feasible || !changed || m.Limit.CancelFlag() {
			return false, nil
		}
		ok, err := t.checkIntegerModel(forms, &p, goal, result)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

type candidateNumeral struct {
	value   int64
	integer bool
}

func (t *NLSATTactic) checkIntegerModel(forms []ast.ASTHandle, p *integerCandidate, goal *basetactic.Goal, result *basetactic.TacticResult) (_ bool, err error) {
	m := t.manager
	var remaining []ast.ASTHandle
	for i, cmp := range p.comparisons {
		if i%64 == 0 && !m.Limit.Inc() {
			return false, nil
		}
		yes, known := cmp.satisfied(p.variables)
		if known && !yes {
			return false, nil
		}
		if !known {
			remaining = append(remaining, forms[i])
		}
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return false, err
	}
	if len(remaining) == 0 && !models {
		if m.Limit.CancelFlag() {
			return false, nil
		}
		return finishIntegerCandidate(goal, result)
	}
	md, err := model.NewModel(m)
	if err != nil {
		return false, err
	}
	var held basetactic.ModelRef
	if err = held.Set(md.Core()); err != nil {
		return false, errors.Join(err, md.Close())
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	numerals := make(map[candidateNumeral]ast.ASTHandle)
	for i := range p.variables {
		v := &p.variables[i]
		if i%64 == 0 && !m.Limit.Inc() {
			return false, nil
		}
		h, ok := numerals[v.numeral]
		if !ok {
			h, err = m.ArithmeticNumeralInt64(v.numeral.value, v.numeral.integer)
			if err != nil {
				return false, err
			}
			numerals[v.numeral] = h
		}
		if err = md.Core().RegisterConstant(v.declaration, h); err != nil {
			return false, err
		}
	}
	if len(remaining) > 0 {
		valid, err := t.evaluateIntegerCandidate(md, remaining)
		if err != nil || !valid {
			return false, err
		}
	}
	if m.Limit.CancelFlag() {
		return false, nil
	}
	if models {
		converter, err := basetactic.NewStoredModel(md.Core())
		if err != nil {
			return false, err
		}
		if err = result.SetModelConverter(converter); err != nil {
			return false, errors.Join(err, converter.Close())
		}
	}
	return finishIntegerCandidate(goal, result)
}

func finishIntegerCandidate(goal *basetactic.Goal, result *basetactic.TacticResult) (bool, error) {
	if err := goal.Reset(); err != nil {
		return false, err
	}
	if err := goal.IncDepth(); err != nil {
		return false, err
	}
	return true, result.AddGoal(goal)
}

// Use the existing exact evaluator for Boolean combinations and polynomial
// expressions outside the small direct-comparison fragment.
func (t *NLSATTactic) evaluateIntegerCandidate(md *model.Model, forms []ast.ASTHandle) (_ bool, err error) {
	m := t.manager
	ev, err := model.NewModelEvaluator(md.Core(), t.services)
	if ev != nil {
		defer func() { err = errors.Join(err, ev.Close()) }()
	}
	if err != nil {
		return false, err
	}
	if err = ev.SetModelCompletion(false); err != nil {
		return false, err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&out)) }()
	// A late constraint often rejects a proposed model. Try it first.
	for j := len(forms) - 1; j >= 0; j-- {
		if !m.Limit.Inc() {
			return false, nil
		}
		if err = ev.Evaluate(forms[j], out); err != nil {
			if rewriter.IsRewriterException(err) {
				return false, nil
			}
			return false, err
		}
		h, err := m.RefGet(out)
		if err != nil || !m.IsTrue(h) {
			return false, err
		}
	}
	return true, nil
}

func (t *NLSATTactic) integerModelResourceLimited() (_ bool, err error) {
	global, err := t.globals.SourceGetModuleName("nlsat")
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, t.context.Del(&global)) }()
	for _, name := range [...]string{"max_memory", "max_conflicts"} {
		limit, err := t.context.GetUIntFallbackName(&t.params, &name, &global, math.MaxUint32)
		if err != nil || limit != math.MaxUint32 {
			return true, err
		}
	}
	return false, nil
}
