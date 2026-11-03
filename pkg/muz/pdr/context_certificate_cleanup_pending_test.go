package pdr

// Source: context::display_certificate in src/muz/pdr/pdr_context.cpp.

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type pdrCertificateCleanupFailureWriter struct{}

func (pdrCertificateCleanupFailureWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestContextDisplayCertificatePreservesResultOwnersOnNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var context *Context
	var query ast.ASTHandle
	var attempt *pdrValidationAttempt
	var certificateErr error
	var receipt memory.ConstructionFailure
	active, callback := false, false
	var runtime *memory.Runtime
	var err error
	runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && !callback {
			callback = true
			// The allocation refusal exists only to enter a native AST execution
			// guard. Certificate construction itself must remain unconstrained.
			if resetErr := runtime.SetMaxCount(0); resetErr != nil {
				certificateErr = resetErr
				return source
			}
			certificateErr = finishPDRValidation(attempt, io.ErrClosedPipe)
		}
		return source
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	parameters, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, parameters, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	context, err = NewContext(manager, nil, 2, ReachableNoCache, smtcore.SMTSolverConfig{Params: parameters, Globals: globals})
	must(err)
	ruleManager, err := base.NewRuleManager(manager)
	must(err)
	rules, err := base.NewRuleSet(ruleManager)
	must(err)
	declaration := func(name string) ast.ASTHandle {
		boolean, declarationErr := manager.MkBoolSort()
		must(declarationErr)
		sym, declarationErr := symbols.Intern(name)
		must(declarationErr)
		result, declarationErr := manager.MkFuncDecl(sym, nil, boolean, ast.ASTFuncDeclInfo{})
		must(declarationErr)
		must(ruleManager.RegisterPredicate(result))
		return result
	}
	fact := declaration("certificate-cleanup-fact")
	query = declaration("certificate-cleanup-query")
	addRule := func(head ast.ASTHandle, tail []ast.ASTHandle, name string) {
		application, ruleErr := manager.MkApp(head, nil)
		must(ruleErr)
		rule, ruleErr := ruleManager.NewRule(application, tail, nil, name)
		must(ruleErr)
		must(rules.Add(rule))
		must(rule.Close())
	}
	addRule(fact, nil, "certificate-cleanup-fact")
	falsity, err := manager.MkFalse()
	must(err)
	addRule(query, []ast.ASTHandle{falsity}, "certificate-cleanup-blocked-query")
	must(context.Initialize(rules))
	status, err := context.Solve(query)
	must(err)
	if status != lbool.False {
		t.Fatal("certificate cleanup fixture result", status)
	}
	if err = context.DisplayCertificate(pdrCertificateCleanupFailureWriter{}, query, false); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("public certificate writer failure", err)
	}
	certificate, err := context.finiteFrameModel(context.inductive, query)
	must(err)
	attempt = newPDRValidationAttempt(manager)
	attempt.modelOwners = append(attempt.modelOwners, &pdrModelExportOwner{manager: manager, model: certificate})

	value, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	writerFailure := errors.Is(certificateErr, io.ErrClosedPipe)
	pending := errors.Is(certificateErr, ast.ErrASTPending)
	hasReceipt := errors.As(certificateErr, &receipt)
	owner := hasReceipt && receipt.RuntimeOwner() == runtime
	if body == nil || !callback || !writerFailure || !pending || !hasReceipt || !owner {
		t.Fatal("certificate cleanup refusal did not preserve the model", body, callback, certificateErr, writerFailure, pending, hasReceipt, owner)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if receipt.CleanupCause() != nil {
		t.Fatal("certificate cleanup retry remained pending", receipt.CleanupCause())
	}

	// The reachable branch owns its Answer reference through the same source
	// write-and-release boundary. Exercise that scalar owner separately because
	// constructing Answer while an AST execution guard is active is forbidden.
	factApplication, err := manager.MkApp(fact, nil)
	must(err)
	answer, err := manager.NewRef(factApplication)
	must(err)
	attempt = newPDRValidationAttempt(manager)
	attempt.references = append(attempt.references, answer)
	callback = false
	certificateErr = nil
	receipt = nil
	must(numbers.SetString(&value, "18446744073709551629/18446744073709551653"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var answerInvocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body = manager.ArithmeticNumeralWithInvocation(&value, false, &answerInvocation)
	active = false
	must(runtime.SetMaxCount(0))
	writerFailure = errors.Is(certificateErr, io.ErrClosedPipe)
	pending = errors.Is(certificateErr, ast.ErrASTPending)
	hasReceipt = errors.As(certificateErr, &receipt)
	owner = hasReceipt && receipt.RuntimeOwner() == runtime
	if body == nil || !callback || !writerFailure || !pending || !hasReceipt || !owner || len(attempt.references) != 1 {
		t.Fatal("reachable certificate cleanup refusal did not preserve the answer", body, callback, certificateErr, writerFailure, pending, hasReceipt, owner)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&answerInvocation))
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if receipt.CleanupCause() != nil || len(attempt.references) != 0 {
		t.Fatal("reachable certificate answer retry remained pending", receipt.CleanupCause(), len(attempt.references))
	}

	var output bytes.Buffer
	must(context.DisplayCertificate(&output, query, false))
	if output.Len() == 0 {
		t.Fatal("certificate retry produced no model")
	}

	must(numbers.Del(&value))
	must(context.Close())
	must(rules.Close())
	must(ruleManager.Close())
	must(manager.Close())
	must(globals.Close())
	must(catalog.Close())
	must(parameters.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("certificate cleanup fixture leaked bytes", budget.Used())
	}
}
