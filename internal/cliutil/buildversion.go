// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: resolve the effective version for binaries built with
// `go install` (which does not apply release-time ldflags).

package cliutil

import "runtime/debug"

// ResolveVersion returns the module version embedded by the Go toolchain
// (runtime/debug build info) when it names a real release, falling back to the
// compiled-in value otherwise. This lets `go install .../cmd/...@vX.Y.Z` report
// vX.Y.Z even though `go install` does not run a release build's -ldflags, while
// a locally built or ldflag-stamped binary keeps its compiled version.
func ResolveVersion(compiled string) string {
	buildVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		buildVersion = info.Main.Version
	}
	return resolveVersionFrom(compiled, buildVersion)
}

// resolveVersionFrom is the pure decision, split out for testing: prefer a
// concrete build version over the compiled default, but ignore the "(devel)"
// and empty placeholders the toolchain uses for non-release builds.
func resolveVersionFrom(compiled, buildVersion string) string {
	if buildVersion != "" && buildVersion != "(devel)" {
		return buildVersion
	}
	return compiled
}
