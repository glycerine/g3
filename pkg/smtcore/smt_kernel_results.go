package smtcore

// Source: src/smt/smt_context.cpp get_assignments,
// get_relevant_labeled_literals, get_relevant_literals, and
// get_guessed_literals.

import (
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (k *SMTKernel) ownLiteral(literal SMTLiteral) (ast.ASTHandle, error) {
	if literal.Var() < SMTFirstBoolVar || int(literal.Var()) >= len(k.internalizer.boolExpr) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	expression := k.internalizer.boolExpr[literal.Var()]
	var err error
	if literal.Sign() {
		expression, err = k.manager.MkNot(expression)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if err = k.manager.IncRef(expression); err != nil {
		return ast.ASTHandle{}, err
	}
	return expression, nil
}

// popToBaseLevel is context::pop_to_base_lvl. Literal queries read the live
// context, so leaving the search scopes also drops the Go snapshot of the
// last search (and the atoms it holds) before the scopes unwind.
func (k *SMTKernel) popToBaseLevel() error {
	if k.internalizer.scopeLevel > k.internalizer.baseLevel {
		if err := k.clearLiteralResults(); err != nil {
			return err
		}
	}
	return k.internalizer.popToBaseLevel()
}

func (k *SMTKernel) clearLiteralResults() error {
	if err := k.releasePendingResults(); err != nil {
		return err
	}
	for _, values := range []*[]ast.ASTHandle{&k.assignments, &k.relevantLiterals, &k.relevantLabeled, &k.guessed} {
		for index := range *values {
			root := &(*values)[index]
			if root.IsNull() {
				continue
			}
			consumed, err := k.manager.DecRefConsumed(*root)
			if consumed {
				*root = ast.ASTHandle{}
			}
			if err != nil {
				return err
			}
		}
		*values = nil
	}
	return nil
}

// smtPendingLiteralResult is one literal with the atom its variable had when
// the check ended; a query skips it if the variable has since been reused.
type smtPendingLiteralResult struct {
	literal SMTLiteral
	atom    ast.ASTHandle
}

type smtPendingLiteralResults struct {
	assignments, relevant, guessed []smtPendingLiteralResult
	labeled                        []ast.ASTHandle
	// held keeps the recorded atoms alive until the results are cleared, as
	// the eagerly built result expressions did; callers rely on it.
	held  []ast.ASTHandle
	built bool
	// unheld marks a capture after an unsatisfiable or unknown check: the
	// source keeps no reference, and a query after the variables were
	// reused skips them.
	unheld bool
}

func (k *SMTKernel) pendingLiteral(pending *smtPendingLiteralResults, literal SMTLiteral) (smtPendingLiteralResult, error) {
	atom := k.internalizer.boolExpr[literal.Var()]
	if pending.unheld {
		return smtPendingLiteralResult{literal: literal, atom: atom}, nil
	}
	if err := k.manager.IncRef(atom); err != nil {
		return smtPendingLiteralResult{}, err
	}
	pending.held = append(pending.held, atom)
	return smtPendingLiteralResult{literal: literal, atom: atom}, nil
}

func (k *SMTKernel) releasePendingResults() error {
	pending := k.pendingResults
	k.pendingResults = nil
	if pending == nil {
		return nil
	}
	var err error
	for _, atom := range pending.held {
		err = errors.Join(err, k.manager.DecRef(atom))
	}
	return err
}

// captureLiteralResults records the literals get_assignments,
// get_relevant_literals, get_guessed_literals and
// get_relevant_labeled_literals report after a satisfiable check. It creates
// no expressions; materializeLiteralResults builds them on the first query.
func (k *SMTKernel) captureLiteralResults(baseLevel uint32) error {
	return k.captureLiteralResultsHeld(baseLevel, true)
}

func (k *SMTKernel) captureLiteralResultsHeld(baseLevel uint32, held bool) error {
	if err := k.clearLiteralResults(); err != nil {
		return err
	}
	pending := &smtPendingLiteralResults{unheld: !held}
	k.pendingResults = pending
	for _, literal := range k.internalizer.assignedLiterals {
		if literal.Var() < SMTFirstBoolVar || int(literal.Var()) >= len(k.internalizer.boolExpr) {
			return ast.ErrASTDomain
		}
		item, err := k.pendingLiteral(pending, literal)
		if err != nil {
			return err
		}
		pending.assignments = append(pending.assignments, item)
		if k.relevancy.IsRelevant(k.internalizer.boolExpr[literal.Var()]) {
			if item, err = k.pendingLiteral(pending, literal); err != nil {
				return err
			}
			pending.relevant = append(pending.relevant, item)
		}
	}
	for level := int(baseLevel); level < len(k.internalizer.scopes); level++ {
		index := k.internalizer.scopes[level].assignedLimit
		if index < 0 || index >= len(k.internalizer.assignedLiterals) {
			continue
		}
		literal := k.internalizer.assignedLiterals[index]
		if k.internalizer.booleanData[literal.Var()].Justification().Kind() != SMTBooleanJustificationAxiom {
			continue
		}
		item, err := k.pendingLiteral(pending, literal)
		if err != nil {
			return err
		}
		pending.guessed = append(pending.guessed, item)
	}
	for variable := int32(SMTFirstBoolVar); int(variable) < len(k.internalizer.boolExpr); variable++ {
		expression := k.internalizer.boolExpr[variable]
		if !k.relevancy.IsRelevant(expression) {
			continue
		}
		value, err := k.internalizer.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return err
		}
		if value != lbool.True {
			continue
		}
		var names []usymbol.Symbol
		label, err := k.manager.IsLabelLit(expression, &names)
		if err != nil {
			return err
		}
		if label {
			if !pending.unheld {
				if err = k.manager.IncRef(expression); err != nil {
					return err
				}
				pending.held = append(pending.held, expression)
			}
			pending.labeled = append(pending.labeled, expression)
		}
	}
	return nil
}

// materializeLiteralResults builds the recorded result expressions, owned by
// the kernel until the results are cleared.
func (k *SMTKernel) materializeLiteralResults() error {
	pending := k.pendingResults
	if pending == nil || pending.built {
		return nil
	}
	pending.built = true
	fail := func(cause error) error { return errors.Join(cause, k.clearLiteralResults()) }
	build := func(items []smtPendingLiteralResult, out *[]ast.ASTHandle) error {
		for _, item := range items {
			v := item.literal.Var()
			if int(v) >= len(k.internalizer.boolExpr) || k.internalizer.boolExpr[v] != item.atom {
				continue
			}
			expression, err := k.ownLiteral(item.literal)
			if err != nil {
				return err
			}
			*out = append(*out, expression)
		}
		return nil
	}
	if err := build(pending.assignments, &k.assignments); err != nil {
		return fail(err)
	}
	if err := build(pending.relevant, &k.relevantLiterals); err != nil {
		return fail(err)
	}
	if err := build(pending.guessed, &k.guessed); err != nil {
		return fail(err)
	}
	for _, expression := range pending.labeled {
		if err := k.manager.IncRef(expression); err != nil {
			return fail(err)
		}
		k.relevantLabeled = append(k.relevantLabeled, expression)
	}
	return nil
}

func (k *SMTKernel) Assignments() ([]ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	if err := k.materializeLiteralResults(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), k.assignments...), nil
}

func (k *SMTKernel) RelevantLiterals() ([]ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	if err := k.materializeLiteralResults(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), k.relevantLiterals...), nil
}

func (k *SMTKernel) RelevantLabeledLiterals(atLabels bool) ([]ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	if err := k.materializeLiteralResults(); err != nil {
		return nil, err
	}
	if !atLabels {
		return append([]ast.ASTHandle(nil), k.relevantLabeled...), nil
	}
	result := make([]ast.ASTHandle, 0, len(k.relevantLabeled))
	for _, expression := range k.relevantLabeled {
		var names []usymbol.Symbol
		label, err := k.manager.IsLabelLit(expression, &names)
		if err != nil {
			return nil, err
		}
		include := false
		for _, name := range names {
			spelling, err := k.manager.Symbols.String(name)
			if err != nil {
				return nil, err
			}
			if strings.Contains(spelling, "@") {
				include = true
				break
			}
		}
		if label && include {
			result = append(result, expression)
		}
	}
	return result, nil
}

func (k *SMTKernel) GuessedLiterals() ([]ast.ASTHandle, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	if err := k.materializeLiteralResults(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), k.guessed...), nil
}

// captureUnsatLiteralResults records get_assignments' view after a check
// that ended unsatisfiable without reaching search's own capture: the
// literals the context still has assigned.
func (k *SMTKernel) captureUnsatLiteralResults() error {
	if k.pendingResults != nil {
		return nil
	}
	return k.captureLiteralResultsHeld(uint32(k.internalizer.scopeLevel), false)
}
