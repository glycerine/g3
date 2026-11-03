package fp_test

// Source: src/muz/base/dl_engine_base.h engine_base::display_certificate.
// rel_context inherits this unconditional unsupported-operation default.

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogDefaultCertificateRefusesWithoutChangingResult(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("datalog"))
	p, application := routingPredicate(t, engine, manager, "certificate-p")
	u, _ := routingPredicate(t, engine, manager, "certificate-u")
	fpMust(t, engine.AddRule(application, "fact"))
	refuse := func(declaration ast.ASTHandle) {
		t.Helper()
		before := manager.Stats()
		var output bytes.Buffer
		formatted := false
		manager.Budget().FailAfter(0)
		found, err := engine.WriteCertificate(&output, declaration, func(io.Writer, ast.ASTHandle) error {
			formatted = true
			return io.ErrClosedPipe
		})
		manager.Budget().FailAfter(-1)
		if found || !errors.Is(err, fp.ErrPropositionalEngine) || output.Len() != 0 || formatted {
			t.Fatal("unsupported certificate", found, output.String(), formatted, err)
		}
		after := manager.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
			t.Fatal("unsupported certificate changed AST ownership", before, after)
		}
	}
	refuse(p)
	for _, test := range []struct {
		declaration ast.ASTHandle
		want        lbool.LBool
	}{{p, lbool.True}, {u, lbool.False}, {p, lbool.True}} {
		status, err := engine.QueryPredicate(test.declaration)
		fpMust(t, err)
		if status != test.want {
			t.Fatal("query result", status, test.want)
		}
		refuse(test.declaration)
		last, err := engine.LastResult()
		fpMust(t, err)
		if last != status {
			t.Fatal("certificate refusal changed result", last, status)
		}
		answer, err := engine.Answer(test.declaration)
		fpMust(t, err)
		handle, err := manager.RefGet(answer)
		fpMust(t, err)
		if manager.IsTrue(handle) != (status == lbool.True) || manager.IsFalse(handle) != (status == lbool.False) {
			t.Fatal("certificate refusal changed answer", handle, status)
		}
		fpMust(t, manager.DelRef(&answer))
	}
}
