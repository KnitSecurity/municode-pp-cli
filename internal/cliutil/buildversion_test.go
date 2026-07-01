// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import "testing"

func TestResolveVersionFrom(t *testing.T) {
	cases := []struct {
		compiled, build, want string
	}{
		{"1.0.0", "v1.0.2", "v1.0.2"}, // go install of a tagged release wins
		{"1.0.0", "(devel)", "1.0.0"}, // local/dev build keeps the compiled value
		{"1.0.0", "", "1.0.0"},        // no build info -> compiled value
		{"0.0.0-dev", "v2.3.4", "v2.3.4"},
	}
	for _, c := range cases {
		if got := resolveVersionFrom(c.compiled, c.build); got != c.want {
			t.Errorf("resolveVersionFrom(%q,%q) = %q, want %q", c.compiled, c.build, got, c.want)
		}
	}
}
