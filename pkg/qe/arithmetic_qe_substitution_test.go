package qe_test

// Source: src/qe/qe_arith_plugin.cpp x_subst and branch_formula.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
)

func TestArithmeticQEXSubstitutionTermAndCoefficient(t *testing.T) {
	f := newArithmeticQEFixture(t)
	substitution, err := qe.NewArithmeticXSubstitution(f.util)
	pluginMust(t, err)
	defer func() { pluginMust(t, substitution.Close()) }()
	pluginMust(t, substitution.SetTerm(f.y))
	pluginMust(t, substitution.SetCoefficient(big.NewRat(3, 1)))
	term, err := substitution.MakeTerm(big.NewRat(2, 1), f.x)
	pluginMust(t, err)
	h, err := f.manager.RefGet(term)
	pluginMust(t, err)
	if kind, err := f.manager.GetDeclKind(h); err != nil || kind != 6 {
		t.Fatal("substitution term kind", kind, err)
	}
	pluginMust(t, f.manager.DelRef(&term))
	coefficient, err := substitution.MakeCoefficient(big.NewRat(4, 1))
	pluginMust(t, err)
	if coefficient.Cmp(big.NewRat(12, 1)) != 0 {
		t.Fatal("substitution coefficient", coefficient)
	}
	copyOwner := *substitution
	if _, err = copyOwner.Coefficient(); err == nil {
		t.Fatal("copied substitution accepted")
	}
}

func TestArithmeticQEBranchFormulaCacheIdentityAndOwnership(t *testing.T) {
	f := newArithmeticQEFixture(t)
	formula := f.pin(f.manager.MkEq(f.x, f.y))
	result := f.pin(f.manager.MkArithmeticLE(f.x, f.y))
	record, err := qe.NewArithmeticBranchFormula(f.manager, formula, f.x, 7, result, big.NewRat(5, 2), f.y, []ast.ASTHandle{f.rx, f.ry})
	pluginMust(t, err)
	cache, err := qe.NewArithmeticBranchCache(f.manager)
	pluginMust(t, err)
	pluginMust(t, cache.Insert(record))
	found, ok, err := cache.Find(qe.ArithmeticBranchKey{Formula: formula, Variable: f.x, Branch: 7})
	pluginMust(t, err)
	if !ok || found != record || found.Coefficient.Cmp(big.NewRat(5, 2)) != 0 || len(found.Variables) != 2 {
		t.Fatal("branch formula cache lookup")
	}
	if err = cache.Insert(record); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("duplicate branch key error", err)
	}
	pluginMust(t, cache.Close())
}
