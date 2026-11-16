package cmd_context

// Source: dparser::parse_pred and parse_args.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// parsePred is dparser::parse_pred. The returned reference is the caller's
// app_ref pred; the expr_ref_vector args is released on return, after pred
// is assigned.
func (p *datalogParser) parsePred(tok dtoken, s string) (_ dtoken, pred ast.ASTRef, _ bool, err error) {
	var args []ast.ASTHandle
	var argNames []usymbol.Symbol
	f, found := p.predicates[s] // context::try_get_predicate_decl
	tok, args, argNames, err = p.parseArgs(tok, f, found, args, argNames)
	argRefs := make([]ast.ASTRef, 0, len(args))
	for _, a := range args {
		r, e := p.hold(a)
		if e != nil {
			err = errors.Join(err, e)
			break
		}
		argRefs = append(argRefs, r)
	}
	defer func() { err = errors.Join(err, p.releaseAll(argRefs)) }()
	if err != nil {
		return tok, ast.ASTRef{}, false, err
	}
	isPredicateDeclaration := !found
	engine, err := p.context.fixedpointContext()
	if err != nil {
		return tok, ast.ASTRef{}, false, err
	}
	if !found {
		// We are in a declaration.
		domain := make([]ast.ASTHandle, len(args))
		for i, arg := range args {
			if domain[i], err = p.manager.GetSort(arg); err != nil {
				return tok, ast.ASTRef{}, false, err
			}
		}
		// mk_func_decl and register_predicate(f, true).
		declare := p.context.DeclareRelation
		if p.context.datalogFrontendManager {
			// read_datalog has no cmd_context: the predicate is only
			// registered with the datalog context.
			declare = p.context.declareParsedRelation
		}
		if f, err = declare(s, domain); err != nil {
			return tok, ast.ASTRef{}, false, err
		}
		p.predicates[s] = f

		for tok == dtkID {
			predPragma := p.lexer.tokenData()
			if predPragma == "printtuples" || predPragma == "outputtuples" {
				if err = engine.SetOutputPredicate(f); err != nil {
					return tok, ast.ASTRef{}, false, err
				}
			}
			tok = p.lexer.nextToken()
		}
		if err = engine.SetArgumentNames(f, argNames); err != nil {
			return tok, ast.ASTRef{}, false, err
		}
	}
	arity, err := p.manager.GetArity(f)
	if err != nil {
		return tok, ast.ASTRef{}, false, err
	}
	if uint32(len(args)) < arity {
		tok, err = p.unexpected(tok, "too few arguments passed to predicate")
		return tok, ast.ASTRef{}, false, err
	}
	app, err := p.manager.MkApp(f, args)
	if err != nil {
		return tok, ast.ASTRef{}, false, err
	}
	pred, err = p.hold(app)
	return tok, pred, isPredicateDeclaration, err
}

// parseArgs parses predicate arguments. If found is false, they are the
// arguments of a predicate declaration, and their names are pushed to
// argNames.
func (p *datalogParser) parseArgs(tok dtoken, f ast.ASTHandle, found bool, args []ast.ASTHandle, argNames []usymbol.Symbol) (dtoken, []ast.ASTHandle, []usymbol.Symbol, error) {
	if tok != dtkLP {
		return tok, args, argNames, nil
	}
	var arity uint32
	if found {
		var err error
		if arity, err = p.manager.GetArity(f); err != nil {
			return tok, args, argNames, err
		}
	}
	argIdx := uint32(0)
	tok = p.lexer.nextToken()
	for tok != dtkEOS && tok != dtkError {
		if !found {
			// We are in a predicate declaration.
			if tok != dtkID {
				tok, err := p.unexpected(tok, "Expecting variable in declaration")
				return tok, args, argNames, err
			}
			varSymbol, err := p.manager.Symbols.Intern(p.lexer.tokenData())
			if err != nil {
				return tok, args, argNames, err
			}
			tok = p.lexer.nextToken()
			if tok != dtkColon {
				tok, err = p.unexpected(tok,
					"Expecting colon in declaration (first occurrence of a predicate must be a declaration)")
				return tok, args, argNames, err
			}
			tok = p.lexer.nextToken()

			if tok != dtkID {
				tok, err = p.unexpected(tok, "Expecting sort after colon in declaration")
				return tok, args, argNames, err
			}
			sortName, ok := extractDomainName(p.lexer.tokenData())
			if !ok {
				tok, err = p.unexpected(dtkID, "sort name")
				return tok, args, argNames, err
			}
			s, err := p.getSort(sortName)
			if err != nil {
				return tok, args, argNames, err
			}
			// Every declaration argument is the variable m_num_vars.
			v, err := p.manager.MkVar(p.numVars, s)
			if err != nil {
				return tok, args, argNames, err
			}
			args = append(args, v)
			argNames = append(argNames, varSymbol)
			tok = p.lexer.nextToken()
		} else {
			if argIdx >= arity {
				tok, err := p.unexpected(tok, "too many arguments passed to predicate")
				return tok, args, argNames, err
			}
			s, err := p.manager.GetDomain(f, argIdx)
			if err != nil {
				return tok, args, argNames, err
			}
			if tok, args, err = p.parseArg(tok, s, args); err != nil {
				return tok, args, argNames, err
			}
		}

		argIdx++

		if tok == dtkRP {
			return p.lexer.nextToken(), args, argNames, nil
		}
		if tok == dtkComma {
			tok = p.lexer.nextToken()
		}
	}
	return tok, args, argNames, nil
}
