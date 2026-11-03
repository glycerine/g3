package base

// Source: src/muz/base/dl_context.cpp:1120-1241 context::display_smt2.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/smt2"
)

// ContextSMT2Config contains the source print switches and borrowed inputs.
// Use DefaultContextSMT2Config for the fixedpoint parameter defaults.
type ContextSMT2Config struct {
	FixedpointExtensions bool
	LowLevel             bool
	DeclareVariables     bool
	Background           []ast.ASTHandle
	Queries              []ast.ASTHandle
	// PendingRules are closed source input formulas not yet flushed into the
	// rule set. PendingNames has one name per borrowed formula.
	PendingRules []ast.ASTHandle
	PendingNames []string
}

func DefaultContextSMT2Config() ContextSMT2Config {
	return ContextSMT2Config{FixedpointExtensions: true, DeclareVariables: true}
}

// WriteContextSMT2 dumps finalized rules with the source output-query and
// printing policies, followed by any borrowed pending closed input formulas.
func (s *RuleSet) WriteContextSMT2(out io.Writer, config ContextSMT2Config) (resultErr error) {
	if out == nil {
		return io.ErrClosedPipe
	}
	if err := s.open(); err != nil {
		return err
	}
	m := s.manager.manager
	var rules, queries []ast.ASTRef
	var names []string
	owner := &contextSMT2Owner{manager: m}
	defer func() {
		owner.refs = append(owner.refs, rules...)
		owner.refs = append(owner.refs, queries...)
		resultErr = owner.finish(resultErr)
	}()
	if err := s.AppendDisplayFormulas(&rules, &queries, &names); err != nil {
		return err
	}
	if len(config.PendingRules) != len(config.PendingNames) {
		return errors.New("muz/base: pending rule/name count mismatch")
	}
	for i, h := range config.PendingRules {
		r, err := m.NewRef(h)
		if err != nil {
			return err
		}
		rules = append(rules, r)
		names = append(names, config.PendingNames[i])
	}
	for _, h := range config.Queries {
		r, err := m.NewRef(h)
		if err != nil {
			return err
		}
		queries = append(queries, r)
	}
	visitor, err := ast.NewASTPPUtil(m)
	if err != nil {
		return err
	}
	fresh := model.NewFreshName(m)
	collect := func(h ast.ASTHandle) error {
		if err := visitor.Collect(h); err != nil {
			return err
		}
		for {
			kind, err := m.GetKind(h)
			if err != nil {
				return err
			}
			if kind != ast.ASTQuantifier {
				break
			}
			h, err = m.GetQuantifierBody(h)
			if err != nil {
				return err
			}
		}
		return fresh.AddAST(h)
	}
	for _, h := range config.Background {
		if err = collect(h); err != nil {
			return err
		}
	}
	for _, refs := range [][]ast.ASTRef{rules, queries} {
		for _, r := range refs {
			h, e := m.RefGet(r)
			if e != nil {
				return e
			}
			if err = collect(h); err != nil {
				return err
			}
		}
	}
	declarations, err := visitor.CollectedDeclarations()
	if err != nil {
		return err
	}
	var relations []ast.ASTHandle
	if config.FixedpointExtensions {
		for _, d := range declarations {
			family, e := m.GetFamilyID(d)
			if e != nil {
				return e
			}
			if _, registered := s.manager.predicates[d]; family == ast.NullFamilyID && registered {
				relations = append(relations, d)
				if err = visitor.RemoveDeclaration(d); err != nil {
					return err
				}
			}
		}
	}
	if !config.FixedpointExtensions {
		if _, err = io.WriteString(out, "(set-logic HORN)\n"); err != nil {
			return err
		}
	}
	if err = visitor.DisplayDeclarations(out); err != nil {
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(m)
	if err != nil {
		return err
	}
	for _, d := range relations {
		if err = s.writeContextRelation(out, printer, d); err != nil {
			return err
		}
	}
	if config.FixedpointExtensions && config.DeclareVariables {
		if err = s.declareContextVariables(out, printer, rules, fresh); err != nil {
			return err
		}
	}
	if len(config.Background) > 0 && !config.FixedpointExtensions {
		return errors.New("Background axioms cannot be used with SMT-LIB2 HORN format")
	}
	for _, h := range config.Background {
		if _, err = io.WriteString(out, "(assert "); err != nil {
			return err
		}
		if err = s.writeContextExpression(out, printer, h, config.LowLevel); err != nil {
			return err
		}
		if _, err = io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	for i, r := range rules {
		prefix := "(assert "
		if config.FixedpointExtensions {
			prefix = "(rule "
		}
		if _, err = io.WriteString(out, prefix); err != nil {
			return err
		}
		if names[i] != "" {
			if _, err = io.WriteString(out, "(! "); err != nil {
				return err
			}
		}
		h, e := m.RefGet(r)
		if e != nil {
			return e
		}
		if err = s.writeContextExpression(out, printer, h, config.LowLevel); err != nil {
			return err
		}
		if names[i] != "" {
			if _, err = io.WriteString(out, " :named "); err != nil {
				return err
			}
			name := names[i]
			for {
				symbol, e := m.Symbols.Intern(name)
				if e != nil {
					return e
				}
				if !fresh.Contains(symbol) {
					fresh.AddSymbol(symbol)
					break
				}
				name += "!"
			}
			if err = s.writeContextSymbol(out, name); err != nil {
				return err
			}
			if _, err = io.WriteString(out, ")"); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	for _, r := range queries {
		h, e := m.RefGet(r)
		if e != nil {
			return e
		}
		if config.FixedpointExtensions {
			if err = s.writeContextQuery(out, printer, h, config.LowLevel); err != nil {
				return err
			}
		} else {
			if len(queries) > 1 {
				if _, err = io.WriteString(out, "(push)\n"); err != nil {
					return err
				}
			}
			if _, err = io.WriteString(out, "(assert "); err != nil {
				return err
			}
			err = func() (resultErr error) {
				queryOwner := &contextSMT2Owner{manager: m}
				defer func() { resultErr = queryOwner.finish(resultErr) }()
				owned, e := queryOwner.reserve()
				if e != nil {
					return e
				}
				not, e := m.MkNot(h)
				if e = errors.Join(e, m.SetRef(owned, not)); e != nil {
					return e
				}
				return s.writeContextExpression(out, printer, not, config.LowLevel)
			}()
			if err != nil {
				return err
			}
			if _, err = io.WriteString(out, ")\n(check-sat)\n"); err != nil {
				return err
			}
			if len(queries) > 1 {
				if _, err = io.WriteString(out, "(pop)\n"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *RuleSet) writeContextSymbol(out io.Writer, name string) error {
	m := s.manager.manager
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return err
	}
	quote, err := smt2.IsSMT2QuotedSymbol(m.Symbols, symbol)
	if err != nil {
		return err
	}
	if quote {
		name, err = smt2.MkSMT2QuotedSymbol(m.Symbols, symbol, nil)
		if err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, name)
	return err
}
func (s *RuleSet) writeContextRelation(out io.Writer, printer *ast.LegacySMTPrinter, d ast.ASTHandle) error {
	m := s.manager.manager
	symbol, err := m.GetASTName(d)
	if err != nil {
		return err
	}
	name, err := m.Symbols.String(symbol)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "(declare-rel "); err != nil {
		return err
	}
	if err = s.writeContextSymbol(out, name); err != nil {
		return err
	}
	if _, err = io.WriteString(out, " ("); err != nil {
		return err
	}
	arity, err := m.GetArity(d)
	if err != nil {
		return err
	}
	for i := uint32(0); i < arity; i++ {
		if i > 0 {
			if _, err = io.WriteString(out, " "); err != nil {
				return err
			}
		}
		sort, e := m.GetDomain(d, i)
		if e != nil {
			return e
		}
		if err = s.writeContextSort(out, sort); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, "))\n")
	return err
}
