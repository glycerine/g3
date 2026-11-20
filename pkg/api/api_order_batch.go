package api

import (
	"errors"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

// Only the internal order renderer supplies these declarations. Sorts,
// functions and constants all put the symbol in the second field.
func orderDeclarationName(line string) string {
	_, tail, _ := strings.Cut(line, " ")
	name, _, _ := strings.Cut(tail, " ")
	return name
}

func (p *orderProofState) needsNewDeclarations(problem order.Problem) bool {
	if p.parser == nil {
		return false
	}
	// A smaller successor should not keep a much larger language alive.
	if len(p.declared) > 2*problem.DeclarationCount {
		return true
	}
	// Inference numbers functions afresh on each query. The same printed
	// name may now have a different arity, domain or range.
	for line := range strings.Lines(problem.Declarations()) {
		if previous, ok := p.declared[orderDeclarationName(line)]; ok && previous != line {
			return true
		}
	}
	return false
}

func (p *orderProofState) closeParser() error {
	if p.parser != nil {
		if err := p.parser.Close(); err != nil {
			return err
		}
		p.parser = nil
	}
	p.declared = nil
	p.premises = ""
	return nil
}

func (p *orderProofState) loadBatch(problem order.Problem) (err error) {
	if p.parser == nil {
		p.parser, err = p.ctx.newSMT2ParserContext(SMT2Bindings{})
		if err != nil {
			return err
		}
		p.parser.GlobalDeclarations = true
		p.declared = make(map[string]string)
	}
	var declarations strings.Builder
	for line := range strings.Lines(problem.Declarations()) {
		if _, exists := p.declared[orderDeclarationName(line)]; exists {
			continue
		}
		// Clone only the declaration. Keeping a slice of problem.Text here
		// would keep every assertion from the introducing query alive.
		declaration := strings.Clone(line)
		p.declared[orderDeclarationName(declaration)] = declaration
		declarations.WriteString(line)
	}
	matches := strings.HasPrefix(problem.Premises(), p.premises)
	suffix := problem.Assertions()
	if matches {
		suffix = suffix[len(p.premises):]
	}
	var input io.Reader = strings.NewReader(suffix)
	if declarations.Len() != 0 {
		input = io.MultiReader(strings.NewReader(declarations.String()), input)
	}
	defer func() { err = errors.Join(err, p.parser.ResetAssertions()) }()
	if err := smt2.ParseAssertions(p.parser, input); err != nil {
		return err
	}
	current, err := p.parser.Assertions()
	if err != nil {
		return err
	}
	if len(current) != strings.Count(suffix, "\n") {
		return ErrSMTContextOwner
	}
	if !matches || p.premises == "" {
		// The parser roots the incoming formulas while the old base is removed.
		// Every attempt is a conjunction. Its final assertion is always scoped.
		if p.premises != "" {
			if err := p.child.Pop(1); err != nil {
				return err
			}
			p.premises = ""
		}
		if len(current) > 1 {
			if err := p.child.Push(); err != nil {
				return err
			}
			for _, formula := range current[:len(current)-1] {
				if err := p.child.Assert(formula); err != nil {
					return err
				}
			}
			p.premises = strings.Clone(problem.Premises())
			current = current[len(current)-1:]
		}
	}
	if err := p.child.Push(); err != nil {
		return err
	}
	for _, formula := range current {
		if err := p.child.Assert(formula); err != nil {
			return err
		}
	}
	return nil
}
