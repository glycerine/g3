package api

// Source: src/api/api_context.cpp:390-403; src/util/version.h.

import uversion "github.com/glycerine/g3/pkg/util/version"

// APIVersion is the fixed source version tuple.
type APIVersion struct {
	Major, Minor, Build, Revision uint32
}

// Version returns the four components exposed by Z3_get_version.
func Version() APIVersion {
	return APIVersion{
		Major:    uversion.Major,
		Minor:    uversion.Minor,
		Build:    uversion.Build,
		Revision: uversion.Revision,
	}
}

// FullVersion returns the generated source version spelling.
func FullVersion() string { return uversion.Full }
