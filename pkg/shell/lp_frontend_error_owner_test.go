package shell_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/shell"
)

func TestLPFrontendMalformedInputAndOwnerLifecycle(t *testing.T) {
	f, out := lpFrontendFixture(t, "NAME BAD\nJUNK\n", nil)
	err := f.Run()
	if err == nil || !strings.Contains(out.String(), "cannot process fixture.mps\n") {
		t.Fatalf("malformed source result err=%v out=%q", err, out.String())
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Run(); !errors.Is(err, shell.ErrShellOptionService) {
		t.Fatalf("closed frontend run %v", err)
	}
	if _, err = shell.NewLPFrontend(shell.LPFrontendConfig{}); !errors.Is(err, shell.ErrShellOptionService) {
		t.Fatalf("nil services %v", err)
	}
}
