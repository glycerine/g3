package smt2

// Source: src/parsers/smt2/smt2parser.cpp:1318-1511 push_match_frame,
// compile_patterns, check_patterns, bind_match, parse_constructor_pattern
// and parse_match_pattern (SMT-LIB 2.6 match expressions).

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// matchExpression is push_match_frame plus the match frame's pop: the
// "match" symbol has been read. It parses (match t ((p1 t1) ... (pn tn))),
// compiles the cases into an ite chain and consumes the closing ')'.
func (p *commandParser) matchExpression() (ast.ASTHandle, string, error) {
	m, err := p.astManager()
	if err != nil {
		return ast.ASTHandle{}, "", err
	}
	t, _, err := p.expression()
	if err != nil {
		return ast.ASTHandle{}, "", err
	}
	srt, err := m.GetSort(t)
	if err != nil {
		return ast.ASTHandle{}, "", err
	}
	if p.token != LeftParenToken {
		return ast.ASTHandle{}, "", errors.New("pattern bindings should be enclosed in a parenthesis")
	}
	if err = p.advance(); err != nil {
		return ast.ASTHandle{}, "", err
	}
	var patterns, cases []ast.ASTHandle
	for p.token != RightParenToken {
		pattern, body, err := p.matchCase(srt)
		if err != nil {
			return ast.ASTHandle{}, "", err
		}
		patterns = append(patterns, pattern)
		cases = append(cases, body)
	}
	if err = p.advance(); err != nil {
		return ast.ASTHandle{}, "", err
	}
	// compilePatterns pins its result on the expression stack.
	result, err := p.compilePatterns(t, patterns, cases)
	if err != nil {
		return ast.ASTHandle{}, "", err
	}
	// pop_match_frame at the match expression's ')'.
	if err = p.expect(RightParenToken); err != nil {
		return ast.ASTHandle{}, "", err
	}
	return result, "", nil
}

// matchCase parses one (pattern term) binding in its own m_env scope; the
// pattern variables' bindings end with the case.
func (p *commandParser) matchCase(srt ast.ASTHandle) (pattern, body ast.ASTHandle, result error) {
	p.env.beginScope()
	numBindings, oldLocals := p.bindings, len(p.locals.refs)
	defer func() {
		p.bindings = numBindings
		result = errors.Join(result, p.env.endScopeWithOwningCleanup(), p.locals.shrink(p.manager, oldLocals))
	}()
	if p.token != LeftParenToken {
		return ast.ASTHandle{}, ast.ASTHandle{}, errors.New("invalid pattern binding, '(' expected")
	}
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	pattern, err := p.matchPattern(srt)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	body, _, err = p.expression()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if p.token != RightParenToken {
		return ast.ASTHandle{}, ast.ASTHandle{}, errors.New("invalid pattern binding, ')' expected")
	}
	return pattern, body, p.advance()
}

// matchPattern is parse_match_pattern: (C x1 ... xn), the wildcard _, a
// nullary constructor C, or a variable x. Source catches every cmd_exception
// of the bare-name constructor lookup and binds the name as a variable.
func (p *commandParser) matchPattern(srt ast.ASTHandle) (ast.ASTHandle, error) {
	m := p.manager
	if p.token == LeftParenToken {
		return p.constructorPattern(srt)
	}
	if p.token == SymbolToken && p.scanner.ID() == "_" {
		if err := p.advance(); err != nil {
			return ast.ASTHandle{}, err
		}
		return p.pinExpression(m.MkVar(0, srt))
	}
	if p.token != SymbolToken {
		return ast.ASTHandle{}, errors.New("constructor symbol or variable expected")
	}
	name := p.scanner.ID()
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	f, found, err := p.context.FindFunctionDeclarationByRange(name, 0, srt)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if found {
		_, constructor, err := m.DatatypeConstructorIndex(f)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		arity, err := m.GetArity(f)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		// "expecting a constructor, got a previously declared function" and
		// "constructor expects arguments, ..." are caught as cmd_exception.
		if constructor && arity == 0 {
			return p.pinExpression(m.MkApp(f, nil))
		}
	}
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.env.bindMatchVariable(m, symbol, srt, &p.bindings, &p.locals); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.pinExpression(m.MkVar(0, srt))
}

// constructorPattern is parse_constructor_pattern after the '(' was seen.
func (p *commandParser) constructorPattern(srt ast.ASTHandle) (ast.ASTHandle, error) {
	m := p.manager
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	if p.token != SymbolToken {
		return ast.ASTHandle{}, errors.New("constructor symbol expected")
	}
	name := p.scanner.ID()
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	var vars []string
	for p.token != RightParenToken {
		if p.token != SymbolToken {
			return ast.ASTHandle{}, errors.New("variable symbol expected")
		}
		v := p.scanner.ID()
		if err := p.advance(); err != nil {
			return ast.ASTHandle{}, err
		}
		if v != "_" {
			for _, prior := range vars {
				if prior == v {
					return ast.ASTHandle{}, errors.New("unexpected repeated variable in pattern expression")
				}
			}
		}
		vars = append(vars, v)
	}
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	f, found, err := p.context.FindFunctionDeclarationByRange(name, uint32(len(vars)), srt)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !found {
		return ast.ASTHandle{}, fmt.Errorf("invalid function declaration reference, unknown function %s", name)
	}
	if _, constructor, err := m.DatatypeConstructorIndex(f); err != nil {
		return ast.ASTHandle{}, err
	} else if !constructor {
		return ast.ASTHandle{}, errors.New("expecting a constructor")
	}
	arity, err := m.GetArity(f)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if int(arity) != len(vars) {
		return ast.ASTHandle{}, errors.New("mismatching number of variables supplied to constructor")
	}
	p.bindings += uint32(len(vars))
	args := make([]ast.ASTHandle, len(vars))
	for i := range vars {
		domain, err := m.GetDomain(f, uint32(i))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		v, err := m.MkVar(uint32(i), domain)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(v)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		p.locals.pushOwned(ref)
		args[i] = v
		if vars[i] != "_" {
			symbol, err := m.Symbols.Intern(vars[i])
			if err != nil {
				return ast.ASTHandle{}, err
			}
			p.env.insert(symbol, smt2Local{term: v, level: p.bindings})
		}
	}
	return p.pinExpression(m.MkApp(f, args))
}

// compilePatterns folds the cases, last first, into an ite chain over the
// constructor recognizers, substituting the accessor terms for the pattern
// variables.
func (p *commandParser) compilePatterns(t ast.ASTHandle, patterns, cases []ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	m := p.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	hold := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return h, nil
	}
	output := func() (ast.ASTRef, error) {
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTRef{}, err
		}
		refs = append(refs, ref)
		return ref, nil
	}
	sub, err := rewriter.NewVarSubstitution(m, false)
	if sub != nil {
		defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	srt, err := m.GetSort(t)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.checkPatterns(patterns, srt); err != nil {
		return ast.ASTHandle{}, err
	}
	for i := len(patterns) - 1; i >= 0; i-- {
		subst, cond, err := p.bindMatch(t, patterns[i], hold)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		newCase := cases[i]
		if len(subst) != 0 {
			out, err := output()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if err = sub.Substitute(cases[i], subst, out); err != nil {
				return ast.ASTHandle{}, err
			}
			if newCase, err = m.RefGet(out); err != nil {
				return ast.ASTHandle{}, err
			}
			inv, err := rewriter.NewInverseVarShifter(m)
			if inv != nil {
				defer func() { resultErr = errors.Join(resultErr, inv.Close()) }()
			}
			if err != nil {
				return ast.ASTHandle{}, err
			}
			shifted, err := output()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if err = inv.InverseShift(newCase, uint32(len(subst)), shifted); err != nil {
				return ast.ASTHandle{}, err
			}
			if newCase, err = m.RefGet(shifted); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		if !result.IsNull() {
			result, err = hold(m.MkIte(cond, newCase, result))
		} else {
			// pattern match binding is ignored.
			result = newCase
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	// Pinned before the deferred release of the intermediate terms.
	return p.pinExpression(result, nil)
}

// checkPatterns is check_patterns.
func (p *commandParser) checkPatterns(patterns []ast.ASTHandle, s ast.ASTHandle) error {
	m := p.manager
	datatype, err := p.isDatatypeSort(s)
	if err != nil {
		return err
	}
	if !datatype {
		return errors.New("pattern matching is only supported for algebraic datatypes")
	}
	constructors, err := m.DatatypeConstructors(s)
	if err != nil {
		return err
	}
	for _, pattern := range patterns {
		kind, err := m.GetKind(pattern)
		if err != nil {
			return err
		}
		if kind == ast.ASTVar {
			return nil
		}
	}
	if len(patterns) < len(constructors) {
		return errors.New("non-exhaustive pattern match")
	}
	marked := make(map[ast.ASTHandle]bool, len(patterns))
	for _, pattern := range patterns {
		decl, err := m.GetDecl(pattern)
		if err != nil {
			return err
		}
		marked[decl] = true
	}
	for _, f := range constructors {
		if !marked[f] {
			return errors.New("a constructor is missing from pattern match")
		}
	}
	return nil
}

// isDatatypeSort is datatype::util::is_datatype.
func (p *commandParser) isDatatypeSort(s ast.ASTHandle) (bool, error) {
	m := p.manager
	family, err := m.DatatypeFamilyID()
	if errors.Is(err, ast.ErrASTDomain) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	sortFamily, err := m.GetFamilyID(s)
	if err != nil {
		return false, err
	}
	kind, err := m.GetDeclKind(s)
	if err != nil {
		return false, err
	}
	return sortFamily == family && kind == ast.ASTDatatypeSort, nil
}

// bindMatch is bind_match: the match condition, and the substitution for
// the pattern's variables over t shifted by their number.
func (p *commandParser) bindMatch(t, pattern ast.ASTHandle, hold func(ast.ASTHandle, error) (ast.ASTHandle, error)) ([]ast.ASTHandle, ast.ASTHandle, error) {
	m := p.manager
	kind, err := m.GetKind(pattern)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	shift := func(n uint32) (ast.ASTHandle, error) {
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		shifter, err := p.varShifter()
		h := ast.ASTHandle{}
		if err == nil {
			err = shifter.ShiftBy(t, n, ref)
		}
		if err == nil {
			h, err = m.RefGet(ref)
		}
		if err == nil {
			h, err = hold(h, nil)
		}
		return h, errors.Join(err, m.DelRef(&ref))
	}
	if kind == ast.ASTVar {
		tsh, err := shift(1)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		cond, err := hold(m.MkTrue())
		return []ast.ASTHandle{tsh}, cond, err
	}
	f, err := m.GetDecl(pattern)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	r, err := hold(m.DatatypeConstructorIs(f))
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	accessors, err := m.DatatypeConstructorAccessors(f)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	tsh, err := shift(uint32(len(accessors)))
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	subst := make([]ast.ASTHandle, 0, len(accessors))
	for _, a := range accessors {
		app, err := hold(m.MkApp(a, []ast.ASTHandle{tsh}))
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		subst = append(subst, app)
	}
	cond, err := hold(m.MkApp(r, []ast.ASTHandle{t}))
	return subst, cond, err
}
