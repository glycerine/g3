package smt2

// Source: src/cmd_context/tactic_cmds.cpp sexpr2tactic and mk_using_params;
// src/parsers/smt2/smt2parser.cpp command argument parsing.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (p *commandParser) sourceBuiltinProbe(name string, manager *ast.ASTManager) (tactic.ProbeEvaluator, error) {
	kinds := map[string]tactic.ProbeKind{
		"memory": tactic.ProbeMemory, "depth": tactic.ProbeDepth, "size": tactic.ProbeSize,
		"num-exprs": tactic.ProbeNumExprs, "num-consts": tactic.ProbeNumConsts,
		"num-bool-consts": tactic.ProbeNumBoolConsts, "num-arith-consts": tactic.ProbeNumArithmeticConsts,
		"num-bv-consts": tactic.ProbeNumBitVectorConsts, "produce-proofs": tactic.ProbeProduceProofs,
		"produce-model": tactic.ProbeProduceModels, "produce-unsat-cores": tactic.ProbeProduceUnsatCores,
		"has-quantifiers": tactic.ProbeHasQuantifier, "has-patterns": tactic.ProbeHasPattern,
		"is-propositional": tactic.ProbeIsPropositional, "is-qfbv": tactic.ProbeIsQFBitVector,
		"is-qfaufbv": tactic.ProbeIsQFArrayUFBitVector, "is-qfufbv": tactic.ProbeIsQFUFBitVector,
	}
	kind, found := kinds[name]
	if found {
		return tactic.NewProbe(manager.Budget(), kind, 0)
	}
	globals, err := p.context.GlobalParameters()
	if err != nil {
		return nil, err
	}
	context := globals.Context
	switch name {
	case "ackr-bound-probe":
		return ackermannization.NewBoundProbe(manager.Budget())
	case "is-qflia", "is-qfauflia", "is-qflra", "is-qflira", "is-ilp", "is-qfnia", "is-qfnra", "is-nia", "is-nra", "is-nira", "is-lia", "is-lra", "is-lira", "is-qfufnra":
		fragments := map[string]arith.ArithmeticFragment{
			"is-qflia": arith.FragmentQFLIA, "is-qfauflia": arith.FragmentQFAUFLIA,
			"is-qflra": arith.FragmentQFLRA, "is-qflira": arith.FragmentQFLIRA,
			"is-ilp": arith.FragmentILP, "is-qfnia": arith.FragmentQFNIA,
			"is-qfnra": arith.FragmentQFNRA, "is-nia": arith.FragmentNIA,
			"is-nra": arith.FragmentNRA, "is-nira": arith.FragmentNIRA,
			"is-lia": arith.FragmentLIA, "is-lra": arith.FragmentLRA,
			"is-lira": arith.FragmentLIRA, "is-qfufnra": arith.FragmentQFUFNRA,
		}
		return arith.NewArithmeticFragmentProbe(manager.Budget(), context, globals, fragments[name])
	case "is-unbounded":
		return arith.NewIsUnboundedProbe(manager.Budget(), context, globals)
	case "is-pb":
		return arith.NewIsPBProbe(manager.Budget(), context, globals)
	case "is-quasi-pb":
		return smtlogics.NewIsQuasiPBProbe(manager, context, globals)
	case "arith-max-deg":
		return arith.NewArithmeticMaximumDegreeProbe(manager.Budget())
	case "arith-avg-deg":
		return arith.NewArithmeticAverageDegreeProbe(manager.Budget())
	case "arith-max-bw":
		return arith.NewArithmeticMaximumBitWidthProbe(manager.Budget())
	case "arith-avg-bw":
		return arith.NewArithmeticAverageBitWidthProbe(manager.Budget())
	case "is-qfbv-eq":
		return bv.NewIsQFBVEqualityProbe(manager.Budget(), context, globals)
	case "is-qffp", "is-qffpbv":
		return smtlogics.NewIsQFFPProbe(manager.Budget())
	case "is-qffplra":
		return smtlogics.NewIsQFFPLRAProbe(manager.Budget())
	default:
		return nil, nil
	}
}

func sourceProbeOperator(name string) (tactic.ProbeKind, bool) {
	kinds := map[string]tactic.ProbeKind{
		"=": tactic.ProbeEqual, "<=": tactic.ProbeLessEqual, ">=": tactic.ProbeGreaterEqual,
		"<": tactic.ProbeLess, ">": tactic.ProbeGreater, "and": tactic.ProbeAnd,
		"or": tactic.ProbeOr, "=>": tactic.ProbeImplies, "implies": tactic.ProbeImplies,
		"*": tactic.ProbeMultiply, "+": tactic.ProbeAdd, "-": tactic.ProbeSubtract, "/": tactic.ProbeDivide,
	}
	kind, found := kinds[name]
	return kind, found
}

func (p *commandParser) parseDeclaredTactic(name, body string) (result tactic.Tactic, err error) {
	for _, active := range p.tacticExpansion {
		if active == name {
			return nil, fmt.Errorf("recursive user tactic %s", name)
		}
	}
	compliant, err := p.context.Compliant()
	if err != nil {
		return nil, err
	}
	subscanner, err := NewScanner(strings.NewReader(body+" "), compliant, p.scanner.Numbers())
	if err != nil {
		return nil, err
	}
	previousScanner, previousToken, previousEnd := p.scanner, p.token, p.cacheEnd
	p.scanner, p.token, p.cacheEnd = subscanner, NullToken, 0
	p.tacticExpansion = append(p.tacticExpansion, name)
	defer func() {
		p.tacticExpansion = p.tacticExpansion[:len(p.tacticExpansion)-1]
		p.scanner, p.token, p.cacheEnd = previousScanner, previousToken, previousEnd
		err = errors.Join(err, subscanner.Close())
		if err != nil && result != nil {
			err = errors.Join(err, result.DecRef())
			result = nil
		}
	}()
	if err = p.advance(); err != nil {
		return nil, err
	}
	result, err = p.parseTactic()
	if err != nil {
		return nil, err
	}
	if p.token != EOFToken {
		err = errors.Join(fmt.Errorf("invalid user tactic %s: trailing input", name), result.DecRef())
		result = nil
		return nil, err
	}
	return result, nil
}

// parseTacticDeclaration reads declare-tactic's CPK_SEXPR argument and,
// as declare_tactic_cmd::execute does, converts it once to check it. The
// body is kept as its source text.
func (p *commandParser) parseTacticDeclaration() (selected tactic.Tactic, body string, err error) {
	var tree *tacticSExpr
	if p.token == LeftParenToken {
		p.scanner.StartCaching()
		defer func() { p.scanner.StopCaching(); p.scanner.ResetCache() }()
		if tree, err = p.readTacticSExpr(); err != nil {
			return nil, "", err
		}
		if body, err = p.scanner.CachedString(0, p.cacheEnd); err != nil {
			return nil, "", errors.Join(err, tree.release())
		}
		body = "(" + body
	} else {
		if p.token == SymbolToken {
			body = p.scanner.ID()
		}
		if tree, err = p.readTacticSExpr(); err != nil {
			return nil, "", err
		}
	}
	defer func() { err = errors.Join(err, tree.release()) }()
	selected, err = p.sexprToTactic(tree)
	return selected, body, err
}

// commandParameter is parse_next_cmd_arg for a parametric command's
// keyword argument: its own texts for the token checks, then the value.
func (p *commandParser) commandParameter(c *uparams.ParamsContext, settings *uparams.ParamsRef, key string, kind uparams.ParamKind) error {
	switch kind {
	case uparams.ParamBool:
		if p.token != SymbolToken || p.scanner.ID() != "true" && p.scanner.ID() != "false" {
			return fmt.Errorf("invalid command argument, true/false expected")
		}
	case uparams.ParamUInt:
		if p.token != IntegerToken {
			return fmt.Errorf("invalid command argument, unsigned integer expected")
		}
	case uparams.ParamNumeral:
		if p.token != IntegerToken && p.token != DecimalToken {
			return fmt.Errorf("invalid command argument, numeral expected")
		}
	}
	return p.tacticParameter(c, settings, key, kind)
}

func (p *commandParser) tacticParameter(c *uparams.ParamsContext, settings *uparams.ParamsRef, key string, kind uparams.ParamKind) (err error) {
	switch kind {
	case uparams.ParamBool:
		if p.token != SymbolToken || p.scanner.ID() != "true" && p.scanner.ID() != "false" {
			return fmt.Errorf("invalid parameter value, true or false expected")
		}
		if err = c.SetBoolName(settings, &key, p.scanner.ID() == "true"); err != nil {
			return err
		}
	case uparams.ParamUInt:
		if p.token != IntegerToken && p.token != DecimalToken {
			return fmt.Errorf("invalid parameter value, unsigned integer expected")
		}
		n, ok, e := p.scannerUnsigned()
		if e != nil {
			return e
		}
		if !ok {
			return fmt.Errorf("invalid parameter value, unsigned integer expected")
		}
		if err = c.SetUIntName(settings, &key, n); err != nil {
			return err
		}
	case uparams.ParamDouble, uparams.ParamNumeral:
		if p.token != IntegerToken && p.token != DecimalToken {
			return fmt.Errorf("invalid parameter value, numeral expected")
		}
		n := p.scanner.Number()
		if kind == uparams.ParamDouble {
			// rational::get_double
			value, e := p.scanner.Numbers().Float64(n)
			if e != nil {
				return e
			}
			if err = c.SetDoubleName(settings, &key, value); err != nil {
				return err
			}
		} else {
			r, e := c.Numerals().NewValue()
			if e != nil {
				return e
			}
			defer func() { err = errors.Join(err, c.Numerals().Del(&r)) }()
			if err = c.Numerals().Manager.Copy(&r.Raw, n); err != nil {
				return err
			}
			if err = c.SetRatName(settings, &key, &r); err != nil {
				return err
			}
		}
	case uparams.ParamSymbol:
		if p.token != SymbolToken {
			return fmt.Errorf("invalid parameter value, symbol expected")
		}
		sym, e := c.SymbolsField.Intern(p.scanner.ID())
		if e != nil {
			return e
		}
		if err = c.SetSymbolName(settings, &key, sym); err != nil {
			return err
		}
	case uparams.ParamString:
		if p.token != StringToken {
			return fmt.Errorf("invalid parameter value, string expected")
		}
		value := p.scanner.String()
		if err = c.SetStringName(settings, &key, &value); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid using-params combinator, unsupported parameter kind")
	}
	return p.advance()
}

func (p *commandParser) checkSatUsing(execute bool) (err error) {
	if p.token == RightParenToken {
		return fmt.Errorf("check-sat-using needs a tactic argument")
	}
	// The tactic is a CPK_SEXPR argument, converted by sexpr2tactic when
	// the command executes.
	tree, err := p.readTacticSExpr()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, tree.release()) }()
	globals, err := p.context.GlobalParameters()
	if err != nil {
		return err
	}
	c := globals.Context
	options, err := c.NewValue()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, c.Del(&options)) }()
	printStatistics, printUnsatCore, printProof, printModel := false, false, false, false
	for p.token == KeywordToken {
		key, e := p.parametricKeyword()
		if e != nil {
			return e
		}
		kind := uparams.ParamKind(0)
		switch key {
		case "timeout", "max_memory":
			kind = uparams.ParamUInt
		case "print_statistics":
			kind = uparams.ParamBool
		case "print_unsat_core":
			kind = uparams.ParamBool
		case "print_proof":
			kind = uparams.ParamBool
		case "print_model":
			kind = uparams.ParamBool
		default:
			return fmt.Errorf("invalid keyword argument")
		}
		if err = p.advance(); err != nil {
			return err
		}
		if err = p.commandParameter(c, &options, key, kind); err != nil {
			return err
		}
		if kind == uparams.ParamBool {
			value, e := c.GetBoolName(&options, &key, false)
			if e != nil {
				return e
			}
			switch key {
			case "print_statistics":
				printStatistics = value
			case "print_unsat_core":
				printUnsatCore = value
			case "print_proof":
				printProof = value
			case "print_model":
				printModel = value
			}
		}
	}
	if err = p.requireEnd(); err != nil {
		return err
	}
	if !execute {
		return nil
	}
	selected, err := p.sexprToTactic(tree)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, selected.DecRef()) }()
	status, validate, err := p.context.CheckSatUsingDeferred(selected, &options)
	tacticFailure := false
	if err != nil {
		var failure cmd_context.TacticCommandFailure
		if errors.As(err, &failure) {
			_, err = fmt.Fprintf(p.out, "(error \"tactic failed: %s\")\n", failure.Error())
			if err != nil {
				return err
			}
			status = lbool.Undef
			tacticFailure = true
		} else {
			return err
		}
	}
	if !tacticFailure {
		text := "unknown"
		if status == lbool.True {
			text = "sat"
		} else if status == lbool.False {
			text = "unsat"
		}
		if _, err = fmt.Fprintln(p.out, text); err != nil {
			return err
		}
	}
	if err = p.context.ValidateInfoStatus(status); err != nil {
		return err
	}
	produceModels, produceProofs, produceCores, err := p.context.ResultProductionEnabled()
	if err != nil {
		return err
	}
	// print_unsat_core prints the linearized core whatever the status.
	if printUnsatCore && produceCores {
		core := p.context.CheckSatUsingCore()
		if _, err = fmt.Fprint(p.out, "(unsat-core"); err != nil {
			return err
		}
		for _, expression := range core {
			if _, err = fmt.Fprint(p.out, " "); err != nil {
				return err
			}
			if err = p.context.WriteExpression(p.out, expression); err != nil {
				return err
			}
		}
		if _, err = fmt.Fprintln(p.out, ")"); err != nil {
			return err
		}
	}
	if printModel && produceModels {
		if err = p.context.WriteModel(p.out); err != nil && err.Error() != "model is not available" {
			return err
		}
	}
	// if (result->status() == l_true) ctx.validate_model(); after the
	// result, the core and the model are printed, before the proof.
	if validate {
		if err = p.context.ValidateCheckSatUsingModel(); err != nil {
			return err
		}
	}
	if printProof && produceProofs && status == lbool.False {
		if err = p.context.WriteProof(p.out); err != nil && err.Error() != "proof is not available" {
			return err
		}
	}
	if printStatistics {
		return p.context.WriteStatistics(p.out)
	}
	return nil
}
