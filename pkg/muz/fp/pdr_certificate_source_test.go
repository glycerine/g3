package fp

// Source: pdr::context::display_certificate and get_answer.

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type pdrCertificateRefusingWriter struct{}

func (pdrCertificateRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestPDRBoogieCertificateParameterSelectsSourcePresentation(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "boogie"}[enabled], func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			e := f.engine
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			value, err := e.params.NewValue()
			must(err)
			defer func() { must(e.params.Del(&value)) }()
			key := "print_boogie_certificate"
			must(e.params.SetBoolName(&value, &key, enabled))
			must(e.SetQueryParameters(&value))
			status, err := e.QueryPredicate(f.declarations["p"])
			must(err)
			if status != lbool.True {
				t.Fatal("fact query", status)
			}
			var output bytes.Buffer
			ordinaryCalls := 0
			found, err := e.WriteCertificate(&output, f.declarations["p"], func(out io.Writer, _ ast.ASTHandle) error {
				ordinaryCalls++
				_, err := io.WriteString(out, "ordinary")
				return err
			})
			must(err)
			if !found {
				t.Fatal("PDR certificate unavailable")
			}
			if enabled {
				if ordinaryCalls != 0 || !strings.Contains(output.String(), "(derivation\n") || !strings.Contains(output.String(), "(step\n") {
					t.Fatalf("Boogie certificate route: calls=%d output=%q", ordinaryCalls, output.String())
				}
				found, err = e.WriteCertificate(pdrCertificateRefusingWriter{}, f.declarations["p"], func(io.Writer, ast.ASTHandle) error {
					t.Fatal("Boogie route invoked ordinary expression writer")
					return nil
				})
				if !found || !errors.Is(err, io.ErrClosedPipe) {
					t.Fatal("Boogie writer refusal", found, err)
				}
			} else if ordinaryCalls != 1 || output.String() != "ordinary" {
				t.Fatalf("ordinary certificate route: calls=%d output=%q", ordinaryCalls, output.String())
			}
		})
	}
}

func TestPDRUnknownAnswerAndCertificateMatchSource(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, manager := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	answer, handled, err := e.pdrUnknownAnswer(lbool.Undef)
	must(err)
	if !handled {
		t.Fatal("unknown PDR answer status was not handled")
	}
	handle, err := manager.RefGet(answer)
	must(err)
	if !manager.IsTrue(handle) {
		t.Fatal("unknown PDR answer is not source true", handle)
	}
	must(manager.DelRef(&answer))
	if answer, handled, err = e.pdrUnknownAnswer(lbool.True); err != nil || handled || answer != (ast.ASTRef{}) {
		t.Fatal("decided PDR answer entered unknown route", answer, handled, err)
	}
	var output bytes.Buffer
	available, err := writePDRUnknownCertificate(&output, lbool.Undef)
	must(err)
	if !available || output.String() != "unknown" {
		t.Fatal("unknown PDR certificate", available, output.String())
	}
	if available, err = writePDRUnknownCertificate(pdrCertificateRefusingWriter{}, lbool.Undef); !available || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("unknown certificate writer refusal", available, err)
	}
	output.Reset()
	if available, err = writePDRUnknownCertificate(&output, lbool.True); err != nil || available || output.Len() != 0 {
		t.Fatal("decided PDR certificate entered unknown route", available, output.String(), err)
	}
}
