// Package version provides build-time version metadata for the application.
//
// Version, Commit, and Date are overridden at build time via
//
//	-ldflags "-X github.com/Nils-witt/OSM_TileGenerator_GO/internal/version.Version=... \
//	          -X github.com/Nils-witt/OSM_TileGenerator_GO/internal/version.Commit=... \
//	          -X github.com/Nils-witt/OSM_TileGenerator_GO/internal/version.Date=..."
//
// which is what GoReleaser does for release builds. For local `go build`/`go run`
// and CI builds that don't pass ldflags, Commit and Date fall back to the VCS
// info Go automatically stamps into the binary since Go 1.18.
package version

import (
	"fmt"
	"runtime/debug"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a single-line build summary, e.g.
// "osm-tile-generator dev (commit a1b2c3d, built unknown)".
func String() string {
	commit, date, modified := Commit, Date, false
	if commit == "none" || date == "unknown" {
		commit, date, modified = vcsFallback(commit, date)
	}
	if modified {
		commit += "-dirty"
	}
	return fmt.Sprintf("osm-tile-generator %s (commit %s, built %s)", Version, commit, date)
}

// vcsFallback fills in commit/date from Go's automatic VCS build-info stamping
// when the values are still at their defaults. Returns whether the working
// tree had uncommitted changes.
func vcsFallback(commit, date string) (string, string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return commit, date, false
	}
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "none" && s.Value != "" {
				commit = s.Value
				if len(commit) >= 7 {
					commit = commit[:7]
				}
			}
		case "vcs.time":
			if date == "unknown" && s.Value != "" {
				date = s.Value
			}
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	return commit, date, modified
}
