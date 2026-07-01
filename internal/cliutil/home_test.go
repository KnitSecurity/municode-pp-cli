// Copyright 2026 Ryan Jamieson and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"path/filepath"
	"testing"
)

func clearPathEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"MUNICODE_HOME", "MUNICODE_CONFIG_DIR", "MUNICODE_DATA_DIR", "MUNICODE_STATE_DIR",
		"MUNICODE_CACHE_DIR", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
	} {
		t.Setenv(k, "")
	}
}

func TestResolveDefaultHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Nothing configured -> ~/.municode.
	clearPathEnv(t)
	if got, want := ResolveDefaultHome(""), filepath.Join(home, DefaultHomeDirName); got != want {
		t.Errorf("bare default = %q, want %q", got, want)
	}

	// An explicit --home value means "already configured" -> no default.
	if got := ResolveDefaultHome("/somewhere"); got != "" {
		t.Errorf("with home flag = %q, want \"\"", got)
	}

	// A path env var means the user has shown intent -> no default.
	t.Setenv("MUNICODE_DATA_DIR", filepath.Join(home, "onlydata"))
	if got := ResolveDefaultHome(""); got != "" {
		t.Errorf("with MUNICODE_DATA_DIR set = %q, want \"\"", got)
	}
}

func TestAnyPathEnvConfigured(t *testing.T) {
	clearPathEnv(t)
	if anyPathEnvConfigured() {
		t.Fatal("no env set, want false")
	}
	t.Setenv("XDG_CONFIG_HOME", "/x")
	if !anyPathEnvConfigured() {
		t.Fatal("XDG_CONFIG_HOME set, want true")
	}
}
