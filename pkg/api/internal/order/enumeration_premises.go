package order

import (
	"errors"
	"slices"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

// EnumerationPremises retains the language of one solver's original premises.
// Handles are weak identity keys. The solver owns the live assertions.
type EnumerationPremises struct {
	manager          *ast.ASTManager
	base             []ast.ASTHandle
	language         *enumerationLanguage
	premises         string
	quantifiedSearch bool
}

func (p *EnumerationPremises) Matches(m *ast.ASTManager, base []ast.ASTHandle) bool {
	return p.manager == m && slices.Equal(p.base, base)
}

func (p *EnumerationPremises) prepare(m *ast.ASTManager, base []ast.ASTHandle) error {
	*p = EnumerationPremises{manager: m, base: slices.Clone(base)}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	dt, err := m.DatatypeFamilyID()
	if err != nil {
		return err
	}
	l := &enumerationLanguage{manager: m, boolean: boolean, datatype: dt}
	w := enumerationWriter{language: l}
	forms, err := w.assertions(base)
	if err != nil {
		return err
	}
	// Small bases were cheaper in the ordinary solver in the held-out
	// verifier runs. This threshold only selects a private proof attempt.
	if l.enumerations < 3 || w.binders < 20 || len(forms)+l.declarations.Len()+l.axiomBytes+10*len(l.axioms) > maxEnumerationBytes {
		return inapplicable
	}
	p.quantifiedSearch = w.quantifiedSearch
	dependencies, err := discoverPredicateDependencies(l, base)
	if err != nil {
		return err
	}
	if dependencies != nil {
		forms, err = dependencies.reduce(l, base, forms)
		if err != nil {
			return err
		}
	}
	var premises strings.Builder
	for _, axiom := range l.axioms {
		premises.WriteString("(assert ")
		premises.WriteString(axiom)
		premises.WriteString(")\n")
	}
	premises.WriteString(forms)
	p.premises = premises.String()
	l.axioms = nil
	p.language = l
	return nil
}

// Project returns a closed first-order problem with the same finite domains.
// A goal may introduce its own sorts. Their axioms are scoped together with
// that goal, so neither they nor its weak handles accumulate across siblings.
func (p *EnumerationPremises) Project(m *ast.ASTManager, forms []ast.ASTHandle) (*Problem, error) {
	if len(forms) < 2 || len(forms) > 8192 || m.Limit.CancelFlag() {
		return nil, nil
	}
	base := forms[:len(forms)-1]
	if !p.Matches(m, base) {
		if err := p.prepare(m, base); err != nil {
			if !errors.Is(err, inapplicable) || m.Limit.CancelFlag() {
				*p = EnumerationPremises{}
			}
			if errors.Is(err, inapplicable) {
				return nil, nil
			}
			return nil, err
		}
	}
	if p.language == nil {
		return nil, nil
	}
	goal := forms[len(base)]
	if !p.quantifiedSearch {
		quantified, err := m.HasQuantifiers(goal)
		if err != nil || !quantified {
			return nil, err
		}
	}
	l := enumerationLanguage{manager: m, boolean: p.language.boolean, datatype: p.language.datatype, base: p.language}
	w := enumerationWriter{language: &l}
	var term strings.Builder
	s, err := w.write(&term, goal, 0)
	if errors.Is(err, inapplicable) {
		return nil, nil
	}
	if err != nil || s != l.boolean || m.Limit.CancelFlag() || !p.quantifiedSearch && !w.quantifiedSearch {
		return nil, err
	}
	size := p.language.declarations.Len() + l.declarations.Len() + len(p.premises) + term.Len() + l.axiomBytes + len(l.axioms) + 18
	if size > maxEnumerationBytes {
		return nil, nil
	}
	var script strings.Builder
	script.Grow(size)
	script.WriteString(p.language.declarations.String())
	script.WriteString(l.declarations.String())
	problem := &Problem{DeclarationCount: p.language.declCount + l.declCount, AssertionsStart: script.Len()}
	script.WriteString(p.premises)
	problem.LastAssertionStart = script.Len()
	script.WriteString("(assert ")
	if len(l.axioms) != 0 {
		script.WriteString("(and ")
		for _, axiom := range l.axioms {
			script.WriteString(axiom)
			script.WriteByte(' ')
		}
	}
	script.WriteString(term.String())
	if len(l.axioms) != 0 {
		script.WriteByte(')')
	}
	script.WriteString(")\n")
	problem.Text = script.String()
	return problem, nil
}
