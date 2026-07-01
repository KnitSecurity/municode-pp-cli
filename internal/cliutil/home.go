// Copyright 2026 Ryan Jamieson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: self-contained home-directory resolution. Lets the CLI and MCP
// server co-locate config, data (database + cloned <city>/ folders), state, and
// cache under a single directory instead of scattering them across XDG paths.

package cliutil

import (
	"os"
	"path/filepath"
)

// HomeMarkerName marks a directory as a self-contained municode home. When it
// sits next to the executable, the binary uses its own directory as the home
// root — a portable "install into a directory, everything lives there" setup
// with no environment variables.
const HomeMarkerName = ".municode-home"

// DefaultHomeDirName is the single-directory default under the user's home when
// nothing else is configured.
const DefaultHomeDirName = ".municode"

// ResolveDefaultHome returns the self-contained home directory to use when the
// user has NOT configured any path override (no --home, no MUNICODE_HOME, no
// per-kind MUNICODE_*_DIR, no XDG_* var). Precedence: a portable binary-relative
// home (marker next to the executable) beats the ~/.municode default. It returns
// "" when an override is already configured, so the normal cliutil resolution
// (env vars, XDG, platform default) is left untouched.
//
// Apply the result with SetHomeOverride so config/data/state/cache all resolve
// under it. Call from both the CLI root command and the MCP server so the two
// surfaces agree on where files live.
func ResolveDefaultHome(homeFlag string) string {
	if homeFlag != "" || anyPathEnvConfigured() {
		return ""
	}
	if bh := PortableBinaryHome(); bh != "" {
		return bh
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, DefaultHomeDirName)
	}
	return ""
}

// PortableBinaryHome returns the executable's directory when it contains the home
// marker (a portable install), otherwise "".
func PortableBinaryHome() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, HomeMarkerName)); err == nil {
		return dir
	}
	return ""
}

// anyPathEnvConfigured reports whether any path-controlling env var is set, so
// the self-contained default never overrides an explicit user choice.
func anyPathEnvConfigured() bool {
	for _, k := range []string{
		"MUNICODE_HOME",
		"MUNICODE_CONFIG_DIR", "MUNICODE_DATA_DIR", "MUNICODE_STATE_DIR", "MUNICODE_CACHE_DIR",
		"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
	} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}
