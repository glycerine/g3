package bmc_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"math/big"
	"testing"
)

// C++ qlinear::get_model requires a fact at level zero and a single
// predecessor at every positive level, even when the model selects a fact.
func TestQuantifiedLinearTraceFactLevel(t *testing.T) {
	for _, tc := range []struct {
		name    string
		level   int64
		wantErr bool
	}{{"zero", 0, false}, {"positive", 1, true}} {
		t.Run(tc.name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, err := cmd_context.NewStandaloneContext()
			must(err)
			defer func() { must(ctx.Close()) }()
			m, err := ctx.Manager()
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("fact-level")
			must(err)
			predicate, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			head, err := m.MkApp(predicate, nil)
			must(err)
			rm, err := base.NewRuleManager(m)
			must(err)
			defer func() { must(rm.Close()) }()
			must(rm.RegisterPredicate(predicate))
			set, err := base.NewRuleSet(rm)
			must(err)
			defer func() { must(set.Close()) }()
			rule, err := rm.NewRule(head, nil, nil, "fact")
			must(err)
			must(set.Add(rule))
			must(rule.Close())
			md, err := model.NewModel(m)
			must(err)
			defer func() { must(md.Close()) }()
			indexSort, err := m.BitVectorSort(4)
			must(err)
			tName, err := m.Symbols.Intern("T")
			must(err)
			index, err := m.MkConst(tName, indexSort)
			must(err)
			tDecl, err := m.GetDecl(index)
			must(err)
			level, err := m.BitVectorNumeralRat(big.NewRat(tc.level, 1), 4)
			must(err)
			must(md.Core().RegisterConstant(tDecl, level))
			selectorName, err := m.Symbols.Intern("fact-level#0")
			must(err)
			selector, err := m.MkFuncDecl(selectorName, []ast.ASTHandle{indexSort}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			truth, err := m.MkTrue()
			must(err)
			fi, err := model.NewFunctionInterpretation(m, 1)
			must(err)
			must(fi.SetElse(truth))
			must(md.Core().RegisterFunction(selector, fi))
			services, err := ctx.ModelEvaluatorConfig()
			must(err)
			compiler, err := bmc.NewCompiler(m)
			must(err)
			defer func() { must(compiler.Close()) }()
			trace, err := compiler.QuantifiedLinearTrace(set, predicate, 4, md.Core(), services)
			if tc.wantErr {
				if !errors.Is(err, ast.ErrASTDomain) {
					t.Fatalf("positive-level fact: got %v, want AST domain failure", err)
				}
			} else {
				must(err)
				if len(trace) != 1 {
					t.Fatalf("zero-level fact: got %d rules", len(trace))
				}
			}
			for _, r := range trace {
				must(r.Close())
			}
		})
	}
}
