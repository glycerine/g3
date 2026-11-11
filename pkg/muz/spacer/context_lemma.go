package spacer

// Source: src/muz/spacer/spacer_context.h and spacer_context.cpp lemma.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

const InfinityLevel = ^uint32(0)

type Lemma struct {
	manager  *ast.ASTManager
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	body     ast.ASTRef
	cube     []ast.ASTRef
	bindings []ast.ASTRef
	// skolems is the pob's zks snapshot kept until mk_expr_core runs.
	skolems []ast.ASTRef
	level   uint32
	closed  bool
}

func NewLemma(owner *Manager, formula ast.ASTHandle, level uint32) (_ *Lemma, resultErr error) {
	if owner == nil || owner.open() != nil || owner.contexts[0] == nil {
		return nil, ErrManagerOwner
	}
	config := owner.contexts[0].config
	body, err := normalizeFormula(owner.manager, config.Params, config.Globals, formula, true, false)
	if err != nil {
		return nil, err
	}
	return &Lemma{manager: owner.manager, context: config.Params, globals: config.Globals, body: body, level: level}, nil
}

func NewLemmaFromProofObligation(obligation *ProofObligation, cube []ast.ASTHandle, level uint32) (_ *Lemma, resultErr error) {
	if obligation == nil || obligation.valid() != nil {
		return nil, ErrManagerOwner
	}
	p := obligation.transformer
	manager := p.manager
	config := p.owner.contexts[0].config
	if cube == nil {
		post, err := obligation.Post()
		if err != nil {
			return nil, err
		}
		cube, err = manager.FlattenAnd([]ast.ASTHandle{post})
		if err != nil {
			return nil, err
		}
	}
	if len(cube) == 0 {
		truth, err := manager.MkTrue()
		if err != nil {
			return nil, err
		}
		cube = append(cube, truth)
	}
	cubeRefs, err := refsFromHandles(manager, cube)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, cubeRefs))
		}
	}()
	// lemma(pob_ref const &p, expr_ref_vector &cube, unsigned lvl): the
	// skolems and bindings are taken from the pob now; the body is built by
	// mk_expr_core on the first get_expr.
	bindings, err := obligation.Bindings()
	if err != nil {
		return nil, err
	}
	bindingRefs, err := refsFromHandles(manager, bindings)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(manager, bindingRefs))
		}
	}()
	var skolemRefs []ast.ASTRef
	if len(bindings) != 0 {
		skolem, err := obligation.Skolems()
		if err != nil {
			return nil, err
		}
		if skolemRefs, err = refsFromHandles(manager, skolem); err != nil {
			return nil, err
		}
	}
	result := &Lemma{manager: manager, context: config.Params, globals: config.Globals, cube: cubeRefs, bindings: bindingRefs, skolems: skolemRefs, level: level}
	committed = true
	return result, nil
}

// mkExprCore is lemma::mk_expr_core for a pob lemma: on the first get_expr
// the body is push_not(mk_and(cube)) normalized, then abstracted over the
// pob's skolems into a universal when the pob is not ground and the body
// has skolem constants (otherwise the bindings are dropped).
func (l *Lemma) mkExprCore() (resultErr error) {
	if l.body != (ast.ASTRef{}) {
		return nil
	}
	manager := l.manager
	cube := make([]ast.ASTHandle, len(l.cube))
	for i := range l.cube {
		handle, err := manager.RefGet(l.cube[i])
		if err != nil {
			return err
		}
		cube[i] = handle
	}
	conjunction, err := manager.MkAndArray(cube)
	if err != nil {
		return err
	}
	// m_body = ::push_not(::mk_and(m_cube)): mk_and's temporary dies at the
	// end of the statement, before normalize.
	conjunctionRef, err := manager.NewRef(conjunction)
	if err != nil {
		return err
	}
	clause, err := manager.PushNot(conjunction)
	if err != nil {
		return errors.Join(err, manager.DelRef(&conjunctionRef))
	}
	if err = manager.DelRef(&conjunctionRef); err != nil {
		return err
	}
	body, err := normalizeFormula(manager, l.context, l.globals, clause, true, false)
	if err != nil {
		return err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, manager.DelRef(&body))
		}
	}()
	if len(l.bindings) != 0 {
		handle, err := manager.RefGet(body)
		if err != nil {
			return err
		}
		hasSkolem, err := HasZKConst(manager, handle)
		if err != nil {
			return err
		}
		if !hasSkolem {
			if err = releaseNormalizeRefs(manager, l.bindings); err != nil {
				return err
			}
			l.bindings = nil
		}
	}
	if len(l.bindings) != 0 {
		skolem := make([]ast.ASTHandle, len(l.skolems))
		for i := range l.skolems {
			if skolem[i], err = manager.RefGet(l.skolems[i]); err != nil {
				return err
			}
		}
		for left, right := 0, len(skolem)-1; left < right; left, right = left+1, right-1 {
			skolem[left], skolem[right] = skolem[right], skolem[left]
		}
		handle, err := manager.RefGet(body)
		if err != nil {
			return err
		}
		abstractor, err := ast.NewExprAbstractor(manager)
		if err != nil {
			return err
		}
		if err = abstractor.Abstract(0, skolem, handle, body); err != nil {
			return errors.Join(err, abstractor.Close())
		}
		if err = abstractor.Close(); err != nil {
			return err
		}
		sorts := make([]ast.ASTHandle, len(skolem))
		names := make([]usymbol.Symbol, len(skolem))
		for i := range skolem {
			if sorts[i], err = manager.GetSort(skolem[i]); err != nil {
				return err
			}
			decl, declErr := manager.GetDecl(skolem[i])
			if declErr != nil {
				return declErr
			}
			if names[i], err = manager.GetASTName(decl); err != nil {
				return err
			}
		}
		abstracted, err := manager.RefGet(body)
		if err != nil {
			return err
		}
		id, err := manager.GetASTID(abstracted)
		if err != nil {
			return err
		}
		quantifier, err := manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: abstracted, QID: usymbol.Numerical(id)})
		if err != nil {
			return err
		}
		if err = manager.SetRef(body, quantifier); err != nil {
			return err
		}
	}
	if err = releaseNormalizeRefs(manager, l.skolems); err != nil {
		return err
	}
	l.skolems = nil
	l.body = body
	return nil
}

func (l *Lemma) open() error {
	if l == nil || l.closed || l.manager == nil || l.context == nil || l.globals == nil || (l.body == (ast.ASTRef{}) && len(l.cube) == 0) {
		return ErrManagerOwner
	}
	return nil
}

func (l *Lemma) Body() (ast.ASTHandle, error) {
	if err := l.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := l.mkExprCore(); err != nil {
		return ast.ASTHandle{}, err
	}
	return l.manager.RefGet(l.body)
}

// IsGround follows lemma::is_ground: the lemma body is not a top-level
// quantifier. This differs from the AST's recursive ground-expression flag.
func (l *Lemma) IsGround() (bool, error) {
	body, err := l.Body()
	if err != nil {
		return false, err
	}
	kind, err := l.manager.GetKind(body)
	return kind != ast.ASTQuantifier, err
}

func (l *Lemma) Level() (uint32, error) {
	if err := l.open(); err != nil {
		return 0, err
	}
	return l.level, nil
}

func (l *Lemma) setLevel(level uint32) error {
	if err := l.open(); err != nil {
		return err
	}
	l.level = level
	return nil
}

func (l *Lemma) makeCube() (resultErr error) {
	if len(l.cube) != 0 {
		return nil
	}
	body, err := l.Body()
	if err != nil {
		return err
	}
	kind, err := l.manager.GetKind(body)
	if err != nil {
		return err
	}
	if kind == ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	negated, err := l.manager.PushNot(body)
	if err != nil {
		return err
	}
	parts, err := l.manager.FlattenAnd([]ast.ASTHandle{negated})
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		truth, trueErr := l.manager.MkTrue()
		if trueErr != nil {
			return trueErr
		}
		parts = append(parts, truth)
	}
	l.cube, err = refsFromHandles(l.manager, parts)
	if err != nil {
		return err
	}
	if err = normalizeLess(l.manager, l.cube); err != nil {
		resultErr = errors.Join(err, releaseNormalizeRefs(l.manager, l.cube))
		l.cube = nil
		return resultErr
	}
	return nil
}

func (l *Lemma) Cube() ([]ast.ASTHandle, error) {
	if err := l.open(); err != nil {
		return nil, err
	}
	if err := l.makeCube(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(l.cube))
	for i := range l.cube {
		handle, err := l.manager.RefGet(l.cube[i])
		if err != nil {
			return nil, err
		}
		result[i] = handle
	}
	return result, nil
}

// IsFalse is lemma::is_false: a one-literal cube that is true, or a built
// body that is false. A pob lemma whose body is not built yet (its cube has
// two or more literals) is not false; get_expr is never forced.
func (l *Lemma) IsFalse() (bool, error) {
	if err := l.open(); err != nil {
		return false, err
	}
	if len(l.cube) == 1 {
		handle, err := l.manager.RefGet(l.cube[0])
		if err != nil {
			return false, err
		}
		return l.manager.IsTrue(handle), nil
	}
	if l.body != (ast.ASTRef{}) {
		body, err := l.manager.RefGet(l.body)
		if err != nil {
			return false, err
		}
		return l.manager.IsFalse(body), nil
	}
	return false, nil
}

func (l *Lemma) AddBindings(bindings []ast.ASTHandle) error {
	if err := l.open(); err != nil {
		return err
	}
	refs, err := refsFromHandles(l.manager, bindings)
	if err != nil {
		return err
	}
	l.bindings = append(l.bindings, refs...)
	return nil
}

func (l *Lemma) bindingHandles() ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(l.bindings))
	for i := range l.bindings {
		handle, err := l.manager.RefGet(l.bindings[i])
		if err != nil {
			return nil, err
		}
		result[i] = handle
	}
	return result, nil
}

func (l *Lemma) Instances() (_ []ast.ASTRef, resultErr error) {
	body, err := l.Body()
	if err != nil {
		return nil, err
	}
	return l.instancesOf(body)
}

func (l *Lemma) instancesOf(body ast.ASTHandle) (_ []ast.ASTRef, resultErr error) {
	kind, err := l.manager.GetKind(body)
	if err != nil || kind != ast.ASTQuantifier || len(l.bindings) == 0 {
		return nil, err
	}
	count, err := l.manager.GetNumDeclarations(body)
	if err != nil || count == 0 || len(l.bindings)%int(count) != 0 {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	bindings, err := l.bindingHandles()
	if err != nil {
		return nil, err
	}
	var result []ast.ASTRef
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(l.manager, result))
		}
	}()
	for offset := 0; offset < len(bindings); offset += int(count) {
		instance, newErr := l.manager.NewRef(ast.ASTHandle{})
		if newErr != nil {
			return nil, newErr
		}
		if newErr = astrewriter.InstantiateQuantifierWithOrder(l.manager, body, bindings[offset:offset+int(count)], false, instance); newErr != nil {
			_ = l.manager.DelRef(&instance)
			return nil, newErr
		}
		result = append(result, instance)
	}
	return result, nil
}

func (l *Lemma) Close() (result error) {
	if l == nil {
		return ErrManagerOwner
	}
	if l.closed {
		return nil
	}
	if l.manager == nil {
		return ErrManagerOwner
	}
	var bodyErr error
	if l.body != (ast.ASTRef{}) {
		bodyErr = l.manager.DelRef(&l.body)
	}
	result = errors.Join(releaseNormalizeRefs(l.manager, l.bindings), releaseNormalizeRefs(l.manager, l.skolems), releaseNormalizeRefs(l.manager, l.cube), bodyErr)
	if result == nil {
		l.bindings = nil
		l.cube = nil
		l.manager = nil
		l.context = nil
		l.globals = nil
		l.closed = true
	}
	return result
}
