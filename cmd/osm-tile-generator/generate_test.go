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
			name:       "unknown log level is a usage error",
			args:       []string{"generate", "-log-level", "loud"},
			wantErr:    errUsage,
			wantStderr: `invalid value "loud" for flag -log-level`,
		},
		{
			name:       "missing config directory is a runtime error",
			args:       []string{"generate", "-config-dir", missingDir},
			wantAnyErr: true,
			wantStderr: "level=INFO msg=starting command=generate",
		},
		{
			name:         "error log level hides progress logs",
			args:         []string{"generate", "-config-dir", missingDir, "-log-level", "error"},
			wantAnyErr:   true,
			rejectStderr: "level=INFO",
		},
	})
}
