package z3exception

// Source: src/test/ex.cpp. The source driver has no ENSURE assertions; these
// checks preserve both caught message paths instead of depending on stderr.

import "testing"

type sourceEX interface{ sourceMessage() string }

type sourceEX2 struct{ message string }

func (e sourceEX2) sourceMessage() string { return e.message }

func sourceThrowEX2() { panic(sourceEX2{message: "testing exception"}) }

func TestExceptionSourceCustomAndDefaultCaughtMessages(t *testing.T) {
	var custom string
	func() {
		defer func() {
			value := recover()
			exception, ok := value.(sourceEX)
			if !ok {
				t.Fatalf("custom exception type %T", value)
			}
			custom = exception.sourceMessage()
		}()
		sourceThrowEX2()
	}()
	if custom != "testing exception" {
		t.Fatal("custom exception message", custom)
	}
	formatted := NewDefaultExceptionf("Format %d %s", 12, "twelve")
	if formatted.Message() != "Format 12 twelve" {
		t.Fatal("default exception message", formatted.Message())
	}
}
