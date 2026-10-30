package base_test

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestEngineBaseDispatchesCertificateAndWriterFailure(t *testing.T) {
	core, engine := newEngineDispatchFixture(t)
	var out bytes.Buffer
	ruleMust(t, engine.DisplayCertificate(&out))
	if core.certificate != 1 || out.String() != "certificate" {
		t.Fatal("certificate dispatch", core.certificate, out.String())
	}
	if err := engine.DisplayCertificate(nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("nil certificate writer accepted", err)
	}
}
