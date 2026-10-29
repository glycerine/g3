package api

import (
	"strings"
	"testing"
)

func TestPublicASTMapDisplay(t *testing.T) {
	_, manager, mapping := newPublicASTMapFixture(t)
	key := publicASTMapAtom(t, manager, "public-map-display-key")
	value := publicASTMapAtom(t, manager, "public-map-display-value")
	astTestMust(t, mapping.Insert(key, value))
	text, err := mapping.String()
	astTestMust(t, err)
	if !strings.HasPrefix(text, "(ast-map\n  (") || !strings.Contains(text, "public-map-display-key") || !strings.Contains(text, "public-map-display-value") || !strings.HasSuffix(text, "))") {
		t.Fatal("AST map display", text)
	}
}
