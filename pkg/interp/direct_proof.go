package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp basic proof rules and term
// classification.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type TermSide uint8

const (
	TermA TermSide = iota
	TermB
	TermMixed
)

// DirectProofInterpolator builds the symbolic proof terms used by the active
// IZ3_TRANSLATE_FULL path. It borrows Base and owns its declarations,
// placeholders, and returned proof-term roots until Close.
type DirectProofInterpolator struct {
	self                  *DirectProofInterpolator
	base                  *Base
	scope                 ScopeRange
	weak                  bool
	contra                ast.ASTHandle
	sum                   ast.ASTHandle
	rotateSum             ast.ASTHandle
	leqToEq               ast.ASTHandle
	eqToLeq               ast.ASTHandle
	excludedMid           ast.ASTHandle
	symmetry              ast.ASTHandle
	congruence            ast.ASTHandle
	modusPonens           ast.ASTHandle
	concat                ast.ASTHandle
	addPosition           ast.ASTHandle
	rewriteA              ast.ASTHandle
	rewriteB              ast.ASTHandle
	normalStep            ast.ASTHandle
	normalChain           ast.ASTHandle
	normal                ast.ASTHandle
	standForall           ast.ASTHandle
	standExists           ast.ASTHandle
	noProof               ast.ASTHandle
	epsilon               ast.ASTHandle
	topPosition           ast.ASTHandle
	placeholders          map[ast.ASTHandle]ast.ASTHandle
	localizationVariables []localizationVariable
	localizationMaps      map[ScopeRange]*localizationMaps
	// simplifyMemo is the member simplify_memo: simplify_rec results,
	// kept for the interpolator's lifetime.
	simplifyMemo map[ast.ASTHandle]ast.ASTHandle
	// owned holds g3's own references to returned proof terms, released
	// first at Close; the source's members below decide when they die.
	owned []ast.ASTRef
	// declarations are the symb members contra ... sexists, which
	// ~iz3proof_itp_impl dec_refs in creation order.
	declarations []ast.ASTRef
	// The ast-holding members, in declaration order: placeholders, then
	// no_proof, epsilon and top_pos (variables), subst_memo,
	// simplify_memo, localization_vars and localization_maps_per_range.
	// After the destructor body they die in reverse.
	placeholderRefs     iz3RefTable
	variables           [3]ast.ASTRef
	substMemoRefs       iz3RefTable
	simplifyMemoRefs    iz3RefTable
	localizationRefs    []localizationVariableRefs
	localizationMapRefs iz3RangeTable
	closed              bool
}

// localizationVariableRefs is a LocVar's ast members var and term; ~LocVar
// releases term, then var.
type localizationVariableRefs struct {
	variable ast.ASTRef
	term     ast.ASTRef
}

func NewDirectProofInterpolator(base *Base, scope ScopeRange, weak bool) (result *DirectProofInterpolator, resultErr error) {
	if base == nil || base.open() != nil {
		return nil, ast.ErrASTHandle
	}
	p := &DirectProofInterpolator{base: base, scope: scope, placeholders: make(map[ast.ASTHandle]ast.ASTHandle), localizationMaps: make(map[ScopeRange]*localizationMaps), simplifyMemo: make(map[ast.ASTHandle]ast.ASTHandle)}
	p.self = p
	// The selected source constructor deliberately forces weak=false even when
	// requested. Preserve that behavior until the disabled weak implementation
	// is selected by a retained caller.
	p.weak = false
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.Close())
			result = nil
		}
	}()
	manager := base.manager
	boolean, err := manager.BoolType()
	if err != nil {
		return nil, err
	}
	integer, err := manager.IntType()
	if err != nil {
		return nil, err
	}
	if p.contra, err = p.newFunction("@contra", []ast.ASTHandle{boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.sum, err = p.newFunction("@sum", []ast.ASTHandle{boolean, integer, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.rotateSum, err = p.newFunction("@rotsum", []ast.ASTHandle{boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.leqToEq, err = p.newFunction("@leq2eq", []ast.ASTHandle{boolean, boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.eqToLeq, err = p.newFunction("@eq2leq", []ast.ASTHandle{boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.congruence, err = p.newFunction("@cong", []ast.ASTHandle{boolean, integer, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.excludedMid, err = p.newFunction("@exmid", []ast.ASTHandle{boolean, boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.symmetry, err = p.newFunction("@symm", []ast.ASTHandle{boolean}, boolean); err != nil {
		return nil, err
	}
	if p.epsilon, err = manager.MakeVariable("@eps", integer); err != nil {
		return nil, err
	}
	if err = p.holdVariable(1, p.epsilon); err != nil {
		return nil, err
	}
	if p.modusPonens, err = p.newFunction("@mp", []ast.ASTHandle{boolean, boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.noProof, err = manager.MakeVariable("@nop", boolean); err != nil {
		return nil, err
	}
	if err = p.holdVariable(0, p.noProof); err != nil {
		return nil, err
	}
	if p.concat, err = p.newFunction("@concat", []ast.ASTHandle{boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.topPosition, err = manager.MakeVariable("@top_pos", boolean); err != nil {
		return nil, err
	}
	if err = p.holdVariable(2, p.topPosition); err != nil {
		return nil, err
	}
	if p.addPosition, err = p.newFunction("@add_pos", []ast.ASTHandle{integer, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.rewriteA, err = p.newFunction("@rewrite_A", []ast.ASTHandle{boolean, boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.rewriteB, err = p.newFunction("@rewrite_B", []ast.ASTHandle{boolean, boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.normalStep, err = p.newFunction("@normal_step", []ast.ASTHandle{boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.normalChain, err = p.newFunction("@normal_chain", []ast.ASTHandle{boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.normal, err = p.newFunction("@normal", []ast.ASTHandle{boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.standForall, err = p.newFunction("@sforall", []ast.ASTHandle{boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	if p.standExists, err = p.newFunction("@sexists", []ast.ASTHandle{boolean, boolean}, boolean); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *DirectProofInterpolator) open() error {
	if p == nil || p.self != p || p.closed || p.base == nil {
		return ast.ErrASTHandle
	}
	return p.base.open()
}

func (p *DirectProofInterpolator) holdVariable(slot int, handle ast.ASTHandle) (err error) {
	p.variables[slot], err = p.base.manager.ast.NewRef(handle)
	return err
}

func (p *DirectProofInterpolator) hold(handle ast.ASTHandle) error {
	ref, err := p.base.manager.ast.NewRef(handle)
	if err != nil {
		return err
	}
	p.owned = append(p.owned, ref)
	return nil
}

func (p *DirectProofInterpolator) keep(handle ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.hold(handle); err != nil {
		return ast.ASTHandle{}, err
	}
	return handle, nil
}

func (p *DirectProofInterpolator) newFunction(name string, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	declaration, err := p.base.manager.Function(name, domain, rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := p.base.manager.ast.NewRef(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.declarations = append(p.declarations, ref)
	return declaration, nil
}

func (p *DirectProofInterpolator) Close() (result error) {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	manager := p.base.manager.ast
	for i := len(p.owned) - 1; i >= 0; i-- {
		result = errors.Join(result, manager.DelRef(&p.owned[i]))
	}
	p.owned = nil
	// ~iz3proof_itp_impl's body, then its members in reverse.
	for i := range p.declarations {
		result = errors.Join(result, manager.DelRef(&p.declarations[i]))
	}
	p.declarations = nil
	result = errors.Join(result, p.localizationMapRefs.clear(manager))
	for i := range p.localizationRefs {
		result = errors.Join(result, manager.DelRef(&p.localizationRefs[i].term))
		result = errors.Join(result, manager.DelRef(&p.localizationRefs[i].variable))
	}
	p.localizationRefs = nil
	result = errors.Join(result, p.simplifyMemoRefs.clear(manager))
	result = errors.Join(result, p.substMemoRefs.clear(manager))
	for i := len(p.variables) - 1; i >= 0; i-- {
		if p.variables[i] != (ast.ASTRef{}) {
			result = errors.Join(result, manager.DelRef(&p.variables[i]))
		}
	}
	result = errors.Join(result, p.placeholderRefs.clear(manager))
	p.placeholders = nil
	p.simplifyMemo = nil
	p.localizationVariables = nil
	p.localizationMaps = nil
	p.closed = true
	return result
}

func (p *DirectProofInterpolator) TermSide(term ast.ASTHandle) (TermSide, error) {
	if err := p.open(); err != nil {
		return TermMixed, err
	}
	scope, err := p.base.ASTScope(term)
	if err != nil {
		return TermMixed, err
	}
	if scope.IsEmpty() {
		return TermMixed, nil
	}
	contained, err := p.base.scopes.RangeContained(scope, p.scope)
	if err != nil {
		return TermMixed, err
	}
	if contained {
		return TermA, nil
	}
	return TermB, nil
}

func (p *DirectProofInterpolator) placeholder(term ast.ASTHandle) (ast.ASTHandle, error) {
	if result, found := p.placeholders[term]; found {
		return result, nil
	}
	// ast &res = placeholders[t]: the entry exists before the constant.
	entry, err := p.placeholderRefs.lookup(p.base.manager.ast, term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := p.base.manager.Type(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.base.manager.FreshConstant("@p", sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.placeholderRefs.setValue(p.base.manager.ast, entry, 0, result); err != nil {
		return ast.ASTHandle{}, err
	}
	p.placeholders[term] = result
	return result, nil
}

func (p *DirectProofInterpolator) makeSymbol(declaration ast.ASTHandle, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return p.base.manager.MakeDeclaration(declaration, args...)
}

func (p *DirectProofInterpolator) makeContraNode(proof ast.ASTHandle, literals []ast.ASTHandle, proofMask int) (ast.ASTHandle, error) {
	if len(literals) == 0 {
		return proof, nil
	}
	falsehood, err := p.base.manager.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	first, err := p.makeSymbol(p.contra, proof, falsehood)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	results := []ast.ASTHandle{first}
	for i, literal := range literals {
		rotated := p.noProof
		if proofMask&(1<<i) != 0 {
			rotated, err = p.makeSymbol(p.rotateSum, literal, proof)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		entry, e := p.makeSymbol(p.contra, rotated, literal)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		results = append(results, entry)
	}
	return p.base.manager.Make(OperatorAnd, results...)
}

func (p *DirectProofInterpolator) MakeAssumption(frame int, assumption []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	inA, err := p.base.scopes.InRange(frame, p.scope)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !inA {
		return p.keep(p.base.manager.Make(OperatorTrue))
	}
	clause := make([]ast.ASTHandle, 0, len(assumption))
	for _, literal := range assumption {
		side, e := p.TermSide(literal)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if side != TermA {
			clause = append(clause, literal)
		}
	}
	return p.keep(p.base.manager.MkOrSlice(clause))
}

func (p *DirectProofInterpolator) MakeContra(premise ast.ASTHandle, conclusion []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.keep(premise, nil)
}

func (p *DirectProofInterpolator) hypothesis(formula ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := p.base.manager.Operator(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorNot {
		body, e := p.base.manager.Arg(formula, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.hypothesis(body)
	}
	side, err := p.TermSide(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch side {
	case TermA:
		return p.base.manager.Make(OperatorFalse)
	case TermB:
		return p.base.manager.Make(OperatorTrue)
	}
	// The reference GCC build evaluates the arguments of make(...) right to
	// left; the ASTs (and @p!n placeholders) are created in that order.
	var proof, negated, formulaPlaceholder, negativePlaceholder ast.ASTHandle
	switch op {
	case OperatorGE, OperatorLE, OperatorGT, OperatorLT:
		// zleqz = make(Leq,make_int("0"),make_int("0"))
		zero, e := p.base.manager.MakeIntText("0")
		if e != nil {
			return ast.ASTHandle{}, e
		}
		zeroInequality, e := p.base.manager.Make(OperatorLE, zero, zero)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		// fark1 = make(sum,zleqz,make_int("1"),get_placeholder(P))
		if formulaPlaceholder, e = p.placeholder(formula); e != nil {
			return ast.ASTHandle{}, e
		}
		one, e := p.base.manager.MakeIntText("1")
		if e != nil {
			return ast.ASTHandle{}, e
		}
		first, e := p.makeSymbol(p.sum, zeroInequality, one, formulaPlaceholder)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		// fark2 = make(sum,fark1,make_int("1"),get_placeholder(mk_not(P)))
		if negated, e = p.base.manager.MkNot(formula); e != nil {
			return ast.ASTHandle{}, e
		}
		if negativePlaceholder, e = p.placeholder(negated); e != nil {
			return ast.ASTHandle{}, e
		}
		if one, e = p.base.manager.MakeIntText("1"); e != nil {
			return ast.ASTHandle{}, e
		}
		proof, err = p.makeSymbol(p.sum, first, one, negativePlaceholder)
	default:
		// em = make(exmid,P,get_placeholder(P),get_placeholder(mk_not(P)))
		if negated, err = p.base.manager.MkNot(formula); err != nil {
			return ast.ASTHandle{}, err
		}
		if negativePlaceholder, err = p.placeholder(negated); err != nil {
			return ast.ASTHandle{}, err
		}
		if formulaPlaceholder, err = p.placeholder(formula); err != nil {
			return ast.ASTHandle{}, err
		}
		proof, err = p.makeSymbol(p.excludedMid, formula, formulaPlaceholder, negativePlaceholder)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// make(And,make(contra,fark2,mk_false()),
	//      make(contra,get_placeholder(mk_not(P)),P),
	//      make(contra,get_placeholder(P),mk_not(P)))
	third, err := p.makeSymbol(p.contra, formulaPlaceholder, negated)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	second, err := p.makeSymbol(p.contra, negativePlaceholder, formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	falsehood, err := p.base.manager.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	first, err := p.makeSymbol(p.contra, proof, falsehood)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.Make(OperatorAnd, first, second, third)
}

func (p *DirectProofInterpolator) MakeHypothesis(formula ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.keep(p.hypothesis(formula))
}

func (p *DirectProofInterpolator) MakeReflexivity(conclusion ast.ASTHandle) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	side, err := p.TermSide(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if side == TermA {
		return p.keep(p.base.manager.Make(OperatorFalse))
	}
	if side == TermB {
		return p.keep(p.base.manager.Make(OperatorTrue))
	}
	// make(And,make(contra,no_proof,mk_false()),make(contra,mk_true(),mk_not(con))),
	// arguments evaluated right to left.
	negated, err := p.base.manager.MkNot(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	second, err := p.makeSymbol(p.contra, truth, negated)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	falsehood, err := p.base.manager.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	first, err := p.makeSymbol(p.contra, p.noProof, falsehood)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.keep(p.base.manager.Make(OperatorAnd, first, second))
}
