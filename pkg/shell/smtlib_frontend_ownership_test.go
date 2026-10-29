package shell_test

import "testing"

func TestSMTLIBFrontendBorrowsContextAndRunsOnlyOnce(t *testing.T) {
	frontend, context, _ := newBorrowedFrontend(t, "(declare-const a Bool)(assert a)(check-sat)", false, nil)
	if err := frontend.Run(); err != nil {
		t.Fatal(err)
	}
	if err := frontend.Run(); err == nil {
		t.Fatal("frontend replayed consumed input")
	}
	if err := frontend.Close(); err != nil {
		t.Fatal(err)
	}
	// Closing the frontend must not close a caller-owned context.
	_, err := context.Compliant()
	if err != nil {
		t.Fatal("borrowed context was closed:", err)
	}
}

func TestSMTLIBFrontendCloseIsIdempotent(t *testing.T) {
	frontend, _, _ := newBorrowedFrontend(t, "", false, nil)
	if err := frontend.Close(); err != nil {
		t.Fatal(err)
	}
	if err := frontend.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := frontend.Result(); err == nil {
		t.Fatal("closed frontend returned result")
	}
}
