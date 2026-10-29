package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestAPIParameterSetSourceKindsDisplayAndValidation(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	parameters, err := context.NewParameterSet()
	astTestMust(t, err)
	descriptions, err := context.ParameterDescriptions("smt")
	astTestMust(t, err)

	kind, err := descriptions.Kind("auto_config")
	astTestMust(t, err)
	if kind != ParameterBool {
		t.Fatal("SMT auto_config kind", kind)
	}
	text, err := descriptions.Documentation("auto_config")
	astTestMust(t, err)
	if text == "" {
		t.Fatal("empty source parameter documentation")
	}
	if _, err = descriptions.Name(^uint32(0)); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("descriptor bounds", err)
	}
	astTestMust(t, parameters.SetBool("auto-config", true))
	astTestMust(t, parameters.SetUInt("random_seed", 17))
	astTestMust(t, parameters.SetDouble("restart_factor", 1.25))
	astTestMust(t, parameters.SetSymbol("logic", "QF_UF"))
	display, err := parameters.String()
	astTestMust(t, err)
	for _, expected := range []string{"auto_config", "true", "random_seed", "17", "restart_factor", "1.25", "logic", "QF_UF"} {
		if !strings.Contains(display, expected) {
			t.Fatalf("parameter display %q lacks %q", display, expected)
		}
	}
	astTestMust(t, parameters.Validate(descriptions))

	astTestMust(t, descriptions.Close())
	astTestMust(t, parameters.Close())
	astTestMust(t, context.Close())
	if _, err = parameters.String(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed parameter owner", err)
	}
}

func TestAPIParameterDescriptionsCopiedCatalogLifetime(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	descriptions, err := context.ParameterDescriptions("fixedpoint")
	astTestMust(t, err)
	n, err := descriptions.Size()
	astTestMust(t, err)
	if n == 0 {
		t.Fatal("empty fixedpoint catalog")
	}
	list, err := descriptions.String()
	astTestMust(t, err)
	if !strings.HasPrefix(list, "(") || !strings.HasSuffix(list, ")") {
		t.Fatal("descriptor list", list)
	}
	astTestMust(t, descriptions.Close())
	astTestMust(t, context.Close())
}
