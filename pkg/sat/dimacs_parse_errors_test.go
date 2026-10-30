package sat

import (
	"errors"
	"strings"
	"testing"
)

func TestParseDIMACSRejectsMalformedAndUnterminatedClauses(t *testing.T) {
	for _, input := range []string{"1 nope 0", "1 -2", "1\nc comment\n0"} {
		if _, err := ReadDIMACSCNF(strings.NewReader(input)); !errors.Is(err, ErrDIMACSParse) {
			t.Fatal("accepted", input, err)
		}
	}
	if _, err := ReadDIMACSCNF(nil); !errors.Is(err, ErrDIMACSParse) {
		t.Fatal("nil input", err)
	}
}
