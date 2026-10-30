package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestFileNameWithoutExtensionMatchesDatalogSource(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"", ""},
		{"plain", "plain"},
		{"/tmp/path/name.rel", "name"},
		{`C:\tmp\a.b\name.tar.rel`, "name.tar"},
		{"/tmp/.rel", ".rel"},
		{"/tmp/name.", "name"},
		{"/tmp/path/", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.FileNameWithoutExtension(tc.name); got != tc.want {
				t.Fatalf("FileNameWithoutExtension(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}
