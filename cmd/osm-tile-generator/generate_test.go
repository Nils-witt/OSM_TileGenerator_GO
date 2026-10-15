package main

import (
	"path/filepath"
	"testing"
)

func TestRunGenerate(t *testing.T) {
	t.Parallel()

	missingDir := filepath.Join(t.TempDir(), "missing")
	runCLICases(t, []cliCase{
		{
			name:       "unknown flag is a usage error",
			args:       []string{"generate", "-zoom", "3"},
			wantErr:    errUsage,
			wantStderr: "flag provided but not defined: -zoom",
		},
		{
			name:       "positional argument is a usage error",
			args:       []string{"generate", "extra"},
			wantErr:    errUsage,
			wantStderr: `unexpected argument "extra"`,
		},
		{
			name:       "min zoom above max zoom is a usage error",
			args:       []string{"generate", "-min-zoom", "16", "-max-zoom", "15"},
			wantErr:    errUsage,
			wantStderr: "-min-zoom (16) must not exceed -max-zoom (15)",
		},
		{
			name:       "negative min zoom is a usage error",
			args:       []string{"generate", "-min-zoom", "-1"},
			wantErr:    errUsage,
			wantStderr: "-min-zoom must not be negative",
		},
		{
			name:       "missing config directory is a runtime error",
			args:       []string{"generate", "-config-dir", missingDir},
			wantAnyErr: true,
		},
	})
}
