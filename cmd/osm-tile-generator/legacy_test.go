package main

import (
	"path/filepath"
	"testing"
)

func TestRunLegacy(t *testing.T) {
	t.Parallel()

	missingDir := filepath.Join(t.TempDir(), "missing")
	runCLICases(t, []cliCase{
		{
			name:       "flags without a command run generate with a deprecation notice",
			args:       []string{"-config-dir", missingDir},
			wantAnyErr: true,
			wantStderr: `use "osm-tile-generator generate" instead`,
		},
		{
			name:       "serve flag runs serve with a deprecation notice",
			args:       []string{"-config-dir", missingDir, "-serve", "8080"},
			wantAnyErr: true,
			wantStderr: "-serve is deprecated",
		},
		{
			name:       "invalid serve port is a usage error",
			args:       []string{"-serve", "-1"},
			wantErr:    errUsage,
			wantStderr: "-serve must be between 1 and 65535, got -1",
		},
		{
			name:       "invalid zoom range is a usage error",
			args:       []string{"-config-dir", missingDir, "-min-zoom", "18", "-max-zoom", "14"},
			wantErr:    errUsage,
			wantStderr: "must not exceed -max-zoom",
		},
	})
}
