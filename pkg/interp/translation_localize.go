package interp

// Source checkpoint: src/interp/iz3translate.cpp fork fix for mixed
// rewrite and conjunction-elimination steps (localize_rewrite,
// localize_elim and their helpers).
//
// z3 4.7.1's asserted_formulas::propagate_values substitutes a constant
// of one frame by an equal constant of another, so a frame's assertion
// is simplified into formulas that mix the A and B vocabularies. A
// rewrite L = R of such a formula cannot be assumed in any frame, and
// translate_main rejected it as an unsupported rule; an and-elim from
// such a conjunction went through make_axiom's localization and gave
// quantified interpolants. Both steps are valid under any renaming of
// constants, so the fork maps the substituted constants back through
// the equalities the proof used (theta), takes the step locally on
// theta(L), and moves between L and theta(L) by congruence.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// rewriteEquivalence is the source's ConstEquiv: the equation
// pre = image and its proof.
type rewriteEquivalence struct {
	equation ast.ASTHandle
	proof    ast.ASTHandle
}

// rewriteLocalizer holds one localize_rewrite or localize_elim call's
// theta map, the memo of apply_rewrite_equivs and the references the
// source's ast wrappers keep until the call returns.
type rewriteLocalizer struct {
	t       *FullTranslation
	visited map[ast.ASTHandle]bool
	theta   map[ast.ASTHandle]rewriteEquivalence
	keys    []ast.ASTHandle
	applied map[ast.ASTHandle]ast.ASTHandle
	pins    []ast.ASTRef
}

func (t *FullTranslation) newRewriteLocalizer() *rewriteLocalizer {
	return &rewriteLocalizer{
		t:       t,
		visited: make(map[ast.ASTHandle]bool),
		theta:   make(map[ast.ASTHandle]rewriteEquivalence),
		applied: make(map[ast.ASTHandle]ast.ASTHandle),
	}
}

func (l *rewriteLocalizer) pin(h ast.ASTHandle) error {
	ref, err := l.t.base.manager.ast.NewRef(h)
	if err != nil {
		return err
	}
	l.pins = append(l.pins, ref)
	return nil
}

func (l *rewriteLocalizer) release() (result error) {
	for i := len(l.pins) - 1; i >= 0; i-- {
		result = errors.Join(result, l.t.base.manager.ast.DelRef(&l.pins[i]))
	}
	l.pins = nil
	return result
}

// isRewriteConstant is is_rewrite_constant: an uninterpreted constant.
func (l *rewriteLocalizer) isRewriteConstant(term ast.ASTHandle) (bool, error) {
	op, err := l.t.base.manager.Operator(term)
	if err != nil || op != OperatorUninterpreted {
		return false, err
	}
	count, err := l.t.base.manager.NumArgs(term)
	return count == 0, err
}

// collect is collect_rewrite_equivs: record each equation between two
// distinct constants, the first proof of an image winning, and descend
// through the congruence, transitivity, modus-ponens and elimination
// steps that carry them.
func (l *rewriteLocalizer) collect(proof ast.ASTHandle) error {
	if l.visited[proof] {
		return nil
	}
	l.visited[proof] = true
	m := l.t.base.manager
	conclusion, err := m.Conclusion(proof)
	if err != nil {
		return err
	}
	op, err := m.Operator(conclusion)
	if err != nil {
		return err
	}
	if op == OperatorEqual || op == OperatorIff {
		pre, e := m.Arg(conclusion, 0)
		if e != nil {
			return e
		}
		image, e := m.Arg(conclusion, 1)
		if e != nil {
			return e
		}
		if pre != image {
			preConstant, e := l.isRewriteConstant(pre)
			if e != nil {
				return e
			}
			imageConstant, e := l.isRewriteConstant(image)
			if e != nil {
				return e
			}
			if preConstant && imageConstant {
				if _, found := l.theta[image]; !found {
					l.theta[image] = rewriteEquivalence{equation: conclusion, proof: proof}
					l.keys = append(l.keys, image)
				}
				return nil
			}
		}
	}
	rule, err := m.ProofRule(proof)
	if err != nil {
		return err
	}
	switch rule {
	case ast.ASTPrMonotonicity, ast.ASTPrTransitivity, ast.ASTPrModusPonens, ast.ASTPrAndElim, ast.ASTPrNotOrElim:
		count, e := m.NumPremises(proof)
		if e != nil {
			return e
		}
		for i := uint32(0); i < count; i++ {
			premise, e := m.Premise(proof, i)
			if e != nil {
				return e
			}
			if e = l.collect(premise); e != nil {
				return e
			}
		}
	}
	return nil
}

// apply is apply_rewrite_equivs: replace each image constant by its
// preimage; quantifiers are left alone.
func (l *rewriteLocalizer) apply(term ast.ASTHandle) (ast.ASTHandle, error) {
	if result, found := l.applied[term]; found {
		return result, nil
	}
	m := l.t.base.manager
	var result ast.ASTHandle
	if equivalence, found := l.theta[term]; found {
		pre, err := m.Arg(equivalence.equation, 0)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result = pre
	} else {
		op, err := m.Operator(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if op == OperatorForall || op == OperatorExists {
			result = term
		} else {
			args, err := m.Args(term)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			for i := range args {
				if args[i], err = l.apply(args[i]); err != nil {
					return ast.ASTHandle{}, err
				}
			}
			if result, err = m.Clone(term, args); err != nil {
				return ast.ASTHandle{}, err
			}
		}
	}
	if err := l.pin(result); err != nil {
		return ast.ASTHandle{}, err
	}
	l.applied[term] = result
	return result, nil
}

// makeEquivalence is make_rewrite_equiv: Iff on Booleans, else Equal.
func (l *rewriteLocalizer) makeEquivalence(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	m := l.t.base.manager
	sort, err := m.Type(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := m.IsBoolType(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if boolean {
		return m.Make(OperatorIff, x, y)
	}
	return m.Make(OperatorEqual, x, y)
}

// prove is prove_rewrite_equivs: a node for term = theta(term) when
// forward, else theta(term) = term.
func (l *rewriteLocalizer) prove(term ast.ASTHandle, forward bool, direct *DirectProofInterpolator, memo *translationMemo) (ast.ASTHandle, error) {
	m := l.t.base.manager
	image, err := l.apply(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if equivalence, found := l.theta[term]; found {
		premiseConclusion := equivalence.equation // theta(term) = term
		proof, e := l.t.translateMain(equivalence.proof, false, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !forward {
			return proof, nil
		}
		conclusion, e := l.makeEquivalence(term, image)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return direct.MakeSymmetry(conclusion, premiseConclusion, proof)
	}
	args, err := m.Args(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var equivalences, premises []ast.ASTHandle
	for _, argument := range args {
		mapped, e := l.apply(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if mapped == argument {
			continue
		}
		var equivalence ast.ASTHandle
		if forward {
			equivalence, e = l.makeEquivalence(argument, mapped)
		} else {
			equivalence, e = l.makeEquivalence(mapped, argument)
		}
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = l.pin(equivalence); e != nil {
			return ast.ASTHandle{}, e
		}
		equivalences = append(equivalences, equivalence)
		premise, e := l.prove(argument, forward, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = l.pin(premise); e != nil {
			return ast.ASTHandle{}, e
		}
		premises = append(premises, premise)
	}
	var conclusion ast.ASTHandle
	if forward {
		conclusion, err = l.makeEquivalence(term, image)
	} else {
		conclusion, err = l.makeEquivalence(image, term)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return direct.MakeCongruences(equivalences, conclusion, premises)
}

// localizeRewriteWith is localize_rewrite_with: prove the mixed rewrite
// L = R from the local rewrite theta(L) = theta(R) assumed in its frame.
// Like the source's amemo, the applied memo and its references last for
// this call only.
func (t *FullTranslation) localizeRewriteWith(l *rewriteLocalizer, rewrite ast.ASTHandle, direct *DirectProofInterpolator, memo *translationMemo) (result ast.ASTHandle, ok bool, resultErr error) {
	l.applied = make(map[ast.ASTHandle]ast.ASTHandle)
	defer func() {
		resultErr = errors.Join(resultErr, l.release())
	}()
	m := t.base.manager
	conclusion, err := m.Conclusion(rewrite)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	lhs, err := m.Arg(conclusion, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	rhs, err := m.Arg(conclusion, 1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	localLHS, err := l.apply(lhs)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	localRHS, err := l.apply(rhs)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	op, err := m.Operator(conclusion)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	localConclusion, err := m.Make(op, localLHS, localRHS)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = l.pin(localConclusion); err != nil {
		return ast.ASTHandle{}, false, err
	}
	scope, err := t.base.ASTScope(localConclusion)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if scope.IsEmpty() {
		return ast.ASTHandle{}, false, nil
	}
	frame := scope.Max()
	if frame >= t.frames {
		frame = t.frames - 1
	}
	localRewrite, err := direct.MakeAssumption(frame, []ast.ASTHandle{localConclusion})
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = l.pin(localRewrite); err != nil {
		return ast.ASTHandle{}, false, err
	}
	first := localRewrite
	if localLHS != lhs {
		proof, e := l.prove(lhs, true, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if first, e = direct.MakeTransitivity(lhs, localLHS, localRHS, proof, localRewrite); e != nil {
			return ast.ASTHandle{}, false, e
		}
		if e = l.pin(first); e != nil {
			return ast.ASTHandle{}, false, e
		}
	}
	result = first
	if localRHS != rhs {
		proof, e := l.prove(rhs, false, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if result, e = direct.MakeTransitivity(lhs, localRHS, rhs, first, proof); e != nil {
			return ast.ASTHandle{}, false, e
		}
	}
	return result, true, nil
}

// collectGlobal is collect_global_equivs: the hypothesis-free equations
// between constants of the whole proof, depth first, the first proof of
// an image winning.
func (t *FullTranslation) collectGlobal(proof ast.ASTHandle, visited map[ast.ASTHandle]bool) error {
	if visited[proof] {
		return nil
	}
	visited[proof] = true
	m := t.base.manager
	conclusion, err := m.Conclusion(proof)
	if err != nil {
		return err
	}
	op, err := m.Operator(conclusion)
	if err != nil {
		return err
	}
	if op == OperatorEqual || op == OperatorIff {
		pre, e := m.Arg(conclusion, 0)
		if e != nil {
			return e
		}
		image, e := m.Arg(conclusion, 1)
		if e != nil {
			return e
		}
		if pre != image {
			l := rewriteLocalizer{t: t}
			preConstant, e := l.isRewriteConstant(pre)
			if e != nil {
				return e
			}
			imageConstant, e := l.isRewriteConstant(image)
			if e != nil {
				return e
			}
			if preConstant && imageConstant {
				if _, found := t.globalTheta[image]; !found {
					hypotheses, e := t.proofHypotheses(proof)
					if e != nil {
						return e
					}
					if len(hypotheses) == 0 {
						t.globalTheta[image] = rewriteEquivalence{equation: conclusion, proof: proof}
						t.globalKeys = append(t.globalKeys, image)
					}
				}
			}
		}
	}
	count, err := m.NumPremises(proof)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		premise, e := m.Premise(proof, i)
		if e != nil {
			return e
		}
		if e = t.collectGlobal(premise, visited); e != nil {
			return e
		}
	}
	return nil
}

// proofBusy is proof_busy: whether translating proof could re-enter a
// translation in progress (one whose memo entry is still unset).
func (t *FullTranslation) proofBusy(proof ast.ASTHandle, memo *translationMemo, seen map[ast.ASTHandle]bool) (bool, error) {
	if result, found := seen[proof]; found {
		return result, nil
	}
	seen[proof] = false
	result := memo.clauseBusy[proof] || memo.termBusy[proof]
	if !result {
		count, err := t.base.manager.NumPremises(proof)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < count && !result; i++ {
			premise, e := t.base.manager.Premise(proof, i)
			if e != nil {
				return false, e
			}
			busy, e := t.proofBusy(premise, memo, seen)
			if e != nil {
				return false, e
			}
			result = busy
		}
	}
	seen[proof] = result
	return result, nil
}

// localizeRewrite is localize_rewrite: theta from the congruence proof
// congruence; when that leaves the rewrite mixed (the congruence's left
// side was already mixed by an earlier propagate_values round), the
// proof's other hypothesis-free constant equations are added.
func (t *FullTranslation) localizeRewrite(congruence, rewrite ast.ASTHandle, direct *DirectProofInterpolator, memo *translationMemo) (ast.ASTHandle, bool, error) {
	l := t.newRewriteLocalizer()
	if err := l.collect(congruence); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if len(l.keys) != 0 {
		result, ok, err := t.localizeRewriteWith(l, rewrite, direct, memo)
		if err != nil || ok {
			return result, ok, err
		}
	}
	if !t.globalDone {
		if err := t.collectGlobal(t.translateRoot, make(map[ast.ASTHandle]bool)); err != nil {
			return ast.ASTHandle{}, false, err
		}
		t.globalDone = true
	}
	count := len(l.keys)
	busy := make(map[ast.ASTHandle]bool)
	for _, image := range t.globalKeys {
		if _, found := l.theta[image]; found {
			continue
		}
		equivalence := t.globalTheta[image]
		taken, err := t.proofBusy(equivalence.proof, memo, busy)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if !taken {
			l.theta[image] = equivalence
			l.keys = append(l.keys, image)
		}
	}
	if len(l.keys) == count {
		return ast.ASTHandle{}, false, nil
	}
	return t.localizeRewriteWith(l, rewrite, direct, memo)
}

// localizeElim is localize_elim: an and-elim or not-or-elim from a
// mixed premise G, taken locally from theta(G) after modus ponens with
// G = theta(G), and moved back to the conclusion by congruence.
func (t *FullTranslation) localizeElim(proof ast.ASTHandle, direct *DirectProofInterpolator, memo *translationMemo) (result ast.ASTHandle, ok bool, resultErr error) {
	m := t.base.manager
	premise, err := m.Premise(proof, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	l := t.newRewriteLocalizer()
	defer func() {
		resultErr = errors.Join(resultErr, l.release())
	}()
	if err = l.collect(premise); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if len(l.keys) == 0 {
		return ast.ASTHandle{}, false, nil
	}
	pivot, err := m.Conclusion(premise)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	conclusion, err := m.Conclusion(proof)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	localPivot, err := l.apply(pivot)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	localConclusion, err := l.apply(conclusion)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if localPivot == pivot {
		return ast.ASTHandle{}, false, nil
	}
	// make(Not,tpiv): raw Not, as the source's rule axiom.
	notLocalPivot, err := m.Make(OperatorNot, localPivot)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = l.pin(notLocalPivot); err != nil {
		return ast.ASTHandle{}, false, err
	}
	ruleAxiom := []ast.ASTHandle{notLocalPivot, localConclusion}
	notScope, err := t.base.ASTScope(notLocalPivot)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	conclusionScope, err := t.base.ASTScope(localConclusion)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	scope, err := t.base.scopes.RangeGLB(notScope, conclusionScope)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if scope.IsEmpty() {
		return ast.ASTHandle{}, false, nil
	}
	premiseProof, err := t.translateMain(premise, false, direct, memo)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if commuted, e := t.isCommuted(premiseProof); e != nil || commuted {
		return ast.ASTHandle{}, false, e
	}
	congruence, err := l.prove(pivot, true, direct, memo)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = l.pin(congruence); err != nil {
		return ast.ASTHandle{}, false, err
	}
	pivotEquivalence, err := l.makeEquivalence(pivot, localPivot)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = l.pin(pivotEquivalence); err != nil {
		return ast.ASTHandle{}, false, err
	}
	localPremise, err := direct.MakeModusPonens(pivotEquivalence, premiseProof, congruence)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = l.pin(localPremise); err != nil {
		return ast.ASTHandle{}, false, err
	}
	axiom, err := direct.MakeAxiom(ruleAxiom)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = l.pin(axiom); err != nil {
		return ast.ASTHandle{}, false, err
	}
	result, err = direct.MakeResolution(localPivot, []ast.ASTHandle{localConclusion}, axiom, localPremise)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if localConclusion != conclusion {
		if err = l.pin(result); err != nil {
			return ast.ASTHandle{}, false, err
		}
		back, e := l.prove(conclusion, false, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if e = l.pin(back); e != nil {
			return ast.ASTHandle{}, false, e
		}
		equivalence, e := l.makeEquivalence(localConclusion, conclusion)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if e = l.pin(equivalence); e != nil {
			return ast.ASTHandle{}, false, e
		}
		if result, e = direct.MakeModusPonens(equivalence, result, back); e != nil {
			return ast.ASTHandle{}, false, e
		}
	}
	return result, true, nil
}

// translateLocalizedStep is translate_main's fork-fix hooks, tried after
// the source's idioms and before the premises are translated.
func (t *FullTranslation) translateLocalizedStep(proof ast.ASTHandle, rule int32, direct *DirectProofInterpolator, memo *translationMemo) (ast.ASTHandle, bool, error) {
	m := t.base.manager
	switch rule {
	case ast.ASTPrTransitivity:
		second, err := m.Premise(proof, 1)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		secondRule, err := m.ProofRule(second)
		if err != nil || secondRule != ast.ASTPrRewrite {
			return ast.ASTHandle{}, false, err
		}
		locality, err := t.Locality(second)
		if err != nil || locality != -1 {
			return ast.ASTHandle{}, false, err
		}
		firstSource, err := m.Premise(proof, 0)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		first, err := t.translateMain(firstSource, false, direct, memo)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if commuted, e := t.isCommuted(first); e != nil || commuted {
			return ast.ASTHandle{}, false, e
		}
		rewrite, ok, err := t.localizeRewrite(firstSource, second, direct, memo)
		if err != nil || !ok {
			return ast.ASTHandle{}, false, err
		}
		firstConclusion, err := m.Conclusion(firstSource)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		secondConclusion, err := m.Conclusion(second)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		x, err := m.Arg(firstConclusion, 0)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		y, err := m.Arg(firstConclusion, 1)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		z, err := m.Arg(secondConclusion, 1)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		result, err := direct.MakeTransitivity(x, y, z, first, rewrite)
		return result, err == nil, err
	case ast.ASTPrAndElim, ast.ASTPrNotOrElim:
		premise, err := m.Premise(proof, 0)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		pivot, err := m.Conclusion(premise)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		scope, err := t.base.ASTScope(pivot)
		if err != nil || !scope.IsEmpty() {
			return ast.ASTHandle{}, false, err
		}
		return t.localizeElim(proof, direct, memo)
	}
	return ast.ASTHandle{}, false, nil
}
